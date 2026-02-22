package verifier

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/aegisrun/aegis-verify/internal/bundle"
	canonicaljson "github.com/gibson042/canonicaljson-go"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildHashChain constructs a valid hash chain of n events for testing.
func buildHashChain(n int) []bundle.Event {
	events := make([]bundle.Event, 0, n)
	for i := 0; i < n; i++ {
		event := bundle.Event{
			EventID:   fmt.Sprintf("evt-%03d", i),
			RunID:     "run-test",
			SeqNo:     i,
			EventType: "step.started",
			Payload:   map[string]interface{}{"step": float64(i)},
		}

		if i == 0 {
			event.PrevHash = nil
		} else {
			prev := events[i-1].EventHash
			event.PrevHash = &prev
		}

		// Compute hash the same way ChainVerifier does.
		eventCopy := event
		eventCopy.EventHash = ""
		canonical, err := canonicaljson.Marshal(eventCopy)
		if err != nil {
			panic(err)
		}

		var prevBytes []byte
		if event.PrevHash != nil && *event.PrevHash != "" {
			prevBytes, _ = hex.DecodeString(*event.PrevHash)
		}
		toHash := append(canonical, prevBytes...)
		h := sha256.Sum256(toHash)
		event.EventHash = hex.EncodeToString(h[:])

		events = append(events, event)
	}
	return events
}

// ---------------------------------------------------------------------------
// ChainVerifier
// ---------------------------------------------------------------------------

func TestChainVerifier_EmptyChain(t *testing.T) {
	cv := NewChainVerifier(Config{})
	result := cv.Verify(nil)

	if !result.Passed {
		t.Fatalf("expected pass on empty chain, got: %s", result.Message)
	}
}

func TestChainVerifier_SingleEvent(t *testing.T) {
	events := buildHashChain(1)
	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if !result.Passed {
		t.Fatalf("expected pass, got: %s", result.Message)
	}
}

func TestChainVerifier_MultipleEvents(t *testing.T) {
	for _, n := range []int{2, 5, 10, 50} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			events := buildHashChain(n)
			cv := NewChainVerifier(Config{})
			result := cv.Verify(events)

			if !result.Passed {
				t.Fatalf("expected pass for %d events, got: %s", n, result.Message)
			}
		})
	}
}

func TestChainVerifier_VerboseOutput(t *testing.T) {
	events := buildHashChain(3)
	cv := NewChainVerifier(Config{Verbose: true})
	result := cv.Verify(events)

	if !result.Passed {
		t.Fatalf("expected pass: %s", result.Message)
	}
}

