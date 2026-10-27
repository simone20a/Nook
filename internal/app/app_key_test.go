package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/editor"
)

// TestHelpKeyToggle checks that f1 opens the help popup without making
// the application quit.
func TestHelpKeyToggle(t *testing.T) {
	m := Model{width: 80, height: 24}

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyF1})

	if quit {
		t.Fatal("quit = true, want false")
	}
	if !m2.helpOpen {
		t.Fatal("helpOpen = false, want true")
	}
}

// TestEditorSpaceFlowsRunes checks that a space sent as KeyRunes
// reaches the editor of the active screen and inserts a space in the text.
// The real case (KeySpace) is in TestEditorSpaceFlows, in keys_test.go.
func TestEditorSpaceFlowsRunes(t *testing.T) {
	m := Model{width: 80, height: 24, screen: screenEditor}
	m.editor = editorScreen{
		ed:     editor.New([]string{"a"}, contentW(80), edHeight(contentH(24))),
		width:  80,
		height: 24,
	}

	m2, _, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	if got := m2.editor.ed.Lines()[0]; got != "a " {
		t.Fatalf("editor.ed.Lines()[0] = %q, want %q", got, "a ")
	}
}
