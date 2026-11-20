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
	Words      []int // width of the longest word of each column
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
		t := Table{Start: h, End: end, Aligns: aligns, Widths: make([]int, len(aligns)), Words: make([]int, len(aligns))}
		for r := h; r <= end; r++ {
			if r == h+1 {
				continue
			}
			for k, c := range splitRow(lines[r]) {
				if k >= len(t.Widths) {
					break
				}
				text := renderInline(c.text, StyleNone, c.off)
				if w := cellWidth(text); w > t.Widths[k] {
					t.Widths[k] = w
				}
				for _, word := range strings.Fields(string(joined(text))) {
					if w := utf8.RuneCountInString(word); w > t.Words[k] {
						t.Words[k] = w
					}
				}
			}
		}
		return t, true
	}
	return Table{}, false
}

// RenderTableLine draws line idx of table t on a single row: the borders, the
// cells padded to the width of their column according to its alignment, and
// for the separator row a line of "─". Cell contents keep their inline styles.
func RenderTableLine(lines []string, t Table, idx int) []Segment {
	return tableRows(lines, t, idx, t.Widths)[0]
}

// tableOverhead is the width of the borders and the blanks around the cells
// of a table with n columns: "│ " before each cell, " " after it, and the last "│".
func tableOverhead(n int) int { return 3*n + 1 }

// TableWidths returns the column widths to draw t in width cells. They are
// the natural ones when the table fits; otherwise the widest columns are
// narrowed one cell at a time, not below the longest word of the column (up
// to 12 cells) when that is possible, not below 3 cells otherwise. ok is
// false when even that does not fit. A width of 0 or less means no limit.
func TableWidths(t Table, width int) (widths []int, ok bool) {
	natural := append([]int(nil), t.Widths...)
	avail := width - tableOverhead(len(natural))
	sum := func(w []int) int {
		n := 0
		for _, v := range w {
			n += v
		}
		return n
	}
	if width <= 0 || sum(natural) <= avail {
		return natural, true
	}
	for _, minWord := range []bool{true, false} {
		floor := make([]int, len(natural))
		for k, nat := range natural {
			floor[k] = min(nat, 3)
			if minWord && k < len(t.Words) {
				floor[k] = min(nat, max(3, min(t.Words[k], 12)))
			}
		}
		if sum(floor) > avail {
			continue
		}
		w := append([]int(nil), natural...)
		for sum(w) > avail {
			widest := -1
			for k := range w {
				if w[k] > floor[k] && (widest < 0 || w[k] > w[widest]) {
					widest = k
				}
			}
			w[widest]--
		}
		return w, true
	}
	return nil, false
}

// RenderTableRows draws line idx of table t in at most width cells. When the
// table is wider, its columns are narrowed (see TableWidths) and the text of
// a cell goes on to further rows inside its column, so a table line may take
// several rows; if the columns cannot be narrowed enough the line is drawn on
// one row, which the caller cuts. A width of 0 or less means no limit.
func RenderTableRows(lines []string, t Table, idx int, width int) [][]Segment {
	widths, ok := TableWidths(t, width)
	if !ok {
		return [][]Segment{RenderTableLine(lines, t, idx)}
	}
	return tableRows(lines, t, idx, widths)
}

// tableRows draws line idx with the given column widths, wrapping the cells.
func tableRows(lines []string, t Table, idx int, widths []int) [][]Segment {
	raw := lines[idx]
	if idx == t.Start+1 {
		parts := make([]string, len(widths))
		for k, w := range widths {
			parts[k] = strings.Repeat("─", w+2)
		}
		return [][]Segment{{{
			Text: "├" + strings.Join(parts, "┼") + "┤", Style: StyleTableBorder, RawFrom: 0, RawTo: len(raw),
		}}}
	}

	base := StyleNone
	if idx == t.Start {
		base = StyleTableHead
	}
	cells := splitRow(raw)
	border := func(at int) Segment {
		return Segment{Text: "│", Style: StyleTableBorder, RawFrom: at, RawTo: at}
	}
	start := 0
	if len(cells) > 0 {
		start = cells[0].off
	}

	// Every cell is cut into rows of its column's width; the line is as tall
	// as its tallest cell.
	parts := make([][][]Segment, len(widths))
	height := 1
	for k, w := range widths {
		c := cellSpan{off: len(raw)}
		if k < len(cells) {
			c = cells[k]
		}
		parts[k] = wrapPlain(renderInline(c.text, base, c.off), w)
		height = max(height, len(parts[k]))
	}

	out := make([][]Segment, height)
	for j := range out {
		segs := []Segment{border(start)}
		for k, w := range widths {
			c := cellSpan{off: len(raw)}
			if k < len(cells) {
				c = cells[k]
			}
			var part []Segment
			if j < len(parts[k]) {
				part = parts[k][j]
			}
			gap := max(w-cellWidth(part), 0)
			padL, padR := 0, gap
			switch t.Aligns[k] {
			case AlignRight:
				padL, padR = gap, 0
			case AlignCenter:
				padL, padR = gap/2, gap-gap/2
			}
			end := c.off + len(c.text)
			segs = append(segs, Segment{Text: " " + strings.Repeat(" ", padL), Style: base, RawFrom: c.off, RawTo: c.off})
			segs = append(segs, part...)
			segs = append(segs, Segment{Text: strings.Repeat(" ", padR) + " ", Style: base, RawFrom: end, RawTo: end})
			segs = append(segs, border(end))
		}
		out[j] = segs
	}
	return out
}
