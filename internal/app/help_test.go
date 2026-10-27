package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHelpNavigation checks that arrows switch page, that unknown keys leave
// the popup open and that esc closes it.
func TestHelpNavigation(t *testing.T) {
	m := Model{width: 80, height: 24}.openHelp()
	if m.helpTab != helpTabList {
		t.Fatalf("helpTab = %d, want %d", m.helpTab, helpTabList)
	}

	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.helpTab != 1 {
		t.Fatalf("helpTab after right = %d, want 1", m.helpTab)
	}
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyLeft})
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyLeft})
	if m.helpTab != len(helpPages)-1 {
		t.Fatalf("helpTab after wrapping left = %d, want %d", m.helpTab, len(helpPages)-1)
	}

	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	if !m.helpOpen {
		t.Fatal("helpOpen = false after unknown key, want true")
	}
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.helpOpen {
		t.Fatal("helpOpen = true after esc, want false")
	}
}

// TestHelpCtrlCQuits checks that ctrl+c still quits with help open.
func TestHelpCtrlCQuits(t *testing.T) {
	m := Model{width: 80, height: 24}.openHelp()
	_, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !quit {
		t.Fatal("quit = false, want true")
	}
}

// TestHelpOpensOnEditorPage checks that help opens on the editor page when
// the editor is the active screen.
func TestHelpOpensOnEditorPage(t *testing.T) {
	m := Model{width: 80, height: 24, screen: screenEditor}.openHelp()
	if m.helpTab != helpTabEditor {
		t.Fatalf("helpTab = %d, want %d", m.helpTab, helpTabEditor)
	}
}

// TestRenderHelpStableWidth checks that every page renders with the same
// width and contains its first key.
func TestRenderHelpStableWidth(t *testing.T) {
	want := helpTextWidth(renderHelp(80, 24, 0, 0))
	for i, p := range helpPages {
		out := renderHelp(80, 24, i, 0)
		if got := helpTextWidth(out); got != want {
			t.Fatalf("page %q width = %d, want %d", p.title, got, want)
		}
		if !strings.Contains(out, p.title) {
			t.Fatalf("page %q missing from tab bar", p.title)
		}
	}
}

// TestHelpScrollClamped checks that scrolling never goes out of range.
func TestHelpScrollClamped(t *testing.T) {
	m := Model{width: 80, height: 14}.openHelp()
	for i := 0; i < 100; i++ {
		m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	max := len(helpPages[m.helpTab].rows) - helpBodyH(m.height)
	if max < 0 {
		max = 0
	}
	if m.helpScroll != max {
		t.Fatalf("helpScroll = %d, want %d", m.helpScroll, max)
	}
}
