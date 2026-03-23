# glib

![glib header](./header.png)

`glib` is a terminal workspace app for repository-first development: authenticate once, pick a repo, inspect and stage changes, review beautiful diffs, and hand off to pi without leaving one UI shell.

## Product Direction

- Repo-first UX: you start from GitHub repositories, not local filesystem spelunking.
- One workspace shell: auth, project, git, diff, and pi workflows live in one full-screen TUI.
- Best-in-class diffs: embedded `bento-diffs` renderer stays the source of truth for diff reading.
- Portable execution: local persisted workspaces or ephemeral workspaces for sandbox/container/VPS flows.

## Quick Start

```bash
go run ./cmd/glib
```

By default, `glib` ships with a built-in GitHub OAuth client id so release users can sign in immediately.

## Auth Model (Device Flow)

- `glib` uses GitHub Device Flow in-terminal.
- Default scope is `repo` (supports private repositories).
- Token is stored locally at your user config path (`<user-config-dir>/glib/github_token`).
- Auth is embedded in the home/projects card under the wordmark.
- `enter` starts sign-in; `l` clears local token.

## Modes

- `PROJECTS` (`p`): GitHub repo list picker and workspace backend toggle.
- `GIT` (`g`): staged/unstaged/untracked operations with commit/push actions.
- `DIFF` (`d`): embedded `bento-diffs` viewer with file/hunk navigation.
- `PI` (`i`): pi RPC chat/runtime in selected project directory.

## Workspace Backends

- `local`: clone/use repos under a persisted root (`GLIB_WORKSPACE_ROOT` or `~/glib-workspaces`).
- `ephemeral`: cached base clone + session worktree per repo.
- Ephemeral cleanup runs at app quit and skips dirty worktrees.
- Toggle backend in projects repo picker with `b`.

## Key Controls

- global: `p` projects, `g` git, `d` diff, `i` pi, `t` theme, `q` quit
- projects repo picker: `j/k` move, `enter` open action chooser, `b` backend toggle, `n` new project, `r` refresh repos
- projects action chooser (shown below repo card): `h/l` or arrows choose action, `enter` run, `esc` back
- diff: `j/k` scroll, `ctrl+d`/`ctrl+u` page, `n/N` file nav, `c` commit diff, `{`/`}` hunk nav
- git: `s` stage, `u` unstage, `d` discard, `c` commit, `p` push, `enter` open file diff
- pi: type in input, `enter` send, `esc` abort/back, `ctrl+o` tool output toggle, `ctrl+t` thinking toggle, `ctrl+g` then (`p`/`d`/`g`/`i`/`m`/`n`/`G`/`j`/`k`) for shortcuts while typing

In `PI`, glib keeps a focused viewport ring around message history + input while the footer remains the global command surface.

## Repo Selection UX

- Repo list viewport is intentionally compact (5 visible rows) with keyboard scrolling.
- Selecting a repo opens a lightweight horizontal action bar below the repo card:
  - `Diff` opens `bento-diffs` workflow for the selected repo.
  - `Git` opens git workflow for the selected repo.
  - `Pi` opens pi mode for the selected repo.

## Environment

- `GLIB_GITHUB_CLIENT_ID`: optional override for OAuth client id.
- `GLIB_WORKSPACE_ROOT`: optional root directory for local backend clones.
- `GLIB_ICONS`: icon mode (`safe` or nerd glyph defaults).

## Architecture Notes

- Unified git+diff domain internals live under `internal/bentodiffs`.
- PI transport/protocol handling lives under `internal/pi`.
- PI chat UI/session/rendering lives under `internal/piui`.
- External diff rendering is provided by `github.com/cloudboy-jh/bento-diffs`.
- `glib` owns the shell/footer; embedded diff viewer footer is intentionally hidden.

## Internal Development Rules

- UI and layout rules are captured in `docs/spec.md`.
- Behavior contracts: `docs/spec.md`.
- Product roadmap: `docs/next-steps.md`.
