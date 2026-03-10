package main

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"glib/internal/app"
)

func main() {
	m := app.NewModel()
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
