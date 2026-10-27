package editor

import (
	tea "github.com/charmbracelet/bubbletea"
)

// --- Dispatch dei tasti ---

// Update gestisce i messaggi diretti all'editor.
//
// Schema dei tasti:
//   - sistema: "ctrl+s" (o "f2") salva, "ctrl+q" chiede di uscire (la schermata
//     decide se mostrare il popup Salva/Scarta/Annulla in base a Dirty),
//     "ctrl+c" chiede di uscire sempre.
//   - formattazione: prefisso "ctrl+t", la lettera successiva applica l'azione;
//     qualsiasi altro tasto annulla l'attesa e viene gestito normalmente qui
//     sotto. "f3" non attende una lettera: segnala con WantsPalette che la
//     schermata deve aprire la palette di formattazione, che poi applica la
//     voce scelta con ApplyFormat.
//   - movimento: frecce, "home", "end", "ctrl+up", "ctrl+down", "ctrl+home",
//     "ctrl+end", "pgup", "pgdown"; "ctrl+left"/"alt+left" portano all'inizio
//     della parola corrente o precedente, "ctrl+right"/"alt+right" alla fine
//     della parola corrente o successiva; "shift+ctrl+left" e
//     "shift+ctrl+right" fanno gli stessi movimenti estendendo la selezione.
//   - cancellazione: "backspace", "delete", "alt+backspace" (fino all'inizio
//     della parola corrente o precedente), "ctrl+u" (dal cursore a inizio riga)
//     e "ctrl+k" (dal cursore a fine riga).
//   - testo: tutte le rune di key.Runes (spazio, IME, paste); un paste
//     multilinea spezza la riga su '\n'.
func (e Editor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return e, nil
	}
	s := key.String()

	// ctrl+c esce sempre, anche con il prefisso di formattazione in attesa.
	if s == "ctrl+c" {
		e.awaitingFormat = false
		e.WantsQuit = true
		return e, nil
	}

	// Prefisso "ctrl+t": lo consuma SOLO una lettera di formattazione.
	// Qualunque altro tasto annulla l'attesa e prosegue nella gestione normale
	// (sistema, movimenti, testo), senza perdere il proprio effetto.
	if e.awaitingFormat {
		e.awaitingFormat = false
		if isFormatKey(s) {
			return e.applyFormatKey(s), nil
		}
	}

	switch s {
	case "ctrl+s":
		e.WantsSave = true
	case "f2":
		e.WantsSave = true
	case "ctrl+q", "esc":
		e.WantsExit = true
	case "ctrl+t":
		e.awaitingFormat = true
	case "f3":
		// F3 non è un prefisso: chiede alla schermata di aprire la palette.
		e.WantsPalette = true

	case "up":
		e = e.moveUp()
	case "down":
		e = e.moveDown()
	case "left":
		e = e.moveLeft()
	case "right":
		e = e.moveRight()
	case "home":
		e = e.moveHome()
	case "end":
		e = e.moveEnd()
	case "ctrl+up":
		e = e.moveDocStart()
	case "ctrl+down":
		e = e.moveDocEnd()
	case "ctrl+home":
		e = e.moveDocStart()
	case "ctrl+end":
		e = e.moveDocEnd()
	case "pgup":
		e = e.pageUp()
	case "pgdown":
		e = e.pageDown()
	case "ctrl+left", "alt+left":
		e = e.moveWordLeft()
	case "ctrl+right", "alt+right":
		e = e.moveWordRight()

	case "shift+up":
		e = withSelection(e, Editor.moveUp)
	case "shift+down":
		e = withSelection(e, Editor.moveDown)
	case "shift+left":
		e = withSelection(e, Editor.moveLeft)
	case "shift+right":
		e = withSelection(e, Editor.moveRight)
	case "shift+home":
		e = withSelection(e, Editor.moveHome)
	case "shift+end":
		e = withSelection(e, Editor.moveEnd)
	case "shift+ctrl+left":
		e = withSelection(e, Editor.moveWordLeft)
	case "shift+ctrl+right":
		e = withSelection(e, Editor.moveWordRight)

	case "backspace":
		e.backspace()
	case "delete":
		e.deleteForward()
	case "alt+backspace":
		e.deleteWordBack()
	case "ctrl+u":
		e.deleteToLineStart()
	case "ctrl+k":
		e.deleteToLineEnd()
	case "enter":
		e.newline()

	default:
		e = e.insertKeyText(key)
	}

	return e, nil
}

