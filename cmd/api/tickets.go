package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
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

func (s *ticketStore) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		http.Error(w, "Invalid ticket ID", http.StatusBadRequest)
		return
	}

	found, ok := s.find(id)
	if !ok {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(found)
}

func (s *ticketStore) find(id int) (ticket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id < 1 || id > len(s.tickets) {
		return ticket{}, false
	}
	return s.tickets[id-1], true
}
