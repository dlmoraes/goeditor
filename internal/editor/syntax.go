package editor

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"goeditor/internal/buffer"
)

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

func commentPrefixFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".go", ".c", ".cpp", ".cc", ".h", ".hpp", ".java", ".js", ".ts", ".rs", ".swift", ".kt":
		return "//"
	case ".py", ".sh", ".rb", ".pl", ".r", ".yml", ".yaml", ".toml":
		return "#"
	case ".lua", ".sql", ".hs":
		return "--"
	case ".html", ".xml":
		return ""
	default:
		return "//"
	}
}

func languageName(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".go": return "Go"
	case ".py": return "Python"
	case ".js": return "JavaScript"
	case ".ts": return "TypeScript"
	case ".rs": return "Rust"
	case ".c", ".h": return "C"
	case ".cpp", ".cc", ".hpp": return "C++"
	case ".java": return "Java"
	case ".lua": return "Lua"
	case ".sh": return "Shell"
	case ".md": return "Markdown"
	case ".toml": return "TOML"
	case ".yaml", ".yml": return "YAML"
	case ".json": return "JSON"
	case ".sql": return "SQL"
	case ".html": return "HTML"
	case ".css": return "CSS"
	default: return "Text"
	}
}

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
