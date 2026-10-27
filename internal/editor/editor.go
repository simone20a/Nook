package editor

import (
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/markdown"
)

// Editor è il modello dell'editor di testo.
type Editor struct {
	Dirty     bool // esportato
	WantsSave bool // esportato, true quando Update riceve ctrl+s
	WantsExit bool // esportato, true quando Update riceve ctrl+q (o esc): chiede di uscire
	WantsQuit bool // esportato, true quando Update riceve ctrl+c: esce sempre
	// WantsPalette è esportato: true quando Update riceve f3, cioè quando va
	// aperta la palette di formattazione. La schermata lo legge e lo azzera.
	WantsPalette bool

	lines          []string
	cursorLine     int
	cursorCol      int
	hScroll        int
	width          int
	height         int
	selecting      bool
	anchorLine     int
	anchorCol      int
	awaitingFormat bool // true dopo ctrl+t: il prossimo tasto formatta
}

// runeLen conta le rune di una stringa.
func runeLen(s string) int { return len([]rune(s)) }

// New crea un Editor sulle righe date; se lines è nil usa una riga vuota.
// Il cursore parte in fondo alla prima riga, cioè dopo l'ultima runa del testo
// già presente: così la prima runa digitata si accoda al testo esistente
// invece di finirgli davanti, e il cursore resta visibile.
func New(lines []string, width, height int) Editor {
	if lines == nil {
		lines = []string{""}
	}
	e := Editor{
		lines:      lines,
		cursorLine: 0,
		cursorCol:  runeLen(lines[0]),
		width:      width,
		height:     height,
	}
	e.ensureCursorVisible()
	return e
}

// Init non esegue alcun comando.
func (e Editor) Init() tea.Cmd { return nil }

// Lines restituisce una copia delle righe.
func (e Editor) Lines() []string {
	out := make([]string, len(e.lines))
	copy(out, e.lines)
	return out
}

// LineCount è il numero di righe.
func (e Editor) LineCount() int { return len(e.lines) }

// CursorRow è il numero di riga del cursore, 1-based.
func (e Editor) CursorRow() int { return e.cursorLine + 1 }

// CursorCol1 è il numero di colonna del cursore, 1-based.
func (e Editor) CursorCol1() int { return e.cursorCol + 1 }

// AwaitingFormat dice se l'editor aspetta il tasto che applica la formattazione
// (vero dopo ctrl+t): la schermata lo usa per il testo del footer.
func (e Editor) AwaitingFormat() bool { return e.awaitingFormat }

// SetSize aggiorna le dimensioni e rende visibile il cursore.
func (e Editor) SetSize(width, height int) Editor {
	e.width = width
	e.height = height
	e.ensureCursorVisible()
	return e
}

// --- Geometria ---

// focusRow è la riga dello schermo dove si trova la riga del cursore.
func (e Editor) focusRow() int {
	r := (e.height - 1) / 2
	if r < 0 {
		r = 0
	}
	return r
}

// docIndexForRow converte una riga dello schermo nell'indice di documento.
func (e Editor) docIndexForRow(r int) int {
	return e.cursorLine - e.focusRow() + r
}

// LineAtRow restituisce l'indice della riga di documento disegnata nella riga r
// dello schermo dell'editor: la view rende una riga per ogni riga di documento,
// con la riga del cursore sulla riga di focus. Il secondo valore è falso quando
// r è fuori dalla view o quando la riga non corrisponde a nessuna riga del
// documento (per esempio le righe vuote oltre gli estremi).
func (e Editor) LineAtRow(r int) (int, bool) {
	if r < 0 || r >= e.height {
		return 0, false
	}
	idx := e.docIndexForRow(r)
	if idx < 0 || idx >= len(e.lines) {
		return 0, false
	}
	return idx, true
}

// LineAt restituisce il testo della riga idx, "" se è fuori dal documento.
func (e Editor) LineAt(idx int) string {
	if idx < 0 || idx >= len(e.lines) {
		return ""
	}
	return e.lines[idx]
}

