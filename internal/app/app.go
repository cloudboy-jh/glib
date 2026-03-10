package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	Files []gitops.File
	Log   []gitops.LogEntry
	Err   error
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
}

func NewModel() *model {
	inp := input.New()
	inp.SetPlaceholder("Search or open a project…")
	promptInp := input.New()
	promptInp.SetPlaceholder("Type value and press enter…")
	lp := selectx.New()
	lp.SetPlaceholder("No directories")
	lp.Focus()
	lp.Open()
	tp := selectx.New()
	tp.SetPlaceholder("No themes")
	tp.Focus()
	tp.Open()

	cwd, _ := os.Getwd()
	m := &model{
		statusBar:   bar.New(),
		inputBox:    inp,
		promptInput: promptInp,
		localPicker: lp,
		themePicker: tp,
		mode:        modeProjects,
		picker:      pickerLocal,
		localDir:    cwd,
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
		m.localPicker.SetSize(max(28, m.width/2), max(8, min(14, m.bodyHeight()/2)))
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

	case tea.KeyMsg:
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
			m.openPrompt(promptTheme, "\ue68b Theme", "j/k move, enter apply, esc cancel", "")
			return m, nil
		case "o":
			if m.mode != modeOpencode {
				return m, m.startOpencodeCmd()
			}
			return m, nil
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
			case "backspace", "h":
				if m.picker == pickerLocal {
					parent := filepath.Dir(m.localDir)
					if parent != m.localDir {
						m.localDir = parent
						_ = m.reloadLocalEntries()
					}
					return m, nil
				}
			case "enter":
				if m.picker == pickerLocal {
					u, cmd := m.localPicker.Update(msg)
					m.localPicker = u.(*selectx.Model)
					sel, ok := m.localPicker.Selected()
					if !ok {
						return m, nil
					}
					entry, found := m.findLocalEntry(sel.Value)
					if !found {
						return m, cmd
					}
					m.localDir = entry.Path
					if gitops.IsGitRepo(entry.Path) {
						m.projectPath = entry.Path
						m.addRecent(entry.Path)
						m.statusMessage = "project selected"
						return m, cmd
					}
					_ = m.reloadLocalEntries()
					return m, cmd
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
			}

			if m.picker == pickerClone {
				u, cmd := m.inputBox.Update(msg)
				m.inputBox = u.(*input.Model)
				return m, cmd
			}
			u, cmd := m.localPicker.Update(msg)
			m.localPicker = u.(*selectx.Model)
			return m, cmd
		}

		if m.mode == modeDiff {
			switch msg.String() {
			case "j", "down":
				m.diff.Scroll = min(m.diff.Scroll+1, max(0, len(m.diff.Lines)-m.bodyHeight()))
			case "k", "up":
				m.diff.Scroll = max(0, m.diff.Scroll-1)
			case "s":
				if m.diff.Source == "commit" {
					return m, nil
				}
				m.diff.ShowStaged = !m.diff.ShowStaged
				return m, m.refreshDiffCmd("", "")
			case "]f":
				m.jumpDiffFile(1)
			case "[f":
				m.jumpDiffFile(-1)
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
	wm := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text.Accent)).
		Bold(true).
		Render(wordmark)
	wmW := lipgloss.Width(wm)
	wmH := lipgloss.Height(wm)

	panelW := clamp(m.width*7/10, 64, 100)
	contentW := max(1, panelW-6)

	header := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Bold(true).Render("\uf07c  Projects")
	modeTag := string(m.picker)

	body := ""
	if m.picker == pickerClone {
		inputStr := viewString(m.inputBox.View())
		recentStr := "no recent projects"
		if len(m.recent) > 0 {
			show := m.recent
			if len(show) > 3 {
				show = show[:3]
			}
			recentStr = strings.Join(show, "   ")
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("\uf09b  clone url"),
			lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary)).Render(inputStr),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(recentStr),
		)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("\ue5ff  root: "+m.localDir),
			"",
			viewString(m.localPicker.View()),
		)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary)).Render("mode: "+modeTag),
		"",
		body,
	)

	block := lipgloss.NewStyle().
		Background(lipgloss.Color(t.Surface.Panel)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Border.Focus)).
		Padding(1, 2).
		Width(contentW).
		Render(content)
	blockW := lipgloss.Width(block)
	blockH := lipgloss.Height(block)

	kbdStr := ""
	if m.picker == pickerClone {
		kbdStr = dim.Render("toggle picker ") + bright.Render("tab") +
			dim.Render("   submit ") + bright.Render("enter")
	} else {
		kbdStr = dim.Render("move ") + bright.Render("j/k") +
			dim.Render("   open/select ") + bright.Render("enter") +
			dim.Render("   parent ") + bright.Render("backspace") +
			dim.Render("   mode ") + bright.Render("tab")
	}
	kbdW := lipgloss.Width(kbdStr)

	dot := lipgloss.NewStyle().Foreground(lipgloss.Color(t.State.Info)).Render("\uf111")
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

	const contentH = 18
	topPad := max(0, (bodyH-contentH)/2)
	y := topPad

	surf.Draw(max(0, (m.width-wmW)/2), y, wm)
	y += wmH + 2
	surf.Draw(max(0, (m.width-blockW)/2), y, block)
	y += blockH + 1
	surf.Draw(max(0, (m.width-kbdW)/2), y, kbdStr)
	y += 2
	surf.Draw(max(0, (m.width-tipW)/2), y, tipStr)
	if status != "" {
		y += 2
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
		Render(title + "  |  " + m.projectPath)
	surf.Draw(2, 1, header)

	if len(m.diff.Lines) == 0 {
		surf.Draw(2, 3, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render("No diff output."))
		return
	}

	viewH := max(0, bodyH-3)
	start := clamp(m.diff.Scroll, 0, max(0, len(m.diff.Lines)-viewH))
	end := min(len(m.diff.Lines), start+viewH)
	y := 3
	for i := start; i < end; i++ {
		line := m.diff.Lines[i]
		surf.Draw(2, y, diffview.StyleLine(line, t))
		y++
	}
}

