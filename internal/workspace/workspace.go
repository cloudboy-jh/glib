package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
}

func NewManager(kind Kind) (*Manager, error) {
	root, err := defaultRoot()
	if err != nil {
		return nil, err
	}
	return &Manager{Kind: kind, Root: root}, nil
}

func (m *Manager) SetKind(kind Kind) {
	m.Kind = kind
}

func (m *Manager) EnsureRepo(fullName, cloneURL string) (string, error) {
	if strings.TrimSpace(cloneURL) == "" {
		return "", fmt.Errorf("missing clone url")
	}
	safeName := strings.ReplaceAll(strings.ToLower(fullName), "/", "__")
	if safeName == "" {
		safeName = "repo"
	}

	if m.Kind == KindEphemeral {
		tmpRoot, err := os.MkdirTemp("", "glib-ephemeral-")
		if err != nil {
			return "", err
		}
		dest := filepath.Join(tmpRoot, safeName)
		return bentodiffs.Clone(cloneURL, dest)
	}

	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(m.Root, safeName)
	if bentodiffs.IsGitRepo(dest) {
		return dest, nil
	}
	return bentodiffs.Clone(cloneURL, dest)
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
