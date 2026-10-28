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

// focusY is the screen row of the Focus Line in newDragTestScreen: one row of
// margin plus the middle row of the text.
const focusY = 4

// TestMouseDragSelects checks that pressing the left button and dragging
// selects the text between the pressed point and the cell reached, and that
// releasing leaves the selection active.
func TestMouseDragSelects(t *testing.T) {
	s := newDragTestScreen(t)

	// Press on the Focus Line (screen row focusY is the central row), then
	// drag one row lower and a few columns to the right.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 6, Y: focusY})
	if !s.dragging {
		t.Fatal("dragging = false after the press, want true")
	}
	if _, ok := s.ed.SelectedText(); ok {
		t.Fatal("a simple press has already selected some text")
	}

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 10, Y: focusY + 1})
	sel, ok := s.ed.SelectedText()
	if !ok {
		t.Fatal("after the drag there is no selection")
	}
	if !strings.Contains(sel, "line of text") || !strings.Contains(sel, "second") {
		t.Errorf("selected text = %q, want from the pressed point through the second line", sel)
	}

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 10, Y: focusY + 1})
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

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 10, Y: focusY + 1})
	if _, ok := s.ed.SelectedText(); ok {
		t.Error("a movement without a press selected some text")
	}
}

// TestMouseClickClearsSelection checks that a new click clears the
// previous selection, setting the new anchor.
func TestMouseClickClearsSelection(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 4, Y: focusY})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 12, Y: focusY})
	if _, ok := s.ed.SelectedText(); !ok {
		t.Fatal("the drag selected nothing")
	}
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 12, Y: focusY})

	// Click on a different point: the selection disappears (when the button
	// is released) and only the cursor remains.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 2, Y: focusY})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 2, Y: focusY})
	if _, ok := s.ed.SelectedText(); ok {
		t.Error("the click did not clear the previous selection")
	}
}

// TestDoubleClickSelectsWord checks that the double click selects the word and
// does not leave dragging active.
func TestDoubleClickSelectsWord(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 7, Y: focusY})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 7, Y: focusY})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 7, Y: focusY})
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
	wheel := func(b tea.MouseButton) {
		s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: b, X: 6, Y: focusY})
	}

	wheel(tea.MouseButtonWheelDown)
	if got := s.ed.CursorRow(); got != 2 {
		t.Fatalf("CursorRow after wheel down = %d, want 2", got)
	}
	wheel(tea.MouseButtonWheelDown)
	wheel(tea.MouseButtonWheelDown)
	if got := s.ed.CursorRow(); got != 3 {
		t.Fatalf("CursorRow at the end = %d, want 3", got)
	}
	wheel(tea.MouseButtonWheelUp)
	if got := s.ed.CursorRow(); got != 2 {
		t.Fatalf("CursorRow after wheel up = %d, want 2", got)
	}
}

// TestFocusLineMovesOnRelease checks that pressing on another line does not move
// the Focus Line, that a release without movement does, and that a drag keeps
// the Focus Line where it was, even after the release.
func TestFocusLineMovesOnRelease(t *testing.T) {
	s := newDragTestScreen(t)
	focusLine := func() int {
		line, _ := s.ed.LineAtRow(s.ed.FocusRow())
		return line
	}
	if got := focusLine(); got != 0 {
		t.Fatalf("initial Focus Line = %d, want 0", got)
	}

	// Press two rows below the Focus Line: nothing moves yet.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 3, Y: focusY + 2})
	if got := focusLine(); got != 0 {
		t.Fatalf("Focus Line after the press = %d, want 0 (it moves on release)", got)
	}
	// Release without moving: the Focus Line goes to the clicked line.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 3, Y: focusY + 2})
	if got := focusLine(); got != 2 {
		t.Fatalf("Focus Line after the click = %d, want 2", got)
	}

	// Drag from the Focus Line to the row above: the selection exists, the
	// Focus Line stays on line 2 also after the release.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 6, Y: focusY})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 6, Y: focusY - 1})
	if _, ok := s.ed.SelectedText(); !ok {
		t.Fatal("the drag selected nothing")
	}
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 6, Y: focusY - 1})
	if got := focusLine(); got != 2 {
		t.Fatalf("Focus Line after the drag = %d, want 2 (it must not move)", got)
	}
	if _, ok := s.ed.SelectedText(); !ok {
		t.Error("the selection disappeared on release")
	}
}
