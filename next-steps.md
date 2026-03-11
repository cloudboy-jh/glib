# glib Next Steps

## Immediate (This PR)

- Finalize native LOCAL picker polish (bontree-style behavior, Bento-consistent rendering).
- Lock footer to one stable full-width row with clean spacing and mode status.
- Stabilize theme picker interactions and glyph consistency.
- Add focused docs for current behavior and keymaps.

### Immediate Contracts

- **Opencode process**
  - Trigger: `o`
  - Embedded PTY when supported
  - Exit: `esc` / `ctrl+b`
  - Windows fallback: non-crashing return to `PROJECTS`
- **Projects workflow**
  - `LOCAL`: native in-app directory picker only
  - `CLONE`: paste URL -> destination prompt -> `git clone`
  - Auth: rely on existing Git credentials (`gh auth`, SSH, GCM)
- **Footer**
  - Single row only
  - Left: actions
  - Right: mode + project + version

## 1.0 Version Scope

- Stable mode contracts for `PROJECTS`, `DIFF`, `GIT`, `OPENCODE`.
- Reliable project selection and clone flow.
- Deterministic diff and git refresh behavior.
- Opencode lifecycle hardened (start, stream, exit, errors).
- Smoke-test checklist required before release.

### 1.0 Definition of Done

- LOCAL picker interaction feels smooth and predictable.
- Footer remains visually stable across all modes.
- No crash path when binaries or PTY support are missing.
- Contracts are documented and match code behavior.

## Roadmap (Post-1.0)

- Expand docs and architecture notes under `docs/`.
- Add release checklist and troubleshooting guide.
- Improve mode-specific UX polish (density, spacing, status context).
- Add automated checks for core git/project flows.

### Suggested Docs Layout

```text
docs/
  architecture.md
  opencode-process.md
  projects-flow.md
  keymap.md
  smoke-tests.md
  release-checklist.md
```
