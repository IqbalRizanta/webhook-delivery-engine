package dispatcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/portfolio/webhook-engine/internal/crypto"
	"github.com/portfolio/webhook-engine/internal/models"
	"github.com/portfolio/webhook-engine/internal/store"
)

func TestDispatcherDelivery(t *testing.T) {
	var receivedCount int32
	var receivedSig string

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&receivedCount, 1)
		receivedSig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer testServer.Close()

	st := store.NewStore()
	secret, _ := crypto.GenerateSecret()
	ep := models.Endpoint{
		ID:         uuid.New().String(),
		URL:        testServer.URL,
		Secret:     secret,
		EventTypes: []string{"test.event"},
		IsActive:   true,
		CreatedAt:  time.Now(),
	}
	st.SaveEndpoint(ep)

	disp := NewDispatcher(st, 2)
	disp.Start()
	defer disp.Stop()

	evt := models.Event{
		ID:        uuid.New().String(),
		EventType: "test.event",
		Payload:   json.RawMessage(`{"hello":"world"}`),
		CreatedAt: time.Now(),
	}
	st.SaveEvent(evt)

	disp.DispatchEvent(evt)

	// Wait up to 2 seconds for worker to process
	time.Sleep(300 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 1 {
		t.Fatalf("expected 1 delivery, got %d", receivedCount)
	}

	if receivedSig == "" {
		t.Fatal("expected X-Webhook-Signature to be received")
	}

	// Verify the attempt in store is marked success
	attempts := st.ListAttempts(10)
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt stored, got %d", len(attempts))
	}
	if attempts[0].Status != models.StatusSuccess {
		t.Fatalf("expected status success, got %s", attempts[0].Status)
	}
}
