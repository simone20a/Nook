package store

import (
	"errors"
	"path/filepath"
	"testing"
)

// newFolderTestStore apre uno store su una directory temporanea del test.
func newFolderTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatalf("apertura store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestCreateAndListFolders verifica che le cartelle si creino, si elenchino per
// genitore e che il nome sia unico solo fra pari livello.
func TestCreateAndListFolders(t *testing.T) {
	st := newFolderTestStore(t)

	root, err := st.CreateFolder("progetti", nil)
	if err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}
	if root.ID <= 0 || root.Name != "progetti" || root.ParentID != nil {
		t.Fatalf("cartella creata = %+v, attesa con id, nome e nessun genitore", root)
	}

	child, err := st.CreateFolder("2026", &root.ID)
	if err != nil {
		t.Fatalf("creazione sottocartella: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("ParentID = %v, want %d", child.ParentID, root.ID)
	}

	// Omonima allo stesso livello: rifiutata, anche cambiando le maiuscole.
	if _, err := st.CreateFolder("Progetti", nil); !errors.Is(err, ErrFolderExists) {
		t.Fatalf("creazione omonima: err = %v, want ErrFolderExists", err)
	}
	// Omonima in un altro livello: ammessa.
	if _, err := st.CreateFolder("progetti", &root.ID); err != nil {
		t.Fatalf("creazione omonima in altra cartella: %v", err)
	}

	roots, err := st.ListFolders(nil)
	if err != nil {
		t.Fatalf("elenco radici: %v", err)
	}
	if len(roots) != 1 || roots[0].ID != root.ID {
		t.Fatalf("cartelle alla radice = %+v, want solo %d", roots, root.ID)
	}

	children, err := st.ListFolders(&root.ID)
	if err != nil {
		t.Fatalf("elenco figli: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("sottocartelle = %d, want 2", len(children))
	}
}

// TestFolderPath verifica la catena radice -> cartella.
func TestFolderPath(t *testing.T) {
	st := newFolderTestStore(t)

	a, err := st.CreateFolder("a", nil)
	if err != nil {
		t.Fatalf("creazione a: %v", err)
	}
	b, err := st.CreateFolder("b", &a.ID)
	if err != nil {
		t.Fatalf("creazione b: %v", err)
	}
	c, err := st.CreateFolder("c", &b.ID)
	if err != nil {
		t.Fatalf("creazione c: %v", err)
	}

	path, err := st.FolderPath(&c.ID)
	if err != nil {
		t.Fatalf("FolderPath: %v", err)
	}
	if len(path) != 3 || path[0].Name != "a" || path[1].Name != "b" || path[2].Name != "c" {
		t.Fatalf("percorso = %+v, want a › b › c", path)
	}

	if path, err := st.FolderPath(nil); err != nil || len(path) != 0 {
		t.Fatalf("FolderPath(nil) = %+v, %v; want vuoto", path, err)
	}
}

// TestNotesInFolders verifica che le note restino nella cartella in cui sono
// create e che il conteggio per cartella sia corretto.
func TestNotesInFolders(t *testing.T) {
	st := newFolderTestStore(t)

	folder, err := st.CreateFolder("idee", nil)
	if err != nil {
		t.Fatalf("creazione cartella: %v", err)
	}
	if _, err := st.CreateIn("dentro", &folder.ID); err != nil {
		t.Fatalf("creazione nota nella cartella: %v", err)
	}
	if _, err := st.Create("fuori"); err != nil {
		t.Fatalf("creazione nota alla radice: %v", err)
	}

	inFolder, err := st.ListIn(&folder.ID, false, false)
	if err != nil {
		t.Fatalf("elenco cartella: %v", err)
	}
	if len(inFolder) != 1 || inFolder[0].Title != "dentro" {
		t.Fatalf("note nella cartella = %+v, want solo \"dentro\"", inFolder)
	}
	if inFolder[0].FolderID == nil || *inFolder[0].FolderID != folder.ID {
		t.Errorf("FolderID = %v, want %d", inFolder[0].FolderID, folder.ID)
	}

	atRoot, err := st.ListIn(nil, false, false)
	if err != nil {
		t.Fatalf("elenco radice: %v", err)
	}
	if len(atRoot) != 1 || atRoot[0].Title != "fuori" {
		t.Fatalf("note alla radice = %+v, want solo \"fuori\"", atRoot)
	}

	if n, err := st.CountNotes(&folder.ID); err != nil || n != 1 {
		t.Errorf("CountNotes(cartella) = %d, %v; want 1", n, err)
	}
	if n, err := st.CountNotes(nil); err != nil || n != 1 {
		t.Errorf("CountNotes(radice) = %d, %v; want 1", n, err)
	}

	// List resta l'elenco completo, di qualunque cartella.
	all, err := st.List(false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List(false) = %d note, want 2", len(all))
	}
}

// TestSoftDeleteFolder verifica che cestinare una cartella porti nel cestino
// anche le sottocartelle e le note contenute.
func TestSoftDeleteFolder(t *testing.T) {
	st := newFolderTestStore(t)

	parent, err := st.CreateFolder("parent", nil)
	if err != nil {
		t.Fatalf("creazione parent: %v", err)
	}
	child, err := st.CreateFolder("child", &parent.ID)
	if err != nil {
		t.Fatalf("creazione child: %v", err)
	}
	noteInChild, err := st.CreateIn("dentro", &child.ID)
	if err != nil {
		t.Fatalf("creazione nota: %v", err)
	}

	if err := st.SoftDeleteFolder(parent.ID); err != nil {
		t.Fatalf("SoftDeleteFolder: %v", err)
	}

	if roots, err := st.ListFolders(nil); err != nil || len(roots) != 0 {
		t.Fatalf("cartelle attive = %+v, %v; want nessuna", roots, err)
	}
	if notes, err := st.ListIn(nil, false, true); err != nil || len(notes) != 0 {
		t.Fatalf("note attive = %+v, %v; want nessuna", notes, err)
	}
	trash, err := st.ListIn(nil, true, true)
	if err != nil {
		t.Fatalf("elenco cestino: %v", err)
	}
	if len(trash) != 1 || trash[0].ID != noteInChild.ID {
		t.Fatalf("note nel cestino = %+v, want la nota %d", trash, noteInChild.ID)
	}

	// Ripristinare la nota la riporta alla radice: la sua cartella è nel cestino,
	// quindi altrimenti sarebbe attiva ma invisibile.
	if err := st.Restore(noteInChild.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	n, err := st.Get(noteInChild.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if n.FolderID != nil {
		t.Errorf("FolderID dopo il ripristino = %v, want nil (radice)", n.FolderID)
	}
	if notes, err := st.ListIn(nil, false, false); err != nil || len(notes) != 1 {
		t.Errorf("note attive alla radice = %+v, %v; want la nota ripristinata", notes, err)
	}
}

// TestRenameFolder verifica la rinomina e l'unicità per livello.
func TestRenameFolder(t *testing.T) {
	st := newFolderTestStore(t)

	a, err := st.CreateFolder("a", nil)
	if err != nil {
		t.Fatalf("creazione a: %v", err)
	}
	if _, err := st.CreateFolder("b", nil); err != nil {
		t.Fatalf("creazione b: %v", err)
	}
	if err := st.RenameFolder(a.ID, "b"); !errors.Is(err, ErrFolderExists) {
		t.Fatalf("rinomina in un nome esistente: err = %v, want ErrFolderExists", err)
	}
	if err := st.RenameFolder(a.ID, "c"); err != nil {
		t.Fatalf("rinomina: %v", err)
	}
	f, err := st.GetFolder(a.ID)
	if err != nil {
		t.Fatalf("GetFolder: %v", err)
	}
	if f.Name != "c" {
		t.Errorf("nome dopo la rinomina = %q, want \"c\"", f.Name)
	}
}

// TestMigrateFromVersion1 verifica che un database creato dalla versione
// precedente (senza cartelle) venga aggiornato senza perdere le note.
func TestMigrateFromVersion1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vecchio.db")

	// Database v1: solo la tabella delle note, con una nota dentro.
	old, err := openRaw(path)
	if err != nil {
		t.Fatalf("apertura database vecchio: %v", err)
	}
	if _, err := old.Exec(createTableSQL); err != nil {
		t.Fatalf("creazione tabella note: %v", err)
	}
	if _, err := old.Exec(
		"INSERT INTO notes (title, body, created_at, updated_at) VALUES ('antica', 'ciao', ?, ?)",
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	); err != nil {
		t.Fatalf("inserimento nota: %v", err)
	}
	if _, err := old.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("versione schema: %v", err)
	}
	old.Close()

	st, err := Open(path)
	if err != nil {
		t.Fatalf("apertura store migrato: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	notes, err := st.List(false)
	if err != nil {
		t.Fatalf("elenco note: %v", err)
	}
	if len(notes) != 1 || notes[0].Title != "antica" {
		t.Fatalf("note dopo la migrazione = %+v, want la nota \"antica\"", notes)
	}
	if notes[0].FolderID != nil {
		t.Errorf("FolderID della nota migrata = %v, want nil", notes[0].FolderID)
	}

	// E le cartelle ora funzionano.
	if _, err := st.CreateFolder("nuova", nil); err != nil {
		t.Fatalf("creazione cartella dopo la migrazione: %v", err)
	}
}
