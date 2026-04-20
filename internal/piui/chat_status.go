package piui

import "strings"

type FooterState struct {
	Context  string
	Scroll   string
	Position string
}

func (s *Session) FooterState(icon, repo string) FooterState {
	prefix := icon
	if strings.TrimSpace(repo) != "" {
		prefix += " " + repo
	}
	state := strings.TrimSpace(s.Status)
	if state == "idle" {
		state = ""
	}
	ctx := strings.TrimSpace(prefix + " ↑/↓ scroll  pgup/pgdn  enter send  esc back")
	if s.Streaming {
		ctx = strings.TrimSpace(prefix + " ↑/↓ scroll  pgup/pgdn  enter send  esc abort  s steer")
	}
	if s.Modal.Active {
		ctx = strings.TrimSpace(prefix + " modal active  enter confirm  esc cancel")
		state = "awaiting input"
	}
	if s.ToolRunning {
		state = "tool running"
	} else if !s.Busy() && (state == "thinking" || state == "tool running" || state == "idle") {
		state = ""
	}
	pos := ""
	if !s.Viewport.AtBottom() {
		pos = "not following"
	}
	return FooterState{Context: ctx, Scroll: state, Position: pos}
}
