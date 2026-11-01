package editor

import (
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/theme"
)

// --- Vim mode ---
//
// With vim mode on (WithVim, turned on by the screen when NOOK_VIM=1) the
// editor is modal: NORMAL mode takes motions, operators and commands, INSERT
// mode types text with the usual keys, VISUAL and V-LINE select characters or
// whole lines. A ":" command line handles :w, :q, :wq, :x, :q!, :<line> and
// :$. The system keys (ctrl+s, ctrl+q, ctrl+c, ctrl+t, ctrl+y) work in every
// mode.

// vimMode is the current mode of vim mode.
type vimMode int

const (
	vimNormal vimMode = iota
	vimInsert
	vimVisual
	vimVisualLine
)

// vimModeNames are the names the footer shows for each mode.
var vimModeNames = [...]string{
	vimNormal:     "NORMAL",
	vimInsert:     "INSERT",
	vimVisual:     "VISUAL",
	vimVisualLine: "V-LINE",
}

// vimHints are the footer hints of each mode, after its name.
var vimHints = [...]string{
	vimNormal:     ":w save · :q exit · i insert · v select · ctrl+/ help",
	vimInsert:     "esc normal · ctrl+s save · ctrl+t format · ctrl+/ help",
	vimVisual:     "d delete · y copy · c change · ctrl+t format · esc cancel",
	vimVisualLine: "d delete · y copy · c change · ctrl+t format · esc cancel",
}

// vimMaxCount caps the count typed before a command.
const vimMaxCount = 99999

// vimInsertEdit is the history kind of the changes made in INSERT mode: the
// command that enters it (o, cw, ...) and everything typed until esc are one
// step of the history, as in vim.
const vimInsertEdit = "vim-insert"

// WithVim turns vim mode on: the editor starts in NORMAL mode.
func (e Editor) WithVim() Editor {
	e.vim = true
	e.vimMode = vimNormal
	e.normalClamp()
	return e
}

// VimBusy reports whether vim mode is in the middle of something that esc
// leaves: INSERT or VISUAL mode, a pending command or the ":" command line.
func (e Editor) VimBusy() bool {
	return e.vim && (e.vimMode != vimNormal || e.vimPending != "" || e.vimCount > 0 || e.vimCmdOn)
}

// inVisual reports whether vim mode is selecting (VISUAL or V-LINE).
func (e Editor) inVisual() bool {
	return e.vim && (e.vimMode == vimVisual || e.vimMode == vimVisualLine)
}

// visualRange widens the ordered selection range [sl:sc, el:ec) to what the
// VISUAL modes cover: VISUAL includes the character under its later end,
// V-LINE takes the lines whole.
func (e Editor) visualRange(sl, sc, el, ec int) (int, int, int, int) {
	if e.vimMode == vimVisualLine {
		return sl, 0, el, runeLen(e.LineAt(el))
	}
	return sl, sc, el, ec + 1
}

// normalClamp keeps the cursor on a character, as NORMAL mode wants: never
// past the last rune of the line.
func (e *Editor) normalClamp() {
	if n := runeLen(e.currentLine()); e.cursorCol >= n {
		e.cursorCol = n - 1
	}
	if e.cursorCol < 0 {
		e.cursorCol = 0
	}
	e.ensureCursorVisible()
}

// vimSettle brings vim mode back to a consistent state after a change made
// outside its keys (undo, redo, formatting, the mouse): VISUAL ends when its
// selection is gone, a pending command is dropped and NORMAL keeps the cursor
// on a character. INSERT mode is left alone.
func (e *Editor) vimSettle() {
	if !e.vim || e.vimMode == vimInsert {
		return
	}
	if e.inVisual() && !e.selecting {
		e.vimMode = vimNormal
	}
	e.vimPending, e.vimCount, e.vimPreCount = "", 0, 0
	if e.vimMode == vimNormal {
		e.normalClamp()
	}
}

