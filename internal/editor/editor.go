package editor

import (
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/markdown"
)

// Editor is the model of the text editor.
type Editor struct {
	Dirty     bool // exported
	WantsSave bool // exported, true when Update receives ctrl+s
	WantsExit bool // exported, true when Update receives ctrl+q (or esc): asks to exit
	WantsQuit bool // exported, true when Update receives ctrl+c: always quits
	// WantsPalette is exported: true when Update receives ctrl+t, that is when the
	// formatting palette has to be opened. The screen reads it and resets it.
	WantsPalette bool

	lines      []string
	cursorLine int
	cursorCol  int
	hScroll    int
	width      int
	height     int
	selecting  bool
	anchorLine int
	anchorCol  int

	// frozen keeps the view still while a selection is dragged with the
	// mouse: the Focus Line stays on viewLine even though the cursor (the end
	// of the selection) is on another line. Any key, click or scroll ends it.
	frozen   bool
	viewLine int
}

// runeLen counts the runes of a string.
func runeLen(s string) int { return len([]rune(s)) }

// New creates an Editor on the given lines; if lines is nil it uses one empty line.
// The cursor starts at the end of the first line, that is after the last rune of the text
// already there: so the first rune typed is appended to the existing text
// instead of ending up in front of it, and the cursor stays visible.
func New(lines []string, width, height int) Editor {
	if lines == nil {
		lines = []string{""}
	}
	e := Editor{
		lines:      lines,
		cursorLine: 0,
		cursorCol:  runeLen(lines[0]),
		width:      width,
		height:     height,
	}
	e.ensureCursorVisible()
	return e
}

// Init does not run any command.
func (e Editor) Init() tea.Cmd { return nil }

// Lines returns a copy of the lines.
func (e Editor) Lines() []string {
	out := make([]string, len(e.lines))
	copy(out, e.lines)
	return out
}

// LineCount is the number of lines.
func (e Editor) LineCount() int { return len(e.lines) }

// CursorRow is the cursor's line number, 1-based.
func (e Editor) CursorRow() int { return e.cursorLine + 1 }

// CursorCol1 is the cursor's column number, 1-based.
func (e Editor) CursorCol1() int { return e.cursorCol + 1 }

// SetSize updates the dimensions and makes the cursor visible.
func (e Editor) SetSize(width, height int) Editor {
	e.width = width
	e.height = height
	e.ensureCursorVisible()
	return e
}

// --- Geometry ---

// focusRow is the screen row where the cursor's line is located.
func (e Editor) focusRow() int {
	r := (e.height - 1) / 2
	if r < 0 {
		r = 0
	}
	return r
}

// FocusRow is the screen row of the Focus Line, the one that shows the raw
// text of the cursor's line.
func (e Editor) FocusRow() int { return e.focusRow() }

// FocusCellToCol converts a cell of the Focus Line (counted from the left of
// the editor) into the column, in runes, of the cursor's line. The Focus Line
// is drawn after a gutter as wide as the markdown prefix removed on the other
// lines, and from the horizontal scroll: both are taken into account, so a
// click on a character lands on that character.
func (e Editor) FocusCellToCol(cell int) int {
	raw := e.LineAt(e.focusLine())
	cell -= markdown.PrefixWidth(raw)
	if cell < 0 {
		cell = 0
	}
	col := cell + e.hScroll
	if n := runeLen(raw); col > n {
		col = n
	}
	return col
}

// focusLine is the document line shown on the Focus Line: the cursor's line,
// or the line the view was frozen on while a selection is being dragged.
func (e Editor) focusLine() int {
	if e.frozen && e.viewLine >= 0 && e.viewLine < len(e.lines) {
		return e.viewLine
	}
	return e.cursorLine
}

// unfreeze ends the frozen view: the Focus Line goes back to the cursor.
func (e *Editor) unfreeze() {
	if e.frozen {
		e.frozen = false
		e.ensureCursorVisible()
	}
}

// docIndexForRow converts a screen row into the document index.
func (e Editor) docIndexForRow(r int) int {
	return e.focusLine() - e.focusRow() + r
}

