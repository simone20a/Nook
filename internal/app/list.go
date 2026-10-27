package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"nook/internal/store"
	"nook/internal/theme"
)

// List screen modes.
const (
	modeNormal = iota
	modeInputNew
	modeInputRename
	modeInputFilter
	modeInputNewFolder
	modeConfirmSoftDelete
	modeConfirmPurge
)

// entryKind distinguishes the two kinds of list entries.
type entryKind int

const (
	entryFolder entryKind = iota
	entryNote
)

// listEntry is an entry of the list: a folder or a note. The title is already
// ready for the card (folders have the "/" suffix), the detail is the
// row below (number of notes for folders, date and words for notes).
type listEntry struct {
	kind   entryKind
	id     int64
	title  string
	detail string
}

// listScreen is the screen of the notes list: title row, plane of
// rows centred on the selection, footer, popups for input, filter and confirmations,
// overlaid toasts.
type listScreen struct {
	st        *store.Store
	notes     []store.Note
	folders   []store.Folder
	folderID  *int64
	path      []store.Folder
	showTrash bool
	// folderNotes is the number of notes contained in each folder shown.
	folderNotes map[int64]int
	selected    int
	mode        int
	input       textinput.Model
	filter      string
	toast       string
	toastUntil  time.Time
	width       int
	height      int
	action      listAction
	actionID    int64
	// lastClick is the last mouse click in the list, for the double click.
	lastClick listClick
}

// newListScreen creates the list screen with an empty list.
func newListScreen(st *store.Store, w, h int) listScreen {
	ti := textinput.New()
	ti.Placeholder = "Title"
	ti.CharLimit = 200
	ti.Width = 40

	return listScreen{
		st:     st,
		input:  ti,
		mode:   modeNormal,
		width:  w,
		height: h,
	}
}

// listToastDuration is how long a list toast stays visible.
const listToastDuration = 3 * time.Second

// listToastClear is the message that turns off the list toast when
// listToastDuration expires.
type listToastClear struct{}

// listToastTickCmd returns the command that after listToastDuration delivers
// listToastClear.
func listToastTickCmd() tea.Cmd {
	return tea.Tick(listToastDuration, func(time.Time) tea.Msg { return listToastClear{} })
}

// toastActive reports whether the toast is still to be shown.
func (s listScreen) toastActive() bool {
	return s.toast != "" && time.Now().Before(s.toastUntil)
}

// showToast sets the toast with the standard duration.
func (s listScreen) showToast(msg string) listScreen {
	s.toast = msg
	s.toastUntil = time.Now().Add(listToastDuration)
	return s
}

// load reloads from the store the folders and notes of the current view and
// applies the current filter to them, case-insensitively.
func (s *listScreen) load() {
	if s.showTrash {
		// The trash shows all deleted notes, from any folder.
		s.folders = nil
		s.folderNotes = nil
		notes, err := s.st.List(true)
		if err != nil {
			s.notes = nil
			s.selected = 0
			*s = s.showToast("error: " + err.Error())
			return
		}
		s.notes = filterNotes(notes, s.filter)
		s.clampSelected()
		return
	}

	folders, err := s.st.ListFolders(s.folderID)
	if err != nil {
		s.folders, s.notes = nil, nil
		s.selected = 0
		*s = s.showToast("error: " + err.Error())
		return
	}
	notes, err := s.st.ListIn(s.folderID, false, false)
	if err != nil {
		s.folders, s.notes = nil, nil
		s.selected = 0
		*s = s.showToast("error: " + err.Error())
		return
	}

	s.folders = filterFolders(folders, s.filter)
	s.folderNotes = make(map[int64]int, len(s.folders))
	for _, f := range s.folders {
		n, err := s.st.CountNotes(&f.ID)
		if err != nil {
			n = 0
		}
		s.folderNotes[f.ID] = n
	}
	s.notes = filterNotes(notes, s.filter)
	s.clampSelected()
}

// clampSelected brings the selection back within the number of current entries.
func (s *listScreen) clampSelected() {
	if s.selected < 0 || s.selected > s.entryCount()-1 {
		s.selected = 0
	}
}

