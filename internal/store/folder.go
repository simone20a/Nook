package store

import (
	"errors"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// ErrFolderExists is returned when trying to create or rename a folder with
// a name already used by another folder with the same parent
// (case-insensitive comparison).
var ErrFolderExists = errors.New("a folder with this name already exists")

// Folder is a folder of notes. ParentID nil means the folder is at the root.
type Folder struct {
	ID        int64
	Name      string
	ParentID  *int64
	CreatedAt time.Time
}

// CreateFolder creates a folder inside parentID (nil = root).
func (s *Store) CreateFolder(name string, parentID *int64) (*Folder, error) {
	name, err := validName(name)
	if err != nil {
		return nil, err
	}
	dir, err := s.folderRel(parentID)
	if err != nil {
		return nil, err
	}
	if nameTaken(s.abs(dir), name, true, "") {
		return nil, ErrFolderExists
	}
	rel := path.Join(dir, name)
	if err := os.Mkdir(s.abs(rel), 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrFolderExists
		}
		return nil, err
	}
	return s.GetFolder(s.idFor(rel))
}

// ListFolders returns the active folders with the given parent (nil = root),
// ordered by name.
func (s *Store) ListFolders(parentID *int64) ([]Folder, error) {
	dir, err := s.folderRel(parentID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.abs(dir))
	if err != nil {
		return nil, err
	}
	out := []Folder{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		f := Folder{ID: s.idFor(path.Join(dir, e.Name())), Name: e.Name(), ParentID: parentID}
		if info, err := e.Info(); err == nil {
			f.CreatedAt = info.ModTime()
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// GetFolder returns the folder with the given id.
func (s *Store) GetFolder(id int64) (*Folder, error) {
	rel, err := s.pathOf(id)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(s.abs(rel))
	if err != nil || !info.IsDir() {
		return nil, ErrNotFound
	}
	f := &Folder{ID: id, Name: path.Base(rel), CreatedAt: info.ModTime()}
	f.ParentID = s.folderIDOf(rel)
	return f, nil
}

// RenameFolder changes the name of a folder.
func (s *Store) RenameFolder(id int64, name string) error {
	name, err := validName(name)
	if err != nil {
		return err
	}
	rel, err := s.folderRel(&id)
	if err != nil || rel == "" {
		return ErrNotFound
	}
	newRel := path.Join(dirOf(rel), name)
	if newRel == rel {
		return nil
	}
	if nameTaken(s.abs(dirOf(rel)), name, true, path.Base(rel)) {
		return ErrFolderExists
	}
	if err := os.Rename(s.abs(rel), s.abs(newRel)); err != nil {
		return err
	}
	s.move(rel, newRel)
	return nil
}

// SoftDeleteFolder moves to the trash the folder, its subfolders and all the
// notes they contain.
func (s *Store) SoftDeleteFolder(id int64) error {
	rel, err := s.folderRel(&id)
	if err != nil || rel == "" {
		return ErrNotFound
	}
	return s.trash(rel)
}

// FolderPath returns the chain of folders from the root to the folder id,
// the latter included. A non-existent folder gives an empty path.
func (s *Store) FolderPath(id *int64) ([]Folder, error) {
	var rev []Folder
	for cur := id; cur != nil; {
		f, err := s.GetFolder(*cur)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				break
			}
			return nil, err
		}
		rev = append(rev, *f)
		cur = f.ParentID
		if len(rev) > 64 { // guard against a possible cycle
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
	dir, err := s.folderRel(folderID)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(s.abs(dir))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && isNoteFile(e.Name()) {
			n++
		}
	}
	return n, nil
}
