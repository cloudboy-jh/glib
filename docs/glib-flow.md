# glib — glib-flow.md
> The complete Diff → Git → PI loop. The lightning edge of glib.

## The Loop

```
PROJECTS
    │
    ▼
  pick repo
    │
    ├──► DIFF ──────────────────────────────────┐
    │     view commit history or open changes    │
    │     review with bento-diffs                │
    │     send file/diff context to PI           │
    │                                            │
    ├──► GIT ────────────────────────────────────┤
    │     beautiful status view                  │
    │     stage / unstage / discard              │
    │     branch panel (switch/create/delete)    │
    │     stash push/pop/list                    │
    │     send staged diff to PI                 │
    │                                            │
    └──► PI ─────────────────────────────────────┘
          persistent session per repo            │
          slash commands + command palette       │
          repo-scoped PI boundary                │
          jump to DIFF/GIT and back              │
          ESC = soft pause, session survives     │
          re-enter = resume session              │
          ──────────────────────────────────────►┘
                    full circle
```

## BentoTUI First

All UI uses BentoTUI v0.6.0+ bricks, recipes, and rooms.

## 1. ESC Behavior Fix (PI mode)

**Current:** ESC in PI = violent kill. Session is destroyed.

**Fix:**
- ESC in PI = soft pause. Return to PROJECTS. PI process stays alive in background.
- Re-entering PI on the same repo = resume existing session, history intact.
- Footer in PROJECTS shows: `● pi active  i resume` when a session is live.
- Session only killed on: `/exit` slash command, new repo selection, or app quit.

## 1b. PROJECTS recent hub freshness

- Recent hub shows up to 5 GitHub recents + up to 5 local-only recents.
- Local rows display freshness badges: `[behind N]` and `[Xd ago]`.
- `F` in PROJECTS runs `git fetch --all --prune` on selected local repo and updates freshness badges.
- `P` in PROJECTS runs `git pull --ff-only` on selected local repo and updates freshness badges.
- Entering/returning to PROJECTS refreshes freshness checks for visible recent rows.

## 2. DIFF mode — two views

### View A: Commit History (Default)
- List-first design: entering DIFF shows commit history picker.
- Uses `selectx` picker component with hash + message display.
- `enter` on commit loads that commit's diff.
- `c` toggles to open changes view.
- `esc/q` returns to projects.

### View B: Open Changes
- Shows current working tree diff via `git diff HEAD`.
- Uses `bento-diffs` renderer.
- `c` toggles back to commit history.
- `i` sends current file + diff as context to PI.
- Empty state: `no open changes  c commit history`.

### DIFF → PI handoff
- `i` key in DIFF mode: sends current file + diff as context to PI.
- If PI session is active on same repo: sends as a steer message.
- If no PI session: starts one and pre-loads the file context.

## 3. GIT mode — full local ops

### Status view
- Three sections: `Staged`, `Unstaged`, `Untracked`.
- File operations: `s` stage, `u` unstage, `d` discard, `a` stage all, `A` unstage all.
- Discard uses a `y/n` confirmation prompt.
- `enter` on file opens it in DIFF mode.

### Commit flow
- `c` opens commit prompt.
- `p` push, `P` pull, `f` fetch.

### Branch panel (`b`)
- List local branches with current branch marked.
- `enter` switch to selected branch.
- `n` create new branch (input prompt).
- `D` delete selected branch (confirm).

### Stash operations
- `z` stash push (including untracked).
- `Z` stash pop.
- `?` view stash list.

### Commit log (`l`)
- `git log --oneline` rendered as list.
- `enter` opens commit in DIFF mode.

### GIT → PI handoff
- `i` from GIT mode: sends staged diff as context to PI.

## 4. PI mode — session lifecycle + mode handoffs

### Session lifecycle
- PI session starts when user enters PI mode on a repo.
- Session is tied to repo path (normalized to git root).
- PI process is started with repo-root cwd and receives an explicit repo-boundary instruction.
- ESC = soft pause, return to PROJECTS, session stays alive.
- Session persists until: `/exit`, repo change, or app quit.
- PROJECTS footer badge: `● pi active` when session is live on selected repo.

### Slash commands
- `/` activates slash command picker.
- Available: `/models`, `/new`, `/sessions`, `/compact`, `/fork`, `/state`, `/stats`, `/commands`, `/thinking`, `/tools`, `/rename`, `/export`, `/undo`, `/theme`, `/help`, `/exit`.
- Recognized commands don't appear as user chat turns.

### PI → DIFF/GIT handoff
- `d` in PI mode: jump to DIFF view.
- `g` in PI mode: jump to GIT mode.
- Session remains active while navigating between modes.

### Mode navigation
| From | Key | To | Session preserved |
|---|---|---|---|
| PI | `esc` | PROJECTS | ✓ soft pause |
| PI | `d` | DIFF | ✓ stays alive |
| PI | `g` | GIT | ✓ stays alive |
| DIFF | `i` | PI | ✓ sends context |
| GIT | `i` | PI | ✓ sends staged diff |
| PROJECTS | `i` | PI | resumes or starts |

## 5. Command Palette (`ctrl+o`)

- Global shortcut opens mode-gated command palette.
- Mode-specific actions available based on current mode.
- Actions: mode switches, PI commands, git operations, theme picker.

## 6. Footer state cross-mode

| Mode | PI session active indicator |
|---|---|
| PROJECTS | `● pi active  i resume` |
| DIFF | `i send diff context + switch to PI` |
| GIT | `i back to pi` |
| PI | normal PI footer |

## Done Criteria

- [x] ESC in PI = soft pause, session survives, PROJECTS shows resume badge
- [x] DIFF shows commit history first (list), then open changes (viewer)
- [x] Commit history uses proper `selectx` picker
- [x] DIFF → PI sends file + diff as context
- [x] GIT status shows staged/unstaged/untracked with full ops
- [x] GIT branch panel create/switch/delete
- [x] GIT stash push/pop/list
- [x] GIT commit log with diff open
- [x] GIT → PI sends staged diff as context
- [x] PI slash commands with autocomplete
- [x] PI → DIFF and PI → GIT navigate without killing session
- [x] Command palette (`ctrl+o`) with mode-gated actions
- [x] Cross-mode footer shows active PI session indicator
- [x] Repo path normalization (PI runs in git root, not editor dir)
- [x] PI repo boundary hardening (repo rebind on switch, no stale pending context)
- [x] No violent exits anywhere in the loop
