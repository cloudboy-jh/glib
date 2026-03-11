package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cloudboy-jh/bentotui/registry/components/bar"
	"github.com/cloudboy-jh/bentotui/registry/components/input"
	selectx "github.com/cloudboy-jh/bentotui/registry/components/select"
	"github.com/cloudboy-jh/bentotui/registry/components/surface"
	"github.com/cloudboy-jh/bentotui/theme"
	"glib/internal/diffview"
	"glib/internal/gitops"
	"glib/internal/gitview"
	"glib/internal/opencode"
	"glib/internal/projects"
)

const version = "v0.1.0"

const wordmark = "" +
	"██████╗  ██╗     ██╗██████╗ \n" +
	"██╔════╝ ██║     ██║██╔══██╗\n" +
	"██║  ███╗██║     ██║██████╔╝\n" +
	"██║   ██║██║     ██║██╔══██╗\n" +
	"╚██████╔╝███████╗██║██████╔╝\n" +
	" ╚═════╝ ╚══════╝╚═╝╚═════╝ "

type appMode string

const (
	modeProjects appMode = "PROJECTS"
	modeDiff     appMode = "DIFF"
	modeGit      appMode = "GIT"
	modeOpencode appMode = "OPENCODE"
)

type pickerMode string

const (
	pickerLocal pickerMode = "LOCAL"
	pickerClone pickerMode = "CLONE"
)

type promptMode string

const (
	promptNone      promptMode = ""
	promptCloneDest promptMode = "clone_dest"
	promptCommit    promptMode = "commit"
	promptError     promptMode = "error"
	promptTheme     promptMode = "theme"
)

type gitRefreshMsg struct {
	Files  []gitops.File
	Log    []gitops.LogEntry
	Action string
	Err    error
}

type diffRefreshMsg struct {
	Lines      []string
	Anchors    []int
	Source     string
	CommitSHA  string
	ProjectDir string
	Err        error
}

type cloneDoneMsg struct {
	ProjectPath string
	Err         error
}

type opencodeChunkMsg struct {
	Data []byte
	Err  error
}

type opencodeDoneMsg struct {
	Err error
}

type localTreeRow struct {
	Path  string
	IsDir bool
	Label string
}

type iconSet struct {
	Projects string
	Clone    string
	Root     string
	Dot      string
	Opencode string
	Diff     string
	Git      string
	Quit     string
	Prompt   string
}

type model struct {
	statusBar     *bar.Model
	inputBox      *input.Model
	promptInput   *input.Model
	localPicker   *selectx.Model
	themePicker   *selectx.Model
	width         int
	height        int
	inputW        int
	mode          appMode
	picker        pickerMode
	projectPath   string
	recent        []string
	statusMessage string
	tunnelBanner  string
	errorText     string
	prompt        promptMode
	promptTitle   string
	promptHint    string
	pendingURL    string
	git           gitview.State
	diff          diffview.State
	opencode      opencode.State
	localDir      string
	localEntries  []projects.Entry
	localRows     []localTreeRow
	localCursor   int
	localScroll   int
	localListH    int
	localExpanded map[string]bool
	icons         iconSet
}

func NewModel() *model {
	inp := input.New()
	inp.SetPlaceholder("Search or open a project…")
	promptInp := input.New()
	promptInp.SetPlaceholder("Type value and press enter…")
	lp := selectx.New()
	lp.SetPlaceholder("")
	lp.Focus()
	lp.Open()
	tp := selectx.New()
	tp.SetPlaceholder("No themes")
	tp.Focus()
	tp.Open()

	cwd, _ := os.Getwd()
	m := &model{
		statusBar:     bar.New(),
		inputBox:      inp,
		promptInput:   promptInp,
		localPicker:   lp,
		themePicker:   tp,
		mode:          modeProjects,
		picker:        pickerLocal,
		localDir:      cwd,
		localExpanded: map[string]bool{},
		icons:         resolveIcons(),
		diff: diffview.State{
			ShowStaged: false,
			Source:     "working",
		},
	}
	_ = m.reloadLocalEntries()
	return m
}

