package store

import (
	"errors"
	"path/filepath"
	"testing"
)

// newTestStore opens a store on a temporary database and closes it at the end of the test.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
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
