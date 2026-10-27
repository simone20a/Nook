package store

import (
	"errors"
	"path/filepath"
	"testing"
)

// newTestStore apre uno store su un database temporaneo e lo chiude a fine test.
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

	note, err := st.Create("Idee")
	if err != nil {
		t.Fatalf("Create(\"Idee\"): %v", err)
	}
	if note.Title != "Idee" {
		t.Fatalf("Title = %q, atteso %q", note.Title, "Idee")
	}
	if note.Body != "" {
		t.Fatalf("Body = %q, atteso vuoto", note.Body)
	}
	if note.ID <= 0 {
		t.Fatalf("ID = %d, atteso > 0", note.ID)
	}

	notes, err := st.List(false)
	if err != nil {
		t.Fatalf("List(false): %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("List(false) ha %d elementi, atteso 1", len(notes))
	}
	if notes[0].Title != "Idee" {
		t.Fatalf("List(false)[0].Title = %q, atteso %q", notes[0].Title, "Idee")
	}
}

func TestDuplicateTitleCaseInsensitive(t *testing.T) {
	st := newTestStore(t)

	if _, err := st.Create("Idee"); err != nil {
		t.Fatalf("primo Create(\"Idee\"): %v", err)
	}

	_, err := st.Create("idee")
	if !errors.Is(err, ErrTitleExists) {
		t.Fatalf("secondo Create(\"idee\") ha dato errore %v, atteso ErrTitleExists", err)
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
		t.Fatalf("Get(%d) dopo rename: %v", id, err)
	}
	if got.Title != "B" {
		t.Fatalf("dopo Rename Title = %q, atteso %q", got.Title, "B")
	}
	if got.Body != note.Body {
		t.Fatalf("dopo Rename Body = %q, atteso invariato %q", got.Body, note.Body)
	}

	if _, err := st.Create("C"); err != nil {
		t.Fatalf("Create(\"C\"): %v", err)
	}
	if err := st.Rename(id, "C"); !errors.Is(err, ErrTitleExists) {
		t.Fatalf("Rename(%d, \"C\") ha dato errore %v, atteso ErrTitleExists", id, err)
	}
}

func TestSave(t *testing.T) {
	st := newTestStore(t)

	note, err := st.Create("A")
	if err != nil {
		t.Fatalf("Create(\"A\"): %v", err)
	}
	id := note.ID

	const body = "corpo con \n due righe"
	if err := st.Save(id, "A", body); err != nil {
		t.Fatalf("Save(%d, \"A\", ...): %v", id, err)
	}

	got, err := st.Get(id)
	if err != nil {
		t.Fatalf("Get(%d) dopo Save: %v", id, err)
	}
	if got.Body != body {
		t.Fatalf("Body = %q, atteso %q", got.Body, body)
	}
	if got.UpdatedAt.Before(got.CreatedAt) {
		t.Fatalf("UpdatedAt (%v) è prima di CreatedAt (%v)", got.UpdatedAt, got.CreatedAt)
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
		t.Fatalf("List(false) dopo SoftDelete: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("List(false) dopo SoftDelete ha %d elementi, atteso 0", len(active))
	}
	deleted, err := st.List(true)
	if err != nil {
		t.Fatalf("List(true) dopo SoftDelete: %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("List(true) dopo SoftDelete ha %d elementi, atteso 1", len(deleted))
	}

	if err := st.Restore(id); err != nil {
		t.Fatalf("Restore(%d): %v", id, err)
	}
	active, err = st.List(false)
	if err != nil {
		t.Fatalf("List(false) dopo Restore: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("List(false) dopo Restore ha %d elementi, atteso 1", len(active))
	}

	if err := st.SoftDelete(id); err != nil {
		t.Fatalf("secondo SoftDelete(%d): %v", id, err)
	}
	if err := st.Purge(id); err != nil {
		t.Fatalf("Purge(%d): %v", id, err)
	}
	deleted, err = st.List(true)
	if err != nil {
		t.Fatalf("List(true) dopo Purge: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("List(true) dopo Purge ha %d elementi, atteso 0", len(deleted))
	}
}
