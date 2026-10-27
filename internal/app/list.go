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

// Modalità della schermata lista.
const (
	modeNormal = iota
	modeInputNew
	modeInputRename
	modeInputFilter
	modeInputNewFolder
	modeConfirmSoftDelete
	modeConfirmPurge
)

// entryKind distingue le due specie di voci della lista.
type entryKind int

const (
	entryFolder entryKind = iota
	entryNote
)

// listEntry è una voce della lista: una cartella o una nota. Il titolo è già
// pronto per la card (le cartelle hanno il suffisso "/"), il dettaglio è la
// riga sotto (numero di note per le cartelle, data e parole per le note).
type listEntry struct {
	kind   entryKind
	id     int64
	title  string
	detail string
}

// listScreen è la schermata della lista delle note: riga di titolo, piano di
// righe centrato sulla selezione, footer, popup per input, filtro e conferme,
// toast sovrapposti.
type listScreen struct {
	st        *store.Store
	notes     []store.Note
	folders   []store.Folder
	folderID  *int64
	path      []store.Folder
	showTrash bool
	// folderNotes è il numero di note contenute in ogni cartella mostrata.
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
	// lastClick è l'ultimo click col mouse nella lista, per il doppio click.
	lastClick listClick
}

// newListScreen crea la schermata lista con la lista vuota.
func newListScreen(st *store.Store, w, h int) listScreen {
	ti := textinput.New()
	ti.Placeholder = "Titolo"
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

// listToastDuration è quanto resta visibile un toast della lista.
const listToastDuration = 3 * time.Second

// listToastClear è il messaggio che spegne il toast della lista allo scadere
// di listToastDuration.
type listToastClear struct{}

// listToastTickCmd restituisce il comando che dopo listToastDuration recapita
// listToastClear.
func listToastTickCmd() tea.Cmd {
	return tea.Tick(listToastDuration, func(time.Time) tea.Msg { return listToastClear{} })
}

// toastActive indica se il toast è ancora da mostrare.
func (s listScreen) toastActive() bool {
	return s.toast != "" && time.Now().Before(s.toastUntil)
}

// showToast imposta il toast con la durata standard.
func (s listScreen) showToast(msg string) listScreen {
	s.toast = msg
	s.toastUntil = time.Now().Add(listToastDuration)
	return s
}

// load ricarica dallo store le cartelle e le note della vista corrente e vi
// applica il filtro corrente, senza distinzione fra maiuscole e minuscole.
func (s *listScreen) load() {
	if s.showTrash {
		// Il cestino mostra tutte le note eliminate, di qualunque cartella.
		s.folders = nil
		s.folderNotes = nil
		notes, err := s.st.List(true)
		if err != nil {
			s.notes = nil
			s.selected = 0
			*s = s.showToast("errore: " + err.Error())
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
		*s = s.showToast("errore: " + err.Error())
		return
	}
	notes, err := s.st.ListIn(s.folderID, false, false)
	if err != nil {
		s.folders, s.notes = nil, nil
		s.selected = 0
		*s = s.showToast("errore: " + err.Error())
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

// clampSelected riporta la selezione dentro il numero di voci correnti.
func (s *listScreen) clampSelected() {
	if s.selected < 0 || s.selected > s.entryCount()-1 {
		s.selected = 0
	}
}

// entryCount è il numero di voci della lista, cartelle più note.
func (s listScreen) entryCount() int { return len(s.folders) + len(s.notes) }

// entries restituisce le voci della lista nell'ordine in cui compaiono: prima
// le cartelle, poi le note.
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

// folderDetail è la riga sotto il nome di una cartella.
func folderDetail(notes int) string {
	switch notes {
	case 0:
		return "vuota"
	case 1:
		return "1 nota"
	default:
		return strconv.Itoa(notes) + " note"
	}
}

// filterFolders tiene solo le cartelle il cui nome contiene filter, senza
// distinzione fra maiuscole e minuscole.
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

// reload ricarica le note e restituisce il tick che spegne un eventuale toast.
func (s listScreen) reload() (listScreen, tea.Cmd) {
	s.load()
	if s.toast != "" {
		return s, listToastTickCmd()
	}
	return s, nil
}

// reloadAndSelect ricarica le note e sposta la selezione sulla nota con l'ID
// indicato, se è ancora nella lista.
func (s listScreen) reloadAndSelect(id int64) listScreen {
	return s.reloadAndSelectEntry(entryNote, id)
}

// reloadAndSelectEntry ricarica la lista e sposta la selezione sulla voce
// (cartella o nota) con l'id indicato.
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

// filterNotes tiene solo le note il cui titolo contiene filter, senza
// distinzione fra maiuscole e minuscole.
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

// currentEntry restituisce la voce selezionata, se c'è.
func (s listScreen) currentEntry() (listEntry, bool) {
	entries := s.entries()
	if s.selected < 0 || s.selected >= len(entries) {
		return listEntry{}, false
	}
	return entries[s.selected], true
}

// current restituisce la nota selezionata, se la voce selezionata è una nota.
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

// enterFolder scende nella cartella id, azzerando la selezione.
func (s listScreen) enterFolder(id int64) listScreen {
	f, err := s.st.GetFolder(id)
	if err != nil {
		return s.showToast("errore: " + err.Error())
	}
	s.folderID = &f.ID
	s.path = append(s.path[:len(s.path):len(s.path)], *f)
	s.selected = 0
	s.load()
	return s
}

// goUp risale alla cartella che contiene quella corrente; alla radice non fa
// nulla.
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

// setSize adatta la schermata alla dimensione del terminale.
func (s listScreen) setSize(w, h int) listScreen {
	s.width, s.height = w, h
	return s
}

// Update gestisce i messaggi della schermata lista.
func (s listScreen) Update(msg tea.Msg) (listScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case listToastClear:
		// Il toast si spegne solo se i suoi 3 secondi sono davvero passati;
		// altrimenti serve un altro tick.
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

// updateKey gestisce un tasto secondo lo schema condiviso.
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
		case "s":
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
		s.input.Placeholder = "Titolo"
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
		s.input.Placeholder = "Cartella"
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
		s.input.Placeholder = "Titolo"
		if e.kind == entryFolder {
			s.input.Placeholder = "Cartella"
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
			return s.showToast("errore: " + err.Error()), listToastTickCmd()
		}
		s = s.showToast("nota ripristinata")
		return s.reload()
	case "f":
		fallthrough
	case "ctrl+f":
		s.mode = modeInputFilter
		s.input.Placeholder = "Filtro"
		s.input.SetValue(s.filter)
		s.input.CursorEnd()
		cmd := s.input.Focus()
		return s, cmd
	}
	return s, nil
}

// listDoubleClickThreshold è l'intervallo massimo fra il primo e il secondo
// click perché siano riconosciuti come doppio click.
const listDoubleClickThreshold = 400 * time.Millisecond

// listClick è l'ultimo click registrato dalla lista: la nota colpita, la cella
// (x, y) del terminale e l'istante. Il valore zero vuol dire "nessun click
// prima".
type listClick struct {
	note int
	x    int
	y    int
	at   time.Time
}

// isDoubleClick è il riconoscimento puro del doppio click, con l'istante now
// esplicito per poterlo provare senza orologio: è doppio click se il click
// precedente esiste, se ha colpito la stessa nota e la stessa cella, e se now
// non dista più di listDoubleClickThreshold da prev.at.
func isDoubleClick(prev listClick, now time.Time, note, x, y int) bool {
	if prev.at.IsZero() || prev.note != note || prev.x != x || prev.y != y {
		return false
	}
	delta := now.Sub(prev.at)
	return delta >= 0 && delta <= listDoubleClickThreshold
}

// planeEntryIndex mappa la riga row del piano costruito da listPlane all'indice
// della voce che vi compare: ogni voce occupa due righe (titolo e dettaglio) e le
// voci sono separate da una riga di separazione. Il secondo valore è falso per le
// righe di separazione e per le righe fuori dal piano.
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

// listHitNote mappa la cella (x, y) dello schermo alla nota che vi compare
// sotto: toglie i margini, salta la riga di titolo e quella del footer,
// riproduce la finestra di listWindow e cerca la voce nel piano. Il secondo
// valore è falso quando la cella non cade su nessuna nota.
func (s listScreen) listHitNote(x, y int) (int, bool) {
	cw, chh := contentW(s.width), contentH(s.height)
	if marginsActive(s.width, s.height) {
		x, y = x-marginX, y-marginY
	}
	if x < 0 || y < 0 || x >= cw || y >= chh || len(s.notes) == 0 {
		return 0, false
	}
	// La card è centrata: i click fuori dalla sua larghezza non colpiscono
	// nessuna nota, anche se cadono nell'area contenuto.
	cardW := listCardW(cw)
	left := cardLeft(cw, cardW)
	if x < left || x >= left+cardW {
		return 0, false
	}
	// La riga 0 del contenuto è il titolo e l'ultima è il footer: in mezzo c'è
	// la lista, alta listH righe.
	listH := listContentH(chh, 1)
	row := y - 1
	if row < 0 || row >= listH {
		return 0, false
	}
	_, selFrom, selTo := s.listPlane(cw)
	return planeEntryIndex(s.entryCount(), listWindowStart(selFrom, selTo, listH)+row)
}

// updateMouse gestisce il mouse della lista: solo in modeNormal e solo per il
// tasto sinistro, un click seleziona la voce (cartella o nota) sotto la cella
// indicata e un doppio click (secondo click sulla stessa voce e sulla stessa
// cella entro listDoubleClickThreshold) apre la nota o entra nella cartella.
// Negli altri modi il mouse è ignorato.
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
	// Il doppio click apre la voce: una nota si apre nell'editor, una cartella
	// ci si entra. Il conteggio riparte da zero, così un terzo click non vale
	// come un altro doppio click.
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

// applyInput esegue l'operazione della modalità di input corrente.
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
		return s.showToast("il nome non può essere vuoto"), listToastTickCmd()
	}

	if s.mode == modeInputNewFolder {
		folder, err := s.st.CreateFolder(value, s.folderID)
		if err != nil {
			return s.showToast("errore: " + err.Error()), listToastTickCmd()
		}
		s.mode = modeNormal
		s.input.Blur()
		s = s.showToast("cartella creata")
		s = s.reloadAndSelectEntry(entryFolder, folder.ID)
		return s, listToastTickCmd()
	}

	if s.mode == modeInputNew {
		note, err := s.st.CreateIn(value, s.folderID)
		if err != nil {
			return s.showToast("errore: " + err.Error()), listToastTickCmd()
		}
		s.mode = modeNormal
		s.input.Blur()
		// La nota nuova resta nella lista: si apre solo con invio. La selezione
		// si sposta sulla nota appena creata, che lo store restituisce in cima
		// perché è la più recente.
		s = s.showToast("nota creata")
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
		return s.showToast("errore: " + err.Error()), listToastTickCmd()
	}
	s.mode = modeNormal
	s.input.Blur()
	return s.reload()
}

// applyConfirm esegue l'operazione distruttiva confermata dal popup.
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
		s = s.showToast("errore: " + err.Error())
	case purge:
		s = s.showToast("nota eliminata")
	case e.kind == entryFolder:
		s = s.showToast("cartella spostata nel cestino")
	default:
		s = s.showToast("nota spostata nel cestino")
	}
	return s.reload()
}

