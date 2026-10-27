package app

import (
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/editor"
	"nook/internal/markdown"
	"nook/internal/store"
	"nook/internal/theme"
)

// edMode is the mode of the editor screen.
type edMode int

const (
	edEditing edMode = iota
	edPrompt
	// edFormatPalette is the formatting palette opened by ctrl+t.
	edFormatPalette
)

// editorScreen is the screen of a note's editor: the text with its
// footer, the Save/Discard/Cancel popup and the overlaid toasts.
type editorScreen struct {
	ed           editor.Editor
	note         store.Note
	st           *store.Store
	mode         edMode
	promptChoice int
	toast        string
	toastUntil   time.Time
	action       editorAction
	width        int
	height       int
	// lastClick is the last mouse click in the editor, for double and triple clicks.
	lastClick edClick
	// paletteChoice is the current entry of the formatting palette.
	paletteChoice int
	// Mouse dragging: anchor of the selection and state of the gesture.
	dragLine int
	dragCol  int
	dragging bool
}

// newEditorScreen creates the editor screen for the given note.
func newEditorScreen(st *store.Store, note *store.Note, w, h int) editorScreen {
	var n store.Note
	if note != nil {
		n = *note
	}
	lines := strings.Split(n.Body, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	return editorScreen{
		ed:     editor.New(lines, contentW(w), edHeight(contentH(h))),
		note:   n,
		st:     st,
		mode:   edEditing,
		width:  w,
		height: h,
	}
}

// edHeight is the height to give the text for a content height ch: the
// content is ch rows high minus the footer one and the empty row above it,
// which the screen draws below the text.
func edHeight(ch int) int {
	if ch-2 > 0 {
		return ch - 2
	}
	return 1
}

// setSize adapts the screen to the terminal size.
func (s editorScreen) setSize(w, h int) editorScreen {
	s.width, s.height = w, h
	s.ed = s.ed.SetSize(contentW(w), edHeight(contentH(h)))
	return s
}

// Dirty reports whether the note has unsaved changes.
func (s editorScreen) Dirty() bool { return s.ed.Dirty }

// edToastDuration is how long an editor toast stays visible.
const edToastDuration = 3 * time.Second

// editorToastClear is the message that turns off the editor toast when
// edToastDuration expires.
type editorToastClear struct{}

// edToastTickCmd returns the command that after edToastDuration delivers
// editorToastClear.
func edToastTickCmd() tea.Cmd {
	return tea.Tick(edToastDuration, func(time.Time) tea.Msg { return editorToastClear{} })
}

// edToastVisible reports whether the toast is still to be shown.
func (s editorScreen) edToastVisible() bool {
	return s.toast != "" && time.Now().Before(s.toastUntil)
}

// edShowToast sets the toast with the standard duration.
func (s editorScreen) edShowToast(msg string) editorScreen {
	s.toast = msg
	s.toastUntil = time.Now().Add(edToastDuration)
	return s
}

// save writes the note's current body to the store. The third value says whether
// saving succeeded; on error the message becomes a toast.
func (s editorScreen) save() (editorScreen, tea.Cmd, bool) {
	body := strings.Join(s.ed.Lines(), "\n")
	if err := s.st.Save(s.note.ID, s.note.Title, body); err != nil {
		s = s.edShowToast("error: " + err.Error())
		return s, edToastTickCmd(), false
	}
	s.ed.Dirty = false
	return s, nil, true
}

// Update handles the messages of the editor screen.
func (s editorScreen) Update(msg tea.Msg) (editorScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case editorToastClear:
		// The toast turns off only when its 3 seconds have really passed;
		// otherwise another tick is needed.
		if s.edToastVisible() {
			return s, edToastTickCmd()
		}
		s.toast = ""
		s.toastUntil = time.Time{}
		return s, nil
	case tea.KeyMsg:
		return s.edUpdateKey(msg)
	case tea.MouseMsg:
		return s.edUpdateMouse(msg)
	}
	return s, nil
}

