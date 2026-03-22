package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	bdcore "github.com/cloudboy-jh/bento-diffs/pkg/bentodiffs"
	"github.com/cloudboy-jh/bentotui/registry/bricks/badge"
	"github.com/cloudboy-jh/bentotui/registry/bricks/card"
	"github.com/cloudboy-jh/bentotui/registry/bricks/input"
	selectx "github.com/cloudboy-jh/bentotui/registry/bricks/select"
	"github.com/cloudboy-jh/bentotui/registry/bricks/surface"
	wordmarkx "github.com/cloudboy-jh/bentotui/registry/bricks/wordmark"
	"github.com/cloudboy-jh/bentotui/registry/recipes/vimstatus"
	"github.com/cloudboy-jh/bentotui/registry/rooms"
	"github.com/cloudboy-jh/bentotui/theme"
	"github.com/cloudboy-jh/bentotui/theme/styles"
	"github.com/hinshun/vt10x"
	"glib/internal/bentodiffs"
	"glib/internal/githubauth"
	"glib/internal/opencode"
	"glib/internal/projects"
	"glib/internal/workspace"
)

const version = "v0.3.3"
const useMockViews = false
const defaultGitHubClientID = "Ov23lipqkO6lVZpjGTZJ"
const githubAuthScope = "repo"

const glibWordmark = "" +
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
	pickerRepos pickerMode = "REPOS"
)

type promptMode string

const (
	promptNone      promptMode = ""
	promptCloneDest promptMode = "clone_dest"
	promptCommit    promptMode = "commit"
	promptDiscard   promptMode = "discard"
	promptError     promptMode = "error"
	promptTheme     promptMode = "theme"
)

type gitRefreshMsg struct {
	State  bentodiffs.GitState
	Action string
	Err    error
}

type diffRefreshMsg struct {
	Diffs        []bdcore.DiffResult
	Source       string
	CommitSHA    string
	ProjectDir   string
	SelectedPath string
	Err          error
}

type cloneDoneMsg struct {
	ProjectPath string
	Err         error
}

type authBootstrapMsg struct {
	Token string
	Err   error
}

type authDeviceMsg struct {
	Device githubauth.DeviceCode
	Err    error
}

type authTokenMsg struct {
	Token string
	Err   error
}

type reposMsg struct {
	Repos []githubauth.Repo
	Err   error
}

type opencodeChunkMsg struct {
	Data []byte
	Err  error
}

type opencodeStartMsg struct {
	State opencode.State
	Err   error
}

type opencodeExitArmTimeoutMsg struct {
	Seq int
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
	footer            *vimstatus.Model
	inputBox          *input.Model
	promptInput       *input.Model
	localPicker       *selectx.Model
	themePicker       *selectx.Model
	width             int
	height            int
	inputW            int
	mode              appMode
	picker            pickerMode
	projectPath       string
	recent            []string
	statusMessage     string
	tunnelBanner      string
	errorText         string
	prompt            promptMode
	promptTitle       string
	promptHint        string
	pendingURL        string
	pendingPath       string
	git               bentodiffs.GitState
	diff              bentodiffs.DiffState
	diffViewer        bdcore.Viewer
	opencode          opencode.State
	localDir          string
	localEntries      []projects.Entry
	localRows         []localTreeRow
	localCursor       int
	localScroll       int
	localListH        int
	localExpanded     map[string]bool
	icons             iconSet
	authStatus        string
	authClientID      string
	authToken         string
	authDevice        githubauth.DeviceCode
	repos             []githubauth.Repo
	repoCursor        int
	repoPage          int
	repoActionOpen    bool
	repoActionCursor  int
	pendingLaunch     string
	opencodeExitArmed bool
	opencodeExitSeq   int
	opencodeTerm      vt10x.Terminal
	workspace         *workspace.Manager
	workspaceKind     workspace.Kind
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
		footer:        vimstatus.New(theme.CurrentTheme()),
		inputBox:      inp,
		promptInput:   promptInp,
		localPicker:   lp,
		themePicker:   tp,
		mode:          modeProjects,
		picker:        pickerLocal,
		localDir:      cwd,
		localExpanded: map[string]bool{},
		icons:         resolveIcons(),
		diff:          bentodiffs.DiffState{},
		authStatus:    githubauth.StatusSignedOut,
		authClientID:  resolveGitHubClientID(),
		repoPage:      1,
		workspaceKind: workspace.KindLocal,
	}
	ws, err := workspace.NewManager(workspace.KindLocal)
	if err == nil {
		m.workspace = ws
	}
	if useMockViews {
		m.ensureDiffViewer()
		m.diffViewer.SetDiffs(bentodiffs.MockDiffs())
		m.git = bentodiffs.MockGitState()
	}
	_ = m.reloadLocalEntries()
	return m
}

