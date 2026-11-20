package markdown

import (
	"reflect"
	"strings"
	"testing"
)

// rowTexts returns the text of every row.
func rowTexts(rows [][]Segment) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = joinText(r)
	}
	return out
}

func TestBreaks(t *testing.T) {
	cases := []struct {
		text string
		w    int
		want []int
	}{
		{"", 5, []int{0}},
		{"short", 5, []int{0}},
		{"hello world foo", 8, []int{0, 6, 12}},
		{"abcdefghij", 4, []int{0, 4, 8}},
		{"abcd efgh", 4, []int{0, 5}}, // the space after a full row goes with it
		{"ab ", 2, []int{0}},          // a trailing space may overflow
		{"a  b", 2, []int{0, 3}},
	}
	for _, c := range cases {
		if got := Breaks([]rune(c.text), c.w); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Breaks(%q, %d) = %v, want %v", c.text, c.w, got, c.want)
		}
	}
	// Every row fits and the rows cover the text.
	text := []rune("the quick brown fox jumps over the lazy dog and keeps running far away")
	for w := 1; w < 30; w++ {
		bs := Breaks(text, w)
		for i, b := range bs {
			end := len(text)
			if i+1 < len(bs) {
				end = bs[i+1]
			}
			if end <= b {
				t.Fatalf("w=%d: empty row %d in %v", w, i, bs)
			}
			if n := len(strings.TrimRight(string(text[b:end]), " ")); n > w {
				t.Fatalf("w=%d: row %d is %d cells", w, i, n)
			}
		}
	}
}

