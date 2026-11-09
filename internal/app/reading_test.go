package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCtrlPTogglesReadingWithToast(t *testing.T) {
	s := newDragTestScreen(t)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !s.ed.Reading() {
		t.Fatal("not reading after ctrl+p")
	}
	if s.ed.ReadChanged {
		t.Fatal("ReadChanged not consumed")
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if s.ed.Reading() {
		t.Fatal("still reading")
	}
}

func TestReadingDefaultOpensRead(t *testing.T) {
	SetReading(true)
	defer SetReading(false)
	s := newDragTestScreen(t)
	if !s.ed.Reading() {
		t.Fatal("note did not open in reading mode")
	}
}

func TestReadingClickMovesNothing(t *testing.T) {
	s := newDragTestScreen(t)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	body := strings.Join(s.ed.Lines(), "\n")
	r, c := s.ed.CursorRow(), s.ed.CursorCol1()
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 6, Y: focusY + 1})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 6, Y: focusY + 1})
	s, _ = s.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if strings.Join(s.ed.Lines(), "\n") != body {
		t.Fatal("text changed")
	}
	if s.ed.CursorRow() != r || s.ed.CursorCol1() != c {
		t.Fatalf("cursor moved to %d:%d", s.ed.CursorRow(), s.ed.CursorCol1())
	}
}
