package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func captureClientIP(t *testing.T, mode string, trustedProxyCount int, remoteAddr string, headers map[string]string) string {
	t.Helper()

	var got string
	h := ClientIPMiddleware(mode, trustedProxyCount)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.GetClientIP(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.RemoteAddr = remoteAddr
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func TestClientIPMiddlewareRemoteAddrIgnoresForwardedHeaders(t *testing.T) {
	got := captureClientIP(t, "remote_addr", 0, "203.0.113.10:4242", map[string]string{
		"X-Forwarded-For": "198.51.100.99",
		"X-Real-IP":       "198.51.100.88",
	})

	if got != "203.0.113.10" {
		t.Fatalf("expected TCP peer IP, got %q", got)
	}
}

func TestClientIPMiddlewareTrustedProxyIgnoresPrependedSpoof(t *testing.T) {
	got := captureClientIP(t, "xff_trusted_proxies", 1, "10.0.0.10:443", map[string]string{
		"X-Forwarded-For": "198.51.100.99, 203.0.113.44",
	})

	if got != "203.0.113.44" {
		t.Fatalf("expected client IP nearest the trusted proxy, got %q", got)
	}
}

func TestClientIPMiddlewareInvalidProxyCountFailsClosed(t *testing.T) {
	got := captureClientIP(t, "xff_trusted_proxies", 0, "203.0.113.10:4242", map[string]string{
		"X-Forwarded-For": "198.51.100.99",
	})

	if got != "203.0.113.10" {
		t.Fatalf("expected fallback to TCP peer IP, got %q", got)
	}
}

func TestClientIPMiddlewareUnknownModeFailsClosed(t *testing.T) {
	got := captureClientIP(t, "trust_everything", 99, "203.0.113.10:4242", map[string]string{
		"X-Forwarded-For": "198.51.100.99",
	})

	if got != "203.0.113.10" {
		t.Fatalf("expected fallback to TCP peer IP, got %q", got)
	}
}
