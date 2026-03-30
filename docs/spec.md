# glib Product Spec

Type: terminal workspace app (BentoTUI)
Stack: Go, Bubble Tea v2, Lipgloss v2, BentoTUI, bento-diffs
Positioning: one terminal shell for repo selection, git operations, diff review, and pi handoff.

## Product Principles

- Zero-to-first-repo in under one minute on a fresh install.
- No hidden mode magic: every state transition must be visible and keyboard-driven.
- Keep porcelain behavior close to git CLI expectations.
- Prefer portable workflows over environment-specific assumptions.
- Sessions persist: PI workflows survive mode switches and re-entry.

## Product Goals

- Reduce context switching between repo selection, git commands, diff inspection, and pi use.
- Keep visual quality of diff review high by embedding `bento-diffs` directly.
- Support multiple execution environments (local dev, remote VPS, containers, sandbox sessions).
- Keep keyboard-first ergonomics with predictable mode transitions.
- Enable seamless PI → Diff → Git → PI loop without losing session context.

## Non-Goals

- Building a second custom diff renderer.
- Running multiple concurrent chrome systems per screen.
- Replacing native git semantics with custom VCS behavior.
- Complex multi-repo PI sessions.

## Runtime Model

`glib` is a full-screen app with four modes:

- `PROJECTS`: repository picker + backend selection
- `GIT`: repo status/stage/commit/push workflow + branches + stash
- `DIFF`: list-first commit history, then embedded diff viewer workflow
- `PI`: subprocess RPC chat via pi with persistent sessions

Footer ownership is global: `glib` owns the bottom row in every mode.

## Core User Journey

- Launch `glib` and authenticate from the projects/home input area.
- Pick a repository from `PROJECTS` and materialize it via selected backend.
- Inspect and edit state in `GIT`, open contextual file diffs in `DIFF`.
- Hand off to `PI` and return to the same workspace context.
- Navigate DIFF → PI → GIT → PI without losing PI session state.

## Auth Contract (embedded in `PROJECTS`)

- Uses GitHub Device Flow (`/login/device/code`, `/login/oauth/access_token`).
- Default OAuth scope is `repo`.
- Client id resolution order:
  - `GLIB_GITHUB_CLIENT_ID` env override
  - built-in release default client id
- Token persistence uses user config dir: `<user-config-dir>/glib/github_token`.
- User settings persist in `<user-config-dir>/glib/config.json`.
- Auth states: `SIGNED_OUT`, `PENDING`, `AUTHORIZED`, `EXPIRED`.

## Projects Contract (`p`)

- Source of truth is authenticated GitHub repo list.
- Repo ordering follows recent activity; last selected repo is pinned first and focused.
- Navigation supports cursor movement and explicit refresh.
- Repo list viewport is fixed to 5 visible rows with scroll window behavior.
- `enter` on repo opens an action chooser rendered below the repo card.
- Action chooser is a compact horizontal bar rendered below the repo card.
- Action chooser options:
  - `Diff`: materialize repo, then route to `DIFF` mode (commit history first)
  - `Git`: materialize repo, then route to `GIT` mode
  - `Pi`: materialize repo, then route to `PI` mode
- `esc` closes action chooser and returns focus to repo list.
- Backend is switchable in-mode:
  - `local`: persisted checkout root (`GLIB_WORKSPACE_ROOT` or `~/glib-workspaces`)
  - `ephemeral`: cached base clone + session worktree per open action
- Ephemeral cleanup runs on app quit and skips dirty worktrees.
- Project selection sets active repository context for `GIT`, `DIFF`, and `PI`.
- Footer shows `● pi active  i resume` when PI session is live for selected repo.

## Distribution Contract

- Release builds include a default GitHub OAuth client id for zero-config onboarding.
- Users may override client id with `GLIB_GITHUB_CLIENT_ID`.
- Device-flow OAuth app used by release builds must have Device Flow enabled.

## Git Contract (`g`)

- Shows branch, tracking, ahead/behind, and grouped staged/unstaged/untracked files.
- Supports stage, unstage, discard (confirm), commit, push, pull, fetch.
- Supports stage all (`a`) and unstage all (`A`).
- Branch panel (`b`): list, switch (`enter`), create (`n`), delete (`D`).
- Stash operations: push (`z`), pop (`Z`), list (`?`).
- Commit log (`l`): view log, `enter` opens commit in DIFF.
- `enter` on file opens file-focused diff in `DIFF` mode.
- `i` sends staged diff to PI as context.

