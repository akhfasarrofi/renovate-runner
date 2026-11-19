package notify

import (
	"strings"
	"testing"

	"renovate-notifier/internal/gitlab"
)

func mr(mutate func(*gitlab.MR)) *gitlab.MR {
	m := &gitlab.MR{
		ProjectID: 316, IID: 588, Action: "open", Repo: "frontend/dashboard", Title: "Update dependency lodash to v4.17.21 [SECURITY]",
		URL: "https://gitlab.test/frontend/dashboard/-/merge_requests/588", Target: "development", Severity: "HIGH",
		Package: "lodash", Change: "4.17.20 → 4.17.21",
		VulnIDs: "CVE-2021-23337 / GHSA-35jh-r3h4-6jhm", VulnSummary: "Command Injection in lodash",
	}
	if mutate != nil {
		mutate(m)
	}
	return m
}

func TestMessage(t *testing.T) {
	got, err := Message(mr(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := "🚨 Security · HIGH\n" +
		"lodash 4.17.20 → 4.17.21\n" +
		"CVE-2021-23337 / GHSA-35jh-r3h4-6jhm — Command Injection in lodash\n\n" +
		"Repo: frontend/dashboard\n" +
		"MR: !588 → development\n" +
		"https://gitlab.test/frontend/dashboard/-/merge_requests/588"
	if got != want {
		t.Fatalf("message:\n%s", got)
	}
}

func TestMessageVariants(t *testing.T) {
	cases := map[string]struct {
		mutate func(*gitlab.MR)
		want   []string // substrings that must appear, in the message
		not    []string
	}{
		"grouped": {func(m *gitlab.MR) { m.MorePackages = 3 }, []string{"lodash 4.17.20 → 4.17.21 (+3 paket lain)\n"}, nil},
		"table unreadable falls back to title": {func(m *gitlab.MR) { m.Package, m.Change = "", "" },
			[]string{"HIGH\nUpdate dependency lodash to v4.17.21 [SECURITY]\n"}, nil},
		"no change cell":          {func(m *gitlab.MR) { m.Change = "" }, []string{"HIGH\nlodash\nCVE"}, nil},
		"ids without summary":     {func(m *gitlab.MR) { m.VulnSummary = "" }, []string{"GHSA-35jh-r3h4-6jhm\n\nRepo"}, []string{"—"}},
		"summary without ids":     {func(m *gitlab.MR) { m.VulnIDs = "" }, []string{"4.17.21\nCommand Injection in lodash\n\nRepo"}, nil},
		"no vulnerability detail": {func(m *gitlab.MR) { m.VulnIDs, m.VulnSummary = "", "" }, []string{"4.17.21\n\nRepo"}, []string{"CVE"}},
		"reopened":                {func(m *gitlab.MR) { m.Action = "reopen" }, []string{"MR: !588 → development · dibuka kembali\n"}, nil},
		"test reopen": {func(m *gitlab.MR) { m.Action, m.Severity, m.TestReopen = "reopen", "", true },
			[]string{"🧪 Test reopen · tanpa severity\n"}, []string{"Security"}},
	}
	for name, c := range cases {
		got, err := Message(mr(c.mutate))
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: missing %q in:\n%s", name, w, got)
			}
		}
		for _, n := range c.not {
			if strings.Contains(got, n) {
				t.Errorf("%s: unexpected %q in:\n%s", name, n, got)
			}
		}
	}
}

func TestMessageLimits(t *testing.T) {
	got, _ := Message(mr(func(m *gitlab.MR) {
		m.Package, m.VulnSummary, m.VulnIDs = strings.Repeat("é", 5000), strings.Repeat("é", 5000), strings.Repeat("é", 5000)
	}))
	if n := len([]rune(got)); n > maxMessageChars {
		t.Fatalf("message has %d chars", n)
	}
	if !strings.Contains(got, strings.Repeat("é", maxVulnChars)+"\n\nRepo") {
		t.Fatal("vulnerability text not cut to its limit")
	}
}
