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
	focusFinder // NOVO ESTADO DE FOCO
)

type Model struct {
	editors     []editor.Model
	activeIndex int
	tree        filetree.Model
	find        finder.Model // A NOSSA MÁQUINA DE BUSCA
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
		find:        finder.New(), // Inicializa o Finder
		showTree:    true,
		treeWidth:   30,
		active:      focusEditor,
	}
}

func (m Model) Init() tea.Cmd {
	return m.editors[m.activeIndex].Init()
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

		// Atualiza o tamanho do Finder
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

	// ── QUANDO O FINDER ENCONTRA E MANDA ABRIR O FICHEIRO ──
	case finder.OpenResultMsg:
		// Verifica se já está aberto
		for i, ed := range m.editors {
			if ed.GetFilename() == msg.Path {
				m.activeIndex = i
				m.active = focusEditor
				m.editors[m.activeIndex].JumpToLine(msg.Line) // Salta para a linha!
				return m, nil
			}
		}

		// Se não estiver aberto, cria a aba, foca e salta para a linha
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
			m.editors[m.activeIndex].JumpToLine(msg.Line) // Salta para a linha!
		}
		return m, nil

	case tea.KeyMsg:
		// Se o Finder estiver ativo e apertarem ESC, fechamos ele
		if m.active == focusFinder && (msg.Type == tea.KeyEsc || msg.Type == tea.KeyCtrlC) {
			m.active = focusEditor
			m.find.Active = false
			return m, nil
		}

		// ATALHO PARA A BUSCA GLOBAL (Alt + F)
		if msg.Alt && len(msg.Runes) > 0 && msg.Runes[0] == 'f' {
			m.active = focusFinder
			m.find.Active = true
			m.find.InputMode = true
			return m, nil
		}

		// ROTEAMENTO DE EVENTOS
		if m.active == focusFinder {
			findModel, findCmd := m.find.Update(msg)
			m.find = findModel.(finder.Model)
			cmds = append(cmds, findCmd)

			// Se o update do Finder disse que ele não está mais ativo, devolve o foco pro Editor
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
			if len(m.editors) > 1 {
				m.editors = append(m.editors[:m.activeIndex], m.editors[m.activeIndex+1:]...)
				if m.activeIndex >= len(m.editors) {
					m.activeIndex = len(m.editors) - 1
				}
			} else {
				return m, tea.Quit
			}
			return m, nil
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

	// Se o Finder está ativo, ele rouba a tela inteira (Modo Tela Cheia)
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
