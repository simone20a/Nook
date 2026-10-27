// Package app contains the screens of the Nook application.
package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/store"
)

// appName is the name of the application, used in the window title when
// the active screen is the list.
const appName = "Nook"

// dirtyMark precedes the note title in the window title when
// the active editor has unsaved changes.
const dirtyMark = "● "

// screen identifies the active screen.
type screen int

const (
	screenList screen = iota
	screenEditor
)

// listAction is the action requested by the list screen from the root model.
type listAction int

const (
	listNone listAction = iota
	listOpen
	listQuit
)

// editorAction is the action requested by the editor screen from the root model.
type editorAction int

const (
	edNone editorAction = iota
	edBackToList
)

// Model is the root model of the application.
type Model struct {
	st       *store.Store
	screen   screen
	list     listScreen
	editor   editorScreen
	helpOpen bool
	width    int
	height   int
}

// New creates the root model with the list of notes already loaded.
func New(st *store.Store) Model {
	m := Model{st: st, screen: screenList, width: 80, height: 24}
	m.list = newListScreen(st, 80, 24)
	m.list.load()
	m.editor = editorScreen{st: st}
	return m
}

// Init sets the initial window title and does nothing else.
func (m Model) Init() tea.Cmd { return m.titleCmd() }

// Update handles messages: keys go first to help and to the exits, then
// to the active screen; every other message (the toast ticks) is
// propagated to the active screen, which turns it into the command that concerns it.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	logMsg(msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list = m.list.setSize(m.width, m.height)
		m.editor = m.editor.setSize(m.width, m.height)
		return m, m.titleCmd()
	case tea.KeyMsg:
		model, cmd, quit := m.updateKey(msg)
		if quit {
			return model, tea.Quit
		}
		// The title has to be re-emitted after every key: Dirty may have changed.
		return model, tea.Batch(cmd, model.titleCmd())
	}
	model, cmd := m.updateScreen(msg)
	return model, cmd
}

// updateKey handles a key at application level. The third value says
// whether the application must quit.
func (m Model) updateKey(keyMsg tea.KeyMsg) (Model, tea.Cmd, bool) {
	logKey(keyMsg)

	key := keyMsg.String()

	// Help is a popup over the current screen: ctrl+_ / f1 open and close it,
	// and any other key closes it without reaching the screen.
	if m.helpOpen {
		m.helpOpen = false
		return m, nil, false
	}

	if isHelpKey(key) {
		m.helpOpen = true
		return m, nil, false
	}

	// Only ctrl+c quits immediately: ctrl+q reaches the active screen, where
	// in the editor it opens the save popup and in the list it is handled by the list.
	if key == "ctrl+c" {
		return m, nil, true
	}

	// "?" opens help from the list, but only when not typing in an input
	// field: in modeNormal ? is a command key, in the other modes
	// it is text and must reach the list.
	if m.screen == screenList && m.list.mode == modeNormal && key == "?" {
		m.helpOpen = true
		return m, nil, false
	}

	if m.screen == screenList {
		ls, cmd := m.list.Update(keyMsg)
		m.list = ls
		return m.applyListAction(cmd)
	}

	es, cmd := m.editor.Update(keyMsg)
	m.editor = es
	if m.editor.action == edBackToList {
		m.editor.action = edNone
		m.list.load()
		m.screen = screenList
	}
	return m, cmd, false
}

// applyListAction performs the action that the list requested from the root model:
// quitting or opening the selected note. Both keys and the
// mouse use it, setting the same listAction. The third value says whether
// the application must quit.
func (m Model) applyListAction(cmd tea.Cmd) (Model, tea.Cmd, bool) {
	switch m.list.action {
	case listQuit:
		m.list.action = listNone
		return m, nil, true
	case listOpen:
		id := m.list.actionID
		m.list.action = listNone
		note, err := m.st.Get(id)
		if err != nil {
			// The error becomes a toast of the list, with its tick.
			m.list = m.list.showToast("error: " + err.Error())
			return m, tea.Batch(cmd, listToastTickCmd()), false
		}
		m.editor = newEditorScreen(m.st, note, m.width, m.height)
		m.screen = screenEditor
	}
	return m, cmd, false
}