// vimMouseSelect turns a selection made with the mouse into a VISUAL one
// (V-LINE for whole lines) outside INSERT mode, so the operators act on it.
// A mouse selection ends before its last cell while VISUAL includes it: the
// later end moves back by one to keep the same text selected.
func (e *Editor) vimMouseSelect(mode vimMode) {
	if !e.vim || e.vimMode == vimInsert {
		return
	}
	if !e.selecting {
		e.vimSettle()
		return
	}
	e.vimMode = mode
	e.vimPending, e.vimCount, e.vimPreCount = "", 0, 0
	if mode == vimVisualLine {
		return
	}
	switch {
	case e.cursorLine > e.anchorLine || e.cursorLine == e.anchorLine && e.cursorCol > e.anchorCol:
		if e.cursorCol > 0 {
			e.cursorCol--
		}
	case e.cursorLine < e.anchorLine || e.cursorCol < e.anchorCol:
		if e.anchorCol > 0 {
			e.anchorCol--
		}
	}
}

// leaveVisual ends VISUAL or V-LINE mode and returns the range it covered,
// upper bound excluded, and whether it was V-LINE.
func (e *Editor) leaveVisual() (sl, sc, el, ec int, lines bool) {
	sl, sc, el, ec = e.selectedRange()
	lines = e.vimMode == vimVisualLine
	e.vimMode = vimNormal
	e.clearSelection()
	return sl, sc, el, ec, lines
}

// takeCount clears the typed counts and returns them: count is 0 when none
// was typed, n is count or 1. A count typed before an operator multiplies the
// one typed after it, so 2d3w deletes six words.
func (e *Editor) takeCount() (count, n int) {
	count = e.vimCount
	if e.vimPreCount > 0 {
		count = min(e.vimPreCount*max(count, 1), vimMaxCount)
	}
	e.vimCount, e.vimPreCount = 0, 0
	return count, max(count, 1)
}

// --- Keys ---

// vimUpdate handles a key in vim mode and records the change in the history.
func (e Editor) vimUpdate(key tea.KeyMsg) Editor {
	e.unfreeze()
	e.vimMsg = ""
	s := key.String()

	switch s {
	case "ctrl+c", "ctrl+s", "f2", "ctrl+q", "ctrl+t":
		m, _ := e.update(key)
		return m.(Editor)
	}
	if e.vimMode == vimInsert {
		return e.vimInsertKey(key)
	}
	if e.vimCmdOn {
		return e.vimCmdKey(key)
	}
	// u and ctrl+r move in the history themselves, outside the commit below.
	if e.vimMode == vimNormal && e.vimPending == "" && (s == "u" || s == "ctrl+r") {
		return e.vimHistory(s == "u")
	}

	before := e.snapshot()
	if key.Paste {
		e = e.vimPaste(key)
	} else {
		e = e.vimKey(key)
	}
	kind := ""
	switch e.vimMode {
	case vimNormal:
		e.normalClamp()
	case vimInsert:
		kind = vimInsertEdit
	}
	return e.commit(before, kind)
}

// vimInsertKey handles a key of INSERT mode: esc goes back to NORMAL mode,
// every other key edits as it does without vim mode.
func (e Editor) vimInsertKey(key tea.KeyMsg) Editor {
	if key.String() == "esc" {
		e.vimMode = vimNormal
		e.clearSelection()
		if e.cursorCol > 0 {
			e.cursorCol--
		}
		e.normalClamp()
		e.lastEdit = ""
		return e
	}
	before := e.snapshot()
	m, _ := e.update(key)
	return m.(Editor).commit(before, vimInsertEdit)
}

// vimHistory undoes (u) or redoes (ctrl+r) as many changes as the count.
func (e Editor) vimHistory(undo bool) Editor {
	_, n := e.takeCount()
	for ; n > 0; n-- {
		switch {
		case undo && len(e.undo) == 0:
			e.vimMsg = "already at oldest change"
			return e
		case !undo && len(e.redo) == 0:
			e.vimMsg = "already at newest change"
			return e
		case undo:
			e = e.Undo()
		default:
			e = e.Redo()
		}
	}
	return e
}

// vimPaste inserts text pasted from the terminal at the cursor, replacing a
// VISUAL selection: a paste is text, never a sequence of commands.
func (e Editor) vimPaste(key tea.KeyMsg) Editor {
	e.vimPending, e.vimCount, e.vimPreCount = "", 0, 0
	if e.inVisual() {
		sl, sc, el, ec, _ := e.leaveVisual()
		e.takeRange(sl, sc, el, ec, true)
	}
	e = e.insertKeyText(key)
	if e.cursorCol > 0 {
		e.cursorCol--
	}
	return e
}