// edUpdateKey handles a key according to the shared scheme.
func (s editorScreen) edUpdateKey(keyMsg tea.KeyMsg) (editorScreen, tea.Cmd) {
	key := keyMsg.String()

	if s.mode == edFormatPalette {
		return s.edUpdatePalette(key)
	}

	if s.mode == edPrompt {
		switch key {
		case "left":
			s.promptChoice = (s.promptChoice + len(edPromptChoices) - 1) % len(edPromptChoices)
			return s, nil
		case "right":
			s.promptChoice = (s.promptChoice + 1) % len(edPromptChoices)
			return s, nil
		case "enter":
			return s.edConfirmPrompt()
		case "esc":
			s.mode = edEditing
			return s, nil
		case "ctrl+c":
			s.mode = edEditing
			s.action = edBackToList
			return s, nil
		}
		return s, nil
	}

	m, cmd := s.ed.Update(keyMsg)
	s.ed = m.(editor.Editor)

	if s.ed.WantsQuit {
		s.ed.WantsQuit = false
		s.action = edBackToList
		return s, cmd
	}

	if s.ed.WantsPalette {
		// ctrl+t: the formatting palette opens on the first entry.
		s.ed.WantsPalette = false
		s.mode = edFormatPalette
		s.paletteChoice = 0
		return s, cmd
	}

	if s.ed.WantsSave {
		s.ed.WantsSave = false
		saved, saveCmd, ok := s.save()
		if !ok {
			// Error: save() has already set the toast; only its tick is needed.
			return saved, saveCmd
		}
		// Save succeeded: "saved" toast with auto-hide, with the same
		// mechanism (edShowToast + edToastTickCmd) used by the error.
		return saved.edShowToast("saved"), edToastTickCmd()
	}

	if s.ed.WantsExit {
		// ctrl+q (and esc) on a modified note: the Save/Discard/Cancel
		// popup opens, without going back to the list; without changes it
		// goes back to the list. The popup state is only mode/promptChoice.
		s.ed.WantsExit = false
		if s.ed.Dirty {
			s.mode = edPrompt
			s.promptChoice = 0
			return s, cmd
		}
		s.action = edBackToList
	}
	return s, cmd
}

// edMultiClickThreshold is the maximum interval between two clicks on the same cell
// for them to count as a double (or triple) click.
const edMultiClickThreshold = 400 * time.Millisecond

// edClick is the last click recorded by the editor: the terminal cell (x, y),
// how many consecutive clicks have been recognised on that cell and
// the instant. The zero value means "no previous click".
type edClick struct {
	x     int
	y     int
	count int
	at    time.Time
}

// edClickCount recognises single, double and triple clicks: it is a pure helper with
// explicit time (the instant now comes from outside) so it can be tested without a
// clock. If the previous click exists, if it hit the same cell and if
// now is no more than edMultiClickThreshold from prev.at the count grows
// up to three clicks; in every other case it restarts from one, so a fourth click
// counts as a new single click.
func edClickCount(prev edClick, now time.Time, x, y int) int {
	if prev.at.IsZero() || prev.x != x || prev.y != y {
		return 1
	}
	delta := now.Sub(prev.at)
	if delta < 0 || delta > edMultiClickThreshold {
		return 1
	}
	if prev.count >= 3 {
		return 1
	}
	return prev.count + 1
}

// edHitLine maps the screen cell (x, y) to the raw document line
// drawn there: it removes the margins, discards cells outside the content
// area (including the footer row, which sits below the text) and converts the
// view row into the document line. It also returns the column x
// inside the content area.
func (s editorScreen) edHitLine(x, y int) (line, cell int, ok bool) {
	cw, chh := contentW(s.width), contentH(s.height)
	if marginsActive(s.width, s.height) {
		x, y = x-marginX, y-marginY
	}
	if x < 0 || y < 0 || x >= cw || y >= chh {
		return 0, 0, false
	}
	idx, ok := s.ed.LineAtRow(y)
	if !ok {
		return 0, 0, false
	}
	return idx, x, true
}