// updateScreen forwards to the active screen the messages that are not keys:
// the toast tea.Ticks, which the screen turns into the appropriate command, and the
// mouse of the list, which may ask to open a note or to quit.
func (m Model) updateScreen(msg tea.Msg) (Model, tea.Cmd) {
	switch m.screen {
	case screenList:
		ls, cmd := m.list.Update(msg)
		m.list = ls
		model, cmd, quit := m.applyListAction(cmd)
		if quit {
			return model, tea.Quit
		}
		return model, cmd
	case screenEditor:
		es, cmd := m.editor.Update(msg)
		m.editor = es
		return m, cmd
	}
	return m, nil
}

// isHelpKey identifies the keys that open and close help: ctrl+_ (which the
// terminal delivers as "ctrl+/") and f1.
func isHelpKey(key string) bool {
	return key == "ctrl+/" || key == "ctrl+_" || key == "f1"
}

// windowTitle is the window title: with the editor active it is the title of the
// note, preceded by "●" when there are unsaved changes; in the list it is the
// name of the application.
func (m Model) windowTitle() string {
	if m.screen == screenEditor {
		if m.editor.Dirty() {
			return dirtyMark + m.editor.note.Title
		}
		return m.editor.note.Title
	}
	return appName
}

// titleCmd returns the command that sets the window title.
func (m Model) titleCmd() tea.Cmd { return tea.SetWindowTitle(m.windowTitle()) }

// helpText is the short help text: the main system, formatting and
// text shortcuts, the mouse and the list commands.
const helpText = "System: F2 save · F3 format · F1 help · ctrl+c quit immediately\n" +
	"Formatting: F3 opens the palette (arrows+enter) · ctrl+t then b i s k u x l q h 0 · markdown can be typed directly\n" +
	"Text: ctrl+left/right or alt+left/right by word · alt+backspace deletes word · ctrl+u/k to start/end of line\n" +
	"Selection: shift+arrows, shift+home/end, shift+ctrl+left/right · with the mouse drag holding the left button\n" +
	"Mouse: click positions · double/triple click selects word/line · in the list click selects, double click opens\n" +
	"List: n note · c folder · enter opens · → enters · ← goes up · r rename · d trash · u restore · o trash view · x delete · f filter · q quit"

// renderHelp builds the content of the help popup: the text of the
// shortcuts, rendered line by line inside the popup space and then centred by
// centerPopup over the current screen.
func renderHelp(w, h int) string {
	lines := strings.Split(helpText, "\n")
	if limit := maxPopupH(h); len(lines) > limit {
		lines = lines[:limit]
	}
	limitW := maxPopupW(w)
	for i, l := range lines {
		lines[i] = truncatePlain(l, limitW)
	}
	return strings.Join(lines, "\n")
}

// View renders the active screen; with help open the screen stays
// visible and the centred help popup is overlaid on top of it.
func (m Model) View() string {
	var base string
	switch m.screen {
	case screenList:
		base = m.list.View()
	case screenEditor:
		base = m.editor.View()
	}
	if !m.helpOpen {
		return base
	}
	if m.width <= 0 || m.height <= 0 {
		return base
	}
	content := contentOnly(base, m.width, m.height)
	content = centerPopup(content, m.width, m.height, renderHelp(m.width, m.height))
	return withMargins(content, m.width, m.height)
}

// contentOnly brings a total view (margins included) back to the content area:
// contentH(h) rows of contentW(w) columns, ready for centerPopup.
func contentOnly(view string, w, h int) string {
	cw, ch := contentW(w), contentH(h)
	if cw <= 0 || ch <= 0 {
		return ""
	}
	rows := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if marginsActive(w, h) && len(rows) > marginY*2 {
		rows = rows[marginY : len(rows)-marginY]
	}
	for i, row := range rows {
		if marginsActive(w, h) {
			row = ansi.Cut(row, marginX, marginX+cw)
		}
		row = ansi.Truncate(row, cw, "")
		rows[i] = padPlain(row, cw)
	}
	for len(rows) < ch {
		rows = append(rows, strings.Repeat(" ", cw))
	}
	return strings.Join(rows[:ch], "\n")
}