// vimKey handles a key in NORMAL, VISUAL and V-LINE mode.
func (e Editor) vimKey(key tea.KeyMsg) Editor {
	if key.Alt {
		return e
	}
	s := key.String()

	// r waits for the character that replaces the one under the cursor.
	if e.vimPending == "r" {
		e.vimPending = ""
		_, n := e.takeCount()
		if len(key.Runes) == 1 && key.Runes[0] >= ' ' {
			e.vimReplace(key.Runes[0], n)
		}
		return e
	}

	if s == "esc" {
		e.vimPending, e.vimCount, e.vimPreCount = "", 0, 0
		if e.vimMode != vimNormal {
			e.vimMode = vimNormal
			e.clearSelection()
		}
		return e
	}

	// A count: 1-9 start it, 0 continues it (alone, 0 is a motion).
	if len(s) == 1 && (s[0] >= '1' && s[0] <= '9' || s[0] == '0' && e.vimCount > 0) {
		e.vimCount = min(e.vimCount*10+int(s[0]-'0'), vimMaxCount)
		return e
	}

	if e.inVisual() {
		return e.vimVisualKey(s)
	}
	if e.vimPending != "" {
		return e.vimOperator(s)
	}
	return e.vimNormalKey(s)
}

// vimNormalKey handles a key of NORMAL mode with no operator pending.
func (e Editor) vimNormalKey(s string) Editor {
	count, n := e.takeCount()
	if line, col, _, _, ok := e.vimMotion(s, count); ok {
		e.cursorLine, e.cursorCol = line, col
		e.ensureCursorVisible()
		return e
	}
	line := e.currentLine()

	switch s {
	case "i":
		e.vimMode = vimInsert
	case "a":
		if runeLen(line) > 0 {
			e.cursorCol++
		}
		e.vimMode = vimInsert
	case "I":
		e.cursorCol = firstNonBlank(line)
		e.vimMode = vimInsert
	case "A":
		e.cursorCol = runeLen(line)
		e.vimMode = vimInsert
	case "o":
		e.vimOpen(true)
	case "O":
		e.vimOpen(false)
	case "x", "delete":
		e.vimChars("d", e.cursorLine, e.cursorCol, e.cursorLine, e.cursorCol+n)
	case "X":
		e.vimChars("d", e.cursorLine, max(e.cursorCol-n, 0), e.cursorLine, e.cursorCol)
	case "s":
		e.vimChars("c", e.cursorLine, e.cursorCol, e.cursorLine, e.cursorCol+n)
	case "D":
		e.vimChars("d", e.cursorLine, e.cursorCol, e.cursorLine, runeLen(line))
	case "C":
		e.vimChars("c", e.cursorLine, e.cursorCol, e.cursorLine, runeLen(line))
	case "S":
		e.vimLines("c", e.cursorLine, clampLine(e.cursorLine+n-1, len(e.lines)))
	case "Y":
		e.vimLines("y", e.cursorLine, clampLine(e.cursorLine+n-1, len(e.lines)))
	case "p":
		e.vimPut(true, n)
	case "P":
		e.vimPut(false, n)
	case "J":
		// J and 2J join two lines, 3J three.
		e.vimJoin(max(n-1, 1))
	case "d", "c", "y", "g", "r":
		// The command waits for its next key; the count stays for it.
		e.vimPending = s
		e.vimPreCount = count
	case "v":
		e.vimMode = vimVisual
		e.beginSelection()
	case "V":
		e.vimMode = vimVisualLine
		e.beginSelection()
	case ":":
		e.vimCmdOn, e.vimCmd = true, ""
	}
	return e
}

// vimOperator handles the key that follows a pending operator (d c y), a
// prefix (g) or an operator with a prefix (dg, di, da ...).
func (e Editor) vimOperator(s string) Editor {
	op := e.vimPending
	e.vimPending = ""
	count, n := e.takeCount()

	if op == "g" {
		if s == "g" {
			e.cursorLine, e.cursorCol, _, _, _ = e.vimMotion("gg", count)
		}
		return e
	}

	verb := op[:1]
	if len(op) == 2 {
		switch {
		case op[1] == 'g' && s == "g":
			line, _, _, _, _ := e.vimMotion("gg", count)
			e.vimApply(verb, line, 0, true, false)
		case (op[1] == 'i' || op[1] == 'a') && s == "w":
			if from, to, ok := e.wordObject(op[1] == 'a'); ok {
				e.vimChars(verb, e.cursorLine, from, e.cursorLine, to)
			}
		}
		return e
	}

	switch s {
	case verb:
		// dd, cc, yy: n whole lines from the cursor's.
		e.vimLines(verb, e.cursorLine, clampLine(e.cursorLine+n-1, len(e.lines)))
	case "g", "i", "a":
		e.vimPending = op + s
		e.vimPreCount = count
	default:
		if line, col, linewise, inclusive, ok := e.vimTarget(verb, s, count); ok {
			e.vimApply(verb, line, col, linewise, inclusive)
		}
	}
	return e
}

