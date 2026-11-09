package editor

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/markdown"
)

// --- Reading mode ---
//
// Reading mode shows the whole note rendered, Focus Line included, in full
// strength and always wrapped to the width, and only lets you scroll it. It
// has its own position, a line and a row of that line on the middle row of
// the screen, that moves independently of the cursor; leaving the mode puts
// the cursor where the reading got to.

// readPos is where reading mode was entered from: the position on the screen
// and the cursor.
type readPos struct {
	line, sub, curLine, curCol int
}

// rowRef says what is drawn on a screen row in reading mode.
type rowRef struct {
	line   int
	ok     bool // false for the rows past either end of the document
	inside bool // the line is inside a code block
	segs   []markdown.Segment
}

// Reading reports whether reading mode is on.
func (e Editor) Reading() bool { return e.reading }

// WithReading turns reading mode on or off, starting from the top of the
// note. It is for opening a note in reading mode; ctrl+p toggles from
// the cursor.
func (e Editor) WithReading(on bool) Editor {
	if on {
		return e.enterReading(false)
	}
	return e.leaveReading()
}

// toggleReading is ctrl+p: it turns reading mode on or off and tells the
// screen (ReadChanged).
func (e Editor) toggleReading() Editor {
	if e.reading {
		e = e.leaveReading()
	} else {
		e = e.enterReading(true)
	}
	e.ReadChanged = true
	return e
}

// renderedRows returns the rows that line idx takes in reading mode, and
// whether it is inside a code block: tables narrow their columns and wrap
// their cells, other lines wrap at spaces.
func (e Editor) renderedRows(idx int) ([][]markdown.Segment, bool) {
	inside := isInsideFence(e.lines, idx)
	if !inside {
		if t, ok := markdown.FindTable(e.lines, idx); ok {
			return markdown.RenderTableRows(e.lines, t, idx, e.width), false
		}
	}
	return markdown.WrapSegments(markdown.RenderLine(e.lines[idx], inside), e.width), inside
}

// rowCount is how many rows line idx takes in reading mode.
func (e Editor) rowCount(idx int) int {
	rows, _ := e.renderedRows(idx)
	return len(rows)
}

// rowSpan returns the largest raw byte offset a row draws, and the offset
// where it starts.
func rowSpan(row []markdown.Segment) (start, end int) {
	for i, s := range row {
		if i == 0 {
			start = s.RawFrom
		}
		end = max(end, s.RawTo)
	}
	return start, end
}

// enterReading turns reading mode on, from the cursor's row or from the top.
func (e Editor) enterReading(fromCursor bool) Editor {
	e.clearSelection()
	e.frozen = false
	if e.vim {
		// Leave whatever vim was in the middle of: reading is not a mode of it.
		e.vimMode = vimNormal
		e.vimPending, e.vimCount, e.vimPreCount = "", 0, 0
		e.vimCmdOn, e.vimCmd = false, ""
	}
	e.readLine, e.readSub = 0, 0
	if fromCursor && len(e.lines) > 0 {
		e.readLine = clampLine(e.cursorLine, len(e.lines))
		rows, _ := e.renderedRows(e.readLine)
		off := len(string([]rune(e.lines[e.readLine])[:min(e.cursorCol, runeLen(e.lines[e.readLine]))]))
		e.readSub = len(rows) - 1
		for i, row := range rows {
			if _, end := rowSpan(row); end > off {
				e.readSub = i
				break
			}
		}
	}
	e.readEntry = readPos{e.readLine, e.readSub, e.cursorLine, e.cursorCol}
	e.reading = true
	return e
}

// leaveReading turns reading mode off. The cursor goes back where it was if
// the reading did not move, otherwise to the start of the row it reached.
func (e Editor) leaveReading() Editor {
	if !e.reading {
		return e
	}
	e.reading = false
	e.clearSelection()
	e.frozen = false
	if e.readLine != e.readEntry.line || e.readSub != e.readEntry.sub {
		e.cursorLine, e.cursorCol = e.readLine, 0
		if rows, _ := e.renderedRows(e.readLine); e.readSub > 0 && e.readSub < len(rows) {
			start, _ := rowSpan(rows[e.readSub])
			raw := e.lines[e.readLine]
			e.cursorCol = utf8.RuneCountInString(raw[:min(start, len(raw))])
		}
	} else {
		e.cursorLine, e.cursorCol = e.readEntry.curLine, e.readEntry.curCol
	}
	e.cursorLine = clampLine(e.cursorLine, len(e.lines))
	e.clampCol()
	e.ensureCursorVisible()
	e.vimSettle()
	return e
}

// readMove moves the reading position n screen rows down (up when n is
// negative), stopping at the ends of the note.
func (e Editor) readMove(n int) Editor {
	line, sub := e.readLine, e.readSub
	rows := e.rowCount(line)
	for ; n > 0; n-- {
		switch {
		case sub+1 < rows:
			sub++
		case line+1 < len(e.lines):
			line++
			sub, rows = 0, e.rowCount(line)
		default:
			n = 1
		}
	}
	for ; n < 0; n++ {
		switch {
		case sub > 0:
			sub--
		case line > 0:
			line--
			rows = e.rowCount(line)
			sub = rows - 1
		default:
			n = -1
		}
	}
	e.readLine, e.readSub = line, sub
	return e
}

