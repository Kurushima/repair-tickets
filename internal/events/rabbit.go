package events

import (
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Exchange         = "tickets.events"
	Queue            = "tickets.status-history"
	StatusChangedKey = "ticket.status_changed"
)

type StatusChanged struct {
	EventID    int64     `json:"event_id"`
	TicketID   int32     `json:"ticket_id"`
	OldStatus  string    `json:"old_status"`
	NewStatus  string    `json:"new_status"`
	OccurredAt time.Time `json:"occurred_at"`
}

func BrokerURL() string {
	if address := os.Getenv("RABBITMQ_URL"); address != "" {
		return address
	}
	return "amqp://repair:repair_rabbit_local@127.0.0.1:5672/%2F"
}

func DeclareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(Queue, true, false, false, false, amqp.Table{"x-queue-type": "classic"}); err != nil {
		return err
	}
	return ch.QueueBind(Queue, StatusChangedKey, Exchange, false, nil)
}
