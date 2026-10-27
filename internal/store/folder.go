package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrFolderExists viene restituito quando si tenta di creare o rinominare una
// cartella con un nome già usato da un'altra cartella con lo stesso genitore
// (confronto case-insensitive).
var ErrFolderExists = errors.New("una cartella con questo nome esiste già")

// Folder è una cartella di note. ParentID nil vuol dire che la cartella sta
// alla radice.
type Folder struct {
	ID        int64
	Name      string
	ParentID  *int64
	CreatedAt time.Time
}

// subtreeSQL seleziona l'id e tutti i suoi discendenti dalla tabella folders.
// Il parametro è l'id di partenza e viene ripetuto dal chiamante.
const subtreeSQL = `(WITH RECURSIVE sub(id) AS (
        SELECT id FROM folders WHERE id = ?
        UNION ALL
        SELECT f.id FROM folders f JOIN sub ON f.parent_id = sub.id
    ) SELECT id FROM sub)`

// idValue traduce un eventuale id di cartella in valore da passare a SQLite:
// la radice è 0, così IFNULL(folder_id, 0) = 0 confronta le note senza cartella.
func idValue(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

// CreateFolder crea una cartella dentro parentID (nil = radice).
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

// ListFolders restituisce le cartelle attive con il genitore indicato (nil =
// radice), ordinate per nome.
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

// GetFolder restituisce la cartella con l'id indicato.
func (s *Store) GetFolder(id int64) (*Folder, error) {
	row := s.db.QueryRow("SELECT id, name, parent_id, created_at FROM folders WHERE id = ?", id)
	return scanFolder(row)
}

// RenameFolder cambia il nome di una cartella.
func (s *Store) RenameFolder(id int64, name string) error {
	_, err := s.db.Exec("UPDATE folders SET name = ? WHERE id = ?", name, id)
	if err != nil && isUniqueViolation(err) {
		return ErrFolderExists
	}
	return err
}

// SoftDeleteFolder sposta nel cestino la cartella, tutte le sue sottocartelle e
// tutte le note che contengono.
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

// FolderPath restituisce la catena di cartelle dalla radice alla cartella id,
// quest'ultima compresa. Una cartella inesistente dà un percorso vuoto.
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
		if len(rev) > 64 { // guardia contro un eventuale ciclo nel database
			break
		}
	}
	out := make([]Folder, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return out, nil
}

// CountNotes conta le note attive contenute direttamente nella cartella
// folderID (nil = radice).
func (s *Store) CountNotes(folderID *int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM notes WHERE deleted_at IS NULL AND IFNULL(folder_id, 0) = ?",
		idValue(folderID),
	).Scan(&n)
	return n, err
}

// rowScannerFolder è soddisfatta sia da *sql.Row sia da *sql.Rows.
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