func TestChainVerifier_TamperedPayload(t *testing.T) {
	events := buildHashChain(5)
	// Tamper with event 2 payload after hash was computed.
	events[2].Payload["injected"] = "evil"

	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if result.Passed {
		t.Fatal("expected failure for tampered payload")
	}
	if result.Message == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestChainVerifier_BrokenPrevHash(t *testing.T) {
	events := buildHashChain(5)
	bad := "0000000000000000000000000000000000000000000000000000000000000000"
	events[3].PrevHash = &bad

	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if result.Passed {
		t.Fatal("expected failure for broken prev_hash link")
	}
}

func TestChainVerifier_NilPrevHashOnNonFirst(t *testing.T) {
	events := buildHashChain(3)
	events[1].PrevHash = nil

	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if result.Passed {
		t.Fatal("expected failure when non-first event has nil prev_hash")
	}
}

func TestChainVerifier_FirstEventNonNilPrevHash(t *testing.T) {
	events := buildHashChain(3)
	bad := "deadbeef"
	events[0].PrevHash = &bad

	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if result.Passed {
		t.Fatal("expected failure when first event has non-nil prev_hash")
	}
}

func TestChainVerifier_WrongSeqNo(t *testing.T) {
	events := buildHashChain(3)
	events[1].SeqNo = 99

	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if result.Passed {
		t.Fatal("expected failure for wrong seq_no")
	}
}

func TestChainVerifier_SwappedEvents(t *testing.T) {
	events := buildHashChain(4)
	events[1], events[2] = events[2], events[1]

	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if result.Passed {
		t.Fatal("expected failure for swapped events")
	}
}

func TestChainVerifier_DuplicateEvent(t *testing.T) {
	events := buildHashChain(3)
	events = append(events, events[2]) // duplicate last

	cv := NewChainVerifier(Config{})
	result := cv.Verify(events)

	if result.Passed {
		t.Fatal("expected failure for duplicate event at end")
	}
}

// ---------------------------------------------------------------------------
// PolicyVerifier
// ---------------------------------------------------------------------------

func TestPolicyVerifier_ValidPolicy(t *testing.T) {
	spec := map[string]interface{}{
		"max_tool_calls": float64(100),
		"rules":          []interface{}{},
	}
	specCanonical, _ := canonicaljson.Marshal(spec)
	h := sha256.Sum256(specCanonical)

	b := &bundle.Bundle{
		Policy: &bundle.Policy{
			PolicyID: "p1",
			Name:     "test-policy",
			Version:  "v1",
			Spec:     spec,
			SpecHash: hex.EncodeToString(h[:]),
		},
	}

	pv := NewPolicyVerifier(Config{})
	result := pv.Verify(b)

	if !result.Passed {
		t.Fatalf("expected pass: %s", result.Message)
	}
}

func TestPolicyVerifier_TamperedSpec(t *testing.T) {
	spec := map[string]interface{}{"rules": []interface{}{}}
	specCanonical, _ := canonicaljson.Marshal(spec)
	h := sha256.Sum256(specCanonical)

	b := &bundle.Bundle{
		Policy: &bundle.Policy{
			PolicyID: "p1",
			Name:     "test-policy",
			Version:  "v1",
			Spec:     map[string]interface{}{"rules": []interface{}{"injected_rule"}},
			SpecHash: hex.EncodeToString(h[:]),
		},
	}

	pv := NewPolicyVerifier(Config{})
	result := pv.Verify(b)

	if result.Passed {
		t.Fatal("expected failure for tampered spec")
	}
}

func TestPolicyVerifier_NilPolicy(t *testing.T) {
	b := &bundle.Bundle{Policy: nil}
	pv := NewPolicyVerifier(Config{})
	result := pv.Verify(b)

	if result.Passed {
		t.Fatal("expected failure for nil policy")
	}
}

func TestPolicyVerifier_VerboseOutput(t *testing.T) {
	spec := map[string]interface{}{}
	specCanonical, _ := canonicaljson.Marshal(spec)
	h := sha256.Sum256(specCanonical)

	b := &bundle.Bundle{
		Policy: &bundle.Policy{
			PolicyID: "p1",
			Name:     "verbose-test",
			Version:  "v2",
			Spec:     spec,
			SpecHash: hex.EncodeToString(h[:]),
		},
	}

	pv := NewPolicyVerifier(Config{Verbose: true})
	result := pv.Verify(b)

	if !result.Passed {
		t.Fatalf("expected pass: %s", result.Message)
	}
}

// ---------------------------------------------------------------------------
// SignatureVerifier
// ---------------------------------------------------------------------------

func TestSignatureVerifier_ValidSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("key gen: %v", err)
	}

	// Build minimal bundle
	events := buildHashChain(3)
	lastEventHash := events[len(events)-1].EventHash

	spec := map[string]interface{}{"rules": []interface{}{}}
	specCanonical, _ := canonicaljson.Marshal(spec)
	specHash := sha256.Sum256(specCanonical)
	specHashHex := hex.EncodeToString(specHash[:])

	// Evidence hash = SHA256(last_event_hash || policy_spec_hash)
	lastBytes, _ := hex.DecodeString(lastEventHash)
	specBytes, _ := hex.DecodeString(specHashHex)
	toHash := append(lastBytes, specBytes...)
	evHash := sha256.Sum256(toHash)
	evidenceHashHex := hex.EncodeToString(evHash[:])

	// Sign the evidence hash
	evHashBytes, _ := hex.DecodeString(evidenceHashHex)
	sig := ed25519.Sign(priv, evHashBytes)
	sigBase64 := base64.StdEncoding.EncodeToString(sig)

	b := &bundle.Bundle{
		Manifest: &bundle.Manifest{
			Signature:   sigBase64,
			SignerKeyID: "key-1",
		},
		Events: events,
		Policy: &bundle.Policy{
			Spec:     spec,
			SpecHash: specHashHex,
		},
		Run:       &bundle.Run{Outcome: nil},
		PublicKey: pub,
	}

	sv := NewSignatureVerifier(Config{})
	result := sv.Verify(b)

	if !result.Passed {
		t.Fatalf("expected pass: %s", result.Message)
	}
}

func TestSignatureVerifier_InvalidSignature(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)

	events := buildHashChain(2)
	spec := map[string]interface{}{}
	specCanonical, _ := canonicaljson.Marshal(spec)
	h := sha256.Sum256(specCanonical)

	b := &bundle.Bundle{
		Manifest: &bundle.Manifest{
			Signature:   base64.StdEncoding.EncodeToString([]byte("not-a-real-signature-of-64-bytes-xxxxxxxxxxxxxxxxxxxxxxxxxxx12345")),
			SignerKeyID: "key-1",
		},
		Events: events,
		Policy: &bundle.Policy{
			Spec:     spec,
			SpecHash: hex.EncodeToString(h[:]),
		},
		Run:       &bundle.Run{},
		PublicKey: pub,
	}

	sv := NewSignatureVerifier(Config{})
	result := sv.Verify(b)

	if result.Passed {
		t.Fatal("expected failure for invalid signature")
	}
}

