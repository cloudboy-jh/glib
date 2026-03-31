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
- [x] **Auth UX hardening**: scope messaging, typed device-flow errors, token-expiry handling
- [x] **Repo picker upgrades**: owner/name filter, pagination, local-vs-clone-needed badges
- [x] **Release packaging baseline**: GoReleaser config + LICENSE + README install path
- [x] **PI repo boundary safeguards**: repo rebind on switch, stale pending context cleared, startup boundary steer
- [x] **Global mode jumps**: `d/g/i/p` mode switching outside PI input capture

## Near-Term Priorities

- Add smoke tests for core path: `PROJECTS(sign-in) -> repo action chooser -> DIFF/GIT/PI`.
- Add retry-counter-based PI crash recovery (multi-retry with backoff) instead of single retry flag.
- Implement inline diff rendering in PI chat for tool `write`/`edit` completions.

## Quality and Reliability

- Add tests around diff source selection (`working` vs `commit`) and selected-path mapping.
- Add snapshot-style checks for ANSI preservation in embedded diff rendering.
- Harden pi RPC lifecycle for abrupt process termination.
- Add backend tests for `local` and `ephemeral` workspace behavior.
- Add integration tests for PI session persistence across mode switches.

## 1.0 Release Gate

- Stable mode transitions across all four modes.
- Reliable private repo access through device flow with `repo` scope.
- PI remains scoped to selected repo across session start/switch/restart.
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
