package markdown

import (
	"strings"
	"unicode/utf8"
)

// Align is the alignment of a table column.
type Align int

// Column alignments, from the ":" marks of the separator row.
const (
	AlignNone Align = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// Table describes a table found in a document: a header row, a separator row
// ("|---|:-:|") and the body rows that follow. The renderer draws it line by
// line, so every line needs the widths computed over the whole table.
type Table struct {
	Start, End int // first and last line, inclusive
	Aligns     []Align
	Widths     []int // width of the text of each column, in cells
}

// cellSpan is the trimmed text of a cell and its byte offset in the raw line.
type cellSpan struct {
	text string
	off  int
}

// hasPipe reports whether the line has a cell separator.
func hasPipe(s string) bool { return strings.Contains(s, "|") }

// isRowLine reports whether a line can belong to a table: it has a pipe and
// is not a fence, a heading or a quote.
func isRowLine(s string) bool {
	return hasPipe(s) && !isFenceLine(s) && !headingRe.MatchString(s) && !quoteRe.MatchString(s)
}

// splitRow splits a table row into its cells. The leading and trailing pipes
// are optional and "\|" does not separate.
func splitRow(raw string) []cellSpan {
	var bars []int
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' {
			i++
			continue
		}
		if raw[i] == '|' {
			bars = append(bars, i)
		}
	}
	start, end := 0, len(raw)
	if lead := len(raw) - len(strings.TrimLeft(raw, " \t")); len(bars) > 0 && bars[0] == lead {
		start = lead + 1
		bars = bars[1:]
	}
	if tail := len(strings.TrimRight(raw, " \t")); len(bars) > 0 && bars[len(bars)-1] == tail-1 {
		end = tail - 1
		bars = bars[:len(bars)-1]
	} else if len(bars) == 0 && start > 0 && tail <= start {
		return nil // just "|"
	}

	mk := func(a, b int) cellSpan {
		if a > b {
			a = b
		}
		seg := raw[a:b]
		trimmed := strings.TrimSpace(seg)
		return cellSpan{text: trimmed, off: a + len(seg) - len(strings.TrimLeft(seg, " \t"))}
	}
	var cells []cellSpan
	from := start
	for _, b := range bars {
		cells = append(cells, mk(from, b))
		from = b + 1
	}
	return append(cells, mk(from, end))
}

// parseSeparator reads a separator row ("|---|:-:|--:|") and returns the
// alignment of each column.
func parseSeparator(raw string) ([]Align, bool) {
	if !hasPipe(raw) {
		return nil, false
	}
	cells := splitRow(raw)
	if len(cells) == 0 {
		return nil, false
	}
	aligns := make([]Align, len(cells))
	for i, c := range cells {
		t := c.text
		left, right := strings.HasPrefix(t, ":"), strings.HasSuffix(t, ":")
		core := strings.Trim(t, ":")
		if core == "" || strings.Trim(core, "-") != "" {
			return nil, false
		}
		switch {
		case left && right:
			aligns[i] = AlignCenter
		case right:
			aligns[i] = AlignRight
		case left:
			aligns[i] = AlignLeft
		}
	}
	return aligns, true
}

// cellWidth is the width of a cell once rendered, that is without the
// inline markers.
func cellWidth(segs []Segment) int {
	w := 0
	for _, s := range segs {
		w += utf8.RuneCountInString(s.Text)
	}
	return w
}

// FindTable looks for the table that contains line idx. lines is the whole
// document and idx must not be inside a code block (the caller knows).
func FindTable(lines []string, idx int) (Table, bool) {
	if idx < 0 || idx >= len(lines) || !isRowLine(lines[idx]) {
		return Table{}, false
	}
	lo := idx
	for lo > 0 && isRowLine(lines[lo-1]) {
		lo--
	}
	for h := lo; h < len(lines)-1 && h <= idx; h++ {
		aligns, ok := parseSeparator(lines[h+1])
		if !ok || len(splitRow(lines[h])) != len(aligns) {
			continue
		}
		end := h + 1
		for end+1 < len(lines) && isRowLine(lines[end+1]) {
			end++
		}
		t := Table{Start: h, End: end, Aligns: aligns, Widths: make([]int, len(aligns))}
		for r := h; r <= end; r++ {
			if r == h+1 {
				continue
			}
			for k, c := range splitRow(lines[r]) {
				if k >= len(t.Widths) {
					break
				}
				if w := cellWidth(renderInline(c.text, StyleNone, c.off)); w > t.Widths[k] {
					t.Widths[k] = w
				}
			}
		}
		return t, true
	}
	return Table{}, false
}

// RenderTableLine draws line idx of table t: the borders, the cells padded to
// the width of their column according to its alignment, and for the separator
// row a line of "─". Cell contents keep their inline styles.
func RenderTableLine(lines []string, t Table, idx int) []Segment {
	raw := lines[idx]
	if idx == t.Start+1 {
		parts := make([]string, len(t.Widths))
		for k, w := range t.Widths {
			parts[k] = strings.Repeat("─", w+2)
		}
		return []Segment{{
			Text: "├" + strings.Join(parts, "┼") + "┤", Style: StyleTableBorder, RawFrom: 0, RawTo: len(raw),
		}}
	}

	base := StyleNone
	if idx == t.Start {
		base = StyleTableHead
	}
	cells := splitRow(raw)
	border := func(at int) Segment {
		return Segment{Text: "│", Style: StyleTableBorder, RawFrom: at, RawTo: at}
	}
	at := 0
	if len(cells) > 0 {
		at = cells[0].off
	}
	segs := []Segment{border(at)}
	for k, w := range t.Widths {
		c := cellSpan{off: len(raw)}
		if k < len(cells) {
			c = cells[k]
		}
		inline := renderInline(c.text, base, c.off)
		gap := w - cellWidth(inline)
		if gap < 0 {
			gap = 0
		}
		padL, padR := 0, gap
		switch t.Aligns[k] {
		case AlignRight:
			padL, padR = gap, 0
		case AlignCenter:
			padL, padR = gap/2, gap-gap/2
		}
		end := c.off + len(c.text)
		segs = append(segs, Segment{Text: " " + strings.Repeat(" ", padL), Style: base, RawFrom: c.off, RawTo: c.off})
		segs = append(segs, inline...)
		segs = append(segs, Segment{Text: strings.Repeat(" ", padR) + " ", Style: base, RawFrom: end, RawTo: end})
		segs = append(segs, border(end))
	}
	return segs
}
