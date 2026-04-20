package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	bdcore "github.com/cloudboy-jh/bento-diffs/pkg/bentodiffs"
	"github.com/cloudboy-jh/bentotui/registry/bricks/dialog"
	"github.com/cloudboy-jh/bentotui/registry/bricks/input"
	selectx "github.com/cloudboy-jh/bentotui/registry/bricks/select"
	"github.com/cloudboy-jh/bentotui/registry/recipes/vimstatus"
	"github.com/cloudboy-jh/bentotui/theme"
	commandpallette "github.com/cloudboy-jh/glib/internal/command-pallette"
	"github.com/cloudboy-jh/glib/internal/diffs"
	"github.com/cloudboy-jh/glib/internal/git"
	"github.com/cloudboy-jh/glib/internal/githubauth"
	"github.com/cloudboy-jh/glib/internal/pi"
	"github.com/cloudboy-jh/glib/internal/piui"
	"github.com/cloudboy-jh/glib/internal/projects"
	"github.com/cloudboy-jh/glib/internal/slash"
	"github.com/cloudboy-jh/glib/internal/workspace"
)

const version = "v0.3.4"
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

type diffViewMode string

const (
	diffViewOpen    diffViewMode = "open"
	diffViewHistory diffViewMode = "history"
)

type gitViewMode string

const (
	gitViewStatus   gitViewMode = "status"
	gitViewBranches gitViewMode = "branches"
	gitViewStash    gitViewMode = "stash"
	gitViewLog      gitViewMode = "log"
)

type promptMode string

const (
	promptNone       promptMode = ""
	promptCloneDest  promptMode = "clone_dest"
	promptCommit     promptMode = "commit"
	promptDiscard    promptMode = "discard"
	promptError      promptMode = "error"
	promptTheme      promptMode = "theme"
	promptNewProj    promptMode = "new_project"
	promptDiffRev    promptMode = "diff_revision"
	promptBranchNew  promptMode = "branch_new"
	promptModelPick  promptMode = "model_pick"
	promptCommitView promptMode = "commit_view"
)