// listContentH è l'altezza delle righe della lista per un'altezza di contenuto
// ch e un titolo alto titleH righe: resta spazio per la riga del footer.
func listContentH(ch, titleH int) int {
	h := ch - titleH - 1
	if h < 1 {
		return 1
	}
	return h
}

// listTitle è la riga di titolo della lista: "NOTE" con il percorso delle
// cartelle in cui si è scesi, oppure "CESTINO" nella vista del cestino. È
// centrata su cw colonne e resa con theme.BannerAcc.
func (s listScreen) listTitle(cw int) string {
	parts := []string{"NOTE"}
	if s.showTrash {
		parts = []string{"CESTINO"}
	} else {
		for _, f := range s.path {
			parts = append(parts, f.Name)
		}
	}
	return centerStyled(strings.Join(parts, " › "), cw, theme.BannerAcc)
}

// listFooterHint è il testo del footer della lista.
const listFooterHint = "↑↓ muovi · invio apri · ← indietro · n nota · c cartella · ? aiuto"

// listFooter rende il footer della lista: una sola riga troncata a cw colonne,
// in colore attenuato senza Faint.
func listFooter(cw int) string {
	return footerLine(listFooterHint, cw, theme.DimText)
}

// noteDetail è la riga "data · N parole" di una nota, letta dai campi Title,
// Body e UpdatedAt già presenti nello store.
func noteDetail(n store.Note) string {
	words := len(strings.Fields(n.Body))
	count := strconv.Itoa(words) + " parole"
	if words == 1 {
		count = "1 parola"
	}
	return n.UpdatedAt.Format("02/01/2006") + " · " + count
}

