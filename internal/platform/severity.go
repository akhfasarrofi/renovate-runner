package platform

import (
	"regexp"
	"strings"
)

var severityRank = map[string]int{
	"UNKNOWN": 0, "LOW": 1, "MODERATE": 2, "MEDIUM": 2, "HIGH": 3, "CRITICAL": 4,
}

// notifySeverities decides which OSV severities are worth a notification.
var notifySeverities = map[string]bool{"MEDIUM": true, "MODERATE": true, "HIGH": true, "CRITICAL": true}

var osvSeverity = regexp.MustCompile(
	`(?im)^#### Severity\s*\n(?:- CVSS Score:\s*[^\n]*?\((CRITICAL|HIGH|MEDIUM|MODERATE|LOW|UNKNOWN)\)` +
		`|\s*(CRITICAL|HIGH|MEDIUM|MODERATE|LOW|UNKNOWN)\s*$)`)

// vulnerability is one OSV advisory section of a Renovate security MR description.
type vulnerability struct {
	Severity string
	IDs      []string // CVE-..., GHSA-..., in the order Renovate lists them
	Summary  string
}

// topVulnerability returns the most severe advisory in the description, or the zero
// value (empty Severity) when it has none. Ties keep the first advisory.
func topVulnerability(description string) vulnerability {
	description = strings.ReplaceAll(description, "\r\n", "\n")
	var best vulnerability
	for _, loc := range osvSeverity.FindAllStringSubmatchIndex(description, -1) {
		start, end := loc[2], loc[3]
		if start < 0 {
			start, end = loc[4], loc[5]
		}
		severity := strings.ToUpper(description[start:end])
		if best.Severity != "" && severityRank[severity] <= severityRank[best.Severity] {
			continue
		}
		best = vulnerability{Severity: severity}
		best.IDs, best.Summary = advisoryHeading(description[:loc[0]])
	}
	return best
}

// highestOSVSeverity returns "" when the description has no severity section.
func highestOSVSeverity(description string) string {
	return topVulnerability(description).Severity
}

var (
	advisoryID = regexp.MustCompile(`\b[A-Z]{2,8}-[0-9A-Za-z][0-9A-Za-z-]*`)
	// An alias line starts with an ID link "[CVE-1](url)" or a bare ID, then " / " or the end.
	aliasLine = regexp.MustCompile(`^(\[[A-Z]{2,8}-[^\]]+\]\([^)]*\)|[A-Z]{2,8}-[0-9A-Za-z-]+)( / |$)`)
)

// advisoryHeading reads the "### <summary>" heading that Renovate writes before each
// advisory: the summary line (optional) then the line of linked IDs.
func advisoryHeading(before string) (ids []string, summary string) {
	i := strings.LastIndex(before, "\n### ")
	if i < 0 {
		return nil, ""
	}
	lines := strings.SplitN(before[i+len("\n### "):], "\n", 3)
	aliases := lines[0]
	if !aliasLine.MatchString(lines[0]) {
		summary = strings.TrimSpace(lines[0])
		aliases = ""
		if len(lines) > 1 {
			aliases = lines[1]
		}
	}
	seen := map[string]bool{}
	for _, id := range advisoryID.FindAllString(aliases, -1) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, summary
}
