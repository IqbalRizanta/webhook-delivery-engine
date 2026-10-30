package crypto

import (
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("unexpected error generating secret: %v", err)
	}
	if len(secret) < 20 {
		t.Fatalf("secret length too short: %s", secret)
	}
	if secret[:6] != "whsec_" {
		t.Fatalf("expected prefix 'whsec_', got %s", secret)
	}
}

func TestSignAndVerify(t *testing.T) {
	secret, _ := GenerateSecret()
	payload := []byte(`{"event":"payment.succeeded","amount":15000}`)
	now := time.Now().UTC()

	sigHeader := Sign(payload, secret, now)
	if sigHeader == "" {
		t.Fatal("expected non-empty signature header")
	}

	// Verify valid signature within 5 minute tolerance
	valid, err := Verify(payload, secret, sigHeader, 5*time.Minute)
	if err != nil {
		t.Fatalf("verification returned error: %v", err)
	}
	if !valid {
		t.Fatal("expected signature to be valid")
	}

	// Verify tampered payload fails
	tamperedPayload := []byte(`{"event":"payment.succeeded","amount":99999}`)
	validTampered, _ := Verify(tamperedPayload, secret, sigHeader, 5*time.Minute)
	if validTampered {
		t.Fatal("expected tampered payload to fail verification")
	}

	// Verify expired timestamp fails
	oldTime := now.Add(-10 * time.Minute)
	expiredSig := Sign(payload, secret, oldTime)
	validExpired, err := Verify(payload, secret, expiredSig, 5*time.Minute)
	if validExpired || err == nil {
		t.Fatal("expected expired timestamp to fail verification")
	}
}
