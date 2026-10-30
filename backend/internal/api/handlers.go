package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/portfolio/webhook-engine/internal/crypto"
	"github.com/portfolio/webhook-engine/internal/dispatcher"
	"github.com/portfolio/webhook-engine/internal/models"
	"github.com/portfolio/webhook-engine/internal/store"
)

type Server struct {
	store      *store.Store
	dispatcher *dispatcher.Dispatcher
	mux        *http.ServeMux
}

func NewServer(s *store.Store, d *dispatcher.Dispatcher) *Server {
	srv := &Server{
		store:      s,
		dispatcher: d,
		mux:        http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS Middleware
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Webhook-Signature, X-Webhook-ID, X-Webhook-Event, X-Webhook-Timestamp")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/v1/endpoints", s.handleEndpoints)
	s.mux.HandleFunc("/api/v1/endpoints/", s.handleEndpointByID)
	s.mux.HandleFunc("/api/v1/events", s.handleEvents)
	s.mux.HandleFunc("/api/v1/attempts", s.handleAttempts)
	s.mux.HandleFunc("/api/v1/attempts/", s.handleAttemptActions)
	s.mux.HandleFunc("/api/v1/stats", s.handleStats)
	s.mux.HandleFunc("/api/v1/events/stream", s.handleSSEStream)
	s.mux.HandleFunc("/api/v1/test-receiver", s.handleTestReceiver)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		endpoints := s.store.ListEndpoints()
		writeJSON(w, http.StatusOK, map[string]any{"data": endpoints})

	case http.MethodPost:
		var req struct {
			URL         string   `json:"url"`
			Description string   `json:"description"`
			EventTypes  []string `json:"event_types"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if req.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
			return
		}
		if len(req.EventTypes) == 0 {
			req.EventTypes = []string{"*"}
		}

		secret, err := crypto.GenerateSecret()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate secret"})
			return
		}

		now := time.Now().UTC()
		ep := models.Endpoint{
			ID:          uuid.New().String(),
			URL:         req.URL,
			Secret:      secret,
			Description: req.Description,
			EventTypes:  req.EventTypes,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		s.store.SaveEndpoint(ep)
		writeJSON(w, http.StatusCreated, ep)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleEndpointByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/endpoints/")
	if id == "" {
		http.Error(w, "missing endpoint id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		ep, err := s.store.GetEndpoint(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
			return
		}
		writeJSON(w, http.StatusOK, ep)

	case http.MethodDelete:
		if err := s.store.DeleteEndpoint(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "endpoint deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil {
				limit = parsed
			}
		}
		events := s.store.ListEvents(limit)
		writeJSON(w, http.StatusOK, map[string]any{"data": events})

	case http.MethodPost:
		var req struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if req.EventType == "" {
			req.EventType = "custom.event"
		}
		if len(req.Payload) == 0 {
			req.Payload = json.RawMessage(`{}`)
		}

		evt := models.Event{
			ID:        uuid.New().String(),
			EventType: req.EventType,
			Payload:   req.Payload,
			CreatedAt: time.Now().UTC(),
		}
		s.store.SaveEvent(evt)

		attempts := s.dispatcher.DispatchEvent(evt)
		writeJSON(w, http.StatusAccepted, map[string]any{
			"event":             evt,
			"dispatched_to":     len(attempts),
			"delivery_attempts": attempts,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAttempts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	attempts := s.store.ListAttempts(limit)
	writeJSON(w, http.StatusOK, map[string]any{"data": attempts})
}

func (s *Server) handleAttemptActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/attempts/")
	parts := strings.Split(path, "/")
	id := parts[0]

	if len(parts) > 1 && parts[1] == "retry" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		newAttempt, err := s.dispatcher.ManualRetry(id)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, newAttempt)
		return
	}

	if r.Method == http.MethodGet {
		attempt, err := s.store.GetAttempt(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "attempt not found"})
			return
		}
		writeJSON(w, http.StatusOK, attempt)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := s.store.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

// handleSSEStream streams delivery attempts real-time to the dashboard
func (s *Server) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sub := s.store.Subscribe()
	defer s.store.Unsubscribe(sub)

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case attempt, ok := <-sub:
			if !ok {
				return
			}
			data, _ := json.Marshal(attempt)
			fmt.Fprintf(w, "event: attempt\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// handleTestReceiver is a built-in mock endpoint to receive webhooks and test verification
func (s *Server) handleTestReceiver(w http.ResponseWriter, r *http.Request) {
	// Query params for testing scenarios
	statusQuery := r.URL.Query().Get("status")
	delayQuery := r.URL.Query().Get("delay")

	if delayQuery != "" {
		if delayMs, err := strconv.Atoi(delayQuery); err == nil && delayMs > 0 {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	body, _ := io.ReadAll(r.Body)
	sig := r.Header.Get("X-Webhook-Signature")
	event := r.Header.Get("X-Webhook-Event")
	webhookID := r.Header.Get("X-Webhook-ID")

	respStatus := http.StatusOK
	if statusQuery != "" {
		if parsed, err := strconv.Atoi(statusQuery); err == nil {
			respStatus = parsed
		}
	}

	writeJSON(w, respStatus, map[string]any{
		"receiver_status":    "acknowledged",
		"received_event":     event,
		"received_id":        webhookID,
		"signature_present":  sig != "",
		"payload_size_bytes": len(body),
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
	})
}
