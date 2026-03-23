package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"glib/internal/bentodiffs"
)

type Kind string

const (
	KindLocal     Kind = "local"
	KindEphemeral Kind = "ephemeral"
)

type Manager struct {
	Kind Kind
	Root string

	ephemeral map[string]string
}

type CleanupResult struct {
	Removed  []string
	Skipped  []string
	Warnings []string
}

func NewManager(kind Kind) (*Manager, error) {
	root, err := defaultRoot()
	if err != nil {
		return nil, err
	}
	return &Manager{Kind: kind, Root: root, ephemeral: map[string]string{}}, nil
}

func (m *Manager) SetKind(kind Kind) {
	m.Kind = kind
}

func (m *Manager) EnsureRepo(fullName, cloneURL string) (string, error) {
	if strings.TrimSpace(cloneURL) == "" {
		return "", fmt.Errorf("missing clone url")
	}
	if m.ephemeral == nil {
		m.ephemeral = map[string]string{}
	}
	safeName := strings.ReplaceAll(strings.ToLower(fullName), "/", "__")
	if safeName == "" {
		safeName = "repo"
	}
	repoRoot := filepath.Join(m.Root, safeName)
	if bentodiffs.IsGitRepo(repoRoot) {
		if m.Kind == KindEphemeral {
			worktreeRoot := filepath.Join(m.Root, safeName+"-worktrees")
			if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
				return "", err
			}
			worktreePath := filepath.Join(worktreeRoot, time.Now().UTC().Format("20060102-150405"))
			if _, _, err := bentodiffs.RunGit(repoRoot, "worktree", "add", "--detach", worktreePath); err != nil {
				return "", err
			}
			m.ephemeral[safeName] = worktreePath
			return worktreePath, nil
		}
		return repoRoot, nil
	}

	if m.Kind == KindEphemeral {
		if existing := strings.TrimSpace(m.ephemeral[safeName]); existing != "" {
			if bentodiffs.IsGitRepo(existing) {
				return existing, nil
			}
			delete(m.ephemeral, safeName)
		}

		base, err := m.ensureBaseClone(repoRoot, cloneURL)
		if err != nil {
			return "", err
		}

		worktreeRoot := filepath.Join(repoRoot, "worktrees")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			return "", err
		}
		worktreePath := filepath.Join(worktreeRoot, time.Now().UTC().Format("20060102-150405"))
		if _, _, err := bentodiffs.RunGit(base, "worktree", "add", "--detach", worktreePath); err != nil {
			return "", err
		}
		m.ephemeral[safeName] = worktreePath
		return worktreePath, nil
	}

	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(repoRoot, "main")
	if bentodiffs.IsGitRepo(dest) {
		return dest, nil
	}
	return bentodiffs.Clone(cloneURL, dest)
}

func (m *Manager) CleanupEphemeral() CleanupResult {
	result := CleanupResult{}
	if len(m.ephemeral) == 0 {
		return result
	}
	names := make([]string, 0, len(m.ephemeral))
	for name := range m.ephemeral {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path := strings.TrimSpace(m.ephemeral[name])
		if path == "" {
			continue
		}
		if !bentodiffs.IsGitRepo(path) {
			delete(m.ephemeral, name)
			continue
		}
		out, _, err := bentodiffs.RunGit(path, "status", "--porcelain")
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: status check failed: %v", name, err))
			result.Skipped = append(result.Skipped, path)
			continue
		}
		if strings.TrimSpace(out) != "" {
			result.Skipped = append(result.Skipped, path)
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: skipped dirty worktree", name))
			continue
		}
		base := resolveWorktreeBase(path)
		if _, _, err := bentodiffs.RunGit(base, "worktree", "remove", path); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: remove failed: %v", name, err))
			result.Skipped = append(result.Skipped, path)
			continue
		}
		delete(m.ephemeral, name)
		result.Removed = append(result.Removed, path)
	}

	for _, name := range names {
		path := strings.TrimSpace(m.ephemeral[name])
		if path == "" {
			continue
		}
		if strings.Contains(path, "worktree") {
			base := resolveWorktreeBase(path)
			_, _, _ = bentodiffs.RunGit(base, "worktree", "prune")
		}
	}

	return result
}

func (m *Manager) ensureBaseClone(repoRoot, cloneURL string) (string, error) {
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		return "", err
	}
	base := filepath.Join(repoRoot, "base")
	if bentodiffs.IsGitRepo(base) {
		if _, _, err := bentodiffs.RunGit(base, "fetch", "--all", "--prune"); err != nil {
			return "", err
		}
		return base, nil
	}
	if _, err := os.Stat(base); err == nil {
		return "", fmt.Errorf("base path exists and is not a git repo: %s", base)
	}
	if _, err := bentodiffs.Clone(cloneURL, base); err != nil {
		return "", err
	}
	return base, nil
}

func resolveWorktreeBase(worktreePath string) string {
	common, _, err := bentodiffs.RunGit(worktreePath, "rev-parse", "--git-common-dir")
	if err == nil {
		common = strings.TrimSpace(common)
		if common != "" {
			if filepath.IsAbs(common) {
				return filepath.Dir(common)
			}
			abs := filepath.Clean(filepath.Join(worktreePath, common))
			return filepath.Dir(abs)
		}
	}
	return filepath.Dir(worktreePath)
}

func defaultRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("GLIB_WORKSPACE_ROOT")); v != "" {
		return filepath.Abs(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "glib-workspaces"), nil
}
