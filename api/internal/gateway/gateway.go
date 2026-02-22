package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/policy"
	"github.com/aegisrun/aegisrun/internal/redaction"
	"github.com/aegisrun/aegisrun/internal/store"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Gateway is the central enforcement point for all tool calls
type Gateway struct {
	store         *store.Store
	runStore      *store.RunStore
	stepStore     *store.StepStore
	toolCallStore *store.ToolCallStore
	eventStore    *store.EventStore
	policyStore   *store.PolicyStore

	evaluator     *policy.Evaluator
	compiler      *policy.Compiler
	budgetTracker *BudgetTracker
	executors     *ExecutorRegistry

	logger *zap.Logger
}

func NewGateway(
	st *store.Store,
	runStore *store.RunStore,
	stepStore *store.StepStore,
	toolCallStore *store.ToolCallStore,
	eventStore *store.EventStore,
	policyStore *store.PolicyStore,
	logger *zap.Logger,
) *Gateway {
	return &Gateway{
		store:         st,
		runStore:      runStore,
		stepStore:     stepStore,
		toolCallStore: toolCallStore,
		eventStore:    eventStore,
		policyStore:   policyStore,
		evaluator:     policy.NewEvaluator(),
		compiler:      policy.NewCompiler(),
		budgetTracker: NewBudgetTracker(runStore),
		executors:     NewExecutorRegistry(),
		logger:        logger,
	}
}

