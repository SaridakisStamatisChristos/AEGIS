package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// authURLGetter abstracts the OIDC provider's GetAuthURL method.
type authURLGetter interface {
	GetAuthURL(state string) string
}

// codeExchanger abstracts the OIDC provider's ExchangeCode method.
type codeExchanger interface {
	ExchangeCode(ctx context.Context, code string) (interface{}, error)
}

// stateEntry records the time a state token was created.
type stateEntry struct {
	createdAt time.Time
}

// AuthHandler orchestrates the OAuth2 / OIDC login flow.
type AuthHandler struct {
	oidc   interface{} // *auth.OIDCProvider or *auth.MockOIDCProvider
	logger *zap.Logger

	// CSRF state cache (production systems should use Redis or a DB)
	stateMu    sync.Mutex
	stateCache map[string]stateEntry
	stateTTL   time.Duration
}

func NewAuthHandler(oidc interface{}, logger *zap.Logger) *AuthHandler {
	h := &AuthHandler{
		oidc:       oidc,
		logger:     logger,
		stateCache: make(map[string]stateEntry),
		stateTTL:   5 * time.Minute,
	}
	// Background goroutine to evict expired state entries
	go h.reapStates()
	return h
}

// Login initiates the OAuth2 authorization-code flow.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := generateState(32)
	if err != nil {
		h.logger.Error("failed to generate CSRF state", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.stateMu.Lock()
	h.stateCache[state] = stateEntry{createdAt: time.Now()}
	h.stateMu.Unlock()

	provider, ok := h.oidc.(authURLGetter)
	if !ok {
		http.Error(w, "auth provider not configured", http.StatusInternalServerError)
		return
	}

	authURL := provider.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles the OAuth2 redirect after the user authenticates.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	// Verify CSRF state token
	if !h.consumeState(state) {
		h.logger.Warn("invalid or expired state token", zap.String("state", state))
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	// Exchange authorization code for tokens
	exchanger, ok := h.oidc.(codeExchanger)
	if !ok {
		http.Error(w, "auth provider does not support code exchange", http.StatusInternalServerError)
		return
	}

	token, err := exchanger.ExchangeCode(r.Context(), code)
	if err != nil {
		h.logger.Error("code exchange failed", zap.Error(err))
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	h.logger.Info("auth callback completed",
		zap.String("state", state))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "authentication successful",
		"token":   token,
	}); err != nil {
		h.logger.Error("failed to encode auth callback response", zap.Error(err))
	}
}

// consumeState validates and removes a CSRF state token.
// Returns false if the token is missing, unknown, or expired.
func (h *AuthHandler) consumeState(state string) bool {
	if state == "" {
		return false
	}
	h.stateMu.Lock()
	defer h.stateMu.Unlock()

	entry, ok := h.stateCache[state]
	if !ok {
		return false
	}
	delete(h.stateCache, state)
	return time.Since(entry.createdAt) < h.stateTTL
}

// reapStates periodically removes expired state tokens.
func (h *AuthHandler) reapStates() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		h.stateMu.Lock()
		for k, v := range h.stateCache {
			if now.Sub(v.createdAt) >= h.stateTTL {
				delete(h.stateCache, k)
			}
		}
		h.stateMu.Unlock()
	}
}

// generateState creates a cryptographically random base64url string.
func generateState(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
