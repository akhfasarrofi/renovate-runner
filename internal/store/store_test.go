package store

import (
	"path/filepath"
	"testing"
)

func TestNotifyOnceTextAndLegacyKeys(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "n.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	// Row written by the old integer-keyed code must still match the text key "316".
	if _, err := s.db.Exec("INSERT INTO sent_channels (project_id, iid, channel) VALUES (316, 7, 'telegram')"); err != nil {
		t.Fatal(err)
	}
	sends := 0
	send := func() error { sends++; return nil }
	for _, d := range []Delivery{
		{RepoID: "316", IID: 7, Channel: "telegram"},      // legacy duplicate
		{RepoID: "{uuid-1}", IID: 7, Channel: "telegram"}, // Bitbucket UUID, first time
		{RepoID: "{uuid-1}", IID: 7, Channel: "telegram"}, // duplicate
	} {
		if _, err := s.NotifyOnce(d, send); err != nil {
			t.Fatal(err)
		}
	}
	if sends != 1 {
		t.Fatalf("want 1 send, got %d", sends)
	}
}
