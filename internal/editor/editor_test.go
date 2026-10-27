package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"nook/internal/markdown"
	"nook/internal/theme"
)

// TestFocusRowCentered checks that the cursor's line is in the middle of the screen.
func TestFocusRowCentered(t *testing.T) {
	e := New([]string{"a", "b", "c", "d", "e"}, 20, 5)

	if got := e.focusRow(); got != 2 {
		t.Fatalf("focusRow() = %d, want 2", got)
	}
	if got := e.docIndexForRow(0); got != -2 {
		t.Fatalf("docIndexForRow(0) = %d, want -2", got)
	}
	if got := e.docIndexForRow(2); got != 0 {
		t.Fatalf("docIndexForRow(2) = %d, want 0", got)
	}
	if got := e.docIndexForRow(4); got != 2 {
		t.Fatalf("docIndexForRow(4) = %d, want 2", got)
	}
}

// TestVerticalScroll checks vertical scrolling with the cursor.
func TestVerticalScroll(t *testing.T) {
	e := New([]string{"a", "b", "c", "d", "e"}, 20, 5)

	e = e.moveDown()
	e = e.moveDown()

	if e.cursorLine != 2 {
		t.Fatalf("cursorLine = %d, want 2", e.cursorLine)
	}
	if got := e.docIndexForRow(2); got != 2 {
		t.Fatalf("docIndexForRow(2) = %d, want 2", got)
	}
}

// TestHScroll checks horizontal scrolling.
func TestHScroll(t *testing.T) {
	e := New([]string{"abcdefghij"}, 5, 3)

	e = e.moveEnd()
	if e.cursorCol != 10 {
		t.Fatalf("cursorCol after moveEnd = %d, want 10", e.cursorCol)
	}
	if e.hScroll != 6 {
		t.Fatalf("hScroll after moveEnd = %d, want 6", e.hScroll)
	}

	e = e.moveHome()
	if e.hScroll != 0 {
		t.Fatalf("hScroll after moveHome = %d, want 0", e.hScroll)
	}
}

// TestClampCol checks that the column is brought back within the end of the line.
func TestClampCol(t *testing.T) {
	e := New([]string{"abc", "d"}, 20, 3)

	e = e.moveEnd()
	if e.cursorCol != 3 {
		t.Fatalf("cursorCol after moveEnd = %d, want 3", e.cursorCol)
	}

	e = e.moveDown()
	if e.cursorLine != 1 {
		t.Fatalf("cursorLine = %d, want 1", e.cursorLine)
	}
	if e.cursorCol != 1 {
		t.Fatalf("cursorCol = %d, want 1", e.cursorCol)
	}
}

// TestDeleteSelection checks the deletion of the selected text.
func TestDeleteSelection(t *testing.T) {
	e := New([]string{"hello world"}, 20, 3)
	e.selecting = true
	e.anchorLine = 0
	e.anchorCol = 0
	e.cursorLine = 0
	e.cursorCol = 5

	e.deleteSelection()

	if e.lines[0] != " world" {
		t.Fatalf("lines[0] = %q, want %q", e.lines[0], " world")
	}
	if e.cursorCol != 0 {
		t.Fatalf("cursorCol = %d, want 0", e.cursorCol)
	}
	if e.selecting {
		t.Fatal("selecting = true, want false")
	}
}

// TestToggleBoldInserts checks the insertion of the bold markers.
func TestToggleBoldInserts(t *testing.T) {
	e := New([]string{"hello"}, 20, 3)
	e.cursorCol = 2

	e.toggleBold()

	if e.lines[0] != "he****llo" {
		t.Fatalf("lines[0] = %q, want %q", e.lines[0], "he****llo")
	}
	if e.cursorCol != 4 {
		t.Fatalf("cursorCol = %d, want 4", e.cursorCol)
	}
}

// TestToggleBoldRemoves checks the removal of the bold markers.
func TestToggleBoldRemoves(t *testing.T) {
	e := New([]string{"**hello**"}, 20, 3)
	e.cursorCol = 4

	e.toggleBold()

	if e.lines[0] != "hello" {
		t.Fatalf("lines[0] = %q, want %q", e.lines[0], "hello")
	}
}

// TestSetHeading checks the addition of a heading.
func TestSetHeading(t *testing.T) {
	e := New([]string{"title"}, 20, 3)

	e.setHeading(1)

	if e.lines[0] != "# title" {
		t.Fatalf("lines[0] = %q, want %q", e.lines[0], "# title")
	}
}

// TestSetHeadingRemoves checks the removal of a heading of the same level.
func TestSetHeadingRemoves(t *testing.T) {
	e := New([]string{"## title"}, 20, 3)

	e.setHeading(2)

	if e.lines[0] != "title" {
		t.Fatalf("lines[0] = %q, want %q", e.lines[0], "title")
	}
}