// edHitCol converts the column cell, in cells of line line, into the column in
// runes of the raw line, with markdown.MapCellToRaw on the segments the
// view draws the line with: the resulting byte offset is counted in runes,
// the unit of the editor's columns.
func (s editorScreen) edHitCol(line, cell int) int {
	raw := s.ed.LineAt(line)
	off := markdown.MapCellToRaw(s.ed.LineSegments(line), cell)
	if off < 0 {
		off = 0
	}
	if off > len(raw) {
		off = len(raw)
	}
	return utf8.RuneCountInString(raw[:off])
}

// edWheelStep is how many lines the document moves for each wheel event.
const edWheelStep = 1

// edUpdateMouse handles the editor's mouse: only in edEditing and only for the
// press of the left button. A click maps the cell to (raw line,
// raw column) and brings the cursor there, so that line becomes the Focus
// Line; the double click (same cell within edMultiClickThreshold) selects the
// word under the cursor and the triple click the whole line. From the second click
// on, the cursor is no longer brought back to the cell: the first click has
// already placed it there, while the view may have scrolled.
func (s editorScreen) edUpdateMouse(m tea.MouseMsg) (editorScreen, tea.Cmd) {
	if s.mode != edEditing {
		return s, nil
	}

	// Wheel and two-finger touchpad scroll: the document moves under the
	// Focus Line, which stays where it is.
	switch m.Button {
	case tea.MouseButtonWheelUp:
		s.ed = s.ed.Scroll(-edWheelStep)
		s.dragging = false
		return s, nil
	case tea.MouseButtonWheelDown:
		s.ed = s.ed.Scroll(edWheelStep)
		s.dragging = false
		return s, nil
	}

	// Dragging: between the press and the release of the left button the
	// mouse movements extend the selection from the anchor set at the click
	// to the cell reached.
	switch m.Action {
	case tea.MouseActionRelease:
		s.dragging = false
		return s, nil
	case tea.MouseActionMotion:
		if !s.dragging {
			return s, nil
		}
		line, cell, ok := s.edHitLine(m.X, m.Y)
		if !ok {
			return s, nil
		}
		s.ed = s.ed.SetSelection(s.dragLine, s.dragCol, line, s.edHitCol(line, cell))
		return s, nil
	}

	if m.Action != tea.MouseActionPress || m.Button != tea.MouseButtonLeft {
		return s, nil
	}
	now := time.Now()
	clicks := edClickCount(s.lastClick, now, m.X, m.Y)
	s.lastClick = edClick{x: m.X, y: m.Y, count: clicks, at: now}

	switch clicks {
	case 1:
		line, cell, ok := s.edHitLine(m.X, m.Y)
		if !ok {
			return s, nil
		}
		col := s.edHitCol(line, cell)
		s.ed = s.ed.SetCursor(line, col)
		// The click sets the anchor of a possible drag selection:
		// mouse movements with the button pressed extend it from here.
		s.dragLine, s.dragCol = line, col
		s.dragging = true
	case 2:
		s.ed = s.ed.SelectWord()
		s.dragging = false
	case 3:
		s.ed = s.ed.SelectLine()
		s.dragging = false
	}
	return s, nil
}

// --- Formatting palette (ctrl+t) ---

// edPaletteAction is an entry of the formatting palette: the letter that
// Editor.ApplyFormat interprets and the short label shown in the popup.
type edPaletteAction struct {
	key   string
	label string
}

// formatPaletteActions are the 10 actions of the formatting palette,
// in the order shown: the letter is the one of applyFormatKey.
var formatPaletteActions = []edPaletteAction{
	{"b", "bold"},
	{"i", "italic"},
	{"h", "heading"},
	{"k", "code"},
	{"x", "checkbox"},
	{"l", "link"},
	{"q", "quote"},
	{"s", "strike"},
	{"u", "list"},
	{"0", "remove heading"},
}