// entryCount is the number of entries of the list, folders plus notes.
func (s listScreen) entryCount() int { return len(s.folders) + len(s.notes) }

// entries returns the entries of the list in the order they appear: first
// the folders, then the notes.
func (s listScreen) entries() []listEntry {
	out := make([]listEntry, 0, s.entryCount())
	for _, f := range s.folders {
		out = append(out, listEntry{
			kind:   entryFolder,
			id:     f.ID,
			title:  f.Name + "/",
			detail: folderDetail(s.folderNotes[f.ID]),
		})
	}
	for _, n := range s.notes {
		out = append(out, listEntry{
			kind:   entryNote,
			id:     n.ID,
			title:  n.Title,
			detail: noteDetail(n),
		})
	}
	return out
}

// folderDetail is the row below the name of a folder.
func folderDetail(notes int) string {
	switch notes {
	case 0:
		return "empty"
	case 1:
		return "1 note"
	default:
		return strconv.Itoa(notes) + " notes"
	}
}

// filterFolders keeps only the folders whose name contains filter, case-
// insensitively.
func filterFolders(folders []store.Folder, filter string) []store.Folder {
	q := strings.ToLower(strings.TrimSpace(filter))
	if q == "" {
		return folders
	}
	out := make([]store.Folder, 0, len(folders))
	for _, f := range folders {
		if strings.Contains(strings.ToLower(f.Name), q) {
			out = append(out, f)
		}
	}
	return out
}

// reload reloads the notes and returns the tick that turns off a possible toast.
func (s listScreen) reload() (listScreen, tea.Cmd) {
	s.load()
	if s.toast != "" {
		return s, listToastTickCmd()
	}
	return s, nil
}

// reloadAndSelect reloads the notes and moves the selection to the note with the given
// ID, if it is still in the list.
func (s listScreen) reloadAndSelect(id int64) listScreen {
	return s.reloadAndSelectEntry(entryNote, id)
}

// reloadAndSelectEntry reloads the list and moves the selection to the entry
// (folder or note) with the given id.
func (s listScreen) reloadAndSelectEntry(kind entryKind, id int64) listScreen {
	s.load()
	for i, e := range s.entries() {
		if e.kind == kind && e.id == id {
			s.selected = i
			return s
		}
	}
	return s
}

// filterNotes keeps only the notes whose title contains filter, case-
// insensitively.
func filterNotes(notes []store.Note, filter string) []store.Note {
	q := strings.ToLower(strings.TrimSpace(filter))
	if q == "" {
		return notes
	}
	out := make([]store.Note, 0, len(notes))
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n.Title), q) {
			out = append(out, n)
		}
	}
	return out
}

// currentEntry returns the selected entry, if any.
func (s listScreen) currentEntry() (listEntry, bool) {
	entries := s.entries()
	if s.selected < 0 || s.selected >= len(entries) {
		return listEntry{}, false
	}
	return entries[s.selected], true
}

// current returns the selected note, if the selected entry is a note.
func (s listScreen) current() (store.Note, bool) {
	e, ok := s.currentEntry()
	if !ok || e.kind != entryNote {
		return store.Note{}, false
	}
	for _, n := range s.notes {
		if n.ID == e.id {
			return n, true
		}
	}
	return store.Note{}, false
}

// enterFolder descends into the folder id, clearing the selection.
func (s listScreen) enterFolder(id int64) listScreen {
	f, err := s.st.GetFolder(id)
	if err != nil {
		return s.showToast("error: " + err.Error())
	}
	s.folderID = &f.ID
	s.path = append(s.path[:len(s.path):len(s.path)], *f)
	s.selected = 0
	s.load()
	return s
}

// goUp goes up to the folder that contains the current one; at the root it does
// nothing.
func (s listScreen) goUp() listScreen {
	if s.showTrash || len(s.path) == 0 {
		return s
	}
	s.path = s.path[:len(s.path)-1]
	if len(s.path) == 0 {
		s.folderID = nil
	} else {
		id := s.path[len(s.path)-1].ID
		s.folderID = &id
	}
	s.selected = 0
	s.load()
	return s
}

