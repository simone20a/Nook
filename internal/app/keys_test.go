package app

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/editor"
	"nook/internal/store"
)

// newKeysTestStore apre lo store su una directory temporanea del test, come
// fanno gli altri test dell'app, e lo chiude alla fine.
func newKeysTestStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatalf("apertura store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestHelpF1Toggles verifica che f1 apra il popup di aiuto senza far uscire
// l'applicazione.
func TestHelpF1Toggles(t *testing.T) {
	m := Model{width: 80, height: 24}

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyF1})

	if quit {
		t.Fatal("quit = true, atteso false")
	}
	if !m2.helpOpen {
		t.Fatal("helpOpen = false, atteso true")
	}
}

// TestHelpQuestionToggles verifica che "?" apra l'aiuto quando la lista è in
// modalità normale.
func TestHelpQuestionToggles(t *testing.T) {
	st := newKeysTestStore(t)
	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	if quit {
		t.Fatal("quit = true, atteso false")
	}
	if !m2.helpOpen {
		t.Fatal("helpOpen = false, atteso true")
	}
}

// TestEditorCtrlQPopup verifica che ctrl+q su una nota modificata apra il popup
// Salva/Scarta/Annulla senza tornare alla lista e senza uscire.
func TestEditorCtrlQPopup(t *testing.T) {
	st := newKeysTestStore(t)
	note, err := st.Create("nota")
	if err != nil {
		t.Fatalf("creazione nota: %v", err)
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
		t.Fatal("quit = true, atteso false")
	}
	if m2.screen != screenEditor {
		t.Fatalf("screen = %v, atteso screenEditor: ctrl+q non deve tornare alla lista", m2.screen)
	}
	if m2.editor.mode != edPrompt {
		t.Fatalf("editor.mode = %v, atteso edPrompt (popup Salva/Scarta/Annulla aperto)", m2.editor.mode)
	}
	if !m2.editor.Dirty() {
		t.Fatal("editor.Dirty() = false, atteso true: la nota non è stata salvata")
	}
}

// TestEditorSpaceFlows verifica che la barra spaziatrice, che arriva come
// tea.KeySpace con la runa dello spazio, finisca nel testo dell'editor.
func TestEditorSpaceFlows(t *testing.T) {
	m := Model{width: 80, height: 24, screen: screenEditor}
	m.editor = editorScreen{
		ed:     editor.New([]string{"a"}, contentW(80), edHeight(contentH(24))),
		width:  80,
		height: 24,
	}

	m2, _, _ := m.updateKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	if got := m2.editor.ed.Lines()[0]; got != "a " {
		t.Fatalf("editor.ed.Lines()[0] = %q, atteso %q", got, "a ")
	}
}

// TestLetterSynonyms verifica i sinonimi a lettera singola della lista: "n"
// apre l'input della nuova nota e "q" esce dall'applicazione.
func TestLetterSynonyms(t *testing.T) {
	st := newKeysTestStore(t)
	if _, err := st.Create("nota"); err != nil {
		t.Fatalf("creazione nota: %v", err)
	}

	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)
	m.list.load()

	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if quit {
		t.Fatal("quit = true dopo \"n\", atteso false")
	}
	if m2.list.mode != modeInputNew {
		t.Fatalf("list.mode = %v dopo \"n\", atteso modeInputNew", m2.list.mode)
	}

	m3, _, quit := m2.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	if quit {
		t.Fatal("quit = true dopo esc, atteso false")
	}
	if m3.list.mode != modeNormal {
		t.Fatalf("list.mode = %v dopo esc, atteso modeNormal", m3.list.mode)
	}

	_, _, quit = m3.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !quit {
		t.Fatal("quit = false dopo \"q\", atteso true")
	}
}

// TestNewNoteStaysInList verifica che creare una nota non apra l'editor: la
// schermata resta la lista, la nota nuova è selezionata e compare nell'elenco.
func TestNewNoteStaysInList(t *testing.T) {
	st := newKeysTestStore(t)
	if _, err := st.Create("esistente"); err != nil {
		t.Fatalf("creazione nota: %v", err)
	}

	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)
	m.list.load()

	// "n" apre il popup del titolo, poi il testo e invio creano la nota.
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.list.mode != modeInputNew {
		t.Fatalf("list.mode = %v dopo \"n\", atteso modeInputNew", m.list.mode)
	}
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nuova")})
	m, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if quit {
		t.Fatal("quit = true dopo invio, atteso false")
	}

	if m.screen != screenList {
		t.Errorf("screen = %v dopo la creazione, atteso screenList (l'editor non deve aprirsi)", m.screen)
	}
	if m.list.mode != modeNormal {
		t.Errorf("list.mode = %v dopo la creazione, atteso modeNormal", m.list.mode)
	}
	if m.list.action == listOpen {
		t.Errorf("list.action = listOpen dopo la creazione, attesa nessuna azione")
	}

	var found bool
	for _, n := range m.list.notes {
		if n.Title == "nuova" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("la nota %q non compare nella lista: %+v", "nuova", m.list.notes)
	}
	if sel, ok := m.list.current(); !ok || sel.Title != "nuova" {
		t.Errorf("nota selezionata = %q, attesa %q", sel.Title, "nuova")
	}
}

