package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
)

var globalCount atomic.Uint64

func init() {
	if seed := os.Getenv("INITIAL_HIT_COUNT"); seed != "" {
		if v, err := strconv.ParseUint(seed, 10, 64); err == nil {
			globalCount.Store(v)
		}
	}
}

func authorize(r *http.Request) bool {
	secret := os.Getenv("SECRET_TOKEN")
	if secret == "" {
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

func Handler(w http.ResponseWriter, r *http.Request) {
	if !authorize(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}
	switch r.URL.Path {
	case "/hit":
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		newVal := globalCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": newVal})
	case "/count":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": globalCount.Load()})
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}