// insertKeyText inserisce il testo trasportato dal tasto: TUTTE le rune di
// key.Runes, senza richiedere key.Type == tea.KeyRunes (lo spazio arriva come
// tea.KeySpace con Runes [" "], il testo IME e il paste come KeyRunes con più
// rune). Vengono saltate solo le rune di controllo; un '\n' spezza la riga.
func (e Editor) insertKeyText(key tea.KeyMsg) Editor {
	if key.Alt || len(key.Runes) == 0 {
		return e
	}
	for _, r := range key.Runes {
		// Paste Windows "\r\n": il ritorno a capo è gestito dal '\n' che segue.
		if key.Paste && r == '\r' {
			continue
		}
		// Un paste multilinea spezza la riga invece di inserire '\n'.
		if r == '\n' {
			e.newline()
			continue
		}
		if r < ' ' {
			continue
		}
		e.insertRune(r)
	}
	return e
}

// isFormatKey dice se s è una delle lettere che consumano il prefisso "ctrl+t".
func isFormatKey(s string) bool {
	switch s {
	case "b", "i", "h", "k", "x", "l", "q", "s", "u", "0":
		return true
	}
	return false
}

// applyFormatKey applica la formattazione scelta dal tasto che segue il
// prefisso "ctrl+t". Un tasto non previsto non fa nulla (l'attesa è già stata
// annullata da Update, che lo gestisce normalmente).
func (e Editor) applyFormatKey(s string) Editor {
	switch s {
	case "b":
		e.toggleBold()
	case "i":
		e.toggleItalic()
	case "h":
		e.cycleHeading()
	case "k":
		e.toggleCode()
	case "x":
		e.toggleCheckbox()
	case "l":
		e.insertLink()
	case "q":
		e.toggleQuote()
	case "s":
		e.toggleStrike()
	case "u":
		e.toggleList()
	case "0":
		e.removeHeading()
	}
	return e
}

// ApplyFormat applica la formattazione della lettera data: esegue la stessa
// azione di applyFormatKey (quella del prefisso "ctrl+t") per b i h k x l q s u
// 0. È il punto d'ingresso della palette di formattazione (F3), che sceglie la
// lettera e la passa qui; una lettera non prevista lascia l'editor invariato.
func (e Editor) ApplyFormat(letter string) Editor {
	return e.applyFormatKey(letter)
}

// withSelection esegue un movimento mantenendo attiva la selezione:
// beginSelection prima del movimento, ancora e selezione ripristinate dopo
// (i movimenti senza shift azzerano la selezione).
func withSelection(e Editor, move func(Editor) Editor) Editor {
	e.beginSelection()
	al, ac := e.anchorLine, e.anchorCol
	e = move(e)
	e.anchorLine = al
	e.anchorCol = ac
	e.selecting = true
	return e
}

// --- Movimento per parola ---
//
// Parola = sequenza di lettere, cifre o underscore (isSelectWordRune); ogni
// altro carattere separa. I movimenti per parola restano dentro la riga del
// cursore: a inizio o fine riga non scavalcano la riga.

// wordStartCol è la colonna d'inizio della parola corrente o precedente:
// salta prima i separatori a sinistra di col, poi la parola stessa.
func wordStartCol(line []rune, col int) int {
	i := col
	if i > len(line) {
		i = len(line)
	}
	if i < 0 {
		i = 0
	}
	for i > 0 && !isSelectWordRune(line[i-1]) {
		i--
	}
	for i > 0 && isSelectWordRune(line[i-1]) {
		i--
	}
	return i
}

// wordEndCol è la colonna di fine della parola corrente o successiva: se col è
// dentro una parola (o sul suo primo carattere) salta la parola, altrimenti
// salta i separatori e poi la parola che segue.
func wordEndCol(line []rune, col int) int {
	i := col
	if i < 0 {
		i = 0
	}
	if i > len(line) {
		return len(line)
	}
	if i < len(line) && isSelectWordRune(line[i]) {
		for i < len(line) && isSelectWordRune(line[i]) {
			i++
		}
		return i
	}
	for i < len(line) && !isSelectWordRune(line[i]) {
		i++
	}
	for i < len(line) && isSelectWordRune(line[i]) {
		i++
	}
	return i
}

// moveWordLeft porta il cursore all'inizio della parola corrente o precedente.
func (e Editor) moveWordLeft() Editor {
	e.clearSelection()
	e.cursorCol = wordStartCol([]rune(e.currentLine()), e.cursorCol)
	e.ensureCursorVisible()
	return e
}

// moveWordRight porta il cursore alla fine della parola corrente o successiva.
func (e Editor) moveWordRight() Editor {
	e.clearSelection()
	e.cursorCol = wordEndCol([]rune(e.currentLine()), e.cursorCol)
	e.ensureCursorVisible()
	return e
}

// --- Modifica del testo ---

