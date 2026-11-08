package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"repair-tickets/internal/events"

	"github.com/jackc/pgx/v5"
)

func (s *ticketStore) history(w http.ResponseWriter, r *http.Request) {
	id, ok := readTicketID(w, r)
	if !ok {
		return
	}
	history, err := s.readHistory(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Read ticket %d history: %v", id, err)
		http.Error(w, "Cannot read ticket history", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func (s *ticketStore) readHistory(ctx context.Context, id int32) ([]events.StatusChanged, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tickets WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, pgx.ErrNoRows
	}

	rows, err := s.db.Query(ctx, `
		SELECT event_id, ticket_id, old_status, new_status, occurred_at
		FROM ticket_status_history WHERE ticket_id = $1 ORDER BY event_id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := make([]events.StatusChanged, 0)
	for rows.Next() {
		var event events.StatusChanged
		if err := rows.Scan(&event.EventID, &event.TicketID, &event.OldStatus, &event.NewStatus, &event.OccurredAt); err != nil {
			return nil, err
		}
		history = append(history, event)
	}
	return history, rows.Err()
}