// setSize adapts the screen to the terminal size.
func (s listScreen) setSize(w, h int) listScreen {
	s.width, s.height = w, h
	return s
}

// Update handles the messages of the list screen.
func (s listScreen) Update(msg tea.Msg) (listScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case listToastClear:
		// The toast turns off only when its 3 seconds have really passed;
		// otherwise another tick is needed.
		if s.toastActive() {
			return s, listToastTickCmd()
		}
		s.toast = ""
		s.toastUntil = time.Time{}
		return s, nil
	case tea.KeyMsg:
		return s.updateKey(msg)
	case tea.MouseMsg:
		return s.updateMouse(msg)
	}
	return s, nil
}

// updateKey handles a key according to the shared scheme.
func (s listScreen) updateKey(keyMsg tea.KeyMsg) (listScreen, tea.Cmd) {
	key := keyMsg.String()
	mode := s.mode

	switch mode {
	case modeInputNew, modeInputRename, modeInputFilter, modeInputNewFolder:
		switch key {
		case "esc":
			s.mode = modeNormal
			s.input.Blur()
			if mode == modeInputFilter {
				s.filter = ""
				return s.reload()
			}
			return s, nil
		case "enter":
			return s.applyInput()
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(keyMsg)
			return s, cmd
		}

	case modeConfirmSoftDelete, modeConfirmPurge:
		switch key {
		case "n":
			fallthrough
		case "esc":
			s.mode = modeNormal
			return s, nil
		case "y":
			fallthrough
		case "enter":
			return s.applyConfirm()
		}
		return s, nil
	}

	switch key {
	case "q":
		fallthrough
	case "ctrl+q", "ctrl+c":
		s.action = listQuit
		return s, nil
	case "up":
		if s.selected > 0 {
			s.selected--
		}
		return s, nil
	case "down":
		if s.selected < s.entryCount()-1 {
			s.selected++
		}
		return s, nil
	case "right":
		fallthrough
	case "enter":
		e, ok := s.currentEntry()
		if !ok {
			return s, nil
		}
		if e.kind == entryFolder {
			return s.enterFolder(e.id), nil
		}
		if key == "enter" {
			s.action = listOpen
			s.actionID = e.id
		}
		return s, nil
	case "left":
		fallthrough
	case "esc":
		return s.goUp(), nil
	case "n":
		fallthrough
	case "ctrl+n":
		if s.showTrash {
			return s, nil
		}
		s.mode = modeInputNew
		s.input.Placeholder = "Title"
		s.input.SetValue("")
		cmd := s.input.Focus()
		return s, cmd
	case "c":
		fallthrough
	case "ctrl+k":
		if s.showTrash {
			return s, nil
		}
		s.mode = modeInputNewFolder
		s.input.Placeholder = "Folder"
		s.input.SetValue("")
		cmd := s.input.Focus()
		return s, cmd
	case "r":
		fallthrough
	case "ctrl+r":
		e, ok := s.currentEntry()
		if !ok || s.showTrash {
			return s, nil
		}
		s.mode = modeInputRename
		s.input.Placeholder = "Title"
		if e.kind == entryFolder {
			s.input.Placeholder = "Folder"
			s.input.SetValue(strings.TrimSuffix(e.title, "/"))
		} else {
			s.input.SetValue(e.title)
		}
		s.input.CursorEnd()
		cmd := s.input.Focus()
		return s, cmd
	case "d":
		fallthrough
	case "ctrl+d":
		if s.showTrash {
			return s, nil
		}
		if _, ok := s.currentEntry(); !ok {
			return s, nil
		}
		s.mode = modeConfirmSoftDelete
		return s, nil
	case "o":
		fallthrough
	case "ctrl+o":
		s.showTrash = !s.showTrash
		s.selected = 0
		s.folderID = nil
		s.path = nil
		s.input.Blur()
		return s.reload()
	case "x":
		fallthrough
	case "ctrl+x":
		if !s.showTrash {
			return s, nil
		}
		if _, ok := s.current(); !ok {
			return s, nil
		}
		s.mode = modeConfirmPurge
		return s, nil
	case "u":
		if !s.showTrash {
			return s, nil
		}
		n, ok := s.current()
		if !ok {
			return s, nil
		}
		if err := s.st.Restore(n.ID); err != nil {
			return s.showToast("error: " + err.Error()), listToastTickCmd()
		}
		s = s.showToast("note restored")
		return s.reload()
	case "f":
		fallthrough
	case "ctrl+f":
		s.mode = modeInputFilter
		s.input.Placeholder = "Filter"
		s.input.SetValue(s.filter)
		s.input.CursorEnd()
		cmd := s.input.Focus()
		return s, cmd
	}
	return s, nil
}

