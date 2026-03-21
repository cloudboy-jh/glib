package bentodiffs

import "time"

func MockGitState() GitState {
	return GitState{
		Branch:       "feat/diff-renderer",
		Tracking:     "origin/main",
		Ahead:        1,
		Behind:       0,
		Staged:       mockStaged(),
		Unstaged:     mockUnstaged(),
		Untracked:    mockUntracked(),
		Cursor:       1,
		ChangedTotal: 5,
		StagedTotal:  2,
		AddedTotal:   189,
		DeletedTotal: 54,
		LastCommit: CommitInfo{
			Hash:    "a3f8c21",
			Message: "feat: add unified diff parser",
			Time:    time.Now().Add(-23 * time.Minute),
		},
	}
}

func mockStaged() []FileEntry {
	return []FileEntry{
		{Status: "M", Path: "internal/bentodiffs/diff_state.go", Added: 77, Deleted: 21},
		{Status: "A", Path: "internal/bentodiffs/mock.go", Added: 42, Deleted: 0},
	}
}

func mockUnstaged() []FileEntry {
	return []FileEntry{
		{Status: "M", Path: "internal/app/app.go", Added: 35, Deleted: 12},
		{Status: "M", Path: "internal/bentodiffs/git_state.go", Added: 24, Deleted: 9},
	}
}

func mockUntracked() []FileEntry {
	return []FileEntry{
		{Status: "?", Path: "internal/bentodiffs/mock.go", Added: 11, Deleted: 0},
	}
}
