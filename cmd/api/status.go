package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type updateStatusRequest struct {
	Status string `json:"status"`
}

func (s *ticketStore) updateStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := readTicketID(w, r)
	if !ok {
		return
	}
	var input updateStatusRequest
	if !readJSON(w, r, &input) {
		return
	}

	var previous string
	switch input.Status {
	case "in_progress":
		previous = "new"
	case "closed":
		previous = "in_progress"
	default:
		http.Error(w, "Status must be in_progress or closed", http.StatusBadRequest)
		return
	}

	var updated ticket
	err := s.db.QueryRow(r.Context(), `
		UPDATE tickets SET status = $1
		WHERE id = $2 AND status = $3
		RETURNING id, description, status
	`, input.Status, id, previous).Scan(&updated.ID, &updated.Description, &updated.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		err = s.db.QueryRow(r.Context(), `
			SELECT EXISTS (SELECT 1 FROM tickets WHERE id = $1)
		`, id).Scan(&exists)
		if err == nil {
			if !exists {
				http.Error(w, "Ticket not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Invalid status transition", http.StatusConflict)
			return
		}
	}
	if err != nil {
		log.Printf("Update ticket %d status: %v", id, err)
		http.Error(w, "Cannot update ticket status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}
