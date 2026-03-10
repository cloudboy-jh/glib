package gitview

import (
	"charm.land/lipgloss/v2"
	"github.com/cloudboy-jh/bentotui/theme"
	"glib/internal/gitops"
)

type State struct {
	Files        []gitops.File
	Cursor       int
	Log          []gitops.LogEntry
	LogCursor    int
	FocusOnLog   bool
	LastAction   string
	LoadedForDir string
}

func StyleStatusLine(line string, f gitops.File, t theme.Theme, selected bool) string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary))
	if selected {
		s = s.Background(lipgloss.Color(t.Surface.Elevated))
	}
	if f.Staged {
		s = s.Foreground(lipgloss.Color(t.State.Success))
	} else if f.X == '?' && f.Y == '?' {
		s = s.Foreground(lipgloss.Color(t.State.Info))
	}
	return s.Render(line)
}

func StyleLogLine(line string, t theme.Theme, selected bool) string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted))
	if selected {
		s = s.Foreground(lipgloss.Color(t.Text.Primary)).Background(lipgloss.Color(t.Surface.Elevated))
	}
	return s.Render(line)
}
