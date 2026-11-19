// GitLab, GitHub or Bitbucket Cloud merge/pull request webhook -> bot-only Telegram and Slack notifications.
//
// Keep this behind an HTTPS reverse proxy.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"renovate-notifier/internal/config"
	"renovate-notifier/internal/notify"
	"renovate-notifier/internal/platform"
	"renovate-notifier/internal/server"
	"renovate-notifier/internal/store"
)

// dbFile is relative to the working directory: the repo root when run locally,
// the /data volume in Docker (WORKDIR /data).
const dbFile = "notified.sqlite3"

func main() {
	log.SetPrefix("renovate-notifier: ")
	// Local runs: read .env from the working directory. Real env variables take precedence.
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	pf, err := platform.New(cfg.Platform, platform.Options{
		Author: cfg.Author, ReopenTestMR: cfg.ReopenTestMR, Secret: cfg.Secret, SigningToken: cfg.SigningToken,
	})
	if err != nil {
		log.Fatal(err)
	}
	st, err := store.Open(dbFile)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	var channels []string
	for _, s := range notify.Senders(cfg) {
		channels = append(channels, s.Name)
	}
	log.Printf("Listening on port 8080; platform: %s; channels: %s", cfg.Platform, strings.Join(channels, ", "))
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           &server.Server{Cfg: cfg, Platform: pf, Store: st, Now: time.Now},
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
