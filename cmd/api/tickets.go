package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
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
	mu      sync.Mutex
	tickets []ticket
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

	s.mu.Lock()
	created := ticket{ID: len(s.tickets) + 1, Description: description, Status: "new"}
	s.tickets = append(s.tickets, created)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}
