# Changelog

All notable changes to glib are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

- No unreleased changes.

## [0.3.8] - 2026-04-21

### Added

- **PROJECTS Recent Hub**: authenticated PROJECTS now defaults to a compact recent-first list (up to 5 GitHub recents + up to 5 local-only recents) with a dedicated "View all repos" row.
- **All repos overlay**: full GitHub repository list now opens as an in-view overlay from Recent Hub (`r` / CTA row), preserving existing filter, pagination, and action chooser behavior.
- **Repo freshness badges**: PROJECTS rows now include staleness context (`[behind N]`, `[Xd ago]`) sourced from local git state.
- **In-place fetch from PROJECTS**: `F` fetches selected local repo directly from Recent Hub or full overlay.
- **Persisted recent history**: user config now stores recent GitHub repos and recent local-only repos for startup continuity.
- **Launch footer version stamp**: footer bottom-right now shows app version on launch/PROJECTS screen.

### Changed

- **Git freshness detection**: last-fetch timestamp resolution now falls back beyond `FETCH_HEAD` (remote refs / packed refs) so age badges render reliably across repo layouts and environments.

### Added

- **Release distribution scaffolding**: added `.goreleaser.yml` with cross-platform `glib` binaries (darwin/linux/windows, amd64/arm64), archives, and checksums.
- **Repository state badges** in PROJECTS repo picker: per-repo `[local]` / `[clone needed]` status using workspace materialization checks.
- **Repo picker filtering**: type-to-filter (owner/name) with backspace/escape clear behavior.
- **PI repo boundary hardening plan doc**: `pi-repo-boundary-hardening-plan.md` with phased implementation and acceptance criteria.
- **License**: added MIT `LICENSE`.

### Changed

- **Module path** changed from local `glib` to `github.com/cloudboy-jh/glib` and all internal imports were updated for release-style installs.
- **README install flow** now documents `go install github.com/cloudboy-jh/glib/cmd/glib@latest` and `glib` as the user-facing entry command.
- **Mode jump keys** `d/g/i/p` now work globally (outside PI input capture), not only from PROJECTS.
- **PI escape behavior**: removed pause confirmation prompt; `esc` in PI now soft-pauses immediately to PROJECTS.
- **Git discard confirmation** changed from typed `DISCARD` to a simple `y/n` prompt.
- **DIFF history list height** now scales with terminal height (`max(5, bodyH/6)`).
- **Projects auth screen copy** now includes explicit `repo` scope messaging and pending polling status.

### Fixed

- **GitHub auth expiry handling**: repo fetch `401 Unauthorized` now maps to token-expired behavior with clearer re-auth UX.
- **Device-flow polling errors** now map to typed cases (`expired_token`, `access_denied`, timeout) instead of raw strings.
- **Unicode wrapping bug** in `wrapPlainText`: removed byte slicing and switched to display-width-safe splitting.
- **JSONL record limit** raised from 1MB to 4MB to prevent failures on larger tool outputs.
- **Empty-state hints** added for branches/stashes in GIT mode (`n create branch`, `z stash current changes`).

### Security

- **PI repo binding tightened**:
  - PI start path is normalized to absolute git root.
  - process sets both `--cwd <repo>` and `cmd.Dir = <repo>`.
  - startup includes explicit repo-boundary steering instruction.
  - repo rebinding now stops old repo PI session and clears pending context to prevent cross-repo drift.

## [0.3.4] - 2026-03-27

### Added

- **User settings persistence** — new `internal/config/settings.go` package persists a `config.json` at `<user-config-dir>/glib/`. Stores selected theme and PI model/provider. Loaded on startup; writes are atomic.
- **`internal/app/settings.go`** — app-level `settingsModel` wrapper exposing `Theme()`, `SetTheme()`, `Model()`, `ModelProvider()`, `SetModel()`.
- **Interactive model picker** — `/models` slash command, `m` key in PI mode, and `pi.model` command palette action now open a `selectx` fullscreen picker instead of dumping a raw text list. Selected model is persisted and re-applied automatically when PI starts.
- **`pi.CmdSetModel(provider, modelID)`** (`internal/pi/rpc.go`) — new RPC helper emitting `set_model` with correct `provider` + `modelId` fields.
- **`trackedPiCmd` helper** (`internal/app/home-screen.go`) — extracts the slash-tracking boilerplate (`nextSlashID` + `pendingSlash` wiring) previously duplicated across all slash command handlers.
- **Commit preview modal** — `l` key in DIFF history opens a `promptCommitView` modal displaying the full commit hash and message. `enter`/`esc` closes it.
- **`openCommitViewPrompt`** helper and `promptBody` model field added to support multi-line prompt bodies separate from the text input prompts.
- **`promptModelPick` and `promptCommitView`** prompt modes added.
- **`clipCommitTitle` + `singleLine`** helpers in `internal/diffs/view.go` for clean single-line commit title rendering with `..` ellipsis suffix.
- **`settings.open` palette action** wired to theme picker as a settings entry point.
- Footer hint for DIFF history updated to include `l preview`.

### Fixed

- **DIFF commit history card — row wrapping** (`internal/diffs/view.go`): `rowW` corrected to `contentW-4` to match actual inner drawable width (block border 1px × 2 + padding 1 × 2 = 4px frame). Rows were previously too wide, causing commit titles to wrap onto a second line.
- **DIFF commit history card — background layering artifact**: `header` and `meta` rows now carry explicit `Background(t.BackgroundPanel())` and run through `padRow()` — same paint path as list rows. Previously these two rows had no background, causing them to render visually distinct ("differently colored") from the list rows when lipgloss composed the block.
- **Commit preview modal — transparent cell bleed-through**: All modal rows (`title`, `hint`, blank separator, each body line) are now rendered with `.Width(bodyW).Background(t.BackgroundPanel())`. Previously text-only glyphs left transparent cells; the surface overlay inherited the diff card's background behind the modal, producing dark strip artifacts.
- **Projects repo picker — row rendering consistency** (`drawRepoProjectsView`): Refactored to use `padRow` + `base`/`active`/`synthStyle` with explicit backgrounds, matching the diff history row pattern. Removed per-row `Width()` style which conflicted with `fitLine` clipping.
- **Blank filler rows** in DIFF history now use `base.Render(padRow(""))` for uniform full-width panel paint instead of a separate `lipgloss.NewStyle().Width(contentW)` style.
- **Command palette width** (`internal/command-pallette/palette.go`): minimum raised from 60 to 76, maximum from 90 to 110 so action label and keybind fit on one row in typical terminal widths.
- **`/models` slash response handler**: now attempts to open the interactive picker first; falls back to text dump only if no model items are parseable.
- **`formatModelSetText`** added to surface a human-readable confirmation when `set_model` RPC responds.

### Changed

- `/models` description updated to "Open interactive model picker" in `internal/slash/slash.go` and docs.
- `m` key in PI mode and `pi.model` palette action changed from `CmdCycleModel()` to a tracked `get_available_models` RPC that feeds the picker.
- Commit preview modal sizing: `2/3` screen width clamped to 48–90 (was generic `width/2` prompt sizing).
- Commit body in modal uses `wrapPlainText` per-line rendering for clean wrapping instead of hard `fitMultiline` clipping.
- `handleSlashCommand` refactored to use `trackedPiCmd` throughout, removing the local `makeTracked` closure.
- README and `docs/spec.md` updated: user settings section added, `/models` description updated, palette row sizing noted, theme persistence documented.

---

## [0.3.3] - prior

See git log for history before changelog was introduced.
