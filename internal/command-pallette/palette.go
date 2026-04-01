package commandpallette

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/cloudboy-jh/bentotui/registry/bricks/dialog"
)

type ActionMsg struct {
	ID string
}

type Command struct {
	Name        string
	Description string
	Mode        string // any, PROJECTS, GIT, DIFF, PI
	ID          string
	Keybind     string
}

var registry = []Command{
	{Name: "Switch to Projects", Description: "Go to PROJECTS", Mode: "any", ID: "switch.projects", Keybind: "p"},
	{Name: "Switch to Git", Description: "Go to GIT", Mode: "any", ID: "switch.git", Keybind: "g"},
	{Name: "Switch to Diff", Description: "Go to DIFF", Mode: "any", ID: "switch.diff", Keybind: "d"},
	{Name: "Switch to Pi", Description: "Go to PI", Mode: "any", ID: "switch.pi", Keybind: "i"},
	{Name: "Switch Theme", Description: "Open theme picker", Mode: "any", ID: "theme.open", Keybind: "t"},
	{Name: "Toggle Backend", Description: "Switch local/ephemeral", Mode: "PROJECTS", ID: "projects.backend"},
	{Name: "New Project", Description: "Create new project", Mode: "PROJECTS", ID: "projects.new", Keybind: "n"},
	{Name: "Refresh Repos", Description: "Reload repositories", Mode: "PROJECTS", ID: "projects.refresh", Keybind: "r"},
	{Name: "Sign Out", Description: "Clear GitHub token", Mode: "PROJECTS", ID: "projects.signout"},
	{Name: "New Pi Session", Description: "Start fresh session", Mode: "PI", ID: "pi.new", Keybind: "n"},
	{Name: "Switch Model", Description: "Cycle model", Mode: "PI", ID: "pi.model", Keybind: "m"},
	{Name: "Compact Context", Description: "Compact pi session", Mode: "PI", ID: "pi.compact"},
	{Name: "Toggle Tool Output", Description: "Show/hide tool blocks", Mode: "PI", ID: "pi.tools", Keybind: "ctrl+e"},
	{Name: "Toggle Thinking", Description: "Show/hide thinking", Mode: "PI", ID: "pi.thinking", Keybind: "ctrl+t"},
	{Name: "Export Session", Description: "Export to HTML", Mode: "PI", ID: "pi.export"},
	{Name: "Rename Session", Description: "Set session display name", Mode: "PI", ID: "pi.rename"},
	{Name: "Stage All", Description: "Stage all files", Mode: "GIT", ID: "git.stage_all"},
	{Name: "Commit", Description: "Open commit prompt", Mode: "GIT", ID: "git.commit", Keybind: "c"},
	{Name: "Push", Description: "Push to remote", Mode: "GIT", ID: "git.push", Keybind: "p"},
	{Name: "Open Settings", Description: "Open settings", Mode: "any", ID: "settings.open"},
}

// Open opens the command palette dialog using bentotui flow sizing.
func Open(mode string, termW, termH int) tea.Cmd {
	items := dialogCommands(mode)
	return openWithTitle("Commands", items, termW, termH)
}

func openWithTitle(title string, items []dialog.Command, termW, termH int) tea.Cmd {
	if strings.TrimSpace(title) == "" {
		title = "Commands"
	}
	return func() tea.Msg {
		palette := dialog.NewCommandPalette(flattenKeybinds(items))
		w, h := paletteDialogSize(items, termW, termH)

		return dialog.Open(dialog.Custom{
			DialogTitle: title,
			Content:     palette,
			Width:       w,
			Height:      h,
		})
	}
}

func paletteDialogSize(items []dialog.Command, termW, termH int) (int, int) {
	maxW := termW - 4
	if maxW < 52 {
		maxW = 52
	}
	w := 64
	if w > maxW {
		w = maxW
	}

	groups := make(map[string]bool)
	entryRows := 0
	for _, it := range items {
		entryRows++
		g := strings.TrimSpace(it.Group)
		if g == "" {
			g = "Commands"
		}
		if !groups[g] {
			groups[g] = true
			entryRows++
		}
	}
	if entryRows == 0 {
		entryRows = 2
	}

	h := entryRows + 8
	if h < 16 {
		h = 16
	}
	if h > 24 {
		h = 24
	}
	maxH := termH - 4
	if maxH < 12 {
		maxH = 12
	}
	if h > maxH {
		h = maxH
	}

	return w, h
}

func flattenKeybinds(items []dialog.Command) []dialog.Command {
	out := make([]dialog.Command, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Keybind) == "" {
			out = append(out, it)
			continue
		}
		it.Label = it.Label + "  · " + it.Keybind
		it.Keybind = ""
		out = append(out, it)
	}
	return out
}

func dialogCommands(mode string) []dialog.Command {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	out := make([]dialog.Command, 0, len(registry))
	for _, c := range registry {
		if !isModeAllowed(mode, c.Mode) {
			continue
		}
		id := c.ID
		out = append(out, dialog.Command{
			Label:   c.Name,
			Group:   groupName(c.Mode),
			Keybind: c.Keybind,
			Action: func() tea.Msg {
				return ActionMsg{ID: id}
			},
		})
	}
	return out
}

func isModeAllowed(activeMode, commandMode string) bool {
	cm := strings.ToUpper(strings.TrimSpace(commandMode))
	if cm == "" || cm == "ANY" {
		return true
	}
	return cm == activeMode
}

func groupName(mode string) string {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	if mode == "" || mode == "ANY" {
		return "General"
	}
	return mode
}
