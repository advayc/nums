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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/cors"
)

// HitCounter holds an atomic counter for visits
// It can be extended later to persist to a database or redis.
// SingleCounter retained for backwards compatibility (default id)
type HitCounter struct{ count uint64 }

func (h *HitCounter) Inc() uint64 { return atomic.AddUint64(&h.count, 1) }
func (h *HitCounter) Get() uint64 { return atomic.LoadUint64(&h.count) }

// MultiCounter manages counts per id (e.g., per link)
type MultiCounter struct {
	mu sync.RWMutex
	m  map[string]*uint64
}

func NewMultiCounter() *MultiCounter { return &MultiCounter{m: make(map[string]*uint64)} }

func (mc *MultiCounter) Inc(id string) uint64 {
	if id == "" {
		id = "default"
	}
	mc.mu.RLock()
	ptr, ok := mc.m[id]
	mc.mu.RUnlock()
	if !ok {
		mc.mu.Lock()
		if ptr, ok = mc.m[id]; !ok {
			var v uint64
			ptr = &v
			mc.m[id] = ptr
		}
		mc.mu.Unlock()
	}
	return atomic.AddUint64(ptr, 1)
}

func (mc *MultiCounter) Get(id string) uint64 {
	if id == "" {
		id = "default"
	}
	mc.mu.RLock()
	ptr := mc.m[id]
	mc.mu.RUnlock()
	if ptr == nil {
		return 0
	}
	return atomic.LoadUint64(ptr)
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

	singleCounter := &HitCounter{}
	multi := NewMultiCounter()

	// Load persisted value if configured
	if persistFile != "" {
		if v, err := loadCountFromFile(persistFile); err != nil {
			log.Printf("(warn) could not load persisted count: %v", err)
		} else {
			atomic.StoreUint64(&singleCounter.count, v)
			log.Printf("loaded count=%d from %s", v, persistFile)
		}
	}

	mux := http.NewServeMux()

	// POST /hit (or GET) increments the counter for given id and returns the new value
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
		id := r.URL.Query().Get("id")
		var newVal uint64
		if id == "" { // legacy single counter path
			newVal = singleCounter.Inc()
			if persistFile != "" {
				if err := saveCountToFile(persistFile, newVal); err != nil {
					log.Printf("(warn) persist failed: %v", err)
				}
			}
		} else {
			newVal = multi.Inc(id)
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "hits": newVal})
	})

	// GET /count just returns current value without incrementing
	mux.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if !authorize(secretToken, r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		id := r.URL.Query().Get("id")
		var val uint64
		if id == "" {
			val = singleCounter.Get()
		} else {
			val = multi.Get(id)
		}
		// Support plain text output via format=txt
		if f := r.URL.Query().Get("format"); f == "txt" || f == "text" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(strconv.FormatUint(val, 10)))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "hits": val})
	})

	// GET /count.txt returns just the numeric count (no JSON) for easy custom badges
	mux.HandleFunc("/count.txt", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !authorize(secretToken, r) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		id := r.URL.Query().Get("id")
		var val uint64
		if id == "" {
			val = singleCounter.Get()
		} else {
			val = multi.Get(id)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(strconv.FormatUint(val, 10)))
	})

	// GET /badge produces an SVG badge for the given id (no increment)
	mux.HandleFunc("/badge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !authorize(secretToken, r) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			id = "default"
		}
		count := multi.Get(id)
		if id == "default" {
			count = singleCounter.Get()
		}
		label := r.URL.Query().Get("label")
		if label == "" {
			label = "hits"
		}
		color := r.URL.Query().Get("color")
		if color == "" {
			color = "blue"
		}
		style := r.URL.Query().Get("style") // reserved for future (e.g., flat, flat-square)
		_ = style
		svg := buildBadgeSVG(label, count, color)
		w.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(svg))
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

// buildBadgeSVG generates a minimal static-width SVG badge (simple style)
func buildBadgeSVG(label string, count uint64, color string) string {
	// Basic size heuristics
	textVal := strconv.FormatUint(count, 10)
	labelWidth := 6*len(label) + 10
	valWidth := 6*len(textVal) + 10
	total := labelWidth + valWidth
	// Very lightweight; no external fonts
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">
<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>
<rect rx="3" width="%d" height="20" fill="#555"/>
<rect rx="3" x="%d" width="%d" height="20" fill="%s"/>
<rect rx="3" width="%d" height="20" fill="url(#s)"/>
<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">
<text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
<text x="%d" y="15">%s</text>
<text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
<text x="%d" y="15">%s</text>
</g>
</svg>`,
		total, label, textVal,
		total, labelWidth, valWidth, color,
		total,
		labelWidth/2, label,
		labelWidth/2, label,
		labelWidth+valWidth/2, textVal,
		labelWidth+valWidth/2, textVal,
	)
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
