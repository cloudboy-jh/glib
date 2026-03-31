package piui

import (
	"strings"

	"github.com/cloudboy-jh/glib/internal/pi"
)

func (s *Session) HandleEvent(evt pi.PiEventMsg) {
	switch evt.Type {
	case "noop":
		return
	case "agent_start", "turn_start":
		s.Streaming = true
		s.Status = "thinking"
	case "message_start":
		msg := asMap(evt.Payload["message"])
		if asString(msg["role"]) == "assistant" {
			s.ensureActiveAssistant()
			s.Streaming = true
		}
		s.clearThinkingBlock()
	case "message_update":
		s.handleMessageUpdate(evt.Payload)
	case "message_end":
		msg := asMap(evt.Payload["message"])
		if asString(msg["role"]) == "assistant" {
			final := extractMessageText(msg)
			idx := s.ensureActiveAssistant()
			if final != "" {
				s.Messages[idx].Text = final
			}
			s.Messages[idx].Streaming = false
			s.activeAssistant = -1
		}
	case "turn_end", "agent_end":
		s.StopStreaming()
		s.resetToolActivity(true)
		s.Status = "idle"
	case "tool_execution_start":
		id := asString(evt.Payload["toolCallId"])
		name := asString(evt.Payload["toolName"])
		args := summarizeToolArgs(name, evt.Payload["args"])
		s.StartToolExecution(id, name, args)
		s.Status = "tool running"
	case "tool_execution_update":
		id := asString(evt.Payload["toolCallId"])
		partial := asMap(evt.Payload["partialResult"])
		out := extractToolResultText(partial)
		s.UpdateToolOutput(id, out)
	case "tool_execution_end":
		id := asString(evt.Payload["toolCallId"])
		ok := true
		if errFlag, hasErr := evt.Payload["isError"].(bool); hasErr {
			ok = !errFlag
		}
		result := asMap(evt.Payload["result"])
		out := extractToolResultText(result)
		if out != "" {
			s.UpdateToolOutput(id, out)
		}
		s.CloseToolByID(id, ok)
		if !s.ToolRunning && s.Status == "tool running" {
			s.Status = "thinking"
			if !s.Streaming {
				s.Status = "idle"
			}
		}
	case "auto_compaction_start":
		s.Status = "compacting"
	case "auto_compaction_end":
		s.Status = "idle"
	case "auto_retry_start":
		s.Status = "retrying"
	case "auto_retry_end":
		s.Status = "idle"
	case "extension_ui_request":
		method := asString(evt.Payload["method"])
		switch method {
		case "notify":
			msg := asString(evt.Payload["message"])
			if msg != "" {
				s.Messages = append(s.Messages, Message{Role: RoleTool, ToolBlock: &ToolBlock{Name: "notify", Output: msg, Done: true, ExitOK: true}})
			}
		case "setStatus":
			status := asString(evt.Payload["statusText"])
			if status != "" {
				s.Status = status
			}
		case "set_editor_text":
			text := asString(evt.Payload["text"])
			s.Input.SetValue(text)
		case "setWidget":
			raw, _ := evt.Payload["widgetLines"].([]any)
			if len(raw) == 0 {
				s.WidgetLines = nil
				break
			}
			lines := make([]string, 0, len(raw))
			for _, item := range raw {
				if line, ok := item.(string); ok {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" {
						lines = append(lines, trimmed)
					}
				}
			}
			s.WidgetLines = lines
		case "setTitle":
			// host UI can ignore for now; header already shows session/model context
		default:
			s.OpenModalFromPayload(evt.Payload)
		}
	case "extension_error":
		s.Messages = append(s.Messages, Message{Role: RoleTool, ToolBlock: &ToolBlock{Name: "extension_error", Output: asString(evt.Payload["error"]), Done: true, ExitOK: false}})
	default:
		if s.DebugEvents && len(evt.Raw) > 0 {
			s.Messages = append(s.Messages, Message{Role: RoleTool, ToolBlock: &ToolBlock{Name: "event", Output: string(evt.Raw), Done: true, ExitOK: true}})
		}
	}
}

func (s *Session) handleMessageUpdate(payload map[string]any) {
	evtData := asMap(payload["assistantMessageEvent"])
	switch asString(evtData["type"]) {
	case "text_start":
		s.ensureActiveAssistant()
		s.Streaming = true
		s.Status = "thinking"
	case "text_delta":
		delta := asString(evtData["delta"])
		if delta == "" {
			delta = extractMessageText(asMap(evtData["partial"]))
		}
		if delta != "" {
			idx := s.ensureActiveAssistant()
			s.Messages[idx].Text += delta
			s.Messages[idx].Streaming = true
		}
		s.Streaming = true
		s.Status = "thinking"
	case "text_end":
		content := asString(evtData["content"])
		if content != "" {
			idx := s.ensureActiveAssistant()
			s.Messages[idx].Text = content
		}
	case "thinking_delta":
		content := asString(evtData["delta"])
		s.UpsertThinking(content)
	case "toolcall_start":
		call := asMap(evtData["toolCall"])
		id := asString(call["id"])
		name := asString(call["name"])
		if name == "" {
			name = "tool"
		}
		s.OpenToolByID(id, name, summarizeToolArgs(name, call["arguments"]))
	case "toolcall_delta":
		id := asString(evtData["toolCallId"])
		delta := summarizeToolArgs("", evtData["delta"])
		if delta != "" && id != "" {
			if idx, ok := s.openTools[id]; ok && idx >= 0 && idx < len(s.Messages) {
				if tb := s.Messages[idx].ToolBlock; tb != nil && strings.TrimSpace(tb.Args) == "" {
					tb.Args = delta
				}
			}
		}
	case "toolcall_end":
		call := asMap(evtData["toolCall"])
		id := asString(call["id"])
		args := summarizeToolArgs(asString(call["name"]), call["arguments"])
		if id != "" {
			if idx, ok := s.openTools[id]; ok && idx >= 0 && idx < len(s.Messages) && s.Messages[idx].ToolBlock != nil {
				s.Messages[idx].ToolBlock.Args = args
			}
		}
	case "error":
		s.Status = "idle"
		s.Streaming = false
		s.resetToolActivity(true)
	}
}

func summarizeToolArgs(name string, args any) string {
	if args == nil {
		return ""
	}
	if str, ok := args.(string); ok {
		return summarizeText(str)
	}
	m, ok := args.(map[string]any)
	if !ok {
		return ""
	}
	if cmd := asString(m["command"]); cmd != "" {
		return summarizeText(cmd)
	}
	if path := asString(m["path"]); path != "" {
		return summarizeText(path)
	}
	if query := asString(m["query"]); query != "" {
		return summarizeText(query)
	}
	if text := asString(m["text"]); text != "" {
		return summarizeText(text)
	}
	if strings.TrimSpace(name) != "" {
		return strings.ToLower(strings.TrimSpace(name))
	}
	return ""
}

func summarizeText(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if len(v) <= 88 {
		return v
	}
	return v[:88] + "..."
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func extractMessageText(msg map[string]any) string {
	content, ok := msg["content"].([]any)
	if !ok {
		return ""
	}
	b := strings.Builder{}
	for _, item := range content {
		part, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if asString(part["type"]) != "text" {
			continue
		}
		text := asString(part["text"])
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	return b.String()
}

func extractToolResultText(payload map[string]any) string {
	content, ok := payload["content"].([]any)
	if !ok {
		return ""
	}
	b := strings.Builder{}
	for _, item := range content {
		part, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if asString(part["type"]) != "text" {
			continue
		}
		text := asString(part["text"])
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	return b.String()
}
