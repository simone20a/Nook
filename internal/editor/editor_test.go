package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"nook/internal/markdown"
	"nook/internal/theme"
)

// TestFocusRowCentered verifica che la riga del cursore stia al centro dello schermo.
func TestFocusRowCentered(t *testing.T) {
	e := New([]string{"a", "b", "c", "d", "e"}, 20, 5)

	if got := e.focusRow(); got != 2 {
		t.Fatalf("focusRow() = %d, atteso 2", got)
	}
	if got := e.docIndexForRow(0); got != -2 {
		t.Fatalf("docIndexForRow(0) = %d, atteso -2", got)
	}
	if got := e.docIndexForRow(2); got != 0 {
		t.Fatalf("docIndexForRow(2) = %d, atteso 0", got)
	}
	if got := e.docIndexForRow(4); got != 2 {
		t.Fatalf("docIndexForRow(4) = %d, atteso 2", got)
	}
}

// TestVerticalScroll verifica lo scorrimento verticale con il cursore.
func TestVerticalScroll(t *testing.T) {
	e := New([]string{"a", "b", "c", "d", "e"}, 20, 5)

	e = e.moveDown()
	e = e.moveDown()

	if e.cursorLine != 2 {
		t.Fatalf("cursorLine = %d, atteso 2", e.cursorLine)
	}
	if got := e.docIndexForRow(2); got != 2 {
		t.Fatalf("docIndexForRow(2) = %d, atteso 2", got)
	}
}

// TestHScroll verifica lo scorrimento orizzontale.
func TestHScroll(t *testing.T) {
	e := New([]string{"abcdefghij"}, 5, 3)

	e = e.moveEnd()
	if e.cursorCol != 10 {
		t.Fatalf("cursorCol dopo moveEnd = %d, atteso 10", e.cursorCol)
	}
	if e.hScroll != 6 {
		t.Fatalf("hScroll dopo moveEnd = %d, atteso 6", e.hScroll)
	}

	e = e.moveHome()
	if e.hScroll != 0 {
		t.Fatalf("hScroll dopo moveHome = %d, atteso 0", e.hScroll)
	}
}

// TestClampCol verifica che la colonna sia riportata entro la fine della riga.
func TestClampCol(t *testing.T) {
	e := New([]string{"abc", "d"}, 20, 3)

	e = e.moveEnd()
	if e.cursorCol != 3 {
		t.Fatalf("cursorCol dopo moveEnd = %d, atteso 3", e.cursorCol)
	}

	e = e.moveDown()
	if e.cursorLine != 1 {
		t.Fatalf("cursorLine = %d, atteso 1", e.cursorLine)
	}
	if e.cursorCol != 1 {
		t.Fatalf("cursorCol = %d, atteso 1", e.cursorCol)
	}
}

// TestDeleteSelection verifica la cancellazione del testo selezionato.
func TestDeleteSelection(t *testing.T) {
	e := New([]string{"hello world"}, 20, 3)
	e.selecting = true
	e.anchorLine = 0
	e.anchorCol = 0
	e.cursorLine = 0
	e.cursorCol = 5

	e.deleteSelection()

	if e.lines[0] != " world" {
		t.Fatalf("lines[0] = %q, atteso %q", e.lines[0], " world")
	}
	if e.cursorCol != 0 {
		t.Fatalf("cursorCol = %d, atteso 0", e.cursorCol)
	}
	if e.selecting {
		t.Fatal("selecting = true, atteso false")
	}
}

// TestToggleBoldInserisce verifica l'inserimento dei marcatori del grassetto.
func TestToggleBoldInserisce(t *testing.T) {
	e := New([]string{"ciao"}, 20, 3)
	e.cursorCol = 2

	e.toggleBold()

	if e.lines[0] != "ci****ao" {
		t.Fatalf("lines[0] = %q, atteso %q", e.lines[0], "ci****ao")
	}
	if e.cursorCol != 4 {
		t.Fatalf("cursorCol = %d, atteso 4", e.cursorCol)
	}
}

// TestToggleBoldRimuove verifica la rimozione dei marcatori del grassetto.
func TestToggleBoldRimuove(t *testing.T) {
	e := New([]string{"**ciao**"}, 20, 3)
	e.cursorCol = 4

	e.toggleBold()

	if e.lines[0] != "ciao" {
		t.Fatalf("lines[0] = %q, atteso %q", e.lines[0], "ciao")
	}
}

// TestSetHeading verifica l'aggiunta di un'intestazione.
func TestSetHeading(t *testing.T) {
	e := New([]string{"titolo"}, 20, 3)

	e.setHeading(1)

	if e.lines[0] != "# titolo" {
		t.Fatalf("lines[0] = %q, atteso %q", e.lines[0], "# titolo")
	}
}

// TestSetHeadingToglie verifica la rimozione di un'intestazione dello stesso livello.
func TestSetHeadingToglie(t *testing.T) {
	e := New([]string{"## titolo"}, 20, 3)

	e.setHeading(2)

	if e.lines[0] != "titolo" {
		t.Fatalf("lines[0] = %q, atteso %q", e.lines[0], "titolo")
	}
}

