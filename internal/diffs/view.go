package diffs

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	bdcore "github.com/cloudboy-jh/bento-diffs/pkg/bentodiffs"
	"github.com/cloudboy-jh/bentotui/theme"
	"github.com/cloudboy-jh/bentotui/theme/styles"
	"glib/internal/git"
)

const diffWordmark = "" +
	"██████╗  ██╗███████╗███████╗\n" +
	"██╔══██╗ ██║██╔════╝██╔════╝\n" +
	"██║  ██║ ██║█████╗  █████╗  \n" +
	"██║  ██║ ██║██╔══╝  ██╔══╝  \n" +
	"██████╔╝ ██║██║     ██║     \n" +
	"╚═════╝  ╚═╝╚═╝     ╚═╝     "

type HistoryRender struct {
	Wordmark  string
	Block     string
	WordmarkX int
	BlockX    int
	Y         int
}

func RenderHistory(commits []git.CommitInfo, cursor, contentW, bodyW, bodyH int, icon string, t theme.Theme) HistoryRender {
	listH := 5
	total := len(commits)
	cur := clamp(cursor, 0, max(0, total-1))
	rowW := max(8, contentW-4)

	lines := make([]string, 0, listH)
	if total == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted()).Render("No commits found"))
	} else {
		start := windowStart(cur, listH, total)
		end := min(total, start+listH)
		base := lipgloss.NewStyle().Width(rowW).Background(t.BackgroundPanel()).Foreground(t.Text())
		active := base.Copy().Background(t.BackgroundInteractive()).Foreground(t.TextInverse()).Bold(true)
		const fixedCols = 13
		for i := start; i < end; i++ {
			c := commits[i]
			prefix := "  "
			marker := "  "
			style := base
			if i == cur {
				prefix = "> "
				style = active
			}
			if i == start && start > 0 {
				marker = "^ "
			} else if i == end-1 && end < total {
				marker = "v "
			}
			msgW := max(1, rowW-fixedCols)
			msg := clipLine(c.Message, msgW)
			var rowStr string
			if i == cur {
				rowStr = styles.ClipANSI(marker+prefix+c.Hash+"  "+msg, rowW)
			} else {
				coloredHash := lipgloss.NewStyle().Foreground(t.Info()).Render(c.Hash)
				rowStr = styles.ClipANSI(marker+prefix+coloredHash+"  "+msg, rowW)
			}
			lines = append(lines, style.Render(rowStr))
		}
	}
	for len(lines) < listH {
		blank := lipgloss.NewStyle().Width(contentW).Background(t.BackgroundPanel())
		lines = append(lines, blank.Render(""))
	}

	header := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(icon + "  Commit History")
	meta := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(fmt.Sprintf("%d commits", total))
	content := lipgloss.JoinVertical(lipgloss.Left, header, meta, "", strings.Join(lines, "\n"))
	block := lipgloss.NewStyle().
		Background(t.BackgroundPanel()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus()).
		Padding(0, 1).
		Width(contentW).
		Render(content)
	blockW := lipgloss.Width(block)
	blockH := lipgloss.Height(block)

	wm := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(diffWordmark)
	wmW := lipgloss.Width(wm)
	wmH := lipgloss.Height(wm)

	stackH := wmH + 1 + blockH
	stackY := max(0, (bodyH-stackH)/2)

	return HistoryRender{
		Wordmark:  wm,
		Block:     block,
		WordmarkX: max(0, (bodyW-wmW)/2),
		BlockX:    max(0, (bodyW-blockW)/2),
		Y:         stackY,
	}
}

func RenderOpen(viewer bdcore.Viewer, width, height int, t theme.Theme) string {
	if viewer == nil {
		return ""
	}
	viewer.SetSize(max(1, width), max(1, height))
	viewer.SetTheme(t)
	v := strings.TrimRight(viewer.View(), "\n")
	if v == "" {
		return lipgloss.NewStyle().Foreground(t.TextMuted()).Render("no open changes  c commit history")
	}
	lines := strings.Split(v, "\n")
	if len(lines) > 1 {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func clipLine(v string, width int) string {
	if width <= 0 {
		return ""
	}
	return styles.ClipANSI(v, width)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func windowStart(cursor, viewRows, total int) int {
	if total <= 0 || viewRows >= total {
		return 0
	}
	if cursor < viewRows {
		return 0
	}
	if cursor >= total-viewRows {
		return total - viewRows
	}
	return cursor - viewRows/2
}
