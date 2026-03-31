# glib — architecture-update.md

> Legacy implementation notes. Source of truth is `docs/spec.md` and `docs/glib-flow.md`.

> Terminal workspace for the AI coding era.  
> One shell. Repo picker. Diff. Git. Pi chat. Done.

---

## Architecture

glib is a BentoTUI app. All UI chrome, layout, theming, and mode management uses BentoTUI rooms and bricks. The pi runtime is [pi](https://github.com/badlogic/pi-mono) running headless via `pi --mode rpc`. glib owns everything the user sees. pi owns execution.

```
glib (BentoTUI shell)
├── PROJECTS mode  — repo picker, backend toggle, action bar
├── DIFF mode      — bento-diffs viewer
├── GIT mode       — stage/unstage/commit/push
└── PI mode        — pi-powered chat, tool stream, session state
        │
        │ stdin/stdout JSONL  (pi --mode rpc --cwd <repo>)
        ▼
    pi process
    (models, tools, sessions, compaction, retry)
```

---

## Modes

| Mode | Key | Description |
|---|---|---|
| `PROJECTS` | `p` | Repo picker, backend selector, action bar |
| `DIFF` | `d` | bento-diffs viewer for current repo |
| `GIT` | `g` | Stage, unstage, commit, push |
| `PI` | `i` | pi RPC chat — streaming text, tool blocks, input |

Global keys always active: `p` `d` `g` `i` `t` (theme) `q` (quit).

---

## PROJECTS mode

- Repo list: j/k move, enter open action bar, b backend toggle, r refresh.
- Action bar (horizontal, below repo card): `Diff` | `Git` | `Pi`.
- `Diff` → resolves repo path, switches to DIFF mode.
- `Git` → resolves repo path, switches to GIT mode.
- `Pi` → resolves repo path, starts pi process, switches to PI mode.
- Backend toggle: `local` (persisted clone root) or `ephemeral` (temp clone). Worktree mode is a future phase.
- Repo resolution: if repo not yet cloned, clone first, then route to selected action.

---

## DIFF mode

- Powered by `github.com/cloudboy-jh/bento-diffs`.
- j/k scroll, ctrl+d/ctrl+u page, n/N file nav, {/} hunk nav.
- Footer shows current file, hunk position.
- `enter` on a file in GIT mode opens that file in DIFF mode.

---

## GIT mode

- Shows staged, unstaged, untracked files.
- s stage, u unstage, d discard, c commit, p push.
- enter opens selected file in DIFF mode.
- Commit flow: prompt for message via glib input brick, then execute.

---

## PI mode

### Layout

Focus ring wraps the entire body — message history, tool blocks, and input. Footer is the only element outside the ring.

```
┌─────────────────────────────────────────┐  ← focus ring (rooms.Focus / BorderFocus)
│                                         │
│  > refactor the auth module             │  ← user message
│                                         │
│  I'll start by reading...               │  ← streaming assistant text
│  ▋                                      │
│                                         │
│  ┌ bash ──────────────────────────┐     │  ← tool block
│  │ cat internal/auth/auth.go      │     │
│  │ exit 0                         │     │
│  └────────────────────────────────┘     │
│                                         │
│ > _                                     │  ← input brick (inside ring, pinned bottom)
└─────────────────────────────────────────┘
  PI  bentotui  esc abort  s steer        ← footer (only thing outside ring)
```

### Message scroll behavior

- Auto-scrolls to bottom on every new `text_delta` or `tool_execution_update`.
- If user scrolls up manually (j/k or mouse wheel), auto-scroll pauses.
- Auto-scroll resumes when user reaches bottom or sends a new prompt.
- Footer shows scroll indicator when not at bottom: `↑ scroll to follow`.

### Input behavior

- Single-line input (BentoTUI input brick) for MVP.
- `enter` sends prompt to pi.
- `esc` sends `{"type": "abort"}` to pi if streaming, otherwise goes back to PROJECTS.
- `s` during streaming sends `{"type": "steer", "message": "..."}` — user types steer message in input first.
- `ctrl+c` in glib always quits glib cleanly (stops pi process first).

---

## internal/pi — pi RPC process manager

Package path: `github.com/cloudboy-jh/glib/internal/pi`

### Responsibilities

- Spawn `pi --mode rpc --cwd <repoPath>` as a child process.
- Write JSONL commands to pi's stdin.
- Read JSONL events from pi's stdout on a goroutine, fire `tea.Msg` for each.
- Handle process lifecycle: start, stop, crash detection, cleanup.
- One pi process per active repo session. Killed when user leaves PI mode or quits glib.

### Types

```go
// PiProcess manages one pi --mode rpc subprocess.
type PiProcess struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    events chan tea.Msg
}

// Events fired into bubbletea loop
type PiEventMsg struct {
    Raw  []byte      // raw JSON line
    Type string      // event type string
}

type PiResponseMsg struct {
    Raw     []byte
    ID      string
    Command string
    Success bool
    Error   string
}

type PiExitMsg struct {
    Err error // nil = clean exit
}
```

### Key methods

```go
func Start(repoPath string) (*PiProcess, error)
func (p *PiProcess) Send(cmd any) error        // marshal + write to stdin
func (p *PiProcess) ReadLoop() tea.Cmd         // receives from internal event channel
func (p *PiProcess) Stop()                     // SIGTERM + wait
```

Implementation detail: stdout reading happens in one long-lived goroutine with strict JSONL framing (`\n` delimiter) to avoid concurrent-reader panics.

### Commands sent to pi

| Action | JSON |
|---|---|
| Send prompt | `{"type":"prompt","message":"..."}` |
| Abort | `{"type":"abort"}` |
| Steer | `{"type":"steer","message":"..."}` |
| New session | `{"type":"new_session"}` |
| Get state | `{"type":"get_state"}` |

### Events received from pi

| Event | glib action |
|---|---|
| `agent_start` | show spinner, disable input |
| `text_delta` | append delta to last assistant message, re-render viewport |
| `tool_execution_start` | open new tool block in viewport |
| `tool_execution_update` | update tool block content |
| `tool_execution_end` | close tool block, show exit status |
| `agent_end` | hide spinner, enable input, scroll to bottom |
| `auto_compaction_start` | show "compacting..." in footer |
| `auto_compaction_end` | clear footer status |
| `auto_retry_start` | show "retrying..." in footer |
| `auto_retry_end` | clear footer status |

---

## internal/piui — message + chat UI state

Package path: `glib/internal/piui`

### Responsibilities

- Owns chat history, input state, modal state, spinner state, and footer state.
- Renders full message history for `viewport.SetContent()`.
- Handles partial streaming state and tool block updates.
- Handles extension UI requests and in-ring modal responses.

### Message types

```go
type MessageRole string
const (
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
    RoleTool      MessageRole = "tool"
)

type ToolBlock struct {
    Name     string
    Args     string
    Output   string
    Done     bool
    ExitOK   bool
}

type Message struct {
    Role      MessageRole
    Text      string           // for user/assistant
    ToolBlock *ToolBlock       // for tool events
    Streaming bool             // true while text_delta still coming
}
```

### Component files

- `session.go` — top-level `Session` state
- `chat_history.go` — message structs + rendering
- `chat_events.go` — PI event reducer
- `chat_input.go` — scroll/follow helpers
- `chat_modal.go` — extension dialog state
- `chat_status.go` — footer mapping
- `chat_spinner.go` — calm braille spinner
- `chat_view.go` — modal view helpers

---

## PI mode — app model fields

Added to `internal/app/app.go`:

```go
pi            *pi.PiProcess
agentMessages []agent.Message
agentViewport viewport.Model      // charm.land/bubbles/v2/viewport
agentInput    *input.Model        // existing BentoTUI input brick
agentAutoScroll bool
agentStreaming  bool
agentSessionID string
```

---

## Cleanup — completed

PI migration removed the previous opencode embedding and PTY path:

- Delete `internal/opencode/` package entirely.
- Remove `vt10x` dependency from `go.mod`.
- Remove `opencodeTerm`, `opencodeExitArmed`, `opencodeExitSeq`, `opencodeViewportSize`, `drawOpencodeView`, `opencodeViewportLines` from `app.go`.
- Remove `modeOpencode` app mode constant.
- Change `Opencode` action in action bar to `Pi` — routes to pi.
- Remove `docs/opencode.md`.

---

## Keybinds — PI mode

| Key | Action |
|---|---|
| `enter` | send prompt (if not streaming) |
| `esc` | abort if streaming, else back to PROJECTS |
| `ctrl+g` + key | command prefix while typing (`p`/`d`/`g`/`i`/`m`/`n`/`G`/`j`/`k`) |
| `s` | steer (types message, then enter to send) |
| `j` / `k` | scroll message history up/down |
| `ctrl+d` / `ctrl+u` | page down/up in history |
| `G` | jump to bottom, re-enable auto-scroll |
| `m` | cycle model (sends `cycle_model` to pi) |
| `n` | new session (sends `new_session` to pi) |

---

## Footer — PI mode states

| State | Footer content |
|---|---|
| Idle | `PI  <repo>  enter send  esc back  ctrl+g shortcuts` |
| Streaming | `PI  <repo>  enter send  esc abort  s steer  thinking...` |
| Tool running | `PI  <repo>  esc abort  bash • running` |
| Compacting | `PI  <repo>  compacting context...` |
| Scrolled up | `PI  <repo>  G bottom  ↑ not following` |
| Cmd prefix | `PI  cmd: p projects  d diff  g git  i pi  m model  n session` |

---

## Build order

1. **Cleanup** — remove opencode embedding, vt10x, all PTY code.
2. **internal/pi** — process manager, JSONL reader, tea message types.
3. **internal/piui** — chat state, rendering, spinner, modal handling.
4. **PI mode in app.go** — viewport init, message update handlers, input wiring, keybinds.
5. **Action bar** — rename `Opencode` → `Pi`, wire to pi start.
6. **Footer** — PI mode states.
7. **Test** — send prompt, streaming text renders, tool blocks render, abort works, exit returns to PROJECTS.

---

## Out of scope (future phases)

- Session history browser (resume past sessions).
- Multi-repo pi sessions.
- Textarea (multi-line input) — single-line MVP first.
- Diff → Pi handoff (open diff, send to pi).
- Git → Pi handoff (pi-assisted commit messages).
- GoReleaser + Homebrew distribution.
