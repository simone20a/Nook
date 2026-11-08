package editor

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// wrapDoc has a heading, two long lines and a short one. At width 12 the
// raw text of line 1 takes the rows "alpha beta ", "gamma delta ", "epsilon".
var wrapDoc = []string{
	"# Title",
	"alpha beta gamma delta epsilon",
	"second line of text that wraps",
	"short",
}

func wrapEditor(width, height int) Editor {
	return New(append([]string(nil), wrapDoc...), width, height).WithWrap(true)
}

func TestWrapLayoutFocusLineRaw(t *testing.T) {
	e := wrapEditor(12, 9).SetCursor(1, 0) // focus row is 4
	got := e.wrapRowsText()
	if got[4] != "alpha beta " || got[5] != "gamma delta " || got[6] != "epsilon" {
		t.Fatalf("focus line rows = %q", got[4:7])
	}
	if got[3] != "Title" {
		t.Errorf("row above = %q, want the rendered heading", got[3])
	}
	if got[0] != "" || got[1] != "" || got[2] != "" {
		t.Errorf("rows past the start = %q", got[:3])
	}
	if got[7] != "second line" || got[8] != "of text that" {
		t.Errorf("rows below = %q", got[7:])
	}
}

func TestWrapCursorRowStaysInTheMiddle(t *testing.T) {
	e := wrapEditor(12, 9).SetCursor(1, 25) // in "epsilon": the third row of the line
	got := e.wrapRowsText()
	if got[4] != "epsilon" || got[3] != "gamma delta " || got[2] != "alpha beta " || got[1] != "Title" {
		t.Errorf("rows = %q", got)
	}
}

func TestWrapMoveByRows(t *testing.T) {
	e := wrapEditor(12, 9).SetCursor(1, 25) // row 2 of line 1, 2 cells in
	steps := []struct{ line, col int }{{1, 13}, {1, 2}, {0, 2}}
	for i, want := range steps {
		e = e.moveUp()
		if e.cursorLine != want.line || e.cursorCol != want.col {
			t.Fatalf("up #%d: cursor (%d,%d), want (%d,%d)", i+1, e.cursorLine, e.cursorCol, want.line, want.col)
		}
	}
	e = e.moveUp() // already at the start of the document
	if e.cursorLine != 0 || e.cursorCol != 2 {
		t.Errorf("up at the top moved to (%d,%d)", e.cursorLine, e.cursorCol)
	}
	e = wrapEditor(12, 9).SetCursor(1, 25)
	e = e.moveDown() // last row of the line: the next line, first row
	if e.cursorLine != 2 || e.cursorCol != 2 {
		t.Errorf("down moved to (%d,%d), want (2,2)", e.cursorLine, e.cursorCol)
	}
}

func TestWrapHomeEndAndScroll(t *testing.T) {
	e := wrapEditor(12, 9).SetCursor(1, 13) // second row, which starts at 11 and ends at 22
	if got := e.moveHome(); got.cursorCol != 11 {
		t.Errorf("home = %d, want 11", got.cursorCol)
	}
	if got := e.moveEnd(); got.cursorCol != 22 {
		t.Errorf("end on a row = %d, want 22", got.cursorCol)
	}
	if got := e.SetCursor(1, 25).moveEnd(); got.cursorCol != 30 {
		t.Errorf("end on the last row = %d, want 30", got.cursorCol)
	}
	e = wrapEditor(12, 9).SetCursor(1, 0)
	if got := e.Scroll(1); got.cursorLine != 1 || got.cursorCol != 11 {
		t.Errorf("scroll 1 row: (%d,%d), want (1,11)", got.cursorLine, got.cursorCol)
	}
	if got := e.Scroll(3); got.cursorLine != 2 || got.cursorCol != 0 {
		t.Errorf("scroll 3 rows: (%d,%d), want (2,0)", got.cursorLine, got.cursorCol)
	}
	if got := e.Scroll(-10); got.cursorLine != 0 {
		t.Errorf("scroll past the start: line %d", got.cursorLine)
	}
	if got := e.Scroll(100); got.cursorLine != 3 {
		t.Errorf("scroll past the end: line %d", got.cursorLine)
	}
}

