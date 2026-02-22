package property

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildChain builds a valid N-event chain with random payloads.
func buildChain(t *testing.T, n int, runID string) []*contracts.Event {
	t.Helper()
	hasher := ledger.NewHasher()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	events := make([]*contracts.Event, 0, n)
	for i := 0; i < n; i++ {
		event := &contracts.Event{
			EventID:   fmt.Sprintf("evt-%s-%04d", runID, i),
			RunID:     runID,
			SeqNo:     i,
			EventType: contracts.EventToolRequested,
			Timestamp: time.Now().UTC(),
			Payload:   map[string]interface{}{"rand": rng.Float64()},
		}
		if i > 0 {
			prev := events[i-1].EventHash
			event.PrevHash = &prev
		}

		hash, err := hasher.HashEvent(event)
		require.NoError(t, err)
		event.EventHash = hash
		events = append(events, event)
	}
	return events
}

// TestProperty_AppendOnly verifies that appending events never changes earlier hashes.
func TestProperty_AppendOnly(t *testing.T) {
	hasher := ledger.NewHasher()

	events := buildChain(t, 5, "run-append")

	// Record hashes from first 5 events
	originalHashes := make([]string, len(events))
	for i, e := range events {
		originalHashes[i] = e.EventHash
	}

	// Append 5 more events
	for i := 5; i < 10; i++ {
		event := &contracts.Event{
			EventID:   fmt.Sprintf("evt-run-append-%04d", i),
			RunID:     "run-append",
			SeqNo:     i,
			EventType: contracts.EventStepStarted,
			Timestamp: time.Now().UTC(),
			Payload:   map[string]interface{}{"idx": float64(i)},
		}
		prev := events[i-1].EventHash
		event.PrevHash = &prev

		hash, err := hasher.HashEvent(event)
		require.NoError(t, err)
		event.EventHash = hash
		events = append(events, event)
	}

	// Original hashes should be unchanged
	for i := 0; i < 5; i++ {
		assert.Equal(t, originalHashes[i], events[i].EventHash,
			"event %d hash should not change after appending", i)
	}

	// Full chain should still verify
	err := hasher.VerifyEventChain(events)
	assert.NoError(t, err)
}

// TestProperty_HashLinkage verifies every event at index i (i>0) has
// prev_hash == events[i-1].event_hash.
func TestProperty_HashLinkage(t *testing.T) {
	events := buildChain(t, 20, "run-link")

	for i := 1; i < len(events); i++ {
		require.NotNil(t, events[i].PrevHash)
		assert.Equal(t, events[i-1].EventHash, *events[i].PrevHash,
			"event %d prev_hash should equal event %d hash", i, i-1)
	}
}

// TestProperty_NoDuplicateHashes verifies all event hashes in a chain are unique.
func TestProperty_NoDuplicateHashes(t *testing.T) {
	events := buildChain(t, 50, "run-nodup")

	seen := make(map[string]int, len(events))
	for i, e := range events {
		if prev, ok := seen[e.EventHash]; ok {
			t.Fatalf("duplicate hash at events %d and %d: %s", prev, i, e.EventHash)
		}
		seen[e.EventHash] = i
	}
}

// TestProperty_OrderPreservation verifies seq_no values are strictly sequential.
func TestProperty_OrderPreservation(t *testing.T) {
	events := buildChain(t, 30, "run-order")

	for i, e := range events {
		assert.Equal(t, i, e.SeqNo, "event %d should have seq_no %d", i, i)
	}
}

