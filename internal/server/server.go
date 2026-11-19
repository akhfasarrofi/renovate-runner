package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"renovate-notifier/internal/config"
	"renovate-notifier/internal/gitlab"
	"renovate-notifier/internal/notify"
	"renovate-notifier/internal/store"
)

const maxBodyBytes = 1_000_000

// Server is the webhook HTTP handler.
type Server struct {
	Cfg   config.Config
	Store *store.Store
	Now   func() time.Time // injectable clock for the replay window
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
// GitLab retries, and 401/400/413 for rejected requests.
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI != "/webhook" {
		respond(w, http.StatusNotFound, "")
		return
	}
	body, status := readBody(r)
	if status != 0 {
		respond(w, status, "")
		return
	}
	if !gitlab.Authorized(r.Header, body, s.Cfg.SigningToken, s.Cfg.LegacySecret, s.Now()) {
		respond(w, http.StatusUnauthorized, "")
		return
	}
	if r.Header.Get("X-Gitlab-Event") != "Merge Request Hook" {
		respond(w, http.StatusOK, "")
		return
	}
	item, err := decodeEvent(body, s.Cfg)
	if err != nil {
		respond(w, http.StatusBadRequest, "")
		return
	}
	if item == nil {
		respond(w, http.StatusOK, "")
		return
	}
	message, err := notify.Message(item)
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

// decodeEvent parses the JSON body and applies the notification filters.
// It returns a nil item when the event is valid but must not notify.
func decodeEvent(body []byte, cfg config.Config) (*gitlab.MR, error) {
	var event map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&event); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, gitlab.ErrBadPayload
	}
	return gitlab.ParseBotMR(event, cfg.AuthorID, cfg.ReopenTestMR)
}

// eventKey identifies one reopen event. GitLab delivery IDs stay stable on retries but
// differ across reopen cycles, so use the MR action time only on instances without one.
func eventKey(item *gitlab.MR, h http.Header, body []byte) string {
	if !item.Reopened() {
		return ""
	}
	for _, key := range []string{h.Get("webhook-id"), h.Get("Idempotency-Key"), item.ActionedAt, item.UpdatedAt} {
		if key != "" {
			return key
		}
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// deliver sends message to every configured channel and reports whether any failed.
func (s *Server) deliver(item *gitlab.MR, key, message string) (failed bool) {
	for _, snd := range notify.Senders(s.Cfg) {
		created, err := s.Store.NotifyOnce(store.Delivery{ProjectID: item.ProjectID, IID: item.IID, Reopened: item.Reopened(), EventKey: key, Channel: snd.Name},
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
		log.Printf("%s notification %s: project=%d iid=%d", snd.Name, state, item.ProjectID, item.IID)
	}
	return failed
}
