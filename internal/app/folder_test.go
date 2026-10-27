package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/store"
)

// newFolderTestStore opens the store in a temporary directory of the test.
func newFolderTestStore(t *testing.T) *store.Store {
	return newKeysTestStore(t)
}

// TestFolderNavigation checks that folders appear in the list, that the
// right arrow (and enter) enter them and that the left arrow (and esc)
// go up to the upper level.
func TestFolderNavigation(t *testing.T) {
	st := newFolderTestStore(t)
	folder, err := st.CreateFolder("projects", nil)
	if err != nil {
		t.Fatalf("creating folder: %v", err)
	}
	if _, err := st.CreateIn("inside", &folder.ID); err != nil {
		t.Fatalf("creating note in the folder: %v", err)
	}
	if _, err := st.Create("outside"); err != nil {
		t.Fatalf("creating note at the root: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	// At the root: one folder and one note.
	if got := len(s.folders); got != 1 {
		t.Fatalf("folders at the root = %d, want 1", got)
	}
	if got := len(s.notes); got != 1 || s.notes[0].Title != "outside" {
		t.Fatalf("notes at the root = %+v, want only \"outside\"", s.notes)
	}

	// The first entry is the folder: the right arrow enters it.
	s.selected = 0
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRight})
	if s.folderID == nil || *s.folderID != folder.ID {
		t.Fatalf("folderID after → = %v, want %d", s.folderID, folder.ID)
	}
	if len(s.path) != 1 || s.path[0].Name != "projects" {
		t.Fatalf("path after → = %+v, want [projects]", s.path)
	}
	if len(s.folders) != 0 {
		t.Errorf("folders inside the folder = %d, want 0", len(s.folders))
	}
	if len(s.notes) != 1 || s.notes[0].Title != "inside" {
		t.Fatalf("notes inside the folder = %+v, want only \"inside\"", s.notes)
	}

	// Left arrow: back to the root.
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyLeft})
	if s.folderID != nil || len(s.path) != 0 {
		t.Fatalf("after ← folderID = %v, path = %+v; want root", s.folderID, s.path)
	}
	if len(s.folders) != 1 || len(s.notes) != 1 {
		t.Errorf("at the root: %d folders and %d notes, want 1 and 1", len(s.folders), len(s.notes))
	}

	// Enter on the folder enters it, esc leaves it.
	s.selected = 0
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if s.folderID == nil {
		t.Fatal("enter on the folder did not enter it")
	}
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	if s.folderID != nil {
		t.Fatal("esc did not go back up to the root")
	}
}