// TestTruncateStyled verifica il troncamento consapevole delle celle larghe e
// delle sequenze ANSI: il risultato è misurato dopo aver tolto i codici colore.
func TestTruncateStyled(t *testing.T) {
	if got := truncate("abcdefghij", 5); got != "abcd…" {
		t.Fatalf("truncate(%q, 5) = %q, atteso %q", "abcdefghij", got, "abcd…")
	}

	if got := ansi.Strip(truncate("日本語", 3)); got != "日…" {
		t.Fatalf("truncate(%q, 3) = %q, atteso %q", "日本語", got, "日…")
	}

	rs := theme.Styles[markdown.StyleBold].Render("abc")
	if got := ansi.Strip(truncate(rs, 2)); got != "a…" {
		t.Fatalf("truncate(stile grassetto %q, 2) = %q, atteso %q", rs, got, "a…")
	}
}

// TestPrefixWidth verifica la larghezza del prefisso di blocco delle righe.
func TestPrefixWidth(t *testing.T) {
	casi := []struct {
		raw  string
		want int
	}{
		{"# T", 2},
		{"## T", 3},
		{"> c", 2},
		{"- v", 2},
		{"  - v", 4},
		{"- [ ] d", 6},
		{"1. v", 3},
		{"plain", 0},
	}
	for _, c := range casi {
		if got := markdown.PrefixWidth(c.raw); got != c.want {
			t.Errorf("PrefixWidth(%q) = %d, atteso %d", c.raw, got, c.want)
		}
	}
}

// TestIsInsideFence verifica il riconoscimento delle righe dentro un blocco di
// codice delimitato da fence.
func TestIsInsideFence(t *testing.T) {
	lines := []string{"```", "a", "```", "b"}

	if !isInsideFence(lines, 1) {
		t.Fatal("isInsideFence(lines, 1) = false, atteso true")
	}
	if isInsideFence(lines, 3) {
		t.Fatal("isInsideFence(lines, 3) = true, atteso false")
	}
}

// TestRenderFocusRowPad verifica che la riga di focus compensi il prefisso di
// blocco con uno spazio, così da non spostare il testo a video.
func TestRenderFocusRowPad(t *testing.T) {
	e := Editor{lines: []string{"# ciao"}, cursorLine: 0, cursorCol: 0, width: 10, height: 1}

	got := ansi.Strip(e.View())
	if len(got) < len("  # ciao") || got[:len("  # ciao")] != "  # ciao" {
		t.Fatalf("View() = %q, atteso inizio %q", got, "  # ciao")
	}
}

// TestSpaceInserts verifica che la barra spaziatrice inserisca uno spazio alla
// posizione del cursore sulla riga corrente.
func TestSpaceInserts(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	e2, ok := m.(Editor)
	if !ok {
		t.Fatalf("Update ha restituito %T, atteso Editor", m)
	}

	if got := e2.Lines()[0]; got != "a " {
		t.Fatalf("Lines()[0] = %q, atteso %q", got, "a ")
	}
}

// TestArrowMoves verifica che la freccia giù sposti il cursore alla riga
// successiva.
func TestArrowMoves(t *testing.T) {
	e := New([]string{"a", "b"}, 10, 5)

	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyDown})
	e2, ok := m.(Editor)
	if !ok {
		t.Fatalf("Update ha restituito %T, atteso Editor", m)
	}

	if got := e2.CursorRow(); got != 2 {
		t.Fatalf("CursorRow() = %d, atteso 2", got)
	}
}

// TestQuoteBarInView verifica che le citazioni mostrino la barra verticale anche
// sulle righe rese e sulla Focus Line, così la barra resta continua su tutto il
// blocco di citazione.
func TestQuoteBarInView(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	lines := []string{
		"testo normale",
		"> prima riga di citazione",
		"> seconda riga",
		">",
		"testo dopo",
	}
	e := New(lines, 50, 11)
	e = e.SetCursor(2, 0) // Focus Line sulla seconda riga di citazione

	view := ansi.Strip(e.View())
	rows := strings.Split(view, "\n")

	var withBar, quoted int
	for _, row := range rows {
		if strings.HasPrefix(row, markdown.QuoteBar) {
			withBar++
		}
		if strings.Contains(row, "riga di citazione") || strings.Contains(row, "seconda riga") {
			if !strings.HasPrefix(row, markdown.QuoteBar) {
				t.Errorf("riga di citazione senza barra a sinistra: %q", row)
			}
			quoted++
		}
	}
	if quoted != 2 {
		t.Errorf("trovate %d righe di citazione nella view, want 2:\n%s", quoted, view)
	}
	// Tre righe di citazione (compresa quella vuota, che qui è resa): tutte
	// portano la barra, compresa la Focus Line, che mostra il grezzo con il suo
	// ">" ma mantiene la barra a sinistra.
	if withBar != 3 {
		t.Errorf("righe con la barra = %d, want 3 (una per riga di citazione):\n%s", withBar, view)
	}
	// La Focus Line è grezza: mostra il marcatore oltre alla barra.
	if !strings.HasPrefix(strings.Split(view, "\n")[e.focusRow()], markdown.QuoteBar+" > ") {
		t.Errorf("la Focus Line non mostra la barra e il grezzo:\n%s", view)
	}

	// La riga "> " vuota è comunque una citazione: la barra non si interrompe.
	for _, row := range rows {
		if strings.TrimSpace(row) == markdown.QuoteBar {
			return
		}
	}
	t.Errorf("la riga di citazione vuota non porta la barra:\n%s", view)
}
