package store

import (
	"errors"
	"io/fs"
	"os"
	"path"
)

// NoteAt returns the note called title in the folder dir of the root (a slash
// path such as "Daily"), or ErrNotFound if there is none.
func (s *Store) NoteAt(dir, title string) (*Note, error) {
	rel := path.Join(dir, title+noteExt)
	if _, err := os.Stat(s.abs(rel)); errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return s.loadNote(rel)
}

// CreateAt writes a new note called title, with the given body, in the folder
// dir of the root, creating the folder if it is missing. It never overwrites:
// ErrTitleExists is returned if the note is already there.
func (s *Store) CreateAt(dir, title, body string) (*Note, error) {
	title, err := validName(title)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.abs(dir), 0o755); err != nil {
		return nil, err
	}
	rel := path.Join(dir, title+noteExt)
	f, err := os.OpenFile(s.abs(rel), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, ErrTitleExists
		}
		return nil, err
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return s.loadNote(rel)
}
