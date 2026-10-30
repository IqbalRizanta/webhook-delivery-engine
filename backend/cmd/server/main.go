package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/portfolio/webhook-engine/internal/api"
	"github.com/portfolio/webhook-engine/internal/crypto"
	"github.com/portfolio/webhook-engine/internal/dispatcher"
	"github.com/portfolio/webhook-engine/internal/models"
	"github.com/portfolio/webhook-engine/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	st := store.NewStore()

	// Seed an initial demo receiver endpoint pointing to internal mock receiver
	demoSecret, _ := crypto.GenerateSecret()
	now := time.Now().UTC()
	demoEndpoint := models.Endpoint{
		ID:          uuid.New().String(),
		URL:         fmt.Sprintf("http://localhost:%s/api/v1/test-receiver", port),
		Secret:      demoSecret,
		Description: "Local Mock Receiver (Zero Config Ready)",
		EventTypes:  []string{"*"},
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	st.SaveEndpoint(demoEndpoint)

	// Create and start worker pool with 10 concurrent goroutines
	disp := dispatcher.NewDispatcher(st, 10)
	disp.Start()
	defer disp.Stop()

	server := api.NewServer(st, disp)

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      server,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown channel
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("=====================================================")
		log.Printf("🚀 Webhook Delivery Engine running on :%s", port)
		log.Printf("📡 Seeded Mock Receiver: %s", demoEndpoint.URL)
		log.Printf("🔑 Mock Endpoint Secret: %s", demoSecret)
		log.Printf("=====================================================")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down Webhook Engine gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Webhook Engine stopped successfully.")
}
