package gitops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Path   string
	X      byte
	Y      byte
	Staged bool
}

type LogEntry struct {
	SHA     string
	Subject string
}

func IsGitRepo(dir string) bool {
	out, _, err := RunGit(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

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

func Status(dir string) ([]File, error) {
	out, _, err := RunGit(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []File{}, nil
	}
	lines := strings.Split(out, "\n")
	files := make([]File, 0, len(lines))
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
		files = append(files, File{
			Path:   path,
			X:      x,
			Y:      y,
			Staged: x != ' ' && x != '?',
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func Log(dir string, n int) ([]LogEntry, error) {
	out, _, err := RunGit(dir, "log", "--oneline", "-n", fmt.Sprint(n))
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []LogEntry{}, nil
	}
	lines := strings.Split(out, "\n")
	entries := make([]LogEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		entries = append(entries, LogEntry{SHA: parts[0], Subject: parts[1]})
	}
	return entries, nil
}

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