func TestWrapParagraph(t *testing.T) {
	raw := "hello **big** world, this is long"
	rows := WrapSegments(RenderLine(raw, false), 12)
	want := []string{"hello big", "world, this", "is long"}
	if got := rowTexts(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	// A click on "w" of "world" lands on the raw "w".
	if got := MapCellToRaw(rows[1], 0); got != strings.Index(raw, "world") {
		t.Errorf("click on row 1 = %d, want %d", got, strings.Index(raw, "world"))
	}
	// "big" is the bold content: its second row piece keeps exact offsets.
	if got := MapCellToRaw(rows[0], 7); got != strings.Index(raw, "ig") {
		t.Errorf("click on 'i' = %d, want %d", got, strings.Index(raw, "ig"))
	}
}

func TestWrapKeepsStylesAcrossRows(t *testing.T) {
	rows := WrapSegments(RenderLine("**aaa bbb ccc ddd**", false), 8)
	if len(rows) < 2 {
		t.Fatalf("rows = %q", rowTexts(rows))
	}
	for i, r := range rows {
		for _, s := range r {
			if s.Style != StyleBold {
				t.Errorf("row %d: style %v, want bold", i, s.Style)
			}
		}
	}
}

func TestWrapListHangingIndent(t *testing.T) {
	rows := WrapSegments(RenderLine("- alpha beta gamma delta", false), 14)
	want := []string{"• alpha beta", "  gamma delta"}
	if got := rowTexts(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	// A click in the indent lands on the start of that row's text.
	if got := MapCellToRaw(rows[1], 0); got != strings.Index("- alpha beta gamma delta", "gamma") {
		t.Errorf("click in the indent = %d", got)
	}
}

func TestWrapQuoteRepeatsBar(t *testing.T) {
	rows := WrapSegments(RenderLine("> one two three four", false), 12)
	got := rowTexts(rows)
	if len(got) < 2 {
		t.Fatalf("rows = %q", got)
	}
	for i, r := range got {
		if !strings.HasPrefix(r, "▌ ") {
			t.Errorf("row %d = %q, want it to start with the bar", i, r)
		}
	}
}

func TestWrapOrderedList(t *testing.T) {
	rows := WrapSegments(RenderLine("1. first second third fourth", false), 14)
	got := rowTexts(rows)
	if len(got) < 2 || !strings.HasPrefix(got[1], "   ") || strings.HasPrefix(got[1], "    ") {
		t.Errorf("rows = %q, want continuation indented 3 cells", got)
	}
}

func TestWrapNeverCutsBlocks(t *testing.T) {
	long := strings.Repeat("x ", 40)
	for _, segs := range [][]Segment{
		RenderLine(long, true),          // code line
		RenderLine("```go", false),      // fence
		RenderLine("----------", false), // rule
	} {
		if n := len(WrapSegments(segs, 10)); n != 1 {
			t.Errorf("block cut into %d rows", n)
		}
	}
	if n := len(WrapSegments(RenderLine("text", false), 0)); n != 1 {
		t.Errorf("width 0 gave %d rows", n)
	}
	if n := len(WrapSegments(RenderLine("- some long text here", false), 5)); n != 1 {
		t.Errorf("no room for the indent gave %d rows", n)
	}
}

func TestWrapEmptyLine(t *testing.T) {
	rows := WrapSegments(RenderLine("", false), 10)
	if len(rows) != 1 {
		t.Errorf("empty line gave %d rows", len(rows))
	}
}

var wideTable = []string{
	"| Name | Description |",
	"|---|---|",
	"| apple | a round fruit that grows on trees |",
	"| kiwi | small |",
}

func TestTableWidthsNarrowTheWidest(t *testing.T) {
	tbl, _ := FindTable(wideTable, 0)
	if w, ok := TableWidths(tbl, 0); !ok || !reflect.DeepEqual(w, tbl.Widths) {
		t.Errorf("no limit: %v, %v", w, ok)
	}
	w, ok := TableWidths(tbl, 30)
	if !ok {
		t.Fatal("did not fit")
	}
	if total := tableOverhead(2) + w[0] + w[1]; total != 30 {
		t.Errorf("widths %v take %d cells, want 30", w, total)
	}
	if w[0] != tbl.Widths[0] {
		t.Errorf("the short column was narrowed: %v", w)
	}
	// Too narrow even for the minimum: no solution.
	if _, ok := TableWidths(tbl, 8); ok {
		t.Error("an impossible width was accepted")
	}
}

func TestRenderTableRowsWraps(t *testing.T) {
	tbl, _ := FindTable(wideTable, 0)
	// Fits: one row per line, as RenderTableLine.
	for i := range wideTable {
		rows := RenderTableRows(wideTable, tbl, i, 80)
		if len(rows) != 1 || joinText(rows[0]) != joinText(RenderTableLine(wideTable, tbl, i)) {
			t.Errorf("line %d changed when it fits: %q", i, rowTexts(rows))
		}
	}
	rows := RenderTableRows(wideTable, tbl, 2, 30)
	texts := rowTexts(rows)
	if len(texts) < 2 {
		t.Fatalf("not wrapped: %q", texts)
	}
	for i, r := range texts {
		if n := len([]rune(r)); n != 30 {
			t.Errorf("row %d is %d cells: %q", i, n, r)
		}
		if !strings.HasPrefix(r, "│ ") || !strings.HasSuffix(r, " │") {
			t.Errorf("row %d lacks borders: %q", i, r)
		}
	}
	if !strings.Contains(texts[0], "apple") || strings.Contains(texts[1], "apple") {
		t.Errorf("first column must show on the first row only: %q", texts)
	}
	// The separator follows the narrowed widths.
	sep := joinText(RenderTableRows(wideTable, tbl, 1, 30)[0])
	if len([]rune(sep)) != 30 {
		t.Errorf("separator is %d cells: %q", len([]rune(sep)), sep)
	}
	// Words that cannot fit make the line one row, cut by the caller.
	if rows := RenderTableRows(wideTable, tbl, 2, 8); len(rows) != 1 {
		t.Errorf("impossible width gave %d rows", len(rows))
	}
}

func TestWrappedTableCellsKeepOffsets(t *testing.T) {
	tbl, _ := FindTable(wideTable, 0)
	raw := wideTable[2]
	rows := RenderTableRows(wideTable, tbl, 2, 30)
	// Find the cell on row 1 that holds the second piece of the description
	// and check a click on its first letter lands on that raw letter.
	text := joinText(rows[1])
	word := strings.Fields(strings.Trim(text, "│ "))[0]
	cell := strings.Index(text, word)
	cell = len([]rune(text[:cell]))
	got := MapCellToRaw(rows[1], cell)
	if got < 0 || got >= len(raw) || !strings.HasPrefix(raw[got:], word) {
		t.Errorf("click on %q (cell %d) = raw %d (%q)", word, cell, got, raw[min(got, len(raw)):])
	}
}