// ExecuteToolCall is the main entry point for tool execution
func (g *Gateway) ExecuteToolCall(ctx context.Context, req *ToolCallRequest) (*ToolCallResponse, error) {
	tracer := otel.Tracer("aegisrun-gateway")
	ctx, span := tracer.Start(ctx, "gateway.ExecuteToolCall",
		trace.WithAttributes(
			attribute.String("run_id", req.RunID),
			attribute.String("step_id", req.StepID),
			attribute.String("tool_name", req.ToolName),
		),
	)
	defer span.End()

	g.logger.Info("tool call requested",
		zap.String("run_id", req.RunID),
		zap.String("step_id", req.StepID),
		zap.String("tool_name", req.ToolName))

	// 1. Load run and policy
	run, err := g.runStore.Get(ctx, req.RunID)
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}

	policyDoc, err := g.policyStore.Get(ctx, run.PolicyRef.PolicyID, run.PolicyRef.Version)
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}

	// 2. Compile policy
	compiledPolicy, err := g.compiler.Compile(&policyDoc.Spec)
	if err != nil {
		return nil, fmt.Errorf("compile policy: %w", err)
	}

	// 3. Initialize budget tracking from policy budgets (hydrates counters from DB)
	g.budgetTracker.InitializeBudget(ctx, req.RunID, compiledPolicy.Budgets)

	// 4. Check budget before proceeding
	if err := g.budgetTracker.CheckBudget(req.RunID); err != nil {
		return &ToolCallResponse{
			Decision: contracts.Decision{
				Action: contracts.ActionBlock,
				Reason: fmt.Sprintf("budget exceeded: %v", err),
			},
			Error: fmt.Sprintf("Blocked by budget: %v", err),
		}, nil
	}

	// 5. Get current counters
	counters, err := g.runStore.GetCounters(ctx, req.RunID)
	if err != nil {
		return nil, fmt.Errorf("get counters: %w", err)
	}

	// 6. Redact args if needed
	redactedArgs := req.Args
	argsRedacted := false
	if compiledPolicy.Redaction != nil {
		redactor := redaction.NewRedactor(compiledPolicy.RedactionPatterns, compiledPolicy.Redaction.MaskStrategy)
		redactedArgs, argsRedacted = redactor.RedactMap(req.Args)
	}

	// 7. Evaluate policy
	evalCtx := &policy.EvaluationContext{
		ToolName:    req.ToolName,
		Args:        redactedArgs,
		StateVector: req.StateVector,
		Metadata:    req.Metadata,
		Counters:    counters,
		RunMetadata: run.Metadata,
	}

	decision, err := g.evaluator.Evaluate(compiledPolicy, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("policy evaluation: %w", err)
	}

	// 8. Create tool call record
	toolCallID := ulid.Make().String()
	seqNo := counters.ToolCalls

	toolCall := &contracts.ToolCall{
		ToolCallID:   toolCallID,
		RunID:        req.RunID,
		StepID:       req.StepID,
		SeqNo:        seqNo,
		ToolName:     req.ToolName,
		Args:         redactedArgs,
		ArgsRedacted: argsRedacted,
		RequestedAt:  time.Now().UTC(),
		Decision:     *decision,
		Metadata: contracts.ToolCallMetadata{
			Executor:   req.Executor,
			RetryCount: 0,
		},
	}

	// 9. Persist in transaction
	var response *ToolCallResponse
	err = g.store.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Acquire run lock to serialize events
		if err := g.store.AcquireRunLock(ctx, tx, req.RunID); err != nil {
			return err
		}

		// Save tool call
		if err := g.toolCallStore.Create(ctx, tx, toolCall); err != nil {
			return fmt.Errorf("create tool_call: %w", err)
		}

		// Increment counter
		if err := g.runStore.IncrementCounters(ctx, tx, req.RunID, "tool_calls", 1); err != nil {
			return fmt.Errorf("increment tool_calls counter: %w", err)
		}

		// Emit events
		if err := g.emitToolRequestedEvent(ctx, tx, toolCall); err != nil {
			return fmt.Errorf("emit tool.requested event: %w", err)
		}

		if err := g.emitToolDecidedEvent(ctx, tx, toolCall); err != nil {
			return fmt.Errorf("emit tool.decided event: %w", err)
		}

		// Execute based on decision
		response = &ToolCallResponse{
			ToolCallID: toolCallID,
			Decision:   *decision,
		}

		switch decision.Action {
		case contracts.ActionAllow:
			// Execute the tool
			execStart := time.Now()
			result, execErr := g.executeToolUnsafe(ctx, req.ToolName, req.Args, req.Executor)
			execDurationMs := float64(time.Since(execStart).Microseconds()) / 1000.0
			if execErr != nil {
				response.Error = execErr.Error()
			} else {
				// Redact response if needed
				responseRedacted := false
				if compiledPolicy.Redaction != nil {
					redactor := redaction.NewRedactor(compiledPolicy.RedactionPatterns, compiledPolicy.Redaction.MaskStrategy)
					result, responseRedacted = redactor.RedactValue(result)
				}

				response.Result = result

				// Validate output schema
				if toolPolicy, exists := compiledPolicy.ToolPolicies[req.ToolName]; exists {
					if err := g.evaluator.ValidateOutput(toolPolicy, result); err != nil {
						g.logger.Warn("output schema validation failed",
							zap.String("tool", req.ToolName),
							zap.Error(err))
					}
				}

				// Save response
				toolResp := &contracts.ToolResponse{
					Result:     result,
					DurationMs: execDurationMs,
				}
				if err := g.toolCallStore.UpdateResponse(ctx, tx, toolCallID, toolResp, responseRedacted); err != nil {
					return fmt.Errorf("update tool_call response: %w", err)
				}

				// Track bytes egressed (heuristic)
				if bytesEgressed := estimateBytesEgressed(result); bytesEgressed > 0 {
					if err := g.runStore.IncrementCounters(ctx, tx, req.RunID, "bytes_egressed", bytesEgressed); err != nil {
						return fmt.Errorf("increment bytes_egressed: %w", err)
					}
				}
			}

		case contracts.ActionWarn:
			// Execute but log warning — persist response and track bytes like ActionAllow
			g.logger.Warn("tool call allowed with warning",
				zap.String("tool", req.ToolName),
				zap.String("reason", decision.Reason))

			execStart := time.Now()
			result, execErr := g.executeToolUnsafe(ctx, req.ToolName, req.Args, req.Executor)
			execDurationMs := float64(time.Since(execStart).Microseconds()) / 1000.0
			if execErr != nil {
				response.Error = execErr.Error()
			} else {
				// Redact response if needed
				responseRedacted := false
				if compiledPolicy.Redaction != nil {
					redactor := redaction.NewRedactor(compiledPolicy.RedactionPatterns, compiledPolicy.Redaction.MaskStrategy)
					result, responseRedacted = redactor.RedactValue(result)
				}

				response.Result = result

				// Save response
				toolResp := &contracts.ToolResponse{
					Result:     result,
					DurationMs: execDurationMs,
				}
				if err := g.toolCallStore.UpdateResponse(ctx, tx, toolCallID, toolResp, responseRedacted); err != nil {
					return fmt.Errorf("update tool_call response: %w", err)
				}

				// Track bytes egressed
				if bytesEgressed := estimateBytesEgressed(result); bytesEgressed > 0 {
					if err := g.runStore.IncrementCounters(ctx, tx, req.RunID, "bytes_egressed", bytesEgressed); err != nil {
						return fmt.Errorf("increment bytes_egressed: %w", err)
					}
				}
			}

		case contracts.ActionBlock:
			// Blocked
			response.Error = fmt.Sprintf("Blocked by policy: %s", decision.Reason)
			if err := g.runStore.IncrementCounters(ctx, tx, req.RunID, "blocks", 1); err != nil {
				return fmt.Errorf("increment blocks counter: %w", err)
			}

		case contracts.ActionRedact:
			// Execute but heavily redact output — persist redacted response
			execStart := time.Now()
			_, execErr := g.executeToolUnsafe(ctx, req.ToolName, req.Args, req.Executor)
			execDurationMs := float64(time.Since(execStart).Microseconds()) / 1000.0
			if execErr != nil {
				response.Error = execErr.Error()
			} else {
				redactedResult := "[REDACTED BY POLICY]"
				response.Result = redactedResult

				// Save redacted response
				toolResp := &contracts.ToolResponse{
					Result:     redactedResult,
					DurationMs: execDurationMs,
				}
				if err := g.toolCallStore.UpdateResponse(ctx, tx, toolCallID, toolResp, true); err != nil {
					return fmt.Errorf("update tool_call response: %w", err)
				}
			}

		case contracts.ActionRequireApproval:
			// Queue for approval (not implemented in this iteration - return error)
			response.Error = "Tool call requires approval (approval workflow not yet implemented)"

		case contracts.ActionDegrade:
			// Execute in degraded mode (limited functionality)
			response.Result = map[string]interface{}{
				"degraded": true,
				"message":  "Tool executed in degraded mode per policy",
			}
		}

		// Emit tool.responded event
		if err := g.emitToolRespondedEvent(ctx, tx, toolCall, response); err != nil {
			return fmt.Errorf("emit tool.responded event: %w", err)
		}

		// Track tool call in budget tracker
		g.budgetTracker.IncrementToolCalls(req.RunID)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("execute tool call transaction: %w", err)
	}

	return response, nil
}

func (g *Gateway) executeToolUnsafe(ctx context.Context, toolName string, args map[string]interface{}, executorName string) (interface{}, error) {
	executor := g.executors.Get(executorName)
	if executor == nil {
		return nil, fmt.Errorf("executor not found: %s", executorName)
	}

	return executor.Execute(ctx, toolName, args)
}

func estimateBytesEgressed(result interface{}) int {
	if result == nil {
		return 0
	}
	data, err := json.Marshal(result)
	if err != nil {
		return 0
	}
	return len(data)
}

type ToolCallRequest struct {
	RunID       string                 `json:"run_id"`
	StepID      string                 `json:"step_id"`
	ToolName    string                 `json:"tool_name"`
	Args        map[string]interface{} `json:"args"`
	StateVector map[string]interface{} `json:"state_vector"`
	Metadata    map[string]interface{} `json:"metadata"`
	Executor    string                 `json:"executor"` // "builtin", "http", etc.
}

type ToolCallResponse struct {
	ToolCallID string             `json:"tool_call_id"`
	Decision   contracts.Decision `json:"decision"`
	Result     interface{}        `json:"result,omitempty"`
	Error      string             `json:"error,omitempty"`
}
