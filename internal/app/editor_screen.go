package app

import (
	"nook/internal/markdown"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/editor"
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
	draft        bool // a daily note not yet written to the disk (see openDaily)
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
	dragLine  int
	dragCol   int
	dragging  bool
	dragMoved bool // the mouse moved while the button was held: it is a selection, not a click
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
	ed := editor.New(lines, contentW(w), edHeight(contentH(h)))
	if vimEnabled() {
		ed = ed.WithVim()
	}
	if readingDefault {
		ed = ed.WithReading(true)
	}
	return editorScreen{
		ed:     ed,
		note:   n,
		st:     st,
		mode:   edEditing,
		width:  w,
		height: h,
	}
}

// vimEnv is the environment variable that turns vim mode in the editor on or
// off: "1" activates it, any other non-empty value leaves the usual keys.
// When it is empty or unset the configuration decides (see SetVim).
const vimEnv = "NOOK_VIM"

// defaultVim is the vim mode setting of the configuration file.
var defaultVim bool

// SetVim sets whether notes open in vim mode when NOOK_VIM does not say.
func SetVim(on bool) { defaultVim = on }

// readingDefault is whether notes open in reading mode (see SetReading).
var readingDefault bool

// SetReading sets whether notes open in reading mode, read only, instead of
// ready to edit. ctrl+p switches mode in the editor either way.
func SetReading(on bool) { readingDefault = on }