type gitRefreshMsg struct {
	State  git.GitState
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

type diffHistoryMsg struct {
	Commits []git.CommitInfo
	Err     error
}

type gitBranchesMsg struct {
	Branches []string
	Current  string
	Err      error
}

type gitStashMsg struct {
	Items []string
	Err   error
}

type gitLogMsg struct {
	Commits []git.CommitInfo
	Err     error
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
	Repos  []githubauth.Repo
	Err    error
	Page   int
	Append bool
}

type authPollTickMsg struct{}

type piStartMsg struct {
	Proc *pi.PiProcess
	Err  error
}

type piSendMsg struct {
	Err error
}

type piContextMsg struct {
	Text   string
	Status string
	Err    error
}

type workspaceCleanupMsg struct {
	Result workspace.CleanupResult
}

type localTreeRow struct {
	Path  string
	IsDir bool
	Label string
}

type modelPickerItem struct {
	ID       string
	Provider string
	Name     string
	Current  bool
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
	dialogs          *dialog.Manager
	inputBox         *input.Model
	promptInput      *input.Model
	localPicker      *selectx.Model
	themePicker      *selectx.Model
	commitPicker     *selectx.Model
	modelPicker      *selectx.Model
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
	promptBody       string
	pendingURL       string
	pendingPath      string
	git              git.GitState
	diff             diffs.DiffState
	diffViewer       bdcore.Viewer
	diffView         diffViewMode
	diffHistory      []git.CommitInfo
	diffHistoryCur   int
	piProc           *pi.PiProcess
	piRepoPath       string
	piPendingContext string
	piStopRequested  bool
	piRestartTried   bool
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
	repoHasMore      bool
	repoFilter       string
	repoActionOpen   bool
	repoActionCursor int
	pendingLaunch    string
	workspace        *workspace.Manager
	workspaceKind    workspace.Kind
	quitting         bool
	pendingSlash     map[string]string
	slashSeq         int
	reposLoading     bool
	authPollDeadline time.Time
	authPollInterval int
	gitView          gitViewMode
	gitBranches      []string
	gitCurrentBranch string
	gitBranchCursor  int
	gitStash         []string
	gitStashCursor   int
	gitLog           []git.CommitInfo
	gitLogCursor     int
	settings         settingsModel
	modelItems       []modelPickerItem
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
	cp := selectx.New()
	cp.SetPlaceholder("No commits")
	cp.Focus()
	cp.Open()
	mp := selectx.New()
	mp.SetPlaceholder("No models")
	mp.Focus()
	mp.Open()
	piSession := piui.NewSession()

	settings, settingsErr := loadSettingsModel()
	if settingsErr == nil {
		if savedTheme := settings.Theme(); savedTheme != "" {
			_, _ = theme.SetTheme(savedTheme)
		}
	}

	launchDir := resolveInitialLocalDir()
	m := &model{
		footer:        vimstatus.New(theme.CurrentTheme()),
		dialogs:       dialog.New(),
		inputBox:      inp,
		promptInput:   promptInp,
		piui:          piSession,
		localPicker:   lp,
		themePicker:   tp,
		commitPicker:  cp,
		modelPicker:   mp,
		mode:          modeProjects,
		picker:        pickerLocal,
		localDir:      launchDir,
		localExpanded: map[string]bool{},
		icons:         resolveIcons(),
		diff:          diffs.DiffState{},
		diffView:      diffViewHistory,
		gitView:       gitViewStatus,
		authStatus:    githubauth.StatusSignedOut,
		authClientID:  resolveGitHubClientID(),
		repoPage:      1,
		workspaceKind: workspace.KindLocal,
		pendingSlash:  map[string]string{},
		settings:      settings,
	}
	if settingsErr != nil {
		m.statusMessage = "settings unavailable: " + settingsErr.Error()
	}
	m.dialogs.SetTheme(theme.CurrentTheme())
	ws, err := workspace.NewManager(workspace.KindLocal)
	if err == nil {
		m.workspace = ws
	}
	if useMockViews {
		m.ensureDiffViewer()
		m.diffViewer.SetDiffs(diffs.MockDiffs())
		m.git = git.MockGitState()
	}
	if root := m.findRepoRoot(m.localDir); root != "" {
		m.localDir = root
		m.rebindProjectPath(root)
		m.activeRepoName = inferRepoNameFromPath(root)
		m.addRecent(root)
	}
	_ = m.reloadLocalEntries()
	return m
}

func (m *model) Init() tea.Cmd {
	m.syncFooter()
	return tea.Batch(m.inputBox.Focus(), m.bootstrapAuthCmd())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.dialogs != nil && m.dialogs.IsOpen() {
		switch msg.(type) {
		case dialog.OpenMsg, dialog.CloseMsg:
			// lifecycle handled below
		default:
			u, cmd := m.dialogs.Update(msg)
			m.dialogs = u.(*dialog.Manager)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.inputW = clamp(m.width*6/10, 50, 90)
		m.inputBox.SetSize(m.inputW-5, 1)
		m.promptInput.SetSize(m.promptBodyWidth(promptCommit), 1)
		m.piui.SetSize(m.width, m.bodyHeight())
		m.resizeLocalPicker()
		m.footer.SetSize(m.width, 1)
		if m.dialogs != nil {
			m.dialogs.SetSize(m.width, m.height)
		}
		if m.prompt == promptTheme {
			w, h := m.promptPickerSize(promptTheme)
			m.themePicker.SetSize(w, h)
		}
		if m.prompt == promptModelPick {
			w, h := m.promptPickerSize(promptModelPick)
			m.modelPicker.SetSize(w, h)
		}
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
		if m.dialogs != nil {
			m.dialogs.SetTheme(theme.CurrentTheme())
		}
		m.refreshAgentViewport()
		return m, nil

	case dialog.OpenMsg, dialog.CloseMsg:
		u, cmd := m.dialogs.Update(msg)
		m.dialogs = u.(*dialog.Manager)
		return m, cmd

	case commandpallette.ActionMsg:
		return m.handlePaletteAction(msg)

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
		m.diff.Diffs = msg.Diffs
		if strings.TrimSpace(msg.SelectedPath) != "" {
			m.diff.SelectedPath = strings.TrimSpace(msg.SelectedPath)
		} else {
			m.syncDiffSelectedPathFromViewer()
		}
		return m, nil

	case diffHistoryMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.diffHistory = msg.Commits
		m.diffHistoryCur = clamp(m.diffHistoryCur, 0, max(0, len(m.diffHistory)-1))
		m.diffView = diffViewHistory
		m.reloadCommitPicker()
		return m, nil

	case gitBranchesMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.gitBranches = msg.Branches
		m.gitCurrentBranch = msg.Current
		m.gitBranchCursor = clamp(m.gitBranchCursor, 0, max(0, len(m.gitBranches)-1))
		m.gitView = gitViewBranches
		return m, nil

	case gitStashMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.gitStash = msg.Items
		m.gitStashCursor = clamp(m.gitStashCursor, 0, max(0, len(m.gitStash)-1))
		m.gitView = gitViewStash
		return m, nil

	case gitLogMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.gitLog = msg.Commits
		m.gitLogCursor = clamp(m.gitLogCursor, 0, max(0, len(m.gitLog)-1))
		m.gitView = gitViewLog
		return m, nil

	case git.OpenDiffMsg:
		m.mode = modeDiff
		m.diffView = diffViewOpen
		m.ensureDiffViewer()
		if useMockViews {
			m.diffViewer.SetDiffs(diffs.MockDiffs())
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
		if strings.TrimSpace(m.projectPath) != "" && strings.TrimSpace(m.projectPath) != strings.TrimSpace(msg.ProjectPath) {
			m.piPendingContext = ""
		}
		m.rebindProjectPath(msg.ProjectPath)
		if strings.TrimSpace(m.pendingRepoName) != "" {
			m.activeRepoName = strings.TrimSpace(m.pendingRepoName)
			m.pendingRepoName = ""
		} else {
			m.activeRepoName = inferRepoNameFromPath(m.projectPath)
		}
		m.addRecent(m.projectPath)
		m.statusMessage = "repo ready"
		launch := m.pendingLaunch
		m.pendingLaunch = ""
		m.repoActionOpen = false
		switch launch {
		case "diff":
			m.mode = modeDiff
			m.diffView = diffViewHistory
			m.ensureDiffViewer()
			if useMockViews {
				m.diffViewer.SetDiffs(diffs.MockDiffs())
				return m, nil
			}
			return m, m.loadDiffHistoryCmd()
		case "git":
			m.mode = modeGit
			if useMockViews {
				m.git = git.MockGitState()
				return m, nil
			}
			return m, m.refreshGitCmd()
		case "pi":
			m.mode = modePI
			if m.piSessionActiveForRepo(m.projectPath) {
				return m, nil
			}
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
		m.authPollInterval = max(1, msg.Device.Interval)
		m.authPollDeadline = time.Now().Add(time.Duration(max(1, msg.Device.ExpiresIn)) * time.Second)
		m.statusMessage = "approve device code in browser"
		return m, tea.Batch(m.pollAuthCmd(msg.Device), m.authPollTickCmd())

	case authTokenMsg:
		if msg.Err != nil {
			m.authStatus = githubauth.StatusExpired
			m.authPollDeadline = time.Time{}
			m.authPollInterval = 0
			m.showError(msg.Err.Error())
			return m, nil
		}
		m.authToken = msg.Token
		m.authStatus = githubauth.StatusAuth
		m.authPollDeadline = time.Time{}
		m.authPollInterval = 0
		m.mode = modeProjects
		m.picker = pickerRepos
		m.statusMessage = "github auth complete"
		m.repoPage = 1
		m.repoHasMore = true
		m.repoFilter = ""
		return m, tea.Batch(m.persistTokenCmd(msg.Token), m.loadReposPageCmd(1, false))

	case authPollTickMsg:
		if m.authStatus != githubauth.StatusPending {
			return m, nil
		}
		if !m.authPollDeadline.IsZero() && time.Now().After(m.authPollDeadline) {
			return m, nil
		}
		return m, m.authPollTickCmd()

	case reposMsg:
		m.reposLoading = false
		if msg.Err != nil {
			if errors.Is(msg.Err, githubauth.ErrTokenExpired) {
				m.authToken = ""
				m.authStatus = githubauth.StatusExpired
				m.authDevice = githubauth.DeviceCode{}
				m.repoPage = 1
				m.repoHasMore = false
				m.repos = nil
				m.showError(msg.Err.Error())
				return m, m.clearTokenCmd()
			}
			m.showError(msg.Err.Error())
			return m, nil
		}
		if msg.Append {
			for _, repo := range msg.Repos {
				exists := false
				for _, existing := range m.repos {
					if existing.FullName == repo.FullName {
						exists = true
						break
					}
				}
				if !exists {
					m.repos = append(m.repos, repo)
				}
			}
			m.repoPage = max(m.repoPage, msg.Page)
		} else {
			m.repos = m.orderRepos(msg.Repos)
			m.repoPage = max(1, msg.Page)
		}
		m.repoHasMore = len(msg.Repos) >= 100
		if len(m.repos) == 0 {
			m.repoCursor = 0
		} else {
			m.repoCursor = clamp(m.repoCursor, 0, max(0, m.repoDisplayLen()-1))
		}
		m.repoActionOpen = false
		m.repoActionCursor = 0
		m.picker = pickerRepos
		if !msg.Append {
			m.statusMessage = "pick a repo and press enter to get started"
		}
		return m, nil

	case piStartMsg:
		if msg.Err != nil {
			if m.piRestartTried {
				m.piRestartTried = false
				m.piui.Status = "disconnected"
				m.showError("pi restart failed: " + msg.Err.Error())
				return m, nil
			}
			m.mode = modeProjects
			m.showError("pi failed to start: " + msg.Err.Error())
			return m, nil
		}
		m.piStopRequested = false
		m.piRestartTried = false
		m.piProc = msg.Proc
		m.piRepoPath = strings.TrimSpace(m.projectPath)
		m.piui.Status = "connected"
		m.refreshAgentViewport()
		base := []tea.Cmd{
			m.readPiEventCmd(),
			piui.SpinnerTickCmd(m.piui.SpinnerInterval()),
			m.sendPiCmd(pi.CmdSteer(m.repoBoundaryInstruction(m.projectPath))),
			m.sendPiCmd(pi.CmdGetState()),
			m.sendPiCmd(map[string]any{"type": "get_commands"}),
			m.sendPiCmd(map[string]any{"type": "get_session_stats"}),
		}
		if modelID := strings.TrimSpace(m.settings.Model()); modelID != "" {
			provider := strings.TrimSpace(m.settings.ModelProvider())
			if provider == "" {
				provider = "openai-codex"
			}
			base = append(base, m.sendPiCmd(pi.CmdSetModel(provider, modelID)))
		}
		if strings.TrimSpace(m.piPendingContext) != "" {
			ctx := m.piPendingContext
			m.piPendingContext = ""
			base = append(base, m.sendPiCmd(pi.CmdPrompt(ctx)))
		}
		return m, tea.Batch(base...)

	case piSendMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		return m, m.readPiEventCmd()

	case piContextMsg:
		if msg.Err != nil {
			m.showError(msg.Err.Error())
			return m, nil
		}
		if strings.TrimSpace(msg.Status) != "" {
			m.statusMessage = strings.TrimSpace(msg.Status)
		}
		if strings.TrimSpace(msg.Text) == "" {
			return m, nil
		}
		m.mode = modePI
		if strings.TrimSpace(msg.Status) == "" {
			m.statusMessage = "sent diff context to PI"
		}
		if m.piSessionActiveForRepo(m.projectPath) {
			return m, m.sendPiCmd(pi.CmdPromptWithStreamingBehavior(msg.Text, "steer"))
		}
		m.piPendingContext = msg.Text
		return m, m.startPiCmd()

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
		wasRequested := m.piStopRequested
		m.piStopRequested = false
		activeRepo := m.normalizeRepoPath(m.projectPath)
		procRepo := m.normalizeRepoPath(m.piRepoPath)
		sameRepo := activeRepo != "" && procRepo != "" && activeRepo == procRepo
		restartEligible := !wasRequested && m.mode == modePI && sameRepo && git.IsGitRepo(activeRepo) && !m.piRestartTried
		if restartEligible {
			draft := strings.TrimSpace(m.piui.Input.Value())
			if draft != "" && strings.TrimSpace(m.piPendingContext) == "" {
				m.piPendingContext = draft
			}
			if m.piui.Streaming && strings.TrimSpace(m.piPendingContext) == "" {
				m.piPendingContext = "Continue from where we left off after the reconnect."
			}
		}
		m.stopPi()
		m.piui.StopStreaming()
		m.piui.Streaming = false
		m.piui.ToolRunning = false
		m.piui.Status = "stopped"
		if restartEligible {
			m.piRestartTried = true
			m.piui.Status = "reconnecting"
			m.statusMessage = "pi disconnected, reconnecting"
			return m, m.startPiCmd()
		}
		if msg.Err != nil && !wasRequested && !isBenignPiExit(msg.Err) {
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
			return m, piui.SpinnerTickCmd(m.piui.SpinnerInterval())
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
		rawKey := msg.String()
		if keyMatchesShortcut(msg, "ctrl+c") {
			return m, m.quitCmd()
		}
		if keyMatchesShortcut(msg, "ctrl+space") {
			return m, m.cycleModes()
		}
		if keyMatchesShortcut(msg, "ctrl+o") || keyMatchesShortcut(msg, "ctrl+/") {
			return m, commandpallette.Open(string(m.mode), m.width, m.height)
		}

		switch rawKey {
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
			if !(m.mode == modeDiff && m.diffView == diffViewOpen) {
				return m, m.jumpToPI()
			}
		case "D":
			return m, m.jumpToDiff()
		case "G":
			return m, m.jumpToGit()
		case "d":
			return m, m.jumpToDiff()
		case "g":
			return m, m.jumpToGit()
		case "p":
			return m, m.jumpToProjects()
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
					m.authPollDeadline = time.Time{}
					m.authPollInterval = 0
					m.repos = nil
					m.repoFilter = ""
					m.repoPage = 1
					m.repoHasMore = false
					m.picker = pickerLocal
					m.statusMessage = "github token cleared"
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
					m.repoCursor = clamp(m.repoCursor+1, 0, max(0, m.repoDisplayLen()-1))
					if m.shouldFetchMoreRepos() {
						return m, m.loadReposPageCmd(m.repoPage+1, true)
					}
					return m, nil
				case "k", "up":
					m.repoCursor = clamp(m.repoCursor-1, 0, max(0, m.repoDisplayLen()-1))
					return m, nil
				case "backspace":
					if m.repoFilter != "" {
						r := []rune(m.repoFilter)
						m.repoFilter = string(r[:len(r)-1])
						m.repoCursor = clamp(m.repoCursor, 0, max(0, m.repoDisplayLen()-1))
					}
					return m, nil
				case "esc":
					if m.repoFilter != "" {
						m.repoFilter = ""
						m.repoCursor = clamp(m.repoCursor, 0, max(0, m.repoDisplayLen()-1))
						return m, nil
					}
				case "n":
					m.openPrompt(promptNewProj, "New Project", "Enter folder path to create + git init", filepath.Join(m.localDir, "new-project"))
					return m, m.promptInput.Focus()
				case "r":
					m.repoPage = 1
					m.repoHasMore = true
					return m, m.loadReposPageCmd(1, false)
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
					if m.repoDisplayLen() == 0 {
						return m, nil
					}
					m.repoActionOpen = true
					m.repoActionCursor = 0
					return m, nil
				}
				if typed := repoFilterKey(msg.String()); typed != "" {
					m.repoFilter += typed
					m.repoCursor = clamp(m.repoCursor, 0, max(0, m.repoDisplayLen()-1))
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
			if m.diffView == diffViewHistory {
				switch msg.String() {
				case "j", "down":
					m.diffHistoryCur = clamp(m.diffHistoryCur+1, 0, max(0, len(m.diffHistory)-1))
				case "k", "up":
					m.diffHistoryCur = clamp(m.diffHistoryCur-1, 0, max(0, len(m.diffHistory)-1))
				case "c":
					m.diffView = diffViewOpen
					return m, m.refreshDiffCmd("", "", "")
				case "enter":
					if len(m.diffHistory) > 0 {
						sha := strings.TrimSpace(m.diffHistory[m.diffHistoryCur].Hash)
						if sha != "" {
							m.diffView = diffViewOpen
							return m, m.refreshDiffCmd("commit", sha, "")
						}
					}
				case "l":
					if len(m.diffHistory) > 0 {
						c := m.diffHistory[m.diffHistoryCur]
						body := strings.TrimSpace(c.Message)
						if body == "" {
							body = "(no commit title)"
						}
						hash := strings.TrimSpace(c.Hash)
						if hash != "" {
							body = hash + "\n\n" + body
						}
						m.openCommitViewPrompt("Commit", "enter/esc close", body)
						return m, nil
					}
				case "esc", "q":
					m.mode = modeProjects
				}
				return m, nil
			}

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
				m.jumpDiffFile(1)
			case "N":
				m.jumpDiffFile(-1)
			case "tab":
				m.jumpDiffFile(1)
			case "shift+tab":
				m.jumpDiffFile(-1)
			case "c":
				return m, m.loadDiffHistoryCmd()
			case "}":
				m.diffViewer.NextHunk()
			case "{":
				m.diffViewer.PrevHunk()
			case "home":
				m.diffViewer.ScrollUp(m.diffViewer.State().Scroll)
			case "end":
				state := m.diffViewer.State()
				m.diffViewer.ScrollDown(max(0, state.MaxScroll-state.Scroll))
			case "i":
				return m, m.sendDiffContextToPiCmd()
			case "g", "G":
				return m, m.jumpToGit()
			case "q", "esc":
				m.diffView = diffViewHistory
				if len(m.diffHistory) == 0 {
					return m, m.loadDiffHistoryCmd()
				}
			}
			return m, nil
		}

		if m.mode == modeGit {
			if m.gitView == gitViewBranches {
				switch msg.String() {
				case "j", "down":
					m.gitBranchCursor = clamp(m.gitBranchCursor+1, 0, max(0, len(m.gitBranches)-1))
				case "k", "up":
					m.gitBranchCursor = clamp(m.gitBranchCursor-1, 0, max(0, len(m.gitBranches)-1))
				case "n":
					m.openPrompt(promptBranchNew, "New Branch", "Enter branch name", "")
					return m, m.promptInput.Focus()
				case "D":
					if len(m.gitBranches) > 0 {
						name := strings.TrimSpace(m.gitBranches[m.gitBranchCursor])
						if name != "" && name != m.gitCurrentBranch {
							return m, m.deleteBranchCmd(name)
						}
					}
				case "enter":
					if len(m.gitBranches) > 0 {
						name := strings.TrimSpace(m.gitBranches[m.gitBranchCursor])
						if name != "" {
							return m, m.switchBranchCmd(name)
						}
					}
				case "esc", "q":
					m.gitView = gitViewStatus
				}
				return m, nil
			}
			if m.gitView == gitViewStash {
				switch msg.String() {
				case "j", "down":
					m.gitStashCursor = clamp(m.gitStashCursor+1, 0, max(0, len(m.gitStash)-1))
				case "k", "up":
					m.gitStashCursor = clamp(m.gitStashCursor-1, 0, max(0, len(m.gitStash)-1))
				case "esc", "q":
					m.gitView = gitViewStatus
				}
				return m, nil
			}
			if m.gitView == gitViewLog {
				switch msg.String() {
				case "j", "down":
					m.gitLogCursor = clamp(m.gitLogCursor+1, 0, max(0, len(m.gitLog)-1))
				case "k", "up":
					m.gitLogCursor = clamp(m.gitLogCursor-1, 0, max(0, len(m.gitLog)-1))
				case "enter":
					if len(m.gitLog) > 0 {
						sha := strings.TrimSpace(m.gitLog[m.gitLogCursor].Hash)
						if sha != "" {
							m.mode = modeDiff
							m.diffView = diffViewOpen
							return m, m.refreshDiffCmd("commit", sha, "")
						}
					}
				case "esc", "q":
					m.gitView = gitViewStatus
				}
				return m, nil
			}

			switch msg.String() {
			case "j", "down":
				m.git.MoveCursor(1)
			case "k", "up":
				m.git.MoveCursor(-1)
			case "s":
				return m, m.stageFileCmd()
			case "a":
				return m, m.stageAllCmd()
			case "u":
				return m, m.unstageFileCmd()
			case "A":
				return m, m.unstageAllCmd()
			case "d":
				if f, ok := m.git.SelectedFile(); ok {
					m.pendingPath = f.Path
					m.openPrompt(promptDiscard, "Discard changes", "Discard selected file? y/n", "n")
					return m, m.promptInput.Focus()
				}
			case "c":
				m.openPrompt(promptCommit, "Commit", "Enter commit message", "")
				return m, m.promptInput.Focus()
			case "p":
				return m, m.pushCmd()
			case "P":
				return m, m.pullCmd()
			case "f":
				return m, m.fetchCmd()
			case "b":
				return m, m.loadBranchesCmd()
			case "l":
				return m, m.loadGitLogCmd()
			case "z":
				return m, m.stashPushCmd()
			case "Z":
				return m, m.stashPopCmd()
			case "?":
				return m, m.loadStashCmd()
			case "i":
				return m, m.sendStagedDiffToPiCmd()
			case "enter":
				return m, m.git.OpenSelectedDiffCmd()
			case "q", "esc":
				return m, m.jumpToProjects()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *model) queueRepoLaunch(launch string) (tea.Cmd, bool) {
	if m.mode != modeProjects || m.picker != pickerRepos {
		return nil, false
	}
	repo, ok := m.selectedRepo()
	if !ok {
		return nil, false
	}
	if m.activeRepoName == repo.FullName && strings.TrimSpace(m.projectPath) != "" && git.IsGitRepo(m.projectPath) {
		return nil, false
	}
	m.pendingLaunch = launch
	m.pendingRepoName = repo.FullName
	m.statusMessage = "opening " + launch + " for " + repo.FullName
	return m.openRepoCmd(repo), true
}

func (m *model) jumpToProjects() tea.Cmd {
	m.mode = modeProjects
	return m.inputBox.Focus()
}

func (m *model) jumpToDiff() tea.Cmd {
	if cmd, queued := m.queueRepoLaunch("diff"); queued {
		return cmd
	}
	if m.projectPath == "" {
		m.showError("select a repository first")
		return nil
	}
	m.mode = modeDiff
	m.diffView = diffViewHistory
	m.ensureDiffViewer()
	if useMockViews {
		m.diffViewer.SetDiffs(diffs.MockDiffs())
		return nil
	}
	return m.loadDiffHistoryCmd()
}

func (m *model) jumpToGit() tea.Cmd {
	if cmd, queued := m.queueRepoLaunch("git"); queued {
		return cmd
	}
	if m.projectPath == "" {
		m.showError("select a repository first")
		return nil
	}
	m.mode = modeGit
	m.gitView = gitViewStatus
	if useMockViews {
		m.git = git.MockGitState()
		return nil
	}
	return m.refreshGitCmd()
}

func (m *model) jumpToPI() tea.Cmd {
	if cmd, queued := m.queueRepoLaunch("pi"); queued {
		return cmd
	}
	if m.projectPath == "" {
		m.showError("select a repository first")
		return nil
	}
	m.mode = modePI
	if m.piSessionActiveForRepo(m.projectPath) {
		return nil
	}
	return m.startPiCmd()
}

func (m *model) cycleModes() tea.Cmd {
	switch m.mode {
	case modeDiff:
		return m.jumpToPI()
	case modePI:
		return m.jumpToGit()
	case modeGit:
		return m.jumpToDiff()
	default:
		return m.jumpToDiff()
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
			if err := m.settings.SetTheme(sel.Value); err != nil {
				m.showError("failed to save theme: " + err.Error())
				return cmd
			}
			m.statusMessage = "theme: " + sel.Label
			m.closePrompt()
			return cmd
		}

		return cmd
	}

	if m.prompt == promptModelPick {
		u, cmd := m.modelPicker.Update(msg)
		m.modelPicker = u.(*selectx.Model)

		switch msg.String() {
		case "esc":
			m.closePrompt()
			return cmd
		case "enter":
			sel, ok := m.modelPicker.Selected()
			if !ok {
				return cmd
			}
			modelID := strings.TrimSpace(sel.Value)
			if modelID == "" {
				return cmd
			}
			provider := strings.TrimSpace(m.modelProviderByID(modelID))
			if provider == "" {
				provider = "openai-codex"
			}
			if err := m.settings.SetModel(provider, modelID); err != nil {
				m.showError("failed to save model: " + err.Error())
				return cmd
			}
			m.statusMessage = "model: " + modelID
			m.closePrompt()
			return tea.Batch(cmd, m.trackedPiCmd("/models:set", pi.CmdSetModel(provider, modelID)))
		}

		return cmd
	}

	if m.prompt == promptCommitView {
		switch msg.String() {
		case "esc", "enter":
			m.closePrompt()
			return nil
		}
		return nil
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
			if !isAffirmative(val) {
				m.statusMessage = "discard cancelled"
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
			m.diffView = diffViewOpen
			return m.refreshDiffCmd("commit", val, "")
		case promptBranchNew:
			m.closePrompt()
			if val == "" {
				m.showError("branch name cannot be empty")
				return nil
			}
			return m.createBranchCmd(val)
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
	m.promptBody = ""
	m.promptInput.SetValue(initial)
	m.promptInput.SetSize(m.promptBodyWidth(kind), 1)
}

func (m *model) openCommitViewPrompt(title, hint, body string) {
	m.prompt = promptCommitView
	m.promptTitle = title
	m.promptHint = hint
	m.promptBody = strings.TrimSpace(body)
	m.promptInput.SetValue("")
}

func (m *model) closePrompt() {
	m.prompt = promptNone
	m.promptTitle = ""
	m.promptHint = ""
	m.promptBody = ""
	m.pendingURL = ""
	m.pendingPath = ""
	m.promptInput.SetValue("")
}

func (m *model) showError(errText string) {
	m.errorText = humanizeError(errText)
	m.prompt = promptError
	m.promptTitle = "Error"
	m.promptHint = "Press enter or esc"
	m.promptInput.SetValue("")
	m.promptInput.SetSize(m.promptBodyWidth(promptError), 1)
}

func (m *model) promptBoxWidth(kind promptMode) int {
	availW := max(44, m.width-6)
	target := 64
	switch kind {
	case promptCommitView:
		target = 84
	case promptError:
		target = 72
	}
	return clamp(target, 44, availW)
}

func (m *model) promptBodyWidth(kind promptMode) int {
	return max(20, m.promptBoxWidth(kind)-6)
}

func (m *model) promptPickerSize(kind promptMode) (int, int) {
	w := m.promptBodyWidth(kind)
	targetH := 14
	if kind == promptTheme {
		targetH = 12
	}
	maxH := max(8, m.bodyHeight()-10)
	h := clamp(targetH, 8, maxH)
	return w, h
}

func humanizeError(errText string) string {
	msg := strings.TrimSpace(errText)
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "github repos failed: 401"):
		return "Your GitHub token expired. Press enter to sign in again."
	case strings.HasPrefix(lower, "not a git repo:"):
		return "This directory isn't a git repository. Pick a different project."
	case strings.Contains(lower, "pi failed to start") && strings.Contains(lower, "executable file not found"):
		return "pi is not installed. Install it from https://shittycodingagent.ai and try again."
	case strings.Contains(lower, "github token expired"):
		return "Your GitHub token expired. Press enter to sign in again."
	case strings.Contains(lower, "device flow token expired"):
		return "Sign-in timed out. Press enter to request a new GitHub device code."
	case strings.Contains(lower, "device flow access denied"):
		return "GitHub sign-in was denied. Press enter to try again."
	case strings.Contains(lower, "device flow timed out"):
		return "Sign-in timed out. Press enter to request a new GitHub device code."
	default:
		return msg
	}
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
		if git.IsGitRepo(row.Path) {
			m.rebindProjectPath(row.Path)
			m.activeRepoName = inferRepoNameFromPath(m.projectPath)
			m.addRecent(m.projectPath)
			m.statusMessage = "project selected"
			return
		}
		m.localExpanded[row.Path] = !m.localExpanded[row.Path]
		m.rebuildLocalTree()
		return
	}
	if root := m.findRepoRoot(row.Path); root != "" {
		m.rebindProjectPath(root)
		m.activeRepoName = inferRepoNameFromPath(root)
		m.addRecent(root)
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
		Git:      "\ue725", // nf-dev-git_branch  (gh-dash style)
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

func resolveInitialLocalDir() string {
	candidates := []string{
		os.Getenv("GLIB_EDITOR_PATH"),
		os.Getenv("GLIB_EDITOR_CWD"),
		os.Getenv("VSCODE_CWD"),
		os.Getenv("PWD"),
	}
	for _, raw := range candidates {
		if path := normalizeDirCandidate(raw); path != "" {
			return path
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return cwd
}

func normalizeDirCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	abs, err := filepath.Abs(raw)
	if err == nil {
		raw = abs
	}
	st, err := os.Stat(raw)
	if err != nil {
		return ""
	}
	if st.IsDir() {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Dir(raw))
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

func (m *model) authPollTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return authPollTickMsg{}
	})
}

func (m *model) loadReposCmd() tea.Cmd {
	m.repoPage = 1
	m.repoHasMore = true
	return m.loadReposPageCmd(1, false)
}

func (m *model) loadReposPageCmd(page int, appendPage bool) tea.Cmd {
	if strings.TrimSpace(m.authToken) == "" {
		return nil
	}
	if page < 1 {
		page = 1
	}
	if appendPage && (!m.repoHasMore || m.reposLoading) {
		return nil
	}
	m.reposLoading = true
	return func() tea.Msg {
		repos, err := githubauth.ListRepos(m.authToken, page, 100)
		return reposMsg{Repos: repos, Err: err, Page: page, Append: appendPage}
	}
}

func (m *model) filteredRepos() []githubauth.Repo {
	query := strings.TrimSpace(m.repoFilter)
	if query == "" {
		return m.repos
	}
	out := make([]githubauth.Repo, 0, len(m.repos))
	for _, repo := range m.repos {
		if fuzzyContains(query, repo.FullName) {
			out = append(out, repo)
		}
	}
	return out
}

func (m *model) repoDisplayRows() []githubauth.Repo {
	return m.filteredRepos()
}

func (m *model) repoDisplayLen() int {
	return len(m.repoDisplayRows())
}

func (m *model) repoAtCursor() (githubauth.Repo, bool) {
	rows := m.repoDisplayRows()
	if len(rows) == 0 {
		return githubauth.Repo{}, false
	}
	idx := clamp(m.repoCursor, 0, len(rows)-1)
	return rows[idx], true
}

func (m *model) shouldFetchMoreRepos() bool {
	if m.reposLoading || !m.repoHasMore || strings.TrimSpace(m.repoFilter) != "" {
		return false
	}
	if m.repoDisplayLen() == 0 {
		return false
	}
	return m.repoCursor >= m.repoDisplayLen()-1
}

func (m *model) selectedRepo() (githubauth.Repo, bool) {
	return m.repoAtCursor()
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
			state := git.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state}
		}
	}
	if m.projectPath == "" {
		m.showError("select a project first")
		return nil
	}
	return func() tea.Msg {
		if !git.IsGitRepo(m.projectPath) {
			return gitRefreshMsg{Err: fmt.Errorf("not a git repo: %s", m.projectPath)}
		}
		state, err := git.Refresh(m.projectPath)
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
			return diffRefreshMsg{Diffs: diffs.MockDiffs(), Source: "mock", SelectedPath: selectedPath}
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
		if !git.IsGitRepo(m.projectPath) {
			return diffRefreshMsg{Err: fmt.Errorf("not a git repo: %s", m.projectPath)}
		}
		var out string
		var err error
		args := []string{}
		switch source {
		case "working":
			working, _, wErr := git.RunGit(m.projectPath, "diff")
			if wErr != nil {
				return diffRefreshMsg{Err: wErr}
			}
			staged, _, sErr := git.RunGit(m.projectPath, "diff", "--cached")
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
		if err == nil && out == "" && len(args) > 0 {
			out, _, err = git.RunGit(m.projectPath, args...)
		}
		if err != nil {
			return diffRefreshMsg{Err: err}
		}
		if strings.TrimSpace(out) == "" {
			return diffRefreshMsg{
				Diffs:        []bdcore.DiffResult{},
				Source:       source,
				CommitSHA:    commitSHA,
				ProjectDir:   m.projectPath,
				SelectedPath: selectedPath,
			}
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
			state := git.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: staged file"}
		}
	}
	f, ok := m.git.SelectedFile()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if err := git.StageFile(m.projectPath, f.Path); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "staged " + f.Path}
	}
}

func (m *model) stageAllCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := git.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: staged all"}
		}
	}
	return func() tea.Msg {
		if m.projectPath == "" {
			return gitRefreshMsg{Err: fmt.Errorf("select a project first")}
		}
		if _, _, err := git.RunGit(m.projectPath, "add", "-A"); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "staged all changes"}
	}
}

