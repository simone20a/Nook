package editor

import (
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss"

	"nook/internal/markdown"
	"nook/internal/theme"
)

// selectionOnLine returns the range of runes selected on line line,
// with the upper bound excluded. The second value is false when the line
// has nothing selected.
func (e Editor) selectionOnLine(line int) (from, to int, ok bool) {
	if !e.selecting || line < 0 || line >= len(e.lines) {
		return 0, 0, false
	}
	sl, sc, el, ec := e.selectedRange()
	if line < sl || line > el {
		return 0, 0, false
	}
	n := runeLen(e.lines[line])
	from, to = 0, n
	if line == sl {
		from = sc
	}
	if line == el {
		to = ec
	}
	if from < 0 {
		from = 0
	}
	if to > n {
		to = n
	}
	if from >= to {
		return 0, 0, false
	}
	return from, to, true
}

// SetSelection sets cursor and anchor in one go: it is used by dragging
// with the mouse, which sets the anchor where the button was pressed and brings the cursor to where it was
// dragged.
func (e Editor) SetSelection(anchorLine, anchorCol, line, col int) Editor {
	e.selecting = true
	e.anchorLine = clampLine(anchorLine, len(e.lines))
	e.anchorCol = anchorCol
	e.cursorLine = clampLine(line, len(e.lines))
	e.cursorCol = col
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// DragSelection is SetSelection for a drag with the mouse: the selection
// extends to (line, col) but the view stays still, so the Focus Line does not
// move while the button is held. The view is released by the next key, click
// or scroll.
func (e Editor) DragSelection(anchorLine, anchorCol, line, col int) Editor {
	focus, h := e.focusLine(), e.hScroll
	e = e.SetSelection(anchorLine, anchorCol, line, col)
	e.frozen, e.viewLine, e.hScroll = true, focus, h
	return e
}

// clampLine brings a line back within the limits of the document.
func clampLine(line, count int) int {
	if line < 0 {
		return 0
	}
	if line > count-1 {
		return count - 1
	}
	return line
}

// SelectedText returns the selected text, with lines separated by "\n".
// The second value is false when there is no selection.
func (e Editor) SelectedText() (string, bool) {
	if !e.selecting {
		return "", false
	}
	sl, sc, el, ec := e.selectedRange()
	if sl == el && sc == ec {
		return "", false
	}
	if sl < 0 || el >= len(e.lines) {
		return "", false
	}
	if sl == el {
		runes := []rune(e.lines[sl])
		return string(runes[minInt(sc, len(runes)):minInt(ec, len(runes))]), true
	}
	var parts []string
	first := []rune(e.lines[sl])
	parts = append(parts, string(first[minInt(sc, len(first)):]))
	for i := sl + 1; i < el; i++ {
		parts = append(parts, e.lines[i])
	}
	last := []rune(e.lines[el])
	parts = append(parts, string(last[:minInt(ec, len(last))]))
	return strings.Join(parts, "\n"), true
}

// minInt returns the smaller of the two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runeRangeToBytes translates the rune range [from, to) of the string s into the
// corresponding byte offsets.
func runeRangeToBytes(s string, from, to int) (int, int) {
	rs := []rune(s)
	if from < 0 {
		from = 0
	}
	if from > len(rs) {
		from = len(rs)
	}
	if to < from {
		to = from
	}
	if to > len(rs) {
		to = len(rs)
	}
	return len(string(rs[:from])), len(string(rs[:to]))
}

// renderSegmentWithSelection renders a markdown segment with its palette,
// highlighting the part that falls inside the selected range of the line,
// expressed as byte offsets on the raw text [selFrom, selTo).
//
// The correspondence between raw bytes and visible characters of the segment is
// proportional: markdown markers do not appear on screen, so the text of the
// segment is shorter than its source span and there is no exact
// character-by-character map.
func renderSegmentWithSelection(seg markdown.Segment, selFrom, selTo int, st lipgloss.Style) string {
	start, end, ok := segmentOverlap(seg, selFrom, selTo)
	if !ok {
		return st.Render(seg.Text)
	}
	runes := []rune(seg.Text)
	return st.Render(string(runes[:start])) +
		theme.Selection.Render(string(runes[start:end])) +
		st.Render(string(runes[end:]))
}

// segmentOverlap translates the overlap between the source span of the
// segment and the selection into rune indices inside seg.Text.
func segmentOverlap(seg markdown.Segment, selFrom, selTo int) (start, end int, ok bool) {
	a, b := seg.RawFrom, seg.RawTo
	if b <= a || selTo <= a || selFrom >= b {
		return 0, 0, false
	}
	runes := []rune(seg.Text)
	n := len(runes)
	if n == 0 {
		return 0, 0, false
	}
	lo, hi := selFrom, selTo
	if lo < a {
		lo = a
	}
	if hi > b {
		hi = b
	}
	if lo >= hi {
		return 0, 0, false
	}
	span := b - a
	start = (lo - a) * n / span
	end = (hi - a) * n / span
	if end <= start {
		// Overlap too small for a character: the nearest one is highlighted
		// anyway, so the selection stays visible even inside the
		// markers.
		if start < n {
			end = start + 1
		} else {
			start, end = n-1, n
		}
	}
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start >= end {
		return 0, 0, false
	}
	return start, end, true
}
