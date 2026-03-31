package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cloudboy-jh/bentotui/registry/bricks/badge"
	"github.com/cloudboy-jh/bentotui/registry/bricks/surface"
	"github.com/cloudboy-jh/bentotui/registry/recipes/vimstatus"
	"github.com/cloudboy-jh/bentotui/registry/rooms"
	"github.com/cloudboy-jh/bentotui/theme"
	"github.com/cloudboy-jh/bentotui/theme/styles"
	"github.com/cloudboy-jh/glib/internal/diffs"
	"github.com/cloudboy-jh/glib/internal/git"
	"github.com/cloudboy-jh/glib/internal/githubauth"
)

func (m *model) View() tea.View {
	t := theme.CurrentTheme()
	canvasColor := t.Background()

	if m.width == 0 {
		v := tea.NewView("")
		v.AltScreen = true
		v.BackgroundColor = canvasColor
		return v
	}

	dim := lipgloss.NewStyle().Foreground(t.TextMuted())
	bright := lipgloss.NewStyle().Foreground(t.Text())
	m.syncFooter()

	body := rooms.RenderFunc(func(width, height int) string {
		bodySurf := surface.New(width, height)
		bodySurf.Fill(canvasColor)
		switch m.mode {
		case modeProjects:
			m.drawProjectsView(bodySurf, height, t, dim, bright)
		case modeDiff:
			m.drawDiffView(bodySurf, width, height, t)
		case modeGit:
			m.drawGitView(bodySurf, width, height, t)
		case modePI:
			m.drawPIView(bodySurf, height, t)
		}
		return bodySurf.Render()
	})

	screen := rooms.Focus(m.width, m.height, body, m.footer)
	surf := surface.New(m.width, m.height)
	surf.Fill(canvasColor)
	surf.Draw(0, 0, screen)
	if m.dialogs != nil && m.dialogs.IsOpen() {
		surf.DrawCenter(viewString(m.dialogs.View()))
	}

	if m.prompt != promptNone {
		surf.DrawCenter(m.renderPrompt(t))
	}

	v := tea.NewView(surf.Render())
	v.AltScreen = true
	v.BackgroundColor = canvasColor
	return v
}

