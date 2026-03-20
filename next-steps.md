# glib Next Steps

## Current Focus

- Lock diff embedding behavior now that bento-diffs viewer integration is live.
- Keep a single glib-owned footer contract across all modes.
- Add smoke checks for project -> git -> diff navigation loops.

## Short-Term

- Add tests around diff source selection (`working` vs `commit`) and selected-path mapping.
- Add snapshot-style visual checks for diff mode ANSI output preservation.
- Harden opencode lifecycle error handling for abrupt PTY termination.

## 1.0 Readiness

- Stable behavior for `PROJECTS`, `DIFF`, `GIT`, `OPENCODE` mode transitions.
- No duplicate chrome/footer across embedded views.
- Deterministic theme and resize behavior in every mode.
- Release checklist documented and executable.

## Docs To Maintain

- `README.md`: operator-facing usage and keymap
- `spec.md`: behavior contracts and architecture notes
- `BENTODIFFS_INTEGRATION.md`: integration status and guardrails