// TestEnterOnNoteOpensEditor checks that enter on a note requests opening
// of the editor, while the right arrow on a note does nothing.
func TestEnterOnNoteOpensEditor(t *testing.T) {
	st := newFolderTestStore(t)
	note, err := st.Create("note")
	if err != nil {
		t.Fatalf("creating note: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRight})
	if s.action == listOpen {
		t.Error("→ on a note opened the editor, want no action")
	}

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if s.action != listOpen || s.actionID != note.ID {
		t.Errorf("enter on the note: action = %v, id = %d; want listOpen on %d", s.action, s.actionID, note.ID)
	}
}

// TestCreateFolderKey checks that "c" opens the folder popup and that enter
// creates it at the current level, without opening the editor.
func TestCreateFolderKey(t *testing.T) {
	st := newFolderTestStore(t)

	m := Model{st: st, width: 80, height: 24, screen: screenList}
	m.list = newListScreen(st, 80, 24)
	m.list.load()

	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if m.list.mode != modeInputNewFolder {
		t.Fatalf("mode after \"c\" = %v, want modeInputNewFolder", m.list.mode)
	}
	m, _, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ideas")})
	m, _, quit := m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if quit {
		t.Fatal("quit = true after enter, want false")
	}
	if m.screen != screenList {
		t.Errorf("screen = %v, want screenList", m.screen)
	}
	if len(m.list.folders) != 1 || m.list.folders[0].Name != "ideas" {
		t.Fatalf("folders = %+v, want the folder \"ideas\"", m.list.folders)
	}
	if e, ok := m.list.currentEntry(); !ok || e.kind != entryFolder || e.title != "ideas/" {
		t.Errorf("selected entry = %+v, want the folder \"ideas/\"", e)
	}
}

// TestNewNoteGoesIntoCurrentFolder checks that a note created inside a
// folder stays there.
func TestNewNoteGoesIntoCurrentFolder(t *testing.T) {
	st := newFolderTestStore(t)
	folder, err := st.CreateFolder("projects", nil)
	if err != nil {
		t.Fatalf("creating folder: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()
	s.selected = 0
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRight})
	if s.folderID == nil {
		t.Fatal("did not enter the folder")
	}

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("inner")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})

	if len(s.notes) != 1 || s.notes[0].Title != "inner" {
		t.Fatalf("notes in the folder = %+v, want only \"inner\"", s.notes)
	}
	got, err := st.Get(s.notes[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FolderID == nil || *got.FolderID != folder.ID {
		t.Errorf("FolderID = %v, want %d", got.FolderID, folder.ID)
	}
}

// TestDeleteFolderTrashesContent checks that trashing a folder sends its contents
// to the trash and that the trash view shows it as a note.
func TestDeleteFolderTrashesContent(t *testing.T) {
	st := newFolderTestStore(t)
	folder, err := st.CreateFolder("projects", nil)
	if err != nil {
		t.Fatalf("creating folder: %v", err)
	}
	note, err := st.CreateIn("inside", &folder.ID)
	if err != nil {
		t.Fatalf("creating note: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()
	s.selected = 0

	// "d" asks for confirmation, "y" confirms.
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if s.mode != modeConfirmSoftDelete {
		t.Fatalf("mode after \"d\" = %v, want modeConfirmSoftDelete", s.mode)
	}
	box := s.boxConfirm()
	if !strings.Contains(box, "folder") {
		t.Errorf("the confirmation does not mention a folder: %q", box)
	}
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})

	if len(s.folders) != 0 || len(s.notes) != 0 {
		t.Fatalf("after deletion: %d folders, %d notes; want 0 and 0", len(s.folders), len(s.notes))
	}

	// The note that was inside appears in the trash.
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if !s.showTrash {
		t.Fatal("did not switch to the trash")
	}
	if len(s.notes) != 1 || s.notes[0].ID != note.ID {
		t.Fatalf("notes in the trash = %+v, want note %d", s.notes, note.ID)
	}
}

// TestListTitleBreadcrumb checks the list title: NOOK at the root and the
// path when descending into a folder.
func TestListTitleBreadcrumb(t *testing.T) {
	st := newFolderTestStore(t)
	root, err := st.CreateFolder("projects", nil)
	if err != nil {
		t.Fatalf("creating folder: %v", err)
	}
	child, err := st.CreateFolder("2026", &root.ID)
	if err != nil {
		t.Fatalf("creating subfolder: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	if got := strings.TrimSpace(s.listTitle(60)); got != "NOOK" {
		t.Errorf("title at the root = %q, want \"NOOK\"", got)
	}

	s = s.enterFolder(root.ID)
	if got := strings.TrimSpace(s.listTitle(60)); got != "NOOK › projects" {
		t.Errorf("title inside the folder = %q, want \"NOOK › projects\"", got)
	}

	s = s.enterFolder(child.ID)
	if got := strings.TrimSpace(s.listTitle(60)); got != "NOOK › projects › 2026" {
		t.Errorf("title in the subfolder = %q, want the full path", got)
	}

	s.showTrash = true
	if got := strings.TrimSpace(s.listTitle(60)); got != "TRASH" {
		t.Errorf("title in the trash = %q, want \"TRASH\"", got)
	}
}

// TestFolderEntryDetail checks the detail row of folders.
func TestFolderEntryDetail(t *testing.T) {
	st := newFolderTestStore(t)
	empty, err := st.CreateFolder("blank", nil)
	if err != nil {
		t.Fatalf("creating empty folder: %v", err)
	}
	one, err := st.CreateFolder("single", nil)
	if err != nil {
		t.Fatalf("creating folder: %v", err)
	}
	if _, err := st.CreateIn("note", &one.ID); err != nil {
		t.Fatalf("creating note: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	details := map[int64]string{}
	for _, e := range s.entries() {
		if e.kind == entryFolder {
			details[e.id] = e.detail
		}
	}
	if got := details[empty.ID]; got != "empty" {
		t.Errorf("detail of empty folder = %q, want \"empty\"", got)
	}
	if got := details[one.ID]; got != "1 note" {
		t.Errorf("detail of folder with one note = %q, want \"1 note\"", got)
	}
}

// TestNewNoteSelectsItAmongFolders checks that, creating a note in a list
// that contains folders, the selection ends up on the new note and not on a
// folder.
func TestNewNoteSelectsItAmongFolders(t *testing.T) {
	st := newFolderTestStore(t)
	if _, err := st.CreateFolder("folder", nil); err != nil {
		t.Fatalf("creating folder: %v", err)
	}
	if _, err := st.Create("existing"); err != nil {
		t.Fatalf("creating note: %v", err)
	}

	s := newListScreen(st, 80, 24)
	s.load()

	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("fresh")})
	s, _ = s.updateKey(tea.KeyMsg{Type: tea.KeyEnter})

	e, ok := s.currentEntry()
	if !ok || e.kind != entryNote || e.title != "fresh" {
		t.Fatalf("selected entry = %+v, want the note \"fresh\"", e)
	}
}
