# PI Repo Boundary Hardening Plan

## Goal

Guarantee PI can only operate inside the currently selected glib repo root, and cannot read/write outside it (including parent editor workspace repos).

## Why this is needed

- Current launch path starts PI with `pi --mode rpc --cwd <repo>` in `internal/pi/client.go`.
- `--cwd` is a soft hint, not a hard sandbox.
- If PI/tooling resolves absolute paths, `..` traversal, symlinks, or inherited context from elsewhere, it can still touch unintended repos.

## Threat model

Protect against:

- Absolute path reads/writes outside selected repo.
- Relative path traversal (`../..`) escaping selected repo.
- Symlink escape from in-repo path to out-of-repo location.
- Git commands accidentally resolving against parent repo.
- Session drift after repo switching or reconnect.

Out of scope for this pass:

- OS-level sandboxing (containers/seccomp/mac sandbox-exec).
- Network egress restrictions.

## Current-state notes

- glib already normalizes selected repo root before PI start (`startPiCmd` in `internal/app/home-screen.go`).
- PI process lifecycle is tracked per repo path (`piRepoPath` in `internal/app/home-screen.go`).
- No hard boundary enforcement exists in glib-side tool dispatch because tools execute inside PI runtime.

## Plan overview

### Phase 1 — Tighten glib-side session boundary (fast, low risk)

1. Add explicit boundary metadata to PI session start context.
   - Include canonical `repoRoot` in a startup system/context message.
   - Include strict instruction: "never read/write outside repoRoot".
2. On every repo switch:
   - Hard-stop PI for old repo if active.
   - Clear pending context that may reference old paths.
3. On reconnect logic:
   - Revalidate `projectPath == piRepoPath` before restart.
   - If mismatch, do not auto-restart.

Deliverable: glib always binds one PI process to one canonical repo root, with no cross-repo session reuse.

### Phase 2 — Add hard path guard in PI runtime/tool layer (required for real safety)

Implement in PI tool execution layer (upstream/runtime side):

1. Canonical root resolution
   - Compute `repoRoot = EvalSymlinks(Abs(cwd))` once per session.
2. Guard helper for every filesystem path argument
   - `abs = EvalSymlinks(Abs(join(cwd, argPath)))`
   - `rel = filepath.Rel(repoRoot, abs)`
   - reject if `rel == ".."` or starts with `".." + string(os.PathSeparator)`
3. Apply guard to tools
   - `read`, `write`, `edit`, glob/search tools, any shell/file ops.
4. Git command enforcement
   - force `git -C <repoRoot> ...` for all git operations.
5. Error UX
   - return friendly denial: "Path is outside selected repo boundary".

Deliverable: impossible for PI tools to access files outside selected repo, even with absolute paths or symlink tricks.

### Phase 3 — Validation harness and regression tests

Add automated tests (PI runtime and/or integration harness):

1. Allow list
   - in-repo read/write succeeds.
2. Block list
   - `../outside.txt` denied.
   - absolute `/tmp/x` denied.
   - symlink inside repo -> outside target denied.
   - git command without repo root context denied or rewritten to `-C <repoRoot>`.
3. Session isolation
   - repo A session cannot access repo B path.
   - switch A -> B kills A process and starts B-bound process.

Deliverable: repeatable proof boundary enforcement holds.

## Implementation details in glib

- `internal/pi/client.go`
  - keep `--cwd <repoRoot>`
  - optionally set `cmd.Dir = repoRoot` as belt-and-suspenders.
- `internal/app/home-screen.go`
  - enforce repo match checks during auto-restart.
  - clear stale pending context on repo transitions.
  - add startup boundary context message when PI session starts.

## UX behavior

- If PI attempts out-of-bound path:
  - show inline tool error block in chat.
  - keep session alive.
  - message should include selected repo root and blocked path.

## Rollout strategy

1. Ship Phase 1 in glib immediately.
2. Coordinate Phase 2 with PI runtime/tooling owner (or patch local fork).
3. Gate launch claim ("repo-isolated") on Phase 2 + Phase 3 pass.

## Acceptance criteria

- PI can operate normally inside selected repo.
- Any path outside selected repo is denied across all file-affecting tools.
- Repo switch guarantees process/session rebind to new root.
- Regression tests cover traversal, absolute path, and symlink escape cases.
