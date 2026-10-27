package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrFolderExists is returned when trying to create or rename a
// folder with a name already used by another folder with the same parent
// (case-insensitive comparison).
var ErrFolderExists = errors.New("a folder with this name already exists")

// Folder is a folder of notes. ParentID nil means the folder is at
// the root.
type Folder struct {
	ID        int64
	Name      string
	ParentID  *int64
	CreatedAt time.Time
}

// subtreeSQL selects the id and all its descendants from the folders table.
// The parameter is the starting id and is repeated by the caller.
const subtreeSQL = `(WITH RECURSIVE sub(id) AS (
        SELECT id FROM folders WHERE id = ?
        UNION ALL
        SELECT f.id FROM folders f JOIN sub ON f.parent_id = sub.id
    ) SELECT id FROM sub)`

// idValue turns an optional folder id into a value to pass to SQLite:
// the root is 0, so IFNULL(folder_id, 0) = 0 matches notes without a folder.
func idValue(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

// CreateFolder creates a folder inside parentID (nil = root).
func (s *Store) CreateFolder(name string, parentID *int64) (*Folder, error) {
	res, err := s.db.Exec(
		"INSERT INTO folders (name, parent_id, created_at) VALUES (?, ?, ?)",
		name, parentID, nowString(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrFolderExists
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetFolder(id)
}

// ListFolders returns the active folders with the given parent (nil =
// root), ordered by name.
func (s *Store) ListFolders(parentID *int64) ([]Folder, error) {
	query := "SELECT id, name, parent_id, created_at FROM folders WHERE deleted_at IS NULL AND IFNULL(parent_id, 0) = ? ORDER BY name COLLATE NOCASE"
	if parentID != nil {
		query = "SELECT id, name, parent_id, created_at FROM folders WHERE deleted_at IS NULL AND parent_id = ? ORDER BY name COLLATE NOCASE"
	}
	rows, err := s.db.Query(query, idValue(parentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Folder{}
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// GetFolder returns the folder with the given id.
func (s *Store) GetFolder(id int64) (*Folder, error) {
	row := s.db.QueryRow("SELECT id, name, parent_id, created_at FROM folders WHERE id = ?", id)
	return scanFolder(row)
}

// RenameFolder changes the name of a folder.
func (s *Store) RenameFolder(id int64, name string) error {
	_, err := s.db.Exec("UPDATE folders SET name = ? WHERE id = ?", name, id)
	if err != nil && isUniqueViolation(err) {
		return ErrFolderExists
	}
	return err
}

// SoftDeleteFolder moves to the trash the folder, all its subfolders and
// all the notes they contain.
func (s *Store) SoftDeleteFolder(id int64) error {
	now := nowString()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"UPDATE notes SET deleted_at = ? WHERE deleted_at IS NULL AND folder_id IN "+subtreeSQL,
		now, id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		"UPDATE folders SET deleted_at = ? WHERE deleted_at IS NULL AND id IN "+subtreeSQL,
		now, id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// FolderPath returns the chain of folders from the root to the folder id,
// the latter included. A non-existent folder gives an empty path.
func (s *Store) FolderPath(id *int64) ([]Folder, error) {
	var rev []Folder
	for cur := id; cur != nil; {
		f, err := s.GetFolder(*cur)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return nil, err
		}
		rev = append(rev, *f)
		cur = f.ParentID
		if len(rev) > 64 { // guard against a possible cycle in the database
			break
		}
	}
	out := make([]Folder, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return out, nil
}

// CountNotes counts the active notes contained directly in the folder
// folderID (nil = root).
func (s *Store) CountNotes(folderID *int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM notes WHERE deleted_at IS NULL AND IFNULL(folder_id, 0) = ?",
		idValue(folderID),
	).Scan(&n)
	return n, err
}

// rowScannerFolder is satisfied by both *sql.Row and *sql.Rows.
type rowScannerFolder interface {
	Scan(dest ...any) error
}

func scanFolder(sc rowScannerFolder) (*Folder, error) {
	var (
		f         Folder
		parentID  sql.NullInt64
		createdAt string
	)
	if err := sc.Scan(&f.ID, &f.Name, &parentID, &createdAt); err != nil {
		return nil, err
	}
	if parentID.Valid {
		id := parentID.Int64
		f.ParentID = &id
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	f.CreatedAt = created
	return &f, nil
}
