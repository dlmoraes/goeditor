package buffer

import (
	"os"
	"strings"
)

// ── Snapshot ──────────────────────────────────────────────────────────────────

// snapshot is an immutable copy of the buffer state stored in the undo history.
type snapshot struct {
	lines   []string
	cursorX int
	cursorY int
}

func (b *Buffer) makeSnapshot() snapshot {
	cp := make([]string, len(b.Lines))
	copy(cp, b.Lines)
	return snapshot{lines: cp, cursorX: b.CursorX, cursorY: b.CursorY}
}

// ── Buffer ────────────────────────────────────────────────────────────────────

// Buffer holds the in-memory state of the open file.
type Buffer struct {
	Lines    []string
	CursorX  int
	CursorY  int
	Filename string
	Modified bool

	past   []snapshot // undo stack
	future []snapshot // redo stack

	Sel Selection // active text selection
}

const maxHistory = 200

// New creates a Buffer. If filename exists on disk it is read; otherwise a
// single empty line is prepared.
func New(filename string) (*Buffer, error) {
	b := &Buffer{
		Filename: filename,
		Lines:    []string{""},
	}
	if filename == "" {
		return b, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return b, nil
		}
		return nil, err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	content = strings.TrimRight(content, "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	b.Lines = lines
	return b, nil
}

// ── History ───────────────────────────────────────────────────────────────────

// pushHistory saves current state before a mutating operation and clears redo.
func (b *Buffer) pushHistory() {
	b.past = append(b.past, b.makeSnapshot())
	if len(b.past) > maxHistory {
		b.past = b.past[len(b.past)-maxHistory:]
	}
	b.future = b.future[:0]
}

// Undo reverts to the previous snapshot. Returns true if an undo was performed.
func (b *Buffer) Undo() bool {
	if len(b.past) == 0 {
		return false
	}
	b.future = append(b.future, b.makeSnapshot())
	snap := b.past[len(b.past)-1]
	b.past = b.past[:len(b.past)-1]
	b.applySnapshot(snap)
	return true
}

// Redo re-applies the most recently undone snapshot. Returns true if performed.
func (b *Buffer) Redo() bool {
	if len(b.future) == 0 {
		return false
	}
	b.past = append(b.past, b.makeSnapshot())
	snap := b.future[len(b.future)-1]
	b.future = b.future[:len(b.future)-1]
	b.applySnapshot(snap)
	return true
}

func (b *Buffer) applySnapshot(s snapshot) {
	b.Lines = make([]string, len(s.lines))
	copy(b.Lines, s.lines)
	b.CursorX = s.cursorX
	b.CursorY = s.cursorY
	b.Modified = true
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (b *Buffer) clamp() {
	if b.CursorY < 0 {
		b.CursorY = 0
	}
	if b.CursorY >= len(b.Lines) {
		b.CursorY = len(b.Lines) - 1
	}
	lineLen := len([]rune(b.Lines[b.CursorY]))
	if b.CursorX < 0 {
		b.CursorX = 0
	}
	if b.CursorX > lineLen {
		b.CursorX = lineLen
	}
}

func (b *Buffer) currentRunes() []rune {
	return []rune(b.Lines[b.CursorY])
}

// leadingWhitespace returns the leading spaces/tabs of line.
func leadingWhitespace(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return line // whole line is whitespace
}

// ── Raw (no-history) internal operations ─────────────────────────────────────

func (b *Buffer) insertCharRaw(ch rune) {
	b.clamp()
	r := b.currentRunes()
	n := make([]rune, 0, len(r)+1)
	n = append(n, r[:b.CursorX]...)
	n = append(n, ch)
	n = append(n, r[b.CursorX:]...)
	b.Lines[b.CursorY] = string(n)
	b.CursorX++
	b.Modified = true
}

func (b *Buffer) insertLineRaw() {
	b.clamp()
	r := b.currentRunes()
	before := string(r[:b.CursorX])
	after := string(r[b.CursorX:])
	indent := leadingWhitespace(b.Lines[b.CursorY])
	b.Lines[b.CursorY] = before
	newLines := make([]string, 0, len(b.Lines)+1)
	newLines = append(newLines, b.Lines[:b.CursorY+1]...)
	newLines = append(newLines, indent+after)
	newLines = append(newLines, b.Lines[b.CursorY+1:]...)
	b.Lines = newLines
	b.CursorY++
	b.CursorX = len([]rune(indent))
	b.Modified = true
}

func (b *Buffer) deleteCharRaw() {
	b.clamp()
	if b.CursorX > 0 {
		r := b.currentRunes()
		n := make([]rune, 0, len(r)-1)
		n = append(n, r[:b.CursorX-1]...)
		n = append(n, r[b.CursorX:]...)
		b.Lines[b.CursorY] = string(n)
		b.CursorX--
		b.Modified = true
		return
	}
	if b.CursorY == 0 {
		return
	}
	prevLen := len([]rune(b.Lines[b.CursorY-1]))
	b.Lines[b.CursorY-1] += b.Lines[b.CursorY]
	newLines := make([]string, 0, len(b.Lines)-1)
	newLines = append(newLines, b.Lines[:b.CursorY]...)
	newLines = append(newLines, b.Lines[b.CursorY+1:]...)
	b.Lines = newLines
	b.CursorY--
	b.CursorX = prevLen
	b.Modified = true
}

// ── Public mutation API ───────────────────────────────────────────────────────

// InsertChar inserts a single rune at the cursor (one undo step).
func (b *Buffer) InsertChar(ch rune) {
	b.pushHistory()
	b.insertCharRaw(ch)
}

// InsertString inserts multiple runes as a single undoable operation.
func (b *Buffer) InsertString(s string) {
	if s == "" {
		return
	}
	b.pushHistory()
	for _, ch := range s {
		b.insertCharRaw(ch)
	}
}

// InsertLine splits the line at the cursor (Enter). Applies auto-indent.
func (b *Buffer) InsertLine() {
	b.pushHistory()
	b.insertLineRaw()
}

// DeleteChar handles Backspace.
func (b *Buffer) DeleteChar() {
	b.pushHistory()
	b.deleteCharRaw()
}

// DeleteCurrentLine removes the line under the cursor (Ctrl+K).
func (b *Buffer) DeleteCurrentLine() {
	b.pushHistory()
	b.clamp()
	if len(b.Lines) == 1 {
		b.Lines[0] = ""
		b.CursorX = 0
		b.Modified = true
		return
	}
	newLines := make([]string, 0, len(b.Lines)-1)
	newLines = append(newLines, b.Lines[:b.CursorY]...)
	newLines = append(newLines, b.Lines[b.CursorY+1:]...)
	b.Lines = newLines
	b.Modified = true
	b.clamp()
}

// DuplicateCurrentLine inserts a copy of the current line below (Ctrl+D).
func (b *Buffer) DuplicateCurrentLine() {
	b.pushHistory()
	b.clamp()
	dup := b.Lines[b.CursorY]
	newLines := make([]string, 0, len(b.Lines)+1)
	newLines = append(newLines, b.Lines[:b.CursorY+1]...)
	newLines = append(newLines, dup)
	newLines = append(newLines, b.Lines[b.CursorY+1:]...)
	b.Lines = newLines
	b.CursorY++
	b.Modified = true
}

// SwapLineUp swaps the current line with the one above (Alt+↑).
func (b *Buffer) SwapLineUp() {
	b.clamp()
	if b.CursorY == 0 {
		return
	}
	b.pushHistory()
	b.Lines[b.CursorY], b.Lines[b.CursorY-1] = b.Lines[b.CursorY-1], b.Lines[b.CursorY]
	b.CursorY--
	b.Modified = true
}

// SwapLineDown swaps the current line with the one below (Alt+↓).
func (b *Buffer) SwapLineDown() {
	b.clamp()
	if b.CursorY >= len(b.Lines)-1 {
		return
	}
	b.pushHistory()
	b.Lines[b.CursorY], b.Lines[b.CursorY+1] = b.Lines[b.CursorY+1], b.Lines[b.CursorY]
	b.CursorY++
	b.Modified = true
}

// ToggleComment inserts or removes commentStr at the start of the current line.
func (b *Buffer) ToggleComment(commentStr string) {
	if commentStr == "" {
		return
	}
	b.pushHistory()
	b.clamp()
	line := b.Lines[b.CursorY]
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	switch {
	case strings.HasPrefix(trimmed, commentStr+" "):
		b.Lines[b.CursorY] = indent + trimmed[len(commentStr)+1:]
	case strings.HasPrefix(trimmed, commentStr):
		b.Lines[b.CursorY] = indent + trimmed[len(commentStr):]
	default:
		b.Lines[b.CursorY] = indent + commentStr + " " + trimmed
	}
	b.Modified = true
}

// ReplaceAll replaces every case-insensitive occurrence of old with new.
// Returns the number of replacements made.
func (b *Buffer) ReplaceAll(old, newStr string) int {
	if old == "" {
		return 0
	}
	b.pushHistory()
	count := 0
	lowerOld := strings.ToLower(old)
	for i, line := range b.Lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, lowerOld) {
			b.Lines[i] = replaceAllCI(line, old, newStr)
			count += strings.Count(lowerLine, lowerOld)
		}
	}
	if count > 0 {
		b.Modified = true
	}
	return count
}