func TestSignatureVerifier_NoSignature(t *testing.T) {
	b := &bundle.Bundle{
		Manifest: &bundle.Manifest{Signature: ""},
	}

	sv := NewSignatureVerifier(Config{})
	result := sv.Verify(b)

	if result.Passed {
		t.Fatal("expected failure when no signature")
	}
}

func TestSignatureVerifier_NoPublicKey(t *testing.T) {
	b := &bundle.Bundle{
		Manifest:  &bundle.Manifest{Signature: "abc"},
		PublicKey: nil,
	}

	sv := NewSignatureVerifier(Config{})
	result := sv.Verify(b)

	if result.Passed {
		t.Fatal("expected failure when no public key")
	}
}

func TestSignatureVerifier_NoEvents(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	b := &bundle.Bundle{
		Manifest:  &bundle.Manifest{Signature: "abc"},
		Events:    nil,
		PublicKey: pub,
	}

	sv := NewSignatureVerifier(Config{})
	result := sv.Verify(b)

	if result.Passed {
		t.Fatal("expected failure when no events")
	}
}

func TestSignatureVerifier_NoPolicy(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	b := &bundle.Bundle{
		Manifest:  &bundle.Manifest{Signature: "abc"},
		Events:    buildHashChain(1),
		Policy:    nil,
		PublicKey: pub,
	}

	sv := NewSignatureVerifier(Config{})
	result := sv.Verify(b)

	if result.Passed {
		t.Fatal("expected failure when no policy")
	}
}

func TestSignatureVerifier_WithOutcome(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)

	events := buildHashChain(2)
	lastEventHash := events[len(events)-1].EventHash

	spec := map[string]interface{}{}
	specCanonical, _ := canonicaljson.Marshal(spec)
	specHash := sha256.Sum256(specCanonical)
	specHashHex := hex.EncodeToString(specHash[:])

	outcome := map[string]interface{}{"result": "success"}
	outcomeCanonical, _ := canonicaljson.Marshal(outcome)

	lastBytes, _ := hex.DecodeString(lastEventHash)
	specBytes, _ := hex.DecodeString(specHashHex)
	toHash := append(lastBytes, specBytes...)
	toHash = append(toHash, outcomeCanonical...)
	evHash := sha256.Sum256(toHash)
	evidenceHashHex := hex.EncodeToString(evHash[:])

	evHashBytes, _ := hex.DecodeString(evidenceHashHex)
	sig := ed25519.Sign(priv, evHashBytes)

	b := &bundle.Bundle{
		Manifest: &bundle.Manifest{
			Signature:   base64.StdEncoding.EncodeToString(sig),
			SignerKeyID: "key-1",
		},
		Events: events,
		Policy: &bundle.Policy{
			Spec:     spec,
			SpecHash: specHashHex,
		},
		Run:       &bundle.Run{Outcome: outcome},
		PublicKey: pub,
	}

	sv := NewSignatureVerifier(Config{Verbose: true})
	result := sv.Verify(b)

	if !result.Passed {
		t.Fatalf("expected pass with outcome: %s", result.Message)
	}
}

// ---------------------------------------------------------------------------
// VerificationResult output
// ---------------------------------------------------------------------------

func TestVerificationResult_NewResult(t *testing.T) {
	r := NewResult("/tmp/bundle.zip", "run-123", 10)

	if r.BundlePath != "/tmp/bundle.zip" {
		t.Errorf("unexpected BundlePath: %s", r.BundlePath)
	}
	if r.RunID != "run-123" {
		t.Errorf("unexpected RunID: %s", r.RunID)
	}
	if r.EventCount != 10 {
		t.Errorf("unexpected EventCount: %d", r.EventCount)
	}
	if !r.Valid {
		t.Error("new result should default to Valid=true")
	}
}

func TestVerificationResult_OutputJSON(t *testing.T) {
	r := NewResult("/tmp/b.zip", "run-1", 5)
	r.ChainIntegrity = CheckResult{Passed: true, Message: "ok"}
	r.SignatureValid = CheckResult{Passed: true, Message: "ok"}
	r.PolicyIntegrity = CheckResult{Passed: true, Message: "ok"}

	// Should not panic
	err := r.OutputJSON()
	if err != nil {
		t.Fatalf("OutputJSON failed: %v", err)
	}
}

func TestVerificationResult_OutputText(t *testing.T) {
	r := NewResult("/tmp/b.zip", "run-2", 3)
	r.ChainIntegrity = CheckResult{Passed: true, Message: "verified"}
	r.SignatureValid = CheckResult{Passed: false, Message: "no key"}
	r.PolicyIntegrity = CheckResult{Passed: true, Message: "ok"}
	r.Valid = false
	r.Warnings = []string{"warn-1"}
	r.Errors = []string{"err-1"}

	err := r.OutputText()
	if err != nil {
		t.Fatalf("OutputText failed: %v", err)
	}
}
