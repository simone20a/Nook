package app

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/store"
)

const (
	// dailyFolder is the folder, at the root of the notes, that holds the
	// daily notes.
	dailyFolder = "Daily"
	// dailyLayout is the layout of the daily notes' names: 2026-11-10.
	dailyLayout = "2006-01-02"
)

// dailyNow gives the current time; tests replace it.
var dailyNow = time.Now

// openDaily opens today's daily note. When it already exists it is opened like
// any note; otherwise the editor opens on a draft that is not on the disk:
// the file, and the Daily folder if needed, are created by the first save.
func (m Model) openDaily(cmd tea.Cmd) (Model, tea.Cmd, bool) {
	title := dailyNow().Format(dailyLayout)
	note, err := m.st.NoteAt(dailyFolder, title)
	switch {
	case err == nil:
		m.editor = newEditorScreen(m.st, note, m.width, m.height)
	case errors.Is(err, store.ErrNotFound):
		m.editor = newEditorScreen(m.st, &store.Note{Title: title, Body: "# " + title + "\n\n"}, m.width, m.height)
		m.editor.draft = true
		m.editor.ed = m.editor.ed.SetCursor(2, 0)
	default:
		m.list = m.list.showToast("error: " + err.Error())
		return m, tea.Batch(cmd, listToastTickCmd()), false
	}
	m.screen = screenEditor
	return m, cmd, false
}

// saveDraft creates the file of a draft (see openDaily) with the body.
func (s editorScreen) saveDraft(body string) (*store.Note, error) {
	n, err := s.st.CreateAt(dailyFolder, s.note.Title, body)
	if errors.Is(err, store.ErrTitleExists) {
		return nil, errors.New("today's note already exists: not overwritten")
	}
	return n, err
}
