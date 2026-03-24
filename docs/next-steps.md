# glib Product Roadmap

## Completed ✓

- [x] **PI Session Persistence**: Soft-pause with ESC, resume on re-entry
- [x] **Cross-mode PI indicators**: Footer badges in PROJECTS/DIFF/GIT
- [x] **Slash commands**: Full registry with autocomplete (`/models`, `/new`, `/theme`, `/help`, etc.)
- [x] **Command palette**: Global `ctrl+k` with mode-gated actions
- [x] **DIFF list-first**: Commit history picker as default view
- [x] **DIFF → PI handoff**: Send diff context to PI
- [x] **GIT expansion**: Stage/unstage all, pull/fetch, branch panel, stash ops, commit log
- [x] **GIT → PI handoff**: Send staged diff to PI
- [x] **Repo path normalization**: PI runs in actual git root
- [x] **Chat rendering**: Role-specific styles, markdown-aware assistant

## Near-Term Priorities

- Harden auth UX for release users:
  - explicit scope messaging and revoke/logout clarity
  - graceful handling of token expiry and device-flow polling errors
- Improve repo picker quality:
  - search/filter by owner/name
  - pagination for large accounts/orgs
  - clear state badges for local vs ephemeral materialization
- Add smoke tests for core path: `PROJECTS(sign-in) -> repo action chooser -> DIFF/GIT/PI`.

## Quality and Reliability

- Add tests around diff source selection (`working` vs `commit`) and selected-path mapping.
- Add snapshot-style checks for ANSI preservation in embedded diff rendering.
- Harden pi RPC lifecycle for abrupt process termination.
- Add backend tests for `local` and `ephemeral` workspace behavior.
- Add integration tests for PI session persistence across mode switches.

## 1.0 Release Gate

- Stable mode transitions across all four modes.
- Reliable private repo access through device flow with `repo` scope.
- Deterministic resize/theme behavior in every mode.
- No duplicate chrome/footer and no ANSI color regressions in diff mode.
- Release checklist and onboarding docs validated on clean machines.

## Post-1.0

- Multi-account auth switching.
- Branch/PR-oriented diff entry points.
- Better repo onboarding (attach existing checkout, smart dedupe, cache strategy).
- PI-assisted commit message generation (stream into commit input).
- Inline diff rendering for write/edit tool completions in PI chat.

## Docs to Keep Accurate

- `README.md`: product framing, install/run, key workflows.
- `docs/spec.md`: contracts, boundaries, runtime behavior.
- `docs/glib-flow.md`: complete Diff → Git → PI loop specification.
- `architecture-update.md`: implementation notes (legacy, merge into spec).
