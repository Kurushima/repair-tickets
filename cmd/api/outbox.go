package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"repair-tickets/internal/events"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

func publishOutbox(ctx context.Context, db *pgxpool.Pool, brokerURL string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := publishNextEvent(attempt, db, brokerURL)
			cancel()
			if err != nil {
				log.Printf("Outbox publish: %v", err)
			}
		}
	}
}

func publishNextEvent(ctx context.Context, db *pgxpool.Pool, brokerURL string) error {
	var event events.StatusChanged
	err := db.QueryRow(ctx, `
		SELECT id, ticket_id, old_status, new_status, occurred_at
		FROM ticket_outbox ORDER BY id LIMIT 1
	`).Scan(&event.EventID, &event.TicketID, &event.OldStatus, &event.NewStatus, &event.OccurredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := publishStatusChanged(ctx, brokerURL, event); err != nil {
		return fmt.Errorf("event %d: %w", event.EventID, err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM ticket_outbox WHERE id = $1`, event.EventID); err != nil {
		return err
	}
	log.Printf("Published ticket.status_changed event=%d ticket=%d", event.EventID, event.TicketID)
	return nil
}

func publishStatusChanged(ctx context.Context, brokerURL string, event events.StatusChanged) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
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
	if err := ch.Confirm(false); err != nil {
		return err
	}
	returned := ch.NotifyReturn(make(chan amqp.Return, 1))
	confirmation, err := ch.PublishWithDeferredConfirmWithContext(ctx, events.Exchange, events.StatusChangedKey, true, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    strconv.FormatInt(event.EventID, 10),
		Body:         body,
	})
	if err != nil {
		return err
	}
	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !acked {
		return errors.New("RabbitMQ did not confirm the message")
	}
	select {
	case reply, ok := <-returned:
		if ok {
			return fmt.Errorf("RabbitMQ returned the message: %s", reply.ReplyText)
		}
	default:
	}
	return nil
}