func (m *model) drawProjectsView(surf *surface.Surface, bodyH int, t theme.Theme, dim, bright lipgloss.Style) {
	if m.authStatus == githubauth.StatusAuth && m.picker == pickerRepos {
		m.drawRepoProjectsView(surf, bodyH, t, dim, bright)
		return
	}

	const (
		logoToCardGap = 1
		cardToHelpGap = 1
		helpToTipGap  = 1
		statusGap     = 1
	)

	wm := lipgloss.NewStyle().
		Foreground(t.TextAccent()).
		Bold(true).
		Render(glibWordmark)
	wmW := lipgloss.Width(wm)
	wmH := lipgloss.Height(wm)

	contentW := m.projectsContentWidth()

	header := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(m.icons.Projects + "  Projects")
	modeTag := string(m.picker)
	if m.authStatus != githubauth.StatusAuth {
		modeTag = "SIGN IN"
	}

	body := ""
	if m.authStatus != githubauth.StatusAuth {
		statusText := strings.ToLower(strings.ReplaceAll(m.authStatus, "_", " "))
		statusStyle := lipgloss.NewStyle().Foreground(t.TextMuted())
		if m.authStatus == githubauth.StatusPending {
			statusStyle = statusStyle.Foreground(t.Warning())
		}
		if m.authStatus == githubauth.StatusExpired {
			statusStyle = statusStyle.Foreground(t.Error())
		}

		headline := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("terminal workspace — git + diff + pi in one shell")
		statusLine := lipgloss.NewStyle().Bold(true).Foreground(t.Text()).Render("status: ") + statusStyle.Render(statusText)
		scopeLine := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("glib requests `repo` scope for private repository access")

		buttonLabel := "Sign in with GitHub (enter)"
		if m.authStatus == githubauth.StatusPending {
			buttonLabel = "Waiting for approval in browser"
		}
		button := lipgloss.NewStyle().
			Width(contentW).
			Align(lipgloss.Center).
			Background(t.BackgroundInteractive()).
			Foreground(t.TextInverse()).
			Bold(true).
			Render(buttonLabel)

		lines := []string{fitLine(headline, contentW), fitLine(statusLine, contentW), fitLine(scopeLine, contentW), "", button}
		if m.authStatus == githubauth.StatusPending {
			codeLine := lipgloss.NewStyle().Foreground(t.Warning()).Render("Code: " + m.authDevice.UserCode)
			urlLine := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("Open: " + m.authDevice.VerificationURI)
			remaining := ""
			if !m.authPollDeadline.IsZero() {
				left := max(0, int(time.Until(m.authPollDeadline).Seconds()))
				attempt := 1
				if m.authPollInterval > 0 {
					attempt = 1 + int(time.Since(m.authPollDeadline.Add(-time.Duration(max(1, m.authDevice.ExpiresIn))*time.Second)).Seconds())/max(1, m.authPollInterval)
				}
				remaining = fmt.Sprintf("Polling attempt %d • %02d:%02d remaining", max(1, attempt), left/60, left%60)
			}
			if remaining == "" {
				remaining = "Waiting for GitHub approval"
			}
			lines = append(lines, "", fitLine(codeLine, contentW), fitLine(urlLine, contentW), fitLine(remaining, contentW))
		}
		body = lipgloss.JoinVertical(lipgloss.Left, lines...)
	} else if m.picker == pickerClone {
		inputVal := strings.TrimSpace(m.inputBox.Value())
		if inputVal == "" {
			inputVal = "Paste git URL (https/ssh)..."
			inputVal = lipgloss.NewStyle().Foreground(t.TextMuted()).Render(inputVal)
		} else {
			inputVal = lipgloss.NewStyle().Foreground(t.Text()).Render(inputVal)
		}
		inputLine := lipgloss.NewStyle().
			Width(contentW).
			PaddingLeft(1).
			PaddingRight(1).
			Foreground(t.Text()).
			Render(inputVal)
		recentStr := "no recent projects"
		if len(m.recent) > 0 {
			show := m.recent
			if len(show) > 3 {
				show = show[:3]
			}
			recentStr = strings.Join(show, "   ")
		}
		recentStr = fitLine(recentStr, contentW)
		body = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(t.TextMuted()).Render(m.icons.Clone+"  clone url"),
			inputLine,
			"",
			lipgloss.NewStyle().Foreground(t.TextMuted()).Render(recentStr),
		)
	} else {
		root := truncateText(m.icons.Root+"  root: "+m.localDir, contentW)
		if len(m.localEntries) == 0 {
			empty := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("No directories")
			body = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(t.TextMuted()).Render(root),
				empty,
			)
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(t.TextMuted()).Render(root),
				m.renderLocalTree(contentW, t),
			)
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().Foreground(t.Text()).Render("mode: "+modeTag),
		body,
	)

	block := lipgloss.NewStyle().
		Background(t.BackgroundPanel()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus()).
		Padding(0, 1).
		Width(contentW).
		Render(content)
	blockW := lipgloss.Width(block)
	blockH := lipgloss.Height(block)

	kbdStr := ""
	if m.authStatus != githubauth.StatusAuth {
		kbdStr = dim.Render("sign in ") + bright.Render("enter") +
			dim.Render("   retry ") + bright.Render("r") +
			dim.Render("   clear token ") + bright.Render("l")
	} else if m.picker == pickerClone {
		kbdStr = dim.Render("toggle picker ") + bright.Render("tab") +
			dim.Render("   submit ") + bright.Render("enter")
	} else {
		kbdStr = dim.Render("move ") + bright.Render("j/k or arrows") +
			dim.Render("   open/select ") + bright.Render("enter") +
			dim.Render("   expand ") + bright.Render("l") +
			dim.Render("   collapse ") + bright.Render("h") +
			dim.Render("   parent ") + bright.Render("backspace") +
			dim.Render("   mode ") + bright.Render("tab")
	}
	kbdW := lipgloss.Width(kbdStr)

	dot := lipgloss.NewStyle().Foreground(t.Info()).Render(m.icons.Dot)
	tipStr := dot + dim.Render("  terminal workspace. git + pi + diff.")
	tipW := lipgloss.Width(tipStr)

	status := ""
	if m.projectPath != "" {
		status = "project: " + m.currentRepoLabel()
	}
	if m.statusMessage != "" {
		if status != "" {
			status += " | "
		}
		status += m.statusMessage
	}
	statusW := lipgloss.Width(status)

	stackH := wmH + logoToCardGap + blockH + cardToHelpGap + 1 + helpToTipGap + 1
	if status != "" {
		stackH += statusGap + 1
	}
	topPad := max(0, (bodyH-stackH)/2)
	y := topPad

	surf.Draw(max(0, (m.width-wmW)/2), y, wm)
	y += wmH + logoToCardGap
	surf.Draw(max(0, (m.width-blockW)/2), y, block)
	y += blockH + cardToHelpGap
	surf.Draw(max(0, (m.width-kbdW)/2), y, kbdStr)
	y += 1 + helpToTipGap
	surf.Draw(max(0, (m.width-tipW)/2), y, tipStr)
	if status != "" {
		y += 1 + statusGap
		surf.Draw(max(0, (m.width-statusW)/2), y, lipgloss.NewStyle().Foreground(t.TextMuted()).Render(status))
	}
}

