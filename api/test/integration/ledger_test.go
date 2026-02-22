package integration

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/aegisrun/aegisrun/internal/contracts"
	"github.com/aegisrun/aegisrun/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildEventChain creates a chain of N events with valid hash linkage.
func buildEventChain(t *testing.T, n int) []*contracts.Event {
	t.Helper()
	hasher := ledger.NewHasher()

	events := make([]*contracts.Event, 0, n)
	for i := 0; i < n; i++ {
		event := &contracts.Event{
			EventID:   "evt-" + string(rune('A'+i)),
			RunID:     "run-000",
			SeqNo:     i,
			EventType: contracts.EventToolRequested,
			Timestamp: time.Now().UTC(),
			Payload:   map[string]interface{}{"idx": float64(i)},
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

// TestLedger_HashChaining tests that events are properly hash-chained.
func TestLedger_HashChaining(t *testing.T) {
	events := buildEventChain(t, 5)
	hasher := ledger.NewHasher()

	err := hasher.VerifyEventChain(events)
	assert.NoError(t, err, "valid chain should verify")
}

// TestLedger_SignatureVerification tests Ed25519 sign/verify round-trip.
func TestLedger_SignatureVerification(t *testing.T) {
	signer := ledger.NewSigner()
	hasher := ledger.NewHasher()

	pub, priv, err := signer.GenerateKey()
	require.NoError(t, err)

	// Create a simple evidence hash
	evidenceHash, err := hasher.HashRunEvidence(
		"aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		"1122334411223344112233441122334411223344112233441122334411223344",
		[]byte(`{"exit_reason":"completed"}`),
	)
	require.NoError(t, err)

	sig, err := signer.SignEvidence(evidenceHash, priv)
	require.NoError(t, err)

	valid, err := signer.VerifySignature(evidenceHash, sig, pub)
	require.NoError(t, err)
	assert.True(t, valid, "signature should be valid")

	// Wrong key → should fail
	pub2, _, err := signer.GenerateKey()
	require.NoError(t, err)
	valid2, err := signer.VerifySignature(evidenceHash, sig, pub2)
	require.NoError(t, err)
	assert.False(t, valid2, "wrong key should fail verification")
}

// TestLedger_VerifyRunIntegrity tests full chain integrity.
func TestLedger_VerifyRunIntegrity(t *testing.T) {
	hasher := ledger.NewHasher()
	events := buildEventChain(t, 10)

	err := hasher.VerifyEventChain(events)
	assert.NoError(t, err)
}

// TestLedger_TamperDetection tests detection of tampered events.
func TestLedger_TamperDetection(t *testing.T) {
	hasher := ledger.NewHasher()

	t.Run("modify_payload", func(t *testing.T) {
		events := buildEventChain(t, 3)
		// Tamper with event 1's payload without recomputing hash
		events[1].Payload = map[string]interface{}{"tampered": true}
		err := hasher.VerifyEventChain(events)
		assert.Error(t, err, "tampered payload should be detected")
		assert.Contains(t, err.Error(), "hash mismatch")
	})

	t.Run("modify_hash", func(t *testing.T) {
		events := buildEventChain(t, 3)
		events[1].EventHash = "0000000000000000000000000000000000000000000000000000000000000000"
		err := hasher.VerifyEventChain(events)
		assert.Error(t, err, "modified hash should be detected")
	})

	t.Run("modify_prev_hash", func(t *testing.T) {
		events := buildEventChain(t, 3)
		bad := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		events[2].PrevHash = &bad
		err := hasher.VerifyEventChain(events)
		assert.Error(t, err, "broken prev_hash link should be detected")
		assert.Contains(t, err.Error(), "broken chain")
	})

	t.Run("first_event_has_prev_hash", func(t *testing.T) {
		events := buildEventChain(t, 2)
		bad := "aabbccdd"
		events[0].PrevHash = &bad
		err := hasher.VerifyEventChain(events)
		assert.Error(t, err, "first event with prev_hash should be detected")
	})

	t.Run("middle_event_nil_prev_hash", func(t *testing.T) {
		events := buildEventChain(t, 3)
		events[1].PrevHash = nil
		err := hasher.VerifyEventChain(events)
		assert.Error(t, err, "nil prev_hash in middle should be detected")
	})
}

// TestLedger_CanonicalJSON tests that canonical JSON produces sorted keys
// and deterministic output.
func TestLedger_CanonicalJSON(t *testing.T) {
	hasher := ledger.NewHasher()

	// Two events with the same data should produce the same hash
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	event1 := &contracts.Event{
		EventID:   "evt-1",
		RunID:     "run-1",
		SeqNo:     0,
		EventType: contracts.EventRunStarted,
		Timestamp: ts,
		Payload:   map[string]interface{}{"z_field": "value", "a_field": "value"},
	}
	event2 := &contracts.Event{
		EventID:   "evt-1",
		RunID:     "run-1",
		SeqNo:     0,
		EventType: contracts.EventRunStarted,
		Timestamp: ts,
		Payload:   map[string]interface{}{"a_field": "value", "z_field": "value"},
	}

	hash1, err := hasher.HashEvent(event1)
	require.NoError(t, err)
	hash2, err := hasher.HashEvent(event2)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2, "canonicalized JSON should produce same hash regardless of key order")
}

// TestLedger_BundleExport tests evidence bundle metadata creation.
func TestLedger_BundleExport(t *testing.T) {
	hasher := ledger.NewHasher()
	signer := ledger.NewSigner()

	pub, priv, err := signer.GenerateKey()
	require.NoError(t, err)
	require.NotNil(t, pub)

	events := buildEventChain(t, 3)
	lastHash := events[len(events)-1].EventHash

	policySpec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
	}
	specHash, err := hasher.HashPolicySpec(policySpec)
	require.NoError(t, err)

	outcome := &contracts.RunOutcome{ExitReason: "completed"}
	evidenceHash, signature, err := signer.SignRun(
		&contracts.Run{
			Outcome: outcome,
		},
		events,
		&contracts.Policy{SpecHash: specHash},
		priv,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, evidenceHash)
	assert.NotEmpty(t, signature)

	// Verify the signature
	valid, err := signer.VerifySignature(evidenceHash, signature, pub)
	require.NoError(t, err)
	assert.True(t, valid)

	// Evidence hash must incorporate the last event hash
	assert.NotEqual(t, lastHash, evidenceHash, "evidence hash should differ from last event hash")
}

// TestLedger_EventOrdering tests that events maintain sequence ordering.
func TestLedger_EventOrdering(t *testing.T) {
	events := buildEventChain(t, 10)

	for i, e := range events {
		assert.Equal(t, i, e.SeqNo, "event %d should have seq_no %d", i, i)
	}

	// Each event's prev_hash should match previous event's hash
	for i := 1; i < len(events); i++ {
		assert.NotNil(t, events[i].PrevHash)
		assert.Equal(t, events[i-1].EventHash, *events[i].PrevHash)
	}
}

// TestLedger_ConcurrentWrites tests that building chains concurrently for
// different runs doesn't cause cross-contamination.
func TestLedger_ConcurrentWrites(t *testing.T) {
	done := make(chan []*contracts.Event, 10)

	for i := 0; i < 10; i++ {
		go func() {
			events := buildEventChain(t, 5)
			done <- events
		}()
	}

	hasher := ledger.NewHasher()
	for i := 0; i < 10; i++ {
		events := <-done
		err := hasher.VerifyEventChain(events)
		assert.NoError(t, err, "chain %d should be valid", i)
	}
}

// TestLedger_SignRun_EndToEnd tests the full sign-run flow.
func TestLedger_SignRun_EndToEnd(t *testing.T) {
	signer := ledger.NewSigner()
	hasher := ledger.NewHasher()

	pub, priv, err := signer.GenerateKey()
	require.NoError(t, err)

	events := buildEventChain(t, 5)

	spec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{
			{Name: "http_request", Action: contracts.ActionAllow},
		},
	}
	specHash, err := hasher.HashPolicySpec(spec)
	require.NoError(t, err)

	pol := &contracts.Policy{SpecHash: specHash}
	outcome := &contracts.RunOutcome{ExitReason: "completed"}
	run := &contracts.Run{Outcome: outcome}

	evidenceHash, sig, err := signer.SignRun(run, events, pol, priv)
	require.NoError(t, err)

	valid, err := signer.VerifySignature(evidenceHash, sig, pub)
	require.NoError(t, err)
	assert.True(t, valid)

	// Different outcome → different evidence hash
	run2 := &contracts.Run{Outcome: &contracts.RunOutcome{ExitReason: "failed"}}
	evidenceHash2, _, err := signer.SignRun(run2, events, pol, priv)
	require.NoError(t, err)
	assert.NotEqual(t, evidenceHash, evidenceHash2)
}

