package editor

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) executeCommand() tea.Cmd {
	cmd := strings.TrimSpace(m.commandInput)
	switch cmd {
	case "w":
		if err := m.buf.Save(); err != nil {
			m.setStatus("Erro ao guardar: "+err.Error(), 2)
		} else {
			m.setStatus("Ficheiro guardado com sucesso.", 1)
		}
		return nil
	case "wq", "x":
		if err := m.buf.Save(); err != nil {
			m.setStatus("Erro ao guardar: "+err.Error(), 2)
			return nil
		}
		return func() tea.Msg { return CloseTabMsg{} }
	case "q":
		if m.buf.Modified {
			m.setStatus("E37: Ficheiro não guardado (use :q! para forçar o fecho)", 2)
			return nil
		}
		return func() tea.Msg { return CloseTabMsg{} }
	case "q!":
		return func() tea.Msg { return CloseTabMsg{} }
	case "clear":
		m.buf.Lines = []string{""}
		m.buf.CursorX = 0
		m.buf.CursorY = 0
		m.buf.Modified = true
		m.adjustCamera()
		m.setStatus("Ficheiro limpo com sucesso.", 1)
		return nil
	case "paste": // COMANDO PARA O MODO COLAGEM
		m.autoIndent = !m.autoIndent
		if m.autoIndent {
			m.setStatus("Auto-Indentação ATIVADA.", 1)
		} else {
			m.setStatus("Modo Colagem (Auto-Indentação DESATIVADA).", 0)
		}
		return nil
	default:
		m.setStatus("Comando desconhecido: "+cmd, 2)
		return nil
	}
}

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
		m.setStatus(fmt.Sprintf("Não encontrado: %q", m.searchQuery), 0)
		return
	}
	m.searchIdx = 0
	m.jumpToHit(m.searchHits[0])
}

func (m *Model) nextSearchHit() {
	if len(m.searchHits) == 0 {
		m.setStatus("Nenhuma busca ativa. Use ^F para buscar.", 0)
		return
	}
	m.searchIdx = (m.searchIdx + 1) % len(m.searchHits)
	m.jumpToHit(m.searchHits[m.searchIdx])
}

func (m *Model) jumpToHit(h searchHit) {
	m.buf.CursorY = h.line
	m.buf.CursorX = h.col
	m.centerCamera(h.line)
	m.setStatus(fmt.Sprintf("Match %d/%d", m.searchIdx+1, len(m.searchHits)), 1)
}

func (m *Model) executeGoto() {
	if m.gotoInput == "" {
		return
	}
	n, err := strconv.Atoi(m.gotoInput)
	if err != nil {
		m.setStatus("Número de linha inválido.", 2)
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
	m.setStatus(fmt.Sprintf("Saltou para a linha %d.", n+1), 1)
}

// Funções do Autocompletar
func (m *Model) startAutocomplete() {
	line := m.buf.Lines[m.buf.CursorY]
	col := m.buf.CursorX

	start := col
	r := []rune(line)
	for start > 0 && isWordChar(r[start-1]) {
		start--
	}

	if start == col {
		m.setStatus("Não há prefixo para autocompletar aqui.", 2)
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
		m.setStatus("Nenhuma sugestão encontrada para: "+m.acPrefix, 0)
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