// LineSegments restituisce i segmenti markdown con cui la view disegna la riga
// idx fuori dal focus, con dentro-fence calcolato come in renderRow: la
// schermata li usa per mappare una colonna in celle alla colonna nella riga
// grezza. Oltre il documento restituisce nil.
func (e Editor) LineSegments(idx int) []markdown.Segment {
	if idx < 0 || idx >= len(e.lines) {
		return nil
	}
	return markdown.RenderLine(e.lines[idx], isInsideFence(e.lines, idx))
}

// SetCursor porta il cursore sulla posizione (riga, colonna) data, con la riga
// e la colonna 0-based in rune, e azzera la selezione. Righe fuori dal
// documento vengono riportate dentro, la colonna viene riportata entro la fine
// della riga; il cursore risultante resta visibile. La riga del cursore è la
// riga di focus, quindi la riga indicata diventa la Focus Line.
func (e Editor) SetCursor(line, col int) Editor {
	e.clearSelection()
	if line < 0 {
		line = 0
	}
	if line > len(e.lines)-1 {
		line = len(e.lines) - 1
	}
	if line < 0 {
		line = 0
	}
	e.cursorLine = line
	if col < 0 {
		col = 0
	}
	if n := runeLen(e.currentLine()); col > n {
		col = n
	}
	e.cursorCol = col
	e.ensureCursorVisible()
	return e
}

// ensureCursorVisible aggiusta lo scroll orizzontale perché il cursore resti in vista.
func (e *Editor) ensureCursorVisible() {
	if e.cursorCol < e.hScroll {
		e.hScroll = e.cursorCol
	}
	if e.cursorCol >= e.hScroll+e.width {
		e.hScroll = e.cursorCol - e.width + 1
	}
	if e.hScroll < 0 {
		e.hScroll = 0
	}
}

// clampCol riporta il cursore entro la fine della riga corrente.
func (e *Editor) clampCol() {
	if e.cursorCol > runeLen(e.currentLine()) {
		e.cursorCol = runeLen(e.currentLine())
	}
}

// clampedLine restituisce la riga corrente come rune, dopo aver riportato la
// colonna del cursore dentro [0, lunghezza della riga].
func (e *Editor) clampedLine() []rune {
	line := []rune(e.currentLine())
	if e.cursorCol > len(line) {
		e.cursorCol = len(line)
	}
	if e.cursorCol < 0 {
		e.cursorCol = 0
	}
	return line
}

// replaceLines sostituisce le righe da sl a el (estremi inclusi) con la sola
// riga merged.
func (e *Editor) replaceLines(sl, el int, merged string) {
	out := make([]string, 0, len(e.lines)-(el-sl))
	out = append(out, e.lines[:sl]...)
	out = append(out, merged)
	out = append(out, e.lines[el+1:]...)
	e.lines = out
}

// currentLine è la riga sotto il cursore ("" se fuori range).
func (e Editor) currentLine() string {
	if e.cursorLine < 0 || e.cursorLine >= len(e.lines) {
		return ""
	}
	return e.lines[e.cursorLine]
}

// --- Movimenti ---

