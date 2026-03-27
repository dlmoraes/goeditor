package editor

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/atotto/clipboard"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.adjustCamera()
		return m, tea.ClearScreen

	case tea.KeyMsg:
		if msg.Paste {
			text := strings.ReplaceAll(string(msg.Runes), "\r", "")
			text = strings.ReplaceAll(text, "\x00", "")

			switch m.currentMode {
			case modeSearch:
				m.searchQuery += strings.ReplaceAll(text, "\n", "")
			case modeReplace:
				if m.replaceField == 0 {
					m.searchQuery += strings.ReplaceAll(text, "\n", "")
				} else {
					m.replaceQuery += strings.ReplaceAll(text, "\n", "")
				}
			case modeGoto:
				m.gotoInput += strings.ReplaceAll(text, "\n", "")
			case modeCommand:
				m.commandInput += strings.ReplaceAll(text, "\n", "")
			default:
				for _, ch := range text {
					if ch == '\n' {
						m.buf.InsertLine()
						if !m.autoIndent {
							linhaAtual := []rune(m.buf.Lines[m.buf.CursorY])
							if m.buf.CursorX > 0 && m.buf.CursorX <= len(linhaAtual) {
								m.buf.Lines[m.buf.CursorY] = string(linhaAtual[m.buf.CursorX:])
								m.buf.CursorX = 0
							}
						}
					} else {
						m.buf.InsertChar(ch)
					}
				}
				m.adjustCamera()
			}
			return m, nil
		}

		if m.currentMode == modeConfirmQuit {
			switch strings.ToLower(msg.String()) {
			case "s", "y":
				_ = m.buf.Save()
				return m, func() tea.Msg { return CloseTabMsg{} }
			case "n":
				return m, func() tea.Msg { return CloseTabMsg{} }
			default:
				m.currentMode = modeNormal
				m.setStatus("Ação cancelada.", 0)
			}
			return m, nil
		}

		if m.currentMode == modeAutocomplete {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.cancelAutocomplete()
				return m, nil
			case tea.KeyTab, tea.KeyDown, tea.KeyCtrlL:
				m.acIndex = (m.acIndex + 1) % len(m.acSuggestions)
				m.applyAutocomplete()
				return m, nil
			case tea.KeyUp, tea.KeyShiftTab:
				m.acIndex--
				if m.acIndex < 0 {
					m.acIndex = len(m.acSuggestions) - 1
				}
				m.applyAutocomplete()
				return m, nil
			case tea.KeyEnter:
				m.currentMode = modeNormal
				return m, nil
			case tea.KeyBackspace:
				m.cancelAutocomplete()
			default:
				m.currentMode = modeNormal
			}
		}

		// ── CORREÇÃO DO ALTGR NOS CAMPOS DE INPUT ──
		if m.currentMode == modeCommand {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.currentMode = modeNormal
				m.commandInput = ""
			case tea.KeyEnter:
				cmd := m.executeCommand()
				m.currentMode = modeNormal
				m.commandInput = ""
				if cmd != nil {
					return m, cmd
				}
			case tea.KeyBackspace:
				m.commandInput = runeBackspace(m.commandInput)
			case tea.KeySpace:
				m.commandInput += " "
			case tea.KeyRunes:
				// Removida a barreira do "if !msg.Alt". Agora o comando aceita barras (/)!
				cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
				m.commandInput += cleanStr
			}
			return m, nil
		}

		if m.currentMode == modeSearch {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.currentMode = modeNormal
				m.searchQuery = ""
				m.searchHits = nil
			case tea.KeyEnter:
				m.executeSearch()
				m.currentMode = modeNormal
			case tea.KeyBackspace:
				m.searchQuery = runeBackspace(m.searchQuery)
			case tea.KeySpace:
				m.searchQuery += " "
			case tea.KeyRunes:
				cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
				m.searchQuery += cleanStr
			}
			return m, nil
		}

		if m.currentMode == modeReplace {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.currentMode = modeNormal
			case tea.KeyTab:
				m.replaceField = 1 - m.replaceField
			case tea.KeyEnter:
				if m.replaceField == 0 {
					m.replaceField = 1
				} else {
					n := m.buf.ReplaceAll(m.searchQuery, m.replaceQuery)
					m.setStatus(fmt.Sprintf("Substituídas %d ocorrência(s).", n), 1)
					m.currentMode = modeNormal
					m.searchHits = nil
				}
			case tea.KeyBackspace:
				if m.replaceField == 0 {
					m.searchQuery = runeBackspace(m.searchQuery)
				} else {
					m.replaceQuery = runeBackspace(m.replaceQuery)
				}
			case tea.KeySpace:
				if m.replaceField == 0 {
					m.searchQuery += " "
				} else {
					m.replaceQuery += " "
				}
			case tea.KeyRunes:
				cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
				if m.replaceField == 0 {
					m.searchQuery += cleanStr
				} else {
					m.replaceQuery += cleanStr
				}
			}
			return m, nil
		}

		if m.currentMode == modeGoto {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.currentMode = modeNormal
				m.gotoInput = ""
			case tea.KeyEnter:
				m.executeGoto()
				m.currentMode = modeNormal
				m.gotoInput = ""
			case tea.KeyBackspace:
				m.gotoInput = runeBackspace(m.gotoInput)
			case tea.KeyRunes:
				for _, ch := range string(msg.Runes) {
					if ch >= '0' && ch <= '9' {
						m.gotoInput += string(ch)
					}
				}
			}
			return m, nil
		}

		m.setStatus("", 0)
		switch msg.Type {

		case tea.KeyCtrlA:
			m.buf.CursorY = 0
			m.buf.CursorX = 0
			m.buf.StartSelection()
			m.buf.CursorY = len(m.buf.Lines) - 1
			m.buf.CursorX = len([]rune(m.buf.Lines[m.buf.CursorY]))
			m.adjustCamera()
			m.setStatus("Todo o texto selecionado (Pressione Backspace para apagar).", 1)
			return m, nil

		case tea.KeyCtrlAt:
			m.startAutocomplete()
			return m, nil

		case tea.KeyCtrlX:
			if m.buf.HasSelection() {
				m.clipboard = m.buf.DeleteSelectedText()
				_ = clipboard.WriteAll(m.clipboard)
				m.setStatus(fmt.Sprintf("Cortados %d chars para a área de transferência.", len([]rune(m.clipboard))), 1)
				m.adjustCamera()
			} else {
				m.buf.ClearSelection()
				if m.buf.Modified {
					m.currentMode = modeConfirmQuit
				} else {
					return m, tea.Quit
				}
			}

		case tea.KeyEsc:
			m.buf.ClearSelection()

		case tea.KeyCtrlC:
			if m.buf.HasSelection() {
				m.clipboard = m.buf.GetSelectedText()
				_ = clipboard.WriteAll(m.clipboard)
				m.setStatus(fmt.Sprintf("Copiados %d chars para a área de transferência.", len([]rune(m.clipboard))), 1)
				m.buf.ClearSelection()
			} else {
				if m.buf.Modified {
					m.currentMode = modeConfirmQuit
				} else {
					return m, tea.Quit
				}
			}

		case tea.KeyCtrlS:
			if err := m.buf.Save(); err != nil {
				m.setStatus("Erro ao guardar: "+err.Error(), 2)
			} else {
				m.setStatus("Ficheiro guardado com sucesso.", 1)
			}

		case tea.KeyCtrlZ:
			if !m.buf.Undo() {
				m.setStatus("Nada para anular.", 0)
			}
			m.adjustCamera()
		case tea.KeyCtrlY:
			if !m.buf.Redo() {
				m.setStatus("Nada para refazer.", 0)
			}
			m.adjustCamera()

		case tea.KeyCtrlK:
			m.buf.ClearSelection()
			m.buf.DeleteCurrentLine()
			m.adjustCamera()

		case tea.KeyCtrlD:
			if m.buf.HasSelection() {
				textToDup := m.buf.GetSelectedText()
				_, _, ey, ex := m.buf.SelBounds()
				m.buf.CursorY = ey
				m.buf.CursorX = ex
				m.buf.ClearSelection()
				m.buf.InsertString(textToDup)
				m.setStatus("Bloco de texto duplicado.", 1)
			} else {
				m.buf.ClearSelection()
				m.buf.DuplicateCurrentLine()
				m.setStatus("Linha atual duplicada.", 1)
			}
			m.adjustCamera()

		case tea.KeyCtrlP:
			m.currentMode = modeCommand
			m.commandInput = ""
			return m, nil

		case tea.KeyCtrlV:
			textToPaste, err := clipboard.ReadAll()
			if err != nil || textToPaste == "" {
				textToPaste = m.clipboard
			}

			if textToPaste != "" {
				if m.buf.HasSelection() {
					m.buf.DeleteSelectedText()
				}
				textToPaste = strings.ReplaceAll(textToPaste, "\r\n", "\n")
				m.buf.InsertString(textToPaste)
				m.adjustCamera()
			}

		case tea.KeyCtrlUnderscore:
			m.buf.ClearSelection()
			m.buf.ToggleComment(commentPrefixFor(m.buf.Filename))

		case tea.KeyCtrlF:
			m.currentMode = modeSearch
			m.searchQuery = ""
		case tea.KeyCtrlR:
			m.currentMode = modeReplace
			m.searchQuery = ""
			m.replaceQuery = ""
			m.replaceField = 0
		case tea.KeyCtrlN:
			m.nextSearchHit()
		case tea.KeyCtrlG:
			m.currentMode = modeGoto
			m.gotoInput = ""

		case tea.KeyUp:
			if msg.Alt {
				m.buf.ClearSelection()
				m.buf.SwapLineUp()
			} else {
				m.buf.ClearSelection()
				m.buf.MoveCursor(0, -1)
			}
			m.adjustCamera()
		case tea.KeyDown:
			if msg.Alt {
				m.buf.ClearSelection()
				m.buf.SwapLineDown()
			} else {
				m.buf.ClearSelection()
				m.buf.MoveCursor(0, 1)
			}
			m.adjustCamera()
		case tea.KeyLeft:
			m.buf.ClearSelection()
			m.buf.MoveCursor(-1, 0)
			m.adjustCamera()
		case tea.KeyRight:
			m.buf.ClearSelection()
			m.buf.MoveCursor(1, 0)
			m.adjustCamera()

		case tea.KeyShiftUp:
			if !m.buf.HasSelection() {
				m.buf.StartSelection()
			}
			m.buf.MoveCursor(0, -1)
			m.adjustCamera()
		case tea.KeyShiftDown:
			if !m.buf.HasSelection() {
				m.buf.StartSelection()
			}
			m.buf.MoveCursor(0, 1)
			m.adjustCamera()
		case tea.KeyShiftLeft:
			if !m.buf.HasSelection() {
				m.buf.StartSelection()
			}
			m.buf.MoveCursor(-1, 0)
			m.adjustCamera()
		case tea.KeyShiftRight:
			if !m.buf.HasSelection() {
				m.buf.StartSelection()
			}
			m.buf.MoveCursor(1, 0)
			m.adjustCamera()

		case tea.KeyHome:
			m.buf.ClearSelection()
			m.buf.CursorX = 0
		case tea.KeyEnd:
			m.buf.ClearSelection()
			m.buf.CursorX = len([]rune(m.buf.Lines[m.buf.CursorY]))
		case tea.KeyPgUp:
			m.buf.ClearSelection()
			m.buf.MoveCursor(0, -m.viewHeight())
			m.adjustCamera()
		case tea.KeyPgDown:
			m.buf.ClearSelection()
			m.buf.MoveCursor(0, m.viewHeight())
			m.adjustCamera()

		case tea.KeyEnter:
			m.buf.ClearSelection()
			m.buf.InsertLine()

			if !m.autoIndent {
				linhaAtual := []rune(m.buf.Lines[m.buf.CursorY])
				if m.buf.CursorX > 0 && m.buf.CursorX <= len(linhaAtual) {
					m.buf.Lines[m.buf.CursorY] = string(linhaAtual[m.buf.CursorX:])
					m.buf.CursorX = 0
				}
			}
			m.adjustCamera()

		case tea.KeyBackspace:
			if m.buf.HasSelection() {
				m.buf.DeleteSelectedText()
			} else {
				m.buf.DeleteChar()
			}
			m.adjustCamera()
		case tea.KeyDelete:
			if m.buf.HasSelection() {
				m.buf.DeleteSelectedText()
				m.adjustCamera()
			} else if m.buf.CursorX < len([]rune(m.buf.Lines[m.buf.CursorY])) {
				m.buf.MoveCursor(1, 0)
				m.buf.DeleteChar()
				m.adjustCamera()
			}

		case tea.KeyTab:
			m.buf.ClearSelection()
			line := m.buf.Lines[m.buf.CursorY]
			col := m.buf.CursorX
			r := []rune(line)

			if col > 0 && col <= len(r) && isWordChar(r[col-1]) {
				m.startAutocomplete()
			} else {
				m.buf.InsertString("    ")
				m.adjustCamera()
			}

		case tea.KeySpace:
			if m.buf.HasSelection() {
				m.buf.DeleteSelectedText()
			}
			m.buf.InsertChar(' ')
			m.adjustCamera()

		case tea.KeyRunes:
			// ── CORREÇÃO DO ALTGR NO EDITOR PRINCIPAL ──
			if msg.Alt {
				if len(msg.Runes) == 1 && msg.Runes[0] == 'i' {
					m.autoIndent = !m.autoIndent
					if m.autoIndent {
						m.setStatus("Auto-Indentação ATIVADA.", 1)
					} else {
						m.setStatus("Modo Colagem (Auto-Indentação DESATIVADA).", 0)
					}
					return m, nil
				}
				if len(msg.Runes) == 1 && msg.Runes[0] == 'o' {
					m.margin++
					m.setStatus(fmt.Sprintf("Ecrã encolhido. Margem inferior: %d", m.margin), 0)
					m.adjustCamera()
					return m, nil
				}
				if len(msg.Runes) == 1 && msg.Runes[0] == 'p' {
					if m.margin > 1 {
						m.margin--
						m.setStatus(fmt.Sprintf("Ecrã expandido. Margem inferior: %d", m.margin), 0)
					} else {
						m.setStatus("Margem inferior já está no mínimo.", 0)
					}
					m.adjustCamera()
					return m, nil
				}
				// Removido o 'return m, nil' que bloqueava o resto do teclado aqui!
			}

			cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
			if cleanStr == "" {
				return m, nil
			}

			if m.buf.HasSelection() {
				m.buf.DeleteSelectedText()
			}
			m.buf.InsertString(cleanStr)
			m.adjustCamera()
		}
	}
	return m, nil
}
