// Package app contiene le schermate dell'applicazione Nook.
package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/store"
)

// appName è il nome dell'applicazione, usato nel titolo della finestra quando
// la schermata attiva è la lista.
const appName = "Nook"

// dirtyMark precede il titolo della nota nel titolo della finestra quando
// l'editor attivo ha modifiche non salvate.
const dirtyMark = "● "

// screen identifica la schermata attiva.
type screen int

const (
	screenList screen = iota
	screenEditor
)

// listAction è l'azione richiesta dalla schermata lista al modello radice.
type listAction int

const (
	listNone listAction = iota
	listOpen
	listQuit
)

// editorAction è l'azione richiesta dalla schermata editor al modello radice.
type editorAction int

const (
	edNone editorAction = iota
	edBackToList
)

// Model è il modello radice dell'applicazione.
type Model struct {
	st       *store.Store
	screen   screen
	list     listScreen
	editor   editorScreen
	helpOpen bool
	width    int
	height   int
}

// New crea il modello radice con la lista delle note già caricata.
func New(st *store.Store) Model {
	m := Model{st: st, screen: screenList, width: 80, height: 24}
	m.list = newListScreen(st, 80, 24)
	m.list.load()
	m.editor = editorScreen{st: st}
	return m
}

// Init imposta il titolo iniziale della finestra e non esegue altro.
func (m Model) Init() tea.Cmd { return m.titleCmd() }

// Update gestisce i messaggi: i tasti vanno prima all'aiuto e alle uscite, poi
// alla schermata attiva; ogni altro messaggio (i tick dei toast) viene
// propagato alla schermata attiva, che lo converte nel comando che la riguarda.
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
		// Il titolo va riemesso dopo ogni tasto: Dirty può essere cambiato.
		return model, tea.Batch(cmd, model.titleCmd())
	}
	model, cmd := m.updateScreen(msg)
	return model, cmd
}

// updateKey gestisce un tasto a livello di applicazione. Il terzo valore dice
// se l'applicazione deve uscire.
func (m Model) updateKey(keyMsg tea.KeyMsg) (Model, tea.Cmd, bool) {
	logKey(keyMsg)

	key := keyMsg.String()

	// L'aiuto è un popup sopra la schermata corrente: ctrl+_ / f1 lo aprono e lo
	// chiudono, e qualsiasi altro tasto lo chiude senza arrivare alla schermata.
	if m.helpOpen {
		m.helpOpen = false
		return m, nil, false
	}

	if isHelpKey(key) {
		m.helpOpen = true
		return m, nil, false
	}

	// Solo ctrl+c esce subito: ctrl+q arriva alla schermata attiva, dove
	// nell'editor apre il popup di salvataggio e nella lista è gestito dalla lista.
	if key == "ctrl+c" {
		return m, nil, true
	}

	// "?" apre l'aiuto dalla lista, ma solo quando non si sta digitando in un
	// campo input: con modeNormal il ? è un tasto di comando, negli altri modi
	// è testo e deve arrivare alla lista.
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

// applyListAction esegue l'azione che la lista ha richiesto al modello radice:
// l'uscita o l'apertura della nota selezionata. La usano sia i tasti sia il
// mouse, che impostano lo stesso listAction. Il terzo valore dice se
// l'applicazione deve uscire.
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
			// L'errore diventa un toast della lista, con il suo tick.
			m.list = m.list.showToast("errore: " + err.Error())
			return m, tea.Batch(cmd, listToastTickCmd()), false
		}
		m.editor = newEditorScreen(m.st, note, m.width, m.height)
		m.screen = screenEditor
	}
	return m, cmd, false
}

// updateScreen inoltra alla schermata attiva i messaggi che non sono tasti:
// i tea.Tick dei toast, che la schermata trasforma nel comando opportuno, e il
// mouse della lista, che può chiedere di aprire una nota o di uscire.
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

// isHelpKey indica i tasti che aprono e chiudono l'aiuto: ctrl+_ (che il
// terminale recapita come "ctrl+/") e f1.
func isHelpKey(key string) bool {
	return key == "ctrl+/" || key == "ctrl+_" || key == "f1"
}

// windowTitle è il titolo della finestra: con l'editor attivo è il titolo della
// nota, preceduto da "●" quando ci sono modifiche non salvate; nella lista è il
// nome dell'applicazione.
func (m Model) windowTitle() string {
	if m.screen == screenEditor {
		if m.editor.Dirty() {
			return dirtyMark + m.editor.note.Title
		}
		return m.editor.note.Title
	}
	return appName
}

// titleCmd restituisce il comando che imposta il titolo della finestra.
func (m Model) titleCmd() tea.Cmd { return tea.SetWindowTitle(m.windowTitle()) }

// helpText è il testo breve dell'aiuto: i principali shortcut di sistema, di
// formattazione e di testo, il mouse e i comandi della lista.
const helpText = "Sistema: F2 salva · F3 formatta · F1 aiuto · ctrl+c esci subito\n" +
	"Formattazione: F3 apre la palette (frecce+invio) · ctrl+t poi b i s k u x l q h 0 · il markdown si può scrivere direttamente\n" +
	"Testo: ctrl+left/right o alt+left/right per parola · alt+backspace cancella parola · ctrl+u/k a inizio/fine riga\n" +
	"Selezione: shift+frecce, shift+home/end, shift+ctrl+left/right · col mouse trascina tenendo premuto il tasto sinistro\n" +
	"Mouse: click posiziona · doppio/triplo click seleziona parola/riga · in lista click seleziona, doppio click apre\n" +
	"Lista: n nota · c cartella · invio apre · → entra · ← esci · r rinomina · d cestina · u ripristina · o cestino · x elimina · f filtra · q esci"

// renderHelp costruisce il contenuto del popup di aiuto: il testo degli
// shortcut, reso riga per riga dentro lo spazio del popup e poi centrato da
// centerPopup sopra la schermata corrente.
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

// View rende la schermata attiva; con l'aiuto aperto la schermata resta
// visibile e sopra di essa viene sovrapposto il popup centrato dell'aiuto.
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

// contentOnly riporta una view totale (margini compresi) all'area contenuto:
// contentH(h) righe di contentW(w) colonne, pronte per centerPopup.
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
