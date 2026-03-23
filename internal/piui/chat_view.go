package piui

import (
	"fmt"
	"strings"
)

func (s *Session) ViewLines() []string {
	return strings.Split(s.Viewport.View(), "\n")
}

func (s *Session) ModalLines(width int) []string {
	if !s.Modal.Active {
		return nil
	}
	if width <= 8 {
		width = 8
	}
	lines := []string{s.Modal.Title}
	if s.Modal.Message != "" {
		lines = append(lines, s.Modal.Message)
	}
	switch s.Modal.Method {
	case "select":
		for i, opt := range s.Modal.Options {
			prefix := "  "
			if i == s.Modal.Cursor {
				prefix = "> "
			}
			lines = append(lines, prefix+opt)
		}
	case "confirm":
		lines = append(lines, "y = yes, n = no")
	case "input", "editor":
		lines = append(lines, "type response then press enter")
	default:
		lines = append(lines, fmt.Sprintf("method: %s", s.Modal.Method))
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, clipWrap(line, width))
	}
	return out
}
