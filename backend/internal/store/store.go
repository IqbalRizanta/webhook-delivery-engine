package store

import (
	"errors"
	"sort"
	"sync"

	"github.com/portfolio/webhook-engine/internal/models"
)

var (
	ErrNotFound = errors.New("record not found")
)

type Store struct {
	mu          sync.RWMutex
	endpoints   map[string]models.Endpoint
	events      map[string]models.Event
	attempts    map[string]models.DeliveryAttempt
	eventOrder  []string
	subscribers map[chan models.DeliveryAttempt]struct{}
	subMu       sync.Mutex
}

func NewStore() *Store {
	return &Store{
		endpoints:   make(map[string]models.Endpoint),
		events:      make(map[string]models.Event),
		attempts:    make(map[string]models.DeliveryAttempt),
		eventOrder:  make([]string, 0),
		subscribers: make(map[chan models.DeliveryAttempt]struct{}),
	}
}

// Subscribe returns a channel that receives real-time delivery attempt events
func (s *Store) Subscribe() chan models.DeliveryAttempt {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	ch := make(chan models.DeliveryAttempt, 100)
	s.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a listener
func (s *Store) Unsubscribe(ch chan models.DeliveryAttempt) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	delete(s.subscribers, ch)
	close(ch)
}

func (s *Store) broadcast(attempt models.DeliveryAttempt) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- attempt:
		default:
			// Non-blocking drop if consumer is slow
		}
	}
}

// Endpoints
func (s *Store) SaveEndpoint(ep models.Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints[ep.ID] = ep
}

func (s *Store) GetEndpoint(id string) (models.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ep, ok := s.endpoints[id]
	if !ok {
		return models.Endpoint{}, ErrNotFound
	}
	return ep, nil
}

func (s *Store) ListEndpoints() []models.Endpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]models.Endpoint, 0, len(s.endpoints))
	for _, ep := range s.endpoints {
		res = append(res, ep)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res
}

func (s *Store) DeleteEndpoint(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.endpoints[id]; !ok {
		return ErrNotFound
	}
	delete(s.endpoints, id)
	return nil
}

// Events
func (s *Store) SaveEvent(evt models.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[evt.ID] = evt
	s.eventOrder = append(s.eventOrder, evt.ID)
}

func (s *Store) GetEvent(id string) (models.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evt, ok := s.events[id]
	if !ok {
		return models.Event{}, ErrNotFound
	}
	return evt, nil
}

func (s *Store) ListEvents(limit int) []models.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := len(s.eventOrder)
	if limit <= 0 || limit > total {
		limit = total
	}
	res := make([]models.Event, 0, limit)
	for i := total - 1; i >= total-limit && i >= 0; i-- {
		id := s.eventOrder[i]
		if evt, ok := s.events[id]; ok {
			res = append(res, evt)
		}
	}
	return res
}

// Delivery Attempts
func (s *Store) SaveAttempt(attempt models.DeliveryAttempt) {
	s.mu.Lock()
	s.attempts[attempt.ID] = attempt
	s.mu.Unlock()
	s.broadcast(attempt)
}

func (s *Store) GetAttempt(id string) (models.DeliveryAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	att, ok := s.attempts[id]
	if !ok {
		return models.DeliveryAttempt{}, ErrNotFound
	}
	return att, nil
}

func (s *Store) ListAttempts(limit int) []models.DeliveryAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]models.DeliveryAttempt, 0, len(s.attempts))
	for _, att := range s.attempts {
		res = append(res, att)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	if limit > 0 && len(res) > limit {
		return res[:limit]
	}
	return res
}

func (s *Store) GetStats() models.SystemStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := models.SystemStats{
		TotalEndpoints: len(s.endpoints),
		TotalEvents:    len(s.events),
		TotalAttempts:  len(s.attempts),
	}

	var totalLatency int64
	var completedCount int64

	for _, att := range s.attempts {
		switch att.Status {
		case models.StatusSuccess:
			stats.SuccessfulAttempts++
		case models.StatusFailed, models.StatusDeadLetter:
			stats.FailedAttempts++
		case models.StatusRetrying, models.StatusPending:
			stats.RetryingAttempts++
		}
		if att.LatencyMs > 0 {
			totalLatency += att.LatencyMs
			completedCount++
		}
	}

	if completedCount > 0 {
		stats.AverageLatencyMs = float64(totalLatency) / float64(completedCount)
	}

	return stats
}
