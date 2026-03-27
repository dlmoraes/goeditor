package app

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"goeditor/internal/buffer"
	"goeditor/internal/editor"
	"goeditor/internal/filetree"
	"goeditor/internal/finder"
)

type focus int

const (
	focusEditor focus = iota
	focusTree
	focusFinder
	focusConfirmQuitAll
	focusHelp // ── NOVO: ESTADO DA TELA DE AJUDA ──
)

type Model struct {
	editors     []editor.Model
	activeIndex int
	tree        filetree.Model
	find        finder.Model
	width       int
	height      int
	showTree    bool
	treeWidth   int
	active      focus
}

func New(buf *buffer.Buffer) Model {
	t := filetree.New()
	t.Active = false

	return Model{
		editors:     []editor.Model{editor.New(buf)},
		activeIndex: 0,
		tree:        t,
		find:        finder.New(),
		showTree:    true,
		treeWidth:   30,
		active:      focusEditor,
	}
}

func (m Model) Init() tea.Cmd {
	return m.editors[m.activeIndex].Init()
}

func (m *Model) handleQuitAllRequest() (tea.Model, tea.Cmd) {
	hasModified := false
	for _, ed := range m.editors {
		if ed.IsModified() {
			hasModified = true
			break
		}
	}

	if hasModified {
		m.active = focusConfirmQuitAll
		return m, nil
	}
	return m, tea.Quit
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		editorWidth := m.width
		if m.showTree {
			editorWidth = m.width - m.treeWidth - 1
			m.tree.Width = m.treeWidth
			m.tree.Height = m.height
		}

		edMsg := tea.WindowSizeMsg{Width: editorWidth, Height: m.height - 1}

		for i := range m.editors {
			edModel, _ := m.editors[i].Update(edMsg)
			m.editors[i] = edModel.(editor.Model)
		}

		treeModel, cmdTree := m.tree.Update(msg)
		m.tree = treeModel.(filetree.Model)
		cmds = append(cmds, cmdTree)

		findModel, cmdFind := m.find.Update(msg)
		m.find = findModel.(finder.Model)
		cmds = append(cmds, cmdFind)

		return m, tea.Batch(cmds...)

	case editor.CloseTabMsg:
		if len(m.editors) > 1 {
			m.editors = append(m.editors[:m.activeIndex], m.editors[m.activeIndex+1:]...)
			if m.activeIndex >= len(m.editors) {
				m.activeIndex = len(m.editors) - 1
			}
			m.active = focusEditor
			m.tree.Active = false
		} else {
			return m, tea.Quit
		}
		return m, nil

	case editor.QuitAllMsg:
		return m.handleQuitAllRequest()
	case editor.ForceQuitAllMsg:
		return m, tea.Quit

	case filetree.FileSelectedMsg:
		for i, ed := range m.editors {
			if ed.GetFilename() == msg.Path {
				m.activeIndex = i
				m.active = focusEditor
				m.tree.Active = false
				return m, nil
			}
		}

		newBuf, err := buffer.New(msg.Path)
		if err == nil {
			newEditor := editor.New(newBuf)
			editorWidth := m.width
			if m.showTree {
				editorWidth = m.width - m.treeWidth - 1
			}
			edModel, _ := newEditor.Update(tea.WindowSizeMsg{Width: editorWidth, Height: m.height - 1})
			m.editors = append(m.editors, edModel.(editor.Model))
			m.activeIndex = len(m.editors) - 1
			m.active = focusEditor
			m.tree.Active = false
		}
		return m, nil

	case finder.OpenResultMsg:
		for i, ed := range m.editors {
			if ed.GetFilename() == msg.Path {
				m.activeIndex = i
				m.active = focusEditor
				m.editors[m.activeIndex].JumpToLine(msg.Line)
				return m, nil
			}
		}
		newBuf, err := buffer.New(msg.Path)
		if err == nil {
			newEditor := editor.New(newBuf)
			editorWidth := m.width
			if m.showTree {
				editorWidth = m.width - m.treeWidth - 1
			}
			edModel, _ := newEditor.Update(tea.WindowSizeMsg{Width: editorWidth, Height: m.height - 1})
			m.editors = append(m.editors, edModel.(editor.Model))
			m.activeIndex = len(m.editors) - 1
			m.active = focusEditor
			m.editors[m.activeIndex].JumpToLine(msg.Line)
		}
		return m, nil

	case tea.KeyMsg:

		// ── LÓGICA DA TELA DE AJUDA ──
		if m.active == focusHelp {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyEnter, tea.KeySpace: // Qualquer tecla comum fecha a ajuda
				m.active = focusEditor
				return m, nil
			case tea.KeyRunes:
				if msg.Alt && len(msg.Runes) > 0 && msg.Runes[0] == 'h' {
					m.active = focusEditor // Alt+H atua como interruptor (Liga/Desliga)
					return m, nil
				}
			}
			return m, nil // Bloqueia outros comandos enquanto lê a ajuda
		}

		// ATALHO PARA ABRIR A AJUDA (Alt + H)
		if msg.Alt && len(msg.Runes) > 0 && msg.Runes[0] == 'h' {
			m.active = focusHelp
			return m, nil
		}

		if m.active == focusConfirmQuitAll {
			switch strings.ToLower(msg.String()) {
			case "s", "y":
				for i := range m.editors {
					if m.editors[i].IsModified() {
						_ = m.editors[i].Save()
					}
				}
				return m, tea.Quit
			case "n":
				return m, tea.Quit
			default:
				m.active = focusEditor
			}
			return m, nil
		}

		if msg.Type == tea.KeyCtrlQ {
			return m.handleQuitAllRequest()
		}

		if m.active == focusFinder && (msg.Type == tea.KeyEsc || msg.Type == tea.KeyCtrlC) {
			m.active = focusEditor
			m.find.Active = false
			return m, nil
		}

		if msg.Alt && len(msg.Runes) > 0 && msg.Runes[0] == 'f' {
			m.active = focusFinder
			m.find.Active = true
			m.find.InputMode = true
			return m, nil
		}

		if m.active == focusFinder {
			findModel, findCmd := m.find.Update(msg)
			m.find = findModel.(finder.Model)
			cmds = append(cmds, findCmd)
			if !m.find.Active {
				m.active = focusEditor
			}
			return m, tea.Batch(cmds...)
		}

		if (msg.Type == tea.KeyRight && msg.Alt) || (msg.Type == tea.KeyPgDown && msg.Alt) {
			if len(m.editors) > 1 {
				m.activeIndex = (m.activeIndex + 1) % len(m.editors)
			}
			return m, nil
		}

		if (msg.Type == tea.KeyLeft && msg.Alt) || (msg.Type == tea.KeyPgUp && msg.Alt) {
			if len(m.editors) > 1 {
				m.activeIndex--
				if m.activeIndex < 0 {
					m.activeIndex = len(m.editors) - 1
				}
			}
			return m, nil
		}

		if msg.Type == tea.KeyCtrlW {
			return m, func() tea.Msg { return editor.CloseTabMsg{} }
		}

		if msg.Type == tea.KeyCtrlE {
			m.showTree = !m.showTree
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}
		}

		if msg.Type == tea.KeyCtrlB {
			if m.showTree {
				if m.active == focusEditor {
					m.active = focusTree
					m.tree.Active = true
				} else {
					m.active = focusEditor
					m.tree.Active = false
				}
			}
			return m, nil
		}

		if m.active == focusTree {
			treeModel, treeCmd := m.tree.Update(msg)
			m.tree = treeModel.(filetree.Model)
			cmds = append(cmds, treeCmd)
		} else {
			edModel, edCmd := m.editors[m.activeIndex].Update(msg)
			m.editors[m.activeIndex] = edModel.(editor.Model)
			cmds = append(cmds, edCmd)
		}
		return m, tea.Batch(cmds...)
	}

	edModel, cmdEd := m.editors[m.activeIndex].Update(msg)
	m.editors[m.activeIndex] = edModel.(editor.Model)
	cmds = append(cmds, cmdEd)

	treeModel, cmdTree := m.tree.Update(msg)
	m.tree = treeModel.(filetree.Model)
	cmds = append(cmds, cmdTree)

	findModel, cmdFind := m.find.Update(msg)
	m.find = findModel.(finder.Model)
	cmds = append(cmds, cmdFind)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	// ── DESENHO DA TELA DE AJUDA MODAL ──
	if m.active == focusHelp {
		helpText := `
  🚀 GoEdit - Central de Atalhos e Comandos

  [ NAVEGAÇÃO & GERAL ]
  Alt + H       : Mostrar/Ocultar esta tela de Ajuda
  Ctrl + E      : Mostrar/Ocultar Gaveta de Arquivos
  Ctrl + B      : Focar na Gaveta / Focar no Editor
  Alt + ⬅ / ➡   : Navegar entre Abas abertas
  Ctrl + W      : Fechar Aba atual
  Ctrl + Q      : Fechar GoEdit (Verifica salvamentos)

  [ EDIÇÃO DE CÓDIGO ]
  Ctrl + S      : Salvar arquivo
  Ctrl + C / X  : Copiar / Cortar (Integra com Windows)
  Ctrl + V      : Colar do sistema
  Ctrl + A      : Selecionar todo o arquivo
  Ctrl + D      : Duplicar Linha atual (ou Bloco selecionado)
  Ctrl + K      : Excluir Linha atual
  Alt + ⬆ / ⬇   : Mover Linha atual para cima/baixo
  Alt + I       : Modo Colagem Segura (Filtro Anti-Escada)
  Ctrl + _ (/)  : Comentar / Descomentar linha

  [ BUSCA & PALETA ]
  Ctrl + F      : Buscar no arquivo atual (Local)
  Ctrl + R      : Localizar e Substituir (Replace)
  Alt + F       : Busca Global (Grep em todo o projeto)
  Ctrl + P      : Paleta de Comandos (:w, :q, :qa, :clear)
  Tab           : Autocompletar Palavras Inteligente
`
		helpBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")). // Verde bonitão
			Padding(1, 4).
			Align(lipgloss.Left).
			Render(helpText + "\n   ( Pressione ESC, Enter ou Alt+H para voltar )")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpBox)
	}

	if m.active == focusConfirmQuitAll {
		dialogBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(1, 4).
			Align(lipgloss.Center).
			Render("⚠️ Existem ficheiros com alterações não salvas!\n\nDeseja salvar tudo antes de encerrar o editor?\n\n[S] Salvar Tudo e Sair  |  [N] Sair Sem Salvar  |  [Esc] Cancelar")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialogBox)
	}

	if m.active == focusFinder {
		return m.find.View()
	}

	var tabs strings.Builder
	activeTabStyle := lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	inactiveTabStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("245")).Padding(0, 1)

	for i, ed := range m.editors {
		name := filepath.Base(ed.GetFilename())
		if name == "" || name == "." {
			name = "[No Name]"
		}
		if ed.IsModified() {
			name += " *"
		}

		if i == m.activeIndex {
			tabs.WriteString(activeTabStyle.Render(name))
		} else {
			tabs.WriteString(inactiveTabStyle.Render(name))
		}
		tabs.WriteString(" ")
	}

	editorView := lipgloss.JoinVertical(lipgloss.Left, tabs.String(), m.editors[m.activeIndex].View())

	if !m.showTree {
		return editorView
	}

	sidebarStyle := lipgloss.NewStyle().
		Width(m.treeWidth).
		Height(m.height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderRight(true)

	rodapeTree := "\n\n  ^E Gaveta | ^B Foco "
	sidebarView := sidebarStyle.Render(m.tree.View() + rodapeTree)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, editorView)
}