// vimVisualKey handles a key of VISUAL and V-LINE mode.
func (e Editor) vimVisualKey(s string) Editor {
	count, _ := e.takeCount()
	if e.vimPending == "g" {
		e.vimPending = ""
		if s == "g" {
			e.cursorLine, e.cursorCol, _, _, _ = e.vimMotion("gg", count)
			e.normalClamp()
		}
		return e
	}
	if line, col, _, _, ok := e.vimMotion(s, count); ok {
		e.cursorLine, e.cursorCol = line, col
		e.normalClamp()
		return e
	}

	switch s {
	case "g":
		e.vimPending = "g"
		e.vimPreCount = count
	case "v", "V":
		want := vimVisual
		if s == "V" {
			want = vimVisualLine
		}
		if e.vimMode == want {
			e.vimMode = vimNormal
			e.clearSelection()
		} else {
			e.vimMode = want
		}
	case "o":
		e.anchorLine, e.cursorLine = e.cursorLine, e.anchorLine
		e.anchorCol, e.cursorCol = e.cursorCol, e.anchorCol
		e.ensureCursorVisible()
	case "d", "x", "y", "c":
		verb := s
		if s == "x" {
			verb = "d"
		}
		sl, sc, el, ec, lines := e.leaveVisual()
		if lines {
			e.vimLines(verb, sl, el)
		} else {
			e.vimChars(verb, sl, sc, el, ec)
		}
	}
	return e
}

// vimCmdKey handles a key of the ":" command line.
func (e Editor) vimCmdKey(key tea.KeyMsg) Editor {
	switch key.String() {
	case "esc":
		e.vimCmdOn, e.vimCmd = false, ""
	case "enter":
		cmd := strings.TrimSpace(e.vimCmd)
		e.vimCmdOn, e.vimCmd = false, ""
		e = e.vimExec(cmd)
	case "backspace":
		if e.vimCmd == "" {
			e.vimCmdOn = false
		} else {
			r := []rune(e.vimCmd)
			e.vimCmd = string(r[:len(r)-1])
		}
	default:
		if !key.Alt {
			for _, r := range key.Runes {
				if r >= ' ' {
					e.vimCmd += string(r)
				}
			}
		}
	}
	return e
}

// vimExec runs a command of the ":" command line. Saving and leaving are
// asked of the screen, as ctrl+s and ctrl+q do.
func (e Editor) vimExec(cmd string) Editor {
	switch cmd {
	case "":
	case "w":
		e.WantsSave = true
	case "q":
		e.WantsExit = true
	case "q!":
		e.WantsQuit = true
	case "wq", "x":
		e.WantsSave, e.WantsExit = true, true
	case "$":
		e.vimGoLine(len(e.lines) - 1)
	default:
		if n, err := strconv.Atoi(cmd); err == nil && cmd[0] >= '0' && cmd[0] <= '9' {
			e.vimGoLine(n - 1)
		} else {
			e.vimMsg = "unknown command :" + cmd
		}
	}
	return e
}

// vimGoLine brings the cursor to the first character of line (0-based).
func (e *Editor) vimGoLine(line int) {
	e.cursorLine = clampLine(line, len(e.lines))
	e.cursorCol = firstNonBlank(e.currentLine())
	e.normalClamp()
}

// --- Motions ---

