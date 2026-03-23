package piui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/cloudboy-jh/bentotui/theme"
	"github.com/cloudboy-jh/bentotui/theme/styles"
)

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type ToolBlock struct {
	Name      string
	Args      string
	Output    string
	Done      bool
	ExitOK    bool
	Running   bool
	Collapsed bool
	Kind      string
}

type Message struct {
	Role      MessageRole
	Text      string
	ToolBlock *ToolBlock
	Streaming bool
}

func RenderHistory(messages []Message, width int, t theme.Theme) string {
	if width <= 4 {
		width = 4
	}
	out := make([]string, 0, len(messages)*2)
	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			bubble := lipgloss.NewStyle().
				Background(t.BackgroundPanel()).
				Foreground(t.Text()).
				Padding(0, 1).
				Render(clipWrap(msg.Text, max(8, width-4)))
			out = append(out, clipWrap(bubble, width))
		case RoleAssistant:
			text := msg.Text
			if msg.Streaming {
				text += "▋"
			}
			out = append(out, clipWrap(text, width-1))
		case RoleTool:
			if msg.ToolBlock == nil {
				continue
			}
			head := clipWrap(toolHeader(msg.ToolBlock), width-2)
			status := toolStatus(msg.ToolBlock)
			line := lipgloss.NewStyle().Foreground(t.TextMuted()).Render("tool ") + head + " " + renderStatus(status, t)
			block := []string{clipWrap(line, width)}
			if body := toolBody(msg.ToolBlock, width-2); body != "" {
				for _, row := range strings.Split(body, "\n") {
					block = append(block, lipgloss.NewStyle().Foreground(t.TextMuted()).Render("  ")+row)
				}
			}
			out = append(out, strings.Join(block, "\n"))
		}
		out = append(out, "")
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func toolHeader(tb *ToolBlock) string {
	if tb == nil {
		return "tool"
	}
	name := strings.TrimSpace(tb.Name)
	if name == "" {
		name = "tool"
	}
	args := strings.TrimSpace(tb.Args)
	if args == "" {
		return name
	}
	return fmt.Sprintf("%s %s", name, args)
}

func toolStatus(tb *ToolBlock) string {
	if tb == nil {
		return "idle"
	}
	if tb.Running {
		return "running"
	}
	if !tb.Done {
		return "pending"
	}
	if tb.ExitOK {
		return "ok"
	}
	return "error"
}

func toolBody(tb *ToolBlock, width int) string {
	if tb == nil {
		return ""
	}
	out := strings.TrimSpace(tb.Output)
	if out == "" {
		return ""
	}
	if tb.Collapsed {
		return clipWrap(previewLines(out, 3), width)
	}
	return clipWrap(out, width)
}

func previewLines(v string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(v, "\r", ""), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}

func renderStatus(status string, t theme.Theme) string {
	base := lipgloss.NewStyle().Padding(0, 1)
	switch status {
	case "running":
		return base.Foreground(t.Warning()).Background(t.BackgroundPanel()).Render("running")
	case "ok":
		return base.Foreground(t.Success()).Background(t.BackgroundPanel()).Render("ok")
	case "error":
		return base.Foreground(t.Error()).Background(t.BackgroundPanel()).Render("error")
	case "pending":
		return base.Foreground(t.TextMuted()).Background(t.BackgroundPanel()).Render("pending")
	default:
		return base.Foreground(t.TextMuted()).Background(t.BackgroundPanel()).Render(status)
	}
}

func clipWrap(text string, width int) string {
	if width <= 0 {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, styles.ClipANSI(p, width))
	}
	return strings.Join(out, "\n")
}