func (m *model) Init() tea.Cmd {
	m.syncFooter()
	return tea.Batch(m.inputBox.Focus(), m.bootstrapAuthCmd())
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
		m.footer.SetSize(m.width, 1)
		if m.diffViewer != nil {
			m.diffViewer.SetSize(max(20, m.width-2), max(1, m.bodyHeight()-2))
		}
		if m.opencode.Running {
			vw, vh := m.opencodeViewportSize()
			m.opencode.Resize(vw, vh)
			if m.opencodeTerm != nil {
				m.opencodeTerm.Resize(vw, vh)
			}
		}
		return m, nil

	case theme.ThemeChangedMsg:
		if m.diffViewer != nil {
			m.diffViewer.SetTheme(theme.CurrentTheme())
		}
		m.footer.SetTheme(theme.CurrentTheme())
		return m, nil

	case gitRefreshMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.git = msg.State
		if msg.Action != "" {
			m.git.LastAction = msg.Action
		}
		rows := m.git.Rows()
		if len(rows) == 0 {
			m.git.Cursor = 0
		} else {
			m.git.Cursor = clamp(m.git.Cursor, 0, len(rows)-1)
		}
		return m, nil

	case diffRefreshMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.ensureDiffViewer()
		m.diffViewer.SetDiffs(msg.Diffs)
		if msg.SelectedPath != "" {
			m.diffViewer.SetFileIndex(diffFileIndexByPath(msg.Diffs, msg.SelectedPath))
		}
		m.diff.Source = msg.Source
		m.diff.CommitSHA = msg.CommitSHA
		m.diff.LoadedForDir = msg.ProjectDir
		m.diff.SelectedPath = msg.SelectedPath
		return m, nil

	case bentodiffs.OpenDiffMsg:
		m.mode = modeDiff
		m.ensureDiffViewer()
		if useMockViews {
			m.diffViewer.SetDiffs(bentodiffs.MockDiffs())
			return m, nil
		}
		if m.projectPath == "" {
			return m, nil
		}
		return m, m.refreshDiffCmd("", "", msg.Path)

	case cloneDoneMsg:
		if msg.Err != nil {
			m.pendingLaunch = ""
			m.repoActionOpen = false
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.projectPath = msg.ProjectPath
		m.addRecent(msg.ProjectPath)
		m.statusMessage = "repo ready"
		launch := m.pendingLaunch
		m.pendingLaunch = ""
		m.repoActionOpen = false
		switch launch {
		case "diff":
			m.mode = modeDiff
			m.ensureDiffViewer()
			if useMockViews {
				m.diffViewer.SetDiffs(bentodiffs.MockDiffs())
				return m, nil
			}
			return m, m.refreshDiffCmd("", "", "")
		case "opencode":
			m.mode = modeOpencode
			m.opencode.Buffer = nil
			return m, m.startOpencodeCmd()
		default:
			m.mode = modeProjects
			return m, nil
		}

	case authBootstrapMsg:
		if msg.Err != nil {
			m.statusMessage = "auth bootstrap failed"
			return m, nil
		}
		if msg.Token != "" {
			m.authToken = msg.Token
			m.authStatus = githubauth.StatusAuth
			m.mode = modeProjects
			m.picker = pickerRepos
			return m, m.loadReposCmd()
		}
		m.authStatus = githubauth.StatusSignedOut
		m.mode = modeProjects
		return m, nil

	case authDeviceMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			m.authStatus = githubauth.StatusSignedOut
			return m, nil
		}
		m.authStatus = githubauth.StatusPending
		m.authDevice = msg.Device
		m.statusMessage = "approve device code in browser"
		return m, m.pollAuthCmd(msg.Device)

	case authTokenMsg:
		if msg.Err != nil {
			m.authStatus = githubauth.StatusExpired
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.authToken = msg.Token
		m.authStatus = githubauth.StatusAuth
		m.mode = modeProjects
		m.picker = pickerRepos
		m.statusMessage = "github auth complete"
		return m, tea.Batch(m.persistTokenCmd(msg.Token), m.loadReposCmd())

	case reposMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		sort.Slice(msg.Repos, func(i, j int) bool {
			return strings.ToLower(msg.Repos[i].FullName) < strings.ToLower(msg.Repos[j].FullName)
		})
		m.repos = msg.Repos
		if len(m.repos) == 0 {
			m.repoCursor = 0
		} else {
			m.repoCursor = clamp(m.repoCursor, 0, len(m.repos)-1)
		}
		m.repoActionOpen = false
		m.repoActionCursor = 0
		m.picker = pickerRepos
		return m, nil

	case opencodeChunkMsg:
		if msg.Err != nil {
			if errors.Is(msg.Err, io.EOF) {
				m.stopOpencode()
				m.tunnelBanner = "opencode -> glib tunnel closed"
				m.statusMessage = "returned from opencode"
				m.mode = modeProjects
				return m, nil
			}
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
			if m.opencodeTerm != nil {
				_, _ = m.opencodeTerm.Write(msg.Data)
			}
		}
		return m, m.readOpencodeChunkCmd()

	case opencodeStartMsg:
		if msg.Err != nil {
			m.mode = modeProjects
			m.showError("opencode failed to start: " + msg.Err.Error())
			return m, nil
		}
		m.opencode = msg.State
		vw, vh := m.opencodeViewportSize()
		m.opencode.Resize(vw, vh)
		m.opencodeTerm = vt10x.New(vt10x.WithSize(vw, vh))
		m.opencodeExitArmed = false
		m.tunnelBanner = "glib -> opencode tunnel open"
		return m, m.readOpencodeChunkCmd()

	case opencodeExitArmTimeoutMsg:
		if m.opencodeExitArmed && m.opencodeExitSeq == msg.Seq {
			m.opencodeExitArmed = false
		}
		return m, nil

	case tea.KeyMsg:
		if m.tunnelBanner != "" && msg.String() != "o" {
			m.tunnelBanner = ""
		}
		if m.prompt != promptNone {
			return m, m.updatePrompt(msg)
		}

		switch msg.String() {
		case "ctrl+c":
			if m.mode == modeOpencode {
				return m, m.forwardOpencodeInputCmd(msg)
			}
			m.stopOpencode()
			return m, tea.Quit
		case "q":
			if m.mode == modeProjects {
				m.stopOpencode()
				return m, tea.Quit
			}
		case "tab":
			if m.mode == modeProjects && m.authStatus == githubauth.StatusAuth {
				if m.picker == pickerLocal {
					m.picker = pickerClone
					m.inputBox.SetPlaceholder("Paste git URL (https/ssh)…")
				} else if m.picker == pickerClone {
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
			if m.mode == modeOpencode {
				return m, nil
			}
			m.mode = modeOpencode
			m.opencode.Buffer = nil
			return m, m.startOpencodeCmd()
		case "D":
			m.mode = modeDiff
			m.ensureDiffViewer()
			if useMockViews {
				m.diffViewer.SetDiffs(bentodiffs.MockDiffs())
				return m, nil
			}
			if m.projectPath == "" {
				return m, nil
			}
			return m, m.refreshDiffCmd("", "", "")
		case "G":
			m.mode = modeGit
			if useMockViews {
				m.git = bentodiffs.MockGitState()
				return m, nil
			}
			return m, m.refreshGitCmd()
		case "d":
			if m.mode != modeProjects {
				break
			}
			m.mode = modeDiff
			m.ensureDiffViewer()
			if useMockViews {
				m.diffViewer.SetDiffs(bentodiffs.MockDiffs())
				return m, nil
			}
			if m.projectPath == "" {
				return m, nil
			}
			return m, m.refreshDiffCmd("", "", "")
		case "g":
			if m.mode != modeProjects {
				break
			}
			m.mode = modeGit
			if useMockViews {
				m.git = bentodiffs.MockGitState()
				return m, nil
			}
			return m, m.refreshGitCmd()
		case "p":
			if m.mode == modeOpencode {
				m.stopOpencode()
			}
			m.mode = modeProjects
			return m, m.inputBox.Focus()
		}

		if m.mode == modeOpencode {
			if msg.String() == "ctrl+g" {
				if m.opencodeExitArmed {
					m.stopOpencode()
					m.mode = modeProjects
					m.statusMessage = "opencode terminated"
					return m, nil
				}
				m.opencodeExitArmed = true
				m.opencodeExitSeq++
				return m, m.opencodeExitArmTimeoutCmd(m.opencodeExitSeq)
			}
			if m.opencodeExitArmed {
				m.opencodeExitArmed = false
			}
			return m, m.forwardOpencodeInputCmd(msg)
		}

		if m.mode == modeProjects {
			if m.authStatus != githubauth.StatusAuth {
				switch msg.String() {
				case "enter":
					return m, m.startAuthCmd()
				case "l":
					m.authToken = ""
					m.authStatus = githubauth.StatusSignedOut
					m.authDevice = githubauth.DeviceCode{}
					m.repos = nil
					m.picker = pickerLocal
					return m, m.clearTokenCmd()
				case "r":
					return m, m.startAuthCmd()
				}
				return m, nil
			}

			if m.picker == pickerRepos {
				if m.repoActionOpen {
					switch msg.String() {
					case "j", "down", "l", "right":
						m.repoActionCursor = clamp(m.repoActionCursor+1, 0, 1)
						return m, nil
					case "k", "up", "h", "left":
						m.repoActionCursor = clamp(m.repoActionCursor-1, 0, 1)
						return m, nil
					case "esc":
						m.repoActionOpen = false
						return m, nil
					case "enter":
						repo, ok := m.selectedRepo()
						if !ok {
							return m, nil
						}
						if m.repoActionCursor == 0 {
							m.pendingLaunch = "diff"
							m.statusMessage = "opening diff for " + repo.FullName
						} else {
							m.pendingLaunch = "opencode"
							m.statusMessage = "opening opencode for " + repo.FullName
						}
						return m, m.openRepoCmd(repo)
					}
					return m, nil
				}

				switch msg.String() {
				case "j", "down":
					m.repoCursor = clamp(m.repoCursor+1, 0, max(0, len(m.repos)-1))
					return m, nil
				case "k", "up":
					m.repoCursor = clamp(m.repoCursor-1, 0, max(0, len(m.repos)-1))
					return m, nil
				case "r":
					return m, m.loadReposCmd()
				case "b":
					if m.workspaceKind == workspace.KindLocal {
						m.workspaceKind = workspace.KindEphemeral
					} else {
						m.workspaceKind = workspace.KindLocal
					}
					if m.workspace != nil {
						m.workspace.SetKind(m.workspaceKind)
					}
					m.statusMessage = "backend: " + string(m.workspaceKind)
					return m, nil
				case "enter":
					if len(m.repos) == 0 {
						return m, nil
					}
					m.repoActionOpen = true
					m.repoActionCursor = 0
					return m, nil
				}
				return m, nil
			}

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
			if m.diffViewer == nil {
				m.ensureDiffViewer()
			}
			halfPage := max(1, m.bodyHeight()/2)
			switch msg.String() {
			case "j", "down":
				m.diffViewer.ScrollDown(1)
			case "k", "up":
				m.diffViewer.ScrollUp(1)
			case "ctrl+d", "pgdown":
				m.diffViewer.ScrollDown(halfPage)
			case "ctrl+u", "pgup":
				m.diffViewer.ScrollUp(halfPage)
			case "n":
				m.diffViewer.NextFile()
			case "N":
				m.diffViewer.PrevFile()
			case "}":
				m.diffViewer.NextHunk()
			case "{":
				m.diffViewer.PrevHunk()
			case "home":
				m.diffViewer.ScrollUp(m.diffViewer.State().Scroll)
			case "end":
				state := m.diffViewer.State()
				m.diffViewer.ScrollDown(max(0, state.MaxScroll-state.Scroll))
			case "g", "G":
				m.mode = modeGit
				if useMockViews {
					m.git = bentodiffs.MockGitState()
					return m, nil
				}
				return m, m.refreshGitCmd()
			case "q", "esc":
				m.mode = modeProjects
			}
			return m, nil
		}

		if m.mode == modeGit {
			switch msg.String() {
			case "j", "down":
				m.git.MoveCursor(1)
			case "k", "up":
				m.git.MoveCursor(-1)
			case "s":
				return m, m.stageFileCmd()
			case "u":
				return m, m.unstageFileCmd()
			case "d":
				if f, ok := m.git.SelectedFile(); ok {
					m.pendingPath = f.Path
					m.openPrompt(promptDiscard, "Discard changes", "Type DISCARD and press enter", "")
					return m, m.promptInput.Focus()
				}
			case "c":
				m.openPrompt(promptCommit, "Commit", "Enter commit message", "")
				return m, m.promptInput.Focus()
			case "p":
				return m, m.pushCmd()
			case "enter":
				return m, m.git.OpenSelectedDiffCmd()
			case "q", "esc":
				m.mode = modeProjects
			}
			return m, nil
		}
	}

	return m, nil
}

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
		case modeOpencode:
			m.drawOpencodeView(bodySurf, height, t)
		}
		return bodySurf.Render()
	})

	screen := rooms.Focus(m.width, m.height, body, m.footer)
	surf := surface.New(m.width, m.height)
	surf.Fill(canvasColor)
	surf.Draw(0, 0, screen)

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

		headline := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("repo access")
		statusLine := lipgloss.NewStyle().Bold(true).Foreground(t.Text()).Render("status: ") + statusStyle.Render(statusText)

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

		lines := []string{fitLine(headline, contentW), fitLine(statusLine, contentW), "", button}
		if m.authStatus == githubauth.StatusPending {
			codeLine := lipgloss.NewStyle().Foreground(t.Warning()).Render("Code: " + m.authDevice.UserCode)
			urlLine := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("Open: " + m.authDevice.VerificationURI)
			lines = append(lines, "", fitLine(codeLine, contentW), fitLine(urlLine, contentW))
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
		surf.Draw(max(0, (m.width-statusW)/2), y, lipgloss.NewStyle().Foreground(t.TextMuted()).Render(status))
	}
}

