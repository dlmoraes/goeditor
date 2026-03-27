package editor

import (
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

	acPrefix      string
	acSuggestions []string
	acIndex       int
	acStartCol    int
	acOriginalX   int

	statusMsg     string
	statusMsgType int // 0 = Info, 1 = Sucesso, 2 = Erro

	clipboard  string
	autoIndent bool // NOVO: Controle para o "Paste Mode"
}

func New(buf *buffer.Buffer) Model {
	return Model{
		buf:        buf,
		margin:     6,
		autoIndent: false, // Auto-indentação ligada por padrão!
	}
}

func (m *Model) setStatus(msg string, sType int) {
	m.statusMsg = msg
	m.statusMsgType = sType
}

func (m Model) Init() tea.Cmd {
	return nil
}

// Exportações para o App Mestre (Abas)
func (m Model) GetFilename() string {
	return m.buf.Filename
}

func (m Model) IsModified() bool {
	return m.buf.Modified
}

// NOVO: Função para o App forçar o editor a saltar para uma linha (usado pelo Finder)
func (m *Model) JumpToLine(line int) {
	if line < 0 {
		line = 0
	}
	if line >= len(m.buf.Lines) {
		line = len(m.buf.Lines) - 1
	}
	m.buf.CursorY = line
	m.buf.CursorX = 0
	m.centerCamera(line)
}
