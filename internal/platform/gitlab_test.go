package platform

import (
	"encoding/json"
	"testing"
)

func TestHighestOSVSeverity(t *testing.T) {
	desc := "#### Severity\n- CVSS Score: 5.3 / 10 (Medium)\n\n#### Severity\nhigh\n\n#### Severity\nLOW\n"
	if got := highestOSVSeverity(desc); got != "HIGH" {
		t.Fatalf("got %q", got)
	}
	if got := highestOSVSeverity("no severity here"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func event(action string, severity string, mutate func(attrs, project map[string]any)) map[string]any {
	attrs := map[string]any{
		"action": action, "author_id": json.Number("42"), "iid": json.Number("7"),
		"target_branch": "main", "title": "Update <lodash>", "url": "https://gitlab.test/g/p/-/merge_requests/7",
		"description": "#### Severity\n" + severity + "\n",
	}
	project := map[string]any{"id": json.Number("316"), "path_with_namespace": "g/p", "default_branch": "main"}
	if mutate != nil {
		mutate(attrs, project)
	}
	return map[string]any{"object_kind": "merge_request", "object_attributes": attrs, "project": project}
}

func TestNewBotMRFilters(t *testing.T) {
	cases := map[string]struct {
		payload map[string]any
		reopen  string
		notify  bool
	}{
		"high":            {event("open", "HIGH", nil), "", true},
		"low skipped":     {event("open", "LOW", nil), "", false},
		"no severity":     {event("open", "", nil), "", false},
		"other author":    {event("open", "HIGH", func(a, _ map[string]any) { a["author_id"] = json.Number("1") }), "", false},
		"not default":     {event("open", "HIGH", func(a, _ map[string]any) { a["target_branch"] = "dev" }), "", false},
		"update action":   {event("update", "HIGH", nil), "", false},
		"bad url":         {event("open", "HIGH", func(a, _ map[string]any) { a["url"] = "javascript:x" }), "", false},
		"reopen unlisted": {event("reopen", "HIGH", nil), "", false},
		"reopen selected": {event("reopen", "", nil), "316:7", true},
	}
	for name, c := range cases {
		item, err := parseGitLab(c.payload, 42, c.reopen)
		if err != nil || (item != nil) != c.notify {
			t.Errorf("%s: item=%v err=%v", name, item, err)
		}
	}
	bad := map[string]any{"object_kind": "merge_request", "object_attributes": "x"}
	if _, err := parseGitLab(bad, 42, ""); err == nil {
		t.Fatal("non-object attributes accepted")
	}
}

func TestParseBotMRReadsDescription(t *testing.T) {
	item, err := parseGitLab(event("open", "", func(a, _ map[string]any) { a["description"] = plainUpdate + osvNote }), 42, "")
	if err != nil || item == nil {
		t.Fatalf("item=%v err=%v", item, err)
	}
	if item.Severity != "HIGH" || item.Package != "@types/node" || item.Change != "20.19.11 → 20.19.43" ||
		item.VulnIDs != "CVE-2021-23337 / GHSA-35jh-r3h4-6jhm" || item.VulnSummary != "Command Injection in lodash" {
		t.Fatalf("fields not populated: %+v", item)
	}
}
