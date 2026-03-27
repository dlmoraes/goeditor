package finder

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Mensagem que o Finder envia para o App quando o utilizador escolhe um resultado
type OpenResultMsg struct {
	Path string
	Line int
}

type Result struct {
	Path    string
	Line    int
	Content string
}

type Model struct {
	Query     string
	Results   []Result
	Selected  int
	Active    bool
	Width     int
	Height    int
	InputMode bool // true = Escrevendo a busca, false = Navegando nos resultados
}

func New() Model {
	return Model{
		Active:    false,
		InputMode: true,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// Ignora pastas pesadas e ficheiros binários para a busca ser instantânea
func isIgnored(path string) bool {
	parts := strings.Split(path, string(os.PathSeparator))
	for _, p := range parts {
		if p == ".git" || p == "node_modules" || p == "vendor" || p == ".dbt" || p == "target" {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".exe", ".dll", ".png", ".jpg", ".jpeg", ".zip", ".tar", ".gz":
		return true
	}
	return false
}

func (m *Model) executeSearch() {
	m.Results = nil
	m.Selected = 0
	if m.Query == "" {
		return
	}

	q := strings.ToLower(m.Query)

	// Varre a pasta atual e subpastas
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || isIgnored(path) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			lineText := scanner.Text()
			if strings.Contains(strings.ToLower(lineText), q) {
				m.Results = append(m.Results, Result{
					Path:    path,
					Line:    lineNum - 1, // Zero-indexed para o nosso buffer
					Content: strings.TrimSpace(lineText),
				})
				// Limite de segurança para não explodir a memória
				if len(m.Results) > 500 {
					return filepath.SkipDir
				}
			}
		}
		return nil
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.InputMode {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.Active = false
			case tea.KeyEnter:
				m.executeSearch()
				if len(m.Results) > 0 {
					m.InputMode = false // Passa para o modo de navegação com setas
				}
			case tea.KeyBackspace:
				if len(m.Query) > 0 {
					m.Query = m.Query[:len(m.Query)-1]
				}
			case tea.KeySpace:
				m.Query += " "
			case tea.KeyRunes:
				cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
				m.Query += cleanStr
			}
		} else {
			// Modo de Navegação nos Resultados
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.Active = false
			case tea.KeyBackspace:
				m.InputMode = true // Volta a editar a busca
			case tea.KeyUp:
				m.Selected--
				if m.Selected < 0 {
					m.Selected = len(m.Results) - 1
				}
			case tea.KeyDown:
				m.Selected = (m.Selected + 1) % len(m.Results)
			case tea.KeyEnter:
				if len(m.Results) > 0 {
					res := m.Results[m.Selected]
					m.Active = false
					// Manda o App abrir este ficheiro e saltar para a linha!
					return m, func() tea.Msg { return OpenResultMsg{Path: res.Path, Line: res.Line} }
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	if !m.Active {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n  🔎 Busca Global (Grep)\n")
	sb.WriteString("  " + strings.Repeat("─", m.Width-4) + "\n\n")

	// Barra de Pesquisa
	cursor := "_"
	if !m.InputMode {
		cursor = ""
	}
	sb.WriteString(fmt.Sprintf("  Buscar por: %s%s\n\n", m.Query, cursor))

	// Resultados
	if len(m.Results) == 0 {
		if !m.InputMode {
			sb.WriteString("  Nenhum resultado encontrado.\n")
		} else {
			sb.WriteString("  Digite a palavra e pressione ENTER para buscar nas pastas...\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("  %d resultado(s) encontrado(s):\n\n", len(m.Results)))

		maxView := m.Height - 12
		start := 0
		if m.Selected > maxView-1 {
			start = m.Selected - maxView + 1
		}

		for i := start; i < len(m.Results) && i < start+maxView; i++ {
			res := m.Results[i]
			prefix := "  "
			if i == m.Selected && !m.InputMode {
				prefix = "▶ "
			}

			// Limita o tamanho do texto para não quebrar a tela
			content := res.Content
			maxLen := m.Width - 10
			if len(content) > maxLen && maxLen > 0 {
				content = content[:maxLen] + "..."
			}

			if i == m.Selected && !m.InputMode {
				selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
				sb.WriteString(selectedStyle.Render(fmt.Sprintf("%s%s:%d -> %s", prefix, res.Path, res.Line+1, content)) + "\n")
			} else {
				sb.WriteString(fmt.Sprintf("%s%s:%d -> %s\n", prefix, res.Path, res.Line+1, content))
			}
		}
	}

	footer := "  Esc Cancelar   Enter Confirmar   Backspace Editar"
	if m.InputMode {
		footer = "  Esc Fechar   Enter Buscar"
	}

	// Posiciona o rodapé no fundo da tela
	linhasAtuais := strings.Count(sb.String(), "\n")
	linhasFaltantes := m.Height - linhasAtuais - 3
	if linhasFaltantes > 0 {
		sb.WriteString(strings.Repeat("\n", linhasFaltantes))
	}

	styleFooter := lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("245")).Width(m.Width)
	sb.WriteString(styleFooter.Render(footer))

	return sb.String()
}
