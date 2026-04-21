package git

import (
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"
)

// FileEntry is a single file in the working tree with its porcelain status
// and line-level diff stats.
type FileEntry struct {
	Status  string
	Path    string
	Added   int
	Deleted int
}

// CommitInfo is a lightweight commit summary.
type CommitInfo struct {
	Hash    string
	Message string
	Time    time.Time
}

// GitState holds all data needed to render the git status view.
type GitState struct {
	Branch    string
	Tracking  string
	Ahead     int
	Behind    int
	Staged    []FileEntry
	Unstaged  []FileEntry
	Untracked []FileEntry
	Cursor    int

	ChangedTotal int
	StagedTotal  int
	AddedTotal   int
	DeletedTotal int

	LastCommit   CommitInfo
	LastFetch    time.Time
	LastAction   string
	LoadedForDir string

	Committing bool
	CommitMsg  string
}

// OpenDiffMsg is emitted when the user wants to open a diff for a file.
type OpenDiffMsg struct {
	Path string
}

type rowKind int

const (
	rowHeader rowKind = iota
	rowFile
)

// Row is one display row in the file list — either a section header or a file.
type Row struct {
	Kind  rowKind
	Label string
	File  FileEntry
}

func (r Row) IsHeader() bool { return r.Kind == rowHeader }
func (r Row) IsFile() bool   { return r.Kind == rowFile }

// Rows builds the ordered display list: STAGED, UNSTAGED, UNTRACKED sections.
func (s *GitState) Rows() []Row {
	rows := make([]Row, 0, len(s.Staged)+len(s.Unstaged)+len(s.Untracked)+6)
	rows = append(rows, Row{Kind: rowHeader, Label: fmt.Sprintf("STAGED (%d)", len(s.Staged))})
	for _, f := range s.Staged {
		rows = append(rows, Row{Kind: rowFile, File: f})
	}
	rows = append(rows, Row{Kind: rowHeader, Label: fmt.Sprintf("UNSTAGED (%d)", len(s.Unstaged))})
	for _, f := range s.Unstaged {
		rows = append(rows, Row{Kind: rowFile, File: f})
	}
	rows = append(rows, Row{Kind: rowHeader, Label: fmt.Sprintf("UNTRACKED (%d)", len(s.Untracked))})
	for _, f := range s.Untracked {
		rows = append(rows, Row{Kind: rowFile, File: f})
	}
	return rows
}

// MoveCursor moves the cursor by delta, skipping header rows.
func (s *GitState) MoveCursor(delta int) {
	rows := s.Rows()
	if len(rows) == 0 {
		s.Cursor = 0
		return
	}
	next := clamp(s.Cursor+delta, 0, len(rows)-1)
	for next >= 0 && next < len(rows) && rows[next].IsHeader() {
		if delta < 0 {
			next--
		} else {
			next++
		}
	}
	if next < 0 || next >= len(rows) || rows[next].IsHeader() {
		return
	}
	s.Cursor = next
}

// SelectedFile returns the FileEntry at the current cursor position.
func (s *GitState) SelectedFile() (FileEntry, bool) {
	rows := s.Rows()
	if len(rows) == 0 || s.Cursor < 0 || s.Cursor >= len(rows) {
		return FileEntry{}, false
	}
	row := rows[s.Cursor]
	if row.IsHeader() {
		return FileEntry{}, false
	}
	return row.File, true
}

// OpenSelectedDiffCmd emits OpenDiffMsg for the selected file.
func (s *GitState) OpenSelectedDiffCmd() tea.Cmd {
	f, ok := s.SelectedFile()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		return OpenDiffMsg{Path: f.Path}
	}
}

// RelativeTime formats a time as a human-readable relative string.
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sortEntries(entries []FileEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
}