func (m *model) drawRepoProjectsView(surf *surface.Surface, bodyH int, t theme.Theme, dim, bright lipgloss.Style) {
	contentW := m.projectsContentWidth()
	lines := make([]string, 0, 5)
	if len(m.repos) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted()).Render("No repositories loaded. Press r to refresh."))
	} else {
		listH := 5
		start := windowStart(m.repoCursor, listH, len(m.repos))
		end := min(len(m.repos), start+listH)
		base := lipgloss.NewStyle().Width(contentW).Background(t.BackgroundPanel()).Foreground(t.Text())
		active := base.Copy().Background(t.BackgroundInteractive()).Foreground(t.TextInverse()).Bold(true)
		for i := start; i < end; i++ {
			prefix := "  "
			marker := "  "
			style := base
			if i == m.repoCursor {
				prefix = "> "
				style = active
			}
			if i == start && start > 0 {
				marker = "^ "
			} else if i == end-1 && end < len(m.repos) {
				marker = "v "
			}
			name := m.repos[i].FullName
			if m.repos[i].Private {
				name += " (private)"
			}
			lines = append(lines, style.Render(fitLine(marker+prefix+name, contentW)))
		}
		for len(lines) < listH {
			lines = append(lines, base.Render(""))
		}
	}

	header := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(m.icons.Projects + "  Repositories")
	meta := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("backend: " + string(m.workspaceKind))
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
	wm := lipgloss.NewStyle().
		Foreground(t.TextAccent()).
		Bold(true).
		Render(glibWordmark)
	wmW := lipgloss.Width(wm)
	wmH := lipgloss.Height(wm)

	actionKbd := dim.Render("move ") + bright.Render("j/k") + dim.Render("  actions ") + bright.Render("enter") + dim.Render("  backend ") + bright.Render("b") + dim.Render("  refresh ") + bright.Render("r")
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
		diffItem := item(fmt.Sprintf("%s Diff", m.icons.Diff), m.repoActionCursor == 0)
		opencodeItem := item(fmt.Sprintf("%s Opencode", m.icons.Opencode), m.repoActionCursor == 1)
		barLine := diffItem + "   " + opencodeItem
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
	if m.diffViewer == nil {
		m.ensureDiffViewer()
	}

	m.diffViewer.SetSize(max(1, bodyW), max(1, bodyH))
	m.diffViewer.SetTheme(t)
	viewer := strings.TrimRight(m.diffViewer.View(), "\n")
	if viewer == "" {
		return
	}
	lines := strings.Split(viewer, "\n")
	if len(lines) > 1 {
		lines = lines[:len(lines)-1]
	}
	surf.Draw(0, 0, strings.Join(lines, "\n"))
}

