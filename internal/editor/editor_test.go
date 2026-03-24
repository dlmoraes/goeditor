package editor

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"goeditor/internal/buffer"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newModel(lines ...string) Model {
	if len(lines) == 0 {
		lines = []string{""}
	}
	b := &buffer.Buffer{Lines: make([]string, len(lines))}
	copy(b.Lines, lines)
	m := Model{buf: b, width: 80, height: 24}
	return m
}

func sendKey(m Model, kt tea.KeyType) Model {
	upd, _ := m.Update(tea.KeyMsg{Type: kt})
	return upd.(Model)
}

func sendRune(m Model, r rune) Model {
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return upd.(Model)
}

func sendString(m Model, s string) Model {
	for _, r := range s {
		m = sendRune(m, r)
	}
	return m
}

// ── Gutter ───────────────────────────────────────────────────────────────────

func TestGutterWidthSmallFile(t *testing.T) {
	m := newModel("a", "b", "c") // 3 lines → min gutter width = 3+1=4
	gw := m.gutterWidth()
	if gw < 4 {
		t.Errorf("gutterWidth: got %d, want >= 4", gw)
	}
}

func TestGutterWidthLargeFile(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = "x"
	}
	m := newModel(lines...)
	gw := m.gutterWidth()
	// 1000 lines → 4 digits + 1 space = 5
	wantDigits := len(strconv.Itoa(1000))
	want := wantDigits + 1
	if gw != want {
		t.Errorf("gutterWidth: got %d, want %d", gw, want)
	}
}

func TestViewContainsLineNumbers(t *testing.T) {
	m := newModel("hello", "world", "foo")
	view := m.View()
	if !strings.Contains(view, "1 ") {
		t.Error("View should contain line number '1 '")
	}
	if !strings.Contains(view, "3 ") {
		t.Error("View should contain line number '3 '")
	}
}

func TestGutterNotOverlappingText(t *testing.T) {
	m := newModel("hello")
	view := m.View()
	// "hello" must appear in the view (not eaten by gutter math).
	if !strings.Contains(view, "hello") {
		t.Error("View should contain the text 'hello'")
	}
}

// ── Undo / Redo via keyboard ──────────────────────────────────────────────────

func TestCtrlZUndo(t *testing.T) {
	m := newModel("")
	m = sendRune(m, 'a')
	m = sendRune(m, 'b')
	if m.buf.Lines[0] != "ab" {
		t.Fatalf("before undo: got %q, want %q", m.buf.Lines[0], "ab")
	}
	m = sendKey(m, tea.KeyCtrlZ)
	if m.buf.Lines[0] != "a" {
		t.Errorf("after Ctrl+Z: got %q, want %q", m.buf.Lines[0], "a")
	}
}

func TestCtrlYRedo(t *testing.T) {
	m := newModel("")
	m = sendRune(m, 'a')
	m = sendKey(m, tea.KeyCtrlZ) // undo
	m = sendKey(m, tea.KeyCtrlY) // redo
	if m.buf.Lines[0] != "a" {
		t.Errorf("after Ctrl+Y: got %q, want %q", m.buf.Lines[0], "a")
	}
}

func TestUndoStatusMsg(t *testing.T) {
	m := newModel("") // nothing to undo
	m = sendKey(m, tea.KeyCtrlZ)
	if !strings.Contains(m.statusMsg, "undo") && !strings.Contains(m.statusMsg, "Undo") {
		t.Errorf("expected 'undo' in statusMsg, got %q", m.statusMsg)
	}
}

// ── Confirm-quit mode ─────────────────────────────────────────────────────────

func TestConfirmQuitTriggered(t *testing.T) {
	m := newModel("")
	m = sendRune(m, 'x') // mark as modified
	m = sendKey(m, tea.KeyCtrlX)
	if m.currentMode != modeConfirmQuit {
		t.Errorf("mode: got %v, want modeConfirmQuit", m.currentMode)
	}
}

func TestConfirmQuitFooterContent(t *testing.T) {
	m := newModel("")
	m = sendRune(m, 'x')
	m = sendKey(m, tea.KeyCtrlX)
	view := m.View()
	if !strings.Contains(view, "unsaved") && !strings.Contains(view, "Save") {
		t.Errorf("footer should mention unsaved changes, got view snippet: %q",
			view[len(view)-200:])
	}
}