// vimMotion computes where the motion of key s brings the cursor; count is
// the typed count, 0 when none. linewise says the motion works on whole lines
// (j k G gg ...) and inclusive that an operator also takes the character it
// lands on (e $). ok is false when s is not a motion, or when j or k cannot
// move: an operator then does nothing, as in vim.
func (e Editor) vimMotion(s string, count int) (line, col int, linewise, inclusive, ok bool) {
	n := max(count, 1)
	line, col = e.cursorLine, e.cursorCol
	half := max(e.height/2, 1)

	switch s {
	case "h", "left":
		col = max(col-n, 0)
	case "l", "right":
		col = min(col+n, runeLen(e.currentLine()))
	case "j", "down", "k", "up":
		if s == "k" || s == "up" {
			n = -n
		}
		line, linewise = clampLine(line+n, len(e.lines)), true
		if line == e.cursorLine {
			return 0, 0, false, false, false
		}
	case "ctrl+d":
		line, linewise = line+half*n, true
	case "ctrl+u":
		line, linewise = line-half*n, true
	case "ctrl+f", "pgdown":
		line, linewise = line+e.height*n, true
	case "ctrl+b", "pgup":
		line, linewise = line-e.height*n, true
	case "0", "home":
		col = 0
	case "^":
		col = firstNonBlank(e.currentLine())
	case "$", "end":
		line = clampLine(line+n-1, len(e.lines))
		col, inclusive = max(runeLen(e.lines[line])-1, 0), true
	case "w":
		for i := 0; i < n; i++ {
			line, col = e.wordForward(line, col)
		}
	case "b":
		for i := 0; i < n; i++ {
			line, col = e.wordBack(line, col)
		}
	case "e":
		for i := 0; i < n; i++ {
			line, col = e.wordEnd(line, col)
		}
		inclusive = true
	case "G", "gg":
		line, linewise = len(e.lines)-1, true
		if s == "gg" {
			line = 0
		}
		if count > 0 {
			line = count - 1
		}
		line = clampLine(line, len(e.lines))
		col = firstNonBlank(e.lines[line])
	default:
		return 0, 0, false, false, false
	}
	return clampLine(line, len(e.lines)), col, linewise, inclusive, true
}

// vimTarget is where the motion s takes the operator verb. On top of
// vimMotion it has vim's two special cases for w: cw changes up to the end of
// the word, and the last word of a line ends at the end of that line, so dw
// there does not join the next line.
func (e Editor) vimTarget(verb, s string, count int) (line, col int, linewise, inclusive, ok bool) {
	if s != "w" {
		return e.vimMotion(s, count)
	}
	n := max(count, 1)
	line, col = e.cursorLine, e.cursorCol
	if verb == "c" {
		col = e.changeWordEnd()
		for i := 1; i < n; i++ {
			line, col = e.wordEnd(line, col)
		}
		return line, col, false, true, true
	}
	for i := 0; i < n; i++ {
		nextLine, nextCol := e.wordForward(line, col)
		if i == n-1 && nextLine > line {
			nextLine, nextCol = line, runeLen(e.lines[line])
		}
		line, col = nextLine, nextCol
	}
	return line, col, false, false, true
}

// vimClass sorts runes for word motions: 0 blank, 1 word, 2 punctuation.
func vimClass(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case isSelectWordRune(r):
		return 1
	}
	return 2
}

// wordForward is the start of the next word (w), across lines; an empty
// line counts as a word.
func (e Editor) wordForward(line, col int) (int, int) {
	rs := []rune(e.LineAt(line))
	if col < len(rs) {
		if cls := vimClass(rs[col]); cls != 0 {
			for col < len(rs) && vimClass(rs[col]) == cls {
				col++
			}
		}
	}
	for {
		for col < len(rs) && vimClass(rs[col]) == 0 {
			col++
		}
		if col < len(rs) {
			return line, col
		}
		if line >= len(e.lines)-1 {
			return line, len(rs)
		}
		line, col = line+1, 0
		rs = []rune(e.lines[line])
		if len(rs) == 0 {
			return line, 0
		}
	}
}

// wordBack is the start of the current or previous word (b), across lines.
func (e Editor) wordBack(line, col int) (int, int) {
	rs := []rune(e.LineAt(line))
	col = min(col, len(rs))
	for {
		col--
		if col < 0 {
			if line == 0 {
				return 0, 0
			}
			line--
			rs = []rune(e.lines[line])
			col = len(rs)
			if col == 0 {
				return line, 0
			}
			continue
		}
		if vimClass(rs[col]) != 0 {
			break
		}
	}
	cls := vimClass(rs[col])
	for col > 0 && vimClass(rs[col-1]) == cls {
		col--
	}
	return line, col
}