// LineAtRow returns the index of the document line drawn on row r
// of the editor's screen: the view renders one row for each document line,
// with the cursor's line on the focus row. The second value is false when
// r is outside the view or when the row does not correspond to any line of the
// document (for example the empty rows beyond the ends).
func (e Editor) LineAtRow(r int) (int, bool) {
	if r < 0 || r >= e.height {
		return 0, false
	}
	idx := e.docIndexForRow(r)
	if idx < 0 || idx >= len(e.lines) {
		return 0, false
	}
	return idx, true
}

// LineAt returns the text of line idx, "" if it is outside the document.
func (e Editor) LineAt(idx int) string {
	if idx < 0 || idx >= len(e.lines) {
		return ""
	}
	return e.lines[idx]
}

// LineSegments returns the markdown segments with which the view draws line
// idx when out of focus, with inside-fence computed as in renderRow: the
// screen uses them to map a column in cells to the column in the raw
// line. Beyond the document it returns nil.
func (e Editor) LineSegments(idx int) []markdown.Segment {
	if idx < 0 || idx >= len(e.lines) {
		return nil
	}
	return markdown.RenderLine(e.lines[idx], isInsideFence(e.lines, idx))
}

// SetCursor moves the cursor to the given (line, column) position, with line
// and column 0-based in runes, and clears the selection. Lines outside the
// document are brought back inside, the column is brought back within the end
// of the line; the resulting cursor stays visible. The cursor's line is the
// focus line, so the given line becomes the Focus Line.
func (e Editor) SetCursor(line, col int) Editor {
	e.clearSelection()
	e.frozen = false
	if line < 0 {
		line = 0
	}
	if line > len(e.lines)-1 {
		line = len(e.lines) - 1
	}
	if line < 0 {
		line = 0
	}
	e.cursorLine = line
	if col < 0 {
		col = 0
	}
	if n := runeLen(e.currentLine()); col > n {
		col = n
	}
	e.cursorCol = col
	e.ensureCursorVisible()
	return e
}

// ensureCursorVisible adjusts the horizontal scroll so that the cursor stays in view.
func (e *Editor) ensureCursorVisible() {
	if e.cursorCol < e.hScroll {
		e.hScroll = e.cursorCol
	}
	if e.cursorCol >= e.hScroll+e.width {
		e.hScroll = e.cursorCol - e.width + 1
	}
	if e.hScroll < 0 {
		e.hScroll = 0
	}
}

// clampCol brings the cursor back within the end of the current line.
func (e *Editor) clampCol() {
	if e.cursorCol > runeLen(e.currentLine()) {
		e.cursorCol = runeLen(e.currentLine())
	}
}

// clampedLine returns the current line as runes, after bringing the
// cursor column back inside [0, length of the line].
func (e *Editor) clampedLine() []rune {
	line := []rune(e.currentLine())
	if e.cursorCol > len(line) {
		e.cursorCol = len(line)
	}
	if e.cursorCol < 0 {
		e.cursorCol = 0
	}
	return line
}

// replaceLines replaces the lines from sl to el (both included) with the single
// line merged.
func (e *Editor) replaceLines(sl, el int, merged string) {
	out := make([]string, 0, len(e.lines)-(el-sl))
	out = append(out, e.lines[:sl]...)
	out = append(out, merged)
	out = append(out, e.lines[el+1:]...)
	e.lines = out
}

// currentLine is the line under the cursor ("" if out of range).
func (e Editor) currentLine() string {
	if e.cursorLine < 0 || e.cursorLine >= len(e.lines) {
		return ""
	}
	return e.lines[e.cursorLine]
}

// --- Movements ---

