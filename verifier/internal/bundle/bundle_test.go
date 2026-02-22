package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// createTestBundle builds a temporary ZIP with the provided components and
// returns the file path.
func createTestBundle(t *testing.T, components map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.zip")

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

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func sampleManifest() *Manifest {
	return &Manifest{
		BundleVersion: "1.0",
		RunID:         "run-test",
		ExportedAt:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		EventCount:    2,
		RootHash:      "aabbccdd",
		Signature:     "sig-base64",
		SignerKeyID:   "key-1",
	}
}

func sampleEvents() []byte {
	e1 := Event{
		EventID:   "e1",
		RunID:     "run-test",
		SeqNo:     0,
		EventType: "run.started",
		Payload:   map[string]interface{}{"k": "v"},
		EventHash: "hash1",
	}
	e2 := Event{
		EventID:   "e2",
		RunID:     "run-test",
		SeqNo:     1,
		EventType: "step.started",
		Payload:   map[string]interface{}{"step": float64(1)},
		PrevHash:  strPtr("hash1"),
		EventHash: "hash2",
	}
	return append(mustJSON(e1), append([]byte("\n"), mustJSON(e2)...)...)
}

func samplePolicy() *Policy {
	return &Policy{
		PolicyID: "p1",
		Name:     "test-policy",
		Version:  "v1",
		Spec:     map[string]interface{}{"rules": []interface{}{}},
		SpecHash: "abcd1234",
	}
}

func sampleRun() *Run {
	return &Run{
		RunID:  "run-test",
		Status: "completed",
	}
}

func strPtr(s string) *string { return &s }

func generatePEMPublicKey(t *testing.T) []byte {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	pkix, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal pkix: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

func TestLoad_CompleteBundle(t *testing.T) {
	components := map[string][]byte{
		"manifest.json":        mustJSON(sampleManifest()),
		"events.jsonl":         sampleEvents(),
		"policy_snapshot.json": mustJSON(samplePolicy()),
		"run.json":             mustJSON(sampleRun()),
		"public_key.pem":       generatePEMPublicKey(t),
	}
	path := createTestBundle(t, components)

	b, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if b.Manifest == nil {
		t.Fatal("expected manifest")
	}
	if b.Manifest.RunID != "run-test" {
		t.Errorf("unexpected RunID: %s", b.Manifest.RunID)
	}
	if len(b.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(b.Events))
	}
	if b.Policy == nil {
		t.Fatal("expected policy")
	}
	if b.Run == nil {
		t.Fatal("expected run")
	}
	if b.PublicKey == nil {
		t.Fatal("expected public key")
	}
}

func TestLoad_BundleWithoutOptionalFiles(t *testing.T) {
	// run.json and public_key.pem are optional
	components := map[string][]byte{
		"manifest.json":        mustJSON(sampleManifest()),
		"events.jsonl":         sampleEvents(),
		"policy_snapshot.json": mustJSON(samplePolicy()),
	}
	path := createTestBundle(t, components)

	b, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if b.Run != nil {
		t.Error("expected nil Run for bundle without run.json")
	}
	if b.PublicKey != nil {
		t.Error("expected nil PublicKey for bundle without public_key.pem")
	}
}

func TestLoad_MissingManifest(t *testing.T) {
	components := map[string][]byte{
		"events.jsonl":         sampleEvents(),
		"policy_snapshot.json": mustJSON(samplePolicy()),
	}
	path := createTestBundle(t, components)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestLoad_MissingEvents(t *testing.T) {
	components := map[string][]byte{
		"manifest.json":        mustJSON(sampleManifest()),
		"policy_snapshot.json": mustJSON(samplePolicy()),
	}
	path := createTestBundle(t, components)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing events")
	}
}

func TestLoad_MissingPolicy(t *testing.T) {
	components := map[string][]byte{
		"manifest.json": mustJSON(sampleManifest()),
		"events.jsonl":  sampleEvents(),
	}
	path := createTestBundle(t, components)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing policy")
	}
}

func TestLoad_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.zip")
	if err := os.WriteFile(path, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid ZIP")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/bundle.zip")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLoad_MalformedManifestJSON(t *testing.T) {
	components := map[string][]byte{
		"manifest.json":        []byte("{invalid json"),
		"events.jsonl":         sampleEvents(),
		"policy_snapshot.json": mustJSON(samplePolicy()),
	}
	path := createTestBundle(t, components)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed manifest JSON")
	}
}

func TestLoad_MalformedEventLine(t *testing.T) {
	components := map[string][]byte{
		"manifest.json":        mustJSON(sampleManifest()),
		"events.jsonl":         []byte("{\"event_id\":\"e1\"}\n{bad json}\n"),
		"policy_snapshot.json": mustJSON(samplePolicy()),
	}
	path := createTestBundle(t, components)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed event line")
	}
}

func TestLoad_EmptyEventsLines(t *testing.T) {
	// events.jsonl with only whitespace lines → should fail (no events)
	components := map[string][]byte{
		"manifest.json":        mustJSON(sampleManifest()),
		"events.jsonl":         []byte("\n\n  \n"),
		"policy_snapshot.json": mustJSON(samplePolicy()),
	}
	path := createTestBundle(t, components)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for events file with no events")
	}
}

// ---------------------------------------------------------------------------
// parsePublicKey edge cases
// ---------------------------------------------------------------------------

func TestParsePublicKey_PKIX(t *testing.T) {
	pubPEM := generatePEMPublicKey(t)

	// Write to a temp zip just to get *zip.File
	dir := t.TempDir()
	path := filepath.Join(dir, "key.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("public_key.pem")
	w.Write(pubPEM)
	zw.Close()
	f.Close()

	reader, _ := zip.OpenReader(path)
	defer reader.Close()

	key, err := parsePublicKey(reader.File[0])
	if err != nil {
		t.Fatalf("parsePublicKey: %v", err)
	}
	if len(key) != ed25519.PublicKeySize {
		t.Errorf("expected %d byte key, got %d", ed25519.PublicKeySize, len(key))
	}
}

func TestParsePublicKey_Raw32Bytes(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	rawPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: []byte(pub), // raw 32 bytes
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "key.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("public_key.pem")
	w.Write(rawPEM)
	zw.Close()
	f.Close()

	reader, _ := zip.OpenReader(path)
	defer reader.Close()

	key, err := parsePublicKey(reader.File[0])
	if err != nil {
		t.Fatalf("parsePublicKey: %v", err)
	}
	if !bytes.Equal(key, pub) {
		t.Error("raw key mismatch")
	}
}

// ---------------------------------------------------------------------------
// Struct field coverage
// ---------------------------------------------------------------------------

func TestEventStruct(t *testing.T) {
	raw := `{"event_id":"e1","run_id":"r1","seq_no":5,"event_type":"step.ended","payload":{"k":"v"},"prev_hash":"abc","event_hash":"def"}`
	var e Event
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e.EventID != "e1" {
		t.Errorf("EventID: %s", e.EventID)
	}
	if e.SeqNo != 5 {
		t.Errorf("SeqNo: %d", e.SeqNo)
	}
	if e.PrevHash == nil || *e.PrevHash != "abc" {
		t.Errorf("PrevHash: %v", e.PrevHash)
	}
}

func TestRunCountersStruct(t *testing.T) {
	raw := `{"steps":10,"tool_calls":5,"bytes_egressed":1024,"retries":2,"blocks":1}`
	var c RunCounters
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Steps != 10 || c.ToolCalls != 5 || c.BytesEgressed != 1024 || c.Retries != 2 || c.Blocks != 1 {
		t.Errorf("unexpected counters: %+v", c)
	}
}