var repoLoadingMessages = []string{
	"loading repos…",
	"fetching from GitHub…",
	"expanding workspace…",
	"organizing list…",
	"almost there…",
}

func (m *model) drawRepoProjectsView(surf *surface.Surface, bodyH int, t theme.Theme, dim, bright lipgloss.Style) {
	contentW := m.projectsContentWidth()
	listH := clamp(bodyH/3, 6, 14)
	rowW := max(8, contentW)
	base := lipgloss.NewStyle().Background(t.BackgroundPanel()).Foreground(t.Text())
	active := base.Copy().Background(t.BackgroundInteractive()).Foreground(t.TextInverse()).Bold(true)
	muted := base.Copy().Foreground(t.TextMuted())
	padRow := func(v string) string {
		v = fitLine(v, rowW)
		if lipgloss.Width(v) < rowW {
			v += strings.Repeat(" ", rowW-lipgloss.Width(v))
		}
		return v
	}
	lines := make([]string, 0, listH)
	displayRows := m.repoDisplayRows()
	if m.reposLoading && len(displayRows) == 0 {
		idx := (int(time.Now().UnixMilli()/600) % len(repoLoadingMessages))
		msg := repoLoadingMessages[idx]
		lines = append(lines, base.Copy().Foreground(t.TextMuted()).Render(padRow(msg)))
		for len(lines) < listH {
			lines = append(lines, base.Render(padRow("")))
		}
	} else if len(displayRows) == 0 {
		lines = append(lines, base.Copy().Foreground(t.TextMuted()).Render(padRow("No repositories found. Press r to refresh.")))
		for len(lines) < listH {
			lines = append(lines, base.Render(padRow("")))
		}
	} else {
		total := len(displayRows)
		start := windowStart(m.repoCursor, listH, total)
		end := min(total, start+listH)
		for i := start; i < end; i++ {
			repo := displayRows[i]
			prefix := "  "
			marker := "  "
			style := base
			if i == m.repoCursor {
				prefix = "> "
				style = active
			}
			if i == start && start > 0 {
				marker = "^ "
			} else if i == end-1 && end < total {
				marker = "v "
			}
			name := repo.FullName
			if repo.Private {
				name += " (private)"
			}
			if m.workspace != nil {
				if m.workspace.RepoExists(repo.FullName) {
					name += "  [local]"
				} else {
					name += "  [clone needed]"
				}
			}
			lines = append(lines, style.Render(padRow(marker+prefix+name)))
		}
		if m.reposLoading {
			lines = append(lines, muted.Render(padRow("loading more repos...")))
		}
		for len(lines) < listH {
			lines = append(lines, base.Render(padRow("")))
		}
	}

	header := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(m.icons.Projects + "  Repositories")
	meta := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(
		fmt.Sprintf("backend: %s  repos: %d  page: %d", m.workspaceKind, len(displayRows), m.repoPage),
	)
	filter := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("filter: ") + lipgloss.NewStyle().Foreground(t.Text()).Render(fitLine(m.repoFilter, max(8, rowW-8)))
	if strings.TrimSpace(m.repoFilter) == "" {
		filter = lipgloss.NewStyle().Foreground(t.TextMuted()).Render("filter: type to search owner/name")
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, meta, filter, "", strings.Join(lines, "\n"))
	block := lipgloss.NewStyle().
		Background(t.BackgroundPanel()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus()).
		Padding(0, 1).
		Width(contentW).
		Render(content)
	blockW := lipgloss.Width(block)
	blockH := lipgloss.Height(block)
	wm := lipgloss.NewStyle().
		Foreground(t.TextAccent()).
		Bold(true).
		Render(glibWordmark)
	wmW := lipgloss.Width(wm)
	wmH := lipgloss.Height(wm)

	actionKbd := dim.Render("move ") + bright.Render("j/k") + dim.Render("  filter ") + bright.Render("type/backspace") + dim.Render("  actions ") + bright.Render("enter") + dim.Render("  backend ") + bright.Render("b") + dim.Render("  refresh ") + bright.Render("r")
	actionBar := ""
	if m.repoActionOpen {
		item := func(label string, active bool) string {
			st := lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(t.Text())
			if active {
				st = st.Background(t.BackgroundInteractive()).Foreground(t.TextInverse()).Bold(true)
			}
			return st.Render(label)
		}
		diffItem := item(fmt.Sprintf("%s Diff — review changes", m.icons.Diff), m.repoActionCursor == 0)
		gitItem := item(fmt.Sprintf("%s Git — stage & commit", m.icons.Git), m.repoActionCursor == 1)
		piItem := item(fmt.Sprintf("%s Pi — AI agent", m.icons.PI), m.repoActionCursor == 2)
		barLine := diffItem + "   " + gitItem + "   " + piItem
		barW := lipgloss.Width(barLine) + 2
		actionBar = lipgloss.NewStyle().
			Background(t.BackgroundPanel()).
			Foreground(t.Text()).
			Padding(0, 1).
			Width(barW).
			Render(barLine)
		actionKbd = dim.Render("choose ") + bright.Render("h/l or arrows") + dim.Render("  run ") + bright.Render("enter") + dim.Render("  back ") + bright.Render("esc")
	}

	stackH := wmH + 1 + blockH + 1 + 1
	if actionBar != "" {
		stackH += lipgloss.Height(actionBar) + 1
	}
	stackY := max(0, (bodyH-stackH)/2)
	x := max(0, (m.width-blockW)/2)
	y := stackY

	surf.Draw(max(0, (m.width-wmW)/2), y, wm)
	y += wmH + 1
	surf.Draw(x, y, block)
	y += blockH + 1

	if actionBar != "" {
		surf.Draw(max(0, (m.width-lipgloss.Width(actionBar))/2), y, actionBar)
		y += lipgloss.Height(actionBar) + 1
	}

	surf.Draw(max(0, (m.width-lipgloss.Width(actionKbd))/2), y, actionKbd)
}

