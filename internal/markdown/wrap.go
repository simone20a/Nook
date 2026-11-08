package markdown

import (
	"strings"
	"unicode/utf8"
)

// Word wrap. A line of text is cut into rows of at most width cells (a cell
// is a rune, as everywhere in the renderer). The cut falls after the last
// space that fits; a word longer than a row is cut where the row ends. A
// row may end with one space more than its width: the space is never drawn.

// Breaks returns the index of the rune where each row of rs starts, always
// beginning with 0, for rows of at most w cells.
func Breaks(rs []rune, w int) []int { return breaks(rs, w, w) }

// breaks is Breaks with a first row of w0 cells, for text that follows a
// prefix on its first row only.
func breaks(rs []rune, w0, w int) []int {
	w0, w = max(w0, 1), max(w, 1)
	starts := []int{0}
	start, width := 0, w0
	for {
		rem := len(rs) - start
		if rem <= width || (rem == width+1 && rs[len(rs)-1] == ' ') {
			return starts
		}
		end := start + width
		for p := start + width; p > start; p-- {
			if rs[p] == ' ' {
				end = p + 1
				break
			}
		}
		starts = append(starts, end)
		start, width = end, w
	}
}

// RowOf returns the row of the text, split at the given breaks, that holds
// the rune col. The position just after the last rune belongs to the last row.
func RowOf(breaks []int, col int) int {
	row := 0
	for i, b := range breaks {
		if b <= col {
			row = i
		}
	}
	return row
}

// RowEnd returns the index after the last rune of a row that a cursor can
// stand on: the next row's start minus one, or n for the last row.
func RowEnd(breaks []int, row, n int) int {
	if row+1 < len(breaks) {
		return breaks[row+1] - 1
	}
	return n
}

// sliceSegments returns the part of segs that covers the runes [from, to) of
// their joined text. A segment that shows its raw text unchanged keeps an
// exact raw span; a synthesised one (a link label, a bullet) keeps the
// whole span of the original in every piece.
func sliceSegments(segs []Segment, from, to int) []Segment {
	var out []Segment
	pos := 0
	for _, s := range segs {
		rs := []rune(s.Text)
		a, b := pos, pos+len(rs)
		pos = b
		lo, hi := max(a, from), min(b, to)
		if lo >= hi {
			continue
		}
		piece := Segment{Text: string(rs[lo-a : hi-a]), Style: s.Style, RawFrom: s.RawFrom, RawTo: s.RawTo}
		if len(s.Text) == s.RawTo-s.RawFrom {
			piece.RawFrom = s.RawFrom + len(string(rs[:lo-a]))
			piece.RawTo = piece.RawFrom + len(piece.Text)
		}
		out = append(out, piece)
	}
	return out
}

// trimSpaces removes the spaces at the end of a row.
func trimSpaces(row []Segment) []Segment {
	for len(row) > 0 {
		last := row[len(row)-1]
		trimmed := strings.TrimRight(last.Text, " ")
		if trimmed == last.Text {
			break
		}
		if len(last.Text) == last.RawTo-last.RawFrom {
			last.RawTo -= len(last.Text) - len(trimmed)
		}
		if trimmed == "" {
			row = row[:len(row)-1]
			continue
		}
		last.Text = trimmed
		row[len(row)-1] = last
		break
	}
	return row
}

// joined returns the runes of the text of segs.
func joined(segs []Segment) []rune {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return []rune(b.String())
}

// wrapPlain cuts segments with no prefix into rows of at most w cells.
func wrapPlain(segs []Segment, w int) [][]Segment {
	rs := joined(segs)
	bs := Breaks(rs, w)
	rows := make([][]Segment, len(bs))
	for i, b := range bs {
		end := len(rs)
		if i+1 < len(bs) {
			end = bs[i+1]
		}
		rows[i] = trimSpaces(sliceSegments(segs, b, end))
	}
	return rows
}

// WrapSegments cuts the segments of one rendered line into rows of at most
// width cells. A quote keeps its bar on every row and a list item or a
// numbered item keeps its text under the first row's text. Rules, fences and
// code lines are never cut: they come back as a single row.
func WrapSegments(segs []Segment, width int) [][]Segment {
	one := [][]Segment{segs}
	if width < 1 {
		return one
	}
	for _, s := range segs {
		switch s.Style {
		case StyleRule, StyleFence, StyleCodeBlock:
			return one
		}
	}

	// The prefix is the bar, bullet or checkbox the renderer put in front.
	n := 0
	for n < len(segs) {
		st := segs[n].Style
		if st != StyleQuoteBar && st != StyleBullet && st != StyleCheckbox {
			break
		}
		n++
	}
	prefix, body := segs[:n], segs[n:]
	prefixW := len(joined(prefix))

	rs := joined(body)
	indentW := prefixW
	if n == 0 {
		if m := orderedRe.FindString(string(rs)); m != "" {
			indentW += utf8.RuneCountInString(m)
		}
	}
	if width-indentW < 4 {
		return one // no room to wrap: the row is cut by the renderer
	}
	if prefixW == 0 && indentW == 0 {
		return wrapPlain(body, width)
	}

	bs := breaks(rs, width-prefixW, width-indentW)
	rows := make([][]Segment, len(bs))
	for i, b := range bs {
		end := len(rs)
		if i+1 < len(bs) {
			end = bs[i+1]
		}
		part := sliceSegments(body, b, end)
		if i == 0 {
			rows[i] = trimSpaces(append(append([]Segment{}, prefix...), part...))
			continue
		}
		at := 0
		if len(part) > 0 {
			at = part[0].RawFrom
		}
		rows[i] = trimSpaces(append(indentSegments(prefix, indentW, at), part...))
	}
	return rows
}

// indentSegments is what starts a continuation row: the bar again for a
// quote, spaces under the marker of a list item. Its raw span is empty and
// sits at the start of the row's text, so a click there lands on that text.
func indentSegments(prefix []Segment, w, at int) []Segment {
	if len(prefix) > 0 && prefix[0].Style == StyleQuoteBar {
		return []Segment{{Text: prefix[0].Text, Style: StyleQuoteBar, RawFrom: at, RawTo: at}}
	}
	return []Segment{{Text: strings.Repeat(" ", w), Style: StyleNone, RawFrom: at, RawTo: at}}
}
