package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aegisrun/aegisrun/internal/store"
)

// isNotFound checks whether an error is a store-layer "not found" sentinel.
func isNotFound(err error) bool {
	return err != nil && errors.Is(err, store.ErrNotFound)
}

// writeJSON encodes v as JSON and writes it with the given HTTP status.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
