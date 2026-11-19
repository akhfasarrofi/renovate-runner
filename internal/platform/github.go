package platform

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type github struct{ Options }

func (g github) Authorized(h http.Header, body []byte, _ time.Time) bool {
	return validHMAC(h, "X-Hub-Signature-256", body, g.Secret)
}

func (g github) Parse(h http.Header, body []byte) (*MR, error) {
	if h.Get("X-GitHub-Event") != "pull_request" {
		return nil, nil
	}
	payload, err := decode(body)
	if err != nil {
		return nil, err
	}
	var action string
	switch text(payload["action"]) {
	case "opened":
		action = actionOpen
	case "reopened":
		action = actionReopen
	default:
		return nil, nil
	}
	pr, err := object(payload["pull_request"])
	if err != nil {
		return nil, err
	}
	repo, err := object(payload["repository"])
	if err != nil {
		return nil, err
	}
	user, err := object(pr["user"])
	if err != nil {
		return nil, err
	}
	base, err := object(pr["base"])
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(text(user["login"]), g.Author) {
		return nil, nil
	}
	defaultBranch := text(repo["default_branch"])
	if defaultBranch == "" || text(base["ref"]) != defaultBranch {
		return nil, nil
	}
	repoID, okRepo := integer(repo["id"])
	number, okNumber := integer(payload["number"])
	if !okRepo || !okNumber {
		return nil, nil
	}
	return build(candidate{
		RepoID: strconv.FormatInt(repoID, 10), IID: number, Action: action,
		Label: "PR", Ref: fmt.Sprintf("#%d", number),
		Repo:        textOr(repo["full_name"], strconv.FormatInt(repoID, 10)),
		Title:       textOr(pr["title"], ""),
		URL:         textOr(pr["html_url"], ""),
		Description: textOr(pr["body"], ""),
		Target:      defaultBranch,
		UpdatedAt:   textOr(pr["updated_at"], ""),
	}, g.ReopenTestMR), nil
}
