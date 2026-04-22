# glib

![glib header](./header.png)

`glib` is a terminal workspace app for repository-first development: authenticate once, pick a repo, inspect and stage changes, review beautiful diffs, and hand off to pi without leaving one UI shell.

![Projects view](./readme-app-launch.png)

<details>
<summary>More screenshots</summary>

![Diff view](./diff-screenshot.png)

![PI view](./pi-screenshot.png)
</details>

## Why glib

AI agents are fast. Reviewing their output is the hard part.

Most terminal workflows drop you into a raw prompt with no project context loaded. You cd in, grep around, and hope the agent caught up. When it produces a diff, you're back to exiting fullscreen, hunting the file tree, scrolling to find the change — reading most of the file just to remember what it was doing.

glib fixes the workflow, not just the tooling. You pick a repo and it preloads context: commits, diffs, docs. When pi produces a change, the diff is right there — rendered by bentodiffs, one keypress to go full screen, one keypress to go back. Git, diff, and agent chat are all reachable from the same shell without breaking your mental model.

The review is the work. glib is built around that.

## Product Direction

- Repo-first UX: you start from GitHub repositories, not local filesystem spelunking.
- One workspace shell: auth, project, git, diff, and pi workflows live in one full-screen TUI.
- Best-in-class diffs: embedded `bento-diffs` renderer stays the source of truth for diff reading.
- Portable execution: local persisted workspaces or ephemeral workspaces for sandbox/container/VPS flows.
- Persistent PI sessions: soft-pause PI and resume later without losing context.

## Quick Start

```bash
go run ./cmd/glib
```

Use the installed `glib` binary once the first release is cut.

## Install

```bash
go install github.com/cloudboy-jh/glib/cmd/glib@latest
glib
```

If `glib` is not found after `go install`, your Go bin directory is not in `PATH`.

### macOS + Ghostty (zsh)

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zprofile
source ~/.zprofile && source ~/.zshrc
hash -r
```

Or run the repo installer, which installs `glib` and configures zsh `PATH` automatically:

```bash
./scripts/install.sh
```

Verify the install:

```bash
command -v glib
```

Expected output includes a path ending in `glib` (for example, `/Users/<you>/go/bin/glib`).

Troubleshooting:

```bash
go env GOPATH GOBIN
```

Make sure your active Go bin path (`$GOBIN` when set, otherwise `$GOPATH/bin`) is in `PATH`.

Prebuilt binaries are published in GitHub Releases for macOS, Linux, and Windows.

By default, `glib` ships with a built-in GitHub OAuth client id so release users can sign in immediately.

## Auth Model (Device Flow)

- `glib` uses GitHub Device Flow in-terminal.
- Default scope is `repo` (supports private repositories).
- Token is stored locally at your user config path (`<user-config-dir>/glib/github_token`).
- User settings are stored at `<user-config-dir>/glib/config.json`.
- Auth is embedded in the home/projects card under the wordmark.
- `enter` starts sign-in; `l` clears local token.

## Modes

- `PROJECTS` (`p`): recent-first repo hub (GitHub + local-only recents), all-repos overlay, and workspace backend toggle.
- `GIT` (`g`): staged/unstaged/untracked operations with commit/push actions, branch management, and stash operations.
- `DIFF` (`d`): list-first commit history and embedded `bento-diffs` viewer with file/hunk navigation.
- `PI` (`i`): pi RPC chat/runtime in selected project directory with slash commands and session persistence.

PI sessions are repo-scoped and boundary-hinted: glib starts pi in the normalized repo root, rebinds on repo switches, and injects an explicit repo-boundary instruction on session start.

## The Loop

```
PROJECTS
    │
    ▼
  pick repo
    │
    ├──► DIFF ──────────────────────────────────┐
    │     view commit history or open changes    │
    │     review with bento-diffs                │
    │     send file/diff context to PI           │
    │                                            │
    ├──► GIT ────────────────────────────────────┤
    │     beautiful status view                  │
    │     stage / unstage / discard              │
    │     branch panel (switch/create/delete)    │
    │     stash push/pop/list                    │
    │     send staged diff to PI                 │
    │                                            │
    └──► PI ─────────────────────────────────────┘
          persistent session per repo            │
          slash commands + command palette       │
          repo-scoped session boundary           │
          jump to DIFF/GIT and back              │
          ESC = soft pause, session survives     │
          re-enter = resume session              │
          ──────────────────────────────────────►┘
                    full circle
