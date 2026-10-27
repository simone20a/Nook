package editor

import (
	"regexp"
	"strings"
)

// --- Formattazione markdown ---

// Espressioni regolari dei prefissi gestiti dalla formattazione.
var (
	// headingRe riconosce un'intestazione "#... testo".
	headingRe = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)
	// listRe riconosce una voce di lista non ordinata.
	listRe = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	// checkboxEmptyRe riconosce una casella non spuntata.
	checkboxEmptyRe = regexp.MustCompile(`^(\s*)- \[ \](.*)$`)
	// checkboxCheckedRe riconosce una casella spuntata.
	checkboxCheckedRe = regexp.MustCompile(`^(\s*)- \[([xX])\](.*)$`)
	// quoteRe riconosce una citazione.
	quoteRe = regexp.MustCompile(`^>[ \t]?(.*)$`)
)

// indexRunes cerca needle in hay a partire da from e restituisce l'indice in rune.
func indexRunes(hay, needle []rune, from int) int {
	if len(needle) == 0 {
		return -1
	}
	if from < 0 {
		from = 0
	}
	for i := from; i+len(needle) <= len(hay); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// affectedLines restituisce il primo e l'ultimo indice di riga interessati
// dall'operazione: la selezione se attiva, altrimenti la riga del cursore.
func (e Editor) affectedLines() (first, last int) {
	if e.selecting {
		sl, _, el, _ := e.selectedRange()
		return sl, el
	}
	return e.cursorLine, e.cursorLine
}

// applyLines applica fn a ogni riga di affectedLines. fn restituisce la nuova
// riga e la differenza in rune tra nuovo e vecchio prefisso; se la riga del
// cursore è tra quelle modificate, cursorCol viene spostato di quella
// differenza (mai sotto zero).
func (e *Editor) applyLines(fn func(line string) (newLine string, delta int)) {
	first, last := e.affectedLines()
	if first > last {
		first, last = last, first
	}
	for i := first; i <= last; i++ {
		if i < 0 || i >= len(e.lines) {
			continue
		}
		newLine, delta := fn(e.lines[i])
		if newLine == e.lines[i] {
			continue
		}
		e.lines[i] = newLine
		if i == e.cursorLine && delta != 0 {
			e.cursorCol += delta
			if e.cursorCol < 0 {
				e.cursorCol = 0
			}
		}
	}
	e.Dirty = true
	e.ensureCursorVisible()
}

// wrapInline racchiude la selezione (o inserisce una coppia vuota al cursore)
// con open e close.
func (e *Editor) wrapInline(open, close string) {
	if e.selecting {
		sl, sc, el, ec := e.selectedRange()
		if sl < 0 || el >= len(e.lines) || sl > el {
			e.clearSelection()
			e.Dirty = true
			e.ensureCursorVisible()
			return
		}
		first := []rune(e.lines[sl])
		if sc < 0 {
			sc = 0
		}
		if sc > len(first) {
			sc = len(first)
		}
		if sl == el {
			if ec < sc {
				ec = sc
			}
			if ec > len(first) {
				ec = len(first)
			}
			text := string(first[sc:ec])
			e.lines[sl] = string(first[:sc]) + open + text + close + string(first[ec:])
		} else {
			lastLine := []rune(e.lines[el])
			if ec < 0 {
				ec = 0
			}
			if ec > len(lastLine) {
				ec = len(lastLine)
			}
			parts := make([]string, 0, el-sl+1)
			parts = append(parts, string(first[sc:]))
			for i := sl + 1; i < el; i++ {
				parts = append(parts, e.lines[i])
			}
			parts = append(parts, string(lastLine[:ec]))
			text := strings.Join(parts, "\n")
			merged := string(first[:sc]) + open + text + close + string(lastLine[ec:])
			e.replaceLines(sl, el, merged)
		}
		e.cursorLine = sl
		e.cursorCol = sc + runeLen(open)
		e.clearSelection()
	} else {
		line := e.clampedLine()
		e.lines[e.cursorLine] = string(line[:e.cursorCol]) + open + close + string(line[e.cursorCol:])
		e.cursorCol += runeLen(open)
	}
	e.Dirty = true
	e.ensureCursorVisible()
}

// toggleInline rimuove o inserisce il marcatore marker attorno al cursore o
// alla selezione.
func (e *Editor) toggleInline(marker string) {
	if e.selecting {
		e.wrapInline(marker, marker)
		return
	}

	m := []rune(marker)
	line := []rune(e.currentLine())
	if e.cursorCol < 0 {
		e.cursorCol = 0
	}

	if len(m) > 0 && len(line) > 0 {
		for p := indexRunes(line, m, 0); p >= 0; p = indexRunes(line, m, p+1) {
			q := indexRunes(line, m, p+len(m))
			if q < 0 {
				continue
			}
			if p+len(m) <= e.cursorCol && e.cursorCol <= q {
				// Rimuovi prima il marcatore di chiusura, poi quello di apertura.
				line = append(line[:q], line[q+len(m):]...)
				line = append(line[:p], line[p+len(m):]...)
				if p < e.cursorCol {
					e.cursorCol -= len(m)
				}
				if e.cursorCol < 0 {
					e.cursorCol = 0
				}
				if e.cursorCol > len(line) {
					e.cursorCol = len(line)
				}
				e.lines[e.cursorLine] = string(line)
				e.Dirty = true
				e.ensureCursorVisible()
				return
			}
		}
	}

	// Nessuna coppia: inserisci marker+marker al cursore.
	if e.cursorCol > len(line) {
		e.cursorCol = len(line)
	}
	e.lines[e.cursorLine] = string(line[:e.cursorCol]) + marker + marker + string(line[e.cursorCol:])
	e.cursorCol += runeLen(marker)
	e.Dirty = true
	e.ensureCursorVisible()
}

// toggleBold applica o toglie il grassetto.
func (e *Editor) toggleBold() { e.toggleInline("**") }

// toggleItalic applica o toglie il corsivo.
func (e *Editor) toggleItalic() { e.toggleInline("_") }

// toggleCode applica o toglie il codice inline.
func (e *Editor) toggleCode() { e.toggleInline("`") }

// toggleStrike applica o toglie il barrato.
func (e *Editor) toggleStrike() { e.toggleInline("~~") }

// setHeading imposta il livello di intestazione (1..3). Senza selezione la
// riga fa da interruttore; con selezione il livello viene imposto a ogni riga.
func (e *Editor) setHeading(level int) {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	prefix := strings.Repeat("#", level) + " "

	if !e.selecting {
		line := e.currentLine()
		if m := headingRe.FindStringSubmatch(line); m != nil {
			var newLine string
			if runeLen(m[1]) == level {
				newLine = m[2]
			} else {
				newLine = prefix + m[2]
			}
			e.lines[e.cursorLine] = newLine
			e.cursorCol += runeLen(newLine) - runeLen(line)
		} else {
			e.lines[e.cursorLine] = prefix + line
			e.cursorCol += runeLen(prefix)
		}
		if e.cursorCol < 0 {
			e.cursorCol = 0
		}
		e.Dirty = true
		e.ensureCursorVisible()
		return
	}

	e.applyLines(func(line string) (string, int) {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			newLine := prefix + m[2]
			return newLine, runeLen(newLine) - runeLen(line)
		}
		newLine := prefix + line
		return newLine, runeLen(newLine) - runeLen(line)
	})
}