func (m *model) unstageFileCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := git.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: unstaged file"}
		}
	}
	f, ok := m.git.SelectedFile()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if err := git.UnstageFile(m.projectPath, f.Path); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "unstaged " + f.Path}
	}
}

func (m *model) discardFileCmd(path string) tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := git.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: discarded " + path}
		}
	}
	return func() tea.Msg {
		if err := git.DiscardFile(m.projectPath, path); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "discarded " + path}
	}
}

func (m *model) commitCmd(message string) tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := git.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: commit created"}
		}
	}
	return func() tea.Msg {
		if err := git.Commit(m.projectPath, message); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "commit created"}
	}
}

func (m *model) pushCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			state := git.MockGitState()
			state.Cursor = m.git.Cursor
			return gitRefreshMsg{State: state, Action: "mock: pushed"}
		}
	}
	return func() tea.Msg {
		if err := git.Push(m.projectPath); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "pushed"}
	}
}

func (m *model) pullCmd() tea.Cmd {
	if useMockViews {
		return m.refreshGitCmd()
	}
	return func() tea.Msg {
		if err := git.Pull(m.projectPath); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "pulled latest"}
	}
}

func (m *model) fetchCmd() tea.Cmd {
	if useMockViews {
		return m.refreshGitCmd()
	}
	return func() tea.Msg {
		if err := git.Fetch(m.projectPath); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "fetched remotes"}
	}
}