// listDoubleClickThreshold is the maximum interval between the first and second
// click for them to be recognised as a double click.
const listDoubleClickThreshold = 400 * time.Millisecond

// listClick is the last click recorded by the list: the note hit, the terminal
// cell (x, y) and the instant. The zero value means "no previous
// click".
type listClick struct {
	note int
	x    int
	y    int
	at   time.Time
}

// isDoubleClick is the pure recognition of the double click, with the instant now
// explicit so it can be tested without a clock: it is a double click if the
// previous click exists, if it hit the same note and the same cell, and if now
// is no more than listDoubleClickThreshold from prev.at.
func isDoubleClick(prev listClick, now time.Time, note, x, y int) bool {
	if prev.at.IsZero() || prev.note != note || prev.x != x || prev.y != y {
		return false
	}
	delta := now.Sub(prev.at)
	return delta >= 0 && delta <= listDoubleClickThreshold
}

// planeEntryIndex maps the row row of the plane built by listPlane to the index
// of the entry that appears there: every entry occupies two rows (title and detail) and the
// entries are separated by a separator row. The second value is false for
// separator rows and for rows outside the plane.
func planeEntryIndex(count, row int) (int, bool) {
	if row < 0 {
		return 0, false
	}
	pos := 0
	for i := 0; i < count; i++ {
		if row < pos+2 {
			return i, true
		}
		pos += 2
		if i < count-1 {
			pos++
		}
	}
	return 0, false
}

// listHitNote maps the screen cell (x, y) to the note that appears
// under it: it removes the margins, skips the title row and the footer one,
// reproduces the listWindow window and looks for the entry in the plane. The second
// value is false when the cell does not fall on any note.
func (s listScreen) listHitNote(x, y int) (int, bool) {
	cw, chh := contentW(s.width), contentH(s.height)
	if marginsActive(s.width, s.height) {
		x, y = x-marginX, y-marginY
	}
	if x < 0 || y < 0 || x >= cw || y >= chh || len(s.notes) == 0 {
		return 0, false
	}
	// The card is centred: clicks outside its width do not hit
	// any note, even if they fall in the content area.
	cardW := listCardW(cw)
	left := cardLeft(cw, cardW)
	if x < left || x >= left+cardW {
		return 0, false
	}
	// Row 0 of the content is the title and the last is the footer: in between is
	// the list, listH rows high.
	listH := listContentH(chh, 1)
	row := y - 1
	if row < 0 || row >= listH {
		return 0, false
	}
	_, selFrom, selTo := s.listPlane(cw)
	return planeEntryIndex(s.entryCount(), listWindowStart(selFrom, selTo, listH)+row)
}