// listCardW è la larghezza della card che contiene le note: al massimo 60
// colonne, e mai più della larghezza disponibile, così su terminali stretti la
// card occupa tutto lo spazio.
func listCardW(cw int) int {
	if cw < 60 {
		return cw
	}
	return 60
}

// cardLeft è lo scostamento da cui comincia la card, per centrarla su cw
// colonne.
func cardLeft(cw, cardW int) int {
	if cardW >= cw {
		return 0
	}
	return (cw - cardW) / 2
}

// cardText centra text su cardW colonne, troncandolo se serve, e lo rende con lo
// stile st. Titolo e metadati di una nota risultano così centrati nella card,
// senza alcun segno di selezione a sinistra: la selezione si vede solo dallo
// stile.
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

// listSeparator è la riga che divide due note: una linea di ─ larga quanto la
// card, attenuata.
func listSeparator(cardW int) string {
	if cardW <= 0 {
		return ""
	}
	return theme.DimText.Render(strings.Repeat("─", cardW))
}

// titleLine rende la riga di titolo della voce idx, centrata nella card: in
// evidenza per la selezionata, attenuata per le altre, con gli stili del cestino
// nella vista cestino. Le cartelle hanno uno stile proprio, così si distinguono
// dalle note a colpo d'occhio.
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

