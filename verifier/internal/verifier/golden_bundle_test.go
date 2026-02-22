package verifier

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/aegisrun/aegis-verify/internal/bundle"
)

func TestGoldenEvidenceBundle_VerifiesAcrossAllChecks(t *testing.T) {
	const expectedSpecHash = "db5dfe95fdd456154afa23ebd8248976f5be02f3140d4fe8844df1e967bb1b0b"
	const expectedEvent0Hash = "02212182fbd5a3e8f5fabc0e4cae5f13aee76786891780a083c24871ddaef379"
	const expectedEvent1Hash = "3a14ca455529a0de3758a238f680b34c9d8675ff073db8e0181a2189401a8aa5"
	const expectedRootHash = "921d3f087e5c3d613f5e8c3b5e5f2f2c27e79ec9c42d9632d32a1a53a4a4212d"
	const expectedSignatureB64 = "maEF8yMy+kcx0qdxWM0hu4lDaHShhZv1N8SO7s3fZundJ/KyDGQb5sLdzTjpEjQssyqjIZdLtXuAYX5wkuyqAA=="
	const expectedPublicKeyB64 = "ebVWLo/mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ="

	manifest := map[string]interface{}{
		"bundle_version": "1.0.0",
		"run_id":         "run-golden",
		"exported_at":    "2026-02-22T00:00:02Z",
		"event_count":    2,
		"root_hash":      expectedRootHash,
		"signature":      expectedSignatureB64,
		"signer_key_id":  "key-golden",
	}

	eventsJSONL := []byte(
		`{"event_id":"evt-000","run_id":"run-golden","seq_no":0,"event_type":"run.started","timestamp":"2026-02-22T00:00:00Z","payload":{"step":0},"event_hash":"` + expectedEvent0Hash + `"}` + "\n" +
			`{"event_id":"evt-001","run_id":"run-golden","seq_no":1,"event_type":"step.started","timestamp":"2026-02-22T00:00:01Z","payload":{"step":1},"prev_hash":"` + expectedEvent0Hash + `","event_hash":"` + expectedEvent1Hash + `"}` + "\n",
	)

	policy := map[string]interface{}{
		"policy_id":  "policy-golden",
		"org_id":     "org-golden",
		"name":       "golden-policy",
		"version":    "v1",
		"status":     "deployed",
		"spec":       map[string]interface{}{"rules": []interface{}{}, "budgets": map[string]interface{}{"max_tool_calls": 100}},
		"spec_hash":  expectedSpecHash,
		"created_at": "2026-02-22T00:00:00Z",
	}

	run := map[string]interface{}{
		"run_id":        "run-golden",
		"org_id":        "org-golden",
		"policy_ref":    map[string]interface{}{"policy_id": "policy-golden", "version": "v1"},
		"metadata":      map[string]interface{}{},
		"created_at":    "2026-02-22T00:00:00Z",
		"status":        "completed",
		"counters":      map[string]interface{}{"steps": 1, "tool_calls": 1, "bytes_egressed": 0, "retries": 0, "blocks": 0},
		"evidence_hash": expectedRootHash,
		"signature":     expectedSignatureB64,
	}

	pubRaw, err := base64.StdEncoding.DecodeString(expectedPublicKeyB64)
	if err != nil {
		t.Fatalf("decode golden public key: %v", err)
	}
	pkixBytes, err := x509.MarshalPKIXPublicKey(ed25519.PublicKey(pubRaw))
	if err != nil {
		t.Fatalf("marshal pkix public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkixBytes})

	bundlePath := createGoldenBundleZip(t, map[string][]byte{
		"manifest.json":        mustMarshalJSON(t, manifest),
		"events.jsonl":         eventsJSONL,
		"policy_snapshot.json": mustMarshalJSON(t, policy),
		"run.json":             mustMarshalJSON(t, run),
		"public_key.pem":       publicKeyPEM,
	})

	b, err := bundle.Load(bundlePath)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}

	if b.Manifest.RootHash != expectedRootHash {
		t.Fatalf("unexpected root hash: got %s want %s", b.Manifest.RootHash, expectedRootHash)
	}
	if len(b.Events) != 2 {
		t.Fatalf("unexpected event count: got %d", len(b.Events))
	}

	cv := NewChainVerifier(Config{})
	chainResult := cv.Verify(b.Events)
	if !chainResult.Passed {
		t.Fatalf("chain verification failed: %s", chainResult.Message)
	}

	pv := NewPolicyVerifier(Config{})
	policyResult := pv.Verify(b)
	if !policyResult.Passed {
		t.Fatalf("policy verification failed: %s", policyResult.Message)
	}

	sv := NewSignatureVerifier(Config{})
	signatureResult := sv.Verify(b)
	if !signatureResult.Passed {
		t.Fatalf("signature verification failed: %s", signatureResult.Message)
	}
}

func createGoldenBundleZip(t *testing.T, components map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "golden-bundle.zip")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, data := range components {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	return path
}

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return b
}
