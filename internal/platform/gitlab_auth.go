package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	signingTokenPrefix = "whsec_"
	signingKeyBytes    = 32
	maxClockSkew       = 300 // seconds, replay window for signed webhooks
)

// gitlabAuthorized prefers Standard Webhooks signatures and supports the older GitLab token.
func gitlabAuthorized(h http.Header, body []byte, signingToken, legacySecret string, now time.Time) bool {
	if signingToken != "" {
		return validSignature(h, body, signingToken, now)
	}
	return legacySecret != "" &&
		subtle.ConstantTimeCompare([]byte(legacySecret), []byte(h.Get("X-Gitlab-Token"))) == 1
}

func validSignature(h http.Header, body []byte, signingToken string, now time.Time) bool {
	messageID, timestamp, signatures := h.Get("webhook-id"), h.Get("webhook-timestamp"), h.Get("webhook-signature")
	if messageID == "" || !isDigits(timestamp) || signatures == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || math.Abs(float64(now.UnixNano())/1e9-float64(ts)) > maxClockSkew {
		return false
	}
	if !strings.HasPrefix(signingToken, signingTokenPrefix) {
		return false
	}
	key, err := base64.StdEncoding.Strict().DecodeString(signingToken[len(signingTokenPrefix):])
	if err != nil || len(key) != signingKeyBytes {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(messageID + "." + timestamp + "."))
	mac.Write(body)
	expected := []byte("v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	// The header may carry several space-separated signatures (key rotation).
	for _, signature := range strings.Fields(signatures) {
		if hmac.Equal(expected, []byte(signature)) {
			return true
		}
	}
	return false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