```

## Key Controls

### Global
- `ctrl+space` — cycle modes (`DIFF` → `PI` → `GIT`)
- `ctrl+o` — open command palette (`ctrl+/` fallback)
- `d/g/i/p` — jump to mode
- `t` — theme picker
- `q` — quit

### Projects
- `j/k` or arrows — move
- `enter` — open action chooser
- `F` — fetch selected local repo
- `P` — pull selected local repo (`--ff-only`)
- `r` — open full repos overlay / refresh in overlay
- `b` — backend toggle (local/ephemeral)
- `n` — new project
- type / `backspace` — filter repos by owner/name
- `tab` — toggle repo/local picker

### Diff (List-first)
- `j/k` — navigate commit history
- `enter` — open selected commit diff
- `c` — toggle to open changes view
- `esc/q` — back to projects repo picker

### Diff (Viewer)
- `j/k` — scroll
- `ctrl+d/ctrl+u` — page down/up
- `tab/shift+tab` — next/previous file
- `n/N` — next/previous file (alias)
- `c` — toggle to commit history
- `i` — send current diff to PI
- `esc/q` — back to commit history

### Git
- `j/k` — move
- `s` — stage file
- `u` — unstage file
- `a` — stage all
- `A` — unstage all
- `d` — discard (confirm)
- `c` — commit
- `p` — push
- `P` — pull
- `f` — fetch
- `b` — branch panel
- `l` — commit log
- `z` — stash push
- `Z` — stash pop
- `?` — stash list
- `i` — send staged diff to PI
- `enter` — open file diff

### PI
- Type in input, `enter` to send
- `/` — activate slash command picker
- `tab` — autocomplete slash command
- `esc` — soft pause (return to projects, session stays alive)
- `ctrl+e` — toggle tool output expansion
- `ctrl+t` — toggle thinking visibility
- `ctrl+space` — cycle modes while keeping session context
- `ctrl+o` — open command palette (`ctrl+/` fallback)

### Slash Commands (PI)
- `/models` — open interactive model picker
- `/new` — start new session
- `/sessions` — browse sessions (UI placeholder)
- `/compact` — compact context
- `/fork` — fork from current message
- `/state` — show session state
- `/stats` — show token/cost stats
- `/commands` — refresh command list
- `/thinking` — toggle thinking display
- `/tools` — toggle tool output display
- `/rename` — rename session
- `/export` — export to HTML
- `/undo` — undo previous
- `/theme` — open theme picker
- `/help` — show all commands
- `/exit` — exit PI (hard stop)

## Workspace Backends

- `local`: clone/use repos under a persisted root (`GLIB_WORKSPACE_ROOT` or `~/glib-workspaces`).
- `ephemeral`: cached base clone + session worktree per repo.
- Ephemeral cleanup runs at app quit and skips dirty worktrees.
- Toggle backend in projects repo picker with `b`.

## PI Session Lifecycle

- PI sessions are tied to repo path and persist until `/exit`, repo change, or app quit.
- `esc` in PI does a soft pause — returns to PROJECTS without killing the process.
- Footer shows `● pi active` in PROJECTS when a session is live.
- Re-entering PI on the same repo resumes the existing session with full history intact.
- Cross-mode navigation (PI → DIFF → PI, PI → GIT → PI) preserves session state.
- Repo switches hard-rebind PI context: old repo sessions are stopped and pending context is cleared.

## Environment

- `GLIB_GITHUB_CLIENT_ID`: optional override for OAuth client id.
- `GLIB_WORKSPACE_ROOT`: optional root directory for local backend clones.
- `GLIB_ICONS`: icon mode (`safe` or nerd glyph defaults).

## User Settings

- Quick user settings are persisted in `<user-config-dir>/glib/config.json`.
- Theme selection is auto-saved and restored on next app launch.
- Last selected PI model is saved and applied when PI starts.
- Recent GitHub and local-only repositories are persisted for PROJECTS recent hub.

## Architecture Notes

- Unified git+diff domain internals live under `internal/bentodiffs`.
- PI transport/protocol handling lives under `internal/pi`.
- PI chat UI/session/rendering lives under `internal/piui`.
- Slash command registry lives under `internal/slash`.
- Command palette lives under `internal/command-pallette`.
- External diff rendering is provided by `github.com/cloudboy-jh/bento-diffs`.
- `glib` owns the shell/footer; embedded diff viewer footer is intentionally hidden.

## Internal Development Rules

- UI and layout rules are captured in `docs/spec.md`.
- Behavior contracts: `docs/spec.md`.
- Product roadmap: `docs/next-steps.md`.
- Complete workflow spec: `docs/glib-flow.md`.