// wordEnd is the last character of the current or next word (e), across
// lines.
func (e Editor) wordEnd(line, col int) (int, int) {
	rs := []rune(e.LineAt(line))
	for {
		col++
		if col >= len(rs) {
			if line >= len(e.lines)-1 {
				return line, max(len(rs)-1, 0)
			}
			line, col = line+1, -1
			rs = []rune(e.lines[line])
			continue
		}
		if vimClass(rs[col]) != 0 {
			break
		}
	}
	cls := vimClass(rs[col])
	for col+1 < len(rs) && vimClass(rs[col+1]) == cls {
		col++
	}
	return line, col
}

// changeWordEnd is where cw stops on the cursor's line: the last character of
// the word (or run of blanks) under the cursor.
func (e Editor) changeWordEnd() int {
	rs := []rune(e.currentLine())
	col := e.cursorCol
	if col >= len(rs) {
		return col
	}
	cls := vimClass(rs[col])
	for col+1 < len(rs) && vimClass(rs[col+1]) == cls {
		col++
	}
	return col
}

// wordObject is the range [from, to) of the word under the cursor: iw is the
// run of runes of the same kind, aw also takes the blanks after it (or, when
// there are none, those before it).
func (e Editor) wordObject(around bool) (from, to int, ok bool) {
	rs := []rune(e.currentLine())
	if len(rs) == 0 {
		return 0, 0, false
	}
	c := min(e.cursorCol, len(rs)-1)
	cls := vimClass(rs[c])
	from, to = c, c+1
	for from > 0 && vimClass(rs[from-1]) == cls {
		from--
	}
	for to < len(rs) && vimClass(rs[to]) == cls {
		to++
	}
	if around && cls != 0 {
		end := to
		for end < len(rs) && vimClass(rs[end]) == 0 {
			end++
		}
		if end > to {
			to = end
		} else {
			for from > 0 && vimClass(rs[from-1]) == 0 {
				from--
			}
		}
	}
	return from, to, true
}

// firstNonBlank is the column of the first rune of s that is not blank.
func firstNonBlank(s string) int {
	for i, r := range []rune(s) {
		if !unicode.IsSpace(r) {
			return i
		}
	}
	return 0
}

// --- Operators ---

// vimApply runs the operator verb (d c y) from the cursor to (line, col).
func (e *Editor) vimApply(verb string, line, col int, linewise, inclusive bool) {
	if linewise {
		e.vimLines(verb, min(e.cursorLine, line), max(e.cursorLine, line))
		return
	}
	sl, sc, el, ec := e.cursorLine, e.cursorCol, line, col
	if el < sl || el == sl && ec < sc {
		sl, sc, el, ec = el, ec, sl, sc
	}
	if inclusive {
		ec++
	}
	e.vimChars(verb, sl, sc, el, ec)
}

// takeRange returns the text from (sl, sc) to (el, ec), upper bound excluded,
// and deletes it when del is true; the cursor ends at the start of the range.
// It reuses the selection code, so it must run outside VISUAL mode.
func (e *Editor) takeRange(sl, sc, el, ec int, del bool) string {
	e.anchorLine, e.anchorCol = sl, sc
	e.cursorLine, e.cursorCol = el, ec
	e.selecting = true
	text, _ := e.SelectedText()
	if del && text != "" {
		e.deleteSelection()
	} else {
		e.clearSelection()
		e.cursorLine, e.cursorCol = sl, sc
	}
	return text
}

// vimChars runs the operator verb on the characters from (sl, sc) to
// (el, ec), upper bound excluded: the text goes to the register, d deletes
// it, c deletes it and enters INSERT mode, y only copies it.
func (e *Editor) vimChars(verb string, sl, sc, el, ec int) {
	if text := e.takeRange(sl, sc, el, ec, verb != "y"); text != "" {
		e.vimReg, e.vimRegLines = text, false
	}
	if verb == "c" {
		e.vimMode = vimInsert
	}
	e.ensureCursorVisible()
}

// vimLines runs the operator verb on the whole lines from a to b: the lines
// go to the register, d removes them, c empties them into one line and
// enters INSERT mode, y only copies them.
func (e *Editor) vimLines(verb string, a, b int) {
	e.vimReg = strings.Join(e.lines[a:b+1], "\n")
	e.vimRegLines = true
	switch verb {
	case "y":
		e.cursorLine = a
	case "d":
		out := append(append([]string(nil), e.lines[:a]...), e.lines[b+1:]...)
		if len(out) == 0 {
			out = []string{""}
		}
		e.lines = out
		e.cursorLine = clampLine(a, len(out))
		e.cursorCol = firstNonBlank(e.currentLine())
		e.Dirty = true
	case "c":
		out := append(append([]string(nil), e.lines[:a]...), "")
		e.lines = append(out, e.lines[b+1:]...)
		e.cursorLine, e.cursorCol = a, 0
		e.vimMode = vimInsert
		e.Dirty = true
	}
	e.ensureCursorVisible()
}