func TestWrapHitCell(t *testing.T) {
	e := wrapEditor(12, 9).SetCursor(1, 0)
	cases := []struct{ y, x, line, col int }{
		{4, 3, 1, 3},   // first row of the Focus Line
		{5, 3, 1, 14},  // second row: starts at 11
		{6, 40, 1, 30}, // past the end of the last row
		{7, 0, 2, 0},   // rendered line below
		{8, 0, 2, 12},  // its second row starts at rune 12
		{3, 0, 0, 2},   // the heading, rendered: "Title" starts after "# "
	}
	for _, c := range cases {
		line, col, ok := e.HitCell(c.y, c.x)
		if !ok || line != c.line || col != c.col {
			t.Errorf("HitCell(%d,%d) = (%d,%d,%v), want (%d,%d)", c.y, c.x, line, col, ok, c.line, c.col)
		}
	}
	if _, _, ok := e.HitCell(0, 0); ok {
		t.Error("a row past the start of the document hit a line")
	}
	if line, ok := e.LineAtRow(5); !ok || line != 1 {
		t.Errorf("LineAtRow(5) = %d, %v", line, ok)
	}
	if got := e.FocusCellToCol(3); got != 3 {
		t.Errorf("FocusCellToCol = %d", got)
	}
	if got := e.SetCursor(1, 25).FocusCellToCol(3); got != 26 {
		t.Errorf("FocusCellToCol on the third row = %d, want 26", got)
	}
}

func TestWrapViewMatchesLayout(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	e := wrapEditor(12, 9).SetCursor(1, 0)
	view := strings.Split(ansi.Strip(e.View()), "\n")
	want := e.wrapRowsText()
	if len(view) != 9 {
		t.Fatalf("view has %d rows", len(view))
	}
	for r := range view {
		if strings.TrimRight(view[r], " ") != strings.TrimRight(want[r], " ") {
			t.Errorf("row %d = %q, want %q", r, view[r], want[r])
		}
		if n := ansi.StringWidth(view[r]); n > 12 {
			t.Errorf("row %d is %d cells wide", r, n)
		}
	}
}

func TestWrapCursorDrawnOnItsRowOnly(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	e := wrapEditor(12, 9).SetCursor(1, 13) // second row
	rows := strings.Split(e.View(), "\n")
	rev := "\x1b[7m"
	for r, row := range rows {
		has := strings.Contains(row, rev)
		if want := r == 4; has != want {
			t.Errorf("row %d: cursor drawn = %v, want %v", r, has, want)
		}
	}
	// And at the very end of the text the cursor sits after the last letter.
	e = wrapEditor(12, 9).SetCursor(1, 30)
	if row := strings.Split(e.View(), "\n")[4]; !strings.Contains(row, rev) {
		t.Errorf("no cursor at the end of the line: %q", row)
	}
}

func TestWrapSelectionAcrossRows(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	plain := strings.Split(wrapEditor(12, 9).SetCursor(1, 0).View(), "\n")
	e := wrapEditor(12, 9).SetCursor(1, 0).SetSelection(1, 0, 2, 30)
	sel := strings.Split(e.View(), "\n")
	for _, r := range []int{4, 5, 6, 7, 8} {
		if sel[r] == plain[r] {
			t.Errorf("row %d shows no selection", r)
		}
	}
}

func TestWrapDragKeepsTheView(t *testing.T) {
	e := wrapEditor(12, 9).SetCursor(1, 0)
	before := e.wrapRowsText()
	e = e.DragSelection(1, 0, 3, 2)
	if got := e.wrapRowsText(); !reflect.DeepEqual(got, before) {
		t.Errorf("the view moved while dragging:\n%q\n%q", got, before)
	}
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyUp}) // any key ends it
	e = m.(Editor)
	if reflect.DeepEqual(e.wrapRowsText(), before) {
		t.Error("the view stayed frozen after a key")
	}
}

