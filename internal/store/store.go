// Package store keeps the notes as plain markdown files: every note is a
// "<title>.md" file and every folder is a directory, all under one root
// directory (by default ~/.nook). Deleting a note or a folder removes it from
// the disk for good.
package store

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Errors returned by the store.
var (
	// ErrTitleExists is returned when a note with the same title (compared
	// case-insensitively) already exists in the folder.
	ErrTitleExists = errors.New("a note with this title already exists")
	// ErrNotFound is returned when an id does not match any note or folder.
	ErrNotFound = errors.New("not found")
	// ErrInvalidName is returned for names that cannot be a file name.
	ErrInvalidName = errors.New("invalid name: it cannot be empty, start with a dot or contain / or \\")
)

const noteExt = ".md"

// Note is a note as it is stored in the store.
type Note struct {
	ID    int64
	Title string
	Body  string
	// CreatedAt and UpdatedAt are both the modification time of the file:
	// the file system does not portably keep a creation time.
	CreatedAt time.Time
	UpdatedAt time.Time
	// FolderID is the folder that contains the note; nil = root.
	FolderID *int64
}

// Store is a directory of markdown files.
//
// Notes and folders are identified by int64 ids, which are handed out while
// scanning the directory and kept in memory: they are stable while the
// program runs (also across renames and moves) but are not
// stored anywhere.
type Store struct {
	root   string
	nextID int64
	byID   map[int64]string // id -> slash path relative to root
	byPath map[string]int64 // slash path relative to root -> id
}

// Open uses root as the notes directory, creating it if needed.
func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	s := &Store{root: root, nextID: 1, byID: map[int64]string{}, byPath: map[string]int64{}}
	if err := s.refresh(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close does nothing: it is there to keep the lifecycle of a store explicit.
func (s *Store) Close() error { return nil }

// Root returns the notes directory.
func (s *Store) Root() string { return s.root }

// --- Registry of ids ---

// abs turns a slash path relative to the root into a file system path.
func (s *Store) abs(rel string) string {
	return filepath.Join(s.root, filepath.FromSlash(rel))
}

// idFor returns the id of rel, handing out a new one if it has none.
func (s *Store) idFor(rel string) int64 {
	if id, ok := s.byPath[rel]; ok {
		return id
	}
	id := s.nextID
	s.nextID++
	s.byID[id] = rel
	s.byPath[rel] = id
	return id
}

// refresh walks the directory: new notes and folders get an id, the ones
// that disappeared lose it.
func (s *Store) refresh() error {
	seen := map[string]bool{}
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			s.idFor(rel)
			seen[rel] = true
			return nil
		}
		if isNoteFile(name) {
			s.idFor(rel)
			seen[rel] = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	for rel, id := range s.byPath {
		if !seen[rel] {
			delete(s.byPath, rel)
			delete(s.byID, id)
		}
	}
	return nil
}

// pathOf returns the path of an id, rescanning once if it is unknown.
func (s *Store) pathOf(id int64) (string, error) {
	if rel, ok := s.byID[id]; ok {
		return rel, nil
	}
	if err := s.refresh(); err != nil {
		return "", err
	}
	if rel, ok := s.byID[id]; ok {
		return rel, nil
	}
	return "", ErrNotFound
}

// move renames rel to newRel in the registry, together with everything
// below it, keeping the ids.
func (s *Store) move(rel, newRel string) {
	var olds []string
	for p := range s.byPath {
		if p == rel || strings.HasPrefix(p, rel+"/") {
			olds = append(olds, p)
		}
	}
	for _, p := range olds {
		id := s.byPath[p]
		np := newRel + p[len(rel):]
		delete(s.byPath, p)
		s.byPath[np] = id
		s.byID[id] = np
	}
}

// forget removes rel and everything below it from the registry.
func (s *Store) forget(rel string) {
	for p, id := range s.byPath {
		if p == rel || strings.HasPrefix(p, rel+"/") {
			delete(s.byPath, p)
			delete(s.byID, id)
		}
	}
}

// --- Helpers on names and paths ---

func isNoteFile(name string) bool {
	return strings.EqualFold(filepath.Ext(name), noteExt) && !strings.HasPrefix(name, ".")
}

// dirOf is the directory of rel ("" for the root).
func dirOf(rel string) string {
	d := path.Dir(rel)
	if d == "." {
		return ""
	}
	return d
}

// validName trims a title or folder name and checks that it can be a file name.
func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") ||
		strings.ContainsAny(name, "/\\\x00") || len(name) > 200 {
		return "", ErrInvalidName
	}
	return name, nil
}

// nameTaken reports whether the directory already has an entry called name
// (case-insensitive) of the wanted kind, ignoring the entry called except.
func nameTaken(dir, name string, folder bool, except string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() != folder {
			continue
		}
		if except != "" && e.Name() == except {
			continue
		}
		if strings.EqualFold(e.Name(), name) {
			return true
		}
	}
	return false
}

// folderRel returns the path of an active folder (nil = root, "").
func (s *Store) folderRel(id *int64) (string, error) {
	if id == nil {
		return "", nil
	}
	rel, err := s.pathOf(*id)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(s.abs(rel)); err != nil || !st.IsDir() {
		return "", ErrNotFound
	}
	return rel, nil
}

