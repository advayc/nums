package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// In-memory fallback (used only if Redis not configured or errors)
var globalCount atomic.Uint64

// Redis client (lazy init)
var (
	redisOnce   sync.Once
	redisClient *redis.Client
)

// buildUpstashRedisURL normalizes a host/password combo (optional helper for Upstash env vars)
func buildUpstashRedisURL(rawHost, password string) string {
	if rawHost == "" || password == "" {
		return ""
	}
	if !strings.HasPrefix(rawHost, "redis://") && !strings.HasPrefix(rawHost, "rediss://") {
		rawHost = "rediss://" + rawHost
	}
	u, err := url.Parse(rawHost)
	if err != nil {
		return ""
	}
	if u.User == nil {
		u.User = url.UserPassword("default", password)
	}
	return u.String()
}

func getRedis() *redis.Client {
	redisOnce.Do(func() {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" { // attempt construction from Upstash REST style vars (host + password)
			host := os.Getenv("UPSTASH_REDIS_URL")      // may be host:port or rediss://...
			pass := os.Getenv("UPSTASH_REDIS_PASSWORD") // password only (NOT the REST token)
			if host != "" && pass != "" {
				redisURL = buildUpstashRedisURL(host, pass)
			}
		}
		if redisURL == "" {
			return
		}
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("(warn) parse redis url failed: %v", err)
			return
		}
		c := redis.NewClient(opt)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := c.Ping(ctx).Err(); err != nil {
			log.Printf("(warn) redis ping failed: %v", err)
			return
		}
		redisClient = c
		log.Printf("redis enabled (addr=%s)", c.Options().Addr)
	})
	return redisClient
}

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
	switch r.URL.Path {
	case "/hit":
		// Only the mutating endpoint (/hit) is protected by auth so badges/counts can be public.
		if !authorize(r) {
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
		var newVal uint64
		id := r.URL.Query().Get("id")
		if id == "" {
			id = "home" // default page id
		}
		// Prefer Redis if configured
		if rc := getRedis(); rc != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer cancel()
			v, err := rc.Incr(ctx, "hits:"+id).Result()
			if err == nil {
				newVal = uint64(v)
			} else {
				log.Printf("(warn) redis INCR failed (falling back to memory): %v", err)
			}
		}
		if newVal == 0 { // fallback path
			newVal = globalCount.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "hits": newVal, "source": func() string {
			if getRedis() != nil {
				return "redis"
			}
			return "memory"
		}()})
	case "/count":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			id = "home"
		}
		var val uint64
		if rc := getRedis(); rc != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer cancel()
			s, err := rc.Get(ctx, "hits:"+id).Result()
			if err == nil {
				if parsed, perr := strconv.ParseUint(s, 10, 64); perr == nil {
					val = parsed
				}
			} else if err != redis.Nil {
				log.Printf("(warn) redis GET failed: %v", err)
			}
		}
		if val == 0 { // fallback memory value (not id-specific; legacy behavior)
			val = globalCount.Load()
		}
		// optional plain text via format=txt
		if f := r.URL.Query().Get("format"); f == "txt" || f == "text" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(strconv.FormatUint(val, 10)))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "hits": val, "source": func() string {
			if getRedis() != nil {
				return "redis"
			}
			return "memory"
		}()})
	case "/count.txt":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			id = "home"
		}
		var val uint64
		if rc := getRedis(); rc != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer cancel()
			if s, err := rc.Get(ctx, "hits:"+id).Result(); err == nil {
				if parsed, perr := strconv.ParseUint(s, 10, 64); perr == nil {
					val = parsed
				}
			}
		}
		if val == 0 {
			val = globalCount.Load()
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(strconv.FormatUint(val, 10)))
	case "/badge":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			id = "home"
		}
		var val uint64
		if rc := getRedis(); rc != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer cancel()
			if s, err := rc.Get(ctx, "hits:"+id).Result(); err == nil {
				if parsed, perr := strconv.ParseUint(s, 10, 64); perr == nil {
					val = parsed
				}
			}
		}
		if val == 0 {
			val = globalCount.Load()
		}
		label := r.URL.Query().Get("label")
		if label == "" {
			label = "views"
		}
		style := r.URL.Query().Get("style")
		if style == "terminal" || style == "mono" { // custom terminal style
			bg := normalizeColor(r.URL.Query().Get("bg"), "#1e1e1e")
			labelColor := normalizeColor(r.URL.Query().Get("labelColor"), "#aaa")
			valueColor := normalizeColor(r.URL.Query().Get("valueColor"), "#3cffb3")
			font := r.URL.Query().Get("font")
			if font == "" {
				font = "SFMono-Regular, SF Mono, Menlo, ui-monospace, monospace"
			}
			svg := buildTerminalBadge(label, val, font, bg, labelColor, valueColor)
			w.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write([]byte(svg))
			return
		}
		color := r.URL.Query().Get("color")
		if color == "" {
			color = "blue"
		}
		font := r.URL.Query().Get("font")
		if font == "" {
			font = "Verdana,Geneva,DejaVu Sans,sans-serif"
		}
		svg := buildBadgeSVG(label, val, color, font)
		w.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(svg))
		return

	case "/badge.json":
		// JSON schema for Shields.io endpoint badge proxy
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			id = "home"
		}
		var val uint64
		if rc := getRedis(); rc != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer cancel()
			if s, err := rc.Get(ctx, "hits:"+id).Result(); err == nil {
				if parsed, perr := strconv.ParseUint(s, 10, 64); perr == nil {
					val = parsed
				}
			}
		}
		if val == 0 {
			val = globalCount.Load()
		}
		label := r.URL.Query().Get("label")
		if label == "" {
			label = "views"
		}
		color := r.URL.Query().Get("color")
		if color == "" {
			color = "blue"
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schemaVersion": 1,
			"label":         label,
			"message":       strconv.FormatUint(val, 10),
			"color":         color,
		})
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}

