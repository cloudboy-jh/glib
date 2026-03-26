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
	for i, msg := range messages {
		if i == 0 && msg.Role == RoleUser {
			out = append(out, "")
		}
		switch msg.Role {
		case RoleUser:
			out = append(out, renderUserCard(msg.Text, width, t))
		case RoleAssistant:
			text := renderAssistantText(msg.Text, width-3, t)
			if msg.Streaming {
				text += lipgloss.NewStyle().Foreground(t.BorderFocus()).Render("▋")
			}
			out = append(out, renderAssistantBlock(text, width, t))
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
		if i < len(messages)-1 {
			out = append(out, "")
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
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

func renderAssistantText(v string, width int, t theme.Theme) string {
	v = strings.ReplaceAll(v, "\r", "")
	if strings.TrimSpace(v) == "" {
		return ""
	}
	lines := strings.Split(v, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			clean := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			out = append(out, lipgloss.NewStyle().Bold(true).Foreground(t.TextAccent()).Render(clean))
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			bullet := lipgloss.NewStyle().Foreground(t.TextAccent()).Render("•")
			body := renderInlineMarkdown(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), t)
			out = append(out, bullet+" "+body)
			continue
		}
		out = append(out, renderInlineMarkdown(line, t))
	}
	return clipWrap(strings.Join(out, "\n"), width)
}

func renderInlineMarkdown(v string, t theme.Theme) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	parts := strings.Split(v, "`")
	b := strings.Builder{}
	for i, part := range parts {
		if i%2 == 1 {
			chunk := lipgloss.NewStyle().Foreground(t.SyntaxString()).Render(part)
			b.WriteString(chunk)
			continue
		}
		plain := strings.ReplaceAll(part, "**", "")
		b.WriteString(lipgloss.NewStyle().Foreground(t.Text()).Render(plain))
	}
	return b.String()
}

func renderUserCard(text string, width int, t theme.Theme) string {
	innerW := maxInt(8, width-2)
	body := clipWrap(strings.TrimSpace(text), innerW-2)
	if strings.TrimSpace(body) == "" {
		body = " "
	}
	content := lipgloss.NewStyle().Foreground(t.Text()).Render(body)
	return lipgloss.NewStyle().
		Background(t.BackgroundPanel()).
		Border(lipgloss.Border{Left: "┃"}, false, false, false, true).
		BorderForeground(t.BorderFocus()).
		PaddingTop(1).
		PaddingBottom(1).
		PaddingLeft(1).
		PaddingRight(1).
		Width(innerW).
		Render(content)
}

func renderAssistantBlock(text string, width int, t theme.Theme) string {
	innerW := maxInt(8, width-2)
	if strings.TrimSpace(text) == "" {
		text = " "
	}
	body := clipWrap(text, innerW-2)
	return lipgloss.NewStyle().
		Foreground(t.Text()).
		PaddingLeft(3).
		Width(innerW).
		Render(body)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