func (m *model) unstageAllCmd() tea.Cmd {
	if useMockViews {
		return m.refreshGitCmd()
	}
	return func() tea.Msg {
		if err := git.UnstageAll(m.projectPath); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "unstaged all changes"}
	}
}

func (m *model) loadDiffHistoryCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			return diffHistoryMsg{Commits: []git.CommitInfo{{Hash: "HEAD", Message: "mock commit"}}}
		}
	}
	return func() tea.Msg {
		commits, err := git.CommitLog(m.projectPath, 100)
		return diffHistoryMsg{Commits: commits, Err: err}
	}
}

func (m *model) loadBranchesCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			return gitBranchesMsg{Branches: []string{"main", "feature/mock"}, Current: "main"}
		}
	}
	return func() tea.Msg {
		branches, current, err := git.BranchList(m.projectPath)
		return gitBranchesMsg{Branches: branches, Current: current, Err: err}
	}
}

func (m *model) createBranchCmd(name string) tea.Cmd {
	if useMockViews {
		m.gitCurrentBranch = strings.TrimSpace(name)
		return nil
	}
	return func() tea.Msg {
		if err := git.BranchCreate(m.projectPath, name); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "created branch " + strings.TrimSpace(name)}
	}
}

func (m *model) switchBranchCmd(name string) tea.Cmd {
	if useMockViews {
		m.gitCurrentBranch = strings.TrimSpace(name)
		m.gitView = gitViewStatus
		return nil
	}
	return func() tea.Msg {
		if err := git.BranchSwitch(m.projectPath, name); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "switched to " + strings.TrimSpace(name)}
	}
}