// normalizeColor restricts colors to safe values (basic allowlist)
func normalizeColor(c string, fallback string) string {
	if c == "" {
		return fallback
	}
	c = strings.TrimSpace(c)
	lc := strings.ToLower(c)
	if strings.HasPrefix(lc, "#") {
		if len(lc) == 4 || len(lc) == 7 { // #rgb or #rrggbb
			return lc
		}
		return fallback
	}
	switch lc {
	case "blue", "green", "red", "orange", "yellow", "gray", "grey", "purple", "teal":
		return lc
	}
	return fallback
}

// buildBadgeSVG creates a small classic style badge, allowing a custom font
func buildBadgeSVG(label string, count uint64, color string, font string) string {
	textVal := strconv.FormatUint(count, 10)
	labelWidth := 6*len(label) + 10
	valWidth := 6*len(textVal) + 10
	total := labelWidth + valWidth
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">
<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>
<rect rx="3" width="%d" height="20" fill="#555"/>
<rect rx="3" x="%d" width="%d" height="20" fill="%s"/>
<rect rx="3" width="%d" height="20" fill="url(#s)"/>
<g fill="#fff" text-anchor="middle" font-family="%s" font-size="11">
<text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
<text x="%d" y="15">%s</text>
<text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
<text x="%d" y="15">%s</text>
</g>
</svg>`,
		total, label, textVal,
		total, labelWidth, valWidth, color,
		total, font,
		labelWidth/2, label,
		labelWidth/2, label,
		labelWidth+valWidth/2, textVal,
		labelWidth+valWidth/2, textVal,
	)
}

// buildTerminalBadge outputs a terminal-like monospace badge with label:value styling
func buildTerminalBadge(label string, count uint64, font, bg, labelColor, valueColor string) string {
	textVal := strconv.FormatUint(count, 10)
	labelText := label + ":"
	// approximate monospace width ~8px per char + padding
	labelWidth := 8*len(labelText) + 14
	valWidth := 8*len(textVal) + 14
	total := labelWidth + valWidth
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="24" role="img" aria-label="%s: %s">
<rect rx="4" width="%d" height="24" fill="%s" />
<text x="%d" y="16" font-family="%s" font-size="12" fill="%s">%s</text>
<text x="%d" y="16" font-family="%s" font-size="12" font-weight="600" fill="%s">%s</text>
</svg>`,
		total, label, textVal,
		total, bg,
		8, font, labelColor, labelText,
		labelWidth, font, valueColor, textVal,
	)
}
