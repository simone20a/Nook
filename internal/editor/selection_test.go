package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"nook/internal/theme"
)

// TestSelectionOnLine checks the computation of the selected range per line,
// including the intermediate lines of a multi-line selection.
func TestSelectionOnLine(t *testing.T) {
	e := New([]string{"first line", "second line", "third line"}, 40, 9)

	// No selection.
	if _, _, ok := e.selectionOnLine(0); ok {
		t.Fatal("selectionOnLine without a selection = true, want false")
	}

	// Selection from column 2 of line 0 to column 4 of line 2.
	e = e.SetSelection(0, 2, 2, 4)

	from, to, ok := e.selectionOnLine(0)
	if !ok || from != 2 || to != runeLen("first line") {
		t.Errorf("line 0: [%d,%d) ok=%v, want [2,%d)", from, to, ok, runeLen("first line"))
	}
	from, to, ok = e.selectionOnLine(1)
	if !ok || from != 0 || to != runeLen("second line") {
		t.Errorf("line 1: [%d,%d) ok=%v, want [0,%d)", from, to, ok, runeLen("second line"))
	}
	from, to, ok = e.selectionOnLine(2)
	if !ok || from != 0 || to != 4 {
		t.Errorf("line 2: [%d,%d) ok=%v, want [0,4)", from, to, ok)
	}

	// A line outside the range is not selected.
	if _, _, ok := e.selectionOnLine(3); ok {
		t.Error("line 3 outside the selection: ok = true, want false")
	}
}

// TestSelectedText checks the text returned for a selection on one line and
// on several lines.
func TestSelectedText(t *testing.T) {
	e := New([]string{"good world", "second", "third"}, 40, 9)

	if _, ok := e.SelectedText(); ok {
		t.Fatal("SelectedText without a selection = true, want false")
	}

	e = e.SetSelection(0, 0, 0, 4)
	if got, ok := e.SelectedText(); !ok || got != "good" {
		t.Errorf("SelectedText = %q, %v; want \"good\"", got, ok)
	}

	e = e.SetSelection(0, 5, 2, 5)
	want := "world\nsecond\nthird"
	if got, ok := e.SelectedText(); !ok || got != want {
		t.Errorf("SelectedText = %q, %v; want %q", got, ok, want)
	}
}

// TestSelectionRendered checks that the selection is visible in the view: the
// Focus Line and the rendered lines contain the selection style when the
// selection crosses them.
func TestSelectionRendered(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	lines := []string{"plain text", "**bold** here", "last line"}
	e := New(lines, 40, 9)

	// Without a selection the view does not contain the selection style.
	plain := e.View()
	if strings.Contains(plain, theme.Selection.Render("t")) {
		t.Fatal("the view without a selection already contains the selection style")
	}

	// Selection from line 0 to line 2: the cursor is on line 2, so lines
	// 0 and 1 are rendered lines (not focus) and must show the highlighting.
	e = e.SetSelection(0, 0, 2, 5)
	view := e.View()

	// The text stays the same on screen, but with the highlighting.
	stripped := ansi.Strip(view)
	for _, l := range lines {
		if !strings.Contains(stripped, strings.ReplaceAll(l, "**", "")) {
			t.Errorf("the view does not contain the line %q:\n%s", l, stripped)
		}
	}
	if !strings.Contains(view, theme.Selection.Render(lines[0])) {
		t.Errorf("the rendered and selected line is not highlighted:\n%q", view)
	}
	if !strings.Contains(view, theme.Selection.Render("bold")) {
		t.Errorf("the inline text of the selection is not highlighted:\n%q", view)
	}
}

// TestDragSelectionWithKeys checks that shift+arrows extend the selection and that
// the selected text grows accordingly.
func TestDragSelectionWithKeys(t *testing.T) {
	e := New([]string{"abcdef"}, 40, 5)
	e = e.SetCursor(0, 0)

	e = press(e, tea.KeyMsg{Type: tea.KeyShiftRight})
	e = press(e, tea.KeyMsg{Type: tea.KeyShiftRight})
	e = press(e, tea.KeyMsg{Type: tea.KeyShiftRight})

	got, ok := e.SelectedText()
	if !ok || got != "abc" {
		t.Fatalf("SelectedText after three shift+right = %q, %v; want \"abc\"", got, ok)
	}
}

// TestSetSelectionClamps checks that SetSelection brings line and column inside
// the limits of the document.
func TestSetSelectionClamps(t *testing.T) {
	e := New([]string{"short", "second"}, 40, 5)
	e = e.SetSelection(-3, 0, 99, 99)

	if e.cursorLine != 1 {
		t.Errorf("cursorLine = %d, want 1", e.cursorLine)
	}
	if e.cursorCol != runeLen("second") {
		t.Errorf("cursorCol = %d, want %d", e.cursorCol, runeLen("second"))
	}
	if e.anchorLine != 0 || e.anchorCol != 0 {
		t.Errorf("anchor = (%d,%d), want (0,0)", e.anchorLine, e.anchorCol)
	}
}

// TestSelectionReplacedByTyping checks that typing replaces the
// selected text, as expected from an editor.
func TestSelectionReplacedByTyping(t *testing.T) {
	e := New([]string{"good world"}, 40, 5)
	e = e.SetSelection(0, 0, 0, 4)

	e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if got := e.Lines()[0]; got != "x world" {
		t.Errorf("line after the replacement = %q, want %q", got, "x world")
	}
	if _, ok := e.SelectedText(); ok {
		t.Error("the selection is still active after typing")
	}
}