func (m *model) Init() tea.Cmd {
	return m.inputBox.Focus()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.inputW = clamp(m.width*6/10, 50, 90)
		m.inputBox.SetSize(m.inputW-5, 1)
		m.promptInput.SetSize(max(20, m.width/2), 1)
		m.resizeLocalPicker()
		m.statusBar.SetSize(m.width, 1)
		return m, nil

	case theme.ThemeChangedMsg:
		return m, nil

	case gitRefreshMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.git.Files = msg.Files
		m.git.Log = msg.Log
		if msg.Action != "" {
			m.git.LastAction = msg.Action
		}
		m.git.Cursor = clamp(m.git.Cursor, 0, max(0, len(m.git.Files)-1))
		m.git.LogCursor = clamp(m.git.LogCursor, 0, max(0, len(m.git.Log)-1))
		return m, nil

	case diffRefreshMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.diff.Lines = msg.Lines
		m.diff.FileAnchors = msg.Anchors
		m.diff.FileAnchorPtr = 0
		m.diff.Scroll = 0
		m.diff.Source = msg.Source
		m.diff.CommitSHA = msg.CommitSHA
		m.diff.LoadedForDir = msg.ProjectDir
		return m, nil

	case cloneDoneMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.projectPath = msg.ProjectPath
		m.addRecent(msg.ProjectPath)
		m.statusMessage = "clone complete"
		m.mode = modeProjects
		return m, nil

	case opencodeChunkMsg:
		if msg.Err != nil {
			m.stopOpencode()
			m.showError("opencode stream closed")
			m.mode = modeProjects
			return m, nil
		}
		if len(msg.Data) > 0 {
			m.opencode.Buffer = append(m.opencode.Buffer, msg.Data...)
			if len(m.opencode.Buffer) > 200000 {
				m.opencode.Buffer = m.opencode.Buffer[len(m.opencode.Buffer)-200000:]
			}
			return m, m.readOpencodeChunkCmd()
		}
		return m, nil

	case opencodeDoneMsg:
		if msg.Err != nil {
			m.tunnelBanner = "opencode -> glib tunnel error"
			m.showError("opencode failed: " + msg.Err.Error())
			return m, nil
		}
		m.tunnelBanner = "opencode -> glib tunnel closed | back in glib"
		m.statusMessage = "returned from opencode"
		return m, nil

	case tea.KeyMsg:
		if m.tunnelBanner != "" && msg.String() != "o" {
			m.tunnelBanner = ""
		}
		if m.prompt != promptNone {
			return m, m.updatePrompt(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.stopOpencode()
			return m, tea.Quit
		case "tab":
			if m.mode == modeProjects {
				if m.picker == pickerLocal {
					m.picker = pickerClone
					m.inputBox.SetPlaceholder("Paste git URL (https/ssh)…")
				} else {
					m.picker = pickerLocal
					m.inputBox.SetPlaceholder("Search or open a project…")
					_ = m.reloadLocalEntries()
				}
				return m, nil
			}
		case "t":
			m.reloadThemeItems()
			m.openPrompt(promptTheme, "Theme", "j/k move, enter apply, esc cancel", "")
			return m, nil
		case "o":
			return m, m.startOpencodeCmd()
		case "d":
			m.mode = modeDiff
			return m, m.refreshDiffCmd("", "")
		case "g":
			m.mode = modeGit
			return m, m.refreshGitCmd()
		case "p":
			if m.mode == modeOpencode {
				m.stopOpencode()
			}
			m.mode = modeProjects
			return m, m.inputBox.Focus()
		}

		if m.mode == modeOpencode {
			switch msg.String() {
			case "esc", "ctrl+b":
				m.stopOpencode()
				m.mode = modeProjects
				return m, nil
			default:
				return m, m.forwardOpencodeInputCmd(msg)
			}
		}

		if m.mode == modeProjects {
			switch msg.String() {
			case "backspace":
				if m.picker == pickerLocal {
					parent := filepath.Dir(m.localDir)
					if parent != m.localDir {
						m.localDir = parent
						_ = m.reloadLocalEntries()
					}
					return m, nil
				}
			case "h", "left":
				if m.picker == pickerLocal {
					m.collapseLocalSelection()
					return m, nil
				}
			case "enter":
				if m.picker == pickerLocal {
					m.activateLocalSelection()
					return m, nil
				}

				val := strings.TrimSpace(m.inputBox.Value())
				if val == "" {
					return m, nil
				}
				m.inputBox.SetValue("")

				m.pendingURL = val
				defaultDest := projects.DefaultCloneDest(val)
				m.openPrompt(promptCloneDest, "Clone destination", "Enter destination path", defaultDest)
				return m, m.promptInput.Focus()
			case "j", "down":
				if m.picker == pickerLocal {
					m.moveLocalCursor(1)
					return m, nil
				}
			case "k", "up":
				if m.picker == pickerLocal {
					m.moveLocalCursor(-1)
					return m, nil
				}
			case "pgdown":
				if m.picker == pickerLocal {
					m.moveLocalCursor(max(1, m.localListH-2))
					return m, nil
				}
			case "pgup":
				if m.picker == pickerLocal {
					m.moveLocalCursor(-max(1, m.localListH-2))
					return m, nil
				}
			case "home":
				if m.picker == pickerLocal {
					m.localCursor = 0
					m.syncLocalScroll()
					return m, nil
				}
			case "end":
				if m.picker == pickerLocal {
					m.localCursor = max(0, len(m.localRows)-1)
					m.syncLocalScroll()
					return m, nil
				}
			case "l", "right":
				if m.picker == pickerLocal {
					m.expandLocalSelection()
					return m, nil
				}
			}

			if m.picker == pickerClone {
				u, cmd := m.inputBox.Update(msg)
				m.inputBox = u.(*input.Model)
				return m, cmd
			}
			return m, nil
		}

		if m.mode == modeDiff {
			switch msg.String() {
			case "j", "down":
				m.diff.Scroll = min(m.diff.Scroll+1, m.diffMaxScroll())
			case "k", "up":
				m.diff.Scroll = max(0, m.diff.Scroll-1)
			case "pgdown", "ctrl+f":
				m.diff.Scroll = min(m.diff.Scroll+max(1, m.diffViewRows()/2), m.diffMaxScroll())
			case "pgup", "ctrl+b":
				m.diff.Scroll = max(0, m.diff.Scroll-max(1, m.diffViewRows()/2))
			case "s":
				if m.diff.Source == "commit" {
					return m, nil
				}
				m.diff.ShowStaged = !m.diff.ShowStaged
				return m, m.refreshDiffCmd("", "")
			case "r":
				if m.diff.Source == "commit" {
					return m, m.refreshDiffCmd("commit", m.diff.CommitSHA)
				}
				return m, m.refreshDiffCmd("", "")
			case "]f":
				m.jumpDiffFile(1)
			case "[f":
				m.jumpDiffFile(-1)
			case "home":
				m.diff.Scroll = 0
			case "end":
				m.diff.Scroll = m.diffMaxScroll()
			}
			return m, nil
		}

		if m.mode == modeGit {
			switch msg.String() {
			case "j", "down":
				if m.git.FocusOnLog {
					m.git.LogCursor = min(m.git.LogCursor+1, max(0, len(m.git.Log)-1))
				} else {
					m.git.Cursor = min(m.git.Cursor+1, max(0, len(m.git.Files)-1))
				}
			case "k", "up":
				if m.git.FocusOnLog {
					m.git.LogCursor = max(0, m.git.LogCursor-1)
				} else {
					m.git.Cursor = max(0, m.git.Cursor-1)
				}
			case "tab":
				m.git.FocusOnLog = !m.git.FocusOnLog
			case "space":
				if !m.git.FocusOnLog {
					return m, m.toggleStageCmd()
				}
			case "c":
				m.openPrompt(promptCommit, "Commit", "Enter commit message", "")
				return m, m.promptInput.Focus()
			case "r":
				return m, m.refreshGitCmd()
			case "enter":
				if m.git.FocusOnLog {
					if len(m.git.Log) == 0 {
						return m, nil
					}
					sha := m.git.Log[m.git.LogCursor].SHA
					m.mode = modeDiff
					return m, m.refreshDiffCmd("commit", sha)
				}
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *model) View() tea.View {
	t := theme.CurrentTheme()
	canvasColor := lipgloss.Color(t.Surface.Canvas)

	if m.width == 0 {
		v := tea.NewView("")
		v.AltScreen = true
		v.BackgroundColor = canvasColor
		return v
	}

	bodyH := m.bodyHeight()

	surf := surface.New(m.width, m.height)
	surf.Fill(canvasColor)

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted))
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary))

	switch m.mode {
	case modeProjects:
		m.drawProjectsView(surf, bodyH, t, dim, bright)
	case modeDiff:
		m.drawDiffView(surf, bodyH, t)
	case modeGit:
		m.drawGitView(surf, bodyH, t)
	case modeOpencode:
		m.drawOpencodeView(surf, bodyH, t)
	}

	if m.prompt != promptNone {
		surf.DrawCenter(m.renderPrompt(t))
	}

	m.syncStatusBar()
	surf.Draw(0, m.height-1, viewString(m.statusBar.View()))

	v := tea.NewView(surf.Render())
	v.AltScreen = true
	v.BackgroundColor = canvasColor
	return v
}

