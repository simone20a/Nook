package editor

import (
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss"

	"nook/internal/markdown"
	"nook/internal/theme"
)

// selectionOnLine restituisce l'intervallo di rune selezionato nella riga line,
// con l'estremo superiore escluso. Il secondo valore è falso quando la riga non
// ha nulla di selezionato.
func (e Editor) selectionOnLine(line int) (from, to int, ok bool) {
	if !e.selecting || line < 0 || line >= len(e.lines) {
		return 0, 0, false
	}
	sl, sc, el, ec := e.selectedRange()
	if line < sl || line > el {
		return 0, 0, false
	}
	n := runeLen(e.lines[line])
	from, to = 0, n
	if line == sl {
		from = sc
	}
	if line == el {
		to = ec
	}
	if from < 0 {
		from = 0
	}
	if to > n {
		to = n
	}
	if from >= to {
		return 0, 0, false
	}
	return from, to, true
}

// SetSelection imposta cursore e ancora in un colpo solo: serve al trascinamento
// col mouse, che fissa l'ancora dove si è premuto e porta il cursore dove si è
// trascinato.
func (e Editor) SetSelection(anchorLine, anchorCol, line, col int) Editor {
	e.selecting = true
	e.anchorLine = clampLine(anchorLine, len(e.lines))
	e.anchorCol = anchorCol
	e.cursorLine = clampLine(line, len(e.lines))
	e.cursorCol = col
	e.clampCol()
	e.ensureCursorVisible()
	return e
}

// clampLine riporta una riga dentro i limiti del documento.
func clampLine(line, count int) int {
	if line < 0 {
		return 0
	}
	if line > count-1 {
		return count - 1
	}
	return line
}

// SelectedText restituisce il testo selezionato, con le righe separate da "\n".
// Il secondo valore è falso quando non c'è selezione.
func (e Editor) SelectedText() (string, bool) {
	if !e.selecting {
		return "", false
	}
	sl, sc, el, ec := e.selectedRange()
	if sl == el && sc == ec {
		return "", false
	}
	if sl < 0 || el >= len(e.lines) {
		return "", false
	}
	if sl == el {
		runes := []rune(e.lines[sl])
		return string(runes[minInt(sc, len(runes)):minInt(ec, len(runes))]), true
	}
	var parts []string
	first := []rune(e.lines[sl])
	parts = append(parts, string(first[minInt(sc, len(first)):]))
	for i := sl + 1; i < el; i++ {
		parts = append(parts, e.lines[i])
	}
	last := []rune(e.lines[el])
	parts = append(parts, string(last[:minInt(ec, len(last))]))
	return strings.Join(parts, "\n"), true
}

// minInt restituisce il minore dei due interi.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runeRangeToBytes traduce l'intervallo di rune [from, to) della stringa s nei
// corrispondenti offset in byte.
func runeRangeToBytes(s string, from, to int) (int, int) {
	rs := []rune(s)
	if from < 0 {
		from = 0
	}
	if from > len(rs) {
		from = len(rs)
	}
	if to < from {
		to = from
	}
	if to > len(rs) {
		to = len(rs)
	}
	return len(string(rs[:from])), len(string(rs[:to]))
}

// renderSegmentWithSelection rende un segmento markdown con la sua palette,
// evidenziando la parte che cade dentro l'intervallo selezionato della riga,
// espresso in offset di byte sul testo grezzo [selFrom, selTo).
//
// La corrispondenza fra byte grezzi e caratteri visibili del segmento è
// proporzionale: i marcatori markdown non compaiono a video, quindi il testo del
// segmento è più corto del suo span di sorgente e non esiste una mappa esatta
// carattere per carattere.
func renderSegmentWithSelection(seg markdown.Segment, selFrom, selTo int, st lipgloss.Style) string {
	start, end, ok := segmentOverlap(seg, selFrom, selTo)
	if !ok {
		return st.Render(seg.Text)
	}
	runes := []rune(seg.Text)
	return st.Render(string(runes[:start])) +
		theme.Selection.Render(string(runes[start:end])) +
		st.Render(string(runes[end:]))
}

// segmentOverlap traduce la sovrapposizione fra lo span di sorgente del
// segmento e la selezione in indici di runa dentro seg.Text.
func segmentOverlap(seg markdown.Segment, selFrom, selTo int) (start, end int, ok bool) {
	a, b := seg.RawFrom, seg.RawTo
	if b <= a || selTo <= a || selFrom >= b {
		return 0, 0, false
	}
	runes := []rune(seg.Text)
	n := len(runes)
	if n == 0 {
		return 0, 0, false
	}
	lo, hi := selFrom, selTo
	if lo < a {
		lo = a
	}
	if hi > b {
		hi = b
	}
	if lo >= hi {
		return 0, 0, false
	}
	span := b - a
	start = (lo - a) * n / span
	end = (hi - a) * n / span
	if end <= start {
		// Sovrapposizione troppo piccola per un carattere: si evidenzia comunque
		// quello più vicino, così la selezione resta visibile anche dentro i
		// marcatori.
		if start < n {
			end = start + 1
		} else {
			start, end = n-1, n
		}
	}
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start >= end {
		return 0, 0, false
	}
	return start, end, true
}
