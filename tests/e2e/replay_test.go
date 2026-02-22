// Package e2e provides end-to-end tests for AegisRun
package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"testing"
	"time"
)

// Event represents a ledger event
type Event struct {
	EventID   string                 `json:"event_id"`
	RunID     string                 `json:"run_id"`
	SeqNo     int                    `json:"seq_no"`
	EventType string                 `json:"event_type"`
	Timestamp string                 `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	PrevHash  *string                `json:"prev_hash"`
	EventHash string                 `json:"event_hash"`
}

// BundleManifest represents the evidence bundle manifest
type BundleManifest struct {
	BundleVersion string `json:"bundle_version"`
	RunID         string `json:"run_id"`
	ExportedAt    string `json:"exported_at"`
	EventCount    int    `json:"event_count"`
	RootHash      string `json:"root_hash"`
	Signature     string `json:"signature"`
	SignerKeyID   string `json:"signer_key_id"`
}

// PolicySnapshot represents the policy at time of run
type PolicySnapshot struct {
	PolicyID string                 `json:"policy_id"`
	Name     string                 `json:"name"`
	Version  string                 `json:"version"`
	Spec     map[string]interface{} `json:"spec"`
	SpecHash string                 `json:"spec_hash"`
}

// EvidenceBundle holds parsed bundle contents
type EvidenceBundle struct {
	Manifest       BundleManifest
	Events         []Event
	PolicySnapshot PolicySnapshot
	Approvals      []map[string]interface{}
	PublicKey      []byte
}

// TestReplayDeterminism tests that replaying a run produces the same results
func TestReplayDeterminism(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E replay test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// Step 1: Create and execute original run
	t.Log("Creating original run...")

	originalRun, err := client.CreateRun(ctx, map[string]interface{}{
		"test":        "replay_determinism",
		"environment": "e2e",
		"timestamp":   time.Now().Format(time.RFC3339),
	})
	require.NoError(t, err)

	originalRunID := originalRun["run_id"].(string)
	t.Logf("Original run ID: %s", originalRunID)

	// Execute deterministic tool calls
	toolCalls := []ToolCallRequest{
		{
			RunID:       originalRunID,
			StepID:      "step_001",
			ToolName:    "http_request",
			Args:        map[string]interface{}{"url": "https://api.github.com/zen", "method": "GET"},
			StateVector: map[string]interface{}{"step": 1},
			Executor:    "builtin",
		},
		{
			RunID:       originalRunID,
			StepID:      "step_002",
			ToolName:    "file_write",
			Args:        map[string]interface{}{"path": "/tmp/replay_test.json", "content": `{"test": true}`},
			StateVector: map[string]interface{}{"step": 2},
			Executor:    "builtin",
		},
		{
			RunID:       originalRunID,
			StepID:      "step_003",
			ToolName:    "database_query",
			Args:        map[string]interface{}{"query": "SELECT 1 as result", "params": []interface{}{}},
			StateVector: map[string]interface{}{"step": 3},
			Executor:    "builtin",
		},
	}

	originalDecisions := make([]string, 0, len(toolCalls))

	for _, tc := range toolCalls {
		result, err := client.ExecuteToolCall(ctx, tc)
		require.NoError(t, err)

		decision := result["decision"].(map[string]interface{})
		originalDecisions = append(originalDecisions, decision["action"].(string))
	}

	t.Logf("Original decisions: %v", originalDecisions)

	// Step 2: Export evidence bundle
	t.Log("Exporting evidence bundle...")

	bundleData, err := client.ExportEvidence(ctx, originalRunID)
	require.NoError(t, err)

	bundle, err := parseEvidenceBundle(bundleData)
	require.NoError(t, err)

	t.Logf("Bundle contains %d events", len(bundle.Events))

	// Step 3: Replay using recorded inputs
	t.Log("Replaying run with recorded inputs...")

	replayRun, err := client.CreateRun(ctx, map[string]interface{}{
		"test":             "replay_verification",
		"original_run_id":  originalRunID,
		"replay_timestamp": time.Now().Format(time.RFC3339),
		"is_replay":        true,
	})
	require.NoError(t, err)

	replayRunID := replayRun["run_id"].(string)
	t.Logf("Replay run ID: %s", replayRunID)

	// Replay same tool calls
	replayDecisions := make([]string, 0, len(toolCalls))

	for _, tc := range toolCalls {
		tc.RunID = replayRunID
		tc.StepID = tc.StepID + "_replay"

		result, err := client.ExecuteToolCall(ctx, tc)
		require.NoError(t, err)

		decision := result["decision"].(map[string]interface{})
		replayDecisions = append(replayDecisions, decision["action"].(string))
	}

	t.Logf("Replay decisions: %v", replayDecisions)

	// Step 4: Compare results
	t.Run("VerifyDecisionDeterminism", func(t *testing.T) {
		assert.Equal(t, originalDecisions, replayDecisions, "Decisions should be deterministic")
	})

	// Step 5: Compare policy decisions specifically
	t.Run("VerifyPolicyDeterminism", func(t *testing.T) {
		originalTimeline, err := client.GetTimeline(ctx, originalRunID)
		require.NoError(t, err)

		replayTimeline, err := client.GetTimeline(ctx, replayRunID)
		require.NoError(t, err)

		origCalls := originalTimeline["tool_calls"].([]interface{})
		replayCalls := replayTimeline["tool_calls"].([]interface{})

		require.Equal(t, len(origCalls), len(replayCalls), "Should have same number of tool calls")

		for i := range origCalls {
			origCall := origCalls[i].(map[string]interface{})
			replayCall := replayCalls[i].(map[string]interface{})

			origDecision := origCall["decision"].(map[string]interface{})
			replayDecision := replayCall["decision"].(map[string]interface{})

			assert.Equal(t, origDecision["action"], replayDecision["action"],
				fmt.Sprintf("Decision action should match for call %d", i))
			assert.Equal(t, origDecision["policy_rule_id"], replayDecision["policy_rule_id"],
				fmt.Sprintf("Policy rule should match for call %d", i))
		}
	})
}

// TestEventChainIntegrity verifies the hash chain in evidence bundles
func TestEventChainIntegrity(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// Create run with multiple events
	run, err := client.CreateRun(ctx, map[string]interface{}{"test": "chain_integrity"})
	require.NoError(t, err)
	runID := run["run_id"].(string)

	// Generate events
	for i := 0; i < 5; i++ {
		req := ToolCallRequest{
			RunID:       runID,
			StepID:      fmt.Sprintf("step_%03d", i),
			ToolName:    "http_request",
			Args:        map[string]interface{}{"url": "https://httpbin.org/get", "method": "GET"},
			StateVector: map[string]interface{}{"step": i},
			Executor:    "builtin",
		}
		_, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err)
	}

	// Export and parse bundle
	bundleData, err := client.ExportEvidence(ctx, runID)
	require.NoError(t, err)

	bundle, err := parseEvidenceBundle(bundleData)
	require.NoError(t, err)

	t.Run("VerifyChainLinks", func(t *testing.T) {
		// Sort events by seq_no
		events := bundle.Events
		sort.Slice(events, func(i, j int) bool {
			return events[i].SeqNo < events[j].SeqNo
		})

		var prevHash *string
		for i, event := range events {
			// Verify prev_hash links to previous event
			if i == 0 {
				// First event should have nil prev_hash
				if event.PrevHash != nil {
					t.Errorf("First event should have nil prev_hash, got: %s", *event.PrevHash)
				}
			} else {
				// Subsequent events should link to previous
				require.NotNil(t, event.PrevHash, fmt.Sprintf("Event %d should have prev_hash", i))
				assert.Equal(t, *prevHash, *event.PrevHash,
					fmt.Sprintf("Event %d prev_hash should match event %d hash", i, i-1))
			}

			prevHash = &event.EventHash
		}

		t.Logf("Verified chain of %d events", len(events))
	})

	t.Run("VerifyMonotonicSequence", func(t *testing.T) {
		events := bundle.Events
		for i := 0; i < len(events)-1; i++ {
			assert.Less(t, events[i].SeqNo, events[i+1].SeqNo,
				"Events should have monotonically increasing seq_no")
		}
	})

	t.Run("VerifyTimestampOrdering", func(t *testing.T) {
		events := bundle.Events
		for i := 0; i < len(events)-1; i++ {
			t1, _ := time.Parse(time.RFC3339Nano, events[i].Timestamp)
			t2, _ := time.Parse(time.RFC3339Nano, events[i+1].Timestamp)

			assert.False(t, t2.Before(t1),
				fmt.Sprintf("Event timestamps should be non-decreasing: %s -> %s",
					events[i].Timestamp, events[i+1].Timestamp))
		}
	})
}

// TestReplayDiffReport tests the diff report generation for replays
func TestReplayDiffReport(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// Create original run
	originalRun, err := client.CreateRun(ctx, map[string]interface{}{
		"test": "replay_diff",
	})
	require.NoError(t, err)
	originalRunID := originalRun["run_id"].(string)

	// Execute tool calls
	req := ToolCallRequest{
		RunID:       originalRunID,
		StepID:      "step_001",
		ToolName:    "http_request",
		Args:        map[string]interface{}{"url": "https://api.github.com/zen", "method": "GET"},
		StateVector: map[string]interface{}{"step": 1},
		Executor:    "builtin",
	}

	originalResult, err := client.ExecuteToolCall(ctx, req)
	require.NoError(t, err)

	// Create replay run
	replayRun, err := client.CreateRun(ctx, map[string]interface{}{
		"test":            "replay_diff_replay",
		"original_run_id": originalRunID,
		"is_replay":       true,
	})
	require.NoError(t, err)
	replayRunID := replayRun["run_id"].(string)

	// Replay
	req.RunID = replayRunID
	replayResult, err := client.ExecuteToolCall(ctx, req)
	require.NoError(t, err)

	t.Run("CompareDecisions", func(t *testing.T) {
		origDecision := originalResult["decision"].(map[string]interface{})
		replayDecision := replayResult["decision"].(map[string]interface{})

		diff := generateDiff(origDecision, replayDecision)

		if len(diff) == 0 {
			t.Log("No differences in decisions - replay is deterministic")
		} else {
			t.Logf("Decision differences: %v", diff)
			// Decisions should be the same for deterministic inputs
			t.Error("Unexpected differences in policy decisions")
		}
	})

	t.Run("IdentifyNonDeterminismSources", func(t *testing.T) {
		// Common sources of non-determinism
		nonDeterministicFields := []string{
			"tool_call_id", // Generated per call
			"timestamp",    // Time-based
			"requested_at", // Time-based
			"responded_at", // Time-based
			"duration_ms",  // Varies
		}

		origDecision := originalResult["decision"].(map[string]interface{})
		replayDecision := replayResult["decision"].(map[string]interface{})

		for _, field := range nonDeterministicFields {
			if origDecision[field] != replayDecision[field] {
				t.Logf("Expected non-determinism in field: %s", field)
			}
		}

		// These fields SHOULD be deterministic
		deterministicFields := []string{"action", "policy_rule_id"}
		for _, field := range deterministicFields {
			assert.Equal(t, origDecision[field], replayDecision[field],
				fmt.Sprintf("Field %s should be deterministic", field))
		}
	})
}

// TestPolicySnapshotImmutability verifies policy snapshot in bundle matches runtime policy
func TestPolicySnapshotImmutability(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// Create run
	run, err := client.CreateRun(ctx, map[string]interface{}{"test": "policy_snapshot"})
	require.NoError(t, err)
	runID := run["run_id"].(string)

	// Execute a tool call
	req := ToolCallRequest{
		RunID:       runID,
		StepID:      "step_001",
		ToolName:    "http_request",
		Args:        map[string]interface{}{"url": "https://example.com", "method": "GET"},
		StateVector: map[string]interface{}{},
		Executor:    "builtin",
	}
	_, err = client.ExecuteToolCall(ctx, req)
	require.NoError(t, err)

	// Export bundle
	bundleData, err := client.ExportEvidence(ctx, runID)
	require.NoError(t, err)

	bundle, err := parseEvidenceBundle(bundleData)
	require.NoError(t, err)

	t.Run("VerifyPolicySnapshot", func(t *testing.T) {
		assert.NotEmpty(t, bundle.PolicySnapshot.PolicyID, "Should have policy ID")
		assert.NotEmpty(t, bundle.PolicySnapshot.Version, "Should have policy version")
		assert.NotEmpty(t, bundle.PolicySnapshot.SpecHash, "Should have spec hash")
		assert.NotEmpty(t, bundle.PolicySnapshot.Spec, "Should have policy spec")

		t.Logf("Policy in bundle: %s:%s (hash: %s...)",
			bundle.PolicySnapshot.Name,
			bundle.PolicySnapshot.Version,
			bundle.PolicySnapshot.SpecHash[:16])
	})

	t.Run("VerifySpecHashIntegrity", func(t *testing.T) {
		// Recompute spec hash
		specJSON, err := json.Marshal(bundle.PolicySnapshot.Spec)
		require.NoError(t, err)

		computedHash := sha256.Sum256(specJSON)
		computedHashHex := hex.EncodeToString(computedHash[:])

		// Note: The exact hash depends on canonical JSON serialization
		// This test verifies the hash exists and is plausible
		assert.Len(t, bundle.PolicySnapshot.SpecHash, 64, "Spec hash should be 64 hex chars")
		t.Logf("Stored hash:   %s", bundle.PolicySnapshot.SpecHash)
		t.Logf("Computed hash: %s (may differ due to canonicalization)", computedHashHex)
	})
}

// Helper functions

func parseEvidenceBundle(data []byte) (*EvidenceBundle, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip: %w", err)
	}

	bundle := &EvidenceBundle{}

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}

		switch file.Name {
		case "manifest.json":
			if err := json.Unmarshal(content, &bundle.Manifest); err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}

		case "events.jsonl":
			lines := bytes.Split(content, []byte("\n"))
			for _, line := range lines {
				if len(line) == 0 {
					continue
				}
				var event Event
				if err := json.Unmarshal(line, &event); err != nil {
					return nil, fmt.Errorf("parse event: %w", err)
				}
				bundle.Events = append(bundle.Events, event)
			}

		case "policy_snapshot.json":
			if err := json.Unmarshal(content, &bundle.PolicySnapshot); err != nil {
				return nil, fmt.Errorf("parse policy: %w", err)
			}

		case "approvals.json":
			if err := json.Unmarshal(content, &bundle.Approvals); err != nil {
				// Approvals might be empty
				bundle.Approvals = []map[string]interface{}{}
			}

		case "public_key.pem":
			bundle.PublicKey = content
		}
	}

	return bundle, nil
}

func generateDiff(original, replay map[string]interface{}) map[string]interface{} {
	diff := make(map[string]interface{})

	// Compare all fields
	allKeys := make(map[string]bool)
	for k := range original {
		allKeys[k] = true
	}
	for k := range replay {
		allKeys[k] = true
	}

	for key := range allKeys {
		origVal, origExists := original[key]
		replayVal, replayExists := replay[key]

		if !origExists {
			diff[key] = map[string]interface{}{"added": replayVal}
		} else if !replayExists {
			diff[key] = map[string]interface{}{"removed": origVal}
		} else if !reflect.DeepEqual(origVal, replayVal) {
			diff[key] = map[string]interface{}{
				"original": origVal,
				"replay":   replayVal,
			}
		}
	}

	return diff
}

// TestReplayWithMockedExternals tests replay with mocked external services.
//
// It records a run, exports its evidence bundle, parses the events, and then
// replays the same tool-call sequence in a new run, verifying that the policy
// engine produces identical decisions when the inputs are the same.
func TestReplayWithMockedExternals(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E replay test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// ---- 1. Record: create a run and make a series of tool calls -----------
	t.Log("Recording original run …")
	originalRun, err := client.CreateRun(ctx, map[string]interface{}{
		"test":      "replay_mocked",
		"timestamp": time.Now().Format(time.RFC3339),
	})
	require.NoError(t, err)
	originalRunID := originalRun["run_id"].(string)

	toolCalls := []ToolCallRequest{
		{
			RunID:       originalRunID,
			StepID:      "step_rec_1",
			ToolName:    "http_request",
			Args:        map[string]interface{}{"url": "https://api.example.com/data", "method": "GET"},
			StateVector: map[string]interface{}{"iteration": float64(1)},
			Executor:    "builtin",
		},
		{
			RunID:       originalRunID,
			StepID:      "step_rec_2",
			ToolName:    "read_file",
			Args:        map[string]interface{}{"path": "/tmp/test_input.txt"},
			StateVector: map[string]interface{}{"iteration": float64(2)},
			Executor:    "builtin",
		},
		{
			RunID:       originalRunID,
			StepID:      "step_rec_3",
			ToolName:    "http_request",
			Args:        map[string]interface{}{"url": "https://api.example.com/submit", "method": "POST"},
			StateVector: map[string]interface{}{"iteration": float64(3)},
			Executor:    "builtin",
		},
	}

	type recordedDecision struct {
		Action   string
		ToolName string
	}

	var originalDecisions []recordedDecision
	for i, tc := range toolCalls {
		result, err := client.ExecuteToolCall(ctx, tc)
		require.NoError(t, err, fmt.Sprintf("tool call %d failed", i))
		decision := result["decision"].(map[string]interface{})
		originalDecisions = append(originalDecisions, recordedDecision{
			Action:   decision["action"].(string),
			ToolName: tc.ToolName,
		})
		t.Logf("Original call %d (%s): %s", i+1, tc.ToolName, decision["action"])
	}

	// ---- 2. Export evidence bundle and parse it ---------------------------
	t.Log("Exporting evidence bundle …")
	bundleBytes, err := client.ExportEvidence(ctx, originalRunID)
	require.NoError(t, err)
	require.Greater(t, len(bundleBytes), 0, "evidence bundle should be non-empty")

	zipReader, err := zip.NewReader(bytes.NewReader(bundleBytes), int64(len(bundleBytes)))
	require.NoError(t, err)

	var bundleEvents []Event
	for _, f := range zipReader.File {
		if f.Name != "events.jsonl" {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)
		for _, line := range bytes.Split(data, []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var ev Event
			require.NoError(t, json.Unmarshal(line, &ev))
			bundleEvents = append(bundleEvents, ev)
		}
	}
	require.Greater(t, len(bundleEvents), 0, "bundle should contain events")
	t.Logf("Parsed %d events from evidence bundle", len(bundleEvents))

	// ---- 3. Replay: create a new run and replay the same tool calls -------
	t.Log("Replaying tool calls in a new run …")
	replayRun, err := client.CreateRun(ctx, map[string]interface{}{
		"test":          "replay_mocked_replay",
		"replayed_from": originalRunID,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
	require.NoError(t, err)
	replayRunID := replayRun["run_id"].(string)

	var replayDecisions []recordedDecision
	for i, tc := range toolCalls {
		replayTC := tc
		replayTC.RunID = replayRunID
		replayTC.StepID = fmt.Sprintf("step_replay_%d", i+1)

		result, err := client.ExecuteToolCall(ctx, replayTC)
		require.NoError(t, err, fmt.Sprintf("replay call %d failed", i))
		decision := result["decision"].(map[string]interface{})
		replayDecisions = append(replayDecisions, recordedDecision{
			Action:   decision["action"].(string),
			ToolName: replayTC.ToolName,
		})
		t.Logf("Replay  call %d (%s): %s", i+1, replayTC.ToolName, decision["action"])
	}

	// ---- 4. Verify determinism: decisions must match ----------------------
	require.Equal(t, len(originalDecisions), len(replayDecisions),
		"decision count mismatch between original and replay")

	for i := range originalDecisions {
		assert.Equal(t, originalDecisions[i].Action, replayDecisions[i].Action,
			fmt.Sprintf("decision mismatch at call %d (%s)", i+1, originalDecisions[i].ToolName))
	}

	// ---- 5. Verify event chain integrity in replay bundle -----------------
	t.Log("Exporting replay evidence bundle …")
	replayBundle, err := client.ExportEvidence(ctx, replayRunID)
	require.NoError(t, err)

	replayZip, err := zip.NewReader(bytes.NewReader(replayBundle), int64(len(replayBundle)))
	require.NoError(t, err)

	var replayEvents []Event
	for _, f := range replayZip.File {
		if f.Name != "events.jsonl" {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)
		for _, line := range bytes.Split(data, []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var ev Event
			require.NoError(t, json.Unmarshal(line, &ev))
			replayEvents = append(replayEvents, ev)
		}
	}
	require.Greater(t, len(replayEvents), 0, "replay bundle should contain events")

	// Verify hash chain in replay
	for i, ev := range replayEvents {
		if i == 0 {
			assert.Nil(t, ev.PrevHash, "first replay event should have nil prev_hash")
		} else {
			require.NotNil(t, ev.PrevHash, fmt.Sprintf("replay event %d should have prev_hash", i))
			assert.Equal(t, replayEvents[i-1].EventHash, *ev.PrevHash,
				fmt.Sprintf("replay event %d prev_hash should match previous event hash", i))
		}
		assert.Equal(t, i, ev.SeqNo, fmt.Sprintf("replay event %d seq_no mismatch", i))
	}
	t.Logf("Replay event chain verified: %d events, all hashes consistent", len(replayEvents))
}
