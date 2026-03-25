// Package bentodiffs re-exports git types and operations from the canonical
// glib/internal/git package. All new code should import glib/internal/git
// directly; this file exists for backward compatibility with home-screen.go.
package bentodiffs

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"glib/internal/git"
)

// Type aliases — existing `bentodiffs.GitState` references keep working.
type FileEntry = git.FileEntry
type CommitInfo = git.CommitInfo
type GitState = git.GitState
type OpenDiffMsg = git.OpenDiffMsg

// Function forwards.

func IsGitRepo(dir string) bool                                 { return git.IsGitRepo(dir) }
func RunGit(dir string, args ...string) (string, string, error) { return git.RunGit(dir, args...) }
func Clone(url, dest string) (string, error)                    { return git.Clone(url, dest) }
func Refresh(dir string) (GitState, error)                      { return git.Refresh(dir) }
func StageFile(dir, path string) error                          { return git.StageFile(dir, path) }
func UnstageFile(dir, path string) error                        { return git.UnstageFile(dir, path) }
func DiscardFile(dir, path string) error                        { return git.DiscardFile(dir, path) }
func Commit(dir, msg string) error                              { return git.Commit(dir, msg) }
func Push(dir string) error                                     { return git.Push(dir) }
func Pull(dir string) error                                     { return git.Pull(dir) }
func Fetch(dir string) error                                    { return git.Fetch(dir) }
func UnstageAll(dir string) error                               { return git.UnstageAll(dir) }
func StashPush(dir, message string) error                       { return git.StashPush(dir, message) }
func StashPop(dir string) error                                 { return git.StashPop(dir) }
func StashList(dir string, limit int) ([]string, error)         { return git.StashList(dir, limit) }
func BranchList(dir string) ([]string, string, error)           { return git.BranchList(dir) }
func BranchCreate(dir, name string) error                       { return git.BranchCreate(dir, name) }
func BranchSwitch(dir, name string) error                       { return git.BranchSwitch(dir, name) }
func BranchDelete(dir, name string) error                       { return git.BranchDelete(dir, name) }
func CommitLog(dir string, limit int) ([]CommitInfo, error)     { return git.CommitLog(dir, limit) }
func RelativeTime(t time.Time) string                           { return git.RelativeTime(t) }

// viewString converts a tea.View to string — kept for home-screen.go compat.
func viewString(v tea.View) string {
	if v.Content == nil {
		return ""
	}
	if r, ok := v.Content.(interface{ Render() string }); ok {
		return r.Render()
	}
	return fmt.Sprint(v.Content)
}
