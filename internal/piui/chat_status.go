package piui

import "strings"

type FooterState struct {
	Context  string
	Scroll   string
	Position string
}

func (s *Session) FooterState(icon, repo string) FooterState {
	spinner := s.SpinnerFrame()
	prefix := icon
	if spinner != "" {
		prefix += " " + spinner
	}
	if strings.TrimSpace(repo) != "" {
		prefix += " " + repo
	}
	if s.CmdPrefix {
		return FooterState{
			Context: prefix + " cmd: p projects  d diff  g git  i pi  m model  n session  G bottom  j/k scroll",
			Scroll:  "awaiting key",
		}
	}
	state := strings.TrimSpace(s.Status)
	if state == "idle" {
		state = ""
	}
	ctx := prefix + " enter send  esc back  ctrl+o tools  ctrl+t thinking"
	if s.Streaming {
		ctx = prefix + " enter send  esc abort  s steer  ctrl+o tools  ctrl+t thinking"
	}
	if s.Modal.Active {
		ctx = prefix + " modal active  enter confirm  esc cancel"
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
