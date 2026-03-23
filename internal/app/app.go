package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	"glib/internal/bentodiffs"
	"glib/internal/githubauth"
	"glib/internal/pi"
	"glib/internal/piui"
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
	modePI       appMode = "PI"
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
	promptNewProj   promptMode = "new_project"
	promptDiffRev   promptMode = "diff_revision"
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

type piStartMsg struct {
	Proc *pi.PiProcess
	Err  error
}

type piSendMsg struct {
	Err error
}

type workspaceCleanupMsg struct {
	Result workspace.CleanupResult
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
	PI       string
	Diff     string
	Git      string
	Quit     string
	Prompt   string
}

type model struct {
	footer           *vimstatus.Model
	inputBox         *input.Model
	promptInput      *input.Model
	localPicker      *selectx.Model
	themePicker      *selectx.Model
	width            int
	height           int
	inputW           int
	mode             appMode
	picker           pickerMode
	projectPath      string
	activeRepoName   string
	pendingRepoName  string
	recent           []string
	lastRepo         string
	statusMessage    string
	errorText        string
	prompt           promptMode
	promptTitle      string
	promptHint       string
	pendingURL       string
	pendingPath      string
	git              bentodiffs.GitState
	diff             bentodiffs.DiffState
	diffViewer       bdcore.Viewer
	piProc           *pi.PiProcess
	piui             *piui.Session
	localDir         string
	localEntries     []projects.Entry
	localRows        []localTreeRow
	localCursor      int
	localScroll      int
	localListH       int
	localExpanded    map[string]bool
	icons            iconSet
	authStatus       string
	authClientID     string
	authToken        string
	authDevice       githubauth.DeviceCode
	repos            []githubauth.Repo
	repoCursor       int
	repoPage         int
	repoActionOpen   bool
	repoActionCursor int
	pendingLaunch    string
	workspace        *workspace.Manager
	workspaceKind    workspace.Kind
	quitting         bool
	pendingSlash     map[string]string
	slashSeq         int
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
	piSession := piui.NewSession()

	cwd, _ := os.Getwd()
	m := &model{
		footer:        vimstatus.New(theme.CurrentTheme()),
		inputBox:      inp,
		promptInput:   promptInp,
		piui:          piSession,
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
		pendingSlash:  map[string]string{},
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
		m.piui.SetSize(m.width, m.bodyHeight())
		m.resizeLocalPicker()
		m.footer.SetSize(m.width, 1)
		if m.diffViewer != nil {
			m.diffViewer.SetSize(max(20, m.width-2), max(1, m.bodyHeight()-2))
		}
		m.refreshAgentViewport()
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
			m.pendingRepoName = ""
			m.repoActionOpen = false
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.projectPath = msg.ProjectPath
		if strings.TrimSpace(m.pendingRepoName) != "" {
			m.activeRepoName = strings.TrimSpace(m.pendingRepoName)
			m.pendingRepoName = ""
		} else {
			m.activeRepoName = inferRepoNameFromPath(msg.ProjectPath)
		}
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
		case "git":
			m.mode = modeGit
			if useMockViews {
				m.git = bentodiffs.MockGitState()
				return m, nil
			}
			return m, m.refreshGitCmd()
		case "pi":
			m.mode = modePI
			return m, m.startPiCmd()
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
		m.repos = m.orderRepos(msg.Repos)
		if len(m.repos) == 0 {
			m.repoCursor = 0
		} else {
			m.repoCursor = m.lastRepoIndex()
		}
		m.repoActionOpen = false
		m.repoActionCursor = 0
		m.picker = pickerRepos
		return m, nil

	case piStartMsg:
		if msg.Err != nil {
			m.mode = modeProjects
			m.showError("pi failed to start: " + msg.Err.Error())
			return m, nil
		}
		m.piProc = msg.Proc
		m.piui.Status = "connected"
		m.refreshAgentViewport()
		return m, tea.Batch(
			m.readPiEventCmd(),
			piui.SpinnerTickCmd(),
			m.sendPiCmd(pi.CmdGetState()),
			m.sendPiCmd(map[string]any{"type": "get_commands"}),
			m.sendPiCmd(map[string]any{"type": "get_session_stats"}),
		)

	case piSendMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		return m, m.readPiEventCmd()

	case pi.PiEventMsg:
		next := m.handlePiEvent(msg)
		m.refreshAgentViewport()
		return m, tea.Batch(m.readPiEventCmd(), next)

	case pi.PiResponseMsg:
		if cmdName, ok := m.pendingSlash[msg.ID]; ok {
			delete(m.pendingSlash, msg.ID)
			m.appendSlashResponse(cmdName, msg)
			m.refreshAgentViewport()
			return m, m.readPiEventCmd()
		}
		if !msg.Success && strings.TrimSpace(msg.Error) != "" {
			m.showError("pi command failed: " + msg.Error)
			return m, m.readPiEventCmd()
		}
		switch msg.Command {
		case "get_state":
			m.piui.ApplyState(msg.Data)
		case "get_session_stats":
			m.piui.ApplyStats(msg.Data)
		case "get_commands":
			m.piui.ApplyCommands(msg.Data)
		}
		m.refreshAgentViewport()
		return m, m.readPiEventCmd()

	case pi.PiExitMsg:
		m.stopPi()
		m.piui.StopStreaming()
		m.piui.Streaming = false
		m.piui.ToolRunning = false
		m.piui.Status = "stopped"
		if msg.Err != nil {
			m.showError("pi exited: " + msg.Err.Error())
			m.piui.Messages = append(m.piui.Messages, piui.Message{
				Role: piui.RoleTool,
				ToolBlock: &piui.ToolBlock{
					Name:   "pi",
					Output: "Process exited. Check PI model/API configuration, then press i to retry.",
					Done:   true,
					ExitOK: false,
				},
			})
			m.refreshAgentViewport()
		}
		return m, nil

	case piui.SpinnerTickMsg:
		m.piui.TickSpinner()
		if m.mode == modePI {
			return m, piui.SpinnerTickCmd()
		}
		return m, nil

	case workspaceCleanupMsg:
		if len(msg.Result.Warnings) > 0 {
			m.statusMessage = "quit cleanup: " + msg.Result.Warnings[0]
		} else if len(msg.Result.Removed) > 0 {
			m.statusMessage = fmt.Sprintf("quit cleanup: removed %d ephemeral worktrees", len(msg.Result.Removed))
		}
		if m.quitting {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyMsg:
		if m.prompt != promptNone {
			return m, m.updatePrompt(msg)
		}
		if m.mode == modePI {
			return m.updatePIKeys(msg)
		}

		switch msg.String() {
		case "ctrl+c":
			return m, m.quitCmd()
		case "q":
			if m.mode == modeProjects {
				return m, m.quitCmd()
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
		case "i":
			if m.mode == modePI {
				return m, nil
			}
			if m.mode == modeProjects && m.picker == pickerRepos {
				repo, ok := m.selectedRepo()
				if ok {
					if m.activeRepoName == repo.FullName && strings.TrimSpace(m.projectPath) != "" && bentodiffs.IsGitRepo(m.projectPath) {
						m.mode = modePI
						return m, m.startPiCmd()
					}
					m.pendingLaunch = "pi"
					m.pendingRepoName = repo.FullName
					m.statusMessage = "opening pi for " + repo.FullName
					return m, m.openRepoCmd(repo)
				}
			}
			if m.projectPath == "" {
				if m.mode == modeProjects && m.picker == pickerRepos {
					repo, ok := m.selectedRepo()
					if ok {
						m.pendingLaunch = "pi"
						m.pendingRepoName = repo.FullName
						m.statusMessage = "opening pi for " + repo.FullName
						return m, m.openRepoCmd(repo)
					}
				}
				m.showError("select a repository first")
				return m, nil
			}
			m.mode = modePI
			return m, m.startPiCmd()
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
			m.stopPi()
			m.mode = modeProjects
			return m, m.inputBox.Focus()
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
						m.repoActionCursor = clamp(m.repoActionCursor+1, 0, 2)
						return m, nil
					case "k", "up", "h", "left":
						m.repoActionCursor = clamp(m.repoActionCursor-1, 0, 2)
						return m, nil
					case "esc":
						m.repoActionOpen = false
						return m, nil
					case "enter":
						repo, ok := m.selectedRepo()
						if !ok {
							return m, nil
						}
						m.lastRepo = repo.FullName
						m.pendingRepoName = repo.FullName
						switch m.repoActionCursor {
						case 0:
							m.pendingLaunch = "diff"
							m.statusMessage = "opening diff for " + repo.FullName
						case 1:
							m.pendingLaunch = "git"
							m.statusMessage = "opening git for " + repo.FullName
						default:
							m.pendingLaunch = "pi"
							m.statusMessage = "opening pi for " + repo.FullName
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
				case "n":
					m.openPrompt(promptNewProj, "New Project", "Enter folder path to create + git init", filepath.Join(m.localDir, "new-project"))
					return m, m.promptInput.Focus()
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
			case "n":
				m.openPrompt(promptNewProj, "New Project", "Enter folder path to create + git init", filepath.Join(m.localDir, "new-project"))
				return m, m.promptInput.Focus()
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
			case "c":
				m.openPrompt(promptDiffRev, "View Commit Diff", "Enter commit sha/ref", "HEAD")
				return m, m.promptInput.Focus()
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
		case modePI:
			m.drawPIView(bodySurf, height, t)
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
		gitItem := item(fmt.Sprintf("%s Git", m.icons.Git), m.repoActionCursor == 1)
		piItem := item(fmt.Sprintf("%s Pi", m.icons.PI), m.repoActionCursor == 2)
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

	headerRows := 1
	inputRows := 5
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
	surf.Draw(1, 1, fitLine(headerRow, vw))

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

	inputContent := viewString(m.piui.Input.View())

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
	inputBlock := lipgloss.JoinVertical(lipgloss.Left,
		blank,
		blank,
		mkRow(inputContent),
		blank,
		blank,
	)
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
			cfg.Context = m.icons.Projects + " enter sign in  r retry  " + m.icons.Quit + " q quit"
			cfg.Scroll = strings.ToLower(m.authStatus)
		} else if m.picker == pickerRepos {
			if m.repoActionOpen {
				cfg.Context = m.icons.Projects + " h/l choose  enter run  esc back"
			} else {
				cfg.Context = m.icons.Projects + " j/k move  enter actions  b backend  n new  r refresh"
			}
			if len(m.repos) > 0 {
				cfg.Position = fmt.Sprintf("%d/%d", min(len(m.repos), m.repoCursor+1), len(m.repos))
			}
			cfg.Scroll = string(m.workspaceKind)
		} else {
			cfg.Context = m.icons.Projects + " p projects  " + m.icons.PI + " i pi  n new  " + m.icons.Clone + " tab picker  " + m.icons.Quit + " q quit"
			if len(m.localRows) > 0 {
				cfg.Position = fmt.Sprintf("%d/%d", min(len(m.localRows), m.localCursor+1), len(m.localRows))
			}
		}
	case modeDiff:
		cfg.Context = m.icons.Diff + " j/k scroll  n/N file  c commit  { } hunk  " + m.icons.Git + " g git"
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
	case modePI:
		st := m.piui.FooterState("", "")
		cfg.Context = st.Context
		cfg.Scroll = st.Scroll
		cfg.Position = st.Position
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
	case modePI:
		return "Pi"
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
		case promptNewProj:
			m.closePrompt()
			if val == "" {
				m.showError("project path cannot be empty")
				return nil
			}
			return m.initProjectCmd(val)
		case promptDiffRev:
			m.closePrompt()
			if val == "" {
				m.showError("commit sha/ref cannot be empty")
				return nil
			}
			m.mode = modeDiff
			return m.refreshDiffCmd("commit", val, "")
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

func (m *model) piViewportSize() (int, int) {
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
			m.activeRepoName = inferRepoNameFromPath(row.Path)
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
		m.activeRepoName = inferRepoNameFromPath(m.localDir)
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
		PI:       "π",
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
		PI:       "π",
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
			working, _, wErr := bentodiffs.RunGit(m.projectPath, "diff")
			if wErr != nil {
				return diffRefreshMsg{Err: wErr}
			}
			staged, _, sErr := bentodiffs.RunGit(m.projectPath, "diff", "--cached")
			if sErr != nil {
				return diffRefreshMsg{Err: sErr}
			}
			out = strings.TrimSpace(strings.Join([]string{working, staged}, "\n"))
		case "commit":
			args = []string{"show", commitSHA, "--"}
		default:
			err = fmt.Errorf("unknown diff source")
		}
		if selectedPath != "" && source != "working" {
			args = append(args, "--", selectedPath)
		}
		if err == nil && out == "" {
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

func (m *model) initProjectCmd(dest string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(dest) == "" {
			return cloneDoneMsg{Err: fmt.Errorf("project path cannot be empty")}
		}
		abs, err := filepath.Abs(dest)
		if err != nil {
			return cloneDoneMsg{Err: err}
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return cloneDoneMsg{Err: err}
		}
		if _, _, err := bentodiffs.RunGit(abs, "init"); err != nil {
			return cloneDoneMsg{Err: err}
		}
		return cloneDoneMsg{ProjectPath: abs}
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

func (m *model) startPiCmd() tea.Cmd {
	targetDir := m.projectPath
	if strings.TrimSpace(targetDir) == "" {
		return func() tea.Msg {
			return piStartMsg{Err: fmt.Errorf("no selected repository")}
		}
	}
	return func() tea.Msg {
		proc, err := pi.Start(targetDir)
		if err != nil {
			return piStartMsg{Err: err}
		}
		return piStartMsg{Proc: proc}
	}
}

func (m *model) readPiEventCmd() tea.Cmd {
	if m.piProc == nil || !m.piProc.Running() {
		return nil
	}
	return m.piProc.ReadLoop()
}

func (m *model) sendPiCmd(payload any) tea.Cmd {
	return func() tea.Msg {
		if m.piProc == nil {
			return piSendMsg{Err: fmt.Errorf("pi not running")}
		}
		return piSendMsg{Err: m.piProc.Send(payload)}
	}
}

func (m *model) stopPi() {
	if m.piProc == nil {
		return
	}
	m.piProc.Stop()
	m.piProc = nil
}

func (m *model) quitCmd() tea.Cmd {
	m.stopPi()
	if m.workspace != nil {
		m.quitting = true
		return func() tea.Msg {
			return workspaceCleanupMsg{Result: m.workspace.CleanupEphemeral()}
		}
	}
	return tea.Quit
}

func (m *model) refreshAgentViewport() {
	m.piui.Refresh(theme.CurrentTheme())
}

func (m *model) updatePIKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.piui.Modal.Active {
		return m.handlePIModalKeys(msg)
	}

	if m.piui.CmdPrefix {
		m.piui.CmdPrefix = false
		switch msg.String() {
		case "p":
			m.stopPi()
			m.mode = modeProjects
			return m, m.inputBox.Focus()
		case "d":
			m.stopPi()
			m.mode = modeDiff
			return m, m.refreshDiffCmd("", "", "")
		case "g":
			m.stopPi()
			m.mode = modeGit
			return m, m.refreshGitCmd()
		case "j":
			m.piui.ScrollDown()
			return m, nil
		case "k":
			m.piui.ScrollUp()
			return m, nil
		case "ctrl+d":
			m.piui.HalfPageDown()
			return m, nil
		case "u":
			m.piui.HalfPageUp()
			return m, nil
		case "n":
			return m, m.sendPiCmd(pi.CmdNewSession())
		case "m":
			return m, m.sendPiCmd(pi.CmdCycleModel())
		case "G":
			m.piui.GotoBottom()
			return m, nil
		}
	}

	if m.piui.SlashActive() {
		switch msg.String() {
		case "up", "k":
			m.piui.MoveSlashCursor(-1)
			return m, nil
		case "down", "j":
			m.piui.MoveSlashCursor(1)
			return m, nil
		case "tab":
			if m.piui.AutocompleteSlashInput() {
				m.refreshAgentViewport()
			}
			return m, nil
		case "enter":
			// handled in general enter flow below
		}
	}

	switch msg.String() {
	case "ctrl+c":
		return m, m.quitCmd()
	case "ctrl+g":
		m.piui.CmdPrefix = true
		return m, nil
	case "ctrl+o":
		m.piui.ToggleToolBody()
		m.refreshAgentViewport()
		return m, nil
	case "ctrl+t":
		m.piui.ToggleThinking()
		m.refreshAgentViewport()
		return m, nil
	case "esc":
		if m.piui.SlashActive() {
			m.piui.Input.SetValue("")
			m.piui.UpdateSlashQuery("")
			return m, nil
		}
		if m.piui.Streaming {
			m.piui.Status = "aborting"
			return m, m.sendPiCmd(pi.CmdAbort())
		}
		m.stopPi()
		m.mode = modeProjects
		return m, m.inputBox.Focus()
	case "s":
		if m.piui.Streaming {
			m.piui.SteerMode = true
			m.piui.Input.SetPlaceholder("Steer message...")
			return m, nil
		}
	case "enter":
		v := strings.TrimSpace(m.piui.Input.Value())
		if v == "" {
			return m, nil
		}
		if m.piui.SlashActive() {
			if selected, ok := m.piui.SelectedSlashCommand(); ok {
				if v == "/" || !m.piui.HasExactSlashCommand(v) {
					v = selected.Name
				}
			}
		}
		m.piui.GotoBottom()
		m.piui.Input.SetValue("")
		m.piui.UpdateSlashQuery("")
		m.piui.AppendUserMessage(v)
		m.refreshAgentViewport()
		if handled, cmd := m.handleSlashCommand(v); handled {
			return m, cmd
		}
		if m.piui.Streaming && !m.piui.SteerMode {
			if strings.HasPrefix(v, "/") {
				return m, m.sendPiCmd(pi.CmdPromptWithStreamingBehavior(v, "steer"))
			}
			return m, m.sendPiCmd(pi.CmdSteer(v))
		}
		if m.piui.SteerMode {
			m.piui.SteerMode = false
			m.piui.Input.SetPlaceholder("Message pi...")
			return m, m.sendPiCmd(pi.CmdSteer(v))
		}
		m.piui.StartStreaming()
		return m, m.sendPiCmd(pi.CmdPrompt(v))
	}

	u, cmd := m.piui.Input.Update(msg)
	m.piui.Input = u.(*input.Model)
	m.piui.UpdateSlashQuery(m.piui.Input.Value())
	if m.piui.SlashActive() && len(m.piui.Commands) == 0 {
		return m, tea.Batch(cmd, m.sendPiCmd(map[string]any{"type": "get_commands"}))
	}
	return m, cmd
}

func (m *model) handlePIModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	modal := m.piui.Modal
	if !modal.Active {
		return m, nil
	}
	respond := func(payload map[string]any) (tea.Model, tea.Cmd) {
		cmdPayload, err := pi.CmdExtensionUIResponse(modal.RequestID, payload)
		m.piui.CloseModal()
		if err != nil {
			m.showError(err.Error())
			return m, nil
		}
		return m, m.sendPiCmd(cmdPayload)
	}

	switch msg.String() {
	case "esc":
		return respond(map[string]any{"cancelled": true})
	}

	switch modal.Method {
	case "select":
		if len(m.piui.Modal.Options) == 0 {
			if msg.String() == "enter" {
				return respond(map[string]any{"cancelled": true})
			}
			return m, nil
		}
		switch msg.String() {
		case "j", "down":
			m.piui.Modal.Cursor = clamp(m.piui.Modal.Cursor+1, 0, len(m.piui.Modal.Options)-1)
			return m, nil
		case "k", "up":
			m.piui.Modal.Cursor = clamp(m.piui.Modal.Cursor-1, 0, len(m.piui.Modal.Options)-1)
			return m, nil
		case "enter":
			if len(m.piui.Modal.Options) == 0 {
				return respond(map[string]any{"cancelled": true})
			}
			return respond(map[string]any{"value": m.piui.Modal.Options[m.piui.Modal.Cursor]})
		}
	case "confirm":
		switch msg.String() {
		case "y", "enter":
			return respond(map[string]any{"confirmed": true})
		case "n":
			return respond(map[string]any{"confirmed": false})
		}
	case "input", "editor":
		if msg.String() == "enter" {
			val := m.piui.Input.Value()
			m.piui.Input.SetValue("")
			return respond(map[string]any{"value": val})
		}
		u, cmd := m.piui.Input.Update(msg)
		m.piui.Input = u.(*input.Model)
		return m, cmd
	default:
		if msg.String() == "enter" {
			return respond(map[string]any{"cancelled": true})
		}
	}

	return m, nil
}

func (m *model) handlePiEvent(evt pi.PiEventMsg) tea.Cmd {
	m.piui.HandleEvent(evt)
	switch evt.Type {
	case "agent_end", "turn_end":
		return tea.Batch(
			m.sendPiCmd(pi.CmdGetState()),
			m.sendPiCmd(map[string]any{"type": "get_session_stats"}),
		)
	default:
		return nil
	}
}

func (m *model) handleSlashCommand(input string) (bool, tea.Cmd) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return false, nil
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false, nil
	}
	name := strings.ToLower(strings.TrimSpace(parts[0]))
	makeTracked := func(cmdName string, payload map[string]any) tea.Cmd {
		id := m.nextSlashID()
		payload["id"] = id
		m.pendingSlash[id] = cmdName
		return m.sendPiCmd(payload)
	}

	switch name {
	case "/models":
		return true, makeTracked(name, map[string]any{"type": "get_available_models"})
	case "/state":
		return true, makeTracked(name, pi.CmdGetState())
	case "/stats":
		return true, makeTracked(name, map[string]any{"type": "get_session_stats"})
	case "/commands":
		return true, makeTracked(name, map[string]any{"type": "get_commands"})
	default:
		return false, nil
	}
}

func (m *model) nextSlashID() string {
	m.slashSeq++
	return fmt.Sprintf("slash-%d", m.slashSeq)
}

func (m *model) appendSlashResponse(cmd string, msg pi.PiResponseMsg) {
	if !msg.Success {
		errText := strings.TrimSpace(msg.Error)
		if errText == "" {
			errText = "command failed"
		}
		m.piui.Messages = append(m.piui.Messages, piui.Message{Role: piui.RoleAssistant, Text: errText})
		return
	}
	var text string
	switch cmd {
	case "/models":
		text = formatModelsText(msg.Data)
	case "/state":
		text = formatStateText(msg.Data)
	case "/stats":
		text = formatStatsText(msg.Data)
	case "/commands":
		text = formatCommandsText(msg.Data)
	default:
		text = "ok"
	}
	if strings.TrimSpace(text) == "" {
		text = "ok"
	}
	m.piui.Messages = append(m.piui.Messages, piui.Message{Role: piui.RoleAssistant, Text: text})
}

func formatModelsText(data map[string]any) string {
	if data == nil {
		return "No models returned."
	}
	raw, _ := data["models"].([]any)
	if len(raw) == 0 {
		return "No models available."
	}
	lines := make([]string, 0, min(len(raw), 12)+1)
	lines = append(lines, "Available models:")
	for i, item := range raw {
		if i >= 12 {
			lines = append(lines, "...")
			break
		}
		m, _ := item.(map[string]any)
		id, _ := m["id"].(string)
		provider, _ := m["provider"].(string)
		name, _ := m["name"].(string)
		label := strings.TrimSpace(name)
		if label == "" {
			label = strings.TrimSpace(id)
		}
		if provider != "" {
			lines = append(lines, fmt.Sprintf("- %s (%s)", label, provider))
		} else {
			lines = append(lines, "- "+label)
		}
	}
	return strings.Join(lines, "\n")
}

func formatStateText(data map[string]any) string {
	if data == nil {
		return "No state available."
	}
	parts := []string{"Session state:"}
	if model, ok := data["model"].(map[string]any); ok {
		name, _ := model["name"].(string)
		id, _ := model["id"].(string)
		if name != "" {
			parts = append(parts, "- model: "+name)
		} else if id != "" {
			parts = append(parts, "- model: "+id)
		}
	}
	if tl, _ := data["thinkingLevel"].(string); tl != "" {
		parts = append(parts, "- thinking: "+tl)
	}
	if sid, _ := data["sessionId"].(string); sid != "" {
		parts = append(parts, "- session: "+sid)
	}
	if pc, ok := data["pendingMessageCount"].(float64); ok {
		parts = append(parts, fmt.Sprintf("- pending: %d", int(pc)))
	}
	return strings.Join(parts, "\n")
}

func formatStatsText(data map[string]any) string {
	if data == nil {
		return "No stats available."
	}
	parts := []string{"Session stats:"}
	if tokens, ok := data["tokens"].(map[string]any); ok {
		if total, ok := tokens["total"].(float64); ok {
			parts = append(parts, fmt.Sprintf("- tokens: %d", int(total)))
		}
	}
	if cost, ok := data["cost"].(float64); ok {
		parts = append(parts, fmt.Sprintf("- cost: $%.4f", cost))
	}
	if toolCalls, ok := data["toolCalls"].(float64); ok {
		parts = append(parts, fmt.Sprintf("- tool calls: %d", int(toolCalls)))
	}
	return strings.Join(parts, "\n")
}

func formatCommandsText(data map[string]any) string {
	if data == nil {
		return "No commands available."
	}
	raw, _ := data["commands"].([]any)
	if len(raw) == 0 {
		return "No commands available."
	}
	parts := []string{"Commands:"}
	for i, item := range raw {
		if i >= 20 {
			parts = append(parts, "...")
			break
		}
		m, _ := item.(map[string]any)
		name, _ := m["name"].(string)
		desc, _ := m["description"].(string)
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		if strings.TrimSpace(desc) != "" {
			parts = append(parts, fmt.Sprintf("- %s  %s", name, desc))
		} else {
			parts = append(parts, "- "+name)
		}
	}
	return strings.Join(parts, "\n")
}

func (m *model) orderRepos(repos []githubauth.Repo) []githubauth.Repo {
	out := make([]githubauth.Repo, 0, len(repos))
	if m.lastRepo != "" {
		for _, r := range repos {
			if r.FullName == m.lastRepo {
				out = append(out, r)
				break
			}
		}
	}
	for _, r := range repos {
		if r.FullName == m.lastRepo {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (m *model) lastRepoIndex() int {
	if len(m.repos) == 0 {
		return 0
	}
	if m.lastRepo == "" {
		return clamp(m.repoCursor, 0, len(m.repos)-1)
	}
	for i, r := range m.repos {
		if r.FullName == m.lastRepo {
			return i
		}
	}
	return clamp(m.repoCursor, 0, len(m.repos)-1)
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

func (m *model) currentRepoLabel() string {
	if name := strings.TrimSpace(m.activeRepoName); name != "" {
		return name
	}
	if m.projectPath != "" {
		return inferRepoNameFromPath(m.projectPath)
	}
	return "no repository"
}

func (m *model) footerRepoLabel() string {
	name := m.currentRepoLabel()
	if name == "" || name == "no repository" {
		return "glib"
	}
	return m.icons.Git + " " + name
}

func inferRepoNameFromPath(path string) string {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return ""
	}
	parts := strings.Split(clean, string(filepath.Separator))
	for _, part := range parts {
		if strings.Contains(part, "__") {
			return strings.Replace(part, "__", "/", 1)
		}
	}
	base := filepath.Base(clean)
	if base == "main" || base == "base" || base == "worktrees" {
		parent := filepath.Base(filepath.Dir(clean))
		if parent != "." && parent != string(filepath.Separator) {
			if strings.Contains(parent, "__") {
				return strings.Replace(parent, "__", "/", 1)
			}
			return parent
		}
	}
	return base
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
