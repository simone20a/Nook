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

// edMode è la modalità della schermata editor.
type edMode int

const (
	edEditing edMode = iota
	edPrompt
	// edFormatPalette è la palette di formattazione aperta da F3.
	edFormatPalette
)

// editorScreen è la schermata dell'editor di una nota: il testo con il suo
// footer, il popup Salva/Scarta/Annulla e i toast sovrapposti.
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
	// lastClick è l'ultimo click col mouse nell'editor, per doppio e triplo click.
	lastClick edClick
	// paletteChoice è la voce corrente della palette di formattazione.
	paletteChoice int
	// Trascinamento col mouse: ancora della selezione e stato del gesto.
	dragLine int
	dragCol  int
	dragging bool
}

// newEditorScreen crea la schermata editor per la nota data.
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

// edHeight è l'altezza da dare al testo per un'altezza di contenuto ch: il
// contenuto è alto ch righe meno quella del footer, che la schermata disegna
// sotto il testo.
func edHeight(ch int) int {
	if ch-1 > 0 {
		return ch - 1
	}
	return 1
}

// setSize adatta la schermata alla dimensione del terminale.
func (s editorScreen) setSize(w, h int) editorScreen {
	s.width, s.height = w, h
	s.ed = s.ed.SetSize(contentW(w), edHeight(contentH(h)))
	return s
}

// Dirty indica se la nota ha modifiche non salvate.
func (s editorScreen) Dirty() bool { return s.ed.Dirty }

// edToastDuration è quanto resta visibile un toast dell'editor.
const edToastDuration = 3 * time.Second

// editorToastClear è il messaggio che spegne il toast dell'editor allo scadere
// di edToastDuration.
type editorToastClear struct{}

// edToastTickCmd restituisce il comando che dopo edToastDuration recapita
// editorToastClear.
func edToastTickCmd() tea.Cmd {
	return tea.Tick(edToastDuration, func(time.Time) tea.Msg { return editorToastClear{} })
}

// edToastVisible indica se il toast è ancora da mostrare.
func (s editorScreen) edToastVisible() bool {
	return s.toast != "" && time.Now().Before(s.toastUntil)
}

// edShowToast imposta il toast con la durata standard.
func (s editorScreen) edShowToast(msg string) editorScreen {
	s.toast = msg
	s.toastUntil = time.Now().Add(edToastDuration)
	return s
}

// save scrive il corpo corrente della nota nello store. Il terzo valore dice se
// il salvataggio è riuscito; in caso di errore il messaggio diventa un toast.
func (s editorScreen) save() (editorScreen, tea.Cmd, bool) {
	body := strings.Join(s.ed.Lines(), "\n")
	if err := s.st.Save(s.note.ID, s.note.Title, body); err != nil {
		s = s.edShowToast("errore: " + err.Error())
		return s, edToastTickCmd(), false
	}
	s.ed.Dirty = false
	return s, nil, true
}

