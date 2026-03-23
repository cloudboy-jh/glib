package piui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const spinnerInterval = 140 * time.Millisecond

type SpinnerTickMsg struct{}

type spinner struct {
	index int
}

func newSpinner() spinner {
	return spinner{}
}

func (s *spinner) tick() {
	s.index = (s.index + 1) % len(spinnerFrames)
}

func (s *spinner) frame() string {
	return spinnerFrames[s.index]
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func SpinnerTickCmd() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return SpinnerTickMsg{} })
}
