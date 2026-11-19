package server

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"renovate-notifier/internal/config"
	"renovate-notifier/internal/notify"
	"renovate-notifier/internal/platform"
	"renovate-notifier/internal/store"
)

const maxBodyBytes = 1_000_000

// Server is the webhook HTTP handler.
type Server struct {
	Cfg      config.Config
	Platform platform.Platform
	Store    *store.Store
	Now      func() time.Time // injectable clock for the replay window
}

func respond(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	io.WriteString(w, body)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.RequestURI != "/healthz" {
			respond(w, http.StatusNotFound, "")
			return
		}
		respond(w, http.StatusOK, "ok\n")
	case http.MethodPost:
		s.webhook(w, r)
	default:
		respond(w, http.StatusNotImplemented, "")
	}
}

// webhook answers 200 for events that need no notification, 503 when any channel fails so
// the platform may retry, and 401/400/413 for rejected requests.
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI != "/renovate/webhook" {
		respond(w, http.StatusNotFound, "")
		return
	}
	body, status := readBody(r)
	if status != 0 {
		respond(w, status, "")
		return
	}
	if !s.Platform.Authorized(r.Header, body, s.Now()) {
		respond(w, http.StatusUnauthorized, "")
		return
	}
	item, err := s.Platform.Parse(r.Header, body)
	if err != nil {
		respond(w, http.StatusBadRequest, "")
		return
	}
	if item == nil {
		respond(w, http.StatusOK, "")
		return
	}
	message, err := notify.Message(item, s.Cfg.Lang)
	if err != nil {
		log.Printf("render message: %v", err)
		respond(w, http.StatusInternalServerError, "")
		return
	}
	if s.deliver(item, eventKey(item, r.Header, body), message) {
		respond(w, http.StatusServiceUnavailable, "")
		return
	}
	respond(w, http.StatusOK, "")
}

// readBody returns the request body, or a non-zero HTTP status to reject the request with.
func readBody(r *http.Request) ([]byte, int) {
	if r.ContentLength < 1 || r.ContentLength > maxBodyBytes {
		return nil, http.StatusRequestEntityTooLarge
	}
	body := make([]byte, r.ContentLength)
	if _, err := io.ReadFull(r.Body, body); err != nil {
		return nil, http.StatusBadRequest
	}
	return body, 0
}

// eventKey identifies one reopen event. Delivery IDs stay stable on retries but
// differ across reopen cycles, so use the MR action time only on instances without one.
func eventKey(item *platform.MR, h http.Header, body []byte) string {
	if !item.Reopened() {
		return ""
	}
	for _, key := range []string{h.Get("webhook-id"), h.Get("Idempotency-Key"), h.Get("X-GitHub-Delivery"), item.ActionedAt, item.UpdatedAt} {
		if key != "" {
			return key
		}
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// deliver sends message to every configured channel and reports whether any failed.
func (s *Server) deliver(item *platform.MR, key, message string) (failed bool) {
	for _, snd := range notify.Senders(s.Cfg) {
		created, err := s.Store.NotifyOnce(store.Delivery{RepoID: item.RepoID, IID: item.IID, Reopened: item.Reopened(), EventKey: key, Channel: snd.Name},
			func() error { return snd.Send(message) })
		if err != nil {
			log.Printf("%s delivery failed; webhook can be retried: %v", snd.Name, err)
			failed = true
			continue
		}
		state := "duplicate"
		if created {
			state = "sent"
		}
		log.Printf("%s notification %s: repo=%s iid=%d", snd.Name, state, item.RepoID, item.IID)
	}
	return failed
}