// folderIDOf is the id of the directory that contains rel, nil at the root.
func (s *Store) folderIDOf(rel string) *int64 {
	d := dirOf(rel)
	if d == "" {
		return nil
	}
	id := s.idFor(d)
	return &id
}

// loadNote reads the note stored at rel.
func (s *Store) loadNote(rel string) (*Note, error) {
	data, err := os.ReadFile(s.abs(rel))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(s.abs(rel))
	if err != nil {
		return nil, err
	}
	base := path.Base(rel)
	return &Note{
		ID:        s.idFor(rel),
		Title:     strings.TrimSuffix(base, filepath.Ext(base)),
		Body:      string(data),
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
		FolderID:  s.folderIDOf(rel),
	}, nil
}

// --- Notes ---

// Create adds an empty note at the root.
func (s *Store) Create(title string) (*Note, error) {
	return s.CreateIn(title, nil)
}

// CreateIn adds an empty note in the folder folderID (nil = root).
func (s *Store) CreateIn(title string, folderID *int64) (*Note, error) {
	title, err := validName(title)
	if err != nil {
		return nil, err
	}
	dir, err := s.folderRel(folderID)
	if err != nil {
		return nil, err
	}
	if nameTaken(s.abs(dir), title+noteExt, false, "") {
		return nil, ErrTitleExists
	}
	rel := path.Join(dir, title+noteExt)
	f, err := os.OpenFile(s.abs(rel), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, ErrTitleExists
		}
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return s.loadNote(rel)
}

// collect loads all the notes, together with the folder they live in.
func (s *Store) collect() ([]Note, []string, error) {
	if err := s.refresh(); err != nil {
		return nil, nil, err
	}
	var notes []Note
	var dirs []string
	for rel := range s.byPath {
		if !isNoteFile(path.Base(rel)) {
			continue
		}
		n, err := s.loadNote(rel)
		if err != nil {
			continue // vanished or unreadable file: skip it
		}
		notes = append(notes, *n)
		dirs = append(dirs, dirOf(rel))
	}
	idx := make([]int, len(notes))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool {
		na, nb := notes[idx[a]], notes[idx[b]]
		if !na.UpdatedAt.Equal(nb.UpdatedAt) {
			return na.UpdatedAt.After(nb.UpdatedAt)
		}
		return na.ID > nb.ID
	})
	sorted := make([]Note, len(notes))
	sortedDirs := make([]string, len(notes))
	for i, j := range idx {
		sorted[i], sortedDirs[i] = notes[j], dirs[j]
	}
	return sorted, sortedDirs, nil
}

// List returns all the notes, most recently updated first.
func (s *Store) List() ([]Note, error) {
	notes, _, err := s.collect()
	if notes == nil {
		notes = []Note{}
	}
	return notes, err
}

// ListIn returns the notes of a folder (nil = root). With recursive true it
// includes the subfolders too.
func (s *Store) ListIn(folderID *int64, recursive bool) ([]Note, error) {
	target := ""
	if folderID != nil {
		rel, err := s.pathOf(*folderID)
		if err != nil {
			return nil, err
		}
		target = rel
	}
	notes, dirs, err := s.collect()
	if err != nil {
		return nil, err
	}
	out := []Note{}
	for i, n := range notes {
		d := dirs[i]
		switch {
		case recursive && target == "":
		case recursive && (d == target || strings.HasPrefix(d, target+"/")):
		case !recursive && d == target:
		default:
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

// Get returns the note with the given id.
func (s *Store) Get(id int64) (*Note, error) {
	rel, err := s.pathOf(id)
	if err != nil {
		return nil, err
	}
	if !isNoteFile(path.Base(rel)) {
		return nil, ErrNotFound
	}
	n, err := s.loadNote(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return n, err
}

// Rename changes the title of a note without touching its content or time.
func (s *Store) Rename(id int64, newTitle string) error {
	_, err := s.rename(id, newTitle)
	return err
}

// rename renames the note file and returns its new path.
func (s *Store) rename(id int64, newTitle string) (string, error) {
	title, err := validName(newTitle)
	if err != nil {
		return "", err
	}
	rel, err := s.pathOf(id)
	if err != nil {
		return "", err
	}
	if !isNoteFile(path.Base(rel)) {
		return "", ErrNotFound
	}
	newRel := path.Join(dirOf(rel), title+noteExt)
	if newRel == rel {
		return rel, nil
	}
	if nameTaken(s.abs(dirOf(rel)), title+noteExt, false, path.Base(rel)) {
		return "", ErrTitleExists
	}
	if err := os.Rename(s.abs(rel), s.abs(newRel)); err != nil {
		return "", err
	}
	s.move(rel, newRel)
	return newRel, nil
}

// Save writes title and body of a note; the file's time is the update time.
func (s *Store) Save(id int64, title, body string) error {
	rel, err := s.rename(id, title)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.abs(rel), []byte(body))
}

// writeFileAtomic writes through a temporary file, so that a crash never
// leaves a half-written note.
func writeFileAtomic(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Delete removes a note from the disk for good.
func (s *Store) Delete(id int64) error {
	rel, err := s.pathOf(id)
	if err != nil {
		return err
	}
	if err := os.Remove(s.abs(rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	s.forget(rel)
	return nil
}
