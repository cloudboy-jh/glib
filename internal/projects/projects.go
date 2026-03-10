package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	selectx "github.com/cloudboy-jh/bentotui/registry/components/select"
)

type Entry struct {
	Name  string
	Path  string
	IsDir bool
}

func DefaultCloneDest(url string) string {
	name := RepoNameFromURL(url)
	if name == "" {
		name = "repo"
	}
	cwd, err := os.Getwd()
	if err != nil {
		return name
	}
	return filepath.Join(cwd, name)
}

func RepoNameFromURL(url string) string {
	clean := strings.TrimSpace(url)
	clean = strings.TrimSuffix(clean, ".git")
	clean = strings.TrimSuffix(clean, "/")
	if i := strings.LastIndex(clean, "/"); i >= 0 {
		return clean[i+1:]
	}
	if i := strings.LastIndex(clean, ":"); i >= 0 {
		return clean[i+1:]
	}
	return clean
}

func NormalizePath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("path does not exist: %s", abs)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", abs)
	}
	return abs, nil
}

func ReadEntries(dir string) ([]Entry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(items)+1)
	parent := filepath.Dir(dir)
	if parent != dir {
		entries = append(entries, Entry{Name: "..", Path: parent, IsDir: true})
	}
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		entries = append(entries, Entry{
			Name:  item.Name(),
			Path:  filepath.Join(dir, item.Name()),
			IsDir: item.IsDir(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name == ".." {
			return true
		}
		if entries[j].Name == ".." {
			return false
		}
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func ToItems(entries []Entry) []selectx.Item {
	items := make([]selectx.Item, 0, len(entries))
	for _, e := range entries {
		label := e.Name
		if e.IsDir {
			label += "/"
		}
		items = append(items, selectx.Item{Label: label, Value: e.Path})
	}
	return items
}
