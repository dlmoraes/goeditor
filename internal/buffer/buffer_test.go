package buffer

import (
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newBuf(lines ...string) *Buffer {
	if len(lines) == 0 {
		lines = []string{""}
	}
	b := &Buffer{Lines: make([]string, len(lines))}
	copy(b.Lines, lines)
	return b
}

func mustLines(t *testing.T, b *Buffer, want ...string) {
	t.Helper()
	if len(b.Lines) != len(want) {
		t.Fatalf("lines count: got %d, want %d\ngot:  %v\nwant: %v",
			len(b.Lines), len(want), b.Lines, want)
	}
	for i, w := range want {
		if b.Lines[i] != w {
			t.Errorf("line[%d]: got %q, want %q", i, b.Lines[i], w)
		}
	}
}

// ── Undo / Redo ───────────────────────────────────────────────────────────────

func TestUndoBasic(t *testing.T) {
	b := newBuf("hello")
	b.CursorX = 5
	b.InsertChar('!') // "hello!"
	b.InsertChar('!') // "hello!!"
	mustLines(t, b, "hello!!")

	ok := b.Undo() // back to "hello!"
	if !ok {
		t.Fatal("Undo returned false, expected true")
	}
	mustLines(t, b, "hello!")

	b.Undo() // back to "hello"
	mustLines(t, b, "hello")
}

func TestUndoOnEmptyStack(t *testing.T) {
	b := newBuf("")
	ok := b.Undo()
	if ok {
		t.Fatal("Undo on empty stack should return false")
	}
}

func TestRedoAfterUndo(t *testing.T) {
	b := newBuf("")
	b.InsertChar('a')
	b.InsertChar('b')
	mustLines(t, b, "ab")

	b.Undo()
	mustLines(t, b, "a")

	ok := b.Redo()
	if !ok {
		t.Fatal("Redo returned false")
	}
	mustLines(t, b, "ab")
}

func TestRedoOnEmptyStack(t *testing.T) {
	b := newBuf("")
	ok := b.Redo()
	if ok {
		t.Fatal("Redo on empty stack should return false")
	}
}

func TestNewEditClearsFuture(t *testing.T) {
	b := newBuf("")
	b.InsertChar('a') // "a"
	b.Undo()          // ""
	b.InsertChar('x') // "x" — should clear redo stack
	ok := b.Redo()
	if ok {
		t.Fatal("Redo should be impossible after new edit")
	}
	mustLines(t, b, "x")
}

func TestUndoAfterDelete(t *testing.T) {
	b := newBuf("hello")
	b.CursorX = 5
	b.DeleteChar() // "hell"
	b.DeleteChar() // "hel"
	mustLines(t, b, "hel")

	b.Undo() // "hell"
	mustLines(t, b, "hell")
	b.Undo() // "hello"
	mustLines(t, b, "hello")
}

func TestUndoLineMerge(t *testing.T) {
	b := newBuf("hello", "world")
	b.CursorY = 1
	b.CursorX = 0
	b.DeleteChar() // merges into "helloworld"
	mustLines(t, b, "helloworld")

	b.Undo()
	mustLines(t, b, "hello", "world")
	if b.CursorY != 1 {
		t.Errorf("CursorY after undo: got %d, want 1", b.CursorY)
	}
}

func TestUndoInsertString(t *testing.T) {
	b := newBuf("")
	b.InsertString("hello world") // single undo step
	mustLines(t, b, "hello world")
	b.Undo()
	mustLines(t, b, "")
}

func TestUndoKillLine(t *testing.T) {
	b := newBuf("aaa", "bbb", "ccc")
	b.CursorY = 1
	b.DeleteCurrentLine()
	mustLines(t, b, "aaa", "ccc")
	b.Undo()
	mustLines(t, b, "aaa", "bbb", "ccc")
}

func TestUndoDuplicateLine(t *testing.T) {
	b := newBuf("hello")
	b.DuplicateCurrentLine()
	mustLines(t, b, "hello", "hello")
	b.Undo()
	mustLines(t, b, "hello")
}

func TestHistoryLimit(t *testing.T) {
	b := newBuf("")
	// Push maxHistory+10 operations — should not panic.
	for i := 0; i < maxHistory+10; i++ {
		b.InsertChar('x')
	}
	// At least maxHistory undos should work (we may lose the earliest ones).
	count := 0
	for b.Undo() {
		count++
	}
	if count < maxHistory {
		t.Errorf("expected at least %d undos, got %d", maxHistory, count)
	}
}

// ── Auto-indent ───────────────────────────────────────────────────────────────

func TestAutoIndentSpaces(t *testing.T) {
	b := newBuf("    func foo() {")
	b.CursorX = len("    func foo() {")
	b.InsertLine()
	if b.Lines[1] != "    " {
		// Allow empty-trimmed result too (some editors strip trailing spaces)
		if strings.TrimRight(b.Lines[1], " \t") != "" {
			t.Errorf("line[1]: got %q, want %q", b.Lines[1], "    ")
		}
	}
	if b.CursorX != 4 {
		t.Errorf("CursorX after auto-indent: got %d, want 4", b.CursorX)
	}
}

func TestAutoIndentTabs(t *testing.T) {
	b := newBuf("\t\tvar x = 1")
	b.CursorX = len("\t\tvar x = 1")
	b.InsertLine()
	if !strings.HasPrefix(b.Lines[1], "\t\t") {
		t.Errorf("line[1]: got %q, expected tab-indented", b.Lines[1])
	}
}

func TestAutoIndentNoIndent(t *testing.T) {
	b := newBuf("hello")
	b.CursorX = 5
	b.InsertLine()
	if b.Lines[1] != "" {
		t.Errorf("line[1]: got %q, want empty (no indent to carry)", b.Lines[1])
	}
}

func TestAutoIndentMidLine(t *testing.T) {
	// "    hello world" — split right after "hello" (position 9).
	// Rune layout: [0..3]=spaces, [4..8]="hello", [9]=' ', [10..14]="world"
	// Splitting at 9 gives before="    hello", after=" world".
	// After auto-indent the new line gets 4 spaces + " world" = "     world".
	b := newBuf("    hello world")
	b.CursorX = 9 // right after "hello", before the space
	b.InsertLine()
	if b.Lines[0] != "    hello" {
		t.Errorf("line[0]: got %q, want %q", b.Lines[0], "    hello")
	}
	// indent (4 spaces) + after (" world") = "     world"
	want1 := "     world"
	if b.Lines[1] != want1 {
		t.Errorf("line[1]: got %q, want %q", b.Lines[1], want1)
	}
}

// ── SwapLines ─────────────────────────────────────────────────────────────────

func TestSwapLineUp(t *testing.T) {
	b := newBuf("a", "b", "c")
	b.CursorY = 1
	b.SwapLineUp()
	mustLines(t, b, "b", "a", "c")
	if b.CursorY != 0 {
		t.Errorf("CursorY: got %d, want 0", b.CursorY)
	}
}

func TestSwapLineUpBoundary(t *testing.T) {
	b := newBuf("a", "b")
	b.CursorY = 0
	b.SwapLineUp() // no-op
	mustLines(t, b, "a", "b")
}

func TestSwapLineDown(t *testing.T) {
	b := newBuf("a", "b", "c")
	b.CursorY = 1
	b.SwapLineDown()
	mustLines(t, b, "a", "c", "b")
	if b.CursorY != 2 {
		t.Errorf("CursorY: got %d, want 2", b.CursorY)
	}
}

func TestSwapLineDownBoundary(t *testing.T) {
	b := newBuf("a", "b")
	b.CursorY = 1
	b.SwapLineDown() // no-op
	mustLines(t, b, "a", "b")
}

func TestSwapUndoable(t *testing.T) {
	b := newBuf("a", "b", "c")
	b.CursorY = 1
	b.SwapLineUp()
	mustLines(t, b, "b", "a", "c")
	b.Undo()
	mustLines(t, b, "a", "b", "c")
}

// ── ToggleComment ─────────────────────────────────────────────────────────────

func TestToggleCommentInsert(t *testing.T) {
	b := newBuf("fmt.Println()")
	b.ToggleComment("//")
	mustLines(t, b, "// fmt.Println()")
}

func TestToggleCommentRemove(t *testing.T) {
	b := newBuf("// fmt.Println()")
	b.ToggleComment("//")
	mustLines(t, b, "fmt.Println()")
}

func TestToggleCommentRemoveNoSpace(t *testing.T) {
	b := newBuf("//fmt.Println()")
	b.ToggleComment("//")
	mustLines(t, b, "fmt.Println()")
}

func TestToggleCommentPreservesIndent(t *testing.T) {
	b := newBuf("    return nil")
	b.ToggleComment("//")
	mustLines(t, b, "    // return nil")
}

func TestToggleCommentHash(t *testing.T) {
	b := newBuf("print('hello')")
	b.ToggleComment("#")
	mustLines(t, b, "# print('hello')")
	b.ToggleComment("#")
	mustLines(t, b, "print('hello')")
}

func TestToggleCommentUndoable(t *testing.T) {
	b := newBuf("hello()")
	b.ToggleComment("//")
	mustLines(t, b, "// hello()")
	b.Undo()
	mustLines(t, b, "hello()")
}

// ── ReplaceAll ────────────────────────────────────────────────────────────────

func TestReplaceAll(t *testing.T) {
	b := newBuf("foo foo foo")
	n := b.ReplaceAll("foo", "bar")
	if n != 3 {
		t.Errorf("replaced: got %d, want 3", n)
	}
	mustLines(t, b, "bar bar bar")
}

func TestReplaceAllCaseInsensitive(t *testing.T) {
	b := newBuf("Foo FOO foo")
	n := b.ReplaceAll("foo", "baz")
	if n != 3 {
		t.Errorf("replaced: got %d, want 3", n)
	}
	mustLines(t, b, "baz baz baz")
}

func TestReplaceAllMultiLine(t *testing.T) {
	b := newBuf("hello world", "hello again")
	b.ReplaceAll("hello", "hi")
	mustLines(t, b, "hi world", "hi again")
}

func TestReplaceAllNotFound(t *testing.T) {
	b := newBuf("hello")
	n := b.ReplaceAll("xyz", "abc")
	if n != 0 {
		t.Errorf("replaced: got %d, want 0", n)
	}
	mustLines(t, b, "hello")
}

func TestReplaceAllDelete(t *testing.T) {
	b := newBuf("hello world")
	b.ReplaceAll("world", "")
	mustLines(t, b, "hello ")
}

func TestReplaceAllUndoable(t *testing.T) {
	b := newBuf("foo foo")
	b.ReplaceAll("foo", "bar")
	mustLines(t, b, "bar bar")
	b.Undo()
	mustLines(t, b, "foo foo")
}

// ── Cursor clamping ───────────────────────────────────────────────────────────

func TestCursorClampOnDelete(t *testing.T) {
	b := newBuf("hello", "world")
	b.CursorY = 1
	b.CursorX = 5
	b.DeleteCurrentLine()
	// After deleting line 1, CursorY must be 0 and CursorX clamped to len("hello")=5
	if b.CursorY != 0 {
		t.Errorf("CursorY: got %d, want 0", b.CursorY)
	}
	if b.CursorX > len("hello") {
		t.Errorf("CursorX: got %d, exceeds line length", b.CursorX)
	}
}

func TestMoveCursorWrapsRight(t *testing.T) {
	b := newBuf("ab", "cd")
	b.CursorY = 0
	b.CursorX = 2 // end of "ab"
	b.MoveCursor(1, 0)
	if b.CursorY != 1 || b.CursorX != 0 {
		t.Errorf("wrap right: got (%d,%d), want (0,1)", b.CursorX, b.CursorY)
	}
}

func TestMoveCursorWrapsLeft(t *testing.T) {
	b := newBuf("ab", "cd")
	b.CursorY = 1
	b.CursorX = 0
	b.MoveCursor(-1, 0)
	if b.CursorY != 0 || b.CursorX != 2 {
		t.Errorf("wrap left: got (%d,%d), want (2,0)", b.CursorX, b.CursorY)
	}
}
