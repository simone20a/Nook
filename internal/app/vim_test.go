package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/store"
)

// openVimNote stores a note with the given body and opens it in the editor
// with NOOK_VIM=1, as the list does. It returns the model and the note's ID.
func openVimNote(t *testing.T, body string) (Model, *store.Store, int64) {
	t.Helper()
	t.Setenv(vimEnv, "1")

	st := newKeysTestStore(t)
	note, err := st.Create("note")
	if err != nil {
		t.Fatalf("creating note: %v", err)
	}
	if err := st.Save(note.ID, note.Title, body); err != nil {
		t.Fatalf("saving note: %v", err)
	}
	if note, err = st.Get(note.ID); err != nil {
		t.Fatalf("reading note: %v", err)
	}

	m := Model{st: st, width: 80, height: 24, screen: screenEditor}
	m.list = newListScreen(st, 80, 24)
	m.editor = newEditorScreen(st, note, 80, 24)
	return m, st, note.ID
}

// typeKeys sends keys to the application one rune at a time, "\x1b" being
// esc and "\r" enter, and reports whether it asked to quit.
func typeKeys(m Model, keys string) (Model, bool) {
	for _, r := range keys {
		k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		switch r {
		case '\x1b':
			k = tea.KeyMsg{Type: tea.KeyEsc}
		case '\r':
			k = tea.KeyMsg{Type: tea.KeyEnter}
		}
		var quit bool
		if m, _, quit = m.updateKey(k); quit {
			return m, true
		}
	}
	return m, false
}

// storedBody is the body of the note as saved on disk.
func storedBody(t *testing.T, st *store.Store, id int64) string {
	t.Helper()
	note, err := st.Get(id)
	if err != nil {
		t.Fatalf("reading note: %v", err)
	}
	return note.Body
}

// TestVimModeFromEnv checks that only NOOK_VIM=1 turns vim mode on.
func TestVimModeFromEnv(t *testing.T) {
	for value, want := range map[string]bool{"1": true, "": false, "0": false, "true": false} {
		t.Setenv(vimEnv, value)
		footer := newEditorScreen(nil, nil, 80, 24).ed.Footer()
		if got := strings.Contains(footer, "NORMAL"); got != want {
			t.Errorf("NOOK_VIM=%q: vim mode %v, want %v (footer %q)", value, got, want, footer)
		}
	}
}

// TestVimWriteQuitSavesAndLeaves checks that :wq saves the note and goes back
// to the list.
func TestVimWriteQuitSavesAndLeaves(t *testing.T) {
	m, st, id := openVimNote(t, "hello")

	m, quit := typeKeys(m, "A!\x1b:wq\r")

	if quit {
		t.Fatal("quit = true, want false")
	}
	if m.screen != screenList {
		t.Fatalf("screen = %v, want screenList", m.screen)
	}
	if got := storedBody(t, st, id); got != "hello!" {
		t.Fatalf("stored body = %q, want %q", got, "hello!")
	}
}

// TestVimWriteKeepsEditing checks that :w saves and stays in the editor.
func TestVimWriteKeepsEditing(t *testing.T) {
	m, st, id := openVimNote(t, "hello")

	m, _ = typeKeys(m, "0x:w\r")

	if m.screen != screenEditor || m.editor.Dirty() {
		t.Fatalf("screen %v dirty %v, want the editor with no unsaved changes", m.screen, m.editor.Dirty())
	}
	if m.editor.toast != "saved" {
		t.Errorf("toast = %q, want %q", m.editor.toast, "saved")
	}
	if got := storedBody(t, st, id); got != "ello" {
		t.Fatalf("stored body = %q, want %q", got, "ello")
	}
}

// TestVimQuitAsksWhenDirty checks that :q with unsaved changes opens the
// Save/Discard/Cancel popup instead of losing them.
func TestVimQuitAsksWhenDirty(t *testing.T) {
	m, st, id := openVimNote(t, "hello")

	m, _ = typeKeys(m, "x:q\r")

	if m.screen != screenEditor || m.editor.mode != edPrompt {
		t.Fatalf("screen %v mode %v, want the editor with the popup open", m.screen, m.editor.mode)
	}
	if got := storedBody(t, st, id); got != "hello" {
		t.Fatalf("stored body = %q, want it untouched", got)
	}
}

// TestVimQuitLeavesWhenClean checks that :q on an unchanged note goes back to
// the list.
func TestVimQuitLeavesWhenClean(t *testing.T) {
	m, _, _ := openVimNote(t, "hello")

	if m, _ = typeKeys(m, ":q\r"); m.screen != screenList {
		t.Fatalf("screen = %v, want screenList", m.screen)
	}
}

// TestVimQuitBangDiscards checks that :q! goes back to the list without
// saving.
func TestVimQuitBangDiscards(t *testing.T) {
	m, st, id := openVimNote(t, "hello")

	m, _ = typeKeys(m, "dd:q!\r")

	if m.screen != screenList {
		t.Fatalf("screen = %v, want screenList", m.screen)
	}
	if got := storedBody(t, st, id); got != "hello" {
		t.Fatalf("stored body = %q, want it untouched", got)
	}
}

