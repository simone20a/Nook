package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/editor"
	"nook/internal/store"
)

// newKeysTestStore opens the store in a temporary directory of the test, as
// the other app tests do, and closes it at the end.
func newKeysTestStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestHelpF1Toggles checks that f1 opens the help popup without making
// the application quit.
func TestHelpF1Toggles(t *testing.T) {
	m := Model{width: 80, height: 24}

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyF1})

	if quit {
		t.Fatal("quit = true, want false")
	}
	if !m2.helpOpen {
		t.Fatal("helpOpen = false, want true")
	}
}

// TestHelpQuestionToggles checks that "?" opens help when the list is in
// normal mode.
func TestHelpQuestionToggles(t *testing.T) {
	st := newKeysTestStore(t)
	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	if quit {
		t.Fatal("quit = true, want false")
	}
	if !m2.helpOpen {
		t.Fatal("helpOpen = false, want true")
	}
}

// TestEditorCtrlQPopup checks that ctrl+q on a modified note opens the
// Save/Discard/Cancel popup without going back to the list and without quitting.
func TestEditorCtrlQPopup(t *testing.T) {
	st := newKeysTestStore(t)
	note, err := st.Create("note")
	if err != nil {
		t.Fatalf("creating note: %v", err)
	}

	ed := editor.New([]string{"a"}, contentW(80), edHeight(contentH(24)))
	ed.Dirty = true

	m := Model{st: st, width: 80, height: 24, screen: screenEditor}
	m.editor = editorScreen{
		ed:     ed,
		note:   *note,
		st:     st,
		mode:   edEditing,
		width:  80,
		height: 24,
	}

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlQ})

	if quit {
		t.Fatal("quit = true, want false")
	}
	if m2.screen != screenEditor {
		t.Fatalf("screen = %v, want screenEditor: ctrl+q must not go back to the list", m2.screen)
	}
	if m2.editor.mode != edPrompt {
		t.Fatalf("editor.mode = %v, want edPrompt (Save/Discard/Cancel popup open)", m2.editor.mode)
	}
	if !m2.editor.Dirty() {
		t.Fatal("editor.Dirty() = false, want true: the note has not been saved")
	}
}

// TestEditorSpaceFlows checks that the space bar, which arrives as
// tea.KeySpace with the space rune, ends up in the editor's text.
func TestEditorSpaceFlows(t *testing.T) {
	m := Model{width: 80, height: 24, screen: screenEditor}
	m.editor = editorScreen{
		ed:     editor.New([]string{"a"}, contentW(80), edHeight(contentH(24))),
		width:  80,
		height: 24,
	}

	m2, _, _ := m.updateKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	if got := m2.editor.ed.Lines()[0]; got != "a " {
		t.Fatalf("editor.ed.Lines()[0] = %q, want %q", got, "a ")
	}
}

// TestLetterSynonyms checks the single-letter synonyms of the list: "n"
// opens the new note input and "q" quits the application.
func TestLetterSynonyms(t *testing.T) {
	st := newKeysTestStore(t)
	if _, err := st.Create("note"); err != nil {
		t.Fatalf("creating note: %v", err)
	}

	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)
	m.list.load()

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if quit {
		t.Fatal("quit = true after \"n\", want false")
	}
	if m2.list.mode != modeInputNew {
		t.Fatalf("list.mode = %v after \"n\", want modeInputNew", m2.list.mode)
	}

	m3, _, quit := m2.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	if quit {
		t.Fatal("quit = true after esc, want false")
	}
	if m3.list.mode != modeNormal {
		t.Fatalf("list.mode = %v after esc, want modeNormal", m3.list.mode)
	}

	_, _, quit = m3.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !quit {
		t.Fatal("quit = false after \"q\", want true")
	}
}

// TestNewNoteStaysInList checks that creating a note does not open the editor: the
// screen stays the list, the new note is selected and appears in the list.
func TestNewNoteStaysInList(t *testing.T) {
	st := newKeysTestStore(t)
	if _, err := st.Create("existing"); err != nil {
		t.Fatalf("creating note: %v", err)
	}

	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)
	m.list.load()

	// "n" opens the title popup, then the text and enter create the note.
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.list.mode != modeInputNew {
		t.Fatalf("list.mode = %v after \"n\", want modeInputNew", m.list.mode)
	}
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("fresh")})
	m, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if quit {
		t.Fatal("quit = true after enter, want false")
	}

	if m.screen != screenList {
		t.Errorf("screen = %v after creation, want screenList (the editor must not open)", m.screen)
	}
	if m.list.mode != modeNormal {
		t.Errorf("list.mode = %v after creation, want modeNormal", m.list.mode)
	}
	if m.list.action == listOpen {
		t.Errorf("list.action = listOpen after creation, want no action")
	}

	var found bool
	for _, n := range m.list.notes {
		if n.Title == "fresh" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("the note %q does not appear in the list: %+v", "fresh", m.list.notes)
	}
	if sel, ok := m.list.current(); !ok || sel.Title != "fresh" {
		t.Errorf("selected note = %q, want %q", sel.Title, "fresh")
	}
}

