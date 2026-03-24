package slash

import "strings"

type Command struct {
	Name        string
	Description string
}

var builtin = []Command{
	{Name: "/models", Description: "List and switch models"},
	{Name: "/new", Description: "Start a new session"},
	{Name: "/sessions", Description: "Browse and resume sessions"},
	{Name: "/compact", Description: "Compact current context"},
	{Name: "/fork", Description: "Fork from current message"},
	{Name: "/state", Description: "Show current session state"},
	{Name: "/stats", Description: "Show token and cost stats"},
	{Name: "/commands", Description: "Refresh pi command list"},
	{Name: "/thinking", Description: "Toggle thinking display"},
	{Name: "/tools", Description: "Toggle tool output display"},
	{Name: "/rename", Description: "Rename current session"},
	{Name: "/export", Description: "Export session to HTML"},
	{Name: "/undo", Description: "Undo previous message"},
	{Name: "/theme", Description: "Open theme picker"},
	{Name: "/help", Description: "Show all slash commands"},
	{Name: "/exit", Description: "Exit PI mode"},
}

func Builtin() []Command {
	out := make([]Command, len(builtin))
	copy(out, builtin)
	return out
}

func Find(name string) (Command, bool) {
	q := normalize(name)
	if q == "" {
		return Command{}, false
	}
	for _, c := range builtin {
		if normalize(c.Name) == q {
			return c, true
		}
	}
	return Command{}, false
}

func normalize(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	return v
}