// cycleHeading fa scorrere il titolo delle righe interessate tra i livelli
// none -> H1 -> H2 -> H3 -> none.
func (e *Editor) cycleHeading() {
	first, _ := e.affectedLines()
	level := 1
	if first >= 0 && first < len(e.lines) {
		if m := headingRe.FindStringSubmatch(e.lines[first]); m != nil {
			switch runeLen(m[1]) {
			case 1:
				level = 2
			case 2:
				level = 3
			default:
				e.removeHeading()
				return
			}
		}
	}
	e.setHeading(level)
}

// removeHeading toglie l'intestazione dalle righe interessate.
func (e *Editor) removeHeading() {
	e.applyLines(func(line string) (string, int) {
		m := headingRe.FindStringSubmatch(line)
		if m == nil {
			return line, 0
		}
		newLine := m[2]
		return newLine, runeLen(newLine) - runeLen(line)
	})
}

// toggleList aggiunge o toglie il bullet di lista.
func (e *Editor) toggleList() {
	e.applyLines(func(line string) (string, int) {
		if m := listRe.FindStringSubmatch(line); m != nil {
			newLine := m[1] + m[2]
			return newLine, runeLen(newLine) - runeLen(line)
		}
		newLine := "- " + line
		return newLine, runeLen(newLine) - runeLen(line)
	})
}

// toggleCheckbox aggiunge o cambia lo stato della casella di controllo.
func (e *Editor) toggleCheckbox() {
	e.applyLines(func(line string) (string, int) {
		if m := checkboxEmptyRe.FindStringSubmatch(line); m != nil {
			newLine := m[1] + "- [x]" + m[2]
			return newLine, runeLen(newLine) - runeLen(line)
		}
		if m := checkboxCheckedRe.FindStringSubmatch(line); m != nil {
			newLine := m[1] + "- [ ]" + m[3]
			return newLine, runeLen(newLine) - runeLen(line)
		}
		newLine := "- [ ] " + line
		return newLine, runeLen(newLine) - runeLen(line)
	})
}

// toggleQuote aggiunge o toglie il prefisso di citazione.
func (e *Editor) toggleQuote() {
	e.applyLines(func(line string) (string, int) {
		if m := quoteRe.FindStringSubmatch(line); m != nil {
			newLine := m[1]
			return newLine, runeLen(newLine) - runeLen(line)
		}
		newLine := "> " + line
		return newLine, runeLen(newLine) - runeLen(line)
	})
}

// insertLink inserisce un link markdown, col cursore tra le parentesi quadre.
func (e *Editor) insertLink() {
	if e.selecting {
		e.wrapInline("[", "](https://)")
		return
	}
	line := e.clampedLine()
	const link = "[](https://)"
	e.lines[e.cursorLine] = string(line[:e.cursorCol]) + link + string(line[e.cursorCol:])
	e.cursorCol++
	e.Dirty = true
	e.ensureCursorVisible()
}
