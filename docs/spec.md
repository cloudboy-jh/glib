# glib Product Spec

Type: terminal workspace app (BentoTUI)
Stack: Go, Bubble Tea v2, Lipgloss v2, BentoTUI, bento-diffs
Positioning: one terminal shell for repo selection, git operations, diff review, and agent handoff.

## Product Principles

- Zero-to-first-repo in under one minute on a fresh install.
- No hidden mode magic: every state transition must be visible and keyboard-driven.
- Keep porcelain behavior close to git CLI expectations.
- Prefer portable workflows over environment-specific assumptions.

## Product Goals

- Reduce context switching between repo selection, git commands, diff inspection, and agent use.
- Keep visual quality of diff review high by embedding `bento-diffs` directly.
- Support multiple execution environments (local dev, remote VPS, containers, sandbox sessions).
- Keep keyboard-first ergonomics with predictable mode transitions.

## Non-Goals

- Building a second custom diff renderer.
- Running multiple concurrent chrome systems per screen.
- Replacing native git semantics with custom VCS behavior.

## Runtime Model

`glib` is a full-screen app with four modes:

- `PROJECTS`: repository picker + backend selection
- `GIT`: repo status/stage/commit/push workflow
- `DIFF`: embedded diff viewer workflow
- `OPENCODE`: subprocess tunnel to opencode CLI

Footer ownership is global: `glib` owns the bottom row in every mode.

## Core User Journey

- Launch `glib` and authenticate from the projects/home input area.
- Pick a repository from `PROJECTS` and materialize it via selected backend.
- Inspect and edit state in `GIT`, open contextual file diffs in `DIFF`.
- Hand off to `OPENCODE` and return to the same workspace context.

## Auth Contract (embedded in `PROJECTS`)

- Uses GitHub Device Flow (`/login/device/code`, `/login/oauth/access_token`).
- Default OAuth scope is `repo`.
- Client id resolution order:
  - `GLIB_GITHUB_CLIENT_ID` env override
  - built-in release default client id
- Token persistence uses user config dir: `<user-config-dir>/glib/github_token`.
- Auth states: `SIGNED_OUT`, `PENDING`, `AUTHORIZED`, `EXPIRED`.

## Projects Contract (`p`)

- Source of truth is authenticated GitHub repo list.
- Navigation supports cursor movement and explicit refresh.
- Repo list viewport is fixed to 5 visible rows with scroll window behavior.
- `enter` on repo opens an action chooser rendered below the repo card.
- Action chooser is a compact horizontal bar rendered below the repo card.
- Action chooser options:
  - `Diff`: materialize repo, then route to `DIFF` mode
  - `Opencode`: materialize repo, then route to `OPENCODE` mode
- `esc` closes action chooser and returns focus to repo list.
- Backend is switchable in-mode:
  - `local`: persisted checkout root (`GLIB_WORKSPACE_ROOT` or `~/glib-workspaces`)
  - `ephemeral`: temp clone per open action
- Implementation status: ephemeral backend currently materializes full temp clones and does not yet use `git worktree`.
- Project selection sets active repository context for `GIT`, `DIFF`, and `OPENCODE`.

## Distribution Contract

- Release builds include a default GitHub OAuth client id for zero-config onboarding.
- Users may override client id with `GLIB_GITHUB_CLIENT_ID`.
- Device-flow OAuth app used by release builds must have Device Flow enabled.

## Git Contract (`g`)

- Shows branch, tracking, ahead/behind, and grouped staged/unstaged/untracked files.
- Supports stage, unstage, discard (confirm), commit, and push.
- `enter` on file opens file-focused diff in `DIFF` mode.

## Diff Contract (`d`)

- Patch source comes from git commands (`diff`, `show`).
- Parsing uses `bento-diffs` unified diff parser.
- Rendering/navigation uses embedded `bento-diffs` viewer.
- Viewer footer is hidden; app footer remains visible.
- ANSI diff output must be preserved (no ANSI-stripping wrappers around viewer render).

## Opencode Contract (`o`)

- Starts `opencode` process in active project directory.
- Streams process output directly in glib body region (no nested opencode frame).
- Body region is wrapped by a thin focus ring viewport while glib footer remains global.
- Forwards keyboard input to subprocess by default.
- Exit contract: `ctrl+g` then `ctrl+g` terminates process and returns to `PROJECTS`.

## Architecture Boundaries

- `internal/app`: mode state machine, key routing, shell rendering.
- `internal/bentodiffs`: grouped git+diff domain state and git operations.
- `internal/githubauth`: OAuth device flow + repo API + token persistence.
- `internal/workspace`: backend abstraction for local vs ephemeral repo materialization.

## UI and Layout Contracts

- Theme source of truth: `theme.CurrentTheme()`.
- Shell composition: Bento `rooms.Focus(...)` + anchored footer.
- Keybind hints in footer must match real key handling.
- No duplicate footers/key legends inside content panes.
