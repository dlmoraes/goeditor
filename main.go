package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"goeditor/internal/app" // <-- IMPORTA A NOSSA NOVA CASCA MESTRE
	"goeditor/internal/buffer"
)

func main() {
	filename := ""
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

	buf, err := buffer.New(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goeditor: cannot open %q: %v\n", filename, err)
		os.Exit(1)
	}

	m := app.New(buf)

	// Inicia o programa usando a nossa casca (App) em vez do Editor direto
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "goeditor: runtime error: %v\n", err)
		os.Exit(1)
	}
}
