package editor

import (
	tea "github.com/charmbracelet/bubbletea"
)

// --- Key dispatch ---

// Update handles the messages addressed to the editor.
//
// Key scheme:
//   - system: "ctrl+s" (or "f2") saves, "ctrl+q" asks to exit (the screen
//     decides whether to show the Save/Discard/Cancel popup based on Dirty),
//     "ctrl+c" always asks to exit.
//   - formatting: "ctrl+t" signals with WantsPalette that the screen has to
//     open the formatting palette, which then applies the chosen entry (picked
//     with its letter, or with the arrows and enter) through ApplyFormat.
//   - movement: arrows, "home", "end", "ctrl+up", "ctrl+down", "ctrl+home",
//     "ctrl+end", "pgup", "pgdown"; "ctrl+left"/"alt+left" go to the start
//     of the current or previous word, "ctrl+right"/"alt+right" to the end
//     of the current or next word; "shift+ctrl+left" and
//     "shift+ctrl+right" make the same movements extending the selection.
//   - deletion: "backspace", "delete", "alt+backspace" (up to the start
//     of the current or previous word), "ctrl+u" (from the cursor to the start of the line)
//     and "ctrl+k" (from the cursor to the end of the line).
//   - text: all the runes of key.Runes (space, IME, paste); a multi-line
//     paste splits the line on '\n'.
//   - history: "ctrl+z" undoes and "ctrl+y" redoes (a terminal sends
//     ctrl+shift+z as plain ctrl+z, so the two cannot be told apart).
func (e Editor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return e, nil
	}

	// Undo and redo; the changes made by every other key are recorded.
	switch key.String() {
	case "ctrl+z":
		return e.Undo(), nil
	case "ctrl+y":
		return e.Redo(), nil
	}
	before := e.snapshot()
	m, cmd := e.update(key)
	return m.(Editor).commit(before, editKind(key)), cmd
}

// update handles a key; Update wraps it to keep the undo history.
func (e Editor) update(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := key.String()

	// Any key ends a frozen view (see DragSelection).
	e.unfreeze()

	// ctrl+c always quits, even with the formatting prefix pending.
	if s == "ctrl+c" {
		e.WantsQuit = true
		return e, nil
	}

	switch s {
	case "ctrl+s":
		e.WantsSave = true
	case "f2":
		e.WantsSave = true
	case "ctrl+q", "esc":
		e.WantsExit = true
	case "ctrl+t":
		// It asks the screen to open the formatting palette.
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

// insertKeyText inserts the text carried by the key: ALL the runes of
// key.Runes, without requiring key.Type == tea.KeyRunes (space arrives as
// tea.KeySpace with Runes [" "], IME text and paste as KeyRunes with several
// runes). Only control runes are skipped; a '\n' splits the line.
func (e Editor) insertKeyText(key tea.KeyMsg) Editor {
	if key.Alt || len(key.Runes) == 0 {
		return e
	}
	for _, r := range key.Runes {
		// Windows paste "\r\n": the line break is handled by the '\n' that follows.
		if key.Paste && r == '\r' {
			continue
		}
		// A multi-line paste splits the line instead of inserting '\n'.
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

// applyFormatKey applies the formatting of the given letter; an unexpected
// letter does nothing.
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

// ApplyFormat applies the formatting of the given letter (b i h k x l q s u 0).
// It is the entry point of the formatting palette (ctrl+t), which chooses the
// letter and passes it here; an unexpected letter leaves the editor unchanged.
func (e Editor) ApplyFormat(letter string) Editor {
	before := e.snapshot()
	return e.applyFormatKey(letter).commit(before, "")
}

// withSelection performs a movement keeping the selection active:
// beginSelection before the movement, anchor and selection restored afterwards
// (movements without shift clear the selection).
func withSelection(e Editor, move func(Editor) Editor) Editor {
	e.beginSelection()
	al, ac := e.anchorLine, e.anchorCol
	e = move(e)
	e.anchorLine = al
	e.anchorCol = ac
	e.selecting = true
	return e
}

// --- Movement by word ---
//
// Word = sequence of letters, digits or underscore (isSelectWordRune); every
// other character separates. Word movements stay inside the cursor's
// line: at the start or end of the line they do not cross to another line.

// wordStartCol is the start column of the current or previous word:
// it first skips the separators to the left of col, then the word itself.
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

// wordEndCol is the end column of the current or next word: if col is
// inside a word (or on its first character) it skips the word, otherwise it
// skips the separators and then the word that follows.
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

// moveWordLeft brings the cursor to the start of the current or previous word.
func (e Editor) moveWordLeft() Editor {
	e.clearSelection()
	e.cursorCol = wordStartCol([]rune(e.currentLine()), e.cursorCol)
	e.ensureCursorVisible()
	return e
}

// moveWordRight brings the cursor to the end of the current or next word.
func (e Editor) moveWordRight() Editor {
	e.clearSelection()
	e.cursorCol = wordEndCol([]rune(e.currentLine()), e.cursorCol)
	e.ensureCursorVisible()
	return e
}

// --- Text editing ---

// insertRune inserts a rune at the cursor position.
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

// backspace deletes the rune before the cursor or merges with the previous line.
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

// deleteForward deletes the rune at the cursor or merges the next line.
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

// deleteRange deletes the columns [from, to) of the cursor's line and leaves the
// cursor at from. The columns are brought back inside the line.
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

// deleteWordBack deletes from the cursor to the start of the current or
// previous word (alt+backspace). At the start of a line, with no word before the cursor,
// it deletes nothing.
func (e *Editor) deleteWordBack() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	e.clampCol()
	e.deleteRange(wordStartCol([]rune(e.currentLine()), e.cursorCol), e.cursorCol)
}

// deleteToLineStart deletes from the cursor to the start of the line (ctrl+u).
func (e *Editor) deleteToLineStart() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	e.clampCol()
	e.deleteRange(0, e.cursorCol)
}

// deleteToLineEnd deletes from the cursor to the end of the line (ctrl+k).
func (e *Editor) deleteToLineEnd() {
	if e.selecting {
		e.deleteSelection()
		return
	}
	e.clampCol()
	e.deleteRange(e.cursorCol, runeLen(e.currentLine()))
}

// newline splits the line at the cursor and inserts the new line below.
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

// deleteSelection deletes the selected text.
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
