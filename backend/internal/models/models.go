package models

import (
	"encoding/json"
	"time"
)

type DeliveryStatus string

const (
	StatusPending    DeliveryStatus = "pending"
	StatusSuccess    DeliveryStatus = "success"
	StatusFailed     DeliveryStatus = "failed"
	StatusRetrying   DeliveryStatus = "retrying"
	StatusDeadLetter DeliveryStatus = "dead_letter"
)

// Endpoint represents a subscriber receiving webhooks
type Endpoint struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Secret      string    `json:"secret"` // Shared secret for HMAC-SHA256 signature
	Description string    `json:"description"`
	EventTypes  []string  `json:"event_types"` // e.g. ["*"] or ["payment.succeeded", "order.created"]
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Event represents an incoming business event ingested by the engine
type Event struct {
	ID        string          `json:"id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// DeliveryAttempt records an execution of sending an event to an endpoint
type DeliveryAttempt struct {
	ID             string          `json:"id"`
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	EndpointID     string          `json:"endpoint_id"`
	EndpointURL    string          `json:"endpoint_url"`
	Payload        json.RawMessage `json:"payload"`
	HeadersSent    map[string]string `json:"headers_sent"`
	AttemptNumber  int             `json:"attempt_number"`
	MaxAttempts    int             `json:"max_attempts"`
	StatusCode     int             `json:"status_code"`
	ResponseBody   string          `json:"response_body"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	LatencyMs      int64           `json:"latency_ms"`
	Status         DeliveryStatus  `json:"status"`
	NextRetryAt    *time.Time      `json:"next_retry_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
}

// SystemStats summarizes metrics
type SystemStats struct {
	TotalEndpoints      int     `json:"total_endpoints"`
	TotalEvents         int     `json:"total_events"`
	TotalAttempts       int     `json:"total_attempts"`
	SuccessfulAttempts  int     `json:"successful_attempts"`
	FailedAttempts      int     `json:"failed_attempts"`
	RetryingAttempts    int     `json:"retrying_attempts"`
	AverageLatencyMs    float64 `json:"average_latency_ms"`
}
