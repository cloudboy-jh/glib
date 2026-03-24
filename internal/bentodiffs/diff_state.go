package bentodiffs

import bdcore "github.com/cloudboy-jh/bento-diffs/pkg/bentodiffs"

type DiffState struct {
	Source       string
	CommitSHA    string
	LoadedForDir string
	SelectedPath string
}

func MockDiffs() []bdcore.DiffResult {
	diffs, err := bdcore.MockDiffs(3)
	if err != nil {
		return nil
	}
	return diffs
}

func DiffForFile(repoPath, filePath string) (string, error) {
	if filePath == "" {
		out, _, err := RunGit(repoPath, "diff", "HEAD")
		return out, err
	}
	out, _, err := RunGit(repoPath, "diff", "HEAD", "--", filePath)
	return out, err
}