func (m *model) drawGitView(surf *surface.Surface, bodyW, bodyH int, t theme.Theme) {
	if !useMockViews && m.projectPath == "" {
		surf.Draw(2, 1, "No project selected. Use p to choose a project.")
		return
	}

	wm := wordmarkx.New("glib")
	wm.SetBold(true)
	tag := badge.New("GIT")
	tag.SetVariant(badge.VariantAccent)
	tag.SetBold(true)

	branchLine := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(m.git.Branch)
	track := m.git.Tracking
	if track == "" {
		track = "(no upstream)"
	}
	sync := ""
	if m.git.Ahead > 0 {
		sync += lipgloss.NewStyle().Foreground(t.Success()).Render(fmt.Sprintf(" ↑%d", m.git.Ahead))
	}
	if m.git.Behind > 0 {
		sync += lipgloss.NewStyle().Foreground(t.Warning()).Render(fmt.Sprintf(" ↓%d", m.git.Behind))
	}
	rows := m.git.Rows()
	headerLine := branchLine + lipgloss.NewStyle().Foreground(t.TextMuted()).Render(" <- "+track) + sync
	summary := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(
		fmt.Sprintf("%d changed  %d staged  +%d -%d", m.git.ChangedTotal, m.git.StagedTotal, m.git.AddedTotal, m.git.DeletedTotal),
	)

	last := ""
	if m.git.LastCommit.Hash != "" {
		last = lipgloss.NewStyle().Foreground(t.Info()).Render(m.git.LastCommit.Hash) +
			"  " + m.git.LastCommit.Message + "  " + lipgloss.NewStyle().Foreground(t.TextMuted()).Render("· "+bentodiffs.RelativeTime(m.git.LastCommit.Time))
	}

	leftPane := rooms.RenderFunc(func(w, h int) string {
		contentW := max(8, w-2)
		contentH := max(1, h-2)
		headerRows := 3
		listH := max(1, contentH-headerRows)
		start := windowStart(m.git.Cursor, listH, len(rows))
		end := min(len(rows), start+listH)

		lines := []string{
			fitLine(headerLine, contentW),
			fitLine(summary, contentW),
			"",
		}

		for i := start; i < end; i++ {
			row := rows[i]
			if row.IsHeader() {
				line := lipgloss.NewStyle().Foreground(t.TextMuted()).Bold(true).Render(strings.ToUpper(row.Label))
				lines = append(lines, fitLine(line, contentW))
				continue
			}
			f := row.File
			cursor := " "
			if i == m.git.Cursor {
				cursor = lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render("▸")
			}
			statusStyle := lipgloss.NewStyle().Foreground(t.TextMuted())
			switch f.Status {
			case "M":
				statusStyle = statusStyle.Foreground(t.Warning())
			case "A":
				statusStyle = statusStyle.Foreground(t.Success())
			case "D":
				statusStyle = statusStyle.Foreground(t.Error())
			case "R":
				statusStyle = statusStyle.Foreground(t.Info())
			}
			path := truncateText(f.Path, max(8, contentW-20))
			stats := lipgloss.NewStyle().Foreground(t.Success()).Render(fmt.Sprintf("+%d", f.Added)) +
				" " + lipgloss.NewStyle().Foreground(t.Error()).Render(fmt.Sprintf("-%d", f.Deleted))
			left := fmt.Sprintf("%s %s  %s", cursor, statusStyle.Render(f.Status), path)
			pad := max(1, contentW-lipgloss.Width(left)-lipgloss.Width(stats)-1)
			line := left + strings.Repeat(" ", pad) + stats
			if i == m.git.Cursor {
				line = lipgloss.NewStyle().Background(t.BackgroundInteractive()).Width(contentW).Render(line)
			}
			lines = append(lines, fitLine(line, contentW))
		}

		content := &staticTextModel{text: strings.Join(lines, "\n")}
		p := card.New(
			card.Title(fmt.Sprintf("%s  %s", viewString(wm.View()), viewString(tag.View()))),
			card.Content(content),
			card.Raised(),
		)
		p.SetSize(w, h)
		return viewString(p.View())
	})

	rightPane := rooms.RenderFunc(func(w, h int) string {
		contentW := max(8, w-2)
		selectedPath := ""
		selectedStatus := ""
		selectedStats := ""
		if f, ok := m.git.SelectedFile(); ok {
			selectedPath = f.Path
			selectedStatus = f.Status
			selectedStats = fmt.Sprintf("+%d -%d", f.Added, f.Deleted)
		}

		statusBadge := badge.New(strings.TrimSpace(selectedStatus))
		statusBadge.SetVariant(badge.VariantNeutral)
		if selectedStatus == "A" {
			statusBadge.SetVariant(badge.VariantSuccess)
		}
		if selectedStatus == "M" {
			statusBadge.SetVariant(badge.VariantWarning)
		}
		if selectedStatus == "D" {
			statusBadge.SetVariant(badge.VariantDanger)
		}
		if selectedStatus == "R" {
			statusBadge.SetVariant(badge.VariantInfo)
		}

		lines := []string{
			fitLine(lipgloss.NewStyle().Bold(true).Render(truncateText(selectedPath, contentW)), contentW),
			fitLine(lipgloss.NewStyle().Foreground(t.TextMuted()).Render("status ")+viewString(statusBadge.View()), contentW),
			fitLine(lipgloss.NewStyle().Foreground(t.TextMuted()).Render("stats "+selectedStats), contentW),
			"",
		}
		if last != "" {
			lines = append(lines,
				fitLine(lipgloss.NewStyle().Foreground(t.TextMuted()).Render("last commit"), contentW),
				fitLine(last, contentW),
			)
		}

		content := &staticTextModel{text: strings.Join(lines, "\n")}
		p := card.New(card.Title("Selection"), card.Content(content), card.Raised())
		p.SetSize(w, h)
		return viewString(p.View())
	})

	split := rooms.HSplit(bodyW, bodyH, leftPane, rightPane)
	surf.Draw(0, 0, split)
}