func (m *model) drawGitView(surf *surface.Surface, bodyH int, t theme.Theme) {
	if m.projectPath == "" {
		surf.Draw(2, 1, "No project selected. Use p to choose a project.")
		return
	}

	header := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Bold(true).
		Render("git status + log  |  " + m.projectPath)
	surf.Draw(2, 1, header)

	leftW := max(30, m.width/2-2)
	rightX := leftW + 4

	focusStatus := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Render("status")
	focusLog := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Accent)).Render("log")
	if m.git.FocusOnLog {
		surf.Draw(2, 2, "status")
		surf.Draw(rightX, 2, focusLog)
	} else {
		surf.Draw(2, 2, focusStatus)
		surf.Draw(rightX, 2, "log")
	}

	maxRows := max(0, bodyH-4)
	for i := 0; i < min(len(m.git.Files), maxRows); i++ {
		f := m.git.Files[i]
		prefix := "  "
		if i == m.git.Cursor && !m.git.FocusOnLog {
			prefix = "> "
		}
		status := fmt.Sprintf("%c%c", f.X, f.Y)
		line := fmt.Sprintf("%s%s %s", prefix, status, f.Path)
		surf.Draw(2, 3+i, gitview.StyleStatusLine(line, f, t, i == m.git.Cursor && !m.git.FocusOnLog))
	}

	for i := 0; i < min(len(m.git.Log), maxRows); i++ {
		l := m.git.Log[i]
		prefix := "  "
		if i == m.git.LogCursor && m.git.FocusOnLog {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s %s", prefix, l.SHA, l.Subject)
		surf.Draw(rightX, 3+i, gitview.StyleLogLine(line, t, i == m.git.LogCursor && m.git.FocusOnLog))
	}

	if m.git.LastAction != "" {
		surf.Draw(2, bodyH-1, lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(m.git.LastAction))
	}
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
	for _, line := range lines {
		surf.Draw(2, y, line)
		y++
		if y >= bodyH {
			break
		}
	}
}

func (m *model) syncStatusBar() {
	left := "\U000f050e o opencode   \uf044 d diff   \ue702 g git   \uf07c p projects   \U000f050e t theme   \uf00d q quit"
	if m.mode == modeOpencode {
		left = "\uf120 esc return   \ue68b t theme   \uf00d q quit"
	}
	projectLabel := "no-project"
	if m.projectPath != "" {
		projectLabel = filepath.Base(m.projectPath)
	}
	m.statusBar.SetCards(nil)
	m.statusBar.SetLeft(left)
	m.statusBar.SetRight(fmt.Sprintf("%s  %s  glib %s", string(m.mode), projectLabel, version))
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
	body := ""
	if m.prompt == promptError {
		body = lipgloss.NewStyle().Foreground(lipgloss.Color(t.State.Danger)).Render(m.errorText)
	} else if m.prompt == promptTheme {
		body = viewString(m.themePicker.View())
	} else {
		body = viewString(m.promptInput.View())
	}
	box := lipgloss.NewStyle().
		Width(max(40, m.width/2)).
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
		return gitRefreshMsg{Files: files, Log: logs}
	}
}

func (m *model) commitCmd(message string) tea.Cmd {
	return func() tea.Msg {
		_, stderr, err := gitops.RunGit(m.projectPath, "commit", "-m", message)
		if err != nil {
			if stderr != "" {
				return gitRefreshMsg{Err: fmt.Errorf(stderr)}
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
		return gitRefreshMsg{Files: files, Log: logs}
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
	m.diff.Scroll = m.diff.FileAnchors[m.diff.FileAnchorPtr]
}

func (m *model) startOpencodeCmd() tea.Cmd {
	if runtime.GOOS == "windows" {
		m.showError("embedded opencode is not supported on this terminal; use diff/git/projects")
		m.mode = modeProjects
		return nil
	}
	if m.opencode.Running {
		m.mode = modeOpencode
		return nil
	}

	state, err := opencode.Start()
	if err != nil {
		m.showError("could not start embedded opencode: " + err.Error())
		m.mode = modeProjects
		return nil
	}
	m.opencode = state
	m.mode = modeOpencode
	return m.readOpencodeChunkCmd()
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
	m.localPicker.SetItems(projects.ToItems(entries))
	m.localPicker.Focus()
	m.localPicker.Open()
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