// vimPut pastes the register n times: whole lines below (p) or above (P)
// the cursor's line, characters after (p) or at (P) the cursor. The cursor
// ends on the first pasted line, or on the last pasted character.
func (e *Editor) vimPut(after bool, n int) {
	if e.vimReg == "" && !e.vimRegLines {
		e.vimMsg = "nothing to paste"
		return
	}
	if e.vimRegLines {
		block := strings.Split(e.vimReg, "\n")
		at := e.cursorLine
		if after {
			at++
		}
		out := append([]string(nil), e.lines[:at]...)
		for i := 0; i < n; i++ {
			out = append(out, block...)
		}
		e.lines = append(out, e.lines[at:]...)
		e.cursorLine = at
		e.cursorCol = firstNonBlank(e.currentLine())
	} else {
		if after && runeLen(e.currentLine()) > 0 {
			e.cursorCol++
		}
		for _, r := range strings.Repeat(e.vimReg, n) {
			if r == '\n' {
				e.newline()
			} else {
				e.insertRune(r)
			}
		}
		if e.cursorCol > 0 {
			e.cursorCol--
		}
	}
	e.Dirty = true
	e.ensureCursorVisible()
}

// vimOpen opens an empty line below (o) or above (O) the cursor's line and
// enters INSERT mode on it.
func (e *Editor) vimOpen(below bool) {
	at := e.cursorLine
	if below {
		at++
	}
	out := append(append([]string(nil), e.lines[:at]...), "")
	e.lines = append(out, e.lines[at:]...)
	e.cursorLine, e.cursorCol = at, 0
	e.vimMode = vimInsert
	e.Dirty = true
	e.ensureCursorVisible()
}

// vimJoin joins the cursor's line with the next one, times times, with one
// space between them (J).
func (e *Editor) vimJoin(times int) {
	for i := 0; i < times && e.cursorLine < len(e.lines)-1; i++ {
		cur := strings.TrimRight(e.lines[e.cursorLine], " \t")
		next := strings.TrimLeft(e.lines[e.cursorLine+1], " \t")
		joined := cur
		if cur != "" && next != "" {
			joined += " "
		}
		e.replaceLines(e.cursorLine, e.cursorLine+1, joined+next)
		e.cursorCol = runeLen(cur)
		e.Dirty = true
	}
	e.ensureCursorVisible()
}

// vimReplace replaces n characters from the cursor with r (r); it does
// nothing when the line has fewer than n characters left.
func (e *Editor) vimReplace(r rune, n int) {
	rs := []rune(e.currentLine())
	if e.cursorCol+n > len(rs) {
		return
	}
	for i := 0; i < n; i++ {
		rs[e.cursorCol+i] = r
	}
	e.lines[e.cursorLine] = string(rs)
	e.cursorCol += n - 1
	e.Dirty = true
}

// --- Footer ---

// vimFooter is the footer in vim mode: the ":" command line while it is open,
// otherwise the mode followed by a message, the command being typed or the
// hints of the mode.
func (e Editor) vimFooter() string {
	var out string
	if e.vimCmdOn {
		out = theme.Selected.Render(truncate(":"+e.vimCmd, e.width-1)) + theme.Rev.Render(" ")
	} else {
		head := truncate("-- "+vimModeNames[e.vimMode]+" --", e.width)
		out = theme.BannerAcc.Render(head)
		if room := e.width - ansi.StringWidth(head) - 2; room > 0 {
			tail, style := vimHints[e.vimMode], theme.DimText
			switch {
			case e.vimMsg != "":
				tail, style = e.vimMsg, theme.Error
			case e.vimPending != "" || e.vimCount > 0:
				tail = vimCountText(e.vimPreCount) + e.vimPending + vimCountText(e.vimCount)
			}
			out += "  " + style.Render(truncate(tail, room))
		}
	}
	if pad := e.width - ansi.StringWidth(out); pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	return out
}

// vimCountText is a count as the footer shows it: nothing when there is none.
func vimCountText(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}
