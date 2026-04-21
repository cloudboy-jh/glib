package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// IsGitRepo returns true if dir is inside a git working tree.
func IsGitRepo(dir string) bool {
	out, _, err := RunGit(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// RunGit runs a git command in dir and returns stdout, stderr, and any error.
func RunGit(dir string, args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	if err != nil && errOut != "" {
		return out, errOut, fmt.Errorf("%s", errOut)
	}
	return out, errOut, err
}

// Clone clones url into dest and returns the absolute path.
func Clone(url, dest string) (string, error) {
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(destAbs); err == nil {
		return "", fmt.Errorf("destination already exists: %s", destAbs)
	}
	parent := filepath.Dir(destAbs)
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		return "", fmt.Errorf("destination parent does not exist: %s", parent)
	}
	cmd := exec.Command("git", "clone", url, destAbs)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return destAbs, nil
}

// Refresh reads the full git status for dir and returns a GitState.
func Refresh(dir string) (GitState, error) {
	if !IsGitRepo(dir) {
		return GitState{}, fmt.Errorf("not a git repo: %s", dir)
	}
	branch, tracking, ahead, behind, err := branchState(dir)
	if err != nil {
		return GitState{}, err
	}
	stats, err := fileStats(dir)
	if err != nil {
		return GitState{}, err
	}
	staged, unstaged, untracked, changedTotal, stagedTotal := groupedFiles(dir, stats)
	lastCommit, err := latestCommit(dir)
	if err != nil {
		return GitState{}, err
	}
	addedTotal, deletedTotal := aggregateTotals(stats)
	lastFetch, _ := LastFetch(dir)

	return GitState{
		Branch:       branch,
		Tracking:     tracking,
		Ahead:        ahead,
		Behind:       behind,
		Staged:       staged,
		Unstaged:     unstaged,
		Untracked:    untracked,
		Cursor:       0,
		ChangedTotal: changedTotal,
		StagedTotal:  stagedTotal,
		AddedTotal:   addedTotal,
		DeletedTotal: deletedTotal,
		LastCommit:   lastCommit,
		LastFetch:    lastFetch,
	}, nil
}

// LastFetch returns the best available proxy for the last fetch/pull time.
// It tries .git/FETCH_HEAD, then loose refs under .git/refs/remotes/, then .git/packed-refs.
func LastFetch(dir string) (time.Time, error) {
	gitDir := filepath.Join(dir, ".git")
	if fi, err := os.Stat(gitDir); err != nil || !fi.IsDir() {
		// .git might be a gitdir file (worktree); resolve real path
		if common, _, err := RunGit(dir, "rev-parse", "--git-common-dir"); err == nil {
			gitDir = filepath.Clean(filepath.Join(dir, strings.TrimSpace(common)))
		}
	}

	candidates := []string{filepath.Join(gitDir, "FETCH_HEAD")}
	if entries, err := os.ReadDir(filepath.Join(gitDir, "refs", "remotes")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				sub, _ := os.ReadDir(filepath.Join(gitDir, "refs", "remotes", e.Name()))
				for _, se := range sub {
					if !se.IsDir() {
						candidates = append(candidates, filepath.Join(gitDir, "refs", "remotes", e.Name(), se.Name()))
					}
				}
			}
		}
	}
	candidates = append(candidates, filepath.Join(gitDir, "packed-refs"))

	var best time.Time
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if best.IsZero() || st.ModTime().After(best) {
			best = st.ModTime()
		}
	}
	if best.IsZero() {
		return time.Time{}, fmt.Errorf("no fetch timestamp available")
	}
	return best, nil
}

func StageFile(dir, path string) error {
	_, _, err := RunGit(dir, "add", "--", path)
	return err
}

func UnstageFile(dir, path string) error {
	_, _, err := RunGit(dir, "restore", "--staged", "--", path)
	return err
}

func DiscardFile(dir, path string) error {
	_, _, err := RunGit(dir, "checkout", "--", path)
	return err
}

func Commit(dir, msg string) error {
	_, _, err := RunGit(dir, "commit", "-m", msg)
	return err
}

func Push(dir string) error {
	_, _, err := RunGit(dir, "push")
	return err
}

func Pull(dir string) error {
	_, _, err := RunGit(dir, "pull", "--ff-only")
	return err
}

func Fetch(dir string) error {
	_, _, err := RunGit(dir, "fetch", "--all", "--prune")
	return err
}

func UnstageAll(dir string) error {
	_, _, err := RunGit(dir, "restore", "--staged", ".")
	return err
}

func StashPush(dir string, message string) error {
	msg := strings.TrimSpace(message)
	if msg == "" {
		_, _, err := RunGit(dir, "stash", "push", "-u")
		return err
	}
	_, _, err := RunGit(dir, "stash", "push", "-u", "-m", msg)
	return err
}

func StashPop(dir string) error {
	_, _, err := RunGit(dir, "stash", "pop")
	return err
}

func StashList(dir string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 20
	}
	out, _, err := RunGit(dir, "stash", "list")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return []string{}, nil
	}
	lines := strings.Split(out, "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return lines, nil
}

