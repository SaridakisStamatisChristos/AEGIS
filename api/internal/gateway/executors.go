package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Executor executes tools
type Executor interface {
	Execute(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error)
}

// ExecutorRegistry manages tool executors
type ExecutorRegistry struct {
	mu        sync.RWMutex
	executors map[string]Executor
}

func NewExecutorRegistry() *ExecutorRegistry {
	registry := &ExecutorRegistry{
		executors: make(map[string]Executor),
	}

	// Register built-in executors
	registry.Register("builtin", NewBuiltinExecutor())
	registry.Register("http", NewHTTPExecutor())

	return registry
}

func (r *ExecutorRegistry) Register(name string, executor Executor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[name] = executor
}

func (r *ExecutorRegistry) Get(name string) Executor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.executors[name]
}

// sandboxDir returns the platform-appropriate sandbox directory.
var sandboxDir = filepath.Join(os.TempDir(), "aegisrun-sandbox")

// BuiltinExecutor executes built-in tools
type BuiltinExecutor struct {
	client *http.Client // H-03: shared client for connection pooling
}

func NewBuiltinExecutor() *BuiltinExecutor {
	return &BuiltinExecutor{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (e *BuiltinExecutor) Execute(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "http_request":
		return e.httpRequest(ctx, args)
	case "local_file_read":
		return e.localFileRead(ctx, args)
	case "shell_exec":
		return nil, fmt.Errorf("shell_exec is disabled by default")
	default:
		return nil, fmt.Errorf("unknown builtin tool: %s", toolName)
	}
}

func (e *BuiltinExecutor) httpRequest(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	url, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'url' argument")
	}

	method := "GET"
	if m, ok := args["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	var bodyReader io.Reader
	if body, ok := args["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add headers if provided
	if headers, ok := args["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Cap the amount we read to 1 MiB to prevent OOM
	const maxBody = 1 << 20
	limited := io.LimitReader(resp.Body, maxBody)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
	}, nil
}

// maxLocalFileSize limits local file reads to 10 MiB to prevent OOM (H-04).
const maxLocalFileSize = 10 << 20

func (e *BuiltinExecutor) localFileRead(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	rawPath, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'path' argument")
	}

	absPath := filepath.Join(sandboxDir, filepath.Clean(rawPath))

	// Prevent directory-traversal escapes.
	if !strings.HasPrefix(absPath, sandboxDir+string(filepath.Separator)) && absPath != sandboxDir {
		return nil, fmt.Errorf("path outside sandbox: %s", rawPath)
	}

	// M-01: Respect context cancellation.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// H-04: Check file size before reading.
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > maxLocalFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxLocalFileSize)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return map[string]interface{}{
		"path":    rawPath,
		"content": string(content),
		"size":    len(content),
	}, nil
}

// ---------- HTTPExecutor — forwards tool calls to external HTTP services ----------

// HTTPExecutor calls external tools via HTTP POST.
// The request body is a JSON object with "tool_name" and "args".
type HTTPExecutor struct {
	client *http.Client
}

func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *HTTPExecutor) Execute(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	endpoint, ok := args["endpoint"].(string)
	if !ok || endpoint == "" {
		return nil, fmt.Errorf("missing or empty 'endpoint' in args")
	}

	payload := map[string]interface{}{
		"tool_name": toolName,
		"args":      args,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Forward optional authorization header if provided
	if authHeader, ok := args["authorization"].(string); ok && authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http executor call failed: %w", err)
	}
	defer resp.Body.Close()

	const maxBody = 1 << 20 // 1 MiB
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("read executor response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("executor returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// If not valid JSON, return as string
		return map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        string(respBody),
		}, nil
	}

	return result, nil
}
