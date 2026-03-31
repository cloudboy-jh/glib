package git

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cloudboy-jh/bentotui/registry/bricks/badge"
	"github.com/cloudboy-jh/bentotui/registry/bricks/card"
	"github.com/cloudboy-jh/bentotui/registry/rooms"
	"github.com/cloudboy-jh/bentotui/theme"
	"github.com/cloudboy-jh/bentotui/theme/styles"
)

// RenderStatusView renders the full git status view using bentotui primitives.
// Returns a string ready to be drawn onto a surface.
func RenderStatusView(state GitState, width, height int, t theme.Theme) string {
	ic := ResolveIcons()

	branchIcon := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(ic.Branch)
	branchName := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(state.Branch)
	track := state.Tracking
	if track == "" {
		track = "(no upstream)"
	}
	trackLine := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(" ← " + track)

	syncStr := ""
	if state.Ahead > 0 {
		syncStr += " " + lipgloss.NewStyle().Foreground(t.Success()).Bold(true).Render(
			fmt.Sprintf("%s%d", ic.Ahead, state.Ahead))
	}
	if state.Behind > 0 {
		syncStr += " " + lipgloss.NewStyle().Foreground(t.Warning()).Bold(true).Render(
			fmt.Sprintf("%s%d", ic.Behind, state.Behind))
	}

	headerLine := branchIcon + " " + branchName + trackLine + syncStr
	summaryLine := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(
		fmt.Sprintf("%d changed  %d staged  +%d -%d",
			state.ChangedTotal, state.StagedTotal, state.AddedTotal, state.DeletedTotal),
	)

	rows := state.Rows()

	// ── Left pane: file list ─────────────────────────────────────────────────
	leftPane := rooms.RenderFunc(func(w, h int) string {
		contentW := max(8, w-2)
		contentH := max(1, h-2)
		headerRows := 3
		listH := max(1, contentH-headerRows)
		start := windowStart(state.Cursor, listH, len(rows))
		end := min(len(rows), start+listH)

		lines := []string{
			clipLine(headerLine, contentW),
			clipLine(summaryLine, contentW),
			"",
		}

		for i := start; i < end; i++ {
			row := rows[i]
			if row.IsHeader() {
				label := lipgloss.NewStyle().
					Foreground(t.TextMuted()).
					Bold(true).
					Render(row.Label)
				lines = append(lines, clipLine(label, contentW))
				continue
			}

			f := row.File
			isSelected := i == state.Cursor

			statusIcon := ic.FileStatusIcon(f.Status)
			var statusStyle lipgloss.Style
			switch f.Status {
			case "M":
				statusStyle = lipgloss.NewStyle().Foreground(t.Warning())
			case "A":
				statusStyle = lipgloss.NewStyle().Foreground(t.Success())
			case "D":
				statusStyle = lipgloss.NewStyle().Foreground(t.Error())
			case "R":
				statusStyle = lipgloss.NewStyle().Foreground(t.Info())
			default:
				statusStyle = lipgloss.NewStyle().Foreground(t.TextMuted())
			}

			cursorStr := " "
			if isSelected {
				cursorStr = lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(ic.Selection)
			}

			addStr := lipgloss.NewStyle().Foreground(t.Success()).Render(fmt.Sprintf("+%d", f.Added))
			delStr := lipgloss.NewStyle().Foreground(t.Error()).Render(fmt.Sprintf("-%d", f.Deleted))
			statsStr := addStr + " " + delStr
			statsW := lipgloss.Width(statsStr)

			iconRendered := statusStyle.Render(statusIcon)
			leftFixedW := 4
			pathW := max(8, contentW-leftFixedW-statsW-2)
			path := clipLine(f.Path, pathW)

			left := cursorStr + " " + iconRendered + " " + path
			leftW := lipgloss.Width(left)
			gap := max(1, contentW-leftW-statsW)
			line := left + strings.Repeat(" ", gap) + statsStr

			if isSelected {
				line = lipgloss.NewStyle().
					Background(t.BackgroundInteractive()).
					Width(contentW).
					Render(line)
			}
			lines = append(lines, clipLine(line, contentW))
		}

		content := &staticTextModel{text: strings.Join(lines, "\n")}
		c := card.New(
			card.Title("glib  GIT"),
			card.Content(content),
			card.Raised(),
		)
		c.SetSize(w, h)
		return viewStr(c.View())
	})

	// ── Right pane: selection detail ─────────────────────────────────────────
	rightPane := rooms.RenderFunc(func(w, h int) string {
		contentW := max(8, w-2)

		var selectedPath, selectedStatus string
		var selectedAdded, selectedDeleted int
		if f, ok := state.SelectedFile(); ok {
			selectedPath = f.Path
			selectedStatus = f.Status
			selectedAdded = f.Added
			selectedDeleted = f.Deleted
		}

		statusBadge := badge.New(strings.TrimSpace(selectedStatus))
		statusBadge.SetVariant(badge.VariantNeutral)
		switch selectedStatus {
		case "A":
			statusBadge.SetVariant(badge.VariantSuccess)
		case "M":
			statusBadge.SetVariant(badge.VariantWarning)
		case "D":
			statusBadge.SetVariant(badge.VariantDanger)
		case "R":
			statusBadge.SetVariant(badge.VariantInfo)
		}

		statsLine := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("stats ") +
			lipgloss.NewStyle().Foreground(t.Success()).Render(fmt.Sprintf("+%d", selectedAdded)) +
			" " +
			lipgloss.NewStyle().Foreground(t.Error()).Render(fmt.Sprintf("-%d", selectedDeleted))

		lines := []string{
			clipLine(lipgloss.NewStyle().Bold(true).Render(truncate(selectedPath, contentW)), contentW),
			clipLine(lipgloss.NewStyle().Foreground(t.TextMuted()).Render("status ")+viewStr(statusBadge.View()), contentW),
			clipLine(statsLine, contentW),
			"",
		}

		if state.LastCommit.Hash != "" {
			commitIcon := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(ic.Commit)
			hash := lipgloss.NewStyle().Foreground(t.Info()).Render(state.LastCommit.Hash)
			msg := truncate(state.LastCommit.Message, max(8, contentW-20))
			rel := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("· " + RelativeTime(state.LastCommit.Time))
			lines = append(lines,
				clipLine(lipgloss.NewStyle().Foreground(t.TextMuted()).Render("last commit"), contentW),
				clipLine(commitIcon+"  "+hash+"  "+msg+"  "+rel, contentW),
			)
		}

		if state.LastAction != "" {
			lines = append(lines,
				"",
				clipLine(lipgloss.NewStyle().Foreground(t.Success()).Render("✓ "+state.LastAction), contentW),
			)
		}

		content := &staticTextModel{text: strings.Join(lines, "\n")}
		c := card.New(card.Title("Selection"), card.Content(content), card.Raised())
		c.SetSize(w, h)
		return viewStr(c.View())
	})

	splitW := width * 55 / 100
	return rooms.Rail(width, height, splitW, leftPane, rightPane)
}

