package editor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sb strings.Builder

	safeWidth := m.width - 1
	if safeWidth < 10 {
		safeWidth = 10
	}

	modMark := ""
	if m.buf.Modified {
		modMark = " *"
	}
	name := m.buf.Filename
	if name == "" {
		name = "[No Name]"
	}
	gutterW := m.gutterWidth()
	textW := safeWidth - gutterW
	if textW < 10 {
		textW = 10
	}

	col := m.buf.CursorX + 1
	header := fmt.Sprintf(" GoEdit  %s%s   Ln %d/%d  Col %d",
		name, modMark, m.buf.CursorY+1, len(m.buf.Lines), col)
	sb.WriteString(padRight(header, safeWidth) + "\n")
	sb.WriteString(padRight("", safeWidth) + "\n")
	sb.WriteString(strings.Repeat("-", safeWidth) + "\n")

	viewH := m.viewHeight()
	lexer := detectLexer(m.buf.Filename)
	totalLines := len(m.buf.Lines)

	for i := 0; i < viewH; i++ {
		lineIdx := m.cameraTop + i
		if lineIdx >= totalLines {
			sb.WriteString(strings.Repeat(" ", gutterW))
			sb.WriteString("~\n")
			continue
		}

		lineNum := fmt.Sprintf("%*d ", gutterW-1, lineIdx+1)
		sb.WriteString(lineNum)

		raw := m.buf.Lines[lineIdx]

		if m.buf.HasSelection() {
			raw = injectSelectionMarks(raw, lineIdx, m.buf)
		}
		if lineIdx == m.buf.CursorY {
			raw = insertCursorMark(raw, m.buf.CursorX)
		}

		raw = strings.ReplaceAll(raw, "\t", "    ")
		raw = padRight(raw, textW)

		sb.WriteString(highlight(raw, lexer, textW) + "\n")
	}

	sb.WriteString(m.renderFooter(safeWidth))
	return sb.String()
}

func (m Model) renderFooter(safeW int) string {
	const inv = "\x1b[7m"
	const rst = "\x1b[0m"

	var l1Rendered, l2Rendered string

	switch m.currentMode {

	case modeAutocomplete:
		var sugList strings.Builder
		sugList.WriteString(fmt.Sprintf(" Autocompletar (%s): ", m.acPrefix))
		for i, s := range m.acSuggestions {
			if i == m.acIndex {
				sugList.WriteString(fmt.Sprintf("[%s] ", s))
			} else {
				sugList.WriteString(fmt.Sprintf("%s ", s))
			}
			if sugList.Len() > safeW-20 {
				sugList.WriteString("...")
				break
			}
		}
		l1Rendered = inv + padRight(sugList.String(), safeW) + rst
		l2Rendered = inv + padRight(" Tab Ciclar Sugestão   Enter Confirmar   Esc Cancelar", safeW) + rst

	case modeCommand:
		l1Rendered = inv + padRight(fmt.Sprintf(" :%s_", m.commandInput), safeW) + rst
		l2Rendered = inv + padRight(" 💡 w (Salvar), q (Sair), wq (Salvar/Sair), clear (Limpar Tudo) | Enter Confirmar", safeW) + rst

	case modeSearch:
		l1Rendered = inv + padRight(fmt.Sprintf(" Search: %s_", m.searchQuery), safeW) + rst
		l2Rendered = inv + padRight(" ^G Cancel   Enter Confirm   ^N Next", safeW) + rst

	case modeReplace:
		find := m.searchQuery
		repl := m.replaceQuery
		if m.replaceField == 0 {
			find += "_"
		} else {
			repl += "_"
		}
		l1Rendered = inv + padRight(fmt.Sprintf(" Find: %-20s  Replace: %s", find, repl), safeW) + rst
		l2Rendered = inv + padRight(" Tab Switch field   Enter Confirm   Esc Cancel   ^A Replace all", safeW) + rst

	case modeGoto:
		l1Rendered = inv + padRight(fmt.Sprintf(" Go to line: %s_", m.gotoInput), safeW) + rst
		l2Rendered = inv + padRight(" Enter Confirm   Esc Cancel", safeW) + rst

	case modeConfirmQuit:
		l1Rendered = inv + padRight(" Tem alterações não guardadas. Guardar antes de fechar a aba?", safeW) + rst
		l2Rendered = inv + padRight(" (S)im e fechar   (N)ão — rejeitar   Qualquer tecla: cancelar", safeW) + rst

	default:
		if m.statusMsg == "" {
			pasteStatus := "ON"
			if !m.autoIndent {
				pasteStatus = "OFF"
			}
			l1Rendered = inv + padRight(fmt.Sprintf(" ^S Save  ^A Tudo  ^X Exit  ^P Cmds  Alt+I Indent [%s]", pasteStatus), safeW) + rst
		} else {
			if m.statusMsgType == 1 {
				style := lipgloss.NewStyle().Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0")).Width(safeW)
				l1Rendered = style.Render(" ✔ " + m.statusMsg)
			} else if m.statusMsgType == 2 {
				style := lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("15")).Width(safeW)
				l1Rendered = style.Render(" ✖ " + m.statusMsg)
			} else {
				l1Rendered = inv + padRight(" "+m.statusMsg, safeW) + rst
			}
		}

		lang := languageName(m.buf.Filename)
		selInfo := ""
		if m.buf.HasSelection() {
			selInfo = fmt.Sprintf("  [SEL %d chars]", len([]rune(m.buf.GetSelectedText())))
		}
		l2Rendered = inv + padRight(fmt.Sprintf(" GoEdit v1.0  |  UTF-8  |  %s%s", lang, selInfo), safeW) + rst
	}

	return l1Rendered + "\n" + l2Rendered
}

func (m Model) viewHeight() int {
	h := m.height - m.margin
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) gutterWidth() int {
	digits := len(strconv.Itoa(len(m.buf.Lines)))
	if digits < 3 {
		digits = 3
	}
	return digits + 1
}

func (m *Model) adjustCamera() {
	vh := m.viewHeight()
	if m.buf.CursorY < m.cameraTop {
		m.cameraTop = m.buf.CursorY
	}
	if m.buf.CursorY >= m.cameraTop+vh {
		m.cameraTop = m.buf.CursorY - vh + 1
	}
	if m.cameraTop < 0 {
		m.cameraTop = 0
	}
}

func (m *Model) centerCamera(lineIdx int) {
	vh := m.viewHeight()
	m.cameraTop = lineIdx - vh/2
	if m.cameraTop < 0 {
		m.cameraTop = 0
	}
}