// updateMouse handles the list's mouse: only in modeNormal and only for the
// left button, a click selects the entry (folder or note) under the given
// cell and a double click (second click on the same entry and the same
// cell within listDoubleClickThreshold) opens the note or enters the folder.
// In the other modes the mouse is ignored.
func (s listScreen) updateMouse(m tea.MouseMsg) (listScreen, tea.Cmd) {
	if s.mode != modeNormal {
		return s, nil
	}
	if m.Action != tea.MouseActionPress || m.Button != tea.MouseButtonLeft {
		return s, nil
	}
	idx, ok := s.listHitNote(m.X, m.Y)
	if !ok {
		return s, nil
	}
	now := time.Now()
	double := isDoubleClick(s.lastClick, now, idx, m.X, m.Y)
	s.lastClick = listClick{note: idx, x: m.X, y: m.Y, at: now}
	s.selected = idx
	if !double {
		return s, nil
	}
	// The double click opens the entry: a note is opened in the editor, a folder
	// is entered. The count restarts from zero, so a third click does not count
	// as another double click.
	s.lastClick = listClick{}
	entries := s.entries()
	if idx < 0 || idx >= len(entries) {
		return s, nil
	}
	if entries[idx].kind == entryFolder {
		return s.enterFolder(entries[idx].id), nil
	}
	s.action = listOpen
	s.actionID = entries[idx].id
	return s, nil
}

// applyInput performs the operation of the current input mode.
func (s listScreen) applyInput() (listScreen, tea.Cmd) {
	value := strings.TrimSpace(s.input.Value())

	if s.mode == modeInputFilter {
		s.filter = value
		s.selected = 0
		s.mode = modeNormal
		s.input.Blur()
		return s.reload()
	}

	if value == "" {
		return s.showToast("the name cannot be empty"), listToastTickCmd()
	}

	if s.mode == modeInputNewFolder {
		folder, err := s.st.CreateFolder(value, s.folderID)
		if err != nil {
			return s.showToast("error: " + err.Error()), listToastTickCmd()
		}
		s.mode = modeNormal
		s.input.Blur()
		s = s.showToast("folder created")
		s = s.reloadAndSelectEntry(entryFolder, folder.ID)
		return s, listToastTickCmd()
	}

	if s.mode == modeInputNew {
		note, err := s.st.CreateIn(value, s.folderID)
		if err != nil {
			return s.showToast("error: " + err.Error()), listToastTickCmd()
		}
		s.mode = modeNormal
		s.input.Blur()
		// The new note stays in the list: it is opened only with enter. The selection
		// moves to the note just created, which the store returns at the top
		// because it is the most recent.
		s = s.showToast("note created")
		s = s.reloadAndSelect(note.ID)
		return s, listToastTickCmd()
	}

	e, ok := s.currentEntry()
	if !ok {
		s.mode = modeNormal
		s.input.Blur()
		return s, nil
	}
	var err error
	if e.kind == entryFolder {
		err = s.st.RenameFolder(e.id, value)
	} else {
		err = s.st.Rename(e.id, value)
	}
	if err != nil {
		return s.showToast("error: " + err.Error()), listToastTickCmd()
	}
	s.mode = modeNormal
	s.input.Blur()
	return s.reload()
}

// applyConfirm performs the destructive operation confirmed by the popup.
func (s listScreen) applyConfirm() (listScreen, tea.Cmd) {
	purge := s.mode == modeConfirmPurge
	s.mode = modeNormal

	e, ok := s.currentEntry()
	if !ok {
		return s.reload()
	}

	var err error
	switch {
	case purge:
		err = s.st.Purge(e.id)
	case e.kind == entryFolder:
		err = s.st.SoftDeleteFolder(e.id)
	default:
		err = s.st.SoftDelete(e.id)
	}

	switch {
	case err != nil:
		s = s.showToast("error: " + err.Error())
	case purge:
		s = s.showToast("note deleted")
	case e.kind == entryFolder:
		s = s.showToast("folder moved to the trash")
	default:
		s = s.showToast("note moved to the trash")
	}
	return s.reload()
}

// listContentH is the height of the list rows for a content height
// ch and a title titleH rows high: space is left for the footer row.
func listContentH(ch, titleH int) int {
	h := ch - titleH - 1
	if h < 1 {
		return 1
	}
	return h
}

// listTitle is the title row of the list: "NOTES" with the path of the
// folders descended into, or "TRASH" in the trash view. It is
// centred over cw columns and rendered with theme.BannerAcc.
func (s listScreen) listTitle(cw int) string {
	parts := []string{"NOTES"}
	if s.showTrash {
		parts = []string{"TRASH"}
	} else {
		for _, f := range s.path {
			parts = append(parts, f.Name)
		}
	}
	return centerStyled(strings.Join(parts, " › "), cw, theme.BannerAcc)
}

