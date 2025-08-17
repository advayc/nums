package api

import (
	"context"
	"encoding/json"
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "hits": val, "source": func() string {
			if getRedis() != nil {
				return "redis"
			}
			return "memory"
		}()})
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}
