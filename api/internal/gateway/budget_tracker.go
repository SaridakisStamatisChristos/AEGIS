package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/store"
)

// BudgetTracker tracks runtime budgets per run.
// On initialization it hydrates current counters from the database so that
// budget enforcement survives process restarts and works across replicas.
type BudgetTracker struct {
	mu       sync.RWMutex
	budgets  map[string]*RunBudget
	runStore *store.RunStore
}

func NewBudgetTracker(runStore *store.RunStore) *BudgetTracker {
	return &BudgetTracker{
		budgets:  make(map[string]*RunBudget),
		runStore: runStore,
	}
}

type RunBudget struct {
	RunID           string
	StartTime       time.Time
	ToolCalls       int
	BytesEgressed   int
	Retries         int
	Blocks          int
	MaxToolCalls    *int
	MaxWallClockSec *int
	MaxRetries      *int
	MaxBytesEgr     *int
}

// InitializeBudget creates or refreshes a budget for a run. Current counters
// are loaded from the database so that already-consumed budget is accounted for.
// C-03 fix: Preserve StartTime from existing entry so wall-clock budget works.
func (bt *BudgetTracker) InitializeBudget(ctx context.Context, runID string, budgets contracts.Budgets) {
	bt.mu.Lock()
	existing, exists := bt.budgets[runID]
	bt.mu.Unlock()

	rb := &RunBudget{
		RunID:           runID,
		MaxToolCalls:    budgets.MaxToolCalls,
		MaxWallClockSec: budgets.MaxWallClockSec,
		MaxRetries:      budgets.MaxRetries,
		MaxBytesEgr:     budgets.MaxBytesEgressed,
	}

	// Preserve the original StartTime if we already track this run.
	if exists && !existing.StartTime.IsZero() {
		rb.StartTime = existing.StartTime
	} else {
		rb.StartTime = time.Now()
	}

	// Hydrate from DB so restarts / new replicas see the real counts.
	if bt.runStore != nil {
		if counters, err := bt.runStore.GetCounters(ctx, runID); err == nil {
			rb.ToolCalls = counters.ToolCalls
			rb.BytesEgressed = counters.BytesEgressed
			rb.Retries = counters.Retries
			rb.Blocks = counters.Blocks
		}
	}

	bt.mu.Lock()
	bt.budgets[runID] = rb
	bt.mu.Unlock()
}

func (bt *BudgetTracker) CheckBudget(runID string) error {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	budget, exists := bt.budgets[runID]
	if !exists {
		return nil // No budget tracking for this run
	}

	if budget.MaxToolCalls != nil && budget.ToolCalls >= *budget.MaxToolCalls {
		return fmt.Errorf("tool call budget exceeded: %d/%d", budget.ToolCalls, *budget.MaxToolCalls)
	}

	if budget.MaxWallClockSec != nil {
		elapsed := int(time.Since(budget.StartTime).Seconds())
		if elapsed >= *budget.MaxWallClockSec {
			return fmt.Errorf("wall clock budget exceeded: %ds/%ds", elapsed, *budget.MaxWallClockSec)
		}
	}

	if budget.MaxRetries != nil && budget.Retries >= *budget.MaxRetries {
		return fmt.Errorf("retry budget exceeded: %d/%d", budget.Retries, *budget.MaxRetries)
	}

	if budget.MaxBytesEgr != nil && budget.BytesEgressed >= *budget.MaxBytesEgr {
		return fmt.Errorf("egress budget exceeded: %d/%d bytes", budget.BytesEgressed, *budget.MaxBytesEgr)
	}

	return nil
}

func (bt *BudgetTracker) IncrementToolCalls(runID string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if budget, exists := bt.budgets[runID]; exists {
		budget.ToolCalls++
	}
}

func (bt *BudgetTracker) IncrementBytesEgressed(runID string, bytes int) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if budget, exists := bt.budgets[runID]; exists {
		budget.BytesEgressed += bytes
	}
}

func (bt *BudgetTracker) IncrementRetries(runID string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if budget, exists := bt.budgets[runID]; exists {
		budget.Retries++
	}
}

func (bt *BudgetTracker) IncrementBlocks(runID string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if budget, exists := bt.budgets[runID]; exists {
		budget.Blocks++
	}
}

func (bt *BudgetTracker) RemoveBudget(runID string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	delete(bt.budgets, runID)
}

func (bt *BudgetTracker) GetBudget(runID string) *RunBudget {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	b := bt.budgets[runID]
	if b == nil {
		return nil
	}
	// H-02: Return a copy to prevent external mutation without holding the mutex.
	copy := *b
	return &copy
}