func BranchList(dir string) ([]string, string, error) {
	out, _, err := RunGit(dir, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, "", err
	}
	cur, _, curErr := RunGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if curErr != nil {
		cur = ""
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return []string{}, strings.TrimSpace(cur), nil
	}
	branches := strings.Split(out, "\n")
	for i := range branches {
		branches[i] = strings.TrimSpace(branches[i])
	}
	return branches, strings.TrimSpace(cur), nil
}

func BranchCreate(dir string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	_, _, err := RunGit(dir, "checkout", "-b", name)
	return err
}

func BranchSwitch(dir string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	_, _, err := RunGit(dir, "checkout", name)
	return err
}

func BranchDelete(dir string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	_, _, err := RunGit(dir, "branch", "-d", name)
	return err
}

func CommitLog(dir string, limit int) ([]CommitInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	out, _, err := RunGit(dir, "log", fmt.Sprintf("-%d", limit), "--format=%h\t%s\t%cI")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return []CommitInfo{}, nil
	}
	lines := strings.Split(out, "\n")
	commits := make([]CommitInfo, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		ci := CommitInfo{Hash: strings.TrimSpace(parts[0]), Message: strings.TrimSpace(parts[1])}
		if len(parts) == 3 {
			if tm, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(parts[2])); parseErr == nil {
				ci.Time = tm
			}
		}
		commits = append(commits, ci)
	}
	return commits, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func branchState(dir string) (branch, tracking string, ahead, behind int, err error) {
	branch, _, err = RunGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", "", 0, 0, err
	}
	tracking, _, err = RunGit(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		tracking = ""
		return branch, tracking, 0, 0, nil
	}
	out, _, err := RunGit(dir, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return branch, tracking, 0, 0, nil
	}
	parts := strings.Fields(out)
	if len(parts) >= 2 {
		ahead, _ = strconv.Atoi(parts[0])
		behind, _ = strconv.Atoi(parts[1])
	}
	return branch, tracking, ahead, behind, nil
}

func groupedFiles(dir string, stats map[string][2]int) (staged, unstaged, untracked []FileEntry, changedTotal, stagedTotal int) {
	out, _, err := RunGit(dir, "status", "--porcelain=v1")
	if err != nil || out == "" {
		return []FileEntry{}, []FileEntry{}, []FileEntry{}, 0, 0
	}
	lines := strings.Split(out, "\n")
	changedSet := map[string]struct{}{}

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		x := line[0]
		y := line[1]
		path := strings.TrimSpace(line[3:])
		if idx := strings.LastIndex(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		changedSet[path] = struct{}{}
		st := stats[path]

		if x == '?' && y == '?' {
			untracked = append(untracked, FileEntry{Status: "?", Path: path, Added: st[0], Deleted: st[1]})
			continue
		}
		if x != ' ' && x != '?' {
			staged = append(staged, FileEntry{Status: string(x), Path: path, Added: st[0], Deleted: st[1]})
		}
		if y != ' ' && y != '?' {
			unstaged = append(unstaged, FileEntry{Status: string(y), Path: path, Added: st[0], Deleted: st[1]})
		}
	}
	sortEntries(staged)
	sortEntries(unstaged)
	sortEntries(untracked)
	return staged, unstaged, untracked, len(changedSet), len(staged)
}

func fileStats(dir string) (map[string][2]int, error) {
	stats := map[string][2]int{}
	add := func(path string, a, d int) {
		cur := stats[path]
		cur[0] += a
		cur[1] += d
		stats[path] = cur
	}
	apply := func(out string) {
		if out == "" {
			return
		}
		for _, line := range strings.Split(out, "\n") {
			parts := strings.Split(line, "\t")
			if len(parts) != 3 {
				continue
			}
			a, _ := strconv.Atoi(strings.ReplaceAll(parts[0], "-", "0"))
			d, _ := strconv.Atoi(strings.ReplaceAll(parts[1], "-", "0"))
			path := parts[2]
			if idx := strings.LastIndex(path, " -> "); idx >= 0 {
				path = path[idx+4:]
			}
			add(path, a, d)
		}
	}

	out, _, err := RunGit(dir, "diff", "--numstat")
	if err != nil {
		return nil, err
	}
	apply(out)
	out, _, err = RunGit(dir, "diff", "--cached", "--numstat")
	if err != nil {
		return nil, err
	}
	apply(out)
	return stats, nil
}

func latestCommit(dir string) (CommitInfo, error) {
	out, _, err := RunGit(dir, "log", "-1", "--format=%h\t%s\t%cI")
	if err != nil {
		return CommitInfo{}, nil
	}
	if out == "" {
		return CommitInfo{}, nil
	}
	parts := strings.SplitN(out, "\t", 3)
	if len(parts) != 3 {
		return CommitInfo{}, nil
	}
	tm, _ := time.Parse(time.RFC3339, strings.TrimSpace(parts[2]))
	return CommitInfo{Hash: parts[0], Message: parts[1], Time: tm}, nil
}

func aggregateTotals(stats map[string][2]int) (int, int) {
	add := 0
	del := 0
	for _, s := range stats {
		add += s[0]
		del += s[1]
	}
	return add, del
}
