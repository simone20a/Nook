// Package store contains the SQLite repository for notes.
package store

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrTitleExists is returned when trying to use a title that is already
// present in the database (case-insensitive comparison).
var ErrTitleExists = errors.New("a note with this title already exists")

// Note is a note as it is stored in the store.
type Note struct {
	ID        int64
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
	// FolderID is the folder that contains the note; nil = root.
	FolderID *int64
}

// Store wraps the notes SQLite database.
type Store struct {
	db *sql.DB
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL UNIQUE COLLATE NOCASE,
    body TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);`

// createFoldersSQL creates the folders table: parent_id nil means
// root, and the name is unique among folders with the same parent.
const createFoldersSQL = `
CREATE TABLE IF NOT EXISTS folders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    parent_id INTEGER REFERENCES folders(id),
    created_at TEXT NOT NULL,
    deleted_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS folders_parent_name
    ON folders(IFNULL(parent_id, 0), name COLLATE NOCASE);`

// addNoteFolderSQL links notes to folders (migration 1 -> 2).
const addNoteFolderSQL = `ALTER TABLE notes ADD COLUMN folder_id INTEGER REFERENCES folders(id);`

// schemaVersion is the schema version written in PRAGMA user_version.
const schemaVersion = 2

// Open opens (or creates) the database at the given path and runs the migration.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// migrate brings the schema to the current version, starting from a new database
// (version 0) or from one created by a previous version of the application.
func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version >= schemaVersion {
		return nil
	}
	if version < 1 {
		if _, err := s.db.Exec(createTableSQL); err != nil {
			return err
		}
	}
	if version < 2 {
		if _, err := s.db.Exec(createFoldersSQL); err != nil {
			return err
		}
		// A new database already has all the columns; only old ones need to
		// be altered. The presence of the column is checked instead of the version
		// so as not to depend on the order of the migrations.
		has, err := s.hasColumn("notes", "folder_id")
		if err != nil {
			return err
		}
		if !has {
			if _, err := s.db.Exec(addNoteFolderSQL); err != nil {
				return err
			}
		}
	}
	_, err := s.db.Exec("PRAGMA user_version = " + strconv.Itoa(schemaVersion))
	return err
}

// hasColumn reports whether the table has the column.
func (s *Store) hasColumn(table, column string) (bool, error) {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Create inserts a new note at the root, with an empty body.
func (s *Store) Create(title string) (*Note, error) {
	return s.CreateIn(title, nil)
}

// CreateIn inserts a new note with an empty body in the folder folderID
// (nil = root) and returns the created note.
func (s *Store) CreateIn(title string, folderID *int64) (*Note, error) {
	now := nowString()
	res, err := s.db.Exec(
		"INSERT INTO notes (title, body, created_at, updated_at, folder_id) VALUES (?, '', ?, ?, ?)",
		title, now, now, folderID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrTitleExists
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

// List returns all notes, from any folder: active
// (includeDeleted == false) or deleted (includeDeleted == true),
// ordered by updated_at DESC, id DESC.
func (s *Store) List(includeDeleted bool) ([]Note, error) {
	query := "SELECT id, title, body, created_at, updated_at, deleted_at, folder_id FROM notes"
	if includeDeleted {
		query += " WHERE deleted_at IS NOT NULL"
	} else {
		query += " WHERE deleted_at IS NULL"
	}
	query += " ORDER BY updated_at DESC, id DESC"
	return s.queryNotes(query)
}

// ListIn returns the notes of a folder (folderID nil = root): active
// or deleted. With recursive true it also includes the notes of the
// subfolders, otherwise only those directly contained; applied to the
// root, recursive therefore includes the whole archive.
func (s *Store) ListIn(folderID *int64, includeDeleted, recursive bool) ([]Note, error) {
	query := "SELECT id, title, body, created_at, updated_at, deleted_at, folder_id FROM notes"
	if includeDeleted {
		query += " WHERE deleted_at IS NOT NULL"
	} else {
		query += " WHERE deleted_at IS NULL"
	}

	var args []any
	switch {
	case recursive && folderID == nil:
		// Recursive root: the whole archive, no filter on the folder.
	case recursive:
		query += " AND folder_id IN " + subtreeSQL
		args = append(args, *folderID)
	case folderID == nil:
		query += " AND IFNULL(folder_id, 0) = 0"
	default:
		query += " AND folder_id = ?"
		args = append(args, *folderID)
	}
	query += " ORDER BY updated_at DESC, id DESC"
	return s.queryNotes(query, args...)
}

// queryNotes runs a query that returns notes and collects them.
func (s *Store) queryNotes(query string, args ...any) ([]Note, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []Note{}
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, *note)
	}
	return notes, rows.Err()
}

// Get returns the note with the given id, even if deleted.
func (s *Store) Get(id int64) (*Note, error) {
	row := s.db.QueryRow(
		"SELECT id, title, body, created_at, updated_at, deleted_at, folder_id FROM notes WHERE id = ?", id,
	)
	return scanNote(row)
}

// Rename changes the title of a note without touching body and updated_at.
func (s *Store) Rename(id int64, newTitle string) error {
	_, err := s.db.Exec("UPDATE notes SET title = ? WHERE id = ?", newTitle, id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrTitleExists
		}
		return err
	}
	return nil
}

// Save saves title and body of a note, updating updated_at.
func (s *Store) Save(id int64, title, body string) error {
	_, err := s.db.Exec(
		"UPDATE notes SET title = ?, body = ?, updated_at = ? WHERE id = ?",
		title, body, nowString(), id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrTitleExists
		}
		return err
	}
	return nil
}

// SoftDelete marks the note as deleted.
func (s *Store) SoftDelete(id int64) error {
	_, err := s.db.Exec("UPDATE notes SET deleted_at = ? WHERE id = ?", nowString(), id)
	return err
}

// Restore undoes the deletion of a note. If its folder is in the trash
// or no longer exists, the note goes back to the root: otherwise it would be active but
// invisible.
func (s *Store) Restore(id int64) error {
	_, err := s.db.Exec(`
UPDATE notes SET deleted_at = NULL,
    folder_id = CASE
        WHEN folder_id IS NOT NULL
         AND NOT EXISTS (SELECT 1 FROM folders f WHERE f.id = notes.folder_id AND f.deleted_at IS NULL)
        THEN NULL ELSE folder_id END
WHERE id = ?`, id)
	return err
}

// Purge permanently deletes the note.
func (s *Store) Purge(id int64) error {
	_, err := s.db.Exec("DELETE FROM notes WHERE id = ?", id)
	return err
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanNote(sc rowScanner) (*Note, error) {
	var (
		note      Note
		createdAt string
		updatedAt string
		deletedAt sql.NullString
		folderID  sql.NullInt64
	)
	if err := sc.Scan(&note.ID, &note.Title, &note.Body, &createdAt, &updatedAt, &deletedAt, &folderID); err != nil {
		return nil, err
	}
	if folderID.Valid {
		id := folderID.Int64
		note.FolderID = &id
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, err
	}
	note.CreatedAt = created
	note.UpdatedAt = updated
	return &note, nil
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}

// openRaw opens the database without applying the migration: it is used by tests that
// need to build a database of a previous schema version.
func openRaw(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}
