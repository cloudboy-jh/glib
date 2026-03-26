package piui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/cloudboy-jh/bentotui/registry/bricks/bar"
	"github.com/cloudboy-jh/bentotui/theme"
)

func (s *Session) HeaderLine(icon, repo string, width int, t theme.Theme) string {
	left := "# session"
	if strings.TrimSpace(repo) != "" {
		left = "# " + strings.TrimSpace(repo)
	}
	right := strings.TrimSpace(s.HeaderRightLine())
	b := bar.New(
		bar.RoleTopBar(),
		bar.Left(left),
		bar.Right(right),
		bar.WithTheme(t),
	)
	if width < 1 {
		width = 1
	}
	b.SetSize(width, 1)
	return headerViewString(b.View())
}

func headerViewString(v tea.View) string {
	if v.Content == nil {
		return ""
	}
	if r, ok := v.Content.(interface{ Render() string }); ok {
		return r.Render()
	}
	if s, ok := v.Content.(interface{ String() string }); ok {
		return s.String()
	}
	return fmt.Sprint(v.Content)
}