func (m *model) drawDiffView(surf *surface.Surface, bodyW, bodyH int, t theme.Theme) {
	if !useMockViews && m.projectPath == "" {
		surf.Draw(2, 1, "No project selected. Use p to choose a project.")
		return
	}
	if m.diffView == diffViewHistory {
		r := diffs.RenderHistory(m.diffHistory, m.diffHistoryCur, m.projectsContentWidth(), bodyW, bodyH, m.icons.Diff, t)
		surf.Draw(r.WordmarkX, r.Y, r.Wordmark)
		surf.Draw(r.BlockX, r.Y+lipgloss.Height(r.Wordmark)+1, r.Block)
		return
	}
	surf.Draw(0, 0, diffs.RenderOpen(m.diffViewer, bodyW, bodyH, t))
}

func (m *model) drawGitView(surf *surface.Surface, bodyW, bodyH int, t theme.Theme) {
	if !useMockViews && m.projectPath == "" {
		surf.Draw(2, 1, "No project selected. Use p to choose a project.")
		return
	}

	switch m.gitView {
	case gitViewBranches:
		surf.Draw(0, 0, git.RenderBranchesView(m.gitBranches, m.gitCurrentBranch, m.gitBranchCursor, bodyW, bodyH, t))
	case gitViewStash:
		surf.Draw(0, 0, git.RenderStashView(m.gitStash, m.gitStashCursor, bodyW, bodyH, t))
	case gitViewLog:
		surf.Draw(0, 0, git.RenderLogView(m.gitLog, m.gitLogCursor, bodyW, bodyH, t))
	default:
		surf.Draw(0, 0, git.RenderStatusView(m.git, bodyW, bodyH, t))
	}
}

