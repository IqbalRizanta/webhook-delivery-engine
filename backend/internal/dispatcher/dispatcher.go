package dispatcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/portfolio/webhook-engine/internal/crypto"
	"github.com/portfolio/webhook-engine/internal/models"
	"github.com/portfolio/webhook-engine/internal/store"
)

type Job struct {
	Attempt models.DeliveryAttempt
	Secret  string
}

type Dispatcher struct {
	store      *store.Store
	jobQueue   chan Job
	workers    int
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewDispatcher(s *store.Store, workerCount int) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		store:    s,
		jobQueue: make(chan Job, 1000),
		workers:  workerCount,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (d *Dispatcher) Start() {
	for i := 0; i < d.workers; i++ {
		go d.worker(i)
	}
}

func (d *Dispatcher) Stop() {
	d.cancel()
	close(d.jobQueue)
}

func (d *Dispatcher) worker(id int) {
	for {
		select {
		case <-d.ctx.Done():
			return
		case job, ok := <-d.jobQueue:
			if !ok {
				return
			}
			d.process(job)
		}
	}
}

// DispatchEvent fans out an incoming event to all matching active endpoints
func (d *Dispatcher) DispatchEvent(evt models.Event) []models.DeliveryAttempt {
	endpoints := d.store.ListEndpoints()
	createdAttempts := make([]models.DeliveryAttempt, 0)

	for _, ep := range endpoints {
		if !ep.IsActive {
			continue
		}
		if !d.isSubscribed(ep.EventTypes, evt.EventType) {
			continue
		}

		attempt := models.DeliveryAttempt{
			ID:            uuid.New().String(),
			EventID:       evt.ID,
			EventType:     evt.EventType,
			EndpointID:    ep.ID,
			EndpointURL:   ep.URL,
			Payload:       evt.Payload,
			AttemptNumber: 1,
			MaxAttempts:   5,
			Status:        models.StatusPending,
			CreatedAt:     time.Now().UTC(),
		}

		d.store.SaveAttempt(attempt)
		createdAttempts = append(createdAttempts, attempt)

		d.jobQueue <- Job{
			Attempt: attempt,
			Secret:  ep.Secret,
		}
	}

	return createdAttempts
}

// ManualRetry re-enqueues an attempt immediately
func (d *Dispatcher) ManualRetry(attemptID string) (*models.DeliveryAttempt, error) {
	attempt, err := d.store.GetAttempt(attemptID)
	if err != nil {
		return nil, err
	}

	ep, err := d.store.GetEndpoint(attempt.EndpointID)
	if err != nil {
		return nil, fmt.Errorf("associated endpoint no longer exists: %w", err)
	}

	// Create a new retry attempt record
	newAttempt := models.DeliveryAttempt{
		ID:            uuid.New().String(),
		EventID:       attempt.EventID,
		EventType:     attempt.EventType,
		EndpointID:    ep.ID,
		EndpointURL:   ep.URL,
		Payload:       attempt.Payload,
		AttemptNumber: attempt.AttemptNumber + 1,
		MaxAttempts:   attempt.MaxAttempts,
		Status:        models.StatusPending,
		CreatedAt:     time.Now().UTC(),
	}

	d.store.SaveAttempt(newAttempt)

	d.jobQueue <- Job{
		Attempt: newAttempt,
		Secret:  ep.Secret,
	}

	return &newAttempt, nil
}

func (d *Dispatcher) process(job Job) {
	attempt := job.Attempt
	now := time.Now().UTC()

	// Generate HMAC signature header
	sigHeader := crypto.Sign(attempt.Payload, job.Secret, now)

	headers := map[string]string{
		"Content-Type":        "application/json",
		"User-Agent":          "WebhookEngine/1.0",
		"X-Webhook-ID":        attempt.ID,
		"X-Webhook-Event":     attempt.EventType,
		"X-Webhook-Timestamp": fmt.Sprintf("%d", now.Unix()),
		"X-Webhook-Signature": sigHeader,
	}
	attempt.HeadersSent = headers

	req, err := http.NewRequestWithContext(d.ctx, http.MethodPost, attempt.EndpointURL, bytes.NewReader(attempt.Payload))
	if err != nil {
		d.failAttempt(&attempt, 0, "", fmt.Sprintf("invalid request: %v", err), job.Secret)
		return
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := d.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	attempt.LatencyMs = latency

	if err != nil {
		d.failAttempt(&attempt, 0, "", fmt.Sprintf("network delivery error: %v", err), job.Secret)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*64)) // Limit response reading to 64KB
	attempt.StatusCode = resp.StatusCode
	attempt.ResponseBody = string(bodyBytes)

	// Webhook standard: 2xx means successfully received and acknowledged
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		attempt.Status = models.StatusSuccess
		completed := time.Now().UTC()
		attempt.CompletedAt = &completed
		d.store.SaveAttempt(attempt)
		return
	}

	// 4xx or 5xx response
	d.failAttempt(&attempt, resp.StatusCode, attempt.ResponseBody, fmt.Sprintf("HTTP %d received", resp.StatusCode), job.Secret)
}

func (d *Dispatcher) failAttempt(attempt *models.DeliveryAttempt, statusCode int, respBody, errMsg, secret string) {
	attempt.StatusCode = statusCode
	attempt.ResponseBody = respBody
	attempt.ErrorMessage = errMsg

	if attempt.AttemptNumber < attempt.MaxAttempts {
		attempt.Status = models.StatusRetrying
		delay := calculateBackoff(attempt.AttemptNumber)
		nextRetry := time.Now().UTC().Add(delay)
		attempt.NextRetryAt = &nextRetry
		d.store.SaveAttempt(*attempt)

		// Schedule next retry execution
		time.AfterFunc(delay, func() {
			select {
			case <-d.ctx.Done():
				return
			default:
				retryAttempt := *attempt
				retryAttempt.ID = uuid.New().String()
				retryAttempt.AttemptNumber++
				retryAttempt.Status = models.StatusPending
				retryAttempt.CreatedAt = time.Now().UTC()
				retryAttempt.NextRetryAt = nil

				d.store.SaveAttempt(retryAttempt)
				d.jobQueue <- Job{
					Attempt: retryAttempt,
					Secret:  secret,
				}
			}
		})
	} else {
		// Exhausted max retries -> Dead Letter Queue
		attempt.Status = models.StatusDeadLetter
		completed := time.Now().UTC()
		attempt.CompletedAt = &completed
		d.store.SaveAttempt(*attempt)
	}
}

// Exponential backoff with full jitter: base * 2^(attempt-1) + jitter
func calculateBackoff(attempt int) time.Duration {
	// Base delays: Attempt 1 -> 3s, Attempt 2 -> 8s, Attempt 3 -> 20s, Attempt 4 -> 60s
	baseSeconds := float64(3)
	switch attempt {
	case 1:
		baseSeconds = 3
	case 2:
		baseSeconds = 8
	case 3:
		baseSeconds = 20
	default:
		baseSeconds = 60
	}

	// Add random jitter up to 1000ms
	jitterMs, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return time.Duration(baseSeconds*float64(time.Second)) + time.Duration(jitterMs.Int64())*time.Millisecond
}

func (d *Dispatcher) isSubscribed(patterns []string, eventType string) bool {
	for _, p := range patterns {
		if p == "*" || p == eventType {
			return true
		}
	}
	return false
}