func (m *model) drawProjectsView(surf *surface.Surface, bodyH int, t theme.Theme, dim, bright lipgloss.Style) {
	const (
		logoToCardGap = 1
		cardToHelpGap = 1
		helpToTipGap  = 1
		statusGap     = 1
	)

	wm := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text.Accent)).
		Bold(true).
		Render(wordmark)
	wmW := lipgloss.Width(wm)
	wmH := lipgloss.Height(wm)

	contentW := m.projectsContentWidth()

	header := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Bold(true).Render(m.icons.Projects + "  Projects")
	modeTag := string(m.picker)

	body := ""
	if m.picker == pickerClone {
		inputVal := strings.TrimSpace(m.inputBox.Value())
		if inputVal == "" {
			inputVal = "Paste git URL (https/ssh)..."
			inputVal = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(inputVal)
		} else {
			inputVal = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary)).Render(inputVal)
		}
		inputLine := lipgloss.NewStyle().
			Width(contentW).
			PaddingLeft(1).
			PaddingRight(1).
			Foreground(lipgloss.Color(t.Text.Primary)).
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
			lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(m.icons.Clone+"  clone url"),
			inputLine,
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(recentStr),
		)
	} else {
		root := truncateText(m.icons.Root+"  root: "+m.localDir, contentW)
		if len(m.localEntries) == 0 {
			empty := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("No directories")
			body = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(root),
				empty,
			)
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(root),
				m.renderLocalTree(contentW, t),
			)
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary)).Render("mode: "+modeTag),
		body,
	)

	block := lipgloss.NewStyle().
		Background(lipgloss.Color(t.Surface.Panel)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Border.Focus)).
		Padding(0, 1).
		Width(contentW).
		Render(content)
	blockW := lipgloss.Width(block)
	blockH := lipgloss.Height(block)

	kbdStr := ""
	if m.picker == pickerClone {
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

	dot := lipgloss.NewStyle().Foreground(lipgloss.Color(t.State.Info)).Render(m.icons.Dot)
	tipStr := dot + dim.Render("  terminal workspace. git + agent + diff.")
	tipW := lipgloss.Width(tipStr)

	status := ""
	if m.projectPath != "" {
		status = "project: " + m.projectPath
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
		surf.Draw(max(0, (m.width-statusW)/2), y, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(status))
	}
}