func (m *model) drawPIView(surf *surface.Surface, bodyH int, t theme.Theme) {
	if m.width <= 2 || bodyH <= 2 {
		return
	}
	vw, vh := m.piViewportSize()
	b := lipgloss.RoundedBorder()
	borderStyle := lipgloss.NewStyle().Foreground(t.BorderFocus())
	top := borderStyle.Render(b.TopLeft + strings.Repeat(b.Top, vw) + b.TopRight)
	bottom := borderStyle.Render(b.BottomLeft + strings.Repeat(b.Bottom, vw) + b.BottomRight)
	surf.Draw(0, 0, top)
	for y := 0; y < vh; y++ {
		middle := borderStyle.Render(b.Left) + strings.Repeat(" ", vw) + borderStyle.Render(b.Right)
		surf.Draw(0, y+1, middle)
	}
	surf.Draw(0, vh+1, bottom)

	headerRows := 3
	inputRows := m.piInputRows(max(8, vw-4))
	slashRows := m.piui.SlashRows(8)
	slashPanelH := 0
	if m.piui.SlashActive() && len(slashRows) > 0 {
		slashPanelH = min(len(slashRows)+1, 8)
	}
	widgetH := 0
	if len(m.piui.WidgetLines) > 0 {
		widgetH = min(3, len(m.piui.WidgetLines))
	}
	historyH := max(1, vh-headerRows-inputRows-slashPanelH-widgetH)
	if m.piui.Viewport.Width() != vw || m.piui.Viewport.Height() != historyH {
		m.piui.Viewport.SetWidth(vw)
		m.piui.Viewport.SetHeight(historyH)
		m.refreshAgentViewport()
	}

	repoLabel := m.currentRepoLabel()
	headerRow := m.piui.HeaderLine(m.icons.PI, repoLabel, max(1, vw-2), t)
	headerFill := lipgloss.NewStyle().Background(t.BackgroundPanel()).Width(vw).Render("")
	headerMid := lipgloss.NewStyle().
		Background(t.BackgroundPanel()).
		Foreground(t.Text()).
		PaddingLeft(2).
		Width(vw).
		Render(fitLine(headerRow, max(1, vw-2)))
	surf.Draw(1, 1, headerFill)
	surf.Draw(1, 2, headerMid)
	surf.Draw(1, 3, headerFill)

	view := strings.Split(m.piui.Viewport.View(), "\n")
	historyStartY := 1 + headerRows
	for y := 0; y < min(historyH, len(view)); y++ {
		surf.Draw(1, historyStartY+y, fitLine(view[y], vw))
	}

	panelStartY := historyStartY + historyH
	if widgetH > 0 {
		for i := 0; i < widgetH; i++ {
			line := ""
			if i < len(m.piui.WidgetLines) {
				line = m.piui.WidgetLines[i]
			}
			styled := lipgloss.NewStyle().
				Background(t.BackgroundPanel()).
				Foreground(t.TextMuted()).
				PaddingLeft(1).
				PaddingRight(1).
				Width(max(1, vw-2)).
				Render(fitLine(line, max(8, vw-4)))
			surf.Draw(1, panelStartY+i, fitLine(styled, vw))
		}
		panelStartY += widgetH
	}
	if slashPanelH > 0 {
		cmdW := clamp(vw/3, 14, 28)
		for i := 0; i < min(len(slashRows), slashPanelH-1); i++ {
			row := slashRows[i]
			left := fitLine(row.Command, cmdW)
			right := fitLine(row.Description, max(8, vw-cmdW-2))
			line := left + strings.Repeat(" ", max(1, cmdW-lipgloss.Width(left)+2)) + right
			style := lipgloss.NewStyle().
				Background(t.BackgroundPanel()).
				Foreground(t.TextMuted()).
				PaddingLeft(1).
				PaddingRight(1).
				Width(max(1, vw-2))
			if row.Selected {
				style = style.Background(t.SelectionBG()).Foreground(t.SelectionFG()).Bold(true)
			}
			surf.Draw(1, panelStartY+i, fitLine(style.Render(line), vw))
		}
		hint := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("tab autocomplete  enter run  esc close")
		surf.Draw(1, panelStartY+slashPanelH-1, fitLine(hint, vw))
	}

	spin := " "
	if frame := m.piui.SpinnerFrame(); frame != "" {
		spin = frame
	}
	inputCore := fitLine(viewString(m.piui.Input.View()), max(8, vw-6))
	inputContent := spin + " " + inputCore

	contentW := max(8, vw-2)
	mkRow := func(content string) string {
		return lipgloss.NewStyle().
			Background(t.InputBG()).
			Foreground(t.InputFG()).
			PaddingLeft(1).
			PaddingRight(1).
			Width(contentW).
			Render(content)
	}
	blank := lipgloss.NewStyle().Background(t.InputBG()).Width(contentW + 2).Render(" ")
	rows := make([]string, 0, inputRows)
	rows = append(rows, blank)
	extraRows := max(0, inputRows-3)
	for i := 0; i < extraRows; i++ {
		rows = append(rows, mkRow(""))
	}
	rows = append(rows, mkRow(inputContent))
	rows = append(rows, blank)
	inputBlock := lipgloss.JoinVertical(lipgloss.Left, rows...)
	inputBox := lipgloss.NewStyle().
		Background(t.InputBG()).
		Border(lipgloss.Border{Left: "┃"}, false, false, false, true).
		BorderForeground(t.BorderFocus()).
		Width(vw - 1).
		Render(inputBlock)
	inputStartY := panelStartY + slashPanelH
	for i, line := range strings.Split(inputBox, "\n") {
		y := inputStartY + i
		if y > vh {
			break
		}
		surf.Draw(1, y, fitLine(line, vw))
	}

	if m.piui.Modal.Active {
		modalW := min(max(32, vw*2/3), vw-2)
		modalH := min(max(6, len(m.piui.ModalLines(modalW-4))+4), vh-1)
		offX := 1 + max(0, (vw-modalW)/2)
		offY := 1 + max(0, (vh-modalH)/2)
		mb := lipgloss.NormalBorder()
		mStyle := lipgloss.NewStyle().Foreground(t.BorderFocus()).Background(t.BackgroundPanel())
		topM := mStyle.Render(mb.TopLeft + strings.Repeat(mb.Top, modalW-2) + mb.TopRight)
		surf.Draw(offX, offY, topM)
		for y := 1; y < modalH-1; y++ {
			surf.Draw(offX, offY+y, mStyle.Render(mb.Left)+strings.Repeat(" ", modalW-2)+mStyle.Render(mb.Right))
		}
		surf.Draw(offX, offY+modalH-1, mStyle.Render(mb.BottomLeft+strings.Repeat(mb.Bottom, modalW-2)+mb.BottomRight))
		for i, line := range m.piui.ModalLines(modalW - 4) {
			if i >= modalH-2 {
				break
			}
			surf.Draw(offX+2, offY+1+i, fitLine(line, modalW-4))
		}
	}
}