// detailLine rende la seconda riga della voce idx, centrata nella card: numero
// di note per le cartelle, data e parole per le note. È attenuata su tutte le
// voci, che abbiano già il titolo in evidenza o in tono basso.
func (s listScreen) detailLine(idx, cardW int, entries []listEntry) string {
	st := theme.DimText
	if s.showTrash {
		st = theme.TrashDim
	}
	return cardText(entries[idx].detail, st, cardW)
}

// listPlane costruisce il piano di righe della lista: ogni voce (cartella o
// nota) occupa 2 righe, titolo e dettaglio, e le voci sono separate da una riga
// di separazione. Tutte le righe sono larghe listCardW(cw). Restituisce anche
// l'intervallo di righe occupato dalla voce selezionata.
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

// listWindowStart è la riga del piano che finisce nella prima riga della
// finestra alta listH righe: centra verticalmente l'intervallo della selezione.
func listWindowStart(selFrom, selTo, listH int) int {
	if selTo < selFrom {
		return 0
	}
	return selFrom - (listH-(selTo-selFrom+1))/2
}

// listWindow ritaglia il piano su listH righe centrando verticalmente
// l'intervallo della selezione; le righe fuori dal piano restano vuote.
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

// centerStyled centra plain su cw colonne, con spazi a sinistra, e lo rende con
// lo stile st.
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

// boxInput è il contenuto del popup di input: il titolo della modalità e la
// riga del textinput.
func (s listScreen) boxInput() string {
	title := "Nuova nota"
	switch s.mode {
	case modeInputRename:
		title = "Rinomina"
	case modeInputFilter:
		title = "Filtra"
	case modeInputNewFolder:
		title = "Nuova cartella"
	}
	return theme.DimText.Render(title) + "\n" + s.input.View()
}

// boxConfirm è il contenuto del popup di conferma dell'operazione distruttiva.
func (s listScreen) boxConfirm() string {
	question := "Cestinare la nota? (s/n)"
	if e, ok := s.currentEntry(); ok && e.kind == entryFolder {
		question = "Cestinare la cartella e tutto il suo contenuto? (s/n)"
	}
	if s.mode == modeConfirmPurge {
		question = "Eliminare definitivamente? (s/n)"
	}
	return theme.DimText.Render(question)
}

// View rende la schermata lista: la riga di titolo, il piano di righe centrato
// sulla selezione, il footer; sopra il tutto il popup della modalità corrente e
// il toast.
func (s listScreen) View() string {
	cw, chh := contentW(s.width), contentH(s.height)

	// Il titolo occupa una riga, il footer l'ultima: il resto è piano di righe.
	listH := listContentH(chh, 1)

	var rows []string
	if s.entryCount() == 0 {
		msg := "Nessuna nota — n per crearne una"
		if s.showTrash {
			msg = "Cestino vuoto"
		}
		rows = make([]string, listH)
		rows[listH/2] = centerStyled(msg, cw, theme.DimText)
	} else {
		plane, from, to := s.listPlane(cw)
		rows = listWindow(plane, from, to, listH)
		// La card è centrata orizzontalmente: ogni riga del piano viene
		// scostata di cardLeft e completata fino a cw colonne.
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
