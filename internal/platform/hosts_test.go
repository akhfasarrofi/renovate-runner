package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func signed(header, secret string, body []byte) http.Header {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	h := http.Header{}
	h.Set(header, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return h
}

func TestHMACPlatformsAuthorized(t *testing.T) {
	body := []byte(`{"a":1}`)
	for name, header := range map[string]string{"github": "X-Hub-Signature-256", "bitbucket": "X-Hub-Signature"} {
		p, _ := New(name, Options{Secret: "s3cret"})
		if !p.Authorized(signed(header, "s3cret", body), body, time.Now()) {
			t.Errorf("%s: valid signature rejected", name)
		}
		if p.Authorized(signed(header, "other", body), body, time.Now()) {
			t.Errorf("%s: wrong secret accepted", name)
		}
		if p.Authorized(signed(header, "s3cret", []byte("x")), body, time.Now()) {
			t.Errorf("%s: wrong body accepted", name)
		}
		if p.Authorized(http.Header{}, body, time.Now()) {
			t.Errorf("%s: missing header accepted", name)
		}
	}
	// Bitbucket's documented test vector.
	h := http.Header{}
	h.Set("X-Hub-Signature", "sha256=a4771c39fbe90f317c7824e83ddef3caae9cb3d976c214ace1f2937e133263c9")
	bb, _ := New("bitbucket", Options{Secret: "It's a Secret to Everybody"})
	if !bb.Authorized(h, []byte("Hello World!"), time.Now()) {
		t.Error("bitbucket test vector rejected")
	}
}

func githubBody(action, severity string, mutate func(pr, repo map[string]any)) []byte {
	pr := map[string]any{
		"html_url": "https://github.com/o/r/pull/7", "title": "Update lodash", "body": "#### Severity\n" + severity + "\n",
		"user": map[string]any{"login": "Renovate[bot]"}, "base": map[string]any{"ref": "main"},
	}
	repo := map[string]any{"id": 55, "full_name": "o/r", "default_branch": "main"}
	if mutate != nil {
		mutate(pr, repo)
	}
	b, _ := json.Marshal(map[string]any{"action": action, "number": 7, "pull_request": pr, "repository": repo})
	return b
}

func TestGitHubParse(t *testing.T) {
	hdr := http.Header{"X-Github-Event": {"pull_request"}}
	cases := map[string]struct {
		body   []byte
		reopen string
		notify bool
	}{
		"high":            {githubBody("opened", "HIGH", nil), "", true},
		"low":             {githubBody("opened", "LOW", nil), "", false},
		"synchronize":     {githubBody("synchronize", "HIGH", nil), "", false},
		"other author":    {githubBody("opened", "HIGH", func(pr, _ map[string]any) { pr["user"] = map[string]any{"login": "bob"} }), "", false},
		"not default":     {githubBody("opened", "HIGH", func(pr, _ map[string]any) { pr["base"] = map[string]any{"ref": "dev"} }), "", false},
		"reopen unlisted": {githubBody("reopened", "HIGH", nil), "", false},
		"reopen selected": {githubBody("reopened", "", nil), "55:7", true},
	}
	for name, c := range cases {
		p, _ := New("github", Options{Author: "renovate[bot]", ReopenTestMR: c.reopen})
		item, err := p.Parse(hdr, c.body)
		if err != nil || (item != nil) != c.notify {
			t.Errorf("%s: item=%v err=%v", name, item, err)
		}
	}
	p, _ := New("github", Options{Author: "renovate[bot]"})
	item, _ := p.Parse(hdr, githubBody("opened", "HIGH", nil))
	if item.RepoID != "55" || item.Ref != "#7" || item.Label != "PR" || item.Repo != "o/r" || item.Target != "main" {
		t.Fatalf("fields: %+v", item)
	}
	if item, err := p.Parse(http.Header{"X-Github-Event": {"ping"}}, []byte("not json")); item != nil || err != nil {
		t.Fatalf("ping must be ignored: %v %v", item, err)
	}
	if _, err := p.Parse(hdr, []byte(`{"action":"opened","pull_request":"x"}`)); err == nil {
		t.Fatal("non-object pull_request accepted")
	}
}

func bitbucketBody(severity string, mutate func(pr, repo map[string]any)) []byte {
	pr := map[string]any{
		"id": 7, "title": "Update lodash", "description": "#### Severity\n" + severity + "\n",
		"author":      map[string]any{"uuid": "{aaaa}", "account_id": "acc1"},
		"destination": map[string]any{"branch": map[string]any{"name": "main"}},
		"links":       map[string]any{"html": map[string]any{"href": "https://bitbucket.org/w/r/pull-requests/7"}},
	}
	repo := map[string]any{"uuid": "{rrrr}", "full_name": "w/r"}
	if mutate != nil {
		mutate(pr, repo)
	}
	b, _ := json.Marshal(map[string]any{"pullrequest": pr, "repository": repo})
	return b
}

func TestBitbucketParse(t *testing.T) {
	hdr := http.Header{"X-Event-Key": {"pullrequest:created"}}
	cases := map[string]struct {
		body   []byte
		author string
		notify bool
	}{
		"by account id": {bitbucketBody("HIGH", nil), "acc1", true},
		"by uuid":       {bitbucketBody("HIGH", nil), "{AAAA}", true},
		"other author":  {bitbucketBody("HIGH", nil), "acc2", false},
		"low":           {bitbucketBody("LOW", nil), "acc1", false},
		"mainbranch differs": {bitbucketBody("HIGH", func(_, repo map[string]any) {
			repo["mainbranch"] = map[string]any{"name": "develop"}
		}), "acc1", false},
		"mainbranch matches": {bitbucketBody("HIGH", func(_, repo map[string]any) {
			repo["mainbranch"] = map[string]any{"name": "main"}
		}), "acc1", true},
	}
	for name, c := range cases {
		p, _ := New("bitbucket", Options{Author: c.author})
		item, err := p.Parse(hdr, c.body)
		if err != nil || (item != nil) != c.notify {
			t.Errorf("%s: item=%v err=%v", name, item, err)
		}
	}
	p, _ := New("bitbucket", Options{Author: "acc1"})
	item, _ := p.Parse(hdr, bitbucketBody("HIGH", nil))
	if item.RepoID != "{rrrr}" || item.Ref != "#7" || item.URL != "https://bitbucket.org/w/r/pull-requests/7" {
		t.Fatalf("fields: %+v", item)
	}
	if item, _ := p.Parse(http.Header{"X-Event-Key": {"pullrequest:updated"}}, bitbucketBody("HIGH", nil)); item != nil {
		t.Fatal("updated event notified")
	}
}

// Real sample from github.com/octokit/webhooks (MIT). Only the body is edited to look like a security PR.
func TestGitHubParseOctokitSample(t *testing.T) {
	raw, err := os.ReadFile("testdata/github_pull_request_opened.json")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := New("github", Options{Author: "codertocat"})
	h := http.Header{"X-Github-Event": {"pull_request"}}
	if item, err := p.Parse(h, raw); err != nil || item != nil {
		t.Fatalf("plain PR must not notify: %v %v", item, err)
	}
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	event["pull_request"].(map[string]any)["body"] = plainUpdate + osvNote
	edited, _ := json.Marshal(event)
	item, err := p.Parse(h, edited)
	if err != nil || item == nil {
		t.Fatalf("item=%v err=%v", item, err)
	}
	if item.Severity != "HIGH" || item.RepoID != "186853002" || item.Repo != "Codertocat/Hello-World" ||
		item.Target != "master" || item.Ref != "#2" || item.Package != "@types/node" {
		t.Fatalf("fields: %+v", item)
	}
}

// Bitbucket (Renovate's massageMarkdown) rewrites <details> into "**More information**",
// GitHub rewrites github.com links to redirect.github.com. Neither may break the parser.
func TestDescriptionSurvivesPlatformRewrites(t *testing.T) {
	bitbucketOSV := strings.NewReplacer("<details>\n<summary>More information</summary>\n", "\n**More information**\n", "</details>\n", "").Replace(osvNote)
	githubOSV := strings.ReplaceAll(osvNote, "](https://github.com/", "](https://redirect.github.com/")
	for name, desc := range map[string]string{"bitbucket": plainUpdate + bitbucketOSV, "github": plainUpdate + githubOSV} {
		v := topVulnerability(desc)
		if v.Severity != "HIGH" || len(v.IDs) != 2 || v.Summary != "Command Injection in lodash" || parseUpdates(desc).Package != "@types/node" {
			t.Errorf("%s: vuln=%+v updates=%+v", name, v, parseUpdates(desc))
		}
	}
}