func (m *model) deleteBranchCmd(name string) tea.Cmd {
	if useMockViews {
		return nil
	}
	return func() tea.Msg {
		if err := git.BranchDelete(m.projectPath, name); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "deleted branch " + strings.TrimSpace(name)}
	}
}

func (m *model) loadStashCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			return gitStashMsg{Items: []string{"stash@{0}: WIP mock"}}
		}
	}
	return func() tea.Msg {
		items, err := git.StashList(m.projectPath, 40)
		return gitStashMsg{Items: items, Err: err}
	}
}

func (m *model) stashPushCmd() tea.Cmd {
	if useMockViews {
		return m.refreshGitCmd()
	}
	return func() tea.Msg {
		if err := git.StashPush(m.projectPath, ""); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "stashed changes"}
	}
}

func (m *model) stashPopCmd() tea.Cmd {
	if useMockViews {
		return m.refreshGitCmd()
	}
	return func() tea.Msg {
		if err := git.StashPop(m.projectPath); err != nil {
			return gitRefreshMsg{Err: err}
		}
		state, err := git.Refresh(m.projectPath)
		if err != nil {
			return gitRefreshMsg{Err: err}
		}
		return gitRefreshMsg{State: state, Action: "popped stash"}
	}
}

func (m *model) loadGitLogCmd() tea.Cmd {
	if useMockViews {
		return func() tea.Msg {
			return gitLogMsg{Commits: []git.CommitInfo{{Hash: "HEAD", Message: "mock log"}}}
		}
	}
	return func() tea.Msg {
		commits, err := git.CommitLog(m.projectPath, 30)
		return gitLogMsg{Commits: commits, Err: err}
	}
}

