package piui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	spinnerIdleInterval     = time.Second / 7
	spinnerThinkingInterval = time.Second / 10
	spinnerToolInterval     = time.Second / 9
)

type SpinnerTickMsg struct{}

type spinner struct {
	index int
	mode  spinnerMode
}

func newSpinner() spinner {
	return spinner{mode: spinnerModeIdle}
}

type spinnerMode int

const (
	spinnerModeIdle spinnerMode = iota
	spinnerModeThinking
	spinnerModeTool
)

func (s *spinner) setMode(mode spinnerMode) {
	if s.mode == mode {
		return
	}
	s.mode = mode
	s.index = 0
}

func (s *spinner) tick(mode spinnerMode) {
	s.setMode(mode)
	frames := s.frames()
	if len(frames) == 0 {
		s.index = 0
		return
	}
	s.index = (s.index + 1) % len(frames)
}

func (s *spinner) frame(mode spinnerMode) string {
	s.setMode(mode)
	frames := s.frames()
	if len(frames) == 0 {
		return ""
	}
	return frames[s.index]
}

func (s *spinner) frames() []string {
	switch s.mode {
	case spinnerModeTool:
		return spinnerToolFrames
	case spinnerModeThinking:
		return spinnerThinkingFrames
	default:
		return spinnerIdleFrames
	}
}

func spinnerInterval(mode spinnerMode) time.Duration {
	switch mode {
	case spinnerModeTool:
		return spinnerToolInterval
	case spinnerModeThinking:
		return spinnerThinkingInterval
	default:
		return spinnerIdleInterval
	}
}

var spinnerIdleFrames = []string{"∙∙∙", "●∙∙", "∙●∙", "∙∙●"}
var spinnerThinkingFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
var spinnerToolFrames = []string{"▱▱▱", "▰▱▱", "▰▰▱", "▰▰▰", "▰▰▱", "▰▱▱"}

func SpinnerTickCmd(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		interval = spinnerIdleInterval
	}
	return tea.Tick(interval, func(time.Time) tea.Msg { return SpinnerTickMsg{} })
}
