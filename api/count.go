package api

import (
	"encoding/json"
	"net/http"
	"os"
)

// CountHandler returns current value
func CountHandler(w http.ResponseWriter, r *http.Request) {
	if !authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"hits": globalCount.Load()})
}

func Count(w http.ResponseWriter, r *http.Request) { // entrypoint for /count
	CountHandler(w, r)
}

func init() {
	// no-op but kept symmetrical to hit.go
	_ = os.Getenv("SECRET_TOKEN")
}