func TestWrapToggleWithCtrlW(t *testing.T) {
	for _, vim := range []bool{false, true} {
		e := New(append([]string(nil), wrapDoc...), 12, 9)
		if vim {
			e = e.WithVim()
		}
		m, _ := e.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
		e = m.(Editor)
		if !e.Wrap() || !e.WrapChanged {
			t.Fatalf("vim=%v: ctrl+w: wrap %v, changed %v", vim, e.Wrap(), e.WrapChanged)
		}
		e.WrapChanged = false
		m, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
		e = m.(Editor)
		if e.Wrap() || !e.WrapChanged {
			t.Fatalf("vim=%v: second ctrl+w: wrap %v", vim, e.Wrap())
		}
		if e.Dirty {
			t.Errorf("vim=%v: ctrl+w changed the text", vim)
		}
	}
}

func TestWrapOffKeepsOneRowPerLine(t *testing.T) {
	e := New(append([]string(nil), wrapDoc...), 12, 9).SetCursor(1, 0)
	rows := e.wrapRowsText()
	if rows[4] != "alpha beta gamma delta epsilon" {
		t.Errorf("focus row with wrap off = %q", rows[4])
	}
	if e.hScroll != 0 {
		t.Errorf("hScroll = %d", e.hScroll)
	}
	e = e.WithWrap(true).WithWrap(false).moveEnd()
	if e.hScroll == 0 {
		t.Error("horizontal scroll does not work again after turning wrap off")
	}
}

func TestWrapTableCellsInEditor(t *testing.T) {
	lines := []string{
		"intro",
		"| Name | Description |",
		"|---|---|",
		"| apple | a round fruit that grows on trees |",
		"| kiwi | small |",
	}
	e := New(lines, 30, 15).WithWrap(true).SetCursor(0, 0) // focus row 7
	rows := e.wrapRowsText()
	if rows[7] != "intro" {
		t.Fatalf("focus row = %q", rows[7])
	}
	rest := rows[8:]
	if len(rest) < 5 {
		t.Fatalf("rows = %q", rest)
	}
	for i, r := range rest {
		if r == "" {
			continue
		}
		if n := len([]rune(r)); n != 30 {
			t.Errorf("table row %d is %d cells: %q", i, n, r)
		}
	}
	// "apple" row wraps its description: more rows than lines.
	nonEmpty := 0
	for _, r := range rest {
		if r != "" {
			nonEmpty++
		}
	}
	if nonEmpty <= 4 {
		t.Errorf("the table did not wrap: %q", rest)
	}
	// Clicking the continuation of the description lands on its raw text.
	for y := 8; y < 15; y++ {
		if !strings.Contains(rows[y], "trees") {
			continue
		}
		cell := strings.Index(rows[y], "trees")
		line, col, ok := e.HitCell(y, len([]rune(rows[y][:cell])))
		if !ok || line != 3 || !strings.HasPrefix(string([]rune(lines[3])[col:]), "trees") {
			t.Errorf("click on 'trees' = (%d,%d,%v)", line, col, ok)
		}
	}
}

func TestWrapLongWordAndEmptyLines(t *testing.T) {
	e := New([]string{"", "supercalifragilisticexpialidocious", ""}, 10, 7).WithWrap(true).SetCursor(1, 34)
	rows := e.wrapRowsText()
	var focus []string
	for _, r := range rows {
		if r != "" {
			focus = append(focus, r)
		}
	}
	if strings.Join(focus, "") != "supercalifragilisticexpialidocious" {
		t.Errorf("rows = %q", rows)
	}
	for _, r := range rows {
		if len([]rune(r)) > 10 {
			t.Errorf("row %q is wider than the editor", r)
		}
	}
	// The cursor at the end of the text has a cell of its own.
	if e.focusSub() != len(e.rawBreaks(1))-1 {
		t.Errorf("focusSub = %d", e.focusSub())
	}
}