// TestPaletteOpensAndApplies verifica che F3 apra la palette di formattazione
// (mode edFormatPalette, senza applicare nulla) e che invio applichi la voce
// corrente, cioè il grassetto, alla riga dell'editor sotto il cursore: la parola
// selezionata viene racchiusa fra "**".
func TestPaletteOpensAndApplies(t *testing.T) {
	const w, h = 80, 24

	// Il cursore parte in fondo alla riga (dopo "prova"), così SelectWord
	// seleziona la parola che lo precede e la palette ha una selezione su cui
	// applicare il grassetto.
	ed := editor.New([]string{"riga di prova"}, contentW(w), edHeight(contentH(h)))
	ed = ed.SelectWord()

	m := Model{width: w, height: h, screen: screenEditor}
	m.editor = editorScreen{
		ed:     ed,
		mode:   edEditing,
		width:  w,
		height: h,
	}

	// F3 apre la palette: il testo non cambia e il flag che la richiede viene
	// azzerato dalla schermata.
	m2, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyF3})
	if quit {
		t.Fatal("quit = true dopo f3, atteso false")
	}
	if m2.screen != screenEditor {
		t.Fatalf("screen = %v dopo f3, atteso screenEditor", m2.screen)
	}
	if m2.editor.mode != edFormatPalette {
		t.Fatalf("editor.mode = %v dopo f3, atteso edFormatPalette", m2.editor.mode)
	}
	if m2.editor.ed.WantsPalette {
		t.Error("WantsPalette = true dopo f3: la schermata deve azzerarlo")
	}
	if got := m2.editor.ed.Lines()[0]; got != "riga di prova" {
		t.Fatalf("riga = %q dopo f3, atteso %q: f3 non deve formattare", got, "riga di prova")
	}

	// Invio applica la voce corrente (la prima: grassetto "b") e chiude la
	// palette, tornando alla modalità di editing.
	m3, _, quit := m2.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if quit {
		t.Fatal("quit = true dopo invio, atteso false")
	}
	if m3.editor.mode != edEditing {
		t.Fatalf("editor.mode = %v dopo invio, atteso edEditing", m3.editor.mode)
	}
	if got, want := m3.editor.ed.Lines()[0], "riga di **prova**"; got != want {
		t.Fatalf("riga = %q dopo invio, atteso %q", got, want)
	}
	if got, want := m3.editor.ed.CursorCol1(), 11; got != want {
		t.Errorf("colonna del cursore = %d dopo il grassetto, attesa %d", got, want)
	}
}

// TestMultiClick verifica l'helper del multi-click a tempo esplicito: 1, 2 e 3
// click sulla stessa cella entro edMultiClickThreshold valgono come click
// singolo, doppio e triplo; oltre la soglia, su un'altra cella o dopo il terzo
// click il conteggio riparte da uno.
func TestMultiClick(t *testing.T) {
	const x, y = 7, 4
	base := time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)

	var prev edClick
	at := base
	for _, want := range []int{1, 2, 3} {
		got := edClickCount(prev, at, x, y)
		if got != want {
			t.Fatalf("edClickCount = %d per il click %d sulla stessa cella entro la soglia, atteso %d", got, want, want)
		}
		prev = edClick{x: x, y: y, count: got, at: at}
		at = at.Add(edMultiClickThreshold - time.Millisecond)
	}

	// La soglia è un limite: un click che arriva dopo non è più un doppio click.
	prev = edClick{x: x, y: y, count: 1, at: base}
	if got := edClickCount(prev, base.Add(edMultiClickThreshold+time.Millisecond), x, y); got != 1 {
		t.Errorf("edClickCount = %d per un click oltre la soglia, atteso 1", got)
	}

	// Un click su un'altra cella riparte da uno.
	if got := edClickCount(prev, base.Add(time.Millisecond), x+1, y); got != 1 {
		t.Errorf("edClickCount = %d per un click su un'altra cella, atteso 1", got)
	}

	// Dopo il triplo click, il quarto entro la soglia vale come un nuovo singolo.
	prev = edClick{x: x, y: y, count: 3, at: base}
	if got := edClickCount(prev, base.Add(time.Millisecond), x, y); got != 1 {
		t.Errorf("edClickCount = %d per il quarto click, atteso 1", got)
	}
}
