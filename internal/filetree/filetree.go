package filetree

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileSelectedMsg é a mensagem que o filetree vai gritar quando você der Enter num arquivo
type FileSelectedMsg struct {
	Path string
}

type Model struct {
	Cwd      string // Current Working Directory (Pasta atual)
	Files    []os.DirEntry
	Cursor   int
	Active   bool // Diz se o foco do teclado está aqui
	Width    int
	Height   int
	viewport int // Para rolar a lista se tiver muitos arquivos
}

func New() Model {
	cwd, _ := os.Getwd()
	m := Model{
		Cwd:    cwd,
		Active: false, // Começa sem foco
	}
	m.loadFiles()
	return m
}

func (m *Model) loadFiles() {
	files, err := os.ReadDir(m.Cwd)
	if err == nil {
		m.Files = files
	}
	m.Cursor = 0
	m.viewport = 0
}

// O CONTRATO DO BUBBLE TEA: O Init que estava faltando!
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Se a árvore não estiver em foco, ignora os atalhos
		if !m.Active {
			return m, nil
		}

		switch msg.Type {
		case tea.KeyUp:
			if m.Cursor > 0 {
				m.Cursor--
				if m.Cursor < m.viewport {
					m.viewport = m.Cursor
				}
			}
		case tea.KeyDown:
			if m.Cursor < len(m.Files)-1 {
				m.Cursor++
				if m.Cursor >= m.viewport+(m.Height-4) {
					m.viewport++
				}
			}
		case tea.KeyBackspace, tea.KeyLeft:
			// Volta uma pasta acima
			m.Cwd = filepath.Dir(m.Cwd)
			m.loadFiles()
			return m, nil
		case tea.KeyEnter, tea.KeyRight:
			if len(m.Files) == 0 {
				return m, nil
			}
			selected := m.Files[m.Cursor]
			path := filepath.Join(m.Cwd, selected.Name())

			if selected.IsDir() {
				// Se for pasta, entra nela
				m.Cwd = path
				m.loadFiles()
				return m, nil
			} else {
				// Se for arquivo, dispara a mensagem de arquivo selecionado!
				return m, func() tea.Msg {
					return FileSelectedMsg{Path: path}
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return ""
	}

	var sb strings.Builder

	// ESTILOS VISUAIS DA GAVETA
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).MarginBottom(1)
	activeItemStyle := lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("255")).Width(m.Width)
	inactiveItemStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Width(m.Width)
	normalItemStyle := lipgloss.NewStyle().Width(m.Width)

	// CABEÇALHO (Nome da pasta atual)
	sb.WriteString(headerStyle.Render(" 📁 " + filepath.Base(m.Cwd)))
	sb.WriteString("\n")

	// ÁREA DOS ARQUIVOS
	visibleLines := m.Height - 4 // Desconta cabeçalho e rodapé
	for i := m.viewport; i < len(m.Files) && i < m.viewport+visibleLines; i++ {
		f := m.Files[i]
		name := f.Name()
		if f.IsDir() {
			name = " 🗀 " + name
		} else {
			name = " 🖹 " + name
		}

		// Corta o nome se for muito grande para caber na gaveta
		if len([]rune(name)) > m.Width-2 {
			name = string([]rune(name)[:m.Width-4]) + ".."
		}

		if i == m.Cursor {
			if m.Active {
				sb.WriteString(activeItemStyle.Render(name) + "\n")
			} else {
				sb.WriteString(inactiveItemStyle.Render(name) + "\n")
			}
		} else {
			sb.WriteString(normalItemStyle.Render(name) + "\n")
		}
	}

	return sb.String()
}
