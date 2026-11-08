package editor

import (
	"strings"
	"unicode/utf8"

	"nook/internal/markdown"
)

// --- Word wrap ---
//
// With word wrap on, a line of the document takes as many screen rows as its
// text needs. The Focus Line shows its raw text cut into rows, and the row
// that holds the cursor stays on the middle row of the screen; every other
// line shows its rendered markdown cut to the width (see markdown.WrapSegments
// and markdown.RenderTableRows). Moving, scrolling and clicking work on those
// rows. With word wrap off none of this is used.

// rowRef says what is drawn on a screen row.
type rowRef struct {
	line, sub int  // document line and row of that line
	ok        bool // false for the rows past either end of the document
	focus     bool // the row belongs to the Focus Line: raw text
	inside    bool // the line is inside a code block (rendered rows only)
	segs      []markdown.Segment
}

// WithWrap turns word wrap on or off.
func (e Editor) WithWrap(on bool) Editor {
	e.wrap = on
	e.hScroll = 0
	e.ensureCursorVisible()
	return e
}

// Wrap reports whether word wrap is on.
func (e Editor) Wrap() bool { return e.wrap }

// toggleWrap turns word wrap on or off and tells the screen (WrapChanged).
func (e Editor) toggleWrap() Editor {
	e = e.WithWrap(!e.wrap)
	e.WrapChanged = true
	return e
}

// rawBreaks returns where each row of the raw text of line idx starts when
// the line is the Focus Line: the rows are as wide as the editor minus the
// gutter, and one cell narrower so the cursor fits after the last character.
func (e Editor) rawBreaks(idx int) []int {
	raw := e.LineAt(idx)
	w := e.width - markdown.PrefixWidth(raw) - 1
	return markdown.Breaks([]rune(raw), max(w, 1))
}

// focusSub is the row of the Focus Line that sits on the focus row: the row
// that holds the cursor, or the one the view was frozen on.
func (e Editor) focusSub() int {
	if !e.wrap {
		return 0
	}
	if e.frozen {
		return e.viewSub
	}
	return markdown.RowOf(e.rawBreaks(e.cursorLine), e.cursorCol)
}

// renderedRows returns the rows that line idx takes when it is not the Focus
// Line, and whether it is inside a code block.
func (e Editor) renderedRows(idx int) ([][]markdown.Segment, bool) {
	inside := isInsideFence(e.lines, idx)
	if !inside {
		if t, ok := markdown.FindTable(e.lines, idx); ok {
			return markdown.RenderTableRows(e.lines, t, idx, e.width), false
		}
	}
	return markdown.WrapSegments(markdown.RenderLine(e.lines[idx], inside), e.width), inside
}

// layout says what is drawn on each screen row, from the top: the Focus Line
// around the focus row, the lines before it above and the lines after it
// below, each as many rows as it takes.
func (e Editor) layout() []rowRef {
	refs := make([]rowRef, max(e.height, 0))
	if len(refs) == 0 || len(e.lines) == 0 {
		return refs
	}
	if !e.wrap {
		for r := range refs {
			if idx := e.docIndexForRow(r); idx >= 0 && idx < len(e.lines) {
				refs[r] = rowRef{line: idx, ok: true, focus: r == e.focusRow()}
			}
		}
		return refs
	}

	fr, focus, sub := e.focusRow(), e.focusLine(), e.focusSub()
	n := len(e.rawBreaks(focus))
	for i := 0; i < n; i++ {
		if r := fr - sub + i; r >= 0 && r < len(refs) {
			refs[r] = rowRef{line: focus, sub: i, ok: true, focus: true}
		}
	}
	r := fr - sub - 1
	for line := focus - 1; r >= 0 && line >= 0; line-- {
		rows, inside := e.renderedRows(line)
		for k := len(rows) - 1; k >= 0 && r >= 0; k-- {
			refs[r] = rowRef{line: line, sub: k, ok: true, inside: inside, segs: rows[k]}
			r--
		}
	}
	r = fr - sub + n
	for line := focus + 1; r < len(refs) && line < len(e.lines); line++ {
		rows, inside := e.renderedRows(line)
		for k := 0; k < len(rows) && r < len(refs); k++ {
			refs[r] = rowRef{line: line, sub: k, ok: true, inside: inside, segs: rows[k]}
			r++
		}
	}
	return refs
}