// TestTruncateStyled checks truncation that is aware of wide cells and
// ANSI sequences: the result is measured after stripping the colour codes.
func TestTruncateStyled(t *testing.T) {
	if got := truncate("abcdefghij", 5); got != "abcd…" {
		t.Fatalf("truncate(%q, 5) = %q, want %q", "abcdefghij", got, "abcd…")
	}

	if got := ansi.Strip(truncate("日本語", 3)); got != "日…" {
		t.Fatalf("truncate(%q, 3) = %q, want %q", "日本語", got, "日…")
	}

	rs := theme.Styles[markdown.StyleBold].Render("abc")
	if got := ansi.Strip(truncate(rs, 2)); got != "a…" {
		t.Fatalf("truncate(bold style %q, 2) = %q, want %q", rs, got, "a…")
	}
}

// TestPrefixWidth checks the width of the block prefix of lines.
func TestPrefixWidth(t *testing.T) {
	casi := []struct {
		raw  string
		want int
	}{
		{"# T", 2},
		{"## T", 3},
		{"> c", 2},
		{"- v", 2},
		{"  - v", 4},
		{"- [ ] d", 6},
		{"1. v", 3},
		{"plain", 0},
	}
	for _, c := range casi {
		if got := markdown.PrefixWidth(c.raw); got != c.want {
			t.Errorf("PrefixWidth(%q) = %d, want %d", c.raw, got, c.want)
		}
	}
}

// TestIsInsideFence checks the recognition of lines inside a
// code block delimited by fences.
func TestIsInsideFence(t *testing.T) {
	lines := []string{"```", "a", "```", "b"}

	if !isInsideFence(lines, 1) {
		t.Fatal("isInsideFence(lines, 1) = false, want true")
	}
	if isInsideFence(lines, 3) {
		t.Fatal("isInsideFence(lines, 3) = true, want false")
	}
}

// TestRenderFocusRowPad checks that the focus row compensates the block
// prefix with a space, so as not to shift the text on screen.
func TestRenderFocusRowPad(t *testing.T) {
	e := Editor{lines: []string{"# hello"}, cursorLine: 0, cursorCol: 0, width: 10, height: 1}

	got := ansi.Strip(e.View())
	if len(got) < len("  # hello") || got[:len("  # hello")] != "  # hello" {
		t.Fatalf("View() = %q, want prefix %q", got, "  # hello")
	}
}

// TestSpaceInserts checks that the space bar inserts a space at the
// cursor position on the current line.
func TestSpaceInserts(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	e2, ok := m.(Editor)
	if !ok {
		t.Fatalf("Update returned %T, want Editor", m)
	}

	if got := e2.Lines()[0]; got != "a " {
		t.Fatalf("Lines()[0] = %q, want %q", got, "a ")
	}
}

// TestArrowMoves checks that the down arrow moves the cursor to the
// next line.
func TestArrowMoves(t *testing.T) {
	e := New([]string{"a", "b"}, 10, 5)

	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyDown})
	e2, ok := m.(Editor)
	if !ok {
		t.Fatalf("Update returned %T, want Editor", m)
	}

	if got := e2.CursorRow(); got != 2 {
		t.Fatalf("CursorRow() = %d, want 2", got)
	}
}

// TestQuoteBarInView checks that quotes show the vertical bar also
// on the rendered lines and on the Focus Line, so the bar stays continuous over the whole
// quote block.
func TestQuoteBarInView(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	lines := []string{
		"plain text",
		"> first quote line",
		"> second line",
		">",
		"text after",
	}
	e := New(lines, 50, 11)
	e = e.SetCursor(2, 0) // Focus Line on the second quote line

	view := ansi.Strip(e.View())
	rows := strings.Split(view, "\n")

	var withBar, quoted int
	for _, row := range rows {
		if strings.HasPrefix(row, markdown.QuoteBar) {
			withBar++
		}
		if strings.Contains(row, "quote line") || strings.Contains(row, "second line") {
			if !strings.HasPrefix(row, markdown.QuoteBar) {
				t.Errorf("quote line without a bar on the left: %q", row)
			}
			quoted++
		}
	}
	if quoted != 2 {
		t.Errorf("found %d quote lines in the view, want 2:\n%s", quoted, view)
	}
	// Three quote lines (including the empty one, which is rendered here): all of them
	// carry the bar, including the Focus Line, which shows the raw text with its
	// ">" but keeps the bar on the left.
	if withBar != 3 {
		t.Errorf("lines with the bar = %d, want 3 (one per quote line):\n%s", withBar, view)
	}
	// The Focus Line is raw: it shows the marker in addition to the bar.
	if !strings.HasPrefix(strings.Split(view, "\n")[e.focusRow()], markdown.QuoteBar+" > ") {
		t.Errorf("the Focus Line does not show the bar and the raw text:\n%s", view)
	}

	// The empty "> " line is still a quote: the bar is not interrupted.
	for _, row := range rows {
		if strings.TrimSpace(row) == markdown.QuoteBar {
			return
		}
	}
	t.Errorf("the empty quote line does not carry the bar:\n%s", view)
}
