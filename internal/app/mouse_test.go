package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newDragTestScreen opens the editor on a note with three lines of text.
func newDragTestScreen(t *testing.T) editorScreen {
	t.Helper()

	st := newKeysTestStore(t)
	created, err := st.Create("note")
	if err != nil {
		t.Fatalf("creating note: %v", err)
	}
	body := "first line of text\nsecond line\nthird line"
	if err := st.Save(created.ID, "note", body); err != nil {
		t.Fatalf("saving: %v", err)
	}
	note, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return newEditorScreen(st, note, 60, 12)
}

// TestMouseDragSelects checks that pressing the left button and dragging
// selects the text between the pressed point and the cell reached, and that
// releasing leaves the selection active.
func TestMouseDragSelects(t *testing.T) {
	s := newDragTestScreen(t)

	// Press on the Focus Line (screen row 5 is the central row), then
	// drag one row lower and a few columns to the right.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 6, Y: 5})
	if !s.dragging {
		t.Fatal("dragging = false after the press, want true")
	}
	if _, ok := s.ed.SelectedText(); ok {
		t.Fatal("a simple press has already selected some text")
	}

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 10, Y: 6})
	sel, ok := s.ed.SelectedText()
	if !ok {
		t.Fatal("after the drag there is no selection")
	}
	if !strings.Contains(sel, "line of text") || !strings.Contains(sel, "second") {
		t.Errorf("selected text = %q, want from the pressed point through the second line", sel)
	}

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 10, Y: 6})
	if s.dragging {
		t.Error("dragging = true after the release, want false")
	}
	if _, ok := s.ed.SelectedText(); !ok {
		t.Error("the release cleared the selection, want it to remain")
	}
}

// TestMouseMotionWithoutPressIgnored checks that mouse movements without a
// press in progress select nothing.
func TestMouseMotionWithoutPressIgnored(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 10, Y: 6})
	if _, ok := s.ed.SelectedText(); ok {
		t.Error("a movement without a press selected some text")
	}
}

// TestMouseClickClearsSelection checks that a new click clears the
// previous selection, setting the new anchor.
func TestMouseClickClearsSelection(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 4, Y: 5})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 12, Y: 5})
	if _, ok := s.ed.SelectedText(); !ok {
		t.Fatal("the drag selected nothing")
	}
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 12, Y: 5})

	// Click on a different point: the selection disappears and only the cursor remains.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 2, Y: 5})
	if _, ok := s.ed.SelectedText(); ok {
		t.Error("the click did not clear the previous selection")
	}
}

// TestDoubleClickSelectsWord checks that the double click selects the word and
// does not leave dragging active.
func TestDoubleClickSelectsWord(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 7, Y: 5})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 7, Y: 5})
	if s.dragging {
		t.Error("dragging = true after the double click, want false")
	}
	sel, ok := s.ed.SelectedText()
	if !ok || strings.TrimSpace(sel) == "" {
		t.Fatalf("the double click did not select a word (sel = %q)", sel)
	}
	if strings.Contains(sel, "\n") {
		t.Errorf("the double-click selection crosses several lines: %q", sel)
	}
}

// TestMouseWheelScrollsDocument checks that the wheel moves the document under
// the Focus Line (the cursor line changes) and stops at the document edges.
func TestMouseWheelScrollsDocument(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 6, Y: 5})
	if got := s.ed.CursorRow(); got != 3 {
		t.Fatalf("CursorRow after wheel down = %d, want 3", got)
	}
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 6, Y: 5})
	if got := s.ed.CursorRow(); got != 3 {
		t.Fatalf("CursorRow at the end = %d, want 3", got)
	}
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, X: 6, Y: 5})
	if got := s.ed.CursorRow(); got != 1 {
		t.Fatalf("CursorRow after wheel up = %d, want 1", got)
	}
}
