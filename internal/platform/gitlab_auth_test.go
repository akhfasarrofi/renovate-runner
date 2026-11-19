package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"testing"
	"time"
)

var signingKey = []byte("0123456789abcdef0123456789abcdef")
var signingToken = "whsec_" + base64.StdEncoding.EncodeToString(signingKey)

func sign(id, ts string, body []byte) string {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestAuthorized(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	body := []byte(`{"a":1}`)
	ts := "1700000000"
	h := http.Header{}
	h.Set("webhook-id", "msg_1")
	h.Set("webhook-timestamp", ts)
	h.Set("webhook-signature", "v1,bad "+sign("msg_1", ts, body))
	if !gitlabAuthorized(h, body, signingToken, "", now) {
		t.Fatal("valid signature among several rejected")
	}
	if gitlabAuthorized(h, []byte(`{"a":2}`), signingToken, "", now) {
		t.Fatal("tampered body accepted")
	}
	if gitlabAuthorized(h, body, signingToken, "", now.Add(301*time.Second)) {
		t.Fatal("stale timestamp accepted")
	}
	if gitlabAuthorized(h, body, "whsec_short", "", now) {
		t.Fatal("malformed signing token accepted")
	}
	legacy := http.Header{}
	legacy.Set("X-Gitlab-Token", "s3cret")
	if !gitlabAuthorized(legacy, body, "", "s3cret", now) || gitlabAuthorized(legacy, body, "", "other", now) {
		t.Fatal("legacy token check wrong")
	}
	if gitlabAuthorized(http.Header{}, body, "", "", now) {
		t.Fatal("empty credentials accepted")
	}
}