func (m *model) syncFooter() {
	cfg := vimstatus.Config{
		Mode:      m.footerModeLabel(),
		Branch:    m.footerRepoLabel(),
		Context:   "",
		ShowClock: true,
	}

	if cfg.Branch == "" {
		cfg.Branch = "glib"
	}

	switch m.mode {
	case modeProjects:
		if m.authStatus != githubauth.StatusAuth {
			cfg.Context = m.icons.Projects + " enter sign in  q quit"
			cfg.Scroll = strings.ToLower(m.authStatus)
		} else if m.picker == pickerRepos {
			if m.repoActionOpen {
				cfg.Context = m.icons.Projects + " h/l choose  enter run  esc back"
			} else {
				cfg.Context = m.icons.Projects + " j/k move  enter actions  esc back"
			}
			cfg.Position = ""
			cfg.Scroll = string(m.workspaceKind)
		} else {
			cfg.Context = m.icons.Projects + " j/k move  enter open  tab picker"
			if len(m.localRows) > 0 {
				cfg.Position = fmt.Sprintf("%d/%d", min(len(m.localRows), m.localCursor+1), len(m.localRows))
			}
		}
		if m.piSessionActiveForRepo(m.projectPath) {
			cfg.Scroll = "● pi active"
		}
	case modeDiff:
		if m.diffView == diffViewHistory {
			cfg.Context = m.icons.Diff + " j/k move  enter open  esc projects"
			cfg.Position = ""
		} else {
			cfg.Context = m.icons.Diff + " j/k scroll  n/N file  i send to pi  esc history"
		}
		if m.diffView == diffViewOpen && m.diffViewer != nil {
			st := m.diffViewer.State()
			cfg.Position = fmt.Sprintf("%d/%d", st.Scroll+1, st.MaxScroll+1)
			cfg.Scroll = fmt.Sprintf("file %d/%d", st.ActiveFile+1, max(1, st.FileCount))
		}
	case modeGit:
		switch m.gitView {
		case gitViewBranches:
			cfg.Context = m.icons.Git + " j/k move  enter switch  esc back"
			cfg.Position = fmt.Sprintf("%d/%d", min(len(m.gitBranches), m.gitBranchCursor+1), max(1, len(m.gitBranches)))
		case gitViewStash:
			cfg.Context = m.icons.Git + " j/k move  esc back"
			cfg.Position = fmt.Sprintf("%d/%d", min(len(m.gitStash), m.gitStashCursor+1), max(1, len(m.gitStash)))
		case gitViewLog:
			cfg.Context = m.icons.Git + " j/k move  enter open diff  esc back"
			cfg.Position = fmt.Sprintf("%d/%d", min(len(m.gitLog), m.gitLogCursor+1), max(1, len(m.gitLog)))
		default:
			cfg.Context = m.icons.Git + " j/k move  s stage  u unstage  c commit"
			rows := m.git.Rows()
			if len(rows) > 0 {
				cfg.Position = fmt.Sprintf("%d/%d", min(len(rows), m.git.Cursor+1), len(rows))
			}
			cfg.Scroll = fmt.Sprintf("+%d -%d", m.git.AddedTotal, m.git.DeletedTotal)
		}
	case modePI:
		st := m.piui.FooterState("", "")
		cfg.Context = st.Context
		cfg.Scroll = st.Scroll
		cfg.Position = st.Position
	}

	hotkeys := m.footerHotkeyCluster()
	if strings.TrimSpace(cfg.Scroll) == "" {
		cfg.Scroll = hotkeys
	} else {
		cfg.Scroll = hotkeys + "  ·  " + cfg.Scroll
	}

	m.footer.SetTheme(theme.CurrentTheme())
	m.footer.SetConfig(cfg)
}