// insertRune inserisce una runa alla posizione del cursore.
func (e *Editor) insertRune(r rune) {
	if e.selecting {
		e.deleteSelection()
	}
	line := e.clampedLine()
	e.lines[e.cursorLine] = string(line[:e.cursorCol]) + string(r) + string(line[e.cursorCol:])
	e.cursorCol++
	e.Dirty = true
	e.ensureCursorVisible()
}

// backspace cancella la runa prima del cursore o unisce alla riga precedente.
func (e *Editor) backspace() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	line := []rune(e.currentLine())
	if e.cursorCol > len(line) {
		e.cursorCol = len(line)
	}
	if e.cursorCol > 0 {
		e.lines[e.cursorLine] = string(line[:e.cursorCol-1]) + string(line[e.cursorCol:])
		e.cursorCol--
	} else if e.cursorLine > 0 {
		prev := []rune(e.lines[e.cursorLine-1])
		e.lines[e.cursorLine-1] = string(prev) + string(line)
		e.lines = append(e.lines[:e.cursorLine], e.lines[e.cursorLine+1:]...)
		e.cursorLine--
		e.cursorCol = len(prev)
	}
	e.Dirty = true
	e.ensureCursorVisible()
}

// deleteForward cancella la runa al cursore o unisce la riga successiva.
func (e *Editor) deleteForward() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	line := []rune(e.currentLine())
	if e.cursorCol > len(line) {
		e.cursorCol = len(line)
	}
	if e.cursorCol < len(line) {
		e.lines[e.cursorLine] = string(line[:e.cursorCol]) + string(line[e.cursorCol+1:])
	} else if e.cursorLine < len(e.lines)-1 {
		e.lines[e.cursorLine] = string(line) + e.lines[e.cursorLine+1]
		e.lines = append(e.lines[:e.cursorLine+1], e.lines[e.cursorLine+2:]...)
	}
	e.Dirty = true
	e.ensureCursorVisible()
}

// deleteRange cancella le colonne [from, to) della riga del cursore e lascia il
// cursore in from. Le colonne vengono riportate dentro la riga.
func (e *Editor) deleteRange(from, to int) {
	line := []rune(e.currentLine())
	if from < 0 {
		from = 0
	}
	if to > len(line) {
		to = len(line)
	}
	if from > to {
		from, to = to, from
	}
	if from < 0 {
		from = 0
	}
	if to < from {
		to = from
	}
	e.lines[e.cursorLine] = string(line[:from]) + string(line[to:])
	e.cursorCol = from
	e.Dirty = true
	e.ensureCursorVisible()
}

// deleteWordBack cancella dal cursore fino all'inizio della parola corrente o
// precedente (alt+backspace). A inizio riga, senza parola prima del cursore,
// non cancella nulla.
func (e *Editor) deleteWordBack() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	e.clampCol()
	e.deleteRange(wordStartCol([]rune(e.currentLine()), e.cursorCol), e.cursorCol)
}

// deleteToLineStart cancella dal cursore a inizio riga (ctrl+u).
func (e *Editor) deleteToLineStart() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	e.clampCol()
	e.deleteRange(0, e.cursorCol)
}

// deleteToLineEnd cancella dal cursore a fine riga (ctrl+k).
func (e *Editor) deleteToLineEnd() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	e.clampCol()
	e.deleteRange(e.cursorCol, runeLen(e.currentLine()))
}

// newline spezza la riga al cursore e inserisce la nuova riga sotto.
func (e *Editor) newline() {
	if e.selecting {
		e.deleteSelection()
	}
	line := e.clampedLine()
	e.lines[e.cursorLine] = string(line[:e.cursorCol])
	e.lines = append(e.lines, "")
	copy(e.lines[e.cursorLine+2:], e.lines[e.cursorLine+1:])
	e.lines[e.cursorLine+1] = string(line[e.cursorCol:])
	e.cursorLine++
	e.cursorCol = 0
	e.Dirty = true
	e.ensureCursorVisible()
}

// deleteSelection cancella il testo selezionato.
func (e *Editor) deleteSelection() {
	if !e.selecting {
		return
	}
	sl, sc, el, ec := e.selectedRange()

	if sl == el {
		line := []rune(e.lines[sl])
		if sc > len(line) {
			sc = len(line)
		}
		if ec > len(line) {
			ec = len(line)
		}
		if sc > ec {
			sc, ec = ec, sc
		}
		e.lines[sl] = string(line[:sc]) + string(line[ec:])
	} else {
		first := []rune(e.lines[sl])
		last := []rune(e.lines[el])
		if sc > len(first) {
			sc = len(first)
		}
		if ec > len(last) {
			ec = len(last)
		}
		merged := string(first[:sc]) + string(last[ec:])
		e.replaceLines(sl, el, merged)
	}

	e.cursorLine = sl
	e.cursorCol = sc
	e.clearSelection()
	e.Dirty = true
	e.ensureCursorVisible()
}