// TestProperty_SignatureValidity signs and verifies chains of varying sizes.
func TestProperty_SignatureValidity(t *testing.T) {
	signer := ledger.NewSigner()
	hasher := ledger.NewHasher()

	pub, priv, err := signer.GenerateKey()
	require.NoError(t, err)

	for _, size := range []int{1, 5, 20, 100} {
		t.Run(fmt.Sprintf("chain_%d", size), func(t *testing.T) {
			events := buildChain(t, size, fmt.Sprintf("run-sig-%d", size))

			spec := &contracts.PolicySpec{
				Tools: []contracts.ToolPolicy{
					{Name: "test", Action: contracts.ActionAllow},
				},
			}
			specHash, err := hasher.HashPolicySpec(spec)
			require.NoError(t, err)

			pol := &contracts.Policy{SpecHash: specHash}
			outcome := &contracts.RunOutcome{ExitReason: "completed"}

			evidenceHash, sig, err := signer.SignRun(
				&contracts.Run{Outcome: outcome},
				events, pol, priv,
			)
			require.NoError(t, err)

			valid, err := signer.VerifySignature(evidenceHash, sig, pub)
			require.NoError(t, err)
			assert.True(t, valid)
		})
	}
}

// TestProperty_TamperEvidence verifies that flipping any byte in any event
// causes verification to fail.
func TestProperty_TamperEvidence(t *testing.T) {
	hasher := ledger.NewHasher()

	for trial := 0; trial < 10; trial++ {
		events := buildChain(t, 5, fmt.Sprintf("run-tamper-%d", trial))

		// Pick a random event (not first) and corrupt its payload
		idx := 1 + rand.Intn(len(events)-1)
		events[idx].Payload = map[string]interface{}{"tampered": true}

		err := hasher.VerifyEventChain(events)
		assert.Error(t, err, "trial %d: tampered event %d should break verification", trial, idx)
	}
}

// TestProperty_CrossRunIsolation verifies that chains for different run IDs
// have completely disjoint hashes.
func TestProperty_CrossRunIsolation(t *testing.T) {
	chain1 := buildChain(t, 10, "run-A")
	chain2 := buildChain(t, 10, "run-B")

	hashes1 := make(map[string]bool, len(chain1))
	for _, e := range chain1 {
		hashes1[e.EventHash] = true
	}
	for _, e := range chain2 {
		assert.False(t, hashes1[e.EventHash],
			"run-B event hash should not appear in run-A chain")
	}
}

// TestProperty_BundleSignatureRoundTrip tests that signing a bundle and
// verifying it consistently succeeds for varying data shapes.
func TestProperty_BundleSignatureRoundTrip(t *testing.T) {
	signer := ledger.NewSigner()
	hasher := ledger.NewHasher()

	pub, priv, err := signer.GenerateKey()
	require.NoError(t, err)

	outcomes := []string{"completed", "failed", "timeout", "blocked"}

	for _, reason := range outcomes {
		t.Run(reason, func(t *testing.T) {
			events := buildChain(t, 5, "run-bundle-"+reason)

			specHash, err := hasher.HashPolicySpec(&contracts.PolicySpec{
				Tools: []contracts.ToolPolicy{{Name: "t", Action: contracts.ActionAllow}},
			})
			require.NoError(t, err)

			evidenceHash, sig, err := signer.SignRun(
				&contracts.Run{Outcome: &contracts.RunOutcome{ExitReason: reason}},
				events,
				&contracts.Policy{SpecHash: specHash},
				priv,
			)
			require.NoError(t, err)

			valid, err := signer.VerifySignature(evidenceHash, sig, pub)
			require.NoError(t, err)
			assert.True(t, valid)
		})
	}
}

// TestProperty_ConcurrentChainBuilding builds multiple chains concurrently
// and verifies they are all independently valid.
func TestProperty_ConcurrentChainBuilding(t *testing.T) {
	const numChains = 20
	hasher := ledger.NewHasher()

	var wg sync.WaitGroup
	results := make([][]*contracts.Event, numChains)

	for i := 0; i < numChains; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = buildChain(t, 10, fmt.Sprintf("concurrent-%d", idx))
		}(i)
	}
	wg.Wait()

	for i, chain := range results {
		err := hasher.VerifyEventChain(chain)
		assert.NoError(t, err, "concurrent chain %d should be valid", i)
	}
}