// listFooterHint is the text of the list footer.
const listFooterHint = "↑↓ move · enter open · ← back · n note · c folder · ? help"

// listFooter renders the list footer: a single row truncated to cw columns,
// in dimmed colour without Faint.
func listFooter(cw int) string {
	return footerLine(listFooterHint, cw, theme.DimText)
}

// noteDetail is the row "date · N words" of a note, read from the Title,
// Body and UpdatedAt fields already present in the store.
func noteDetail(n store.Note) string {
	words := len(strings.Fields(n.Body))
	count := strconv.Itoa(words) + " words"
	if words == 1 {
		count = "1 word"
	}
	return n.UpdatedAt.Format("02/01/2006") + " · " + count
}

// listCardW is the width of the card that contains the notes: at most 60
// columns, and never more than the available width, so on narrow terminals the
// card takes all the space.
func listCardW(cw int) int {
	if cw < 60 {
		return cw
	}
	return 60
}

// cardLeft is the offset the card starts from, to centre it over cw
// columns.
func cardLeft(cw, cardW int) int {
	if cardW >= cw {
		return 0
	}
	return (cw - cardW) / 2
}

// cardText centres text over cardW columns, truncating it if needed, and renders it with
// the style st. The title and metadata of a note thus come out centred in the card,
// without any selection mark on the left: the selection is seen only from the
// style.
func cardText(text string, st lipgloss.Style, cardW int) string {
	if cardW <= 0 {
		return ""
	}
	plain := truncatePlain(text, cardW)
	w := lipgloss.Width(plain)
	left := (cardW - w) / 2
	right := cardW - w - left
	return strings.Repeat(" ", left) + st.Render(plain) + strings.Repeat(" ", right)
}

// listSeparator is the row that divides two notes: a line of ─ as wide as the
// card, dimmed.
func listSeparator(cardW int) string {
	if cardW <= 0 {
		return ""
	}
	return theme.DimText.Render(strings.Repeat("─", cardW))
}

// titleLine renders the title row of entry idx, centred in the card: highlighted
// for the selected one, dimmed for the others, with the trash styles
// in the trash view. Folders have a style of their own, so they stand out
// from notes at a glance.
func (s listScreen) titleLine(idx, cardW int, entries []listEntry) string {
	e := entries[idx]
	st := theme.DimText
	switch {
	case s.showTrash && idx == s.selected:
		st = theme.TrashSel
	case s.showTrash:
		st = theme.TrashDim
	case e.kind == entryFolder && idx == s.selected:
		st = theme.FolderSel
	case e.kind == entryFolder:
		st = theme.FolderDim
	case idx == s.selected:
		st = theme.Selected
	}
	return cardText(e.title, st, cardW)
}

// detailLine renders the second row of entry idx, centred in the card: number
// of notes for folders, date and words for notes. It is dimmed on all
// entries, whether their title is highlighted or in a low tone.
func (s listScreen) detailLine(idx, cardW int, entries []listEntry) string {
	st := theme.DimText
	if s.showTrash {
		st = theme.TrashDim
	}
	return cardText(entries[idx].detail, st, cardW)
}

// listPlane builds the plane of rows of the list: every entry (folder or
// note) occupies 2 rows, title and detail, and the entries are separated by a
// separator row. All the rows are listCardW(cw) wide. It also returns
// the range of rows occupied by the selected entry.
func (s listScreen) listPlane(cw int) (plane []string, selFrom, selTo int) {
	cardW := listCardW(cw)
	entries := s.entries()
	selFrom, selTo = 0, -1
	for i := range entries {
		from := len(plane)
		plane = append(plane, s.titleLine(i, cardW, entries), s.detailLine(i, cardW, entries))
		if i == s.selected {
			selFrom, selTo = from, len(plane)-1
		}
		if i < len(entries)-1 {
			plane = append(plane, listSeparator(cardW))
		}
	}
	return plane, selFrom, selTo
}