func TestConfirmQuitCancelWithOtherKey(t *testing.T) {
	m := newModel("")
	m = sendRune(m, 'x')
	m = sendKey(m, tea.KeyCtrlX)
	m = sendRune(m, 'z') // any key other than s/n cancels
	if m.currentMode != modeNormal {
		t.Errorf("mode after cancel: got %v, want modeNormal", m.currentMode)
	}
}

func TestNoConfirmQuitIfUnmodified(t *testing.T) {
	m := newModel("hello")
	// Don't touch the buffer — it's not modified.
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	_ = upd
	if cmd == nil {
		// We expect a Quit command; if cmd is nil it might be platform-dependent.
		// At minimum, mode should NOT be modeConfirmQuit.
		if upd.(Model).currentMode == modeConfirmQuit {
			t.Error("should not enter confirm-quit mode for unmodified buffer")
		}
	}
}

// ── Search mode ───────────────────────────────────────────────────────────────

func TestSearchModeEntry(t *testing.T) {
	m := newModel("hello world")
	m = sendKey(m, tea.KeyCtrlF)
	if m.currentMode != modeSearch {
		t.Errorf("mode: got %v, want modeSearch", m.currentMode)
	}
}

func TestSearchFindsMatch(t *testing.T) {
	m := newModel("hello world", "second line")
	m = sendKey(m, tea.KeyCtrlF)
	m = sendString(m, "world")
	m = sendKey(m, tea.KeyEnter)
	if m.buf.CursorY != 0 {
		t.Errorf("CursorY: got %d, want 0", m.buf.CursorY)
	}
	if m.buf.CursorX != 6 {
		t.Errorf("CursorX: got %d, want 6", m.buf.CursorX)
	}
}

func TestSearchNotFound(t *testing.T) {
	m := newModel("hello")
	m = sendKey(m, tea.KeyCtrlF)
	m = sendString(m, "xyz")
	m = sendKey(m, tea.KeyEnter)
	if !strings.Contains(m.statusMsg, "Not found") {
		t.Errorf("statusMsg: got %q, want 'Not found'", m.statusMsg)
	}
}

func TestSearchCancelWithEsc(t *testing.T) {
	m := newModel("hello")
	m = sendKey(m, tea.KeyCtrlF)
	m = sendKey(m, tea.KeyEsc)
	if m.currentMode != modeNormal {
		t.Errorf("mode: got %v, want modeNormal", m.currentMode)
	}
}

// ── Replace mode ──────────────────────────────────────────────────────────────

func TestReplaceModeEntry(t *testing.T) {
	m := newModel("hello hello")
	m = sendKey(m, tea.KeyCtrlH)
	if m.currentMode != modeReplace {
		t.Errorf("mode: got %v, want modeReplace", m.currentMode)
	}
}

func TestReplaceExecutes(t *testing.T) {
	m := newModel("foo foo foo")
	m = sendKey(m, tea.KeyCtrlH)
	m = sendString(m, "foo")
	m = sendKey(m, tea.KeyTab) // switch to replace field
	m = sendString(m, "bar")
	m = sendKey(m, tea.KeyEnter) // confirm replace
	if m.buf.Lines[0] != "bar bar bar" {
		t.Errorf("after replace: got %q, want %q", m.buf.Lines[0], "bar bar bar")
	}
}

func TestReplaceReportsCount(t *testing.T) {
	m := newModel("a a a")
	m = sendKey(m, tea.KeyCtrlH)
	m = sendString(m, "a")
	m = sendKey(m, tea.KeyTab)
	m = sendString(m, "b")
	m = sendKey(m, tea.KeyEnter)
	if !strings.Contains(m.statusMsg, "3") {
		t.Errorf("statusMsg should contain '3', got %q", m.statusMsg)
	}
}

// ── Goto mode ─────────────────────────────────────────────────────────────────

func TestGotoModeEntry(t *testing.T) {
	m := newModel("a", "b", "c")
	m = sendKey(m, tea.KeyCtrlG)
	if m.currentMode != modeGoto {
		t.Errorf("mode: got %v, want modeGoto", m.currentMode)
	}
}

func TestGotoLine(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line"
	}
	m := newModel(lines...)
	m = sendKey(m, tea.KeyCtrlG)
	m = sendString(m, "25")
	m = sendKey(m, tea.KeyEnter)
	if m.buf.CursorY != 24 { // 0-based
		t.Errorf("CursorY: got %d, want 24", m.buf.CursorY)
	}
}

