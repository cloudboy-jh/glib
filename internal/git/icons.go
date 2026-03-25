package git

import (
	"os"
	"strings"
)

// Icon set sourced from gh-dash (dlvhdr/gh-dash) nerd font codepoints.
// Falls back to ASCII when GLIB_ICONS=safe.

type Icons struct {
	Modified  string
	Added     string
	Deleted   string
	Renamed   string
	Untracked string
	Branch    string
	Commit    string
	Success   string
	Failure   string
	Open      string
	Merged    string
	Closed    string
	Draft     string
	Ahead     string
	Behind    string
	Selection string
	Search    string
}

var NerdIcons = Icons{
	Modified:  "\uea73", // nf-cod-pencil
	Added:     "\uea60", // nf-cod-diff_added
	Deleted:   "\uea62", // nf-cod-diff_removed
	Renamed:   "\uea64", // nf-cod-diff_renamed
	Untracked: "\uea61", // nf-cod-diff_ignored
	Branch:    "\ue725", // nf-dev-git_branch
	Commit:    "\ue729", // nf-oct-git_commit
	Success:   "\uf00c", // nf-fa-check
	Failure:   "󰅙",      // nf-md-close_circle
	Open:      "\ue727", // nf-oct-git_pull_request
	Merged:    "\ue726", // nf-oct-git_merge
	Closed:    "\ue728", // nf-cod-git_pull_request_closed
	Draft:     "\ueabf", // nf-cod-git_pull_request_draft
	Ahead:     "↑",
	Behind:    "↓",
	Selection: "❯",
	Search:    "\uf002", // nf-fa-search
}

var SafeIcons = Icons{
	Modified:  "~",
	Added:     "+",
	Deleted:   "-",
	Renamed:   "R",
	Untracked: "?",
	Branch:    "br",
	Commit:    "*",
	Success:   "OK",
	Failure:   "X",
	Open:      "O",
	Merged:    "M",
	Closed:    "C",
	Draft:     "D",
	Ahead:     "^",
	Behind:    "v",
	Selection: ">",
	Search:    "/",
}

func ResolveIcons() Icons {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GLIB_ICONS")))
	if mode == "safe" {
		return SafeIcons
	}
	return NerdIcons
}

// FileStatusIcon returns the appropriate icon and a semantic label for a
// porcelain status code (single char: M, A, D, R, ?, etc.).
func (ic Icons) FileStatusIcon(status string) string {
	switch status {
	case "M":
		return ic.Modified
	case "A":
		return ic.Added
	case "D":
		return ic.Deleted
	case "R":
		return ic.Renamed
	case "?":
		return ic.Untracked
	default:
		return status
	}
}
