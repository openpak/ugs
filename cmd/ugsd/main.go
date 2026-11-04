// Command ugsd serves the Unity Gaming Services hostnames Gang Beasts dials, from
// OpenPak's own Postgres state. Plain HTTP on the block base (Traefik ends the guest TLS in
// front, the same arrangement as servers/t17), one handler per hostname, health on base+1.
// Nothing is ever forwarded to Unity.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"openpak/ugs/internal/serve"
	"openpak/ugs/internal/store"
)

func main() {
	addr := flag.String("addr", ":22680", "HTTP listen address (Traefik terminates guest TLS in front)")
	healthAddr := flag.String("health-addr", ":22681", "health listen address")
	flag.Parse()

	dsn := os.Getenv("UGS_DATABASE_URL")
	if dsn == "" {
		log.Fatal("UGS_DATABASE_URL is required")
	}
	ctx := context.Background()
	db, err := store.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer db.Close()

	cfg := serve.Config{RelayHost: os.Getenv("UGS_RELAY_HOST"), Region: os.Getenv("UGS_REGION")}
	if v := os.Getenv("UGS_RELAY_PORT"); v != "" {
		if cfg.RelayPort, err = strconv.Atoi(v); err != nil {
			log.Fatal("UGS_RELAY_PORT must be a number")
		}
	}
	srv, err := serve.New(ctx, db, cfg)
	if err != nil {
		log.Fatalf("serve: %v", err)
	}
	if cfg.RelayHost == "" {
		slog.Warn("relay: UGS_RELAY_HOST not set; allocations and matches are not issued")
	}
	go func() {
		for range time.Tick(time.Minute) {
			if err := db.Sweep(ctx, 600); err != nil {
				slog.Error("sweep", "err", err)
			}
		}
	}()
	go func() {
		h := http.NewServeMux()
		h.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			c, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if db.Ping(c) != nil {
				http.Error(w, `{"status":"down"}`, http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"ok"}`)
		})
		h.HandleFunc("/internal/accounts/", eraseHandler("ugs", eraseNothing))
		if err := http.ListenAndServe(*healthAddr, h); err != nil {
			log.Fatalf("health: %v", err)
		}
	}()
	log.Printf("ugsd listening %s health %s", *addr, *healthAddr)
	log.Fatalf("serve: %v", http.ListenAndServe(*addr, srv))
}