// moveUp sposta il cursore di una riga in su.
func (e Editor) moveUp() Editor {
	e.clearSelection()
	if e.cursorLine > 0 {
		e.cursorLine--
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// moveDown sposta il cursore di una riga in giù.
func (e Editor) moveDown() Editor {
	e.clearSelection()
	if e.cursorLine < len(e.lines)-1 {
		e.cursorLine++
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// moveLeft sposta il cursore di una runa a sinistra, a cavallo di riga.
func (e Editor) moveLeft() Editor {
	e.clearSelection()
	if e.cursorCol > 0 {
		e.cursorCol--
	} else if e.cursorLine > 0 {
		e.cursorLine--
		e.cursorCol = runeLen(e.currentLine())
	}
	e.ensureCursorVisible()
	return e
}

// moveRight sposta il cursore di una runa a destra, a cavallo di riga.
func (e Editor) moveRight() Editor {
	e.clearSelection()
	if e.cursorCol < runeLen(e.currentLine()) {
		e.cursorCol++
	} else if e.cursorLine < len(e.lines)-1 {
		e.cursorLine++
		e.cursorCol = 0
	}
	e.ensureCursorVisible()
	return e
}

// moveHome porta il cursore a inizio riga.
func (e Editor) moveHome() Editor {
	e.clearSelection()
	e.cursorCol = 0
	e.ensureCursorVisible()
	return e
}

// moveEnd porta il cursore a fine riga.
func (e Editor) moveEnd() Editor {
	e.clearSelection()
	e.cursorCol = runeLen(e.currentLine())
	e.ensureCursorVisible()
	return e
}

// moveDocStart porta il cursore a inizio documento.
func (e Editor) moveDocStart() Editor {
	e.clearSelection()
	e.cursorLine = 0
	e.cursorCol = 0
	e.ensureCursorVisible()
	return e
}

// moveDocEnd porta il cursore a fine documento.
func (e Editor) moveDocEnd() Editor {
	e.clearSelection()
	e.cursorLine = len(e.lines) - 1
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	e.cursorCol = runeLen(e.currentLine())
	e.ensureCursorVisible()
	return e
}

// pageUp sposta il cursore di height righe in su.
func (e Editor) pageUp() Editor {
	e.clearSelection()
	e.cursorLine -= e.height
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// pageDown sposta il cursore di height righe in giù.
func (e Editor) pageDown() Editor {
	e.clearSelection()
	e.cursorLine += e.height
	if e.cursorLine > len(e.lines)-1 {
		e.cursorLine = len(e.lines) - 1
	}
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// --- Selezione ---

// beginSelection fissa l'ancora della selezione se non è già attiva.
func (e *Editor) beginSelection() {
	if !e.selecting {
		e.anchorLine = e.cursorLine
		e.anchorCol = e.cursorCol
		e.selecting = true
	}
}

// clearSelection disattiva la selezione.
func (e *Editor) clearSelection() {
	e.selecting = false
}

// selectedRange normalizza ancora e cursore in ordine crescente di (riga, colonna).
func (e Editor) selectedRange() (sl, sc, el, ec int) {
	if e.anchorLine < e.cursorLine || (e.anchorLine == e.cursorLine && e.anchorCol <= e.cursorCol) {
		return e.anchorLine, e.anchorCol, e.cursorLine, e.cursorCol
	}
	return e.cursorLine, e.cursorCol, e.anchorLine, e.anchorCol
}

// isSelectWordRune dice se r è un carattere di parola per la selezione col
// mouse: una lettera, una cifra o l'underscore. Ogni altro carattere separa le
// parole.
func isSelectWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// SelectWord seleziona la parola sotto il cursore, riusando il modello di
// selezione esistente: l'ancora va all'inizio della parola e il cursore alla sua
// fine. Se il cursore sta subito dopo una parola (per esempio a fine riga) vale
// la parola che lo precede. Se sotto il cursore non c'è nessuna parola la
// selezione viene azzerata e il cursore resta dov'è.
func (e Editor) SelectWord() Editor {
	line := e.clampedLine()
	at := e.cursorCol
	if at >= len(line) || !isSelectWordRune(line[at]) {
		if at > 0 && isSelectWordRune(line[at-1]) {
			at--
		} else {
			e.clearSelection()
			e.ensureCursorVisible()
			return e
		}
	}
	start := at
	for start > 0 && isSelectWordRune(line[start-1]) {
		start--
	}
	end := at
	for end < len(line) && isSelectWordRune(line[end]) {
		end++
	}
	e.anchorLine = e.cursorLine
	e.anchorCol = start
	e.selecting = true
	e.cursorCol = end
	e.ensureCursorVisible()
	return e
}

// SelectLine seleziona l'intera riga del cursore, dall'inizio alla fine, con lo
// stesso modello di selezione (ancora a colonna 0, cursore in fondo alla riga).
func (e Editor) SelectLine() Editor {
	e.anchorLine = e.cursorLine
	e.anchorCol = 0
	e.selecting = true
	e.cursorCol = runeLen(e.currentLine())
	e.ensureCursorVisible()
	return e
}