// listWindowStart is the row of the plane that ends up in the first row of the
// window listH rows high: it vertically centres the range of the selection.
func listWindowStart(selFrom, selTo, listH int) int {
	if selTo < selFrom {
		return 0
	}
	return selFrom - (listH-(selTo-selFrom+1))/2
}

// listWindow crops the plane to listH rows vertically centring
// the range of the selection; the rows outside the plane stay empty.
func listWindow(plane []string, selFrom, selTo, listH int) []string {
	rows := make([]string, listH)
	start := listWindowStart(selFrom, selTo, listH)
	for r := 0; r < listH; r++ {
		if i := start + r; i >= 0 && i < len(plane) {
			rows[r] = plane[i]
		}
	}
	return rows
}

// centerStyled centres plain over cw columns, with spaces on the left, and renders it with
// the style st.
func centerStyled(plain string, cw int, st lipgloss.Style) string {
	plain = truncatePlain(plain, cw)
	w := lipgloss.Width(plain)
	if w > cw {
		w = cw
	}
	left := (cw - w) / 2
	right := cw - w - left
	if right < 0 {
		right = 0
	}
	return strings.Repeat(" ", left) + st.Render(plain) + strings.Repeat(" ", right)
}

// boxInput is the content of the input popup: the title of the mode and the
// textinput row.
func (s listScreen) boxInput() string {
	title := "New note"
	switch s.mode {
	case modeInputRename:
		title = "Rename"
	case modeInputFilter:
		title = "Filter"
	case modeInputNewFolder:
		title = "New folder"
	}
	return theme.DimText.Render(title) + "\n" + s.input.View()
}

// boxConfirm is the content of the confirmation popup of the destructive operation.
func (s listScreen) boxConfirm() string {
	question := "Move the note to the trash? (y/n)"
	if e, ok := s.currentEntry(); ok && e.kind == entryFolder {
		question = "Move the folder and all its contents to the trash? (y/n)"
	}
	if s.mode == modeConfirmPurge {
		question = "Delete permanently? (y/n)"
	}
	return theme.DimText.Render(question)
}

// View renders the list screen: the title row, the plane of rows centred
// on the selection, the footer; on top of it all the popup of the current mode and
// the toast.
func (s listScreen) View() string {
	cw, chh := contentW(s.width), contentH(s.height)

	// The title occupies one row, the footer the last: the rest is the plane of rows.
	listH := listContentH(chh, 1)

	var rows []string
	if s.entryCount() == 0 {
		msg := "No notes — press n to create one"
		if s.showTrash {
			msg = "Trash is empty"
		}
		rows = make([]string, listH)
		rows[listH/2] = centerStyled(msg, cw, theme.DimText)
	} else {
		plane, from, to := s.listPlane(cw)
		rows = listWindow(plane, from, to, listH)
		// The card is centred horizontally: every row of the plane is
		// shifted by cardLeft and padded up to cw columns.
		left := strings.Repeat(" ", cardLeft(cw, listCardW(cw)))
		blank := strings.Repeat(" ", cw)
		for i, r := range rows {
			if r == "" {
				rows[i] = blank
				continue
			}
			rows[i] = padPlain(left+r, cw)
		}
	}

	lines := make([]string, 0, listH+2)
	lines = append(lines, s.listTitle(cw))
	lines = append(lines, rows...)
	lines = append(lines, listFooter(cw))
	if chh > 0 && len(lines) > chh {
		lines = lines[:chh]
	}
	content := strings.Join(lines, "\n")

	switch s.mode {
	case modeInputNew, modeInputRename, modeInputFilter, modeInputNewFolder:
		content = centerPopup(content, s.width, s.height, s.boxInput())
	case modeConfirmSoftDelete, modeConfirmPurge:
		content = centerPopup(content, s.width, s.height, s.boxConfirm())
	}

	if s.toastActive() {
		content = overlayAt(content, s.width, s.height, chh-1, 0, toastLine(s.toast, s.width))
	}

	return withMargins(content, s.width, s.height)
}
