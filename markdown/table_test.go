package markdown

import (
	"strings"
	"testing"
)

func joinText(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

var sampleTable = []string{
	"| Name | Qty |",
	"|---|--:|",
	"| apple | 3 |",
	"| kiwi | 12 |",
	"",
	"text",
}

// TestFindTable checks the detection of the table, its extent, the
// alignments and the column widths.
func TestFindTable(t *testing.T) {
	for idx := 0; idx <= 3; idx++ {
		tbl, ok := FindTable(sampleTable, idx)
		if !ok || tbl.Start != 0 || tbl.End != 3 {
			t.Fatalf("FindTable(idx=%d) = %+v, %v; want a table from 0 to 3", idx, tbl, ok)
		}
	}
	tbl, _ := FindTable(sampleTable, 2)
	if len(tbl.Widths) != 2 || tbl.Widths[0] != 5 || tbl.Widths[1] != 3 {
		t.Errorf("Widths = %v, want [5 3]", tbl.Widths)
	}
	if tbl.Aligns[0] != AlignNone || tbl.Aligns[1] != AlignRight {
		t.Errorf("Aligns = %v, want [none right]", tbl.Aligns)
	}
	for _, idx := range []int{4, 5} {
		if _, ok := FindTable(sampleTable, idx); ok {
			t.Errorf("FindTable(idx=%d) found a table outside it", idx)
		}
	}
}

// TestNotATable checks that text with pipes is not a table without a
// matching separator row.
func TestNotATable(t *testing.T) {
	cases := [][]string{
		{"a | b", "text"},
		{"a|b", "---"},
		{"a | b | c", "|---|---|"}, // 3 columns, separator with 2
		{"# a | b", "|---|---|"},   // a heading
		{"| a |"},
	}
	for _, lines := range cases {
		for idx := range lines {
			if _, ok := FindTable(lines, idx); ok {
				t.Errorf("FindTable(%q, %d) found a table", lines, idx)
			}
		}
	}
}

// TestRenderTableLine checks the drawing of header, separator and body rows
// with alignment.
func TestRenderTableLine(t *testing.T) {
	tbl, _ := FindTable(sampleTable, 0)
	want := []string{
		"│ Name  │ Qty │",
		"├───────┼─────┤",
		"│ apple │   3 │",
		"│ kiwi  │  12 │",
	}
	for i, w := range want {
		if got := joinText(RenderTableLine(sampleTable, tbl, i)); got != w {
			t.Errorf("row %d = %q, want %q", i, got, w)
		}
	}
}

// TestTableCenterAndInline checks the centred alignment, the rows with
// missing cells and that inline markers do not count in the width.
func TestTableCenterAndInline(t *testing.T) {
	lines := []string{"a | b", ":-:|:-:", "**xx** |", "y"}
	tbl, ok := FindTable(lines, 2)
	if !ok {
		t.Fatal("table not found")
	}
	if tbl.End != 2 {
		t.Fatalf("End = %d, want 2 (the last line has no pipe)", tbl.End)
	}
	if tbl.Widths[0] != 2 || tbl.Widths[1] != 1 {
		t.Fatalf("Widths = %v, want [2 1]", tbl.Widths)
	}
	want := []string{"│ a  │ b │", "├────┼───┤", "│ xx │   │"}
	for i, w := range want {
		if got := joinText(RenderTableLine(lines, tbl, i)); got != w {
			t.Errorf("row %d = %q, want %q", i, got, w)
		}
	}
}

// TestTableSegmentOffsets checks that the content of a cell keeps the offsets
// of the raw text, so that the mouse lands on the right character.
func TestTableSegmentOffsets(t *testing.T) {
	tbl, _ := FindTable(sampleTable, 0)
	raw := sampleTable[2] // "| apple | 3 |"
	segs := RenderTableLine(sampleTable, tbl, 2)
	// "│ apple │   3 │": the cell at display column 2 is the "a" of apple.
	if got := MapCellToRaw(segs, 2); got != strings.Index(raw, "apple") {
		t.Errorf("MapCellToRaw(2) = %d, want %d", got, strings.Index(raw, "apple"))
	}
	// Display column 12 is the "3" in the right-aligned cell.
	if got := MapCellToRaw(segs, 12); got != strings.Index(raw, "3") {
		t.Errorf("MapCellToRaw(12) = %d, want %d", got, strings.Index(raw, "3"))
	}
}