func (m *model) drawOpencodeView(surf *surface.Surface, bodyH int, t theme.Theme) {
	if m.width <= 2 || bodyH <= 2 {
		return
	}
	vw, vh := m.opencodeViewportSize()
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

	lines := m.opencodeViewportLines(vw, vh)
	for y, line := range lines {
		surf.Draw(1, y+1, line)
	}
}

func (m *model) opencodeViewportLines(cols, rows int) []string {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	lines := make([]string, 0, rows)
	if m.opencodeTerm == nil {
		msg := "starting opencode..."
		if m.opencode.Running {
			msg = "connected, waiting for output..."
		}
		lines = append(lines, fitLine(msg, cols))
		for len(lines) < rows {
			lines = append(lines, "")
		}
		return lines
	}
	m.opencodeTerm.Lock()
	defer m.opencodeTerm.Unlock()

	for y := 0; y < rows; y++ {
		r := make([]rune, cols)
		for x := 0; x < cols; x++ {
			g := m.opencodeTerm.Cell(x, y)
			ch := g.Char
			if ch == 0 {
				ch = ' '
			}
			r[x] = ch
		}
		line := strings.TrimRight(string(r), " ")
		lines = append(lines, fitLine(line, cols))
	}
	return lines
}

func (m *model) syncFooter() {
	cfg := vimstatus.Config{
		Mode:      m.footerModeLabel(),
		Branch:    "glib",
		Context:   "",
		ShowClock: true,
	}

	if m.projectPath != "" {
		cfg.Branch = filepath.Base(m.projectPath)
	}

	switch m.mode {
	case modeProjects:
		if m.authStatus != githubauth.StatusAuth {
			cfg.Context = m.icons.Projects + " enter sign in  r retry  " + m.icons.Quit + " q quit"
			cfg.Scroll = strings.ToLower(m.authStatus)
		} else if m.picker == pickerRepos {
			if m.repoActionOpen {
				cfg.Context = m.icons.Projects + " h/l choose  enter run  esc back"
			} else {
				cfg.Context = m.icons.Projects + " j/k move  enter actions  b backend  r refresh"
			}
			if len(m.repos) > 0 {
				cfg.Position = fmt.Sprintf("%d/%d", min(len(m.repos), m.repoCursor+1), len(m.repos))
			}
			cfg.Scroll = string(m.workspaceKind)
		} else {
			cfg.Context = m.icons.Projects + " p projects  " + m.icons.Clone + " tab picker  " + m.icons.Quit + " q quit"
			if len(m.localRows) > 0 {
				cfg.Position = fmt.Sprintf("%d/%d", min(len(m.localRows), m.localCursor+1), len(m.localRows))
			}
		}
	case modeDiff:
		cfg.Context = m.icons.Diff + " j/k scroll  n/N file  { } hunk  " + m.icons.Git + " g git"
		if m.diffViewer != nil {
			st := m.diffViewer.State()
			cfg.Position = fmt.Sprintf("%d/%d", st.Scroll+1, st.MaxScroll+1)
			cfg.Scroll = fmt.Sprintf("file %d/%d", st.ActiveFile+1, max(1, st.FileCount))
		}
	case modeGit:
		cfg.Context = m.icons.Git + " s stage  u unstage  c commit  d discard  enter diff"
		rows := m.git.Rows()
		if len(rows) > 0 {
			cfg.Position = fmt.Sprintf("%d/%d", min(len(rows), m.git.Cursor+1), len(rows))
		}
		cfg.Scroll = fmt.Sprintf("+%d -%d", m.git.AddedTotal, m.git.DeletedTotal)
	case modeOpencode:
		if m.opencodeExitArmed {
			cfg.Context = m.icons.Opencode + " press ctrl+g again to exit"
		} else {
			cfg.Context = m.icons.Opencode + " passthrough  ctrl+g ctrl+g exit"
		}
		if m.opencode.Running {
			cfg.Scroll = "live"
		} else {
			cfg.Scroll = "starting"
		}
	}

	if m.tunnelBanner != "" {
		cfg.Context = m.tunnelBanner
	}

	m.footer.SetTheme(theme.CurrentTheme())
	m.footer.SetConfig(cfg)
}