// TestLedger_EmptyEventsSign tests that signing with no events returns error.
func TestLedger_EmptyEventsSign(t *testing.T) {
	signer := ledger.NewSigner()
	_, priv, err := signer.GenerateKey()
	require.NoError(t, err)

	_, _, err = signer.SignRun(
		&contracts.Run{},
		[]*contracts.Event{},
		&contracts.Policy{SpecHash: "aa"},
		priv,
	)
	assert.Error(t, err, "signing with no events should error")
}

// TestLedger_GenerateKey tests key generation produces valid Ed25519 key pair.
func TestLedger_GenerateKey(t *testing.T) {
	signer := ledger.NewSigner()
	pub, priv, err := signer.GenerateKey()
	require.NoError(t, err)
	assert.Len(t, pub, ed25519.PublicKeySize)
	assert.Len(t, priv, ed25519.PrivateKeySize)
}

// TestLedger_GoldenEvidenceVectors validates deterministic, fixed evidence vectors
// used as cross-module contract inputs for API and verifier behavior.
func TestLedger_GoldenEvidenceVectors(t *testing.T) {
	hasher := ledger.NewHasher()
	signer := ledger.NewSigner()

	const expectedSpecHash = "12306e591a4e25fdbd3936cfc96c74c2f1125969e59ef12cf5494639751e506c"
	const expectedEvent0Hash = "02212182fbd5a3e8f5fabc0e4cae5f13aee76786891780a083c24871ddaef379"
	const expectedEvent1Hash = "3a14ca455529a0de3758a238f680b34c9d8675ff073db8e0181a2189401a8aa5"
	const expectedRootHash = "0d7b71ac9417f1956be85f4f168df4cfb4671b8a2ab01637e2831abaae423740"
	const expectedSignatureB64 = "DjHcXWyjuJEW5ATI7hsNaVQbVC+itV8/AFHwbrobwtl0TH2TOQgs1ilmkTiDiczIE68zCV0dLHC8wDjFHPziCQ=="

	policySpec := &contracts.PolicySpec{
		Tools: []contracts.ToolPolicy{},
		Budgets: contracts.Budgets{
			MaxToolCalls: intPtr(100),
		},
	}
	specHash, err := hasher.HashPolicySpec(policySpec)
	require.NoError(t, err)
	assert.Equal(t, expectedSpecHash, specHash)

	t0 := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	e0 := &contracts.Event{
		EventID:   "evt-000",
		RunID:     "run-golden",
		SeqNo:     0,
		EventType: contracts.EventRunStarted,
		Timestamp: t0,
		Payload:   map[string]interface{}{"step": float64(0)},
	}

	h0, err := hasher.HashEvent(e0)
	require.NoError(t, err)
	e0.EventHash = h0
	assert.Equal(t, expectedEvent0Hash, h0)

	e1 := &contracts.Event{
		EventID:   "evt-001",
		RunID:     "run-golden",
		SeqNo:     1,
		EventType: contracts.EventStepStarted,
		Timestamp: t1,
		Payload:   map[string]interface{}{"step": float64(1)},
		PrevHash:  &e0.EventHash,
	}

	h1, err := hasher.HashEvent(e1)
	require.NoError(t, err)
	e1.EventHash = h1
	assert.Equal(t, expectedEvent1Hash, h1)

	rootHash, err := hasher.HashRunEvidence(h1, specHash, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedRootHash, rootHash)

	seed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)

	sig, err := signer.SignEvidence(rootHash, priv)
	require.NoError(t, err)
	assert.Equal(t, expectedSignatureB64, sig)

	pub := priv.Public().(ed25519.PublicKey)
	valid, err := signer.VerifySignature(rootHash, sig, pub)
	require.NoError(t, err)
	assert.True(t, valid)

	decodedSig, err := base64.StdEncoding.DecodeString(sig)
	require.NoError(t, err)
	assert.Len(t, decodedSig, ed25519.SignatureSize)
}

func intPtr(v int) *int {
	return &v
}
