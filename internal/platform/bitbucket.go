package platform

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// bitbucket handles Bitbucket Cloud. Bitbucket Data Center/Server uses other headers and
// payloads and is not supported.
type bitbucket struct{ Options }

func (b bitbucket) Authorized(h http.Header, body []byte, _ time.Time) bool {
	return validHMAC(h, "X-Hub-Signature", body, b.Secret)
}

func (b bitbucket) Parse(h http.Header, body []byte) (*MR, error) {
	// Bitbucket Cloud cannot reopen a pull request, so only "created" matters.
	if h.Get("X-Event-Key") != "pullrequest:created" {
		return nil, nil
	}
	payload, err := decode(body)
	if err != nil {
		return nil, err
	}
	pr, err := object(payload["pullrequest"])
	if err != nil {
		return nil, err
	}
	repo, err := object(payload["repository"])
	if err != nil {
		return nil, err
	}
	author, err := object(pr["author"])
	if err != nil {
		return nil, err
	}
	destination, err := object(pr["destination"])
	if err != nil {
		return nil, err
	}
	branch, err := object(destination["branch"])
	if err != nil {
		return nil, err
	}
	mainBranch, err := object(repo["mainbranch"])
	if err != nil {
		return nil, err
	}
	links, err := object(pr["links"])
	if err != nil {
		return nil, err
	}
	html, err := object(links["html"])
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(text(author["account_id"]), b.Author) && !strings.EqualFold(text(author["uuid"]), b.Author) {
		return nil, nil
	}
	// The payload may not state the repository's main branch; check it only when present.
	target := text(branch["name"])
	if main := text(mainBranch["name"]); main != "" && main != target {
		return nil, nil
	}
	id, okID := integer(pr["id"])
	repoID := text(repo["uuid"])
	if !okID || repoID == "" {
		return nil, nil
	}
	return build(candidate{
		RepoID: repoID, IID: id, Action: actionOpen,
		Label: "PR", Ref: fmt.Sprintf("#%d", id),
		Repo:        textOr(repo["full_name"], repoID),
		Title:       textOr(pr["title"], ""),
		URL:         textOr(html["href"], ""),
		Description: textOr(pr["description"], ""),
		Target:      target,
		UpdatedAt:   textOr(pr["updated_on"], ""),
	}, b.ReopenTestMR), nil
}
