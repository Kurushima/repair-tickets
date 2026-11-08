package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"repair-tickets/internal/events"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Configure PostgreSQL: %v", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("Connect to PostgreSQL: %v", err)
	}

	cacheAddr := os.Getenv("KEYDB_ADDR")
	if cacheAddr == "" {
		cacheAddr = "127.0.0.1:16379"
	}
	cache := redis.NewClient(&redis.Options{
		Addr:            cacheAddr,
		Protocol:        2,
		DisableIdentity: true,
		DialTimeout:     500 * time.Millisecond,
		ReadTimeout:     500 * time.Millisecond,
		WriteTimeout:    500 * time.Millisecond,
		MaxRetries:      -1,
		DialerRetries:   1,
	})
	defer cache.Close()
	outboxCtx, stopOutbox := context.WithCancel(context.Background())
	defer stopOutbox()
	go publishOutbox(outboxCtx, db, events.BrokerURL())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	tickets := &ticketStore{db: db, cache: cache}
	mux.HandleFunc("POST /tickets", tickets.create)
	mux.HandleFunc("GET /tickets/{id}", tickets.get)
	mux.HandleFunc("PATCH /tickets/{id}/status", tickets.updateStatus)

	log.Printf("Starting HTTP server on http://%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}
