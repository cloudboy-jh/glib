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
	// The block has Padding(0,1) + border (1px each side) = 4 chars of frame.
	// rowW must match the actual inner drawable width so rows don't wrap.
	rowW := max(8, contentW-4)
	base := lipgloss.NewStyle().Background(t.BackgroundPanel()).Foreground(t.Text())
	active := base.Copy().Background(t.BackgroundInteractive()).Foreground(t.TextInverse()).Bold(true)
	muted := base.Copy().Foreground(t.TextMuted())
	hashW := 7
	gap := "  "
	padRow := func(v string) string {
		v = clipLine(v, rowW)
		if lipgloss.Width(v) < rowW {
			v += strings.Repeat(" ", rowW-lipgloss.Width(v))
		}
		return v
	}
	lines := make([]string, 0, listH)
	if total == 0 {
		lines = append(lines, muted.Render(padRow("No commits found")))
	} else {
		start := windowStart(cur, listH, total)
		end := min(total, start+listH)
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
			hash := strings.TrimSpace(c.Hash)
			if len(hash) > 7 {
				hash = hash[:7]
			}
			if hash == "" {
				hash = "-------"
			}
			if lipgloss.Width(hash) < hashW {
				hash += strings.Repeat(" ", hashW-lipgloss.Width(hash))
			}
			lead := marker + prefix + hash + gap
			msgW := max(1, rowW-lipgloss.Width(lead))
			row := lead + clipCommitTitle(c.Message, msgW)
			lines = append(lines, style.Render(padRow(row)))
		}
	}
	for len(lines) < listH {
		lines = append(lines, base.Render(padRow("")))
	}

	// Render header and meta at the same inner width so all lines are uniformly
	// pre-painted and the block background doesn't create a second-layer artifact.
	headerStyle := lipgloss.NewStyle().Background(t.BackgroundPanel()).Foreground(t.TextAccent()).Bold(true)
	metaStyle := lipgloss.NewStyle().Background(t.BackgroundPanel()).Foreground(t.TextMuted())
	header := headerStyle.Render(padRow(icon + "  Commit History"))
	meta := metaStyle.Render(padRow(fmt.Sprintf("%d commits", total)))
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

func singleLine(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	if i := strings.IndexByte(v, '\n'); i >= 0 {
		v = v[:i]
	}
	v = strings.Join(strings.Fields(v), " ")
	return strings.TrimSpace(v)
}

func clipCommitTitle(v string, width int) string {
	v = singleLine(v)
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(v) <= width {
		return v
	}
	if width <= 2 {
		return styles.ClipANSI(v, width)
	}
	return styles.ClipANSI(v, width-2) + ".."
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
