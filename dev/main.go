package main

import (
	"log"
	"net/http"
	"os"

	"github.com/advayc/nums/api"
)

// Dev server to exercise the serverless handler locally.
// Usage:
//
//	set -a; source .env; set +a
//	go run ./dev
//
// Then in another terminal run curl commands:
//
//	curl -H "X-Auth-Token: $SECRET_TOKEN" "http://localhost:${PORT:-8080}/hit?id=home"
//	curl -H "X-Auth-Token: $SECRET_TOKEN" "http://localhost:${PORT:-8080}/count?id=home"
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/hit", api.Handler)
	mux.HandleFunc("/count", api.Handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("dev hit counter listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
