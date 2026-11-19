package platform

import (
	"regexp"
	"strings"
)

var (
	linkText = regexp.MustCompile(`\[([^\]]+)\]\(`)
	change   = regexp.MustCompile("`([^`]+)` → `([^`]+)`")
)

// updateSummary describes the dependency update table of a Renovate MR description.
type updateSummary struct {
	Package string // first row's package name, "" when the table is not recognised
	Change  string // "1.0.0 → 1.0.1"
	More    int    // further rows (grouped MRs)
}

// parseUpdates reads the first row of the "| Package | ... | Change | ... |" table.
// Any surprise yields the zero value: callers fall back to the MR title.
func parseUpdates(description string) updateSummary {
	var pkgCol, changeCol = -1, -1
	var rows int
	var out updateSummary
	for _, line := range strings.Split(strings.ReplaceAll(description, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if pkgCol >= 0 && rows > 0 {
				break
			}
			continue
		}
		cells := splitRow(line)
		if pkgCol < 0 {
			pkgCol, changeCol = indexOf(cells, "Package"), indexOf(cells, "Change")
			if pkgCol < 0 || changeCol < 0 {
				pkgCol = -1
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(cells[0]), "-") || len(cells) <= max(pkgCol, changeCol) {
			continue // separator row or malformed row
		}
		if rows++; rows == 1 {
			out.Package = packageName(cells[pkgCol])
			if m := change.FindStringSubmatch(cells[changeCol]); m != nil {
				out.Change = m[1] + " → " + m[2]
			}
		}
	}
	if rows > 1 {
		out.More = rows - 1
	}
	return out
}

// splitRow splits a markdown table row on unescaped pipes and trims the outer pipes.
func splitRow(line string) []string {
	line = strings.ReplaceAll(line, `\|`, "\x00")
	cells := strings.Split(strings.Trim(line, "|"), "|")
	for i, c := range cells {
		cells[i] = strings.TrimSpace(strings.ReplaceAll(c, "\x00", "|"))
	}
	return cells
}

func indexOf(cells []string, name string) int {
	for i, c := range cells {
		if c == name {
			return i
		}
	}
	return -1
}

// packageName prefers the first link text; Renovate inserts a zero-width-space entity
// after a leading "@" to avoid pinging users.
func packageName(cell string) string {
	name := cell
	if m := linkText.FindStringSubmatch(cell); m != nil {
		name = m[1]
	} else if i := strings.Index(cell, " ("); i > 0 {
		name = cell[:i]
	}
	return strings.TrimSpace(strings.ReplaceAll(name, "&#8203;", ""))
}