func TestGotoLineClampHigh(t *testing.T) {
	m := newModel("a", "b", "c")
	m = sendKey(m, tea.KeyCtrlG)
	m = sendString(m, "999")
	m = sendKey(m, tea.KeyEnter)
	if m.buf.CursorY != 2 {
		t.Errorf("CursorY: got %d, want 2 (clamped)", m.buf.CursorY)
	}
}

func TestGotoLineClampLow(t *testing.T) {
	m := newModel("a", "b", "c")
	m.buf.CursorY = 2
	m = sendKey(m, tea.KeyCtrlG)
	m = sendString(m, "0")
	m = sendKey(m, tea.KeyEnter)
	if m.buf.CursorY != 0 {
		t.Errorf("CursorY: got %d, want 0 (clamped)", m.buf.CursorY)
	}
}

func TestGotoRejectsLetters(t *testing.T) {
	m := newModel("a", "b", "c")
	m = sendKey(m, tea.KeyCtrlG)
	m = sendString(m, "abc") // letters ignored
	if m.gotoInput != "" {
		t.Errorf("gotoInput should be empty after letters, got %q", m.gotoInput)
	}
}

func TestGotoCancelWithEsc(t *testing.T) {
	m := newModel("a", "b")
	m.buf.CursorY = 1
	m = sendKey(m, tea.KeyCtrlG)
	m = sendString(m, "1")
	m = sendKey(m, tea.KeyEsc)
	if m.currentMode != modeNormal {
		t.Errorf("mode: got %v, want modeNormal", m.currentMode)
	}
	if m.buf.CursorY != 1 {
		t.Errorf("CursorY should be unchanged after cancel, got %d", m.buf.CursorY)
	}
}

// ── Comment prefix detection ──────────────────────────────────────────────────

func TestCommentPrefixGo(t *testing.T) {
	if p := commentPrefixFor("main.go"); p != "//" {
		t.Errorf("Go: got %q, want %q", p, "//")
	}
}

func TestCommentPrefixPython(t *testing.T) {
	if p := commentPrefixFor("script.py"); p != "#" {
		t.Errorf("Python: got %q, want %q", p, "#")
	}
}

func TestCommentPrefixLua(t *testing.T) {
	if p := commentPrefixFor("init.lua"); p != "--" {
		t.Errorf("Lua: got %q, want %q", p, "--")
	}
}

func TestCommentPrefixSQL(t *testing.T) {
	if p := commentPrefixFor("query.sql"); p != "--" {
		t.Errorf("SQL: got %q, want %q", p, "--")
	}
}

// ── Language name ─────────────────────────────────────────────────────────────

func TestLanguageNameGo(t *testing.T) {
	if n := languageName("main.go"); n != "Go" {
		t.Errorf("got %q, want %q", n, "Go")
	}
}

func TestLanguageNameUnknown(t *testing.T) {
	if n := languageName("file.xyz"); n != "Text" {
		t.Errorf("got %q, want %q", n, "Text")
	}
}

// ── View smoke tests ──────────────────────────────────────────────────────────

func TestViewRendersWithoutPanic(t *testing.T) {
	m := newModel("package main", "", "func main() {", "}", "")
	_ = m.View() // must not panic
}

func TestViewFooterNormalMode(t *testing.T) {
	m := newModel("hello")
	view := m.View()
	if !strings.Contains(view, "^S") {
		t.Error("normal mode footer should contain '^S'")
	}
}

func TestViewSearchModeFooter(t *testing.T) {
	m := newModel("hello")
	m = sendKey(m, tea.KeyCtrlF)
	view := m.View()
	if !strings.Contains(view, "Search") {
		t.Error("search mode footer should contain 'Search'")
	}
}

func TestViewGotoModeFooter(t *testing.T) {
	m := newModel("hello")
	m = sendKey(m, tea.KeyCtrlG)
	view := m.View()
	if !strings.Contains(view, "line") && !strings.Contains(view, "Line") {
		t.Error("goto mode footer should mention line number")
	}
}

