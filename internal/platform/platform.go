// Package platform turns webhooks from GitLab, GitHub and Bitbucket Cloud into one MR type.
package platform

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	actionOpen   = "open"
	actionReopen = "reopen"
)

// MR is a merge/pull request event that passed every notification filter.
type MR struct {
	RepoID       string // stable repository ID; dedup key together with IID
	IID          int64
	Action       string
	Label        string // "MR" (GitLab) or "PR"
	Ref          string // "!7" (GitLab) or "#7"
	Repo         string
	Title        string
	URL          string
	Target       string // the repository's default branch
	Severity     string // highest OSV severity; "" for a simulated reopen
	TestReopen   bool   // reopen of REOPEN_TEST_MR without severity
	Package      string // first updated package, "" if the description table is unreadable
	Change       string // "1.0.0 → 1.0.1"
	MorePackages int    // further packages in a grouped MR
	VulnIDs      string // "CVE-... / GHSA-..." of the most severe advisory
	VulnSummary  string
	ActionedAt   string
	UpdatedAt    string
}

// Reopened reports whether the event is a reopen rather than a first open.
func (m *MR) Reopened() bool { return m.Action == actionReopen }

// Options carries the settings a platform needs.
type Options struct {
	Author       string // GitLab user ID, GitHub login, or Bitbucket account_id/uuid of the Renovate bot
	ReopenTestMR string // "repo_id:iid"
	Secret       string // webhook secret (all platforms)
	SigningToken string // GitLab 19.1+ signing token (whsec_...)
}

// Platform verifies and parses webhooks of one hosting service.
type Platform interface {
	Authorized(h http.Header, body []byte, now time.Time) bool
	// Parse returns nil when the event must not notify, ErrBadPayload for malformed events.
	Parse(h http.Header, body []byte) (*MR, error)
}

// New returns the platform named "gitlab", "github" or "bitbucket".
func New(name string, o Options) (Platform, error) {
	switch name {
	case "gitlab":
		return gitlab{o}, nil
	case "github":
		return github{o}, nil
	case "bitbucket":
		return bitbucket{o}, nil
	}
	return nil, fmt.Errorf("unknown platform %q", name)
}

// decode reads one JSON object; trailing data is a bad payload.
func decode(body []byte) (map[string]any, error) {
	var event map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&event); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, ErrBadPayload
	}
	return event, nil
}

// validHMAC checks "sha256=<hex>" in header against HMAC-SHA256(secret, body).
// GitHub (X-Hub-Signature-256) and Bitbucket Cloud (X-Hub-Signature) share this format.
func validHMAC(h http.Header, header string, body []byte, secret string) bool {
	got, ok := strings.CutPrefix(h.Get(header), "sha256=")
	if !ok || secret == "" {
		return false
	}
	sig, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(sig, mac.Sum(nil))
}

// candidate is the platform-neutral view of an event, filled by each platform parser.
type candidate struct {
	RepoID, Repo, Label, Ref string
	IID                      int64
	Action                   string
	Title, URL, Description  string
	Target                   string
	ActionedAt, UpdatedAt    string
}

// build applies the filters shared by every platform; nil means "do not notify".
func build(c candidate, reopenTestMR string) *MR {
	selected := reopenTestMR == fmt.Sprintf("%s:%d", c.RepoID, c.IID)
	// Reopen is enabled only for the selected MR during simulation.
	if c.Action == actionReopen && !selected {
		return nil
	}
	if !strings.HasPrefix(c.URL, "https://") && !strings.HasPrefix(c.URL, "http://") {
		return nil
	}
	vuln := topVulnerability(c.Description)
	testReopen := c.Action == actionReopen && selected && !notifySeverities[vuln.Severity]
	if !notifySeverities[vuln.Severity] && !testReopen {
		return nil
	}
	updates := parseUpdates(c.Description)
	return &MR{
		RepoID: c.RepoID, IID: c.IID, Action: c.Action, Label: c.Label, Ref: c.Ref,
		Repo: c.Repo, Title: c.Title, URL: c.URL, Target: c.Target,
		Severity: vuln.Severity, TestReopen: testReopen,
		Package: updates.Package, Change: updates.Change, MorePackages: updates.More,
		VulnIDs: strings.Join(vuln.IDs, " / "), VulnSummary: vuln.Summary,
		ActionedAt: c.ActionedAt, UpdatedAt: c.UpdatedAt,
	}
}
