package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type createTicketRequest struct {
	Description string `json:"description"`
}

type ticket struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type ticketStore struct {
	db *pgxpool.Pool
}

func (s *ticketStore) create(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}

	var input createTicketRequest
	if err := json.Unmarshal(body, &input); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	description := strings.TrimSpace(input.Description)
	if description == "" {
		http.Error(w, "Description is required", http.StatusBadRequest)
		return
	}

	var created ticket
	err = s.db.QueryRow(r.Context(), `
		INSERT INTO tickets (description)
		VALUES ($1)
		RETURNING id, description, status
	`, description).Scan(&created.ID, &created.Description, &created.Status)
	if err != nil {
		log.Printf("Create ticket: %v", err)
		http.Error(w, "Cannot create ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *ticketStore) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil || id < 1 {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	var found ticket
	err = s.db.QueryRow(r.Context(), `
		SELECT id, description, status FROM tickets WHERE id = $1
	`, int32(id)).Scan(&found.ID, &found.Description, &found.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Read ticket %d: %v", id, err)
		http.Error(w, "Cannot read ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(found)
}
