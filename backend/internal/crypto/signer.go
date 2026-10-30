package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GenerateSecret creates a cryptographically secure random secret with prefix 'whsec_'
func GenerateSecret() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(bytes), nil
}

// Sign creates an industry standard HMAC-SHA256 signature string: "t=1614555845,v1=hex"
func Sign(payload []byte, secret string, timestamp time.Time) string {
	t := timestamp.Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	// To prevent replay attacks and header tampering: sign "<timestamp>.<payload>"
	mac.Write([]byte(fmt.Sprintf("%d.", t)))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%d,v1=%s", t, signature)
}

// Verify validates the signature header against the payload, secret, and allows a tolerance window
func Verify(payload []byte, secret string, signatureHeader string, tolerance time.Duration) (bool, error) {
	parts := strings.Split(signatureHeader, ",")
	var timestampInt int64
	var signatureHex string

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			var err error
			timestampInt, err = strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return false, fmt.Errorf("invalid timestamp format: %w", err)
			}
		case "v1":
			signatureHex = kv[1]
		}
	}

	if timestampInt == 0 || signatureHex == "" {
		return false, fmt.Errorf("missing timestamp or v1 signature component")
	}

	// Verify tolerance to prevent replay attacks
	sigTime := time.Unix(timestampInt, 0)
	diff := time.Since(sigTime)
	if diff < 0 {
		diff = -diff
	}
	if tolerance > 0 && diff > tolerance {
		return false, fmt.Errorf("signature timestamp expired or too far in the future")
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestampInt)))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Constant time comparison to prevent timing attacks
	return hmac.Equal([]byte(expectedSignature), []byte(signatureHex)), nil
}