// replaceAllCI is a case-insensitive string replacement.
func replaceAllCI(s, old, newStr string) string {
	lowerS := strings.ToLower(s)
	lowerOld := strings.ToLower(old)
	var result strings.Builder
	for len(s) > 0 {
		idx := strings.Index(lowerS, lowerOld)
		if idx < 0 {
			result.WriteString(s)
			break
		}
		result.WriteString(s[:idx])
		result.WriteString(newStr)
		s = s[idx+len(old):]
		lowerS = lowerS[idx+len(old):]
	}
	return result.String()
}

// MoveCursor moves by (dx, dy) with line-wrapping on horizontal movement.
func (b *Buffer) MoveCursor(dx, dy int) {
	b.clamp()
	if dy != 0 {
		b.CursorY += dy
		if b.CursorY < 0 {
			b.CursorY = 0
		}
		if b.CursorY >= len(b.Lines) {
			b.CursorY = len(b.Lines) - 1
		}
		if lineLen := len([]rune(b.Lines[b.CursorY])); b.CursorX > lineLen {
			b.CursorX = lineLen
		}
	}
	if dx != 0 {
		b.CursorX += dx
		lineLen := len([]rune(b.Lines[b.CursorY]))
		if b.CursorX < 0 {
			if b.CursorY > 0 {
				b.CursorY--
				b.CursorX = len([]rune(b.Lines[b.CursorY]))
			} else {
				b.CursorX = 0
			}
		} else if b.CursorX > lineLen {
			if b.CursorY < len(b.Lines)-1 {
				b.CursorY++
				b.CursorX = 0
			} else {
				b.CursorX = lineLen
			}
		}
	}
}