func (m *model) footerModeLabel() string {
	switch m.mode {
	case modeProjects:
		return m.icons.Projects + " " + string(m.mode)
	case modeDiff:
		return m.icons.Diff + " " + string(m.mode)
	case modeGit:
		return m.icons.Git + " " + string(m.mode)
	case modeOpencode:
		return m.icons.Opencode + " " + string(m.mode)
	default:
		return string(m.mode)
	}
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
		case promptDiscard:
			m.closePrompt()
			if strings.ToUpper(val) != "DISCARD" {
				m.showError("discard not confirmed")
				return nil
			}
			if m.pendingPath == "" {
				return nil
			}
			path := m.pendingPath
			m.pendingPath = ""
			return m.discardFileCmd(path)
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
	m.pendingPath = ""
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
	title := lipgloss.NewStyle().Foreground(t.TextAccent()).Bold(true).Render(m.promptTitle)
	hint := lipgloss.NewStyle().Foreground(t.TextMuted()).Render(m.promptHint)
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
		body = lipgloss.NewStyle().Foreground(t.Error()).Render(fitMultiline(m.errorText, bodyW, 4))
	} else if m.prompt == promptTheme {
		body = viewString(m.themePicker.View())
	} else {
		body = viewString(m.promptInput.View())
	}
	box := lipgloss.NewStyle().
		Width(boxW).
		Background(t.BackgroundPanel()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus()).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, hint, "", body))
	return box
}

