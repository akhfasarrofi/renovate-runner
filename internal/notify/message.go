package notify

import (
	_ "embed"
	"strings"
	"text/template"

	"renovate-notifier/internal/gitlab"
)

// The notification text lives in message.tmpl; edit it there.
//
//go:embed message.tmpl
var messageTemplateText string

var messageTemplate = template.Must(template.New("message").Parse(messageTemplateText))

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
	IID        int64
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
func Message(m *gitlab.MR) (string, error) {
	var out strings.Builder
	err := messageTemplate.Execute(&out, messageData{
		Severity:   m.Severity,
		TestReopen: m.TestReopen,
		Reopened:   m.Reopened(),
		Repo:       cut(m.Repo, maxRepoChars),
		IID:        m.IID,
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
