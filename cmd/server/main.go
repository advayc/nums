package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/cors"
)

// HitCounter holds an atomic counter for visits
// It can be extended later to persist to a database or redis.
type HitCounter struct {
	count uint64
}

func (h *HitCounter) Inc() uint64 {
	return atomic.AddUint64(&h.count, 1)
}

func (h *HitCounter) Get() uint64 {
	return atomic.LoadUint64(&h.count)
}

// JSON response helpers
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	port := getenv("PORT", "8080")
	secretToken := os.Getenv("SECRET_TOKEN") // if set, required via header X-Auth-Token or query param token
	persistFile := os.Getenv("PERSIST_FILE") // if set, counter value persisted to this file
	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")

	counter := &HitCounter{}

	// Load persisted value if configured
	if persistFile != "" {
		if v, err := loadCountFromFile(persistFile); err != nil {
			log.Printf("(warn) could not load persisted count: %v", err)
		} else {
			atomic.StoreUint64(&counter.count, v)
			log.Printf("loaded count=%d from %s", v, persistFile)
		}
	}

	mux := http.NewServeMux()

	// POST /hit (or GET) increments the counter and returns the new value
	mux.HandleFunc("/hit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, POST")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if !authorize(secretToken, r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		newVal := counter.Inc()
		if persistFile != "" {
			if err := saveCountToFile(persistFile, newVal); err != nil {
				log.Printf("(warn) persist failed: %v", err)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"hits": newVal})
	})

	// GET /count just returns current value without incrementing
	mux.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if !authorize(secretToken, r) { // protect count too (optional)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"hits": counter.Get()})
	})

	// Simple health endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Determine allowed origins
	var allowedOrigins []string
	if allowedOriginsEnv == "" {
		allowedOrigins = []string{"*"}
	} else {
		parts := strings.Split(allowedOriginsEnv, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				allowedOrigins = append(allowedOrigins, p)
			}
		}
	}

	// Middleware chain: CORS + basic security headers
	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		MaxAge:           300,
	})

	baseHandler := c.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers (lightweight)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	}))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           requestLogger(baseHandler),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("hit counter server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("bye")
}

// requestLogger logs minimal info about each request
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(lrw, r)
		dur := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lrw.status, dur)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.status = code
	l.ResponseWriter.WriteHeader(code)
}

// getenv returns env var or fallback
func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

// authorize checks secret token if configured. If secretToken is empty, always true.
func authorize(secretToken string, r *http.Request) bool {
	if secretToken == "" {
		return true
	}
	// Header first
	if h := r.Header.Get("X-Auth-Token"); h != "" && h == secretToken {
		return true
	}
	// Query param fallback (?token=...)
	if q := r.URL.Query().Get("token"); q != "" && q == secretToken {
		return true
	}
	return false
}

// loadCountFromFile reads a uint64 from a file.
func loadCountFromFile(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, nil
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse persisted count: %w", err)
	}
	return v, nil
}

// saveCountToFile writes the count to file atomically (best-effort).
func saveCountToFile(path string, val uint64) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatUint(val, 10)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Example of how you might parse a seed initial value from env
func init() {
	if seedStr := os.Getenv("INITIAL_HIT_COUNT"); seedStr != "" {
		if seed, err := strconv.ParseUint(seedStr, 10, 64); err == nil {
			// Use unsafe pointer; easier: just store globally and set after main constructs counter
			// Simplicity: let main ignore seed; advanced: redesign HitCounter to accept seed.
			log.Printf("(info) seed %d provided but not applied (extend HitCounter for persistence)", seed)
		}
	}
}
