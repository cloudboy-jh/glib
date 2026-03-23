package piui

func (s *Session) ScrollDown() {
	s.AutoScroll = false
	s.Viewport.ScrollDown(1)
	if s.Viewport.AtBottom() {
		s.AutoScroll = true
	}
}

func (s *Session) ScrollUp() {
	s.AutoScroll = false
	s.Viewport.ScrollUp(1)
}

func (s *Session) HalfPageDown() {
	s.AutoScroll = false
	s.Viewport.HalfPageDown()
	if s.Viewport.AtBottom() {
		s.AutoScroll = true
	}
}

func (s *Session) HalfPageUp() {
	s.AutoScroll = false
	s.Viewport.HalfPageUp()
}

func (s *Session) GotoBottom() {
	s.AutoScroll = true
	s.Viewport.GotoBottom()
}
