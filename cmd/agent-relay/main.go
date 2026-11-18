package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/benladamm/agent-relay/internal/adapter"
	"github.com/benladamm/agent-relay/internal/tui"
)

func main() {
	reg := adapter.NewDefaultRegistry()
	model := tui.NewModel(reg)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur lors du lancement de l'interface : %v\n", err)
		os.Exit(1)
	}
}
