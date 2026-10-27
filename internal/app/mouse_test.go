package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newDragTestScreen apre l'editor su una nota con tre righe di testo.
func newDragTestScreen(t *testing.T) editorScreen {
	t.Helper()

	st := newKeysTestStore(t)
	created, err := st.Create("nota")
	if err != nil {
		t.Fatalf("creazione nota: %v", err)
	}
	body := "prima riga di testo\nseconda riga\nterza riga"
	if err := st.Save(created.ID, "nota", body); err != nil {
		t.Fatalf("salvataggio: %v", err)
	}
	note, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return newEditorScreen(st, note, 60, 12)
}

// TestMouseDragSelects verifica che premendo il tasto sinistro e trascinando si
// selezioni il testo fra il punto premuto e la cella raggiunta, e che il
// rilascio lasci la selezione attiva.
func TestMouseDragSelects(t *testing.T) {
	s := newDragTestScreen(t)

	// Pressione sulla Focus Line (riga 5 dello schermo è la riga centrale), poi
	// trascinamento di una riga più in basso e di qualche colonna a destra.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 6, Y: 5})
	if !s.dragging {
		t.Fatal("dragging = false dopo la pressione, want true")
	}
	if _, ok := s.ed.SelectedText(); ok {
		t.Fatal("una semplice pressione ha già selezionato del testo")
	}

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 10, Y: 6})
	sel, ok := s.ed.SelectedText()
	if !ok {
		t.Fatal("dopo il trascinamento non c'è selezione")
	}
	if !strings.Contains(sel, "riga di testo") || !strings.Contains(sel, "seconda") {
		t.Errorf("testo selezionato = %q, want dal punto premuto attraverso la seconda riga", sel)
	}

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 10, Y: 6})
	if s.dragging {
		t.Error("dragging = true dopo il rilascio, want false")
	}
	if _, ok := s.ed.SelectedText(); !ok {
		t.Error("il rilascio ha azzerato la selezione, want che resti")
	}
}

// TestMouseMotionWithoutPressIgnored verifica che i movimenti del mouse senza una
// pressione in corso non selezionino nulla.
func TestMouseMotionWithoutPressIgnored(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 10, Y: 6})
	if _, ok := s.ed.SelectedText(); ok {
		t.Error("un movimento senza pressione ha selezionato del testo")
	}
}

// TestMouseClickClearsSelection verifica che un nuovo click azzeri la selezione
// precedente, fissando la nuova ancora.
func TestMouseClickClearsSelection(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 4, Y: 5})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 12, Y: 5})
	if _, ok := s.ed.SelectedText(); !ok {
		t.Fatal("il trascinamento non ha selezionato nulla")
	}
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 12, Y: 5})

	// Click in un punto diverso: la selezione sparisce e resta solo il cursore.
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 2, Y: 5})
	if _, ok := s.ed.SelectedText(); ok {
		t.Error("il click non ha azzerato la selezione precedente")
	}
}

// TestDoubleClickSelectsWord verifica che il doppio click selezioni la parola e
// che non lasci il trascinamento attivo.
func TestDoubleClickSelectsWord(t *testing.T) {
	s := newDragTestScreen(t)

	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 7, Y: 5})
	s, _ = s.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 7, Y: 5})
	if s.dragging {
		t.Error("dragging = true dopo il doppio click, want false")
	}
	sel, ok := s.ed.SelectedText()
	if !ok || strings.TrimSpace(sel) == "" {
		t.Fatalf("il doppio click non ha selezionato una parola (sel = %q)", sel)
	}
	if strings.Contains(sel, "\n") {
		t.Errorf("la selezione del doppio click attraversa più righe: %q", sel)
	}
}