// HitCell converts a click on the screen row y, at cell x of the editor, into
// the document line and the column, in runes, it stands for. On the Focus
// Line the text is raw, drawn after a gutter; on any other row the cell goes
// through the segments the row is drawn with. ok is false when no line is
// drawn there.
func (e Editor) HitCell(y, x int) (line, col int, ok bool) {
	refs := e.layout()
	if y < 0 || y >= len(refs) || !refs[y].ok {
		return 0, 0, false
	}
	ref := refs[y]
	raw := e.lines[ref.line]
	if ref.focus {
		if !e.wrap {
			return ref.line, e.FocusCellToCol(x), true
		}
		bs := e.rawBreaks(ref.line)
		col = bs[ref.sub] + max(x-markdown.PrefixWidth(raw), 0)
		return ref.line, min(col, markdown.RowEnd(bs, ref.sub, runeLen(raw))), true
	}
	segs := ref.segs
	if !e.wrap {
		segs = e.LineSegments(ref.line)
	}
	off := min(max(markdown.MapCellToRaw(segs, x), 0), len(raw))
	return ref.line, utf8.RuneCountInString(raw[:off]), true
}

// stepRows returns the position n rows after (or before, n < 0) row sub of
// line, stopping at the ends of the document. Lines are measured as they are
// when the Focus Line, which is what the one the cursor lands on will be.
func (e Editor) stepRows(line, sub, n int) (int, int) {
	bs := e.rawBreaks(line)
	for ; n > 0; n-- {
		switch {
		case sub+1 < len(bs):
			sub++
		case line+1 < len(e.lines):
			line++
			bs, sub = e.rawBreaks(line), 0
		default:
			return line, sub
		}
	}
	for ; n < 0; n++ {
		switch {
		case sub > 0:
			sub--
		case line > 0:
			line--
			bs = e.rawBreaks(line)
			sub = len(bs) - 1
		default:
			return line, sub
		}
	}
	return line, sub
}

// moveRows moves the cursor n screen rows down (up when n is negative),
// keeping its distance from the left of the row where it can.
func (e Editor) moveRows(n int) Editor {
	line, sub := e.focusLine(), e.focusSub()
	off := 0
	if line == e.cursorLine {
		off = max(e.cursorCol-e.rawBreaks(line)[sub], 0)
	}
	line, sub = e.stepRows(line, sub, n)
	bs := e.rawBreaks(line)
	e.cursorLine = line
	e.cursorCol = min(bs[sub]+off, markdown.RowEnd(bs, sub, runeLen(e.lines[line])))
	e.ensureCursorVisible()
	return e
}

// rowStart is the column where the row of the Focus Line that holds the
// cursor starts, and rowEnd the one where it ends.
func (e Editor) rowStart() int {
	bs := e.rawBreaks(e.cursorLine)
	return bs[markdown.RowOf(bs, e.cursorCol)]
}

func (e Editor) rowEnd() int {
	bs := e.rawBreaks(e.cursorLine)
	return markdown.RowEnd(bs, markdown.RowOf(bs, e.cursorCol), runeLen(e.currentLine()))
}

// wrapRowsText is used by tests: the text of every screen row, without styles.
func (e Editor) wrapRowsText() []string {
	var out []string
	for _, ref := range e.layout() {
		switch {
		case !ref.ok:
			out = append(out, "")
		case ref.focus && !e.wrap:
			out = append(out, e.lines[ref.line])
		case ref.focus:
			bs := e.rawBreaks(ref.line)
			rs := []rune(e.lines[ref.line])
			end := len(rs)
			if ref.sub+1 < len(bs) {
				end = bs[ref.sub+1]
			}
			out = append(out, string(rs[bs[ref.sub]:end]))
		default:
			var b strings.Builder
			for _, s := range ref.segs {
				b.WriteString(s.Text)
			}
			out = append(out, b.String())
		}
	}
	return out
}