func (m *model) drawDiffView(surf *surface.Surface, bodyH int, t theme.Theme) {
	title := "git diff"
	if m.diff.Source == "commit" {
		title = "commit diff " + m.diff.CommitSHA
	} else if m.diff.ShowStaged {
		title = "git diff --staged"
	}

	if m.projectPath == "" {
		surf.Draw(2, 1, "No project selected. Use p to choose a project.")
		return
	}

	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text.Accent)).
		Bold(true).
		Render(title + "  |  " + filepath.Base(m.projectPath))
	surf.Draw(2, 1, header)

	rangeStart, rangeEnd := m.diffRange()
	meta := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(
		fmt.Sprintf("lines: %d  files: %d  scroll: %d", rangeEnd-rangeStart, len(m.diff.FileAnchors), m.diff.Scroll),
	)
	surf.Draw(2, 2, meta)

	if len(m.diff.Lines) == 0 {
		surf.Draw(2, 4, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("No diff output."))
		return
	}

	listX := 2
	listY := 4
	listW := 0
	if len(m.diff.FileAnchors) > 0 && m.width >= 90 {
		listW = 28
	}

	if listW > 0 {
		listTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("files")
		surf.Draw(listX, listY, listTitle)
		rows := max(1, bodyH-6)
		start := windowStart(m.diff.FileAnchorPtr, rows, len(m.diff.FileAnchors))
		end := min(len(m.diff.FileAnchors), start+rows)
		for i := start; i < end; i++ {
			line := fitLine(diffFileLabel(m.diff.Lines[m.diff.FileAnchors[i]]), listW-3)
			if i == m.diff.FileAnchorPtr {
				line = lipgloss.NewStyle().Background(lipgloss.Color(t.Surface.Elevated)).Foreground(lipgloss.Color(t.Text.Accent)).Render(line)
			}
			surf.Draw(listX, listY+1+(i-start), line)
		}
	}

	viewX := 2
	if listW > 0 {
		viewX = listX + listW + 2
	}
	viewW := max(20, m.width-viewX-2)
	viewH := m.diffViewRows()
	start := rangeStart + m.diff.Scroll
	end := min(rangeEnd, start+viewH)
	y := 4
	for i := start; i < end; i++ {
		surf.Draw(viewX, y, diffview.StyleLine(fitLine(m.diff.Lines[i], viewW), t))
		y++
	}

	help := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("j/k move  pgup/pgdown jump  [f ]f file  home/end  s staged  r refresh")
	surf.Draw(2, bodyH-1, help)
}