// RenderBranchesView renders the branch list view.
func RenderBranchesView(branches []string, current string, cursor int, width, height int, t theme.Theme) string {
	ic := ResolveIcons()
	contentW := max(8, width-4)
	lines := []string{}

	if len(branches) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted()).Render("No local branches — n create branch"))
	} else {
		for i, b := range branches {
			isCurrent := b == current
			isSelected := i == cursor

			prefix := "  "
			if isCurrent {
				prefix = lipgloss.NewStyle().Foreground(t.Success()).Render(ic.Selection) + " "
			}

			line := prefix + b
			if isSelected {
				line = lipgloss.NewStyle().
					Background(t.BackgroundInteractive()).
					Foreground(t.TextInverse()).
					Width(contentW).
					Render(line)
			} else if isCurrent {
				line = lipgloss.NewStyle().Foreground(t.TextAccent()).Render(line)
			}
			lines = append(lines, clipLine(line, contentW))
		}
	}

	content := &staticTextModel{text: strings.Join(lines, "\n")}
	c := card.New(
		card.Title(ic.Branch+"  Branches"),
		card.Content(content),
		card.Raised(),
	)
	c.SetSize(width, height)
	return viewStr(c.View())
}

// RenderLogView renders the commit log view.
func RenderLogView(commits []CommitInfo, cursor int, width, height int, t theme.Theme) string {
	ic := ResolveIcons()
	contentW := max(8, width-4)
	lines := []string{}

	if len(commits) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted()).Render("No commits"))
	} else {
		for i, cm := range commits {
			isSelected := i == cursor
			hash := lipgloss.NewStyle().Foreground(t.Info()).Render(cm.Hash)
			if isSelected {
				hash = cm.Hash
			}
			rel := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(RelativeTime(cm.Time))
			msgW := max(8, contentW-len(cm.Hash)-len(RelativeTime(cm.Time))-6)
			msg := truncate(cm.Message, msgW)
			line := ic.Commit + "  " + hash + "  " + msg + "  " + rel
			if isSelected {
				line = lipgloss.NewStyle().
					Background(t.BackgroundInteractive()).
					Foreground(t.TextInverse()).
					Width(contentW).
					Render(line)
			}
			lines = append(lines, clipLine(line, contentW))
		}
	}

	content := &staticTextModel{text: strings.Join(lines, "\n")}
	c := card.New(
		card.Title(ic.Commit+"  Commit Log"),
		card.Content(content),
		card.Raised(),
	)
	c.SetSize(width, height)
	return viewStr(c.View())
}

