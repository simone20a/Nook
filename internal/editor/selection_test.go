package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"nook/internal/theme"
)

// TestSelectionOnLine verifica il calcolo dell'intervallo selezionato per riga,
// comprese le righe intermedie di una selezione su più righe.
func TestSelectionOnLine(t *testing.T) {
	e := New([]string{"prima riga", "seconda riga", "terza riga"}, 40, 9)

	// Nessuna selezione.
	if _, _, ok := e.selectionOnLine(0); ok {
		t.Fatal("selectionOnLine senza selezione = true, want false")
	}

	// Selezione dalla colonna 2 della riga 0 alla colonna 4 della riga 2.
	e = e.SetSelection(0, 2, 2, 4)

	from, to, ok := e.selectionOnLine(0)
	if !ok || from != 2 || to != runeLen("prima riga") {
		t.Errorf("riga 0: [%d,%d) ok=%v, want [2,%d)", from, to, ok, runeLen("prima riga"))
	}
	from, to, ok = e.selectionOnLine(1)
	if !ok || from != 0 || to != runeLen("seconda riga") {
		t.Errorf("riga 1: [%d,%d) ok=%v, want [0,%d)", from, to, ok, runeLen("seconda riga"))
	}
	from, to, ok = e.selectionOnLine(2)
	if !ok || from != 0 || to != 4 {
		t.Errorf("riga 2: [%d,%d) ok=%v, want [0,4)", from, to, ok)
	}

	// La riga fuori dall'intervallo non è selezionata.
	if _, _, ok := e.selectionOnLine(3); ok {
		t.Error("riga 3 fuori dalla selezione: ok = true, want false")
	}
}

// TestSelectedText verifica il testo restituito per una selezione su una riga e
// su più righe.
func TestSelectedText(t *testing.T) {
	e := New([]string{"ciao mondo", "seconda", "terza"}, 40, 9)

	if _, ok := e.SelectedText(); ok {
		t.Fatal("SelectedText senza selezione = true, want false")
	}

	e = e.SetSelection(0, 0, 0, 4)
	if got, ok := e.SelectedText(); !ok || got != "ciao" {
		t.Errorf("SelectedText = %q, %v; want \"ciao\"", got, ok)
	}

	e = e.SetSelection(0, 5, 2, 5)
	want := "mondo\nseconda\nterza"
	if got, ok := e.SelectedText(); !ok || got != want {
		t.Errorf("SelectedText = %q, %v; want %q", got, ok, want)
	}
}

// TestSelectionRendered verifica che la selezione sia visibile nella view: la
// Focus Line e le righe rese contengono lo stile di selezione quando la
// selezione le attraversa.
func TestSelectionRendered(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	lines := []string{"testo semplice", "**grassetto** qui", "ultima riga"}
	e := New(lines, 40, 9)

	// Senza selezione la view non contiene lo stile di selezione.
	plain := e.View()
	if strings.Contains(plain, theme.Selection.Render("t")) {
		t.Fatal("la view senza selezione contiene già lo stile di selezione")
	}

	// Selezione dalla riga 0 alla riga 2: il cursore sta sulla riga 2, quindi le
	// righe 0 e 1 sono righe rese (non focus) e devono mostrare l'evidenziazione.
	e = e.SetSelection(0, 0, 2, 5)
	view := e.View()

	// Il testo resta lo stesso a video, ma con l'evidenziazione.
	stripped := ansi.Strip(view)
	for _, l := range lines {
		if !strings.Contains(stripped, strings.ReplaceAll(l, "**", "")) {
			t.Errorf("la view non contiene la riga %q:\n%s", l, stripped)
		}
	}
	if !strings.Contains(view, theme.Selection.Render(lines[0])) {
		t.Errorf("la riga resa e selezionata non è evidenziata:\n%q", view)
	}
	if !strings.Contains(view, theme.Selection.Render("grassetto")) {
		t.Errorf("il testo inline della selezione non è evidenziato:\n%q", view)
	}
}

// TestDragSelectionWithKeys verifica che shift+frecce estenda la selezione e che
// il testo selezionato cresca di conseguenza.
func TestDragSelectionWithKeys(t *testing.T) {
	e := New([]string{"abcdef"}, 40, 5)
	e = e.SetCursor(0, 0)

	e = press(e, tea.KeyMsg{Type: tea.KeyShiftRight})
	e = press(e, tea.KeyMsg{Type: tea.KeyShiftRight})
	e = press(e, tea.KeyMsg{Type: tea.KeyShiftRight})

	got, ok := e.SelectedText()
	if !ok || got != "abc" {
		t.Fatalf("SelectedText dopo tre shift+right = %q, %v; want \"abc\"", got, ok)
	}
}

// TestSetSelectionClamps verifica che SetSelection riporti riga e colonna dentro
// i limiti del documento.
func TestSetSelectionClamps(t *testing.T) {
	e := New([]string{"corta", "seconda"}, 40, 5)
	e = e.SetSelection(-3, 0, 99, 99)

	if e.cursorLine != 1 {
		t.Errorf("cursorLine = %d, want 1", e.cursorLine)
	}
	if e.cursorCol != runeLen("seconda") {
		t.Errorf("cursorCol = %d, want %d", e.cursorCol, runeLen("seconda"))
	}
	if e.anchorLine != 0 || e.anchorCol != 0 {
		t.Errorf("anchor = (%d,%d), want (0,0)", e.anchorLine, e.anchorCol)
	}
}

// TestSelectionReplacedByTyping verifica che scrivendo si sostituisca il testo
// selezionato, come ci si aspetta da un editor.
func TestSelectionReplacedByTyping(t *testing.T) {
	e := New([]string{"ciao mondo"}, 40, 5)
	e = e.SetSelection(0, 0, 0, 4)

	e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if got := e.Lines()[0]; got != "x mondo" {
		t.Errorf("riga dopo la sostituzione = %q, want %q", got, "x mondo")
	}
	if _, ok := e.SelectedText(); ok {
		t.Error("la selezione è ancora attiva dopo la scrittura")
	}
}