## Diff Contract (`d`)

- **List-first design**: entering DIFF opens commit history picker by default.
- Two views: commit history (list) and open changes (viewer).
- `c` toggles between history and open-changes views.
- Commit history uses `selectx` picker with hash + message display.
- Patch source comes from git commands (`diff`, `show`).
- Parsing uses `bento-diffs` unified diff parser.
- Rendering/navigation uses embedded `bento-diffs` viewer.
- Viewer footer is hidden; app footer remains visible.
- ANSI diff output must be preserved (no ANSI-stripping wrappers around viewer render).
- `i` sends current file/diff context to PI (steer if active, start+preload if not).
- `esc` in viewer returns to commit history; `esc` in commit history returns to PROJECTS repo picker.

## PI Contract (`i`)

- Starts `pi --mode rpc --cwd <repo>` in active project directory.
- Repo path is normalized to git root (`git rev-parse --show-toplevel`) before starting.
- PI transport uses strict JSONL (`\n` delimiter) and a single stdout reader goroutine.
- Renders streaming assistant text and tool blocks in glib body region.
- Input-first keymap: typing goes to input.
- Global mode cycle: `ctrl+space` cycles `DIFF` -> `PI` -> `GIT` (from non-ring modes, enters `DIFF`).
- Global command palette: `ctrl+o` opens full mode-aware command list (`ctrl+/` accepted as fallback alias).
- Stored quick settings (theme/model) are loaded from config and applied on startup/PI launch.
- `ctrl+e` toggles inline tool output expansion, `ctrl+t` toggles thinking visibility.
- `esc` soft-pauses: returns to `PROJECTS`, session stays alive in background.
- Extension dialog requests (`extension_ui_request`) render as in-ring modals and respond with `extension_ui_response`.
- Footer shows calm braille spinner only while PI is actively working (thinking/tool/retry/compaction).
- Footer shows `● pi active` in PROJECTS when session persists.

### Slash Commands

- `/` activates slash command picker with autocomplete.
- Recognized commands don't appear as user chat turns.
- Available commands:
  - `/models` (interactive picker), `/new`, `/sessions`, `/compact`, `/fork`
  - `/state`, `/stats`, `/commands`
  - `/thinking`, `/tools`, `/rename`, `/export`, `/undo`
  - `/theme` — opens theme picker
  - `/help` — list all commands
  - `/exit` — hard stop PI session

### Session Lifecycle

- Sessions are repo-scoped: tied to normalized repo root path.
- Session persists across mode switches (PI → DIFF → PI, PI → GIT → PI).
- Session only terminates on: `/exit` command, repo change, or app quit.
- Re-entering PI on same repo resumes existing session with full history.
- Starting PI on different repo stops previous session.

## Command Palette Contract (`ctrl+o`)

- Global shortcut opens mode-gated command palette.
- Available actions depend on current mode.
- Actions include: mode switches, PI commands, git operations, theme picker.
- Palette rows render action and keybind on one line.
- Unimplemented actions fail gracefully without breaking UI.

## Architecture Boundaries

- `internal/app`: mode state machine, key routing, shell rendering.
- `internal/pi`: pi process lifecycle + RPC protocol + JSONL transport.
- `internal/piui`: PI chat session state, rendering, modal dialogs, footer status/spinner.
- `internal/bentodiffs`: grouped git+diff domain state and git operations.
- `internal/githubauth`: OAuth device flow + repo API + token persistence.
- `internal/workspace`: backend abstraction for local vs ephemeral repo materialization.
- `internal/slash`: slash command registry and builtin definitions.
- `internal/command-pallette`: global command palette with mode-gated actions.

## UI and Layout Contracts

- Theme source of truth: `theme.CurrentTheme()`.
- Theme changes persist immediately to user config and are restored on startup.
- Shell composition: Bento `rooms.Focus(...)` + anchored footer.
- Keybind hints in footer must match real key handling.
- No duplicate footers/key legends inside content panes.
- Chat rendering: role-specific styles (user = muted `>`, assistant = markdown-aware, tool = status-colored).
