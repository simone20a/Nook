package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newTestStore opens a store on a temporary directory and closes it at the end of the test.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	st, err := Open(root)
	if err != nil {
		t.Fatalf("Open(%q): %v", root, err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return st
}

func TestCreateAndList(t *testing.T) {
	st := newTestStore(t)

	note, err := st.Create("Ideas")
	if err != nil {
		t.Fatalf("Create(\"Ideas\"): %v", err)
	}
	if note.Title != "Ideas" {
		t.Fatalf("Title = %q, want %q", note.Title, "Ideas")
	}
	if note.Body != "" {
		t.Fatalf("Body = %q, want empty", note.Body)
	}
	if note.ID <= 0 {
		t.Fatalf("ID = %d, want > 0", note.ID)
	}

	notes, err := st.List(false)
	if err != nil {
		t.Fatalf("List(false): %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("List(false) has %d items, want 1", len(notes))
	}
	if notes[0].Title != "Ideas" {
		t.Fatalf("List(false)[0].Title = %q, want %q", notes[0].Title, "Ideas")
	}
}

func TestDuplicateTitleCaseInsensitive(t *testing.T) {
	st := newTestStore(t)

	if _, err := st.Create("Ideas"); err != nil {
		t.Fatalf("first Create(\"Ideas\"): %v", err)
	}

	_, err := st.Create("ideas")
	if !errors.Is(err, ErrTitleExists) {
		t.Fatalf("second Create(\"ideas\") returned error %v, want ErrTitleExists", err)
	}
}

func TestRename(t *testing.T) {
	st := newTestStore(t)

	note, err := st.Create("A")
	if err != nil {
		t.Fatalf("Create(\"A\"): %v", err)
	}
	id := note.ID

	if err := st.Rename(id, "B"); err != nil {
		t.Fatalf("Rename(%d, \"B\"): %v", id, err)
	}
	got, err := st.Get(id)
	if err != nil {
		t.Fatalf("Get(%d) after rename: %v", id, err)
	}
	if got.Title != "B" {
		t.Fatalf("after Rename Title = %q, want %q", got.Title, "B")
	}
	if got.Body != note.Body {
		t.Fatalf("after Rename Body = %q, want unchanged %q", got.Body, note.Body)
	}

	if _, err := st.Create("C"); err != nil {
		t.Fatalf("Create(\"C\"): %v", err)
	}
	if err := st.Rename(id, "C"); !errors.Is(err, ErrTitleExists) {
		t.Fatalf("Rename(%d, \"C\") returned error %v, want ErrTitleExists", id, err)
	}
}

func TestSave(t *testing.T) {
	st := newTestStore(t)

	note, err := st.Create("A")
	if err != nil {
		t.Fatalf("Create(\"A\"): %v", err)
	}
	id := note.ID

	const body = "body with \n two lines"
	if err := st.Save(id, "A", body); err != nil {
		t.Fatalf("Save(%d, \"A\", ...): %v", id, err)
	}

	got, err := st.Get(id)
	if err != nil {
		t.Fatalf("Get(%d) after Save: %v", id, err)
	}
	if got.Body != body {
		t.Fatalf("Body = %q, want %q", got.Body, body)
	}
	if got.UpdatedAt.Before(got.CreatedAt) {
		t.Fatalf("UpdatedAt (%v) is before CreatedAt (%v)", got.UpdatedAt, got.CreatedAt)
	}
}

func TestSoftDeleteRestorePurge(t *testing.T) {
	st := newTestStore(t)

	note, err := st.Create("A")
	if err != nil {
		t.Fatalf("Create(\"A\"): %v", err)
	}
	id := note.ID

	if err := st.SoftDelete(id); err != nil {
		t.Fatalf("SoftDelete(%d): %v", id, err)
	}
	active, err := st.List(false)
	if err != nil {
		t.Fatalf("List(false) after SoftDelete: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("List(false) after SoftDelete has %d items, want 0", len(active))
	}
	deleted, err := st.List(true)
	if err != nil {
		t.Fatalf("List(true) after SoftDelete: %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("List(true) after SoftDelete has %d items, want 1", len(deleted))
	}

	if err := st.Restore(id); err != nil {
		t.Fatalf("Restore(%d): %v", id, err)
	}
	active, err = st.List(false)
	if err != nil {
		t.Fatalf("List(false) after Restore: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("List(false) after Restore has %d items, want 1", len(active))
	}

	if err := st.SoftDelete(id); err != nil {
		t.Fatalf("second SoftDelete(%d): %v", id, err)
	}
	if err := st.Purge(id); err != nil {
		t.Fatalf("Purge(%d): %v", id, err)
	}
	deleted, err = st.List(true)
	if err != nil {
		t.Fatalf("List(true) after Purge: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("List(true) after Purge has %d items, want 0", len(deleted))
	}
}

// TestNotesAreMarkdownFiles checks that notes are stored as "<title>.md"
// files with the body as content, inside folders that are directories.
func TestNotesAreMarkdownFiles(t *testing.T) {
	st := newTestStore(t)

	folder, err := st.CreateFolder("ideas", nil)
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	note, err := st.CreateIn("plan", &folder.ID)
	if err != nil {
		t.Fatalf("CreateIn: %v", err)
	}
	if err := st.Save(note.ID, "plan", "# Plan\n"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(st.Root(), "ideas", "plan.md"))
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	if string(data) != "# Plan\n" {
		t.Fatalf("file content = %q, want %q", data, "# Plan\n")
	}

	// Renaming through Save renames the file and keeps the id.
	if err := st.Save(note.ID, "roadmap", "# Plan\n"); err != nil {
		t.Fatalf("Save with a new title: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.Root(), "ideas", "roadmap.md")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.Root(), "ideas", "plan.md")); !os.IsNotExist(err) {
		t.Fatalf("old file still there (err = %v)", err)
	}
	got, err := st.Get(note.ID)
	if err != nil || got.Title != "roadmap" {
		t.Fatalf("Get after rename = %+v, %v; want title \"roadmap\"", got, err)
	}
}

// TestInvalidNames checks that names that cannot be file names are refused.
func TestInvalidNames(t *testing.T) {
	st := newTestStore(t)
	for _, name := range []string{"", "  ", "a/b", ".hidden", "..", "a\\b"} {
		if _, err := st.Create(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Create(%q) error = %v, want ErrInvalidName", name, err)
		}
		if _, err := st.CreateFolder(name, nil); !errors.Is(err, ErrInvalidName) {
			t.Errorf("CreateFolder(%q) error = %v, want ErrInvalidName", name, err)
		}
	}
}

// TestTrashKeepsFiles checks that a trashed note stays on disk under .trash
// and returns to its folder when restored.
func TestTrashKeepsFiles(t *testing.T) {
	st := newTestStore(t)

	folder, _ := st.CreateFolder("work", nil)
	note, err := st.CreateIn("todo", &folder.ID)
	if err != nil {
		t.Fatalf("CreateIn: %v", err)
	}
	if err := st.SoftDelete(note.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.Root(), "work", "todo.md")); !os.IsNotExist(err) {
		t.Fatalf("file still in the folder after SoftDelete (err = %v)", err)
	}
	if err := st.Restore(note.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	got, err := st.Get(note.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FolderID == nil || *got.FolderID != folder.ID {
		t.Fatalf("FolderID after Restore = %v, want %d", got.FolderID, folder.ID)
	}
}

// TestReopenKeepsNotes checks that a new store on the same directory sees
// the notes written by the previous one.
func TestReopenKeepsNotes(t *testing.T) {
	root := t.TempDir()
	st, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	n, _ := st.Create("kept")
	if err := st.Save(n.ID, "kept", "text"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	again, err := Open(root)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	notes, err := again.List(false)
	if err != nil || len(notes) != 1 || notes[0].Title != "kept" || notes[0].Body != "text" {
		t.Fatalf("List after reopening = %+v, %v; want the note \"kept\"", notes, err)
	}
}
