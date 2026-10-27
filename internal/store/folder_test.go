package store

import (
	"errors"
	"path/filepath"
	"testing"
)

// newFolderTestStore opens a store in a temporary directory of the test.
func newFolderTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestCreateAndListFolders checks that folders can be created, listed by
// parent, and that the name is unique only among siblings.
func TestCreateAndListFolders(t *testing.T) {
	st := newFolderTestStore(t)

	root, err := st.CreateFolder("projects", nil)
	if err != nil {
		t.Fatalf("creating folder: %v", err)
	}
	if root.ID <= 0 || root.Name != "projects" || root.ParentID != nil {
		t.Fatalf("created folder = %+v, want one with id, name and no parent", root)
	}

	child, err := st.CreateFolder("2026", &root.ID)
	if err != nil {
		t.Fatalf("creating subfolder: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("ParentID = %v, want %d", child.ParentID, root.ID)
	}

	// Same name at the same level: rejected, even with different capitalisation.
	if _, err := st.CreateFolder("Projects", nil); !errors.Is(err, ErrFolderExists) {
		t.Fatalf("creating same-named folder: err = %v, want ErrFolderExists", err)
	}
	// Same name at another level: allowed.
	if _, err := st.CreateFolder("projects", &root.ID); err != nil {
		t.Fatalf("creating same-named folder in another folder: %v", err)
	}

	roots, err := st.ListFolders(nil)
	if err != nil {
		t.Fatalf("listing roots: %v", err)
	}
	if len(roots) != 1 || roots[0].ID != root.ID {
		t.Fatalf("folders at the root = %+v, want only %d", roots, root.ID)
	}

	children, err := st.ListFolders(&root.ID)
	if err != nil {
		t.Fatalf("listing children: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("subfolders = %d, want 2", len(children))
	}
}

// TestFolderPath checks the root -> folder chain.
func TestFolderPath(t *testing.T) {
	st := newFolderTestStore(t)

	a, err := st.CreateFolder("a", nil)
	if err != nil {
		t.Fatalf("creating a: %v", err)
	}
	b, err := st.CreateFolder("b", &a.ID)
	if err != nil {
		t.Fatalf("creating b: %v", err)
	}
	c, err := st.CreateFolder("c", &b.ID)
	if err != nil {
		t.Fatalf("creating c: %v", err)
	}

	path, err := st.FolderPath(&c.ID)
	if err != nil {
		t.Fatalf("FolderPath: %v", err)
	}
	if len(path) != 3 || path[0].Name != "a" || path[1].Name != "b" || path[2].Name != "c" {
		t.Fatalf("path = %+v, want a › b › c", path)
	}

	if path, err := st.FolderPath(nil); err != nil || len(path) != 0 {
		t.Fatalf("FolderPath(nil) = %+v, %v; want empty", path, err)
	}
}

// TestNotesInFolders checks that notes stay in the folder they were
// created in, and that the per-folder count is correct.
func TestNotesInFolders(t *testing.T) {
	st := newFolderTestStore(t)

	folder, err := st.CreateFolder("ideas", nil)
	if err != nil {
		t.Fatalf("creating folder: %v", err)
	}
	if _, err := st.CreateIn("inside", &folder.ID); err != nil {
		t.Fatalf("creating note in the folder: %v", err)
	}
	if _, err := st.Create("outside"); err != nil {
		t.Fatalf("creating note at the root: %v", err)
	}

	inFolder, err := st.ListIn(&folder.ID, false, false)
	if err != nil {
		t.Fatalf("listing folder: %v", err)
	}
	if len(inFolder) != 1 || inFolder[0].Title != "inside" {
		t.Fatalf("notes in the folder = %+v, want only \"inside\"", inFolder)
	}
	if inFolder[0].FolderID == nil || *inFolder[0].FolderID != folder.ID {
		t.Errorf("FolderID = %v, want %d", inFolder[0].FolderID, folder.ID)
	}

	atRoot, err := st.ListIn(nil, false, false)
	if err != nil {
		t.Fatalf("listing root: %v", err)
	}
	if len(atRoot) != 1 || atRoot[0].Title != "outside" {
		t.Fatalf("notes at the root = %+v, want only \"outside\"", atRoot)
	}

	if n, err := st.CountNotes(&folder.ID); err != nil || n != 1 {
		t.Errorf("CountNotes(folder) = %d, %v; want 1", n, err)
	}
	if n, err := st.CountNotes(nil); err != nil || n != 1 {
		t.Errorf("CountNotes(root) = %d, %v; want 1", n, err)
	}

	// List remains the complete list, from any folder.
	all, err := st.List(false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List(false) = %d notes, want 2", len(all))
	}
}

// TestSoftDeleteFolder checks that trashing a folder also sends to the trash
// its subfolders and the notes it contains.
func TestSoftDeleteFolder(t *testing.T) {
	st := newFolderTestStore(t)

	parent, err := st.CreateFolder("parent", nil)
	if err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	child, err := st.CreateFolder("child", &parent.ID)
	if err != nil {
		t.Fatalf("creating child: %v", err)
	}
	noteInChild, err := st.CreateIn("inside", &child.ID)
	if err != nil {
		t.Fatalf("creating note: %v", err)
	}

	if err := st.SoftDeleteFolder(parent.ID); err != nil {
		t.Fatalf("SoftDeleteFolder: %v", err)
	}

	if roots, err := st.ListFolders(nil); err != nil || len(roots) != 0 {
		t.Fatalf("active folders = %+v, %v; want none", roots, err)
	}
	if notes, err := st.ListIn(nil, false, true); err != nil || len(notes) != 0 {
		t.Fatalf("active notes = %+v, %v; want none", notes, err)
	}
	trash, err := st.ListIn(nil, true, true)
	if err != nil {
		t.Fatalf("listing trash: %v", err)
	}
	if len(trash) != 1 || trash[0].ID != noteInChild.ID {
		t.Fatalf("notes in the trash = %+v, want note %d", trash, noteInChild.ID)
	}

	// Restoring the note brings it back to the root: its folder is in the trash,
	// so otherwise it would be active but invisible.
	if err := st.Restore(noteInChild.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	n, err := st.Get(noteInChild.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if n.FolderID != nil {
		t.Errorf("FolderID after restoring = %v, want nil (root)", n.FolderID)
	}
	if notes, err := st.ListIn(nil, false, false); err != nil || len(notes) != 1 {
		t.Errorf("active notes at the root = %+v, %v; want the restored note", notes, err)
	}
}

// TestRenameFolder checks renaming and uniqueness per level.
func TestRenameFolder(t *testing.T) {
	st := newFolderTestStore(t)

	a, err := st.CreateFolder("a", nil)
	if err != nil {
		t.Fatalf("creating a: %v", err)
	}
	if _, err := st.CreateFolder("b", nil); err != nil {
		t.Fatalf("creating b: %v", err)
	}
	if err := st.RenameFolder(a.ID, "b"); !errors.Is(err, ErrFolderExists) {
		t.Fatalf("renaming to an existing name: err = %v, want ErrFolderExists", err)
	}
	if err := st.RenameFolder(a.ID, "c"); err != nil {
		t.Fatalf("renaming: %v", err)
	}
	f, err := st.GetFolder(a.ID)
	if err != nil {
		t.Fatalf("GetFolder: %v", err)
	}
	if f.Name != "c" {
		t.Errorf("name after renaming = %q, want \"c\"", f.Name)
	}
}

// TestMigrateFromVersion1 checks that a database created by the previous
// version (without folders) is upgraded without losing notes.
func TestMigrateFromVersion1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	// v1 database: only the notes table, with one note in it.
	old, err := openRaw(path)
	if err != nil {
		t.Fatalf("opening old database: %v", err)
	}
	if _, err := old.Exec(createTableSQL); err != nil {
		t.Fatalf("creating notes table: %v", err)
	}
	if _, err := old.Exec(
		"INSERT INTO notes (title, body, created_at, updated_at) VALUES ('ancient', 'hello', ?, ?)",
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	); err != nil {
		t.Fatalf("inserting note: %v", err)
	}
	if _, err := old.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("schema version: %v", err)
	}
	old.Close()

	st, err := Open(path)
	if err != nil {
		t.Fatalf("opening migrated store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	notes, err := st.List(false)
	if err != nil {
		t.Fatalf("listing notes: %v", err)
	}
	if len(notes) != 1 || notes[0].Title != "ancient" {
		t.Fatalf("notes after migration = %+v, want the note \"ancient\"", notes)
	}
	if notes[0].FolderID != nil {
		t.Errorf("FolderID of the migrated note = %v, want nil", notes[0].FolderID)
	}

	// And folders now work.
	if _, err := st.CreateFolder("new", nil); err != nil {
		t.Fatalf("creating folder after migration: %v", err)
	}
}
