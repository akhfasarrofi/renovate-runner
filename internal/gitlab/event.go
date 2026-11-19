package gitlab

import (
	"fmt"
	"strings"
)

const (
	actionOpen   = "open"
	actionReopen = "reopen"
)

// MR is a merge request event that passed every notification filter.
type MR struct {
	ProjectID, IID int64
	Action         string
	Repo           string
	Title          string
	URL            string
	Target         string // the project's default branch
	Severity       string // highest OSV severity; "" for a simulated reopen
	TestReopen     bool   // reopen of REOPEN_TEST_MR without severity
	Package        string // first updated package, "" if the description table is unreadable
	Change         string // "1.0.0 → 1.0.1"
	MorePackages   int    // further packages in a grouped MR
	VulnIDs        string // "CVE-... / GHSA-..." of the most severe advisory
	VulnSummary    string
	ActionedAt     string
	UpdatedAt      string
}

// ParseBotMR returns nil when the event must not notify, ErrBadPayload for malformed events.
func ParseBotMR(payload map[string]any, authorID int64, reopenTestMR string) (*MR, error) {
	if payload["object_kind"] != "merge_request" {
		return nil, nil
	}
	attributes, err := object(payload["object_attributes"])
	if err != nil {
		return nil, err
	}
	project, err := object(payload["project"])
	if err != nil {
		return nil, err
	}
	action := text(attributes["action"])
	if action != actionOpen && action != actionReopen {
		return nil, nil
	}
	if author, ok := integer(attributes["author_id"]); !ok || author != authorID {
		return nil, nil
	}
	// Compare against GitLab's own default branch; each repo may use a different name.
	defaultBranch, _ := project["default_branch"].(string)
	if defaultBranch == "" || attributes["target_branch"] != any(defaultBranch) {
		return nil, nil
	}
	projectID, okProject := integer(project["id"])
	iid, okIID := integer(attributes["iid"])
	if !okProject || !okIID {
		return nil, nil
	}
	selected := reopenTestMR == fmt.Sprintf("%d:%d", projectID, iid)
	// Reopen is enabled only for the selected MR during simulation.
	if action == actionReopen && !selected {
		return nil, nil
	}
	url := textOr(attributes["url"], "")
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil, nil
	}
	description := textOr(attributes["description"], "")
	vuln := topVulnerability(description)
	severity := vuln.Severity
	updates := parseUpdates(description)
	testReopen := action == actionReopen && selected && !notifySeverities[severity]
	if !notifySeverities[severity] && !testReopen {
		return nil, nil
	}
	return &MR{
		ProjectID:    projectID,
		IID:          iid,
		Action:       action,
		Repo:         textOr(project["path_with_namespace"], textOr(project["name"], fmt.Sprint(projectID))),
		Title:        textOr(attributes["title"], ""),
		URL:          url,
		Target:       defaultBranch,
		Severity:     severity,
		TestReopen:   testReopen,
		Package:      updates.Package,
		Change:       updates.Change,
		MorePackages: updates.More,
		VulnIDs:      strings.Join(vuln.IDs, " / "),
		VulnSummary:  vuln.Summary,
		ActionedAt:   textOr(attributes["actioned_at"], ""),
		UpdatedAt:    textOr(attributes["updated_at"], ""),
	}, nil
}

// Reopened reports whether the event is a reopen rather than a first open.
func (m *MR) Reopened() bool { return m.Action == actionReopen }