func (m *model) drawGitView(surf *surface.Surface, bodyH int, t theme.Theme) {
	if m.projectPath == "" {
		surf.Draw(2, 1, "No project selected. Use p to choose a project.")
		return
	}

	header := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Bold(true).
		Render("git status + log  |  " + filepath.Base(m.projectPath))
	surf.Draw(2, 1, header)

	leftW := max(24, (m.width-6)/2)
	rightX := leftW + 4

	statusSummary := fmt.Sprintf("status (%d)", len(m.git.Files))
	logSummary := fmt.Sprintf("log (%d)", len(m.git.Log))
	if m.git.FocusOnLog {
		surf.Draw(2, 2, statusSummary)
		surf.Draw(rightX, 2, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Render(logSummary))
	} else {
		surf.Draw(2, 2, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Render(statusSummary))
		surf.Draw(rightX, 2, logSummary)
	}

	maxRows := max(1, bodyH-6)
	statusStart := windowStart(m.git.Cursor, maxRows, len(m.git.Files))
	statusEnd := min(len(m.git.Files), statusStart+maxRows)
	for i := statusStart; i < statusEnd; i++ {
		f := m.git.Files[i]
		prefix := "  "
		if i == m.git.Cursor && !m.git.FocusOnLog {
			prefix = "> "
		}
		status := fmt.Sprintf("%c%c", f.X, f.Y)
		line := fmt.Sprintf("%s%s %s", prefix, status, fitLine(f.Path, leftW-6))
		surf.Draw(2, 3+(i-statusStart), gitview.StyleStatusLine(line, f, t, i == m.git.Cursor && !m.git.FocusOnLog))
	}
	if len(m.git.Files) == 0 {
		surf.Draw(2, 3, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("Working tree clean."))
	}

	logStart := windowStart(m.git.LogCursor, maxRows, len(m.git.Log))
	logEnd := min(len(m.git.Log), logStart+maxRows)
	for i := logStart; i < logEnd; i++ {
		l := m.git.Log[i]
		prefix := "  "
		if i == m.git.LogCursor && m.git.FocusOnLog {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s %s", prefix, l.SHA, fitLine(l.Subject, max(16, m.width-rightX-6)))
		surf.Draw(rightX, 3+(i-logStart), gitview.StyleLogLine(line, t, i == m.git.LogCursor && m.git.FocusOnLog))
	}
	if len(m.git.Log) == 0 {
		surf.Draw(rightX, 3, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("No commits yet."))
	}

	help := "tab switch pane  j/k move  space stage/unstage  enter show commit  c commit  r refresh"
	if m.git.LastAction != "" {
		help += "  |  " + m.git.LastAction
	}
	surf.Draw(2, bodyH-1, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(help))
}

func (m *model) drawOpencodeView(surf *surface.Surface, bodyH int, t theme.Theme) {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Bold(true).Render("opencode")
	surf.Draw(2, 1, header)

	if !m.opencode.Running {
		surf.Draw(2, 3, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("opencode is not running."))
		return
	}

	lines := strings.Split(strings.ReplaceAll(string(m.opencode.Buffer), "\r", ""), "\n")
	if len(lines) > bodyH-3 {
		lines = lines[len(lines)-(bodyH-3):]
	}
	y := 3
	lineW := max(10, m.width-4)
	for _, line := range lines {
		surf.Draw(2, y, fitLine(line, lineW))
		y++
		if y >= bodyH {
			break
		}
	}
}

func (m *model) syncStatusBar() {
	left := fmt.Sprintf("%s o opencode   %s d diff   %s g git   %s p projects   t theme   %s q quit", m.icons.Opencode, m.icons.Diff, m.icons.Git, m.icons.Projects, m.icons.Quit)
	if m.mode == modeOpencode {
		left = fmt.Sprintf("%s esc return   t theme   %s q quit", m.icons.Prompt, m.icons.Quit)
	}
	if m.tunnelBanner != "" {
		left = m.tunnelBanner
	}
	projectLabel := "no-project"
	if m.projectPath != "" {
		projectLabel = filepath.Base(m.projectPath)
	}
	right := fmt.Sprintf("%s  %s  glib %s", string(m.mode), projectLabel, version)
	if lipgloss.Width(right) > max(8, m.width-2) {
		right = truncateText(right, max(8, m.width-2))
	}
	leftMax := max(0, m.width-lipgloss.Width(right)-1)
	if lipgloss.Width(left) > leftMax {
		left = truncateText(left, leftMax)
	}
	m.statusBar.SetCards(nil)
	m.statusBar.SetLeft(left)
	m.statusBar.SetRight(right)
}

func (m *model) updatePrompt(msg tea.KeyMsg) tea.Cmd {
	if m.prompt == promptTheme {
		u, cmd := m.themePicker.Update(msg)
		m.themePicker = u.(*selectx.Model)

		switch msg.String() {
		case "esc":
			m.closePrompt()
			return cmd
		case "enter":
			sel, ok := m.themePicker.Selected()
			if !ok {
				return cmd
			}
			if _, err := theme.SetTheme(sel.Value); err != nil {
				m.showError(err.Error())
				return cmd
			}
			m.statusMessage = "theme: " + sel.Label
			m.closePrompt()
			return cmd
		}

		return cmd
	}

	switch msg.String() {
	case "esc":
		m.closePrompt()
		return nil
	case "enter":
		val := strings.TrimSpace(m.promptInput.Value())
		switch m.prompt {
		case promptCloneDest:
			m.closePrompt()
			if val == "" {
				m.showError("destination path cannot be empty")
				return nil
			}
			return m.cloneRepoCmd(m.pendingURL, val)
		case promptCommit:
			m.closePrompt()
			if val == "" {
				m.showError("commit message cannot be empty")
				return nil
			}
			return m.commitCmd(val)
		case promptError:
			m.closePrompt()
			return nil
		}
	}

	u, cmd := m.promptInput.Update(msg)
	m.promptInput = u.(*input.Model)
	return cmd
}

func (m *model) openPrompt(kind promptMode, title, hint, initial string) {
	m.prompt = kind
	m.promptTitle = title
	m.promptHint = hint
	m.promptInput.SetValue(initial)
}

func (m *model) closePrompt() {
	m.prompt = promptNone
	m.promptTitle = ""
	m.promptHint = ""
	m.pendingURL = ""
	m.promptInput.SetValue("")
}

func (m *model) showError(errText string) {
	m.errorText = errText
	m.prompt = promptError
	m.promptTitle = "Error"
	m.promptHint = "Press enter or esc"
	m.promptInput.SetValue("")
}

func (m *model) renderPrompt(t theme.Theme) string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Bold(true).Render(m.promptTitle)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(m.promptHint)
	availW := max(16, m.width-4)
	boxW := m.width / 2
	if boxW < 40 {
		boxW = 40
	}
	if boxW > availW {
		boxW = availW
	}
	bodyW := max(10, boxW-6)
	body := ""
	if m.prompt == promptError {
		body = lipgloss.NewStyle().Foreground(lipgloss.Color(t.State.Danger)).Render(fitMultiline(m.errorText, bodyW, 4))
	} else if m.prompt == promptTheme {
		body = viewString(m.themePicker.View())
	} else {
		body = viewString(m.promptInput.View())
	}
	box := lipgloss.NewStyle().
		Width(boxW).
		Background(lipgloss.Color(t.Surface.Panel)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Border.Focus)).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, hint, "", body))
	return box
}

