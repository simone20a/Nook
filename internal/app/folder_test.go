package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/store"
)

// newFolderTestStore apre lo store su una directory temporanea del test.
func newFolderTestStore(t *testing.T) *store.Store {
	return newKeysTestStore(t)
}

// TestFolderNavigation verifica che le cartelle compaiano nella lista, che la
// freccia destra (e invio) ci entrino dentro e che la freccia sinistra (e esc)
// risalgano al livello superiore.
func TestFolderNavigation(t *testing.T) {
	st := newFolderTestStore(t)
	folder, err := st.CreateFolder("progetti", nil)
	if err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}
	if _, err := st.CreateIn("dentro", &folder.ID); err != nil {
		t.Fatalf("creazione nota nella cartella: %v", err)
	}
	if _, err := st.Create("fuori"); err != nil {
		t.Fatalf("creazione nota alla radice: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	// Alla radice: una cartella e una nota.
	if got := len(s.folders); got != 1 {
		t.Fatalf("cartelle alla radice = %d, want 1", got)
	}
	if got := len(s.notes); got != 1 || s.notes[0].Title != "fuori" {
		t.Fatalf("note alla radice = %+v, want la sola \"fuori\"", s.notes)
	}

	// La prima voce è la cartella: freccia destra ci entra.
	s.selected = 0
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRight})
	if s.folderID == nil || *s.folderID != folder.ID {
		t.Fatalf("folderID dopo → = %v, want %d", s.folderID, folder.ID)
	}
	if len(s.path) != 1 || s.path[0].Name != "progetti" {
		t.Fatalf("percorso dopo → = %+v, want [progetti]", s.path)
	}
	if len(s.folders) != 0 {
		t.Errorf("cartelle dentro la cartella = %d, want 0", len(s.folders))
	}
	if len(s.notes) != 1 || s.notes[0].Title != "dentro" {
		t.Fatalf("note dentro la cartella = %+v, want la sola \"dentro\"", s.notes)
	}

	// Freccia sinistra: si torna alla radice.
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyLeft})
	if s.folderID != nil || len(s.path) != 0 {
		t.Fatalf("dopo ← folderID = %v, path = %+v; want radice", s.folderID, s.path)
	}
	if len(s.folders) != 1 || len(s.notes) != 1 {
		t.Errorf("alla radice: %d cartelle e %d note, want 1 e 1", len(s.folders), len(s.notes))
	}

	// Invio sulla cartella ci entra, esc ne esce.
	s.selected = 0
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if s.folderID == nil {
		t.Fatal("invio sulla cartella non è entrato")
	}
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	if s.folderID != nil {
		t.Fatal("esc non è risalito alla radice")
	}
}

// TestEnterOnNoteOpensEditor verifica che invio su una nota chieda l'apertura
// dell'editor, mentre la freccia destra su una nota non fa nulla.
func TestEnterOnNoteOpensEditor(t *testing.T) {
	st := newFolderTestStore(t)
	note, err := st.Create("nota")
	if err != nil {
		t.Fatalf("creazione nota: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRight})
	if s.action == listOpen {
		t.Error("→ su una nota ha aperto l'editor, want nessuna azione")
	}

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if s.action != listOpen || s.actionID != note.ID {
		t.Errorf("invio sulla nota: action = %v, id = %d; want listOpen su %d", s.action, s.actionID, note.ID)
	}
}

// TestCreateFolderKey verifica che "c" apra il popup della cartella e che invio
// la crei nel livello corrente, senza aprire l'editor.
func TestCreateFolderKey(t *testing.T) {
	st := newFolderTestStore(t)

	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)
	m.list.load()

	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if m.list.mode != modeInputNewFolder {
		t.Fatalf("mode dopo \"c\" = %v, want modeInputNewFolder", m.list.mode)
	}
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("idee")})
	m, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if quit {
		t.Fatal("quit = true dopo invio, want false")
	}
	if m.screen != screenList {
		t.Errorf("screen = %v, want screenList", m.screen)
	}
	if len(m.list.folders) != 1 || m.list.folders[0].Name != "idee" {
		t.Fatalf("cartelle = %+v, want la cartella \"idee\"", m.list.folders)
	}
	if e, ok := m.list.currentEntry(); !ok || e.kind != entryFolder || e.title != "idee/" {
		t.Errorf("voce selezionata = %+v, want la cartella \"idee/\"", e)
	}
}

