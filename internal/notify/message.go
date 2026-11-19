package notify

import (
	_ "embed"
	"strings"
	"text/template"

	"renovate-notifier/internal/platform"
)

// The notification text lives in message.<lang>.tmpl; edit it there.
//
//go:embed message.en.tmpl
var messageEN string

//go:embed message.id.tmpl
var messageID string

var templates = map[string]*template.Template{
	"en": template.Must(template.New("en").Parse(messageEN)),
	"id": template.Must(template.New("id").Parse(messageID)),
}

// Limits in characters. 4096 is Telegram's maximum message length.
const (
	maxRepoChars    = 180
	maxTitleChars   = 450
	maxURLChars     = 2000
	maxPackageChars = 150
	maxVulnChars    = 300
	maxMessageChars = 4096
)

type messageData struct {
	Severity   string
	TestReopen bool
	Reopened   bool
	Repo       string
	Label      string
	Ref        string
	Title      string
	Target     string
	URL        string

	Package      string
	Change       string
	MorePackages int
	VulnIDs      string
	VulnSummary  string
}

// Message renders the notification text for a channel-agnostic MR event.
// lang is "en" or "id"; anything else falls back to "en".
func Message(m *platform.MR, lang string) (string, error) {
	tmpl := templates[lang]
	if tmpl == nil {
		tmpl = templates["en"]
	}
	var out strings.Builder
	err := tmpl.Execute(&out, messageData{
		Severity:   m.Severity,
		TestReopen: m.TestReopen,
		Reopened:   m.Reopened(),
		Repo:       cut(m.Repo, maxRepoChars),
		Label:      m.Label,
		Ref:        m.Ref,
		Title:      cut(m.Title, maxTitleChars),
		Target:     m.Target,
		URL:        cut(m.URL, maxURLChars),

		Package:      cut(m.Package, maxPackageChars),
		Change:       cut(m.Change, maxPackageChars),
		MorePackages: m.MorePackages,
		VulnIDs:      cut(m.VulnIDs, maxVulnChars),
		VulnSummary:  cut(m.VulnSummary, maxVulnChars),
	})
	if err != nil {
		return "", err
	}
	// The template file ends with one newline that is not part of the message.
	return cut(strings.TrimSuffix(out.String(), "\n"), maxMessageChars), nil
}

// cut truncates by characters, not bytes, so multi-byte titles stay valid UTF-8.
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