func (m *model) bodyHeight() int {
	return max(0, m.height-1)
}

func (m *model) opencodeViewportSize() (int, int) {
	return max(1, m.width-2), max(1, m.bodyHeight()-2)
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
		if bentodiffs.IsGitRepo(row.Path) {
			m.projectPath = row.Path
			m.addRecent(row.Path)
			m.statusMessage = "project selected"
			return
		}
		m.localExpanded[row.Path] = !m.localExpanded[row.Path]
		m.rebuildLocalTree()
		return
	}
	if bentodiffs.IsGitRepo(m.localDir) {
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
		Projects: "\uea83", // nf-cod-folder
		Clone:    "\ueb3e", // nf-cod-repo_clone
		Root:     "\ueb46", // nf-cod-root_folder
		Dot:      "\uea71", // nf-cod-circle_filled
		Opencode: "\uea85", // nf-cod-terminal
		Diff:     "\ueae1", // nf-cod-diff
		Git:      "\uea68", // nf-cod-source_control
		Quit:     "\uea76", // nf-cod-close
		Prompt:   "\ueab6", // nf-cod-chevron_right
	}

	switch mode {
	case "safe":
		return safe
	case "nerd", "auto", "", "unicode", "utf", "utf8":
		return nerd
	default:
		return nerd
	}
}

func resolveGitHubClientID() string {
	if v := strings.TrimSpace(os.Getenv("GLIB_GITHUB_CLIENT_ID")); v != "" {
		return v
	}
	return defaultGitHubClientID
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

func (m *model) bootstrapAuthCmd() tea.Cmd {
	return func() tea.Msg {
		tok, err := githubauth.LoadToken()
		return authBootstrapMsg{Token: tok, Err: err}
	}
}

func (m *model) startAuthCmd() tea.Cmd {
	if strings.TrimSpace(m.authClientID) == "" {
		m.showError("missing GitHub client id")
		return nil
	}
	return func() tea.Msg {
		device, err := githubauth.StartDeviceFlow(m.authClientID, githubAuthScope)
		return authDeviceMsg{Device: device, Err: err}
	}
}

func (m *model) pollAuthCmd(device githubauth.DeviceCode) tea.Cmd {
	return func() tea.Msg {
		tok, err := githubauth.PollAccessToken(m.authClientID, device.DeviceCode, device.Interval)
		return authTokenMsg{Token: tok, Err: err}
	}
}

func (m *model) persistTokenCmd(token string) tea.Cmd {
	return func() tea.Msg {
		if err := githubauth.SaveToken(token); err != nil {
			return authTokenMsg{Err: err}
		}
		return nil
	}
}

func (m *model) clearTokenCmd() tea.Cmd {
	return func() tea.Msg {
		if err := githubauth.ClearToken(); err != nil {
			return authBootstrapMsg{Err: err}
		}
		return nil
	}
}

func (m *model) loadReposCmd() tea.Cmd {
	if strings.TrimSpace(m.authToken) == "" {
		return nil
	}
	page := m.repoPage
	if page < 1 {
		page = 1
	}
	return func() tea.Msg {
		repos, err := githubauth.ListRepos(m.authToken, page, 100)
		return reposMsg{Repos: repos, Err: err}
	}
}

func (m *model) selectedRepo() (githubauth.Repo, bool) {
	if len(m.repos) == 0 {
		return githubauth.Repo{}, false
	}
	idx := clamp(m.repoCursor, 0, len(m.repos)-1)
	return m.repos[idx], true
}

func (m *model) openRepoCmd(repo githubauth.Repo) tea.Cmd {
	if m.workspace == nil {
		m.showError("workspace manager unavailable")
		return nil
	}
	return func() tea.Msg {
		path, err := m.workspace.EnsureRepo(repo.FullName, repo.CloneURL)
		if err != nil {
			return cloneDoneMsg{Err: err}
		}
		return cloneDoneMsg{ProjectPath: path}
	}
}

func (m *model) refreshGitCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := bentodiffs.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state}
		}
	}
	if m.projectPath == "" {
		m.showError("select a project first")
		return nil
	}
	return func() tea.Msg {
		if !bentodiffs.IsGitRepo(m.projectPath) {
			return gitRefreshMsg{Err: fmt.Errorf("not a git repo: %s", m.projectPath)}
		}
		state, err := bentodiffs.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		state.Cursor = m.git.Cursor
		return gitRefreshMsg{State: state}
	}
}