func (m *model) footerHotkeyCluster() string {
	cycleKey := m.footerBadge("CTRL+SPACE", badge.VariantAccent)
	cmdKey := m.footerBadge("CTRL+O", badge.VariantInfo)

	if m.width < 88 {
		return cycleKey
	}
	if m.width < 116 {
		return cycleKey + " " + cmdKey
	}
	muted := lipgloss.NewStyle().Foreground(theme.CurrentTheme().FooterMuted())
	return cycleKey + " " + muted.Render("cycle") + "  " + cmdKey + " " + muted.Render("cmds")
}

func (m *model) footerBadge(text string, variant badge.Variant) string {
	b := badge.New(text)
	b.SetTheme(theme.CurrentTheme())
	b.SetBold(true)
	b.SetVariant(variant)
	return viewString(b.View())
}

func (m *model) footerModeLabel() string {
	switch m.mode {
	case modeProjects:
		return m.icons.Projects + " " + string(m.mode)
	case modeDiff:
		return m.icons.Diff + " " + string(m.mode)
	case modeGit:
		return m.icons.Git + " " + string(m.mode)
	case modePI:
		return "Pi"
	default:
		return string(m.mode)
	}
}

func (m *model) renderPrompt(t theme.Theme) string {
	availW := max(16, m.width-4)
	boxW := m.width / 2
	if boxW < 40 {
		boxW = 40
	}
	if boxW > availW {
		boxW = availW
	}

	if m.prompt == promptCommitView {
		boxW = clamp(m.width*2/3, 48, min(availW, 90))
	}

	bodyW := max(10, boxW-6)

	rowStyle := lipgloss.NewStyle().Background(t.BackgroundPanel()).Width(bodyW)
	title := rowStyle.Copy().Foreground(t.TextAccent()).Bold(true).Render(fitLine(m.promptTitle, bodyW))
	hint := rowStyle.Copy().Foreground(t.TextMuted()).Render(fitLine(m.promptHint, bodyW))
	blank := rowStyle.Render("")

	body := ""
	if m.prompt == promptError {
		body = rowStyle.Copy().Foreground(t.Error()).Render(fitMultiline(m.errorText, bodyW, 4))
	} else if m.prompt == promptCommitView {
		maxLines := clamp(m.bodyHeight()-10, 6, 30)
		lines := wrapPlainText(m.promptBody, bodyW)
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		rendered := make([]string, len(lines))
		for i, l := range lines {
			rendered[i] = rowStyle.Copy().Foreground(t.Text()).Render(fitLine(l, bodyW))
		}
		body = strings.Join(rendered, "\n")
	} else if m.prompt == promptTheme {
		body = viewString(m.themePicker.View())
	} else if m.prompt == promptModelPick {
		body = viewString(m.modelPicker.View())
	} else {
		body = viewString(m.promptInput.View())
	}
	box := lipgloss.NewStyle().
		Width(boxW).
		Background(t.BackgroundPanel()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus()).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, hint, blank, body))
	return box
}

