package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tea "github.com/charmbracelet/bubbletea"

	"goeditor/internal/buffer"
)

// ── Mensagens Exportadas ──────────────────────────────────────────────────────
type CloseTabMsg struct{}

// ── Modes ─────────────────────────────────────────────────────────────────────

type mode int

const (
	modeNormal mode = iota
	modeSearch
	modeReplace
	modeGoto
	modeConfirmQuit
	modeCommand
	modeAutocomplete
)

// ── Search hit ────────────────────────────────────────────────────────────────

type searchHit struct {
	line int
	col  int
}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	buf    *buffer.Buffer
	width  int
	height int
	margin int

	cameraTop int

	currentMode mode

	searchQuery  string
	replaceQuery string
	replaceField int
	searchHits   []searchHit
	searchIdx    int

	gotoInput    string
	commandInput string

	// DADOS DO AUTOCOMPLETAR
	acPrefix      string
	acSuggestions []string
	acIndex       int
	acStartCol    int
	acOriginalX   int

	statusMsg string
	clipboard string
}

func New(buf *buffer.Buffer) Model {
	return Model{
		buf:    buf,
		margin: 28,
	}
}

// ── Bubble Tea ────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.adjustCamera()
		return m, tea.ClearScreen

	case tea.KeyMsg:
		// ── Bracketed paste (Protegido contra NUL bytes) ──
		if msg.Paste {
			text := strings.ReplaceAll(string(msg.Runes), "\r", "")
			text = strings.ReplaceAll(text, "\x00", "") // FILTRO ANTI-NUL

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
				m.statusMsg = "Cancelado."
			}
			return m, nil
		}

		// ── Modo de Autocompletar ──
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
				if !msg.Alt {
					cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
					m.commandInput += cleanStr
				}
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
				if !msg.Alt {
					cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
					m.searchQuery += cleanStr
				}
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
					m.statusMsg = fmt.Sprintf("Substituídas %d ocorrência(s).", n)
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
				if !msg.Alt {
					cleanStr := strings.ReplaceAll(string(msg.Runes), "\x00", "")
					if m.replaceField == 0 {
						m.searchQuery += cleanStr
					} else {
						m.replaceQuery += cleanStr
					}
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
				if !msg.Alt {
					for _, ch := range string(msg.Runes) {
						if ch >= '0' && ch <= '9' {
							m.gotoInput += string(ch)
						}
					}
				}
			}
			return m, nil
		}

		m.statusMsg = ""
		switch msg.Type {

		case tea.KeyCtrlAt:
			m.startAutocomplete()
			return m, nil

		case tea.KeyCtrlX:
			if m.buf.HasSelection() {
				m.clipboard = m.buf.DeleteSelectedText()
				m.statusMsg = fmt.Sprintf("Cortados %d chars.", len([]rune(m.clipboard)))
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
				m.statusMsg = fmt.Sprintf("Copiados %d chars.", len([]rune(m.clipboard)))
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
				m.statusMsg = "Erro ao guardar: " + err.Error()
			} else {
				m.statusMsg = "Ficheiro guardado."
			}

		case tea.KeyCtrlZ:
			if !m.buf.Undo() {
				m.statusMsg = "Nada para anular."
			}
			m.adjustCamera()
		case tea.KeyCtrlY:
			if !m.buf.Redo() {
				m.statusMsg = "Nada para refazer."
			}
			m.adjustCamera()

		case tea.KeyCtrlK:
			m.buf.ClearSelection()
			m.buf.DeleteCurrentLine()
			m.adjustCamera()
		case tea.KeyCtrlD:
			m.buf.ClearSelection()
			m.buf.DuplicateCurrentLine()
			m.adjustCamera()

		case tea.KeyCtrlP:
			m.currentMode = modeCommand
			m.commandInput = ""
			return m, nil

		case tea.KeyCtrlV:
			if m.buf.HasSelection() {
				m.buf.DeleteSelectedText()
			}
			m.buf.InsertString(m.clipboard)
			m.adjustCamera()

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
			if msg.Alt {
				if len(msg.Runes) == 1 && msg.Runes[0] == 'o' {
					m.margin++
					m.statusMsg = fmt.Sprintf("Ecrã encolhido. Margem inferior: %d", m.margin)
					m.adjustCamera()
					return m, nil
				}
				if len(msg.Runes) == 1 && msg.Runes[0] == 'p' {
					if m.margin > 1 {
						m.margin--
						m.statusMsg = fmt.Sprintf("Ecrã expandido. Margem inferior: %d", m.margin)
					} else {
						m.statusMsg = "Margem inferior já está no mínimo."
					}
					m.adjustCamera()
					return m, nil
				}
				return m, nil
			}

			// FILTRO DE SEGURANÇA MÁXIMA PARA O TEXTO INSERIDO
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

// ── Funções de Autocompletar ──────────────────────────────────────────────────

func isWordChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func extractWords(line string) []string {
	var words []string
	var current strings.Builder
	for _, c := range line {
		if isWordChar(c) {
			current.WriteRune(c)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

func (m *Model) startAutocomplete() {
	line := m.buf.Lines[m.buf.CursorY]
	col := m.buf.CursorX

	start := col
	r := []rune(line)
	for start > 0 && isWordChar(r[start-1]) {
		start--
	}

	if start == col {
		m.statusMsg = "Não há prefixo para autocompletar aqui."
		return
	}

	m.acPrefix = string(r[start:col])
	m.acStartCol = start
	m.acOriginalX = col

	seen := make(map[string]bool)
	m.acSuggestions = nil
	for _, l := range m.buf.Lines {
		words := extractWords(l)
		for _, w := range words {
			if strings.HasPrefix(w, m.acPrefix) && w != m.acPrefix && !seen[w] {
				seen[w] = true
				m.acSuggestions = append(m.acSuggestions, w)
			}
		}
	}

	if len(m.acSuggestions) == 0 {
		m.statusMsg = "Nenhuma sugestão encontrada para: " + m.acPrefix
		return
	}

	m.currentMode = modeAutocomplete
	m.acIndex = 0
	m.applyAutocomplete()
}

func (m *Model) applyAutocomplete() {
	line := []rune(m.buf.Lines[m.buf.CursorY])
	suggestion := []rune(m.acSuggestions[m.acIndex])

	newLen := m.acStartCol + len(suggestion) + len(line) - m.buf.CursorX
	out := make([]rune, 0, newLen)
	out = append(out, line[:m.acStartCol]...)
	out = append(out, suggestion...)
	out = append(out, line[m.buf.CursorX:]...)

	m.buf.Lines[m.buf.CursorY] = string(out)
	m.buf.CursorX = m.acStartCol + len(suggestion)
	m.buf.Modified = true
}

func (m *Model) cancelAutocomplete() {
	line := []rune(m.buf.Lines[m.buf.CursorY])
	prefix := []rune(m.acPrefix)

	out := make([]rune, 0)
	out = append(out, line[:m.acStartCol]...)
	out = append(out, prefix...)
	out = append(out, line[m.buf.CursorX:]...)

	m.buf.Lines[m.buf.CursorY] = string(out)
	m.buf.CursorX = m.acOriginalX
	m.currentMode = modeNormal
}

// ── View ─────────────────────────────────────────────────────────────────────

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

// ── Footer ───────────────────────────────────────────────────────────────────

func (m Model) renderFooter(safeW int) string {
	const inv = "\x1b[7m"
	const rst = "\x1b[0m"

	var l1, l2 string

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
		l1 = padRight(sugList.String(), safeW)
		l2 = padRight(" Tab Ciclar Sugestão   Enter Confirmar   Esc Cancelar", safeW)

	case modeCommand:
		l1 = padRight(fmt.Sprintf(" :%s_", m.commandInput), safeW)
		l2 = padRight(" Enter Executar   Esc Cancelar", safeW)

	case modeSearch:
		l1 = padRight(fmt.Sprintf(" Search: %s_", m.searchQuery), safeW)
		l2 = padRight(" ^G Cancel   Enter Confirm   ^N Next", safeW)

	case modeReplace:
		find := m.searchQuery
		repl := m.replaceQuery
		if m.replaceField == 0 {
			find += "_"
		} else {
			repl += "_"
		}
		l1 = padRight(fmt.Sprintf(" Find: %-20s  Replace: %s", find, repl), safeW)
		l2 = padRight(" Tab Switch field   Enter Confirm   Esc Cancel   ^A Replace all", safeW)

	case modeGoto:
		l1 = padRight(fmt.Sprintf(" Go to line: %s_", m.gotoInput), safeW)
		l2 = padRight(" Enter Confirm   Esc Cancel", safeW)

	case modeConfirmQuit:
		l1 = padRight(" Tem alterações não guardadas. Guardar antes de fechar a aba?", safeW)
		l2 = padRight(" (S)im e fechar   (N)ão — rejeitar   Qualquer tecla: cancelar", safeW)

	default:
		msg := m.statusMsg
		if msg == "" {
			msg = " ^S Save  ^X Exit  ^P Comandos  Tab Autocompletar"
		}
		l1 = padRight(msg, safeW)
		lang := languageName(m.buf.Filename)
		selInfo := ""
		if m.buf.HasSelection() {
			selInfo = fmt.Sprintf("  [SEL %d chars]", len([]rune(m.buf.GetSelectedText())))
		}
		l2 = padRight(fmt.Sprintf(" GoEdit v1.0  |  UTF-8  |  %s%s", lang, selInfo), safeW)
	}

	return inv + l1 + rst + "\n" + inv + l2 + rst
}

// ── Comandos do Neovim ────────────────────────────────────────────────────────

func (m *Model) executeCommand() tea.Cmd {
	cmd := strings.TrimSpace(m.commandInput)
	switch cmd {
	case "w":
		if err := m.buf.Save(); err != nil {
			m.statusMsg = "Erro ao guardar: " + err.Error()
		} else {
			m.statusMsg = "Ficheiro guardado com sucesso."
		}
		return nil
	case "wq", "x":
		if err := m.buf.Save(); err != nil {
			m.statusMsg = "Erro ao guardar: " + err.Error()
			return nil
		}
		return func() tea.Msg { return CloseTabMsg{} }
	case "q":
		if m.buf.Modified {
			m.statusMsg = "E37: Não guardado (use :q! para forçar)"
			return nil
		}
		return func() tea.Msg { return CloseTabMsg{} }
	case "q!":
		return func() tea.Msg { return CloseTabMsg{} }
	default:
		m.statusMsg = "Comando desconhecido: " + cmd
		return nil
	}
}

// ── Camera / Viewport ─────────────────────────────────────────────────────────

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

// ── Search ───────────────────────────────────────────────────────────────────

func (m *Model) executeSearch() {
	if m.searchQuery == "" {
		return
	}
	q := strings.ToLower(m.searchQuery)
	m.searchHits = nil

	for i, line := range m.buf.Lines {
		lower := strings.ToLower(line)
		off := 0
		for {
			idx := strings.Index(lower[off:], q)
			if idx < 0 {
				break
			}
			m.searchHits = append(m.searchHits, searchHit{line: i, col: off + idx})
			off += idx + 1
		}
	}

	if len(m.searchHits) == 0 {
		m.statusMsg = fmt.Sprintf("Not found: %q", m.searchQuery)
		return
	}
	m.searchIdx = 0
	m.jumpToHit(m.searchHits[0])
}

func (m *Model) nextSearchHit() {
	if len(m.searchHits) == 0 {
		m.statusMsg = "No active search. Use ^F to search."
		return
	}
	m.searchIdx = (m.searchIdx + 1) % len(m.searchHits)
	m.jumpToHit(m.searchHits[m.searchIdx])
}

func (m *Model) jumpToHit(h searchHit) {
	m.buf.CursorY = h.line
	m.buf.CursorX = h.col
	m.centerCamera(h.line)
	m.statusMsg = fmt.Sprintf("Match %d/%d", m.searchIdx+1, len(m.searchHits))
}

// ── Goto ──────────────────────────────────────────────────────────────────────

func (m *Model) executeGoto() {
	if m.gotoInput == "" {
		return
	}
	n, err := strconv.Atoi(m.gotoInput)
	if err != nil {
		m.statusMsg = "Invalid line number."
		return
	}
	n--
	if n < 0 {
		n = 0
	}
	if n >= len(m.buf.Lines) {
		n = len(m.buf.Lines) - 1
	}
	m.buf.CursorY = n
	m.buf.CursorX = 0
	m.centerCamera(n)
	m.statusMsg = fmt.Sprintf("Jumped to line %d.", n+1)
}

// ── Syntax highlighting ───────────────────────────────────────────────────────

const cursorPlaceholder = "\U000F0001"
const selStartPlaceholder = "\U000F0002"
const selEndPlaceholder = "\U000F0003"

func insertCursorMark(line string, cursorX int) string {
	r := []rune(line)
	if cursorX > len(r) {
		cursorX = len(r)
	}
	out := make([]rune, 0, len(r)+1)
	out = append(out, r[:cursorX]...)
	out = append(out, []rune(cursorPlaceholder)...)
	out = append(out, r[cursorX:]...)
	return string(out)
}

func injectSelectionMarks(line string, lineIdx int, buf *buffer.Buffer) string {
	if !buf.HasSelection() {
		return line
	}
	sy, sx, ey, ex := buf.SelBounds()
	r := []rune(line)
	lineLen := len(r)

	var start, end int
	switch {
	case lineIdx < sy || lineIdx > ey:
		return line
	case lineIdx == sy && lineIdx == ey:
		start, end = sx, ex
	case lineIdx == sy:
		start, end = sx, lineLen
	case lineIdx == ey:
		start, end = 0, ex
	default:
		start, end = 0, lineLen
	}

	if start < 0 {
		start = 0
	}
	if end > lineLen {
		end = lineLen
	}
	if start >= end {
		return line
	}

	out := make([]rune, 0, lineLen+4)
	out = append(out, r[:start]...)
	out = append(out, []rune(selStartPlaceholder)...)
	out = append(out, r[start:end]...)
	out = append(out, []rune(selEndPlaceholder)...)
	out = append(out, r[end:]...)
	return string(out)
}

func highlight(line string, lexer chroma.Lexer, _ int) string {
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}
	it, err := lexer.Tokenise(nil, line)
	if err != nil {
		return strings.ReplaceAll(line, cursorPlaceholder, "|")
	}
	var buf strings.Builder
	if err := formatter.Format(&buf, style, it); err != nil {
		return strings.ReplaceAll(line, cursorPlaceholder, "|")
	}
	result := strings.TrimRight(buf.String(), "\n")
	result = strings.ReplaceAll(result, cursorPlaceholder, "\x1b[7m|\x1b[27m")
	result = strings.ReplaceAll(result, selStartPlaceholder, "\x1b[4;48;5;237m")
	result = strings.ReplaceAll(result, selEndPlaceholder, "\x1b[0m")
	return result
}

func detectLexer(filename string) chroma.Lexer {
	if filename != "" {
		if l := lexers.Match(filepath.Base(filename)); l != nil {
			return chroma.Coalesce(l)
		}
	}
	return chroma.Coalesce(lexers.Fallback)
}

// ── Language helpers ──────────────────────────────────────────────────────────

func commentPrefixFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".go", ".c", ".cpp", ".cc", ".h", ".hpp", ".java", ".js", ".ts", ".rs", ".swift", ".kt":
		return "//"
	case ".py", ".sh", ".rb", ".pl", ".r", ".yml", ".yaml", ".toml":
		return "#"
	case ".lua":
		return "--"
	case ".sql":
		return "--"
	case ".hs":
		return "--"
	case ".html", ".xml":
		return ""
	default:
		return "//"
	}
}

func languageName(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".rs":
		return "Rust"
	case ".c", ".h":
		return "C"
	case ".cpp", ".cc", ".hpp":
		return "C++"
	case ".java":
		return "Java"
	case ".lua":
		return "Lua"
	case ".sh":
		return "Shell"
	case ".md":
		return "Markdown"
	case ".toml":
		return "TOML"
	case ".yaml", ".yml":
		return "YAML"
	case ".json":
		return "JSON"
	case ".sql":
		return "SQL"
	case ".html":
		return "HTML"
	case ".css":
		return "CSS"
	default:
		return "Text"
	}
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func padRight(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		if width <= 0 {
			return ""
		}
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func runeBackspace(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

// ── Exportações para o App Mestre (Abas) ──────────────────────────────────────

func (m Model) GetFilename() string {
	return m.buf.Filename
}

func (m Model) IsModified() bool {
	return m.buf.Modified
}
