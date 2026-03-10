# glib

![glib header](./header.png)

Terminal workspace for project picking, git status/diff flow, and agent execution.

## Run

```bash
go run ./cmd/glib
```

## Current MVP

- Projects view (`p`) with local navigation + clone from URL
- Diff view (`d`) with staged/unstaged toggle and file jumps
- Git view (`g`) with stage/unstage, commit, and log-to-diff
- Opencode view (`o`) with embedded mode + fallback behavior

## Controls

- `o` opencode
- `d` diff
- `g` git
- `p` projects
- `t` theme picker
- `q` quit
