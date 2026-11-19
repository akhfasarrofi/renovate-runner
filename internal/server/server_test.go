package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"renovate-notifier/internal/config"
	"renovate-notifier/internal/notify"
	"renovate-notifier/internal/platform"
	"renovate-notifier/internal/store"
)

type fakeChannels struct {
	mu           sync.Mutex
	telegram     []string
	slack        []string
	failTelegram bool
}

func (f *fakeChannels) server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/botTOKEN/sendMessage":
			if f.failTelegram {
				w.WriteHeader(502)
				return
			}
			f.telegram = append(f.telegram, body["text"].(string))
			io.WriteString(w, `{"ok":true}`)
		case "/slack":
			f.slack = append(f.slack, body["text"].(string))
			io.WriteString(w, "ok")
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestWebhookDeliveryAndDedup(t *testing.T) {
	fake := &fakeChannels{}
	ts := fake.server(t)
	defer ts.Close()
	old := notify.TelegramAPI
	notify.TelegramAPI = ts.URL
	t.Cleanup(func() { notify.TelegramAPI = old })
	n, err := store.Open(filepath.Join(t.TempDir(), "n.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Platform: "gitlab", Author: "42", Secret: "s3cret", Lang: "en", TelegramToken: "TOKEN", TelegramChatID: "-1",
		SlackWebhookURL: ts.URL + "/slack",
	}
	pf, err := platform.New(cfg.Platform, platform.Options{Author: cfg.Author, Secret: cfg.Secret})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Platform: pf, Store: n, Now: time.Now}
	post := func(payload any, secret string) int {
		body, _ := json.Marshal(payload)
		r := httptest.NewRequest("POST", "/renovate/webhook", strings.NewReader(string(body)))
		r.Header.Set("X-Gitlab-Token", secret)
		r.Header.Set("X-Gitlab-Event", "Merge Request Hook")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w.Code
	}
	high := event("open", "HIGH")

	if code := post(high, "wrong"); code != 401 {
		t.Fatalf("wrong secret: %d", code)
	}
	fake.failTelegram = true
	if code := post(high, "s3cret"); code != 503 {
		t.Fatalf("telegram down: %d", code)
	}
	fake.failTelegram = false
	for i := 0; i < 2; i++ {
		if code := post(high, "s3cret"); code != 200 {
			t.Fatalf("retry %d: %d", i, code)
		}
	}
	if len(fake.telegram) != 1 || len(fake.slack) != 1 {
		t.Fatalf("want one message per channel, got telegram=%d slack=%d", len(fake.telegram), len(fake.slack))
	}
	if !strings.Contains(fake.slack[0], "Update &lt;lodash&gt;") {
		t.Fatalf("slack text not escaped: %s", fake.slack[0])
	}
	if code := post(event("open", "LOW"), "s3cret"); code != 200 || len(fake.telegram) != 1 {
		t.Fatalf("LOW severity notified: %d", code)
	}
	if code := post([]int{1}, "s3cret"); code != 400 {
		t.Fatalf("non-object JSON: %d", code)
	}

	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 || w.Body.String() != "ok\n" {
		t.Fatalf("healthz: %d %q", w.Code, w.Body.String())
	}
}

// event builds a GitLab merge request payload from the test bot (author 42) on the default branch.
func event(action, severity string) map[string]any {
	return map[string]any{
		"object_kind": "merge_request",
		"object_attributes": map[string]any{
			"action": action, "author_id": json.Number("42"), "iid": json.Number("7"),
			"target_branch": "main", "title": "Update <lodash>",
			"url":         "https://gitlab.test/g/p/-/merge_requests/7",
			"description": "#### Severity\n" + severity + "\n",
		},
		"project": map[string]any{"id": json.Number("316"), "path_with_namespace": "g/p", "default_branch": "main"},
	}
}