func (m *model) refreshDiffCmd(source, commitSHA, selectedPath string) tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			return diffRefreshMsg{Diffs: bentodiffs.MockDiffs(), Source: "mock", SelectedPath: selectedPath}
		}
	}
	if m.projectPath == "" {
		m.showError("select a project first")
		return nil
	}
	if source == "" {
		source = "working"
	}

	return func() tea.Msg {
		if !bentodiffs.IsGitRepo(m.projectPath) {
			return diffRefreshMsg{Err: fmt.Errorf("not a git repo: %s", m.projectPath)}
		}
		var out string
		var err error
		args := []string{}
		switch source {
		case "working":
			args = []string{"diff"}
		case "commit":
			args = []string{"show", commitSHA, "--"}
		default:
			err = fmt.Errorf("unknown diff source")
		}
		if selectedPath != "" {
			args = append(args, "--", selectedPath)
		}
		if err == nil {
			out, _, err = bentodiffs.RunGit(m.projectPath, args...)
		}
		if err != nil {
			return diffRefreshMsg{Err: err}
		}
		diffs, parseErr := bdcore.ParseUnifiedDiffs(out)
		if parseErr != nil {
			return diffRefreshMsg{Err: parseErr}
		}
		return diffRefreshMsg{
			Diffs:        diffs,
			Source:       source,
			CommitSHA:    commitSHA,
			ProjectDir:   m.projectPath,
			SelectedPath: selectedPath,
		}
	}
}

func (m *model) stageFileCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := bentodiffs.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: staged file"}
		}
	}
	f, ok := m.git.SelectedFile()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if err := bentodiffs.StageFile(m.projectPath, f.Path); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := bentodiffs.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "staged " + f.Path}
	}
}

func (m *model) unstageFileCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := bentodiffs.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: unstaged file"}
		}
	}
	f, ok := m.git.SelectedFile()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if err := bentodiffs.UnstageFile(m.projectPath, f.Path); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := bentodiffs.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "unstaged " + f.Path}
	}
}

func (m *model) discardFileCmd(path string) tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := bentodiffs.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: discarded " + path}
		}
	}
	return func() tea.Msg {
		if err := bentodiffs.DiscardFile(m.projectPath, path); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := bentodiffs.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "discarded " + path}
	}
}

func (m *model) commitCmd(message string) tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := bentodiffs.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: commit created"}
		}
	}
	return func() tea.Msg {
		if err := bentodiffs.Commit(m.projectPath, message); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := bentodiffs.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "commit created"}
	}
}

func (m *model) pushCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := bentodiffs.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: pushed"}
		}
	}
	return func() tea.Msg {
		if err := bentodiffs.Push(m.projectPath); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := bentodiffs.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "pushed"}
	}
}

func (m *model) cloneRepoCmd(url, dest string) tea.Cmd {
	return func() tea.Msg {
		projectPath, err := bentodiffs.Clone(url, dest)
		if err != nil {
			return cloneDoneMsg{Err: err}
		}
		return cloneDoneMsg{ProjectPath: projectPath}
	}
}

func (m *model) ensureDiffViewer() {
	if m.diffViewer == nil {
		m.diffViewer = bdcore.NewViewer(bdcore.ViewerOptions{
			SyntaxEnabled:   true,
			ShowLineNumbers: true,
			Theme:           theme.CurrentTheme(),
		})
	}
}

func diffFileIndexByPath(diffs []bdcore.DiffResult, selectedPath string) int {
	if selectedPath == "" || len(diffs) == 0 {
		return 0
	}
	for i, d := range diffs {
		newName := strings.TrimPrefix(d.NewFile, "b/")
		oldName := strings.TrimPrefix(d.OldFile, "a/")
		displayName := strings.TrimPrefix(d.DisplayFile, "b/")
		if selectedPath == newName || selectedPath == oldName || selectedPath == displayName {
			return i
		}
	}
	return 0
}

func (m *model) startOpencodeCmd() tea.Cmd {
	targetDir := m.projectPath
	if targetDir == "" {
		targetDir = m.localDir
	}
	return func() tea.Msg {
		state, err := opencode.Start(targetDir)
		if err != nil {
			return opencodeStartMsg{Err: err}
		}
		return opencodeStartMsg{State: state}
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

func (m *model) opencodeExitArmTimeoutCmd(seq int) tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg {
		return opencodeExitArmTimeoutMsg{Seq: seq}
	})
}

func (m *model) stopOpencode() {
	m.opencodeExitArmed = false
	m.opencodeTerm = nil
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

type staticTextModel struct {
	text   string
	width  int
	height int
}

func (s *staticTextModel) Init() tea.Cmd { return nil }

func (s *staticTextModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

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
		out = append(out, fitLine(lines[i], max(1, s.width)))
	}
	return tea.NewView(strings.Join(out, "\n"))
}

func viewString(v tea.View) string {
	if v.Content == nil {
		return ""
	}
	if r, ok := v.Content.(interface{ Render() string }); ok {
		return r.Render()
	}
	if s, ok := v.Content.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprint(v.Content)
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
