package editor

import tea "github.com/charmbracelet/bubbletea"

// maxUndo is how many changes the history keeps.
const maxUndo = 200

// snapshot is the state restored by undo and redo: the text and the cursor.
type snapshot struct {
	lines     []string
	line, col int
}

// snapshot copies the current state.
func (e Editor) snapshot() snapshot {
	return snapshot{lines: append([]string(nil), e.lines...), line: e.cursorLine, col: e.cursorCol}
}

// restore puts a snapshot back as the current state.
func (e *Editor) restore(s snapshot) {
	e.lines = append([]string(nil), s.lines...)
	e.cursorLine, e.cursorCol = s.line, s.col
	e.clearSelection()
	e.frozen = false
	e.lastEdit = ""
	e.Dirty = true
	e.clampCol()
	e.ensureCursorVisible()
}

// sameLines reports whether two texts are identical.
func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// editKind groups the keys whose effects are undone together: a run of typed
// characters, or a run of deletions, is one step of the history.
func editKind(key tea.KeyMsg) string {
	switch key.Type {
	case tea.KeyRunes, tea.KeySpace:
		if !key.Alt {
			return "type"
		}
	case tea.KeyBackspace, tea.KeyDelete:
		return "delete"
	}
	return ""
}

// commit records in the history the change made since before, if the text
// is different. A change of the same kind on the same line as the previous one
// (typing, deleting) is merged into it; any other key ends the run.
func (e Editor) commit(before snapshot, kind string) Editor {
	if sameLines(before.lines, e.lines) {
		e.lastEdit = ""
		return e
	}
	if kind == "" || kind != e.lastEdit || before.line != e.lastEditLine || len(e.undo) == 0 {
		e.undo = append(append([]snapshot(nil), e.undo...), before)
		if len(e.undo) > maxUndo {
			e.undo = e.undo[len(e.undo)-maxUndo:]
		}
	}
	e.redo = nil
	e.lastEdit = kind
	e.lastEditLine = e.cursorLine
	return e
}

// Undo takes back the last change, restoring text and cursor.
func (e Editor) Undo() Editor {
	e.frozen = false
	if len(e.undo) == 0 {
		return e
	}
	last := e.undo[len(e.undo)-1]
	e.redo = append(append([]snapshot(nil), e.redo...), e.snapshot())
	e.undo = e.undo[:len(e.undo)-1]
	e.restore(last)
	return e
}

// Redo repeats the change taken back by Undo.
func (e Editor) Redo() Editor {
	e.frozen = false
	if len(e.redo) == 0 {
		return e
	}
	next := e.redo[len(e.redo)-1]
	e.undo = append(append([]snapshot(nil), e.undo...), e.snapshot())
	e.redo = e.redo[:len(e.redo)-1]
	e.restore(next)
	return e
}
