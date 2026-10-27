package editor

import (
	"regexp"
	"strings"
)

// --- Markdown formatting ---

// Regular expressions of the prefixes handled by formatting.
var (
	// headingRe recognises a heading "#... text".
	headingRe = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)
	// listRe recognises an unordered list item.
	listRe = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	// checkboxEmptyRe recognises an unchecked box.
	checkboxEmptyRe = regexp.MustCompile(`^(\s*)- \[ \](.*)$`)
	// checkboxCheckedRe recognises a checked box.
	checkboxCheckedRe = regexp.MustCompile(`^(\s*)- \[([xX])\](.*)$`)
	// quoteRe recognises a quote.
	quoteRe = regexp.MustCompile(`^>[ \t]?(.*)$`)
)

// indexRunes looks for needle in hay starting from from and returns the index in runes.
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

// affectedLines returns the first and last line index affected
// by the operation: the selection if active, otherwise the cursor's line.
func (e Editor) affectedLines() (first, last int) {
	if e.selecting {
		sl, _, el, _ := e.selectedRange()
		return sl, el
	}
	return e.cursorLine, e.cursorLine
}

// applyLines applies fn to every line of affectedLines. fn returns the new
// line and the difference in runes between the new and old prefix; if the
// cursor's line is among the modified ones, cursorCol is shifted by that
// difference (never below zero).
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

// wrapInline wraps the selection (or inserts an empty pair at the cursor)
// with open and close.
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

// toggleInline removes or inserts the marker around the cursor or
// the selection.
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
				// Remove the closing marker first, then the opening one.
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

	// No pair: insert marker+marker at the cursor.
	if e.cursorCol > len(line) {
		e.cursorCol = len(line)
	}
	e.lines[e.cursorLine] = string(line[:e.cursorCol]) + marker + marker + string(line[e.cursorCol:])
	e.cursorCol += runeLen(marker)
	e.Dirty = true
	e.ensureCursorVisible()
}

// toggleBold applies or removes bold.
func (e *Editor) toggleBold() { e.toggleInline("**") }

// toggleItalic applies or removes italics.
func (e *Editor) toggleItalic() { e.toggleInline("_") }

// toggleCode applies or removes inline code.
func (e *Editor) toggleCode() { e.toggleInline("`") }

// toggleStrike applies or removes strikethrough.
func (e *Editor) toggleStrike() { e.toggleInline("~~") }

// setHeading sets the heading level (1..3). Without a selection the
// line acts as a switch; with a selection the level is imposed on every line.
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

// cycleHeading cycles the heading of the affected lines through the levels
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

// removeHeading removes the heading from the affected lines.
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

// toggleList adds or removes the list bullet.
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

// toggleCheckbox adds or changes the state of the checkbox.
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

// toggleQuote adds or removes the quote prefix.
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

// insertLink inserts a markdown link, with the cursor between the square brackets.
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
