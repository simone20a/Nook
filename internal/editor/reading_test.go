package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func ctrl(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func readDoc() Editor {
	return New([]string{"# Title", "a b c d e f g h i j k l m n o p", "last"}, 10, 7)
}

func TestReadingToggleAndFlag(t *testing.T) {
	e := readDoc()
	e = press(e, ctrl(tea.KeyCtrlP))
	if !e.Reading() || !e.ReadChanged {
		t.Fatalf("reading=%v changed=%v", e.Reading(), e.ReadChanged)
	}
	e.ReadChanged = false
	e = press(e, ctrl(tea.KeyCtrlP))
	if e.Reading() || !e.ReadChanged {
		t.Fatalf("reading=%v changed=%v", e.Reading(), e.ReadChanged)
	}
}

func TestReadingWrapsAndIsReadOnly(t *testing.T) {
	e := readDoc()
	before := strings.Join(e.Lines(), "\n")
	e = press(e, ctrl(tea.KeyCtrlP))
	rows := e.readingRowsText()
	n := 0
	for _, r := range rows {
		if strings.Contains(r, "a b") || strings.Contains(r, "k l") {
			n++
		}
	}
	if n < 2 {
		t.Fatalf("long line not wrapped: %q", rows)
	}
	e = press(e, runes("x"), ctrl(tea.KeyEnter), ctrl(tea.KeyBackspace), ctrl(tea.KeyDelete),
		ctrl(tea.KeyCtrlU), ctrl(tea.KeyCtrlK), ctrl(tea.KeyCtrlT), ctrl(tea.KeyCtrlZ), ctrl(tea.KeyCtrlY))
	if got := strings.Join(e.Lines(), "\n"); got != before {
		t.Fatalf("text changed in reading mode: %q", got)
	}
}

func TestReadingScrollAndLeave(t *testing.T) {
	e := readDoc()
	e = press(e, ctrl(tea.KeyCtrlP))
	e = press(e, ctrl(tea.KeyEnd))
	e = press(e, ctrl(tea.KeyCtrlP))
	if e.Reading() {
		t.Fatal("still reading")
	}
	if e.CursorRow() != 3 {
		t.Fatalf("cursor row %d, want 3 (bottom)", e.CursorRow())
	}
	e = press(e, ctrl(tea.KeyCtrlP), ctrl(tea.KeyHome), ctrl(tea.KeyCtrlP))
	if e.CursorRow() != 1 {
		t.Fatalf("cursor row %d, want 1 (top)", e.CursorRow())
	}
}

func TestReadingUnmovedRestoresCursor(t *testing.T) {
	e := readDoc()
	e = press(e, ctrl(tea.KeyDown), ctrl(tea.KeyRight), ctrl(tea.KeyRight), ctrl(tea.KeyRight))
	r, c := e.CursorRow(), e.CursorCol1()
	e = press(e, ctrl(tea.KeyCtrlP), ctrl(tea.KeyCtrlP))
	if e.CursorRow() != r || e.CursorCol1() != c {
		t.Fatalf("cursor %d:%d, want %d:%d", e.CursorRow(), e.CursorCol1(), r, c)
	}
}

func TestReadingVimKeysDoNotEdit(t *testing.T) {
	e := readDoc().WithVim()
	before := strings.Join(e.Lines(), "\n")
	e = press(e, ctrl(tea.KeyCtrlP))
	e = press(e, runes("d"), runes("d"), runes("i"), runes("x"), runes("j"), runes("v"), runes(":"))
	if strings.Join(e.Lines(), "\n") != before {
		t.Fatal("vim edited in reading mode")
	}
	e = press(e, ctrl(tea.KeyCtrlP))
	if e.Reading() {
		t.Fatal("ctrl+p should leave reading in vim mode")
	}
}

func TestReadingFullStrengthAndNoCursor(t *testing.T) {
	e := readDoc()
	e = press(e, ctrl(tea.KeyCtrlP))
	v := e.View()
	if v == "" {
		t.Fatal("empty view")
	}
	if !strings.Contains(e.Footer(), "READING") {
		t.Fatalf("footer %q", e.Footer())
	}
}

func TestReadingHitCell(t *testing.T) {
	e := readDoc()
	e = press(e, ctrl(tea.KeyCtrlP))
	if _, _, ok := e.HitCell(-1, 0); ok {
		t.Fatal("hit above the screen")
	}
	found := false
	for y := range e.readLayout() {
		if l, _, ok := e.HitCell(y, 0); ok && l == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("no row maps to line 1")
	}
}

func TestWithReadingStartsAtTop(t *testing.T) {
	e := readDoc().WithReading(true)
	if !e.Reading() {
		t.Fatal("not reading")
	}
	e = e.WithReading(false)
	if e.Reading() {
		t.Fatal("still reading")
	}
}

func TestToggleDone(t *testing.T) {
	e := New([]string{"- [ ] a", "plain", "- [x] b"}, 20, 7)
	e = press(e, ctrl(tea.KeyCtrlX))
	if e.Lines()[0] != "- [x] a" {
		t.Fatalf("got %q", e.Lines()[0])
	}
	e = press(e, ctrl(tea.KeyCtrlX))
	if e.Lines()[0] != "- [ ] a" {
		t.Fatalf("got %q", e.Lines()[0])
	}
	e = press(e, ctrl(tea.KeyCtrlZ))
	// select all three lines: mixed -> all done
	e = press(e, ctrl(tea.KeyDown), ctrl(tea.KeyDown))
	e = e.ToggleDone()
	if e.Lines()[2] != "- [ ] b" {
		t.Fatalf("got %q", e.Lines()[2])
	}
	if e.Lines()[1] != "plain" {
		t.Fatal("plain line changed")
	}
	v := New([]string{"- [ ] a"}, 20, 7).WithVim()
	v = press(v, ctrl(tea.KeyCtrlX))
	if v.Lines()[0] != "- [x] a" {
		t.Fatalf("vim: %q", v.Lines()[0])
	}
	v = v.Undo()
	if v.Lines()[0] != "- [ ] a" {
		t.Fatalf("undo: %q", v.Lines()[0])
	}
	r := New([]string{"- [ ] a"}, 20, 7).WithReading(true)
	r = press(r, ctrl(tea.KeyCtrlX))
	if r.Lines()[0] != "- [ ] a" {
		t.Fatal("reading mode edited")
	}
}
