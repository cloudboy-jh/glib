# glib

![glib header](./header.png)

Terminal workspace for project picking, git status/diff flow, and agent execution.

Diff mode is powered by the embedded `bento-diffs` viewer.

## Run

```bash
go run ./cmd/glib
```

## Current MVP

- Projects view (`p`) with local navigation + clone from URL
- Diff view (`d`) backed by `bento-diffs` rendering/navigation
- Git view (`g`) with stage/unstage, commit, and log-to-diff
- Opencode view (`o`) with embedded mode + fallback behavior

## Controls

- `o` opencode
- `d` diff
- `g` git
- `p` projects
- `t` theme picker
- `q` quit

### Diff Controls

- `j/k` or arrows: scroll
- `ctrl+d`/`pgdown`: half-page down
- `ctrl+u`/`pgup`: half-page up
- `n`/`N`: next/previous file
- `}`/`{`: next/previous hunk
- `g`/`G`: open git mode
- `q`/`esc`: back to projects

### Diff Integration Notes

- glib keeps its own global footer.
- The embedded viewer footer is intentionally not shown in glib.
- Theme and resize updates are forwarded into the embedded viewer.

## Development Rules

- UI/layout policy: see `opencode.md`