func (m *model) bodyHeight() int {
	return max(0, m.height-1)
}

func (m *model) piViewportSize() (int, int) {
	return max(1, m.width-2), max(1, m.bodyHeight()-2)
}

func (m *model) piInputRows(contentWidth int) int {
	if contentWidth <= 0 {
		contentWidth = 32
	}
	text := strings.TrimSpace(m.piui.Input.Value())
	if text == "" {
		return 3
	}
	lines := wrapPlainText(text, contentWidth)
	contentRows := clamp(len(lines), 1, 4)
	return contentRows + 2
}

func (m *model) projectsPanelWidth() int {
	target := m.width * 40 / 100
	target = min(target, 62)
	target = max(target, 42)
	target = min(target, max(30, m.width-6))
	return target
}

func (m *model) projectsContentWidth() int {
	return max(24, m.projectsPanelWidth()-4)
}

func (m *model) renderLocalTree(contentW int, t theme.Theme) string {
	if len(m.localRows) == 0 {
		return lipgloss.NewStyle().Foreground(t.TextMuted()).Render("No entries")
	}
	start := clamp(m.localScroll, 0, max(0, len(m.localRows)-m.localListH))
	end := min(len(m.localRows), start+m.localListH)
	lineW := max(12, contentW)
	baseStyle := lipgloss.NewStyle().
		Background(t.BackgroundPanel()).
		Foreground(t.Text()).
		Width(lineW)
	activeStyle := baseStyle.Copy().
		Background(t.Info()).
		Foreground(t.TextInverse()).
		Bold(true)
	lines := make([]string, 0, m.localListH+1)
	for row := 0; row < m.localListH; row++ {
		i := start + row
		if i >= end {
			lines = append(lines, baseStyle.Render(""))
			continue
		}
		prefix := "  "
		lineStyle := baseStyle
		if i == m.localCursor {
			prefix = "> "
			lineStyle = activeStyle
		}
		marker := "  "
		if row == 0 && start > 0 {
			marker = "^ "
		} else if row == m.localListH-1 && end < len(m.localRows) {
			marker = "v "
		}
		line := fitLine(marker+prefix+m.localRows[i].Label, lineW)
		lines = append(lines, lineStyle.Render(line))
	}
	footer := lipgloss.NewStyle().
		Background(t.BackgroundPanel()).
		Foreground(t.TextMuted()).
		Width(lineW).
		Render(
			fmt.Sprintf("%d/%d", min(len(m.localRows), m.localCursor+1), len(m.localRows)),
		)
	lines = append(lines, footer)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func fitMultiline(text string, maxWidth, maxLines int) string {
	if maxWidth <= 0 || maxLines <= 0 {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	out := make([]string, 0, min(len(parts), maxLines))
	for i := 0; i < len(parts) && len(out) < maxLines; i++ {
		out = append(out, fitLine(parts[i], maxWidth))
	}
	if len(parts) > maxLines {
		out[maxLines-1] = fitLine(out[maxLines-1]+" ...", maxWidth)
	}
	return strings.Join(out, "\n")
}

func wrapPlainText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	text = strings.ReplaceAll(text, "\r", "")
	raw := strings.Split(text, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if line == "" {
			out = append(out, "")
			continue
		}
		for lipgloss.Width(line) > maxWidth {
			head, tail := splitByDisplayWidth(line, maxWidth)
			if head == "" {
				break
			}
			out = append(out, head)
			line = tail
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func splitByDisplayWidth(text string, maxWidth int) (string, string) {
	if maxWidth <= 0 || text == "" {
		return "", text
	}
	runes := []rune(text)
	width := 0
	idx := 0
	for idx < len(runes) {
		rw := lipgloss.Width(string(runes[idx]))
		if rw <= 0 {
			rw = 1
		}
		if width+rw > maxWidth {
			break
		}
		width += rw
		idx++
	}
	if idx == 0 {
		idx = 1
	}
	return string(runes[:idx]), string(runes[idx:])
}

func fitLine(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
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

func truncateText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	return styles.ClipANSI(text, maxWidth)
}