// Update gestisce i messaggi della schermata editor.
func (s editorScreen) Update(msg tea.Msg) (editorScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case editorToastClear:
		// Il toast si spegne solo se i suoi 3 secondi sono davvero passati;
		// altrimenti serve un altro tick.
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

// edUpdateKey gestisce un tasto secondo lo schema condiviso.
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
		// F3: si apre la palette di formattazione sulla prima voce.
		s.ed.WantsPalette = false
		s.mode = edFormatPalette
		s.paletteChoice = 0
		return s, cmd
	}

	if s.ed.WantsSave {
		s.ed.WantsSave = false
		saved, saveCmd, ok := s.save()
		if !ok {
			// Errore: save() ha già impostato il toast; serve solo il suo tick.
			return saved, saveCmd
		}
		// Salvataggio riuscito: toast "salvato" con auto-hide, con lo stesso
		// meccanismo (edShowToast + edToastTickCmd) usato dall'errore.
		return saved.edShowToast("salvato"), edToastTickCmd()
	}

	if s.ed.WantsExit {
		// ctrl+q (e esc) su una nota modificata: si apre il popup
		// Salva/Scarta/Annulla, senza tornare alla lista; senza modifiche si
		// torna alla lista. Lo stato del popup è solo mode/promptChoice.
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

// edMultiClickThreshold è l'intervallo massimo fra due click sulla stessa cella
// perché valgano come doppio (o triplo) click.
const edMultiClickThreshold = 400 * time.Millisecond

// edClick è l'ultimo click registrato dall'editor: la cella (x, y) del
// terminale, quanti click consecutivi sono stati riconosciuti su quella cella e
// l'istante. Il valore zero vuol dire "nessun click prima".
type edClick struct {
	x     int
	y     int
	count int
	at    time.Time
}

// edClickCount riconosce click singolo, doppio e triplo: è un helper puro a
// tempo esplicito (l'istante now arriva da fuori) così è provabile senza
// orologio. Se il click precedente esiste, se ha colpito la stessa cella e se
// now non dista più di edMultiClickThreshold da prev.at il conteggio cresce
// fino a tre click; in ogni altro caso riparte da uno, così un quarto click
// vale come un click singolo nuovo.
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

// edHitLine mappa la cella (x, y) dello schermo alla riga grezza del documento
// che vi è disegnata: toglie i margini, scarta le celle fuori dall'area
// contenuto (compresa la riga del footer, che sta sotto il testo) e converte la
// riga della view nella riga del documento. Restituisce anche la colonna x
// dentro l'area contenuto.
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

// edHitCol converte la colonna cell in celle della riga line nella colonna in
// rune della riga grezza, con markdown.MapCellToRaw sui segmenti con cui la
// view disegna la riga: l'offset in byte che ne esce viene contato in rune,
// l'unità delle colonne dell'editor.
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

// edUpdateMouse gestisce il mouse dell'editor: solo in edEditing e solo per la
// pressione del tasto sinistro. Un click mappa la cella in (riga grezza,
// colonna grezza) e porta lì il cursore, così quella riga diventa la Focus
// Line; il doppio click (stessa cella entro edMultiClickThreshold) seleziona la
// parola sotto il cursore e il triplo click l'intera riga. Dal secondo click in
// poi il cursore non viene più riportato sulla cella: è il primo click ad
// averlo già posato lì, mentre la view può essersi scorsa.
func (s editorScreen) edUpdateMouse(m tea.MouseMsg) (editorScreen, tea.Cmd) {
	if s.mode != edEditing {
		return s, nil
	}

	// Trascinamento: fra la pressione e il rilascio del tasto sinistro i
	// movimenti del mouse estendono la selezione dall'ancora fissata al click
	// fino alla cella raggiunta.
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
		// Il click fissa l'ancora di un'eventuale selezione per trascinamento:
		// i movimenti del mouse con il tasto premuto la estendono da qui.
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

// --- Palette di formattazione (F3) ---

// edPaletteAction è una voce della palette di formattazione: la lettera che
// Editor.ApplyFormat interpreta e l'etichetta breve mostrata nel popup.
type edPaletteAction struct {
	key   string
	label string
}

// formatPaletteActions sono le 10 azioni della palette di formattazione,
// nell'ordine mostrato: la lettera è quella di applyFormatKey.
var formatPaletteActions = []edPaletteAction{
	{"b", "grassetto"},
	{"i", "corsivo"},
	{"h", "titolo"},
	{"k", "codice"},
	{"x", "checkbox"},
	{"l", "link"},
	{"q", "citazione"},
	{"s", "barrato"},
	{"u", "lista"},
	{"0", "togli titolo"},
}

// edUpdatePalette gestisce i tasti della palette di formattazione: su e giù
// cambiano la voce in modo circolare, invio applica la formattazione della voce
// corrente e chiude, esc chiude senza applicare. Finché la palette è aperta ogni
// altro tasto è sospeso, come nel popup di uscita.
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
	case "esc":
		s.mode = edEditing
	}
	return s, nil
}

// formatPaletteBox costruisce il contenuto del popup della palette: la riga di
// titolo e una riga per azione, con la voce corrente fra parentesi quadre.
func formatPaletteBox(s editorScreen) string {
	lines := make([]string, 0, len(formatPaletteActions)+1)
	lines = append(lines, edPromptStyle.Render("Formattazione"))
	for i, a := range formatPaletteActions {
		label := a.label + " (" + a.key + ")"
		if i == s.paletteChoice {
			lines = append(lines, theme.Selected.Render("["+label+"]"))
		} else {
			lines = append(lines, edPromptStyle.Render(" "+label+" "))
		}
	}
	return strings.Join(lines, "\n")
}

// edPromptChoices sono le scelte del popup di uscita, nell'ordine mostrato.
var edPromptChoices = []string{"Salva", "Scarta", "Annulla"}

// Indici delle scelte del popup.
const (
	edPromptSave = iota
	edPromptDiscard
	edPromptCancel
)

// edConfirmPrompt applica la scelta corrente del popup.
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

// edPromptStyle è lo stile del testo del popup.
var edPromptStyle = theme.DimText

// promptBox costruisce il contenuto del popup Salva/Scarta/Annulla: la riga di
// titolo, una riga vuota e la riga delle scelte con quella corrente tra
// parentesi quadre.
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
	return edPromptStyle.Render("Modifiche non salvate") + "\n\n" + line
}

// View rende la schermata editor: il testo, la riga del footer, con il popup del
// prompt e il toast sovrapposti.
func (s editorScreen) View() string {
	// Il testo occupa l'area contenuto meno la riga del footer.
	chh := contentH(s.height)
	var lines []string
	if body := s.ed.View(); body != "" {
		lines = append(strings.Split(body, "\n"), s.ed.Footer())
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
