package platform

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type gitlab struct{ Options }

func (g gitlab) Authorized(h http.Header, body []byte, now time.Time) bool {
	return gitlabAuthorized(h, body, g.SigningToken, g.Secret, now)
}

func (g gitlab) Parse(h http.Header, body []byte) (*MR, error) {
	if h.Get("X-Gitlab-Event") != "Merge Request Hook" {
		return nil, nil
	}
	payload, err := decode(body)
	if err != nil {
		return nil, err
	}
	authorID, _ := strconv.ParseInt(g.Author, 10, 64)
	return parseGitLab(payload, authorID, g.ReopenTestMR)
}

// parseGitLab returns nil when the event must not notify, ErrBadPayload for malformed events.
func parseGitLab(payload map[string]any, authorID int64, reopenTestMR string) (*MR, error) {
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
	return build(candidate{
		RepoID: strconv.FormatInt(projectID, 10), IID: iid, Action: action,
		Label: "MR", Ref: fmt.Sprintf("!%d", iid),
		Repo:        textOr(project["path_with_namespace"], textOr(project["name"], fmt.Sprint(projectID))),
		Title:       textOr(attributes["title"], ""),
		URL:         textOr(attributes["url"], ""),
		Description: textOr(attributes["description"], ""),
		Target:      defaultBranch,
		ActionedAt:  textOr(attributes["actioned_at"], ""),
		UpdatedAt:   textOr(attributes["updated_at"], ""),
	}, reopenTestMR), nil
}