// vimEnabled reports whether the editor starts in vim mode.
func vimEnabled() bool {
	if v := os.Getenv(vimEnv); v != "" {
		return v == "1"
	}
	return defaultVim
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
	if s.draft {
		n, err := s.saveDraft(body)
		if err != nil {
			s = s.edShowToast("error: " + err.Error())
			return s, edToastTickCmd(), false
		}
		s.note, s.draft = *n, false
		s.ed.Dirty = false
		return s, nil, true
	}
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

	if s.ed.ReadChanged {
		s.ed.ReadChanged = false
		if s.ed.Reading() {
			return s.edShowToast("reading mode"), edToastTickCmd()
		}
		return s.edShowToast("editing"), edToastTickCmd()
	}

	if s.ed.WantsSave {
		s.ed.WantsSave = false
		// :wq in vim mode asks to save and then exit.
		exit := s.ed.WantsExit
		s.ed.WantsExit = false
		saved, saveCmd, ok := s.save()
		if !ok {
			// Error: save() has already set the toast; only its tick is needed.
			return saved, saveCmd
		}
		if exit {
			saved.action = edBackToList
			return saved, cmd
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

// edHit maps the screen cell (x, y) to the raw document position drawn
// there: it removes the margins, discards cells outside the content area
// (including the footer row, which sits below the text) and lets the editor
// convert the view cell into (line, column in runes).
func (s editorScreen) edHit(x, y int) (line, col int, ok bool) {
	cw, chh := contentW(s.width), contentH(s.height)
	if marginsActive(s.width, s.height) {
		x, y = x-marginX, y-marginY
	}
	if x < 0 || y < 0 || x >= cw || y >= chh {
		return 0, 0, false
	}
	return s.ed.HitCell(y, x)
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

	// Dragging: the press only records where the gesture starts; nothing moves
	// while the button is held. Moving the mouse extends the selection from the
	// press point to the cell reached, with the view still. On release a plain
	// click (no movement) moves the Focus Line there, while after a drag the
	// selection stays and the Focus Line does not move.
	switch m.Action {
	case tea.MouseActionRelease:
		if s.dragging && !s.dragMoved && !s.ed.Reading() {
			s.ed = s.ed.SetCursor(s.dragLine, s.dragCol)
		}
		s.dragging, s.dragMoved = false, false
		return s, nil
	case tea.MouseActionMotion:
		if !s.dragging {
			return s, nil
		}
		line, col, ok := s.edHit(m.X, m.Y)
		if !ok {
			return s, nil
		}
		s.ed = s.ed.DragSelection(s.dragLine, s.dragCol, line, col)
		s.dragMoved = true
		return s, nil
	}

	if m.Action != tea.MouseActionPress || m.Button != tea.MouseButtonLeft {
		return s, nil
	}
	// A click on a rendered link opens it: in reading mode, where a plain
	// click does nothing else, and with ctrl held while editing.
	if s.ed.Reading() || m.Ctrl {
		if line, col, ok := s.edHit(m.X, m.Y); ok {
			raw := []rune(s.ed.LineAt(line))
			col = min(col, len(raw))
			if url, ok := markdown.LinkAt(string(raw), len(string(raw[:col]))); ok {
				if err := openURL(url); err != nil {
					s = s.edShowToast("error: " + err.Error())
				} else {
					s = s.edShowToast("opening " + url)
				}
				s.dragging = false
				return s, edToastTickCmd()
			}
		}
	}
	now := time.Now()
	clicks := edClickCount(s.lastClick, now, m.X, m.Y)
	s.lastClick = edClick{x: m.X, y: m.Y, count: clicks, at: now}

	switch clicks {
	case 1:
		line, col, ok := s.edHit(m.X, m.Y)
		if !ok {
			return s, nil
		}
		// The press sets the anchor of a possible drag selection, but does
		// not move anything: the release decides (see above).
		s.dragLine, s.dragCol = line, col
		s.dragging, s.dragMoved = true, false
	case 2:
		if !s.ed.Reading() {
			s.ed = s.ed.SelectWord()
		}
		s.dragging = false
	case 3:
		if !s.ed.Reading() {
			s.ed = s.ed.SelectLine()
		}
		s.dragging = false
	}
	return s, nil
}

// --- Formatting palette (ctrl+t) ---

// edPaletteAction is an entry of the formatting palette: the letter that
// Editor.ApplyFormat interprets, the label shown in the popup, a short
// example of the result and the group it is listed under.
type edPaletteAction struct {
	key     string
	label   string
	example string
	group   string
}

// formatPaletteActions are the 10 actions of the formatting palette, in the
// order shown (grouped): the letter is the one of ApplyFormat.
var formatPaletteActions = []edPaletteAction{
	{"b", "Bold", "**text**", "Text"},
	{"i", "Italic", "_text_", "Text"},
	{"s", "Strikethrough", "~~text~~", "Text"},
	{"k", "Code", "`text`", "Text"},
	{"h", "Heading", "# → ## → ###", "Line"},
	{"q", "Quote", "> text", "Line"},
	{"u", "List", "- item", "Line"},
	{"x", "Checkbox", "- [ ] task", "Line"},
	{"0", "Remove heading", "plain line", "Line"},
	{"l", "Link", "[text](url)", "Insert"},
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

// Widths of the columns of the palette popup.
const (
	paletteLabelW   = 15
	paletteExampleW = 14
)

// formatPaletteBox builds the content of the palette popup: a title with the
// way to use it, then the actions grouped under a heading, each on one row
// with its letter, its name and an example of the result. The current entry
// is highlighted.
func formatPaletteBox(s editorScreen) string {
	width := 2 + 1 + 2 + paletteLabelW + paletteExampleW
	lines := []string{
		theme.BannerAcc.Render("Formatting"),
		theme.DimText.Render("press a letter, or ↑/↓ and enter"),
	}
	group := ""
	for i, a := range formatPaletteActions {
		if a.group != group {
			group = a.group
			lines = append(lines, theme.DimText.Render(strings.ToUpper(group)))
		}
		if i == s.paletteChoice {
			plain := "› " + a.key + "  " + padPlain(a.label, paletteLabelW) + padPlain(a.example, paletteExampleW)
			lines = append(lines, theme.Rev.Render(padPlain(plain, width)))
			continue
		}
		lines = append(lines, "  "+theme.BannerAcc.Render(a.key)+"  "+
			padPlain(a.label, paletteLabelW)+theme.DimText.Render(padPlain(a.example, paletteExampleW)))
	}
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
			parts[i] = theme.DimText.Render(" " + label + " ")
		}
	}
	line := truncatePlain(strings.Join(parts, " "), maxPopupW(s.width))
	return theme.DimText.Render("Unsaved changes") + "\n\n" + line
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
