// Package store contiene il repository SQLite delle note.
package store

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrTitleExists viene restituito quando si tenta di usare un titolo già
// presente nella base dati (confronto case-insensitive).
var ErrTitleExists = errors.New("una nota con questo titolo esiste già")

// Note è una nota così come è memorizzata nello store.
type Note struct {
	ID        int64
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
	// FolderID è la cartella che contiene la nota; nil = radice.
	FolderID *int64
}

// Store avvolge il database SQLite delle note.
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

// createFoldersSQL crea la tabella delle cartelle: parent_id nil vuol dire
// radice, e il nome è unico fra cartelle con lo stesso genitore.
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

// addNoteFolderSQL collega le note alle cartelle (migrazione 1 -> 2).
const addNoteFolderSQL = `ALTER TABLE notes ADD COLUMN folder_id INTEGER REFERENCES folders(id);`

// schemaVersion è la versione dello schema scritta in PRAGMA user_version.
const schemaVersion = 2

// Open apre (o crea) il database al percorso indicato ed esegue la migrazione.
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

// migrate porta lo schema alla versione corrente, partendo da un database nuovo
// (versione 0) o da uno creato da una versione precedente dell'applicazione.
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
		// Un database nuovo ha già tutte le colonne; solo quelli vecchi vanno
		// alterati. Si controlla la presenza della colonna invece della versione
		// per non dipendere dall'ordine delle migrazioni.
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

// hasColumn indica se la tabella table ha la colonna column.
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

// Close chiude il database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Create inserisce una nuova nota alla radice, con corpo vuoto.
func (s *Store) Create(title string) (*Note, error) {
	return s.CreateIn(title, nil)
}

// CreateIn inserisce una nuova nota con corpo vuoto nella cartella folderID
// (nil = radice) e restituisce la nota creata.
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

// List restituisce tutte le note, di qualunque cartella: attive
// (includeDeleted == false) oppure eliminate (includeDeleted == true),
// ordinate per updated_at DESC, id DESC.
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

// ListIn restituisce le note di una cartella (folderID nil = radice): attive
// oppure eliminate. Con recursive true comprende anche le note delle
// sottocartelle, altrimenti solo quelle contenute direttamente; applicato alla
// radice, recursive comprende quindi l'intero archivio.
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
		// Radice ricorsiva: tutto l'archivio, nessun filtro sulla cartella.
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

// queryNotes esegue una query che restituisce note e le raccoglie.
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

// Get restituisce la nota con l'id indicato, anche se eliminata.
func (s *Store) Get(id int64) (*Note, error) {
	row := s.db.QueryRow(
		"SELECT id, title, body, created_at, updated_at, deleted_at, folder_id FROM notes WHERE id = ?", id,
	)
	return scanNote(row)
}

// Rename cambia il titolo di una nota senza toccare corpo e updated_at.
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

// Save salva titolo e corpo di una nota aggiornando updated_at.
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

// SoftDelete marca la nota come eliminata.
func (s *Store) SoftDelete(id int64) error {
	_, err := s.db.Exec("UPDATE notes SET deleted_at = ? WHERE id = ?", nowString(), id)
	return err
}

// Restore annulla l'eliminazione di una nota. Se la sua cartella è nel cestino
// o non esiste più, la nota torna alla radice: altrimenti sarebbe attiva ma
// invisibile.
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

// Purge elimina definitivamente la nota.
func (s *Store) Purge(id int64) error {
	_, err := s.db.Exec("DELETE FROM notes WHERE id = ?", id)
	return err
}

// rowScanner è soddisfatta sia da *sql.Row sia da *sql.Rows.
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

// openRaw apre il database senza applicare la migrazione: serve ai test che
// devono costruire un database di una versione precedente dello schema.
func openRaw(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}
