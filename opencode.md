# OpenCode Rules

This repo is Bento-first for TUI UI work.

## Core rule

- Do not mix Bento rooms/bricks with ad-hoc custom UI chrome in the same view.
- If Bento provides a brick/room for the job, use it.
- Custom Lip Gloss rendering is only allowed for domain content rows (for example, diff line coloring), not for shell/chrome.
- For embedded viewer content (for example bento-diffs), render viewer ANSI output directly in the body surface; do not pass it through ANSI-stripping containers.

## Layout and chrome rules

- One screen shell: use `rooms.Focus(...)` for page body + anchored footer.
- One footer per page view. No duplicate keybind legends inside pane bodies.
- Use Bento `bar` for all footer keybind UX (`FooterAnchored` + compact cards by default).
- In glib diff mode, glib footer is the only footer shown; embedded viewer footer must be hidden.
- Use Bento bricks (`panel`/`list`/`badge`/`separator`/`kbd`) before inventing custom container patterns.
- For split screens, prefer room grammar (`rooms.HSplit`, `rooms.DrawerRight`, `rooms.Frame*`) over manual width math.

## Theme engine rules

- Always read theme at render time from `theme.CurrentTheme()`.
- Prefer Bento styles system (`styles.New(...)`) and component theming contracts.
- Avoid hardcoded colors unless required by domain rendering semantics.
- Every rendered row must be width-constrained and background-owned to avoid bleed/misalignment.
- Use ANSI-safe helpers for styled text clipping (`styles.ClipANSI`, `styles.RowClip`) instead of custom rune slicing.
- Anchored footer appearance should come from theme footer tokens when present; do not manually force selection-like footer colors in app code.

## Keybind UX rules

- Footer cards are the source of truth for keybind hints.
- Footer hints must exactly match real key handling.
- If a key is advertised in footer, it must work in that mode.
- Use icon + shortcut labeling consistently (`icon`, `command`, `label`).
- Keep command semantics case-stable across modes (example: git `d` discard vs `D` open diff).

## Exception policy

- If Bento cannot express a required behavior, add a short code comment naming the gap and why custom rendering is needed.
- Keep custom rendering isolated behind a small helper.
- Revisit and remove the exception when Bento adds the missing primitive.
