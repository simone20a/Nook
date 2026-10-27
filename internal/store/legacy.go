package store

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ImportLegacyDB copies the notes of an old SQLite database (the storage used
// before Nook kept plain files) into the notes directory root: active notes
// and folders become files and directories, deleted ones go to the trash. It
// returns the number of notes imported. The database itself is not modified.
func ImportLegacyDB(dbPath, root string) (int, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	type folderRow struct {
		name    string
		parent  sql.NullInt64
		deleted sql.NullString
	}
	folders := map[int64]folderRow{}
	if rows, err := db.Query("SELECT id, name, parent_id, deleted_at FROM folders"); err == nil {
		for rows.Next() {
			var id int64
			var f folderRow
			if err := rows.Scan(&id, &f.name, &f.parent, &f.deleted); err != nil {
				rows.Close()
				return 0, err
			}
			folders[id] = f
		}
		rows.Close()
	} // no folders table: a database from the first version of the schema

	// dirOf builds the relative directory of a folder, from the root.
	var dirOf func(id int64, depth int) string
	dirOf = func(id int64, depth int) string {
		f, ok := folders[id]
		if !ok || depth > 64 {
			return ""
		}
		parent := ""
		if f.parent.Valid {
			parent = dirOf(f.parent.Int64, depth+1)
		}
		return path.Join(parent, legacyName(f.name))
	}

	query := "SELECT title, body, updated_at, deleted_at, folder_id FROM notes"
	if has, _ := legacyHasColumn(db, "notes", "folder_id"); !has {
		query = "SELECT title, body, updated_at, deleted_at, NULL FROM notes"
	}
	rows, err := db.Query(query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	batch := path.Join(trashDir, "legacy-import")
	n := 0
	for rows.Next() {
		var (
			title, body, updated string
			deleted              sql.NullString
			folder               sql.NullInt64
		)
		if err := rows.Scan(&title, &body, &updated, &deleted, &folder); err != nil {
			return n, err
		}
		dir := ""
		if folder.Valid {
			dir = dirOf(folder.Int64, 0)
		}
		if deleted.Valid {
			dir = path.Join(batch, dir)
		}
		abs := filepath.Join(root, filepath.FromSlash(dir))
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return n, err
		}
		file, err := uniqueFile(abs, legacyName(title))
		if err != nil {
			return n, err
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			return n, err
		}
		if t, err := time.Parse(time.RFC3339, updated); err == nil {
			_ = os.Chtimes(file, t, t)
		}
		n++
	}
	return n, rows.Err()
}

// legacyHasColumn reports whether the table has the column.
func legacyHasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
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

// legacyName turns an old title or folder name into a safe file name.
func legacyName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.TrimLeft(name, ".")
	if name == "" {
		return "untitled"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// uniqueFile returns the path of "<name>.md" in dir, adding " (2)", " (3)"...
// when a file with that name (any case) already exists.
func uniqueFile(dir, name string) (string, error) {
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s (%d)", name, i)
		}
		if !nameTaken(dir, candidate+noteExt, false, "") {
			return filepath.Join(dir, candidate+noteExt), nil
		}
	}
}