// TestPaletteOpensAndApplies checks that ctrl+t opens the formatting palette
// (mode edFormatPalette, without applying anything) and that enter applies the
// current entry, that is bold, to the editor line under the cursor: the word
// selected is wrapped in "**".
func TestPaletteOpensAndApplies(t *testing.T) {
	const w, h = 80, 24

	// The cursor starts at the end of the line (after "line"), so SelectWord
	// selects the word that precedes it and the palette has a selection on which
	// to apply bold.
	ed := editor.New([]string{"a test line"}, contentW(w), edHeight(contentH(h)))
	ed = ed.SelectWord()

	m := Model{width: w, height: h, screen: screenEditor}
	m.editor = editorScreen{
		ed:     ed,
		mode:   edEditing,
		width:  w,
		height: h,
	}

	// ctrl+t opens the palette: the text does not change and the flag that requests it is
	// cleared by the screen.
	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlT})
	if quit {
		t.Fatal("quit = true after ctrl+t, want false")
	}
	if m2.screen != screenEditor {
		t.Fatalf("screen = %v after ctrl+t, want screenEditor", m2.screen)
	}
	if m2.editor.mode != edFormatPalette {
		t.Fatalf("editor.mode = %v after ctrl+t, want edFormatPalette", m2.editor.mode)
	}
	if m2.editor.ed.WantsPalette {
		t.Error("WantsPalette = true after ctrl+t: the screen must clear it")
	}
	if got := m2.editor.ed.Lines()[0]; got != "a test line" {
		t.Fatalf("line = %q after ctrl+t, want %q: ctrl+t must not format", got, "a test line")
	}

	// Enter applies the current entry (the first one: bold "b") and closes the
	// palette, going back to editing mode.
	m3, _, quit := m2.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if quit {
		t.Fatal("quit = true after enter, want false")
	}
	if m3.editor.mode != edEditing {
		t.Fatalf("editor.mode = %v after enter, want edEditing", m3.editor.mode)
	}
	if got, want := m3.editor.ed.Lines()[0], "a test **line**"; got != want {
		t.Fatalf("line = %q after enter, want %q", got, want)
	}
	if got, want := m3.editor.ed.CursorCol1(), 10; got != want {
		t.Errorf("cursor column = %d after bold, want %d", got, want)
	}
}

// TestMultiClick checks the multi-click helper with explicit time: 1, 2 and 3
// clicks on the same cell within edMultiClickThreshold count as a single,
// double and triple click; beyond the threshold, on another cell or after the third
// click the count restarts from one.
func TestMultiClick(t *testing.T) {
	const x, y = 7, 4
	base := time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)

	var prev edClick
	at := base
	for _, want := range []int{1, 2, 3} {
		got := edClickCount(prev, at, x, y)
		if got != want {
			t.Fatalf("edClickCount = %d for click %d on the same cell within the threshold, want %d", got, want, want)
		}
		prev = edClick{x: x, y: y, count: got, at: at}
		at = at.Add(edMultiClickThreshold - time.Millisecond)
	}

	// The threshold is a limit: a click that arrives later is no longer a double click.
	prev = edClick{x: x, y: y, count: 1, at: base}
	if got := edClickCount(prev, base.Add(edMultiClickThreshold+time.Millisecond), x, y); got != 1 {
		t.Errorf("edClickCount = %d for a click beyond the threshold, want 1", got)
	}

	// A click on another cell restarts from one.
	if got := edClickCount(prev, base.Add(time.Millisecond), x+1, y); got != 1 {
		t.Errorf("edClickCount = %d for a click on another cell, want 1", got)
	}

	// After the triple click, the fourth within the threshold counts as a new single.
	prev = edClick{x: x, y: y, count: 3, at: base}
	if got := edClickCount(prev, base.Add(time.Millisecond), x, y); got != 1 {
		t.Errorf("edClickCount = %d for the fourth click, want 1", got)
	}
}

// TestPaletteLetterApplies checks that pressing the letter of an entry while
// the palette is open applies it at once and goes back to editing.
func TestPaletteLetterApplies(t *testing.T) {
	const w, h = 80, 24
	ed := editor.New([]string{"a test line"}, contentW(w), edHeight(contentH(h))).SelectWord()

	m := Model{width: w, height: h, screen: screenEditor}
	m.editor = editorScreen{ed: ed, mode: edEditing, width: w, height: h}

	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlT})
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})

	if m.editor.mode != edEditing {
		t.Fatalf("editor.mode = %v after the letter, want edEditing", m.editor.mode)
	}
	if got, want := m.editor.ed.Lines()[0], "a test _line_"; got != want {
		t.Fatalf("line = %q after \"i\", want %q", got, want)
	}
}

// TestCtrlCCopiesSelection checks that ctrl+c copies the selected text in the
// editor instead of quitting, and that it quits when nothing is selected.
func TestCtrlCCopiesSelection(t *testing.T) {
	const w, h = 80, 24
	var copied string
	old := copyText
	copyText = func(s string) error { copied = s; return nil }
	t.Cleanup(func() { copyText = old })

	ed := editor.New([]string{"a test line"}, contentW(w), edHeight(contentH(h))).SelectWord()
	m := Model{width: w, height: h, screen: screenEditor}
	m.editor = editorScreen{ed: ed, mode: edEditing, width: w, height: h}

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if quit {
		t.Fatal("quit = true with a selection, want false (copy)")
	}
	if copied != "line" {
		t.Fatalf("copied = %q, want %q", copied, "line")
	}

	m.editor.ed = editor.New([]string{"a test line"}, contentW(w), edHeight(contentH(h)))
	if _, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlC}); !quit {
		t.Fatal("quit = false without a selection, want true")
	}
	_ = m2
}
