package store

import (
	"database/sql"
	"errors"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Store remembers which MR events were already delivered, per channel.
type Store struct {
	db *sql.DB
	mu sync.Mutex // one send at a time, like the single-threaded Python server
}

func Open(path string) (*Store, error) {
	if strings.Contains(path, "?") {
		return nil, errors.New("database path must not contain '?'")
	}
	// immediate: take the write lock at BEGIN, so concurrent processes cannot both send.
	db, err := sql.Open("sqlite", path+"?_txlock=immediate&_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	for _, stmt := range []string{
		"CREATE TABLE IF NOT EXISTS sent_channels (" +
			"project_id INTEGER NOT NULL, iid INTEGER NOT NULL, " +
			"channel TEXT NOT NULL, PRIMARY KEY (project_id, iid, channel))",
		"CREATE TABLE IF NOT EXISTS sent_reopens (" +
			"project_id INTEGER NOT NULL, iid INTEGER NOT NULL, " +
			"event_key TEXT NOT NULL, channel TEXT NOT NULL, " +
			"PRIMARY KEY (project_id, iid, event_key, channel))",
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	// A previous Telegram-only deployment stored successes in sent_mrs.
	var previous int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='sent_mrs'").Scan(&previous); err != nil {
		return err
	}
	if previous > 0 {
		_, err := db.Exec("INSERT OR IGNORE INTO sent_channels (project_id, iid, channel) " +
			"SELECT project_id, iid, 'telegram' FROM sent_mrs")
		return err
	}
	return nil
}

// Delivery identifies one notification: an MR event sent to one channel.
type Delivery struct {
	// RepoID is text so Bitbucket UUIDs fit. SQLite's INTEGER affinity stores numeric
	// strings as integers, so rows written by older versions still match.
	RepoID   string
	IID      int64
	Reopened bool
	EventKey string // distinguishes reopen cycles; ignored for first-open events
	Channel  string
}

// NotifyOnce calls send unless d was already delivered, and records success.
// The transaction serializes duplicates across webhook retries and restarts.
func (s *Store) NotifyOnce(d Delivery, send func() error) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var row *sql.Row
	if !d.Reopened {
		row = tx.QueryRow("SELECT 1 FROM sent_channels WHERE project_id=? AND iid=? AND channel=?",
			d.RepoID, d.IID, d.Channel)
	} else {
		row = tx.QueryRow("SELECT 1 FROM sent_reopens WHERE project_id=? AND iid=? AND event_key=? AND channel=?",
			d.RepoID, d.IID, d.EventKey, d.Channel)
	}
	var found int
	switch err := row.Scan(&found); {
	case err == nil:
		return false, nil
	case !errors.Is(err, sql.ErrNoRows):
		return false, err
	}

	if err := send(); err != nil {
		return false, err
	}
	if !d.Reopened {
		_, err = tx.Exec("INSERT INTO sent_channels (project_id, iid, channel) VALUES (?, ?, ?)",
			d.RepoID, d.IID, d.Channel)
	} else {
		_, err = tx.Exec("INSERT INTO sent_reopens (project_id, iid, event_key, channel) VALUES (?, ?, ?, ?)",
			d.RepoID, d.IID, d.EventKey, d.Channel)
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