// TestNewNoteGoesIntoCurrentFolder verifica che una nota creata dentro una
// cartella resti lì.
func TestNewNoteGoesIntoCurrentFolder(t *testing.T) {
	st := newFolderTestStore(t)
	folder, err := st.CreateFolder("progetti", nil)
	if err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()
	s.selected = 0
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRight})
	if s.folderID == nil {
		t.Fatal("non è entrato nella cartella")
	}

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("interna")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})

	if len(s.notes) != 1 || s.notes[0].Title != "interna" {
		t.Fatalf("note nella cartella = %+v, want la sola \"interna\"", s.notes)
	}
	got, err := st.Get(s.notes[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FolderID == nil || *got.FolderID != folder.ID {
		t.Errorf("FolderID = %v, want %d", got.FolderID, folder.ID)
	}
}

// TestDeleteFolderTrashesContent verifica che cestinando una cartella si porti
// nel cestino il suo contenuto e che la vista cestino la mostri come nota.
func TestDeleteFolderTrashesContent(t *testing.T) {
	st := newFolderTestStore(t)
	folder, err := st.CreateFolder("progetti", nil)
	if err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}
	note, err := st.CreateIn("dentro", &folder.ID)
	if err != nil {
		t.Fatalf("creazione nota: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()
	s.selected = 0

	// "d" chiede conferma, "s" conferma.
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if s.mode != modeConfirmSoftDelete {
		t.Fatalf("mode dopo \"d\" = %v, want modeConfirmSoftDelete", s.mode)
	}
	box := s.boxConfirm()
	if !strings.Contains(box, "cartella") {
		t.Errorf("la conferma non parla di cartella: %q", box)
	}
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	if len(s.folders) != 0 || len(s.notes) != 0 {
		t.Fatalf("dopo la cancellazione: %d cartelle, %d note; want 0 e 0", len(s.folders), len(s.notes))
	}

	// Nel cestino compare la nota che era dentro.
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if !s.showTrash {
		t.Fatal("non è passato al cestino")
	}
	if len(s.notes) != 1 || s.notes[0].ID != note.ID {
		t.Fatalf("note nel cestino = %+v, want la nota %d", s.notes, note.ID)
	}
}

// TestListTitleBreadcrumb verifica il titolo della lista: NOTE alla radice e il
// percorso quando si scende in una cartella.
func TestListTitleBreadcrumb(t *testing.T) {
	st := newFolderTestStore(t)
	root, err := st.CreateFolder("progetti", nil)
	if err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}
	child, err := st.CreateFolder("2026", &root.ID)
	if err != nil {
		t.Fatalf("creazione sottocartella: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	if got := strings.TrimSpace(s.listTitle(60)); got != "NOTE" {
		t.Errorf("titolo alla radice = %q, want \"NOTE\"", got)
	}

	s = s.enterFolder(root.ID)
	if got := strings.TrimSpace(s.listTitle(60)); got != "NOTE › progetti" {
		t.Errorf("titolo dentro la cartella = %q, want \"NOTE › progetti\"", got)
	}

	s = s.enterFolder(child.ID)
	if got := strings.TrimSpace(s.listTitle(60)); got != "NOTE › progetti › 2026" {
		t.Errorf("titolo nella sottocartella = %q, want il percorso completo", got)
	}

	s.showTrash = true
	if got := strings.TrimSpace(s.listTitle(60)); got != "CESTINO" {
		t.Errorf("titolo nel cestino = %q, want \"CESTINO\"", got)
	}
}

// TestFolderEntryDetail verifica la riga di dettaglio delle cartelle.
func TestFolderEntryDetail(t *testing.T) {
	st := newFolderTestStore(t)
	empty, err := st.CreateFolder("vuota", nil)
	if err != nil {
		t.Fatalf("creazione cartella vuota: %v", err)
	}
	one, err := st.CreateFolder("una", nil)
	if err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}
	if _, err := st.CreateIn("nota", &one.ID); err != nil {
		t.Fatalf("creazione nota: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	details := map[int64]string{}
	for _, e := range s.entries() {
		if e.kind == entryFolder {
			details[e.id] = e.detail
		}
	}
	if got := details[empty.ID]; got != "vuota" {
		t.Errorf("dettaglio cartella vuota = %q, want \"vuota\"", got)
	}
	if got := details[one.ID]; got != "1 nota" {
		t.Errorf("dettaglio cartella con una nota = %q, want \"1 nota\"", got)
	}
}

// TestNewNoteSelectsItAmongFolders verifica che, creando una nota in una lista
// che contiene cartelle, la selezione finisca sulla nota nuova e non su una
// cartella.
func TestNewNoteSelectsItAmongFolders(t *testing.T) {
	st := newFolderTestStore(t)
	if _, err := st.CreateFolder("cartella", nil); err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}
	if _, err := st.Create("esistente"); err != nil {
		t.Fatalf("creazione nota: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nuova")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})

	e, ok := s.currentEntry()
	if !ok || e.kind != entryNote || e.title != "nuova" {
		t.Fatalf("voce selezionata = %+v, want la nota \"nuova\"", e)
	}
}
