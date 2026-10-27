package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// TestImportLegacyDB checks that a database of the old SQLite storage is
// turned into files: active notes in their folders, deleted ones in the trash.
func TestImportLegacyDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "notes.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	stmts := []string{
		`CREATE TABLE folders (id INTEGER PRIMARY KEY, name TEXT, parent_id INTEGER, created_at TEXT, deleted_at TEXT)`,
		`CREATE TABLE notes (id INTEGER PRIMARY KEY, title TEXT, body TEXT, created_at TEXT, updated_at TEXT, deleted_at TEXT, folder_id INTEGER)`,
		`INSERT INTO folders VALUES (1, 'projects', NULL, '2024-01-01T00:00:00Z', NULL)`,
		`INSERT INTO folders VALUES (2, 'nook', 1, '2024-01-01T00:00:00Z', NULL)`,
		`INSERT INTO notes VALUES (1, 'root note', 'hello', '2024-01-01T00:00:00Z', '2024-01-02T00:00:00Z', NULL, NULL)`,
		`INSERT INTO notes VALUES (2, 'deep/note', 'deep', '2024-01-01T00:00:00Z', '2024-01-02T00:00:00Z', NULL, 2)`,
		`INSERT INTO notes VALUES (3, 'gone', 'bye', '2024-01-01T00:00:00Z', '2024-01-02T00:00:00Z', '2024-02-01T00:00:00Z', NULL)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	db.Close()

	root := filepath.Join(dir, "notes")
	n, err := ImportLegacyDB(dbPath, root)
	if err != nil {
		t.Fatalf("ImportLegacyDB: %v", err)
	}
	if n != 3 {
		t.Fatalf("imported %d notes, want 3", n)
	}

	for rel, want := range map[string]string{
		"root note.md":                 "hello",
		"projects/nook/deep-note.md":   "deep",
		".trash/legacy-import/gone.md": "bye",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || string(data) != want {
			t.Errorf("%s = %q, %v; want %q", rel, data, err, want)
		}
	}

	st, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if active, _ := st.List(false); len(active) != 2 {
		t.Errorf("active notes = %d, want 2", len(active))
	}
	if trashed, _ := st.List(true); len(trashed) != 1 {
		t.Errorf("trashed notes = %d, want 1", len(trashed))
	}
}