func (m *model) bodyHeight() int {
	return max(0, m.height-1)
}

func (m *model) diffViewRows() int {
	return max(1, m.bodyHeight()-5)
}

func (m *model) diffRange() (int, int) {
	if len(m.diff.Lines) == 0 {
		return 0, 0
	}
	if len(m.diff.FileAnchors) == 0 {
		return 0, len(m.diff.Lines)
	}
	ptr := clamp(m.diff.FileAnchorPtr, 0, len(m.diff.FileAnchors)-1)
	start := m.diff.FileAnchors[ptr]
	end := len(m.diff.Lines)
	if ptr+1 < len(m.diff.FileAnchors) {
		end = m.diff.FileAnchors[ptr+1]
	}
	return start, end
}

func (m *model) diffMaxScroll() int {
	start, end := m.diffRange()
	return max(0, (end-start)-m.diffViewRows())
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

func (m *model) resizeLocalPicker() {
	pickerMaxH := max(8, min(12, m.bodyHeight()-10))
	pickerH := clamp(len(m.localRows), 8, pickerMaxH)
	m.localListH = pickerH
	m.syncLocalScroll()
}

func (m *model) moveLocalCursor(delta int) {
	if len(m.localRows) == 0 {
		m.localCursor = 0
		m.localScroll = 0
		return
	}
	m.localCursor = clamp(m.localCursor+delta, 0, len(m.localRows)-1)
	m.syncLocalScroll()
}

func (m *model) syncLocalScroll() {
	if len(m.localRows) == 0 {
		m.localScroll = 0
		return
	}
	if m.localListH <= 0 {
		m.localListH = 8
	}
	maxScroll := max(0, len(m.localRows)-m.localListH)
	if m.localCursor < m.localScroll {
		m.localScroll = m.localCursor
	}
	if m.localCursor >= m.localScroll+m.localListH {
		m.localScroll = m.localCursor - m.localListH + 1
	}
	m.localScroll = clamp(m.localScroll, 0, maxScroll)
}

func (m *model) selectedLocalRow() (localTreeRow, bool) {
	if len(m.localRows) == 0 {
		return localTreeRow{}, false
	}
	idx := clamp(m.localCursor, 0, len(m.localRows)-1)
	return m.localRows[idx], true
}

func (m *model) activateLocalSelection() {
	row, ok := m.selectedLocalRow()
	if !ok {
		return
	}
	if row.IsDir {
		if row.Path == filepath.Dir(m.localDir) {
			m.localDir = row.Path
			m.localExpanded = map[string]bool{}
			_ = m.reloadLocalEntries()
			return
		}
		if gitops.IsGitRepo(row.Path) {
			m.projectPath = row.Path
			m.addRecent(row.Path)
			m.statusMessage = "project selected"
			return
		}
		m.localExpanded[row.Path] = !m.localExpanded[row.Path]
		m.rebuildLocalTree()
		return
	}
	if gitops.IsGitRepo(m.localDir) {
		m.projectPath = m.localDir
		m.addRecent(m.localDir)
		m.statusMessage = "project selected"
	}
}

func (m *model) expandLocalSelection() {
	row, ok := m.selectedLocalRow()
	if !ok || !row.IsDir {
		return
	}
	if row.Path == filepath.Dir(m.localDir) {
		return
	}
	m.localExpanded[row.Path] = true
	m.rebuildLocalTree()
}

func (m *model) collapseLocalSelection() {
	row, ok := m.selectedLocalRow()
	if !ok {
		return
	}
	if row.IsDir && row.Path != filepath.Dir(m.localDir) && m.localExpanded[row.Path] {
		delete(m.localExpanded, row.Path)
		m.rebuildLocalTree()
	}
}

func (m *model) rebuildLocalTree() {
	rows := make([]localTreeRow, 0, len(m.localEntries)+8)
	rows = append(rows, buildTreeRows(m.localEntries, "", m.localExpanded)...)
	m.localRows = rows
	if len(m.localRows) == 0 {
		m.localCursor = 0
		m.localScroll = 0
		return
	}
	m.localCursor = clamp(m.localCursor, 0, len(m.localRows)-1)
	m.syncLocalScroll()
}

func (m *model) renderLocalTree(contentW int, t theme.Theme) string {
	if len(m.localRows) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("No entries")
	}
	start := clamp(m.localScroll, 0, max(0, len(m.localRows)-m.localListH))
	end := min(len(m.localRows), start+m.localListH)
	lineW := max(12, contentW)
	baseStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(t.Surface.Panel)).
		Foreground(lipgloss.Color(t.Text.Primary)).
		Width(lineW)
	activeStyle := baseStyle.Copy().
		Background(lipgloss.Color(t.State.Info)).
		Foreground(lipgloss.Color(t.Surface.Canvas)).
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
		Background(lipgloss.Color(t.Surface.Panel)).
		Foreground(lipgloss.Color(t.Text.Muted)).
		Width(lineW).
		Render(
			fmt.Sprintf("%d/%d", min(len(m.localRows), m.localCursor+1), len(m.localRows)),
		)
	lines = append(lines, footer)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func resolveIcons() iconSet {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GLIB_ICONS")))
	safe := iconSet{
		Projects: "[P]",
		Clone:    "[C]",
		Root:     "[R]",
		Dot:      "*",
		Opencode: "[O]",
		Diff:     "[D]",
		Git:      "[G]",
		Quit:     "[Q]",
		Prompt:   "[>]",
	}
	nerd := iconSet{
		Projects: "\uf07c",
		Clone:    "\uf09b",
		Root:     "\ue5ff",
		Dot:      "\uf111",
		Opencode: "\ueea7",
		Diff:     "\uf044",
		Git:      "\ue702",
		Quit:     "\uf00d",
		Prompt:   "\uf120",
	}

	switch mode {
	case "nerd":
		return nerd
	case "safe":
		return safe
	case "auto", "":
		if shouldUseNerdIconsAuto() {
			return nerd
		}
		return safe
	default:
		return safe
	}
}