func (m *model) cloneRepoCmd(url, dest string) tea.Cmd {
	return func() tea.Msg {
		projectPath, err := git.Clone(url, dest)
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
		if _, _, err := git.RunGit(abs, "init"); err != nil {
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

func diffPath(d bdcore.DiffResult) string {
	newName := strings.TrimSpace(strings.TrimPrefix(d.NewFile, "b/"))
	oldName := strings.TrimSpace(strings.TrimPrefix(d.OldFile, "a/"))
	displayName := strings.TrimSpace(strings.TrimPrefix(d.DisplayFile, "b/"))
	switch {
	case displayName != "":
		return displayName
	case newName != "":
		return newName
	default:
		return oldName
	}
}

func (m *model) syncDiffSelectedPathFromViewer() {
	if m.diffViewer == nil || len(m.diff.Diffs) == 0 {
		return
	}
	st := m.diffViewer.State()
	idx := clamp(st.ActiveFile, 0, len(m.diff.Diffs)-1)
	m.diff.SelectedPath = diffPath(m.diff.Diffs[idx])
}

func (m *model) jumpDiffFile(delta int) {
	if m.diffViewer == nil || delta == 0 {
		return
	}
	before := m.diffViewer.State()
	if delta > 0 {
		for i := 0; i < delta; i++ {
			m.diffViewer.NextFile()
		}
	} else {
		for i := 0; i < -delta; i++ {
			m.diffViewer.PrevFile()
		}
	}
	after := m.diffViewer.State()
	if after.ActiveFile != before.ActiveFile && after.Scroll > 0 {
		m.diffViewer.ScrollUp(after.Scroll)
	}
	m.syncDiffSelectedPathFromViewer()
}

func (m *model) piSessionActiveForRepo(repoPath string) bool {
	repoPath = m.normalizeRepoPath(repoPath)
	if repoPath == "" || m.piProc == nil || !m.piProc.Running() {
		return false
	}
	return m.normalizeRepoPath(m.piRepoPath) == repoPath
}

func (m *model) sendDiffContextToPiCmd() tea.Cmd {
	if strings.TrimSpace(m.projectPath) == "" {
		return nil
	}
	m.syncDiffSelectedPathFromViewer()
	repoPath := strings.TrimSpace(m.projectPath)
	source := strings.TrimSpace(m.diff.Source)
	if source == "" {
		source = "working"
	}
	commitSHA := strings.TrimSpace(m.diff.CommitSHA)
	selected := strings.TrimSpace(m.diff.SelectedPath)
	return func() tea.Msg {
		var (
			d   string
			err error
		)
		switch source {
		case "commit":
			if commitSHA == "" {
				return piContextMsg{Err: fmt.Errorf("missing commit SHA for diff context")}
			}
			d, err = diffs.DiffForCommitFile(repoPath, commitSHA, selected)
		default:
			d, err = diffs.DiffForFile(repoPath, selected)
		}
		if err != nil {
			return piContextMsg{Err: err}
		}
		if strings.TrimSpace(d) == "" {
			return piContextMsg{Status: "no diff context to send"}
		}
		name := selected
		if strings.TrimSpace(name) == "" {
			if source == "commit" {
				short := commitSHA
				if len(short) > 7 {
					short = short[:7]
				}
				name = "commit " + short
			} else {
				name = "working tree"
			}
		}
		text := fmt.Sprintf("Here is the current diff for %s:\n\n%s\n\nWhat would you like to do?", name, d)
		return piContextMsg{Text: text, Status: "sent diff context to PI"}
	}
}

func (m *model) sendStagedDiffToPiCmd() tea.Cmd {
	if strings.TrimSpace(m.projectPath) == "" {
		return nil
	}
	return func() tea.Msg {
		d, _, err := git.RunGit(m.projectPath, "diff", "--cached")
		if err != nil {
			return piContextMsg{Err: err}
		}
		if strings.TrimSpace(d) == "" {
			return piContextMsg{}
		}
		text := "Here is the staged diff:\n\n" + d + "\n\nWhat would you like to do?"
		return piContextMsg{Text: text}
	}
}

func (m *model) startPiCmd() tea.Cmd {
	targetDir := m.findRepoRoot(m.projectPath)
	if strings.TrimSpace(targetDir) == "" {
		return func() tea.Msg {
			return piStartMsg{Err: fmt.Errorf("selected path is not a git repository")}
		}
	}
	m.rebindProjectPath(targetDir)
	if m.piSessionActiveForRepo(targetDir) {
		return nil
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
		m.piStopRequested = false
		m.piRepoPath = ""
		return
	}
	m.piStopRequested = true
	m.piProc.Stop()
	m.piProc = nil
	m.piRepoPath = ""
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
			// handled below
		}
	}

	rawKey := msg.String()
	if keyMatchesShortcut(msg, "ctrl+d") {
		m.piui.HalfPageDown()
		m.refreshAgentViewport()
		return m, nil
	}
	if keyMatchesShortcut(msg, "ctrl+u") {
		m.piui.HalfPageUp()
		m.refreshAgentViewport()
		return m, nil
	}
	switch {
	case keyMatchesShortcut(msg, "ctrl+c"):
		return m, m.quitCmd()
	case keyMatchesShortcut(msg, "ctrl+space"):
		return m, m.cycleModes()
	case keyMatchesShortcut(msg, "ctrl+o"), keyMatchesShortcut(msg, "ctrl+/"):
		return m, commandpallette.Open(string(m.mode), m.width, m.height)
	}

	switch rawKey {
	case "up":
		m.piui.ScrollUp()
		m.refreshAgentViewport()
		return m, nil
	case "down":
		m.piui.ScrollDown()
		m.refreshAgentViewport()
		return m, nil
	case "pgup":
		m.piui.HalfPageUp()
		m.refreshAgentViewport()
		return m, nil
	case "pgdown":
		m.piui.HalfPageDown()
		m.refreshAgentViewport()
		return m, nil
	case "end":
		m.piui.GotoBottom()
		m.refreshAgentViewport()
		return m, nil
	case "ctrl+e":
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
		m.mode = modeProjects
		m.statusMessage = "pi paused"
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
		if handled, cmd := m.handleSlashCommand(v); handled {
			m.refreshAgentViewport()
			return m, cmd
		}
		m.piui.AppendUserMessage(v)
		m.refreshAgentViewport()
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

func (m *model) handlePaletteAction(msg commandpallette.ActionMsg) (tea.Model, tea.Cmd) {
	switch msg.ID {
	case "switch.projects":
		return m, m.jumpToProjects()
	case "switch.git":
		return m, m.jumpToGit()
	case "switch.diff":
		return m, m.jumpToDiff()
	case "switch.pi":
		return m, m.jumpToPI()
	case "theme.open":
		m.reloadThemeItems()
		m.openPrompt(promptTheme, "Theme", "j/k move, enter apply, esc cancel", "")
		return m, nil
	case "settings.open":
		m.reloadThemeItems()
		m.openPrompt(promptTheme, "Settings", "Theme: j/k move, enter apply, esc cancel", "")
		return m, nil
	case "projects.backend":
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
	case "projects.new":
		m.openPrompt(promptNewProj, "New Project", "Enter folder path to create + git init", filepath.Join(m.localDir, "new-project"))
		return m, m.promptInput.Focus()
	case "projects.refresh":
		if m.mode == modeProjects && m.picker == pickerRepos {
			return m, m.loadReposCmd()
		}
		if err := m.reloadLocalEntries(); err != nil {
			m.showError(err.Error())
		}
		return m, nil
	case "projects.signout":
		m.authToken = ""
		m.authStatus = githubauth.StatusSignedOut
		m.authDevice = githubauth.DeviceCode{}
		m.repos = nil
		m.picker = pickerLocal
		return m, m.clearTokenCmd()
	case "pi.new":
		return m, m.sendPiCmd(pi.CmdNewSession())
	case "pi.model":
		return m, m.trackedPiCmd("/models", map[string]any{"type": "get_available_models"})
	case "pi.compact":
		return m, m.sendPiCmd(map[string]any{"type": "compact"})
	case "pi.tools":
		m.piui.ToggleToolBody()
		m.refreshAgentViewport()
		return m, nil
	case "pi.thinking":
		m.piui.ToggleThinking()
		m.refreshAgentViewport()
		return m, nil
	case "pi.export":
		return m, m.sendPiCmd(map[string]any{"type": "export_html"})
	case "pi.rename":
		return m, m.sendPiCmd(map[string]any{"type": "set_session_name"})
	case "git.stage_all":
		return m, m.stageAllCmd()
	case "git.commit":
		m.openPrompt(promptCommit, "Commit", "Enter commit message", "")
		return m, m.promptInput.Focus()
	case "git.push":
		return m, m.pushCmd()
	default:
		return m, nil
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
	if _, ok := slash.Find(name); !ok {
		return false, nil
	}

	switch name {
	case "/models":
		return true, m.trackedPiCmd(name, map[string]any{"type": "get_available_models"})
	case "/new":
		return true, m.trackedPiCmd(name, map[string]any{"type": "new_session"})
	case "/sessions":
		m.piui.Messages = append(m.piui.Messages, piui.Message{Role: piui.RoleAssistant, Text: "Session browser is not exposed in the current PI RPC surface yet."})
		m.refreshAgentViewport()
		return true, nil
	case "/compact":
		return true, m.trackedPiCmd(name, map[string]any{"type": "compact"})
	case "/fork":
		return true, m.trackedPiCmd(name, map[string]any{"type": "fork"})
	case "/state":
		return true, m.trackedPiCmd(name, pi.CmdGetState())
	case "/stats":
		return true, m.trackedPiCmd(name, map[string]any{"type": "get_session_stats"})
	case "/commands":
		return true, m.trackedPiCmd(name, map[string]any{"type": "get_commands"})
	case "/thinking":
		m.piui.ToggleThinking()
		m.refreshAgentViewport()
		return true, nil
	case "/tools":
		m.piui.ToggleToolBody()
		m.refreshAgentViewport()
		return true, nil
	case "/rename":
		return true, m.trackedPiCmd(name, map[string]any{"type": "set_session_name"})
	case "/export":
		return true, m.trackedPiCmd(name, map[string]any{"type": "export_html"})
	case "/undo":
		return true, m.trackedPiCmd(name, map[string]any{"type": "fork"})
	case "/theme":
		m.reloadThemeItems()
		m.openPrompt(promptTheme, "Theme", "j/k move, enter apply, esc cancel", "")
		return true, nil
	case "/help":
		cmds := slash.Builtin()
		lines := make([]string, 0, len(cmds)+1)
		lines = append(lines, "Commands:")
		for _, c := range cmds {
			lines = append(lines, fmt.Sprintf("- %s  %s", c.Name, c.Description))
		}
		m.piui.Messages = append(m.piui.Messages, piui.Message{Role: piui.RoleAssistant, Text: strings.Join(lines, "\n")})
		m.refreshAgentViewport()
		return true, nil
	case "/exit":
		m.stopPi()
		m.mode = modeProjects
		return true, m.inputBox.Focus()
	default:
		return false, nil
	}
}

func (m *model) nextSlashID() string {
	m.slashSeq++
	return fmt.Sprintf("slash-%d", m.slashSeq)
}

func (m *model) trackedPiCmd(tag string, payload map[string]any) tea.Cmd {
	id := m.nextSlashID()
	payload["id"] = id
	m.pendingSlash[id] = tag
	return m.sendPiCmd(payload)
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
		if m.openModelPicker(msg.Data) {
			return
		}
		text = formatModelsText(msg.Data)
	case "/models:set":
		text = formatModelSetText(msg.Data)
	case "/state":
		text = formatStateText(msg.Data)
	case "/stats":
		text = formatStatsText(msg.Data, m.piui.ThinkingLevel)
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

func (m *model) openModelPicker(data map[string]any) bool {
	items := parseModelPickerItems(data)
	if len(items) == 0 {
		return false
	}
	m.modelItems = items
	selectItems := make([]selectx.Item, 0, len(items))
	for _, it := range items {
		label := strings.TrimSpace(it.Name)
		if label == "" {
			label = it.ID
		}
		if it.Provider != "" {
			label += " (" + it.Provider + ")"
		}
		if it.Current {
			label = "* " + label
		}
		selectItems = append(selectItems, selectx.Item{Label: label, Value: it.ID})
	}
	m.modelPicker.SetItems(selectItems)
	w, h := m.promptPickerSize(promptModelPick)
	m.modelPicker.SetSize(w, h)
	m.modelPicker.Focus()
	m.modelPicker.Open()
	m.openPrompt(promptModelPick, "Model", "j/k move, enter set, esc cancel", "")
	return true
}

func parseModelPickerItems(data map[string]any) []modelPickerItem {
	if data == nil {
		return nil
	}
	raw, _ := data["models"].([]any)
	if len(raw) == 0 {
		return nil
	}
	currentID := ""
	if v, ok := data["currentModelId"].(string); ok {
		currentID = strings.TrimSpace(v)
	}
	if currentID == "" {
		if current, ok := data["current"].(map[string]any); ok {
			if id, ok := current["id"].(string); ok {
				currentID = strings.TrimSpace(id)
			}
		}
	}
	out := make([]modelPickerItem, 0, len(raw))
	for _, item := range raw {
		v, _ := item.(map[string]any)
		id, _ := v["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		provider, _ := v["provider"].(string)
		name, _ := v["name"].(string)
		current, _ := v["current"].(bool)
		out = append(out, modelPickerItem{
			ID:       id,
			Provider: strings.TrimSpace(provider),
			Name:     strings.TrimSpace(name),
			Current:  current || (currentID != "" && id == currentID),
		})
	}
	return out
}

func (m *model) modelProviderByID(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}
	for _, it := range m.modelItems {
		if strings.TrimSpace(it.ID) == modelID {
			return strings.TrimSpace(it.Provider)
		}
	}
	return ""
}

func formatModelSetText(data map[string]any) string {
	if data == nil {
		return "Model updated."
	}
	if name, ok := data["name"].(string); ok && strings.TrimSpace(name) != "" {
		return "Model set to " + strings.TrimSpace(name) + "."
	}
	if id, ok := data["id"].(string); ok && strings.TrimSpace(id) != "" {
		return "Model set to " + strings.TrimSpace(id) + "."
	}
	if model, ok := data["model"].(map[string]any); ok {
		if name, ok := model["name"].(string); ok && strings.TrimSpace(name) != "" {
			return "Model set to " + strings.TrimSpace(name) + "."
		}
		if id, ok := model["id"].(string); ok && strings.TrimSpace(id) != "" {
			return "Model set to " + strings.TrimSpace(id) + "."
		}
	}
	if id, ok := data["modelId"].(string); ok && strings.TrimSpace(id) != "" {
		return "Model set to " + strings.TrimSpace(id) + "."
	}
	return "Model updated."
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

func formatStatsText(data map[string]any, thinkingLevel string) string {
	if data == nil {
		return "No stats available."
	}
	parts := []string{"Session stats:"}
	if tokens, ok := data["tokens"].(map[string]any); ok {
		input := tokenInt(tokens["input"])
		output := tokenInt(tokens["output"])
		reasoning := tokenInt(tokens["reasoning"])
		if reasoning == 0 {
			reasoning = tokenInt(tokens["reasoningTokens"])
		}
		raw := tokenInt(tokens["total"])
		if raw <= 0 {
			raw = input + output + reasoning
		}
		adjusted := input + output + int(float64(reasoning)*thinkingWeight(thinkingLevel)+0.5)
		if adjusted <= 0 {
			adjusted = raw
		}
		if adjusted > 0 {
			parts = append(parts, fmt.Sprintf("- tokens (adjusted): %d", adjusted))
		}
		if raw > 0 {
			parts = append(parts, fmt.Sprintf("- tokens (raw): %d", raw))
		}
		if input > 0 || output > 0 || reasoning > 0 {
			parts = append(parts, fmt.Sprintf("- breakdown: in %d, out %d, reasoning %d", input, output, reasoning))
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

func tokenInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return parsed
		}
	}
	return 0
}

func thinkingWeight(level string) float64 {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "low":
		return 0.85
	case "med", "medium":
		return 1.00
	case "high":
		return 1.20
	default:
		return 1.00
	}
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
	// When lastRepo is set, the synthetic row is index 0 in the display list.
	// Preselect it so the user lands on it immediately.
	if m.lastRepo != "" {
		return 0
	}
	return 0
}

func (m *model) reloadThemeItems() {
	names := theme.AvailableThemes()
	items := make([]selectx.Item, 0, len(names))
	for _, n := range names {
		items = append(items, selectx.Item{Label: n, Value: n})
	}
	m.themePicker.SetItems(items)
	w, h := m.promptPickerSize(promptTheme)
	m.themePicker.SetSize(w, h)
	m.themePicker.Focus()
	m.themePicker.Open()
}

func (m *model) reloadCommitPicker() {
	items := make([]selectx.Item, 0, len(m.diffHistory))
	for _, c := range m.diffHistory {
		label := c.Hash
		if c.Message != "" {
			label = c.Hash + "  " + c.Message
		}
		items = append(items, selectx.Item{Label: label, Value: c.Hash})
	}
	m.commitPicker.SetItems(items)
	m.commitPicker.SetSize(m.width-4, m.bodyHeight()-4)
	m.commitPicker.Focus()
	m.commitPicker.Open()
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

func (m *model) normalizeRepoPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	root, _, gitErr := git.RunGit(path, "rev-parse", "--show-toplevel")
	if gitErr == nil {
		root = strings.TrimSpace(root)
		if root != "" {
			if absRoot, err := filepath.Abs(root); err == nil {
				return filepath.Clean(absRoot)
			}
			return filepath.Clean(root)
		}
	}
	return filepath.Clean(path)
}

func (m *model) rebindProjectPath(path string) {
	next := m.normalizeRepoPath(path)
	prev := m.normalizeRepoPath(m.projectPath)
	if next == "" {
		m.projectPath = ""
		m.piPendingContext = ""
		return
	}
	if prev != "" && prev != next {
		if m.piSessionActiveForRepo(prev) {
			m.stopPi()
			m.piui.StopStreaming()
			m.piui.Streaming = false
			m.piui.ToolRunning = false
			m.piui.Status = "stopped"
			m.piRestartTried = false
		}
		m.piPendingContext = ""
	}
	m.projectPath = next
}

func (m *model) findRepoRoot(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		path = filepath.Dir(path)
	}
	for {
		if git.IsGitRepo(path) {
			return m.normalizeRepoPath(path)
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return ""
}

func normalizeShortcutKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "ctrl+_", "ctrl+?", "ctrl+7", "ctrl+slash":
		return "ctrl+/"
	default:
		return key
	}
}

func repoFilterKey(key string) string {
	if key == "space" {
		return " "
	}
	if len(key) != 1 {
		return ""
	}
	r := []rune(key)[0]
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
		return string(r)
	}
	switch r {
	case '/', '-', '_', '.', ':':
		return string(r)
	default:
		return ""
	}
}

func fuzzyContains(query, value string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	value = strings.ToLower(value)
	if query == "" {
		return true
	}
	if strings.Contains(value, query) {
		return true
	}
	qr := []rune(query)
	vr := []rune(value)
	q := 0
	for _, ch := range vr {
		if q < len(qr) && ch == qr[q] {
			q++
		}
	}
	return q == len(qr)
}

func isAffirmative(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "y" || v == "yes"
}

func (m *model) repoBoundaryInstruction(repoPath string) string {
	repoPath = m.normalizeRepoPath(repoPath)
	if repoPath == "" {
		return "Operate only inside the currently selected repository. Do not access parent directories or sibling repositories."
	}
	return "Repository boundary: only read/write files inside " + repoPath + ". Never access parent directories, sibling repositories, or absolute paths outside this root."
}

func keyMatchesShortcut(msg tea.KeyMsg, target string) bool {
	target = normalizeShortcutKey(target)
	if target == "" {
		return false
	}
	key := msg.Key()
	candidates := []string{
		normalizeShortcutKey(msg.String()),
		normalizeShortcutKey(key.Keystroke()),
	}
	if key.Mod&tea.ModCtrl != 0 {
		switch key.Code {
		case 0x1f, '/', '?':
			candidates = append(candidates, "ctrl+/")
		case ' ':
			candidates = append(candidates, "ctrl+space")
		case 'c', 'C':
			candidates = append(candidates, "ctrl+c")
		}
	}
	for _, c := range candidates {
		if c == target {
			return true
		}
	}
	return false
}

func isBenignPiExit(err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return true
	}
	return strings.Contains(text, "signal: killed") || strings.Contains(text, "killed")
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
