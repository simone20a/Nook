package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/editor"
)

// TestHelpKeyToggle verifica che f1 apra il popup di aiuto senza far uscire
// l'applicazione.
func TestHelpKeyToggle(t *testing.T) {
	m := Model{width: 80, height: 24}

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyF1})

	if quit {
		t.Fatal("quit = true, atteso false")
	}
	if !m2.helpOpen {
		t.Fatal("helpOpen = false, atteso true")
	}
}

// TestEditorSpaceFlowsRunes verifica che uno spazio inviato come KeyRunes
// arrivi all'editor della schermata attiva e inserisca uno spazio nel testo.
// Il caso reale (KeySpace) è in TestEditorSpaceFlows, in keys_test.go.
func TestEditorSpaceFlowsRunes(t *testing.T) {
	m := Model{width: 80, height: 24, screen: screenEditor}
	m.editor = editorScreen{
		ed:     editor.New([]string{"a"}, contentW(80), edHeight(contentH(24))),
		width:  80,
		height: 24,
	}

	m2, _, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	if got := m2.editor.ed.Lines()[0]; got != "a " {
		t.Fatalf("editor.ed.Lines()[0] = %q, atteso %q", got, "a ")
	}
}
