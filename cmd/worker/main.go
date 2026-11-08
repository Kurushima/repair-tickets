package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"repair-tickets/internal/events"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Configure PostgreSQL: %v", err)
	}
	defer db.Close()

	brokerURL := events.BrokerURL()
	for ctx.Err() == nil {
		if err := consume(ctx, db, brokerURL); err != nil && ctx.Err() == nil {
			log.Printf("History worker will retry: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func consume(ctx context.Context, db *pgxpool.Pool, brokerURL string) error {
	conn, err := amqp.DialConfig(brokerURL, amqp.Config{Dial: amqp.DefaultDial(3 * time.Second)})
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err := events.DeclareTopology(ch); err != nil {
		return err
	}
	if err := ch.Qos(1, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, events.Queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	log.Printf("Listening for ticket history events on %s", events.Queue)
	for delivery := range deliveries {
		if err := saveDelivery(ctx, db, delivery); err != nil {
			return err
		}
	}
	return errors.New("RabbitMQ delivery stream closed")
}

func saveDelivery(ctx context.Context, db *pgxpool.Pool, delivery amqp.Delivery) error {
	var event events.StatusChanged
	if err := json.Unmarshal(delivery.Body, &event); err != nil ||
		delivery.RoutingKey != events.StatusChangedKey || event.EventID < 1 || event.TicketID < 1 ||
		event.OldStatus == "" || event.NewStatus == "" || event.OccurredAt.IsZero() {
		log.Printf("Rejected invalid ticket history event")
		return delivery.Reject(false)
	}

	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := db.Exec(writeCtx, `
		INSERT INTO ticket_status_history (event_id, ticket_id, old_status, new_status, occurred_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (event_id) DO NOTHING
	`, event.EventID, event.TicketID, event.OldStatus, event.NewStatus, event.OccurredAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			log.Printf("Rejected history event=%d for missing ticket=%d", event.EventID, event.TicketID)
			return delivery.Reject(false)
		}
		return errors.Join(err, delivery.Nack(false, true))
	}
	if err := delivery.Ack(false); err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		log.Printf("Ignored duplicate history event=%d ticket=%d", event.EventID, event.TicketID)
	} else {
		log.Printf("Saved history event=%d ticket=%d %s -> %s", event.EventID, event.TicketID, event.OldStatus, event.NewStatus)
	}
	return nil
}
