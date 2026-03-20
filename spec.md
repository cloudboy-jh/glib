# glib spec (current)

Type: BentoTUI app
Stack: Go, Bubble Tea v2, Lipgloss v2, BentoTUI, bento-diffs
Tagline: terminal workspace. git + agent + diff.

## Overview

glib is a full-screen terminal workspace with four modes:

- `PROJECTS`: local tree picker and clone flow
- `DIFF`: embedded bento-diffs viewer (glib shell)
- `GIT`: repo status/stage/commit/push workflow
- `OPENCODE`: subprocess handoff and return tunnel

Footer ownership is global: glib always owns the bottom footer row.

## Mode Contracts

### Projects (`p`)

- Local picker with expandable tree navigation
- Clone picker: paste URL, confirm destination, run clone
- Project selection updates active repo context for git/diff modes

### Diff (`d`)

- Patch source comes from git (`git diff` or `git show`)
- Parsing uses `bentodiffs.ParseUnifiedDiffs`
- Rendering/navigation uses embedded `bentodiffs.NewViewer`
- glib key routing forwards movement to viewer imperative methods
- Viewer footer is intentionally hidden; glib footer remains visible

Diff keys:

- `j/k`, `down/up`: line scroll
- `ctrl+d`/`pgdown`: half-page down
- `ctrl+u`/`pgup`: half-page up
- `n`/`N`: next/previous file
- `}`/`{`: next/previous hunk
- `g`/`G`: switch to git mode
- `q`/`esc`: back to projects

### Git (`g`)

- Shows branch, tracking/ahead/behind, staged/unstaged sections
- `s` stage, `u` unstage, `d` discard (confirm prompt)
- `c` commit prompt, `p` push
- `enter` opens selected file diff in diff mode

### Opencode (`o`)

- Starts opencode subprocess handoff in selected project directory
- Streams output into glib screen
- `esc` / `ctrl+b` returns to projects mode

## Theme and Layout Contracts

- Theme source of truth is `theme.CurrentTheme()`
- On `theme.ThemeChangedMsg`, mode surfaces update in place
- On window resize, glib resizes app shell and embedded viewer
- App shell is built with Bento `rooms.Focus(...)` + anchored footer

## Diff Integration Notes

- glib does not maintain a second diff renderer pipeline.
- `internal/diffview` is minimal state/mock glue only.
- Embedded viewer content is drawn directly to preserve ANSI diff colors.
- Wrapping viewer output in ANSI-stripping containers is not allowed.
