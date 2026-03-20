package diffview

import bentodiffs "github.com/cloudboy-jh/bento-diffs/pkg/bentodiffs"

type State struct {
	Source       string
	CommitSHA    string
	LoadedForDir string
	SelectedPath string
}

func MockDiffs() []bentodiffs.DiffResult {
	diffs, err := bentodiffs.MockDiffs(3)
	if err != nil {
		return nil
	}
	return diffs
}