// TestVimWriteQuitStaysWhenSaveFails checks that :wq does not leave the note
// when saving fails: the error is shown and the changes are kept.
func TestVimWriteQuitStaysWhenSaveFails(t *testing.T) {
	t.Setenv(vimEnv, "1")
	st := newKeysTestStore(t)
	m := Model{st: st, width: 80, height: 24, screen: screenEditor}
	m.list = newListScreen(st, 80, 24)
	m.editor = newEditorScreen(st, &store.Note{ID: 424242, Title: "gone", Body: "hello"}, 80, 24)

	m, _ = typeKeys(m, "0x:wq\r")

	if m.screen != screenEditor {
		t.Fatalf("screen = %v, want the editor", m.screen)
	}
	if !strings.HasPrefix(m.editor.toast, "error:") {
		t.Errorf("toast = %q, want an error", m.editor.toast)
	}
	if !m.editor.Dirty() || m.editor.ed.Lines()[0] != "ello" {
		t.Errorf("dirty %v text %q, want the unsaved change kept", m.editor.Dirty(), m.editor.ed.Lines()[0])
	}
}

// TestVimCtrlCCancels checks that in vim mode ctrl+c leaves INSERT and VISUAL
// mode and cancels a pending command without quitting, and quits from
// NORMAL mode as usual.
func TestVimCtrlCCancels(t *testing.T) {
	ctrlC := tea.KeyMsg{Type: tea.KeyCtrlC}
	for _, keys := range []string{"ihi", "vl", "d", "3", ":w"} {
		m, _, _ := openVimNote(t, "hello")
		m, _ = typeKeys(m, keys)

		m, _, quit := m.updateKey(ctrlC)
		if quit {
			t.Fatalf("%q then ctrl+c: quit = true, want false", keys)
		}
		if footer := m.editor.ed.Footer(); !strings.Contains(footer, "NORMAL") || m.editor.ed.VimBusy() {
			t.Errorf("%q then ctrl+c: footer %q busy %v, want idle NORMAL", keys, footer, m.editor.ed.VimBusy())
		}
		if m.editor.ed.WantsSave {
			t.Errorf("%q then ctrl+c: the command line ran", keys)
		}
		if _, _, quit = m.updateKey(ctrlC); !quit {
			t.Errorf("%q then ctrl+c twice: quit = false, want true", keys)
		}
	}
}

// TestCtrlCQuitsWithoutVim checks that without vim mode ctrl+c still quits
// from the editor at once.
func TestCtrlCQuitsWithoutVim(t *testing.T) {
	t.Setenv(vimEnv, "")
	m := Model{width: 80, height: 24, screen: screenEditor}
	m.editor = newEditorScreen(nil, nil, 80, 24)

	m, _ = typeKeys(m, "ab")
	if _, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlC}); !quit {
		t.Fatal("quit = false, want true")
	}
}

// TestVimEscStaysInEditor checks that esc in NORMAL mode does not leave the
// note.
func TestVimEscStaysInEditor(t *testing.T) {
	m, _, _ := openVimNote(t, "hello")

	m, quit := typeKeys(m, "\x1b\x1b")

	if quit || m.screen != screenEditor || m.editor.mode != edEditing {
		t.Fatalf("quit %v screen %v mode %v, want to stay in the editor", quit, m.screen, m.editor.mode)
	}
}

// TestVimPaletteInVisual checks the formatting palette on a VISUAL
// selection: the format wraps the whole selection.
func TestVimPaletteInVisual(t *testing.T) {
	m, _, _ := openVimNote(t, "hello")

	m, _ = typeKeys(m, "0vl")
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.editor.mode != edFormatPalette {
		t.Fatalf("mode = %v after ctrl+t, want the palette", m.editor.mode)
	}
	m, _ = typeKeys(m, "b")

	if got := m.editor.ed.Lines()[0]; got != "**he**llo" {
		t.Fatalf("text = %q, want %q", got, "**he**llo")
	}
	if !strings.Contains(m.editor.ed.Footer(), "NORMAL") {
		t.Errorf("footer = %q, want NORMAL mode after formatting", m.editor.ed.Footer())
	}
}

// TestVimHelpKeepsMode checks that opening and closing the help does not
// change the vim mode: its esc closes the help only.
func TestVimHelpKeepsMode(t *testing.T) {
	m, _, _ := openVimNote(t, "hello")

	m, _ = typeKeys(m, "i")
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyF1})
	if !m.helpOpen || m.helpTab != helpTabEditor {
		t.Fatalf("help open %v tab %d, want the editor page", m.helpOpen, m.helpTab)
	}
	m, _ = typeKeys(m, "\x1b")

	if m.helpOpen || !strings.Contains(m.editor.ed.Footer(), "INSERT") {
		t.Fatalf("help open %v footer %q, want help closed and INSERT kept", m.helpOpen, m.editor.ed.Footer())
	}
}

// TestVimSurvivesResize checks that a terminal resize keeps the mode and the
// text, and that the footer follows the new width.
func TestVimSurvivesResize(t *testing.T) {
	m, _, _ := openVimNote(t, "hello")
	m, _ = typeKeys(m, "v")

	model, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	m = model.(Model)

	footer := m.editor.ed.Footer()
	if !strings.Contains(footer, "VISUAL") {
		t.Errorf("footer = %q, want VISUAL kept", footer)
	}
	if got := m.editor.ed.Lines()[0]; got != "hello" {
		t.Errorf("text = %q, want %q", got, "hello")
	}
}
