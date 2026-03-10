package diffview

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/cloudboy-jh/bentotui/theme"
)

type State struct {
	Lines         []string
	Scroll        int
	FileAnchors   []int
	FileAnchorPtr int
	ShowStaged    bool
	Source        string
	CommitSHA     string
	LoadedForDir  string
}

func ParseAnchors(lines []string) []int {
	anchors := make([]int, 0)
	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			anchors = append(anchors, i)
		}
	}
	return anchors
}

func StyleLine(line string, t theme.Theme) string {
	s := lipgloss.NewStyle()
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return s.Foreground(lipgloss.Color(t.Text.Muted)).Render(line)
	case strings.HasPrefix(line, "+"):
		return s.Foreground(lipgloss.Color(t.State.Success)).Render(line)
	case strings.HasPrefix(line, "-"):
		return s.Foreground(lipgloss.Color(t.State.Danger)).Render(line)
	case strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index "):
		return s.Foreground(lipgloss.Color(t.Text.Accent)).Render(line)
	default:
		return s.Foreground(lipgloss.Color(t.Text.Primary)).Render(line)
	}
}