// edUpdatePalette handles the keys of the formatting palette: up and down
// change the entry circularly, enter applies the formatting of the current
// entry and closes, the letter shown next to an entry applies it straight
// away, esc (or ctrl+t again) closes without applying. As long as the palette
// is open every other key is suspended, as in the exit popup.
func (s editorScreen) edUpdatePalette(key string) (editorScreen, tea.Cmd) {
	switch key {
	case "up":
		s.paletteChoice = (s.paletteChoice + len(formatPaletteActions) - 1) % len(formatPaletteActions)
	case "down":
		s.paletteChoice = (s.paletteChoice + 1) % len(formatPaletteActions)
	case "enter":
		if s.paletteChoice < 0 || s.paletteChoice >= len(formatPaletteActions) {
			s.paletteChoice = 0
		}
		s.ed = s.ed.ApplyFormat(formatPaletteActions[s.paletteChoice].key)
		s.mode = edEditing
	case "esc", "ctrl+t":
		s.mode = edEditing
	default:
		for i, a := range formatPaletteActions {
			if a.key == key {
				s.paletteChoice = i
				s.ed = s.ed.ApplyFormat(a.key)
				s.mode = edEditing
				break
			}
		}
	}
	return s, nil
}

// formatPaletteBox builds the content of the palette popup: the title row
// and one row per action, with the current entry between square brackets.
func formatPaletteBox(s editorScreen) string {
	lines := make([]string, 0, len(formatPaletteActions)+1)
	lines = append(lines, edPromptStyle.Render("Formatting"))
	for i, a := range formatPaletteActions {
		label := a.label + " (" + a.key + ")"
		if i == s.paletteChoice {
			lines = append(lines, theme.Selected.Render("["+label+"]"))
		} else {
			lines = append(lines, edPromptStyle.Render(" "+label+" "))
		}
	}
	lines = append(lines, edPromptStyle.Render("letter, or ↑/↓ enter"))
	return strings.Join(lines, "\n")
}

// edPromptChoices are the choices of the exit popup, in the order shown.
var edPromptChoices = []string{"Save", "Discard", "Cancel"}

// Indices of the popup choices.
const (
	edPromptSave = iota
	edPromptDiscard
	edPromptCancel
)

// edConfirmPrompt applies the current choice of the popup.
func (s editorScreen) edConfirmPrompt() (editorScreen, tea.Cmd) {
	switch s.promptChoice {
	case edPromptSave:
		saved, cmd, ok := s.save()
		if ok {
			saved.action = edBackToList
		}
		return saved, cmd
	case edPromptDiscard:
		s.action = edBackToList
		return s, nil
	default:
		s.mode = edEditing
		return s, nil
	}
}

// edPromptStyle is the style of the popup text.
var edPromptStyle = theme.DimText

// promptBox builds the content of the Save/Discard/Cancel popup: the title
// row, an empty row and the row of choices with the current one between
// square brackets.
func promptBox(s editorScreen) string {
	nameW := 0
	for _, c := range edPromptChoices {
		if len(c) > nameW {
			nameW = len(c)
		}
	}
	parts := make([]string, len(edPromptChoices))
	for i, c := range edPromptChoices {
		label := c + strings.Repeat(" ", nameW-len(c))
		if i == s.promptChoice {
			parts[i] = theme.Selected.Render("[" + label + "]")
		} else {
			parts[i] = edPromptStyle.Render(" " + label + " ")
		}
	}
	line := truncatePlain(strings.Join(parts, " "), maxPopupW(s.width))
	return edPromptStyle.Render("Unsaved changes") + "\n\n" + line
}

// View renders the editor screen: the text, the footer row, with the prompt popup
// and the toast overlaid.
func (s editorScreen) View() string {
	// The text occupies the content area minus the footer row.
	chh := contentH(s.height)
	var lines []string
	if body := s.ed.View(); body != "" {
		lines = append(strings.Split(body, "\n"), "", s.ed.Footer())
	}
	if chh > 0 && len(lines) > chh {
		lines = lines[:chh]
	}
	base := strings.Join(lines, "\n")

	if s.mode == edPrompt {
		base = centerPopup(base, s.width, s.height, promptBox(s))
	}
	if s.mode == edFormatPalette {
		base = centerPopup(base, s.width, s.height, formatPaletteBox(s))
	}
	if s.edToastVisible() {
		base = overlayAt(base, s.width, s.height,
			contentH(s.height)-1, 0, toastLine(s.toast, s.width))
	}
	return withMargins(base, s.width, s.height)
}