// Save writes the buffer to disk using LF endings.
func (b *Buffer) Save() error {
	if b.Filename == "" {
		return nil
	}
	content := strings.Join(b.Lines, "\n") + "\n"
	err := os.WriteFile(b.Filename, []byte(content), 0644)
	if err == nil {
		b.Modified = false
	}
	return err
}

// ── Selection ─────────────────────────────────────────────────────────────────

// SelectionAnchor holds the fixed end of a selection (the other end is the
// live cursor position).
type SelectionAnchor struct {
	X, Y int
}

// Selection state is stored directly on Buffer so methods can read it.
// Fields are exported so the editor can inspect them for rendering.
type Selection struct {
	Active bool
	Anchor SelectionAnchor // fixed end
}

// StartSelection marks the current cursor position as the anchor.
func (b *Buffer) StartSelection() {
	b.Sel = Selection{
		Active: true,
		Anchor: SelectionAnchor{X: b.CursorX, Y: b.CursorY},
	}
}

// ClearSelection deactivates the selection.
func (b *Buffer) ClearSelection() {
	b.Sel = Selection{}
}

// HasSelection returns true when a selection is active.
func (b *Buffer) HasSelection() bool {
	return b.Sel.Active
}

// selectionBounds returns (startY, startX, endY, endX) normalised so that
// start always comes before end in document order.
func (b *Buffer) selectionBounds() (sy, sx, ey, ex int) {
	return b.SelBounds()
}

// SelBounds is the exported version used by the editor's render pipeline.
func (b *Buffer) SelBounds() (sy, sx, ey, ex int) {
	ay, ax := b.Sel.Anchor.Y, b.Sel.Anchor.X
	cy, cx := b.CursorY, b.CursorX
	if ay < cy || (ay == cy && ax <= cx) {
		return ay, ax, cy, cx
	}
	return cy, cx, ay, ax
}

// GetSelectedText returns the text covered by the active selection.
// Returns "" if no selection is active.
func (b *Buffer) GetSelectedText() string {
	if !b.Sel.Active {
		return ""
	}
	sy, sx, ey, ex := b.selectionBounds()
	if sy == ey {
		r := []rune(b.Lines[sy])
		if sx > len(r) {
			sx = len(r)
		}
		if ex > len(r) {
			ex = len(r)
		}
		return string(r[sx:ex])
	}
	var parts []string
	// First line: from sx to end.
	r0 := []rune(b.Lines[sy])
	if sx > len(r0) {
		sx = len(r0)
	}
	parts = append(parts, string(r0[sx:]))
	// Middle lines: whole lines.
	for i := sy + 1; i < ey; i++ {
		parts = append(parts, b.Lines[i])
	}
	// Last line: from start to ex.
	rN := []rune(b.Lines[ey])
	if ex > len(rN) {
		ex = len(rN)
	}
	parts = append(parts, string(rN[:ex]))
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(p)
	}
	return sb.String()
}

// DeleteSelectedText removes the selected region and places the cursor at the
// start of the removed region. Records history before mutating.
// Returns the deleted text.
func (b *Buffer) DeleteSelectedText() string {
	if !b.Sel.Active {
		return ""
	}
	text := b.GetSelectedText()
	b.pushHistory()

	sy, sx, ey, ex := b.selectionBounds()

	if sy == ey {
		r := []rune(b.Lines[sy])
		if sx > len(r) {
			sx = len(r)
		}
		if ex > len(r) {
			ex = len(r)
		}
		b.Lines[sy] = string(r[:sx]) + string(r[ex:])
		b.CursorY = sy
		b.CursorX = sx
		b.ClearSelection()
		b.Modified = true
		return text
	}

	// Multi-line delete: keep prefix of first line + suffix of last line.
	prefix := string([]rune(b.Lines[sy])[:sx])
	rN := []rune(b.Lines[ey])
	if ex > len(rN) {
		ex = len(rN)
	}
	suffix := string(rN[ex:])
	merged := prefix + suffix

	newLines := make([]string, 0, len(b.Lines)-(ey-sy))
	newLines = append(newLines, b.Lines[:sy]...)
	newLines = append(newLines, merged)
	newLines = append(newLines, b.Lines[ey+1:]...)
	b.Lines = newLines

	b.CursorY = sy
	b.CursorX = sx
	b.ClearSelection()
	b.Modified = true
	return text
}