// RenderStashView renders the stash list view.
func RenderStashView(items []string, cursor int, width, height int, t theme.Theme) string {
	contentW := max(8, width-4)
	lines := []string{}

	if len(items) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted()).Render("No stashes — z stash current changes"))
	} else {
		for i, item := range items {
			isSelected := i == cursor
			line := item
			if isSelected {
				line = lipgloss.NewStyle().
					Background(t.BackgroundInteractive()).
					Foreground(t.TextInverse()).
					Width(contentW).
					Render(item)
			}
			lines = append(lines, clipLine(line, contentW))
		}
	}

	content := &staticTextModel{text: strings.Join(lines, "\n")}
	c := card.New(card.Title("Stash"), card.Content(content), card.Raised())
	c.SetSize(width, height)
	return viewStr(c.View())
}

// ── internal helpers ──────────────────────────────────────────────────────────

// staticTextModel satisfies tea.Model and card.Content for static text rendering.
type staticTextModel struct {
	text   string
	width  int
	height int
}

func (s *staticTextModel) Init() tea.Cmd                           { return nil }
func (s *staticTextModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s *staticTextModel) SetSize(width, height int) {
	s.width = width
	s.height = height
}
func (s *staticTextModel) View() tea.View {
	if s.height <= 0 {
		return tea.NewView("")
	}
	lines := strings.Split(s.text, "\n")
	out := make([]string, 0, min(len(lines), s.height))
	for i := 0; i < len(lines) && i < s.height; i++ {
		out = append(out, clipLine(lines[i], max(1, s.width)))
	}
	return tea.NewView(strings.Join(out, "\n"))
}

func viewStr(v tea.View) string {
	if v.Content == nil {
		return ""
	}
	if r, ok := v.Content.(interface{ Render() string }); ok {
		return r.Render()
	}
	return fmt.Sprint(v.Content)
}

func clipLine(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	return styles.ClipANSI(text, maxWidth)
}

func truncate(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	return styles.ClipANSI(text, maxWidth)
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
