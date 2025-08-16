package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
)

// Using a package-level counter (ephemeral per cold start)
var globalCount atomic.Uint64

// For simple auth
func authorized(r *http.Request) bool {
	secret := os.Getenv("SECRET_TOKEN")
	if secret == "" { // open if not set
		return true
	}
	if r.Header.Get("X-Auth-Token") == secret {
		return true
	}
	if r.URL.Query().Get("token") == secret {
		return true
	}
	return false
}

// HitHandler increments the counter
func HitHandler(w http.ResponseWriter, r *http.Request) {
	if !authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}
	newVal := globalCount.Add(1)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"hits": newVal})
}

// Vercel entrypoint
func Hit(w http.ResponseWriter, r *http.Request) { // entrypoint for /hit
	HitHandler(w, r)
}

// Optional: seed from env on cold start
func init() {
	if seed := os.Getenv("INITIAL_HIT_COUNT"); seed != "" {
		if v, err := strconv.ParseUint(seed, 10, 64); err == nil {
			globalCount.Store(v)
		}
	}
}
