# glib — spec v0.1

**Type**: BentoTUI app  
**Stack**: Go, BubbleTea, Lipgloss, BentoTUI (github.com/cloudboy-jh/bentotui@main)  
**Tagline**: terminal workspace. git + agent + diff.

---

## Overview

glib is a terminal workspace app built on BentoTUI. Run `glib` anywhere — the home screen opens as a project picker. Select a project and each view takes over the full canvas. The footer is always glib's, even inside opencode.

---

## Home Screen

The BentoTUI starter app shell, repurposed as glib's launch screen.

- **Wordmark**: `glib` in slant ascii, centered
- **Input block**: project search / path entry, recent projects shown in hint row
- **Tagline**: `● terminal workspace. git + agent + diff.`
- **Footer**: `o opencode   d diff   g git   p projects   q quit` / `glib v0.1.0`

`enter` on a path or selected project → opens that project in the active view  
`glib .` skips the picker and opens CWD directly (future flag)

---

## Views

All views are full-canvas. Footer (1 row) always belongs to glib.

### `o` — opencode
- Spawns `opencode` as a subprocess in a pty
- opencode owns the full canvas minus the footer row
- Footer shows active mode: `OPENCODE` on right slot
- `esc` or `ctrl+b` suspends opencode, returns to glib home

### `d` — diff
- Renders `git diff` output for the current project
- Lipgloss syntax highlighting: green added, red removed
- `j/k` scroll, `]f/[f` jump between files
- `s` toggles staged/unstaged

### `g` — git
- Compact git surface: status, stage/unstage, commit
- `space` stage/unstage file under cursor
- `c` open commit message input (dialog)
- `enter` on log entry → loads that commit's diff

### `p` — projects
- Returns to home screen / project picker

### `q` — quit

---

## Footer

Single row, always visible, always glib's.

```
o opencode   d diff   g git   p projects   q quit        DIFF
```

- Left: all keybind hints
- Right: current mode label
- Active view key is highlighted
- In opencode mode, left shows `esc return` only

---

## BentoTUI Components Used

| Component | Status | Notes |
|-----------|--------|-------|
| `surface` | exists | full canvas renderer |
| `input` | exists | project picker |
| `bar` | exists | footer + topbar |
| `dialog` | exists | commit input, help overlay |
| `theme` | exists | inherited from BentoTUI |

---

## Components To Build

| Component | Notes |
|-----------|-------|
| `pty` | subprocess embed for opencode view |
| `diffview` | git diff renderer with lipgloss highlighting |
| `tree` | file tree, read-only, vim nav (future) |
| `gitstrip` | stage/unstage/commit surface (future) |

---

## Phases

**Phase 1** — home screen + shell ✓  
Home screen running with wordmark, project picker input, glib footer, key routing stubs.

**Phase 2** — opencode view  
PTY embed, footer persistence, esc to return.

**Phase 3** — diff view  
`git diff` output, lipgloss highlighting, scroll/jump.

**Phase 4** — git view  
Status, stage/unstage, commit dialog.

**Phase 5** — polish  
Recent projects from history, `glib .` flag, themes.

---

## Repo Structure

```
glib/
  main.go       ← home screen + key routing
  go.mod
  go.sum
```

Single file until phase 2 requires splitting views into separate files.
