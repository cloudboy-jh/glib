package piui

import (
	"strings"
)

type Modal struct {
	Active      bool
	RequestID   string
	Method      string
	Title       string
	Message     string
	Options     []string
	Cursor      int
	Placeholder string
}

func (s *Session) OpenModalFromPayload(payload map[string]any) {
	id, _ := payload["id"].(string)
	method, _ := payload["method"].(string)
	title, _ := payload["title"].(string)
	message, _ := payload["message"].(string)
	placeholder, _ := payload["placeholder"].(string)

	options := []string{}
	if raw, ok := payload["options"].([]any); ok {
		for _, item := range raw {
			if str, ok := item.(string); ok {
				options = append(options, str)
			}
		}
	}

	s.Modal = Modal{
		Active:      true,
		RequestID:   id,
		Method:      method,
		Title:       strings.TrimSpace(title),
		Message:     strings.TrimSpace(message),
		Options:     options,
		Cursor:      0,
		Placeholder: placeholder,
	}
	if s.Modal.Title == "" {
		s.Modal.Title = "PI Request"
	}
	if s.Modal.Method == "input" || s.Modal.Method == "editor" {
		s.Input.SetPlaceholder(maxString("Type response and press enter", s.Modal.Placeholder))
	}
}

func (s *Session) CloseModal() {
	s.Modal = Modal{}
	s.Input.SetPlaceholder("Message pi...")
}

func maxString(a, b string) string {
	if strings.TrimSpace(b) != "" {
		return b
	}
	return a
}
