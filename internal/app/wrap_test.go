package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// wrapBody has a long second line: at the 56 columns of newEditorScreen's
// 60-column window its text wraps after "abcdefgh " 6 times, i.e. at rune 54.
var wrapBody = "start\n" + strings.Repeat("abcdefgh ", 12) + "\nend"

func newWrapTestScreen(t *testing.T, wrap bool) editorScreen {
	t.Helper()
	SetWrap(wrap)
	t.Cleanup(func() { SetWrap(false) })

	st := newKeysTestStore(t)
	created, err := st.Create("note")
	if err != nil {
		t.Fatalf("creating note: %v", err)
	}
	if err := st.Save(created.ID, "note", wrapBody); err != nil {
		t.Fatalf("saving: %v", err)
	}
	note, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return newEditorScreen(st, note, 60, 12)
}

// TestWrapDefaultFromSetting checks that notes open with word wrap as the
// setting says.
func TestWrapDefaultFromSetting(t *testing.T) {
	if newWrapTestScreen(t, false).ed.Wrap() {
		t.Error("word wrap is on although the setting is off")
	}
	if !newWrapTestScreen(t, true).ed.Wrap() {
		t.Error("word wrap is off although the setting is on")
	}
}

// TestWrapToggleKey checks that ctrl+w turns word wrap on and off, shows it,
// and that the choice carries over to the next note opened.
func TestWrapToggleKey(t *testing.T) {
	s := newWrapTestScreen(t, false)

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if !s.ed.Wrap() {
		t.Fatal("ctrl+w did not turn word wrap on")
	}
	if !strings.Contains(s.View(), "word wrap on") {
		t.Errorf("no toast after turning it on:\n%s", s.View())
	}
	if s.ed.WrapChanged {
		t.Error("WrapChanged was not reset")
	}
	if !newEditorScreen(s.st, nil, 60, 12).ed.Wrap() {
		t.Error("the next note does not keep word wrap on")
	}

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if s.ed.Wrap() {
		t.Fatal("second ctrl+w did not turn word wrap off")
	}
	if !strings.Contains(s.View(), "word wrap off") {
		t.Errorf("no toast after turning it off:\n%s", s.View())
	}
}

// TestWrapClickOnContinuationRow checks that a click on the second screen row
// of a wrapped line puts the cursor on the text drawn there.
func TestWrapClickOnContinuationRow(t *testing.T) {
	s := newWrapTestScreen(t, true)

	// The Focus Line is "start" (the cursor starts on the first line). Line 1
	// is drawn below it on two rows: the click goes on the second one.
	x, y := marginX, focusY+2
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: x, Y: y})
	if got, want := s.ed.CursorRow(), 2; got != want {
		t.Fatalf("cursor on line %d, want %d", got, want)
	}
	if got, want := s.ed.CursorCol1(), 55; got != want {
		t.Errorf("cursor column %d, want %d (the start of the second row)", got, want)
	}
}

// TestWrapWheelMovesByRows checks that the wheel moves one screen row.
func TestWrapWheelMovesByRows(t *testing.T) {
	s := newWrapTestScreen(t, true)
	s, _ = s.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	// The cursor starts at the end of "start" (column 5) and keeps its
	// distance from the left of the row.
	if s.ed.CursorRow() != 2 || s.ed.CursorCol1() != 6 {
		t.Fatalf("after one wheel step the cursor is at %d:%d, want 2:6", s.ed.CursorRow(), s.ed.CursorCol1())
	}
	s, _ = s.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if s.ed.CursorRow() != 2 || s.ed.CursorCol1() != 60 {
		t.Errorf("after two wheel steps the cursor is at %d:%d, want 2:60 (second row of the line)", s.ed.CursorRow(), s.ed.CursorCol1())
	}
}