// moveUp moves the cursor one line up.
func (e Editor) moveUp() Editor {
	e.clearSelection()
	if e.cursorLine > 0 {
		e.cursorLine--
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// moveDown moves the cursor one line down.
func (e Editor) moveDown() Editor {
	e.clearSelection()
	if e.cursorLine < len(e.lines)-1 {
		e.cursorLine++
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// Scroll moves the cursor by n lines (negative = up), staying inside the
// document and clearing the selection. Since the cursor line is the Focus
// Line, the document scrolls under it: this is what the mouse wheel and the
// touchpad do.
func (e Editor) Scroll(n int) Editor {
	e.clearSelection()
	e.cursorLine = e.focusLine() + n
	e.frozen = false
	if e.cursorLine > len(e.lines)-1 {
		e.cursorLine = len(e.lines) - 1
	}
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// moveLeft moves the cursor one rune to the left, across lines.
func (e Editor) moveLeft() Editor {
	e.clearSelection()
	if e.cursorCol > 0 {
		e.cursorCol--
	} else if e.cursorLine > 0 {
		e.cursorLine--
		e.cursorCol = runeLen(e.currentLine())
	}
	e.ensureCursorVisible()
	return e
}

// moveRight moves the cursor one rune to the right, across lines.
func (e Editor) moveRight() Editor {
	e.clearSelection()
	if e.cursorCol < runeLen(e.currentLine()) {
		e.cursorCol++
	} else if e.cursorLine < len(e.lines)-1 {
		e.cursorLine++
		e.cursorCol = 0
	}
	e.ensureCursorVisible()
	return e
}

// moveHome brings the cursor to the start of the line.
func (e Editor) moveHome() Editor {
	e.clearSelection()
	e.cursorCol = 0
	e.ensureCursorVisible()
	return e
}

// moveEnd brings the cursor to the end of the line.
func (e Editor) moveEnd() Editor {
	e.clearSelection()
	e.cursorCol = runeLen(e.currentLine())
	e.ensureCursorVisible()
	return e
}

// moveDocStart brings the cursor to the start of the document.
func (e Editor) moveDocStart() Editor {
	e.clearSelection()
	e.cursorLine = 0
	e.cursorCol = 0
	e.ensureCursorVisible()
	return e
}

// moveDocEnd brings the cursor to the end of the document.
func (e Editor) moveDocEnd() Editor {
	e.clearSelection()
	e.cursorLine = len(e.lines) - 1
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	e.cursorCol = runeLen(e.currentLine())
	e.ensureCursorVisible()
	return e
}

// pageUp moves the cursor height lines up.
func (e Editor) pageUp() Editor {
	e.clearSelection()
	e.cursorLine -= e.height
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// pageDown moves the cursor height lines down.
func (e Editor) pageDown() Editor {
	e.clearSelection()
	e.cursorLine += e.height
	if e.cursorLine > len(e.lines)-1 {
		e.cursorLine = len(e.lines) - 1
	}
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// --- Selection ---

// beginSelection sets the selection anchor if it is not already active.
func (e *Editor) beginSelection() {
	if !e.selecting {
		e.anchorLine = e.cursorLine
		e.anchorCol = e.cursorCol
		e.selecting = true
	}
}

// clearSelection deactivates the selection.
func (e *Editor) clearSelection() {
	e.selecting = false
}

// selectedRange normalises anchor and cursor in ascending (line, column) order.
func (e Editor) selectedRange() (sl, sc, el, ec int) {
	if e.anchorLine < e.cursorLine || (e.anchorLine == e.cursorLine && e.anchorCol <= e.cursorCol) {
		return e.anchorLine, e.anchorCol, e.cursorLine, e.cursorCol
	}
	return e.cursorLine, e.cursorCol, e.anchorLine, e.anchorCol
}

// isSelectWordRune says whether r is a word character for selection with the
// mouse: a letter, a digit or the underscore. Every other character separates
// words.
func isSelectWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// SelectWord selects the word under the cursor, reusing the existing selection
// model: the anchor goes to the start of the word and the cursor to its
// end. If the cursor is right after a word (for example at the end of the line) the
// word that precedes it is used. If there is no word under the cursor the
// selection is cleared and the cursor stays where it is.
func (e Editor) SelectWord() Editor {
	e.frozen = false
	line := e.clampedLine()
	at := e.cursorCol
	if at >= len(line) || !isSelectWordRune(line[at]) {
		if at > 0 && isSelectWordRune(line[at-1]) {
			at--
		} else {
			e.clearSelection()
			e.ensureCursorVisible()
			return e
		}
	}
	start := at
	for start > 0 && isSelectWordRune(line[start-1]) {
		start--
	}
	end := at
	for end < len(line) && isSelectWordRune(line[end]) {
		end++
	}
	e.anchorLine = e.cursorLine
	e.anchorCol = start
	e.selecting = true
	e.cursorCol = end
	e.ensureCursorVisible()
	return e
}

// SelectLine selects the whole line of the cursor, from start to end, with the
// same selection model (anchor at column 0, cursor at the end of the line).
func (e Editor) SelectLine() Editor {
	e.frozen = false
	e.anchorLine = e.cursorLine
	e.anchorCol = 0
	e.selecting = true
	e.cursorCol = runeLen(e.currentLine())
	e.ensureCursorVisible()
	return e
}