// readTop and readBottom put the reading position on the first and the last
// row of the note.
func (e Editor) readTop() Editor {
	e.readLine, e.readSub = 0, 0
	return e
}

func (e Editor) readBottom() Editor {
	e.readLine = max(len(e.lines)-1, 0)
	e.readSub = max(e.rowCount(e.readLine)-1, 0)
	return e
}

// readingUpdate handles a key in reading mode: it scrolls, or leaves the mode,
// the note or Nook; everything else is ignored, so the text cannot change.
func (e Editor) readingUpdate(key tea.KeyMsg) Editor {
	page := max(e.height-1, 1)
	switch key.String() {
	case "ctrl+p":
		return e.toggleReading()
	case "ctrl+c":
		e.WantsQuit = true
	case "ctrl+s", "f2":
		e.WantsSave = true
	case "ctrl+q":
		e.WantsExit = true
	case "esc":
		// As in vim's NORMAL mode, esc does not leave the note there.
		e.WantsExit = !e.vim
	case "up", "k":
		return e.readMove(-1)
	case "down", "j":
		return e.readMove(1)
	case "pgup", "ctrl+b":
		return e.readMove(-page)
	case "pgdown", "ctrl+f", " ":
		return e.readMove(page)
	case "ctrl+u":
		return e.readMove(-max(page/2, 1))
	case "ctrl+d":
		return e.readMove(max(page/2, 1))
	case "home", "ctrl+home", "ctrl+up", "g":
		return e.readTop()
	case "end", "ctrl+end", "ctrl+down", "G":
		return e.readBottom()
	}
	return e
}

// readLayout says what is drawn on each screen row in reading mode: the
// rows of the line being read around the middle row, the lines before it above
// and the lines after it below.
func (e Editor) readLayout() []rowRef {
	refs := make([]rowRef, max(e.height, 0))
	if len(refs) == 0 || len(e.lines) == 0 {
		return refs
	}
	mid, line := e.focusRow(), clampLine(e.readLine, len(e.lines))
	rows, inside := e.renderedRows(line)
	sub := min(e.readSub, len(rows)-1)
	for i, row := range rows {
		if r := mid - sub + i; r >= 0 && r < len(refs) {
			refs[r] = rowRef{line: line, ok: true, inside: inside, segs: row}
		}
	}
	r := mid - sub - 1
	for l := line - 1; r >= 0 && l >= 0; l-- {
		rows, inside := e.renderedRows(l)
		for k := len(rows) - 1; k >= 0 && r >= 0; k-- {
			refs[r] = rowRef{line: l, ok: true, inside: inside, segs: rows[k]}
			r--
		}
	}
	r = mid - sub + len(rows)
	for l := line + 1; r < len(refs) && l < len(e.lines); l++ {
		rows, inside := e.renderedRows(l)
		for k := 0; k < len(rows) && r < len(refs); k++ {
			refs[r] = rowRef{line: l, ok: true, inside: inside, segs: rows[k]}
			r++
		}
	}
	return refs
}

// viewReading draws the editor in reading mode.
func (e Editor) viewReading() string {
	rows := make([]string, max(e.height, 0))
	for r, ref := range e.readLayout() {
		if ref.ok {
			rows[r] = e.renderSegs(ref.line, ref.segs, ref.inside)
		}
	}
	return strings.Join(rows, "\n")
}

// HitCell converts a click on the screen row y, at cell x of the editor, into
// the document line and the column, in runes, it stands for. On the Focus
// Line the text is raw, drawn after a gutter; on any other row (every row in
// reading mode) the cell goes through the segments the row is drawn with. ok
// is false when no line is drawn there.
func (e Editor) HitCell(y, x int) (line, col int, ok bool) {
	var segs []markdown.Segment
	if e.reading {
		refs := e.readLayout()
		if y < 0 || y >= len(refs) || !refs[y].ok {
			return 0, 0, false
		}
		line, segs = refs[y].line, refs[y].segs
	} else {
		if line, ok = e.LineAtRow(y); !ok {
			return 0, 0, false
		}
		if y == e.focusRow() {
			return line, e.FocusCellToCol(x), true
		}
		segs = e.LineSegments(line)
	}
	raw := e.lines[line]
	off := min(max(markdown.MapCellToRaw(segs, x), 0), len(raw))
	return line, utf8.RuneCountInString(raw[:off]), true
}

// readingRowsText is used by tests: the text of every screen row, without styles.
func (e Editor) readingRowsText() []string {
	var out []string
	for _, ref := range e.readLayout() {
		var b strings.Builder
		for _, s := range ref.segs {
			b.WriteString(s.Text)
		}
		out = append(out, b.String())
	}
	return out
}