func shouldUseNerdIconsAuto() bool {
	if v := os.Getenv("GLIB_NERD_ICONS"); v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	if v := os.Getenv("NERD_FONT"); v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	return false
}

func buildTreeRows(entries []projects.Entry, prefix string, expanded map[string]bool) []localTreeRow {
	g := resolveTreeGlyphs()
	rows := make([]localTreeRow, 0, len(entries))
	for i, e := range entries {
		branch := g.Branch
		nextPrefix := prefix + g.Stem
		if i == len(entries)-1 {
			branch = g.Last
			nextPrefix = prefix + g.Gap
		}
		label := e.Name
		if e.IsDir {
			label += "/"
		}
		if e.Name == ".." {
			label = "../"
		}
		rows = append(rows, localTreeRow{Path: e.Path, IsDir: e.IsDir, Label: prefix + branch + label})
		if e.IsDir && e.Name != ".." && expanded[e.Path] {
			children, err := projects.ReadEntriesWithParent(e.Path, false)
			if err == nil && len(children) > 0 {
				rows = append(rows, buildTreeRows(children, nextPrefix, expanded)...)
			}
		}
	}
	return rows
}

type treeGlyphs struct {
	Branch string
	Last   string
	Stem   string
	Gap    string
}

func resolveTreeGlyphs() treeGlyphs {
	if strings.EqualFold(os.Getenv("GLIB_TREE_GLYPHS"), "ascii") {
		return treeGlyphs{Branch: "|- ", Last: "`- ", Stem: "|  ", Gap: "   "}
	}
	if strings.EqualFold(os.Getenv("GLIB_ICONS"), "safe") {
		return treeGlyphs{Branch: "|- ", Last: "`- ", Stem: "|  ", Gap: "   "}
	}
	return treeGlyphs{Branch: "├─ ", Last: "└─ ", Stem: "│  ", Gap: "   "}
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

func fitLine(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	return truncateText(text, maxWidth)
}

func diffFileLabel(diffHeader string) string {
	parts := strings.Fields(diffHeader)
	if len(parts) >= 4 {
		left := strings.TrimPrefix(parts[2], "a/")
		right := strings.TrimPrefix(parts[3], "b/")
		if right != "" {
			return right
		}
		return left
	}
	return diffHeader
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
	if maxWidth <= 0 || lipgloss.Width(text) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return text[:maxWidth]
	}
	b := strings.Builder{}
	for _, r := range text {
		next := b.String() + string(r)
		if lipgloss.Width(next+"...") > maxWidth {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "..."
}

func (m *model) refreshGitCmd() tea.Cmd {
	if m.projectPath == "" {
		m.showError("select a project first")
		return nil
	}
	return func() tea.Msg {
		if !gitops.IsGitRepo(m.projectPath) {
			return gitRefreshMsg{Err: fmt.Errorf("not a git repo: %s", m.projectPath)}
		}
		files, err := gitops.Status(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		logs, err := gitops.Log(m.projectPath, 30)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{Files: files, Log: logs}
	}
}

func (m *model) refreshDiffCmd(source, commitSHA string) tea.Cmd {
	if m.projectPath == "" {
		m.showError("select a project first")
		return nil
	}
	if source == "" {
		source = "working"
		if m.diff.ShowStaged {
			source = "staged"
		}
	}

	return func() tea.Msg {
		if !gitops.IsGitRepo(m.projectPath) {
			return diffRefreshMsg{Err: fmt.Errorf("not a git repo: %s", m.projectPath)}
		}
		var out string
		var err error
		switch source {
		case "working":
			out, _, err = gitops.RunGit(m.projectPath, "diff")
		case "staged":
			out, _, err = gitops.RunGit(m.projectPath, "diff", "--staged")
		case "commit":
			out, _, err = gitops.RunGit(m.projectPath, "show", commitSHA, "--")
		default:
			err = fmt.Errorf("unknown diff source")
		}
		if err != nil {
			return diffRefreshMsg{Err: err}
		}
		lines := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
		anchors := diffview.ParseAnchors(lines)
		return diffRefreshMsg{
			Lines:      lines,
			Anchors:    anchors,
			Source:     source,
			CommitSHA:  commitSHA,
			ProjectDir: m.projectPath,
		}
	}
}

func (m *model) toggleStageCmd() tea.Cmd {
	if len(m.git.Files) == 0 {
		return nil
	}
	file := m.git.Files[m.git.Cursor]
	return func() tea.Msg {
		var err error
		if file.Staged {
			_, _, err = gitops.RunGit(m.projectPath, "restore", "--staged", "--", file.Path)
		} else {
			_, _, err = gitops.RunGit(m.projectPath, "add", "--", file.Path)
		}
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		files, err := gitops.Status(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		logs, err := gitops.Log(m.projectPath, 30)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		action := "staged " + file.Path
		if file.Staged {
			action = "unstaged " + file.Path
		}
		return gitRefreshMsg{Files: files, Log: logs, Action: action}
	}
}

func (m *model) commitCmd(message string) tea.Cmd {
	return func() tea.Msg {
		_, stderr, err := gitops.RunGit(m.projectPath, "commit", "-m", message)
		if err != nil {
			if stderr != "" {
				return gitRefreshMsg{Err: fmt.Errorf("%s", stderr)}
			}
			return gitRefreshMsg{Err: err}
		}
		files, err := gitops.Status(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		logs, err := gitops.Log(m.projectPath, 30)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{Files: files, Log: logs, Action: "commit created"}
	}
}

func (m *model) cloneRepoCmd(url, dest string) tea.Cmd {
	return func() tea.Msg {
		projectPath, err := gitops.Clone(url, dest)
		if err != nil {
			return cloneDoneMsg{Err: err}
		}
		return cloneDoneMsg{ProjectPath: projectPath}
	}
}

func (m *model) jumpDiffFile(step int) {
	if len(m.diff.FileAnchors) == 0 {
		return
	}
	m.diff.FileAnchorPtr = (m.diff.FileAnchorPtr + step) % len(m.diff.FileAnchors)
	if m.diff.FileAnchorPtr < 0 {
		m.diff.FileAnchorPtr = len(m.diff.FileAnchors) - 1
	}
	m.diff.Scroll = 0
}

func (m *model) startOpencodeCmd() tea.Cmd {
	targetDir := m.projectPath
	if targetDir == "" {
		targetDir = m.localDir
	}
	m.tunnelBanner = "glib -> opencode tunnel open | Ctrl+B returns to glib"
	return func() tea.Msg {
		time.Sleep(250 * time.Millisecond)
		err := opencode.Handoff(targetDir)
		return opencodeDoneMsg{Err: err}
	}
}

func (m *model) readOpencodeChunkCmd() tea.Cmd {
	if !m.opencode.Running {
		return nil
	}
	return func() tea.Msg {
		buf, err := m.opencode.ReadChunk()
		if err != nil {
			return opencodeChunkMsg{Err: err}
		}
		return opencodeChunkMsg{Data: buf}
	}
}

func (m *model) forwardOpencodeInputCmd(k tea.KeyMsg) tea.Cmd {
	if !m.opencode.Running {
		return nil
	}
	return func() tea.Msg {
		m.opencode.ForwardInput(k)
		return nil
	}
}

func (m *model) stopOpencode() {
	m.opencode.Stop()
}

func (m *model) reloadThemeItems() {
	names := theme.AvailableThemes()
	items := make([]selectx.Item, 0, len(names))
	for _, n := range names {
		items = append(items, selectx.Item{Label: n, Value: n})
	}
	m.themePicker.SetItems(items)
	m.themePicker.SetSize(max(24, m.width/2), max(8, min(16, m.bodyHeight()-8)))
	m.themePicker.Focus()
	m.themePicker.Open()
}

func (m *model) reloadLocalEntries() error {
	entries, err := projects.ReadEntries(m.localDir)
	if err != nil {
		return err
	}
	m.localEntries = entries
	m.rebuildLocalTree()
	m.resizeLocalPicker()
	return nil
}

func (m *model) findLocalEntry(path string) (projects.Entry, bool) {
	for _, e := range m.localEntries {
		if e.Path == path {
			return e, true
		}
	}
	return projects.Entry{}, false
}

func (m *model) addRecent(path string) {
	if path == "" {
		return
	}
	next := []string{path}
	for _, p := range m.recent {
		if p != path {
			next = append(next, p)
		}
		if len(next) >= 6 {
			break
		}
	}
	m.recent = next
}

func viewString(v tea.View) string {
	return v.Content
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