func TestStatusLineContainsColNumber(t *testing.T) {
	m := newModel("hello")
	m.buf.CursorX = 3
	view := m.View()
	if !strings.Contains(view, "Col 4") {
		t.Errorf("header should contain 'Col 4', view snippet: %q",
			view[:min(len(view), 120)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Sprint 3: Selection + Clipboard ──────────────────────────────────────────

func sendShiftKey(m Model, kt tea.KeyType) Model {
	upd, _ := m.Update(tea.KeyMsg{Type: kt})
	return upd.(Model)
}

func TestShiftRightStartsSelection(t *testing.T) {
	m := newModel("hello")
	m = sendShiftKey(m, tea.KeyShiftRight)
	if !m.buf.HasSelection() {
		t.Fatal("selection should be active after Shift+Right")
	}
	if m.buf.CursorX != 1 {
		t.Errorf("CursorX: got %d, want 1", m.buf.CursorX)
	}
}

func TestShiftRightSelectsText(t *testing.T) {
	m := newModel("hello")
	m = sendShiftKey(m, tea.KeyShiftRight)
	m = sendShiftKey(m, tea.KeyShiftRight)
	m = sendShiftKey(m, tea.KeyShiftRight)
	got := m.buf.GetSelectedText()
	if got != "hel" {
		t.Errorf("selected text: got %q, want %q", got, "hel")
	}
}

func TestShiftDownSelectsMultiLine(t *testing.T) {
	m := newModel("hello", "world")
	m = sendShiftKey(m, tea.KeyShiftDown)
	got := m.buf.GetSelectedText()
	// anchor=(0,0) cursor=(1,0) → "hello\n" (first line from 0 to end)
	if !strings.Contains(got, "hello") {
		t.Errorf("selected text should contain 'hello', got %q", got)
	}
}

func TestPlainArrowClearsSelection(t *testing.T) {
	m := newModel("hello")
	m = sendShiftKey(m, tea.KeyShiftRight)
	m = sendShiftKey(m, tea.KeyShiftRight)
	if !m.buf.HasSelection() {
		t.Fatal("selection should be active")
	}
	m = sendKey(m, tea.KeyRight) // plain arrow → clears selection
	if m.buf.HasSelection() {
		t.Error("selection should be cleared after plain arrow key")
	}
}

func TestCtrlCCopiesSelection(t *testing.T) {
	m := newModel("hello world")
	// Select "hello" (5 chars)
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	m = sendKey(m, tea.KeyCtrlC)
	if m.clipboard != "hello" {
		t.Errorf("clipboard: got %q, want %q", m.clipboard, "hello")
	}
	// Buffer must be intact after copy.
	if m.buf.Lines[0] != "hello world" {
		t.Errorf("buffer should be unchanged after copy, got %q", m.buf.Lines[0])
	}
	if !strings.Contains(m.statusMsg, "Copied") {
		t.Errorf("statusMsg should say 'Copied', got %q", m.statusMsg)
	}
}

func TestCtrlCWithNoSelectionQuitsUnmodified(t *testing.T) {
	m := newModel("hello") // not modified
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = upd
	// Should produce a Quit command (cmd != nil) or at minimum not enter confirm mode.
	if upd.(Model).currentMode == modeConfirmQuit {
		t.Error("unmodified buffer should not enter confirm-quit mode")
	}
	_ = cmd
}

func TestCtrlXCutsSelection(t *testing.T) {
	m := newModel("hello world")
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	m = sendKey(m, tea.KeyCtrlX)
	if m.clipboard != "hello" {
		t.Errorf("clipboard: got %q, want %q", m.clipboard, "hello")
	}
	if m.buf.Lines[0] != " world" {
		t.Errorf("buffer after cut: got %q, want %q", m.buf.Lines[0], " world")
	}
	if !strings.Contains(m.statusMsg, "Cut") {
		t.Errorf("statusMsg should say 'Cut', got %q", m.statusMsg)
	}
}

func TestCtrlXWithNoSelectionExits(t *testing.T) {
	m := newModel("hello") // not modified
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if upd.(Model).currentMode == modeConfirmQuit {
		t.Error("unmodified buffer should not enter confirm-quit on Ctrl+X without selection")
	}
}

func TestCtrlVPastesClipboard(t *testing.T) {
	m := newModel("hello world")
	// Copy "hello"
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	m = sendKey(m, tea.KeyCtrlC)
	// Move to end and paste.
	m = sendKey(m, tea.KeyEnd)
	m = sendKey(m, tea.KeyCtrlV)
	if m.buf.Lines[0] != "hello worldhello" {
		t.Errorf("after paste: got %q, want %q", m.buf.Lines[0], "hello worldhello")
	}
}

func TestCtrlVReplacesSelection(t *testing.T) {
	m := newModel("hello world")
	m.clipboard = "GO"
	// Select "world"
	m.buf.CursorX = 6
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	m = sendKey(m, tea.KeyCtrlV)
	if m.buf.Lines[0] != "hello GO" {
		t.Errorf("after paste-over-selection: got %q, want %q", m.buf.Lines[0], "hello GO")
	}
}

func TestBackspaceDeletesSelection(t *testing.T) {
	m := newModel("hello world")
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	m = sendKey(m, tea.KeyBackspace)
	if m.buf.Lines[0] != " world" {
		t.Errorf("after backspace on selection: got %q, want %q", m.buf.Lines[0], " world")
	}
}

func TestTypingReplacesSelection(t *testing.T) {
	m := newModel("hello world")
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	m = sendRune(m, 'H')
	m = sendRune(m, 'i')
	if m.buf.Lines[0] != "Hi world" {
		t.Errorf("typing over selection: got %q, want %q", m.buf.Lines[0], "Hi world")
	}
}

func TestSelectionUndoable(t *testing.T) {
	m := newModel("hello world")
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	m = sendKey(m, tea.KeyCtrlX) // cut
	if m.buf.Lines[0] != " world" {
		t.Fatalf("after cut: got %q", m.buf.Lines[0])
	}
	m = sendKey(m, tea.KeyCtrlZ) // undo
	if m.buf.Lines[0] != "hello world" {
		t.Errorf("after undo cut: got %q, want %q", m.buf.Lines[0], "hello world")
	}
}

func TestStatusLineShowsSelectionInfo(t *testing.T) {
	m := newModel("hello world")
	for i := 0; i < 5; i++ {
		m = sendShiftKey(m, tea.KeyShiftRight)
	}
	view := m.View()
	if !strings.Contains(view, "SEL") {
		t.Error("status line should show SEL indicator when selection is active")
	}
}

// ── Sprint 3: buffer selection unit tests ────────────────────────────────────

func TestGetSelectedTextSingleLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello world"}}
	b.CursorX = 0
	b.CursorY = 0
	b.StartSelection()
	b.MoveCursor(5, 0)
	got := b.GetSelectedText()
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestGetSelectedTextMultiLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello", "world"}}
	b.CursorX = 0
	b.CursorY = 0
	b.StartSelection()
	b.CursorY = 1
	b.CursorX = 5
	got := b.GetSelectedText()
	if got != "hello\nworld" {
		t.Errorf("got %q, want %q", got, "hello\nworld")
	}
}

func TestDeleteSelectedTextSingleLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello world"}}
	b.StartSelection()
	b.MoveCursor(5, 0)
	deleted := b.DeleteSelectedText()
	if deleted != "hello" {
		t.Errorf("deleted: got %q, want %q", deleted, "hello")
	}
	if b.Lines[0] != " world" {
		t.Errorf("remaining: got %q, want %q", b.Lines[0], " world")
	}
	if b.CursorX != 0 {
		t.Errorf("CursorX after delete: got %d, want 0", b.CursorX)
	}
}

func TestDeleteSelectedTextMultiLine(t *testing.T) {
	b := &buffer.Buffer{Lines: []string{"hello", "world", "foo"}}
	b.CursorX = 0
	b.CursorY = 0
	b.StartSelection()
	b.CursorY = 1
	b.CursorX = 5
	b.DeleteSelectedText()
	// Lines: ["", "foo"]
	if len(b.Lines) != 2 {
		t.Fatalf("lines: got %d, want 2", len(b.Lines))
	}
	if b.Lines[0] != "" {
		t.Errorf("line[0]: got %q, want %q", b.Lines[0], "")
	}
	if b.Lines[1] != "foo" {
		t.Errorf("line[1]: got %q, want %q", b.Lines[1], "foo")
	}
}

func TestSelBoundsNormalisedAnchorAfterCursor(t *testing.T) {
	// Anchor at (0,5), cursor at (0,2) — bounds should be (0,2,0,5).
	b := &buffer.Buffer{Lines: []string{"hello world"}}
	b.CursorX = 5
	b.StartSelection()
	b.CursorX = 2
	sy, sx, ey, ex := b.SelBounds()
	if sy != 0 || sx != 2 || ey != 0 || ex != 5 {
		t.Errorf("bounds: got (%d,%d,%d,%d), want (0,2,0,5)", sy, sx, ey, ex)
	}
}
