package gitlab

import (
	"reflect"
	"testing"
)

// Real Renovate output for a plain update (no vulnerability).
const plainUpdate = "This MR contains the following updates:\n\n" +
	"| Package | Change | [Age](https://docs.renovatebot.com/merge-confidence/) | [Confidence](https://docs.renovatebot.com/merge-confidence/) |\n" +
	"|---|---|---|---|\n" +
	"| [@types/node](https://github.com/DefinitelyTyped/DefinitelyTyped/tree/master/types/node) ([source](https://github.com/DefinitelyTyped/DefinitelyTyped/tree/HEAD/types/node)) | [`20.19.11` → `20.19.43`](https://renovatebot.com/diffs/npm/@types%2fnode/20.19.11/20.19.43) | ![age](https://developer.mend.io/a?slim=true) | ![confidence](https://developer.mend.io/c?slim=true) |\n\n" +
	"---\n\n### Configuration\n\n📅 **Schedule**: (UTC)\n"

// Shape follows lib/workers/repository/process/vulnerabilities.ts.
const osvNote = "\n\n---\n\n### Command Injection in lodash\n" +
	"[CVE-2021-23337](https://nvd.nist.gov/vuln/detail/CVE-2021-23337) / [GHSA-35jh-r3h4-6jhm](https://github.com/advisories/GHSA-35jh-r3h4-6jhm)\n\n" +
	"<details>\n<summary>More information</summary>\n\n#### Details\nlodash is vulnerable.\n\n" +
	"#### Severity\n- CVSS Score: 7.2 / 10 (High)\n- Vector String: `CVSS:3.1/AV:N`\n\n#### References\n- [x](https://x)\n\n" +
	"This data is provided by [OSV](https://osv.dev/vulnerability/GHSA-35jh-r3h4-6jhm).\n</details>\n"

func TestParseUpdates(t *testing.T) {
	cases := map[string]struct {
		in   string
		want updateSummary
	}{
		"real MR":      {plainUpdate, updateSummary{"@types/node", "20.19.11 → 20.19.43", 0}},
		"no table":     {"just text", updateSummary{}},
		"crlf":         {"| Package | Change |\r\n|---|---|\r\n| [a](u) | [`1` → `2`](u) |\r\n", updateSummary{"a", "1 → 2", 0}},
		"column order": {"| Change | Package |\n|---|---|\n| [`1` → `2`](u) | b |\n", updateSummary{"b", "1 → 2", 0}},
		"grouped": {"| Package | Change |\n|---|---|\n| [a](u) | `1` → `2` |\n| [b](u) | `3` → `4` |\n| [c](u) | `5` → `6` |\n\nafter",
			updateSummary{"a", "1 → 2", 2}},
		"no change cell match": {"| Package | Change |\n|---|---|\n| [a](u) | pinned |\n", updateSummary{"a", "", 0}},
		"zero width space":     {"| Package | Change |\n|---|---|\n| @&#8203;scope/pkg (x) | `1` → `2` |\n", updateSummary{"@scope/pkg", "1 → 2", 0}},
		"unrelated table":      {"| Name | Value |\n|---|---|\n| a | b |\n", updateSummary{}},
	}
	for name, c := range cases {
		if got := parseUpdates(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", name, got, c.want)
		}
	}
}

func TestTopVulnerability(t *testing.T) {
	got := topVulnerability(plainUpdate + osvNote)
	want := vulnerability{"HIGH", []string{"CVE-2021-23337", "GHSA-35jh-r3h4-6jhm"}, "Command Injection in lodash"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}

	// A second, more severe advisory wins; an advisory without summary starts with the ID line.
	critical := "\n\n---\n\n### [GHSA-aaaa-bbbb-cccc](https://github.com/advisories/GHSA-aaaa-bbbb-cccc)\n\n<details>\n#### Severity\nCritical\n</details>\n"
	got = topVulnerability(osvNote + critical)
	if got.Severity != "CRITICAL" || got.Summary != "" || !reflect.DeepEqual(got.IDs, []string{"GHSA-aaaa-bbbb-cccc"}) {
		t.Fatalf("got %+v", got)
	}

	if got := topVulnerability(plainUpdate); got.Severity != "" {
		t.Fatalf("plain update has severity %q", got.Severity)
	}
}
