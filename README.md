
# Distributed Webhook Delivery & Live Inspection Engine

An enterprise-grade, high-throughput webhook delivery engine built in **Golang**, paired with a modern real-time inspection console built in **Next.js 14 / React**.

Designed to handle millions of asynchronous outbound webhook deliveries with guaranteed **at-least-once delivery semantics**, cryptographic verification, and intelligent resilience patterns.

---

## 🌟 Key Features

* **High-Throughput Dispatcher**: Concurrent worker pool using Go channels & Goroutines capable of delivering thousands of webhooks/sec.
* **Cryptographic Signatures (HMAC-SHA256)**: Implements Stripe/Svix industry-standard header signatures (`X-Webhook-Signature: t=1730000000,v1=...`) with timestamped replay attack prevention.
* **Resilient Retry Engine**: Exponential backoff with randomized jitter algorithm (`3s`, `8s`, `20s`, `60s`) and Dead-Letter-Queue (DLQ) for exhausted attempts.
* **Real-time Live Inspection**: Server-Sent Events (SSE) feed showing in-flight requests, latency, HTTP status codes, and sent payloads.
* **Manual Replay Mechanism**: Instant re-queueing of failed or dead-lettered delivery attempts.
* **Zero-Dependency Mock Receiver**: Built-in test endpoint (`/api/v1/test-receiver`) with customizable latency and status code simulation (`?status=500&delay=2000`).

---

## 📐 Architecture Overview

```text
[ Business Service / API ]
            │  POST /api/v1/events
            ▼
┌────────────────────────────────────────────────────────┐
│               Go Webhook Dispatcher Engine             │
│                                                        │
│   ┌────────────────┐      ┌────────────────────────┐   │
│   │ Event Ingestion│ ───► │ Subscription Matcher   │   │
│   └────────────────┘      └───────────┬────────────┘   │
│                                       ▼                │
│                           ┌────────────────────────┐   │
│                           │   Bounded Job Queue    │   │
│                           └───────────┬────────────┘   │
│                                       ▼                │
│                         ┌───────────────────────────┐  │
│                         │ Concurrent Worker Pool    │  │
│                         │ (Goroutines: 10 Workers)  │  │
│                         └─────────────┬─────────────┘  │
│                                       │                │
│            ┌──────────────────────────┴──────────┐     │
│            ▼                                     ▼     │
│   [ HMAC-SHA256 Signer ]               [ HTTP Client ] │
│   (t=timestamp, v1=sig)                (Timeout: 8s)   │
└──────────────────────────────────────────────┬─────────┘
                                               │
               ┌───────────────────────────────┴───────────────────────────────┐
               ▼                                                               ▼
        [ 2xx Success ]                                                 [ 4xx / 5xx Fail ]
               │                                                               │
     Recorded in Store (Done)                                        ┌─────────┴─────────┐
                                                                     ▼                   ▼
                                                              [ Attempt < Max ]   [ Max Exhausted ]
                                                                     │                   │
                                                              Exponential         Moved to Dead
                                                           Backoff + Jitter       Letter Queue
                                                           (Auto-Rescheduled)
```

---

## 🔐 HMAC-SHA256 Signature Specification

Every webhook dispatched includes the following signed headers:

| Header Name | Description | Example |
| :--- | :--- | :--- |
| `X-Webhook-ID` | Unique UUID for the delivery attempt | `3fa85f64-5717-4562-b3fc-2c963f66afa6` |
| `X-Webhook-Event` | Registered event type | `payment.succeeded` |
| `X-Webhook-Timestamp` | Unix timestamp of dispatch | `1730298000` |
| `X-Webhook-Signature` | Versioned HMAC-SHA256 signature | `t=1730298000,v1=a1b2c3d4...` |

### How Receivers Verify the Signature (Pseudo-code)

```python
# Receiver verification
expected_mac = hmac_sha256(secret, f"{timestamp}.{raw_body}")
is_valid = hmac.compare_digest(expected_mac, v1_signature)
```

---

## 🚀 Getting Started

### 1. Prerequisites
* **Go** 1.22+
* **Node.js** 18+ (for Next.js frontend)

### 2. Run the Golang Backend
```bash
cd backend
go run ./cmd/server/main.go
```
The server will start on `http://localhost:8080`.

### 3. Run the Next.js Frontend
```bash
cd frontend
npm install
npm run dev
```
Open [http://localhost:3000](http://localhost:3000) in your browser.

*(Optional)* You can also double click `frontend/public/standalone.html` to view the console immediately without npm!

---

## 🧪 Testing the Engine

1. Run automated backend test suites:
   ```bash
   cd backend
   go test -v ./...
   ```
2. Trigger an automated test delivery:
   ```bash
   curl -X POST http://localhost:8080/api/v1/events \
     -H "Content-Type: application/json" \
     -d '{"event_type": "order.completed", "payload": {"order_id": "ord_123", "total": 450000}}'
   ```
3. Test the exponential backoff & auto-retry mechanism:
   Register a subscriber pointing to:
   `http://localhost:8080/api/v1/test-receiver?status=500`
   Watch the delivery attempt automatically retry with backoff delays in the dashboard!

---

## 📂 Project Structure

```text
01-webhook-engine/
├── backend/
│   ├── cmd/server/main.go          # Entrypoint & graceful shutdown
│   ├── internal/
│   │   ├── api/                    # HTTP REST handlers & SSE stream
│   │   ├── crypto/                 # HMAC-SHA256 signer & verification tests
│   │   ├── dispatcher/             # Worker pool, job queue & backoff logic
│   │   ├── models/                 # Data types & domain definitions
│   │   └── store/                  # Thread-safe in-memory store & metrics
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── app/
│   │   ├── page.tsx                # Next.js interactive console & simulator
│   │   ├── layout.tsx
│   │   └── globals.css
│   ├── public/standalone.html      # Zero-dependency instant preview UI
│   ├── package.json
│   └── tailwind.config.js
└── README.md
```
