package editor

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	ansi "github.com/charmbracelet/x/ansi"
)

// vimNamedKeys are the keys vimKeys accepts between angle brackets.
var vimNamedKeys = map[string]tea.KeyType{
	"esc": tea.KeyEsc, "cr": tea.KeyEnter, "bs": tea.KeyBackspace, "del": tea.KeyDelete,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"home": tea.KeyHome, "end": tea.KeyEnd,
	"C-r": tea.KeyCtrlR, "C-d": tea.KeyCtrlD, "C-u": tea.KeyCtrlU, "C-s": tea.KeyCtrlS,
	"C-q": tea.KeyCtrlQ, "C-t": tea.KeyCtrlT, "C-y": tea.KeyCtrlY,
}

// vimKeys turns a key sequence into KeyMsgs: one per rune, or a named key
// between angle brackets ("dw", "ihi<esc>", "<C-r>").
func vimKeys(s string) []tea.KeyMsg {
	var out []tea.KeyMsg
	for s != "" {
		if s[0] == '<' {
			if end := strings.IndexByte(s, '>'); end > 0 {
				if t, ok := vimNamedKeys[s[1:end]]; ok {
					out = append(out, tea.KeyMsg{Type: t})
					s = s[end+1:]
					continue
				}
			}
		}
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == ' ' {
			out = append(out, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		} else {
			out = append(out, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	return out
}

// vimRun opens an editor in vim mode on lines, with the cursor on the first
// character, and types keys.
func vimRun(lines []string, keys string) Editor {
	e := New(lines, 40, 10).WithVim().SetCursor(0, 0)
	return press(e, vimKeys(keys)...)
}

// paste is a KeyMsg carrying text pasted from the terminal.
func paste(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true}
}

// text is the editor's text, lines joined by "\n".
func text(e Editor) string { return strings.Join(e.Lines(), "\n") }

// TestVimCommands checks the text left by the commands of every mode, alone
// and combined.
func TestVimCommands(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		keys  string
		want  string
	}{
		// Insert commands.
		{"i inserts before the cursor", []string{"abc"}, "lixy<esc>", "axybc"},
		{"a appends after the cursor", []string{"ac"}, "ab<esc>", "abc"},
		{"I inserts at the first character", []string{"  ab"}, "$Ix<esc>", "  xab"},
		{"A appends at the line end", []string{"ab"}, "Acd<esc>", "abcd"},
		{"o opens a line below", []string{"a", "b"}, "onew<esc>", "a\nnew\nb"},
		{"O opens a line above", []string{"a"}, "Onew<esc>", "new\na"},
		{"INSERT keeps the usual keys", []string{"abc"}, "A<bs>d<cr>e<esc>", "abd\ne"},
		{"INSERT ctrl+u deletes to the line start", []string{"abc def"}, "A<C-u><esc>", ""},

		// Single-key edits.
		{"x deletes the character", []string{"abc"}, "x", "bc"},
		{"3x deletes three characters", []string{"abcdef"}, "3x", "def"},
		{"x past the line end stops there", []string{"abc"}, "l5x", "a"},
		{"delete works as x", []string{"abc"}, "<del>", "bc"},
		{"X deletes before the cursor", []string{"abc"}, "$X", "ac"},
		{"2X deletes two before", []string{"abcd"}, "$2X", "ad"},
		{"X at the line start does nothing", []string{"abc"}, "X", "abc"},
		{"r replaces the character", []string{"abc"}, "lrX", "aXc"},
		{"3r replaces three", []string{"abcd"}, "3rx", "xxxd"},
		{"r past the line end does nothing", []string{"ab"}, "3rx", "ab"},
		{"r takes a space", []string{"ab"}, "r ", " b"},
		{"s substitutes the character", []string{"abc"}, "sX<esc>", "Xbc"},
		{"2s substitutes two", []string{"abc"}, "2sX<esc>", "Xc"},
		{"D deletes to the line end", []string{"abc def"}, "wD", "abc "},
		{"C changes to the line end", []string{"abc def"}, "wCxyz<esc>", "abc xyz"},
		{"S changes the line", []string{"a", "b"}, "Sz<esc>", "z\nb"},
		{"J joins with one space", []string{"a", "  b"}, "J", "a b"},
		{"3J joins three lines", []string{"a", "b", "c", "d"}, "3J", "a b c\nd"},
		{"J with an empty line adds no space", []string{"a", "", "b"}, "J", "a\nb"},
		{"J on the last line does nothing", []string{"a"}, "J", "a"},

		// Operators with motions.
		{"dw deletes a word", []string{"one two three"}, "dw", "two three"},
		{"d2w deletes two words", []string{"one two three"}, "d2w", "three"},
		{"2dw deletes two words", []string{"one two three"}, "2dw", "three"},
		{"2d2w multiplies the counts", []string{"a b c d e f"}, "2d2w", "e f"},
		{"dw on the last word stops at the line end", []string{"one", "two"}, "dw", "\ntwo"},
		{"d3w crosses lines", []string{"a b", "c d"}, "d3w", "d"},
		{"de deletes to the word end", []string{"one two"}, "de", " two"},
		{"db deletes back to the word start", []string{"one two"}, "$db", "one o"},
		{"d$ deletes to the line end", []string{"abc def"}, "ld$", "a"},
		{"d0 deletes to the line start", []string{"abc def"}, "wd0", "def"},
		{"d^ deletes to the first character", []string{"  abc"}, "$d^", "  c"},
		{"dl deletes the character", []string{"abc"}, "dl", "bc"},
		{"dh deletes the one before", []string{"abc"}, "ldh", "bc"},
		{"dj deletes two lines", []string{"a", "b", "c"}, "dj", "c"},
		{"dk deletes two lines", []string{"a", "b", "c"}, "jdk", "c"},
		{"dk on the first line does nothing", []string{"a", "b"}, "dk", "a\nb"},
		{"dj on the last line does nothing", []string{"a", "b"}, "jdj", "a\nb"},
		{"dG deletes to the last line", []string{"a", "b", "c"}, "jdG", "a"},
		{"dgg deletes to the first line", []string{"a", "b", "c"}, "jdgg", "c"},
		{"cw changes to the word end", []string{"one two"}, "cwuno<esc>", "uno two"},
		{"c2w changes two words", []string{"one two three"}, "c2wx<esc>", "x three"},
		{"cw on the last character of a word", []string{"ab cd"}, "lcwX<esc>", "aX cd"},
		{"ce changes to the word end", []string{"one two"}, "cex<esc>", "x two"},
		{"c$ changes to the line end", []string{"abc def"}, "wc$x<esc>", "abc x"},
		{"yw copies a word", []string{"one two"}, "ywP", "one one two"},
		{"y$ copies to the line end", []string{"abc"}, "y$p", "aabcbc"},
		{"yj copies two lines", []string{"a", "b", "c"}, "yjp", "a\na\nb\nb\nc"},

		// Text objects.
		{"diw deletes the word", []string{"one two three"}, "wdiw", "one  three"},
		{"daw deletes the word and the blank after", []string{"one two three"}, "wdaw", "one three"},
		{"daw on the last word takes the blank before", []string{"one two"}, "wdaw", "one"},
		{"diw on blanks deletes them", []string{"a   b"}, "ldiw", "ab"},
		{"ciw changes the word", []string{"one two"}, "wciwdos<esc>", "one dos"},
		{"yiw copies the word", []string{"ab cd"}, "yiwwP", "ab abcd"},

		// Whole lines.
		{"dd deletes the line", []string{"a", "b", "c"}, "jdd", "a\nc"},
		{"2dd deletes two lines", []string{"a", "b", "c"}, "2dd", "c"},
		{"dd past the end stops at the last line", []string{"a", "b", "c"}, "j5dd", "a"},
		{"dd on the only line empties it", []string{"a"}, "dd", ""},
		{"cc changes the line", []string{"a", "b"}, "jccz<esc>", "a\nz"},
		{"yyp duplicates the line", []string{"a", "b"}, "yyp", "a\na\nb"},
		{"yyP pastes above", []string{"a", "b"}, "jyyP", "a\nb\nb"},
		{"Yp duplicates the line", []string{"a", "b"}, "Yp", "a\na\nb"},
		{"yy3p pastes three times", []string{"a"}, "yy3p", "a\na\na\na"},
		{"ddp swaps lines", []string{"a", "b"}, "ddp", "b\na"},

		// Characters.
		{"xp swaps characters", []string{"ab"}, "xp", "ba"},
		{"P pastes at the cursor", []string{"abc"}, "yl$P", "abac"},
		{"p of several lines of characters", []string{"ab", "cd"}, "lvjyGp", "ab\ncb\ncdd"},
		{"p with nothing copied does nothing", []string{"ab"}, "p", "ab"},

		// VISUAL and V-LINE.
		{"v d deletes the selection", []string{"abcdef"}, "lvlld", "aef"},
		{"v x deletes the selection", []string{"abcdef"}, "lvlx", "adef"},
		{"v c changes the selection", []string{"abc"}, "vlcX<esc>", "Xc"},
		{"v y copies the selection", []string{"abc"}, "vly$p", "abcab"},
		{"v across lines", []string{"abc", "def"}, "lvjd", "af"},
		{"v with a count", []string{"abcdef"}, "v3ld", "ef"},
		{"o jumps to the other end", []string{"abcdef"}, "lvllohd", "ef"},
		{"v v leaves VISUAL", []string{"abc"}, "vvx", "bc"},
		{"V d deletes lines", []string{"a", "b", "c"}, "Vjd", "c"},
		{"V y copies lines", []string{"a", "b"}, "Vyp", "a\na\nb"},
		{"V c changes lines", []string{"a", "b", "c"}, "Vjcx<esc>", "x\nc"},
		{"v V switches to lines", []string{"ab", "cd"}, "lvVd", "cd"},
		{"V gg selects to the top", []string{"a", "b", "c"}, "GVggd", ""},

		// History.
		{"u undoes", []string{"abc"}, "xxu", "bc"},
		{"3u undoes three changes", []string{"abcd"}, "xxx3u", "abcd"},
		{"ctrl+r redoes", []string{"abc"}, "xu<C-r>", "bc"},
		{"ctrl+y redoes", []string{"abc"}, "xu<C-y>", "bc"},
		{"an insert is one undo step", []string{"a"}, "ohello<esc>u", "a"},
		{"cw and its text are one step", []string{"one two"}, "cwuno<esc>u", "one two"},
		{"an insert across lines is one step", []string{"a"}, "Ab<cr>c<esc>u", "a"},
		{"two inserts are two steps", []string{"a"}, "Ab<esc>Ac<esc>u", "ab"},
		{"u in VISUAL does nothing", []string{"abc"}, "xvu<esc>", "bc"},

		// Cancelling and unknown keys.
		{"esc in NORMAL does nothing", []string{"abc"}, "<esc>", "abc"},
		{"esc cancels an operator", []string{"abc"}, "d<esc>x", "bc"},
		{"esc cancels a count", []string{"abcd"}, "3<esc>x", "bcd"},
		{"an operator with a non-motion does nothing", []string{"abc"}, "dzx", "bc"},
		{"unknown keys do nothing", []string{"abc"}, "QZ", "abc"},

		// Empty notes and lines.
		{"dd on an empty note", []string{""}, "dd", ""},
		{"x on an empty line", []string{""}, "x", ""},
		{"D on an empty line", []string{""}, "D", ""},
		{"diw on an empty line", []string{""}, "diw", ""},
		{"v d on an empty line", []string{""}, "vd", ""},
		{"V d on an empty note", []string{""}, "Vd", ""},
		{"cw on an empty line", []string{""}, "cwx<esc>", "x"},
		{"o on an empty note", []string{""}, "o<esc>", "\n"},
		{"yyp of an empty line", []string{""}, "yyp", "\n"},
		{"motions on an empty note", []string{""}, "$wbeGgg0^jkhl", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := vimRun(tt.lines, tt.keys)
			if got := text(e); got != tt.want {
				t.Fatalf("keys %q on %q = %q, want %q", tt.keys, tt.lines, got, tt.want)
			}
		})
	}
}

// TestVimCursor checks where the commands leave the cursor.
func TestVimCursor(t *testing.T) {
	tests := []struct {
		name      string
		lines     []string
		keys      string
		line, col int
	}{
		{"x at the line end stays on a character", []string{"abc"}, "$x", 0, 1},
		{"esc from INSERT steps back", []string{"abc"}, "ixy<esc>", 0, 1},
		{"esc after A is on the last character", []string{"abc"}, "A<esc>", 0, 2},
		{"esc at the line start stays", []string{"abc"}, "i<esc>", 0, 0},
		{"p of lines goes to the first pasted one", []string{"a", "  b"}, "jyykp", 1, 2},
		{"P of characters ends on the last pasted", []string{"abc"}, "yl$P", 0, 2},
		{"dd goes to the first character below", []string{"a", "  b"}, "dd", 0, 2},
		{"dd on the last line goes up", []string{"a", "b"}, "jdd", 0, 0},
		{"J leaves the cursor at the join", []string{"ab", "cd"}, "J", 0, 2},
		{"yy keeps the cursor", []string{"abc"}, "lyy", 0, 1},
		{"yk moves up", []string{"a", "b"}, "jyk", 0, 0},
		{"y in VISUAL goes to the start", []string{"abcd"}, "$vhhy", 0, 1},
		{"u puts the cursor back", []string{"abc"}, "$xu", 0, 2},
		{"j keeps the column", []string{"abc", "abc"}, "llj", 1, 2},
		{"j clamps the column", []string{"abc", "a"}, "llj", 1, 0},
		{"G with a count past the end", []string{"a", "b"}, "9G", 1, 0},
		{"10j stops at the last line", []string{"a", "b", "c", "d"}, "10j", 3, 0},
		{"10l counts with a zero", []string{"abcdefghijklmn"}, "10l", 0, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := vimRun(tt.lines, tt.keys)
			if e.cursorLine != tt.line || e.cursorCol != tt.col {
				t.Fatalf("keys %q: cursor = (%d, %d), want (%d, %d)",
					tt.keys, e.cursorLine, e.cursorCol, tt.line, tt.col)
			}
		})
	}
}

// TestVimMotions checks where the motions bring the cursor.
func TestVimMotions(t *testing.T) {
	lines := []string{"one two.three", "", "  four"}
	tests := []struct {
		keys      string
		line, col int
	}{
		{"w", 0, 4},
		{"ww", 0, 7},
		{"www", 0, 8},
		{"wwww", 1, 0},
		{"5w", 2, 2},
		{"e", 0, 2},
		{"ee", 0, 6},
		{"$b", 0, 8},
		{"$bb", 0, 7},
		{"Gb", 1, 0},
		{"$", 0, 12},
		{"<end>", 0, 12},
		{"$0", 0, 0},
		{"$<home>", 0, 0},
		{"G", 2, 2},
		{"G0", 2, 0},
		{"G0^", 2, 2},
		{"Ggg", 0, 0},
		{"3G", 2, 2},
		{"G2gg", 1, 0},
		{"2j", 2, 0},
		{"<down><down>", 2, 0},
		{"G<up>", 1, 0},
		{"l", 0, 1},
		{"<right>", 0, 1},
		{"l<left>", 0, 0},
		{"3l2h", 0, 1},
		{"h", 0, 0},
		{"b", 0, 0},
		{"$l", 0, 12},
		{"Gw", 2, 5},
		{"Gee", 2, 5},
		{"<C-d>", 2, 0},
		{"<C-d><C-u>", 0, 0},
	}
	for _, tt := range tests {
		e := vimRun(lines, tt.keys)
		if e.cursorLine != tt.line || e.cursorCol != tt.col {
			t.Errorf("keys %q: cursor = (%d, %d), want (%d, %d)",
				tt.keys, e.cursorLine, e.cursorCol, tt.line, tt.col)
		}
	}
}

// TestVimCountIsCapped checks that a huge count neither overflows nor hangs.
func TestVimCountIsCapped(t *testing.T) {
	e := vimRun([]string{"abc"}, "9999999999")
	if e.vimCount != vimMaxCount {
		t.Fatalf("vimCount = %d, want %d", e.vimCount, vimMaxCount)
	}
	e = press(e, vimKeys("x")...)
	if got := text(e); got != "" {
		t.Fatalf("text = %q, want %q", got, "")
	}
	e = vimRun([]string{"a b"}, "99999w9999d9999w")
	if e.vimCount != 0 || e.vimPending != "" {
		t.Fatalf("count %d, pending %q after the command, want none", e.vimCount, e.vimPending)
	}
}

// TestVimMessages checks the footer messages of commands that cannot run.
func TestVimMessages(t *testing.T) {
	tests := []struct {
		keys, want string
	}{
		{"u", "already at oldest change"},
		{"<C-r>", "already at newest change"},
		{"p", "nothing to paste"},
		{":nope<cr>", "unknown command :nope"},
		{":+1<cr>", "unknown command :+1"},
	}
	for _, tt := range tests {
		e := vimRun([]string{"abc"}, tt.keys)
		if !strings.Contains(e.Footer(), tt.want) {
			t.Errorf("keys %q: footer = %q, want it to contain %q", tt.keys, e.Footer(), tt.want)
		}
		if e = press(e, vimKeys("l")...); strings.Contains(e.Footer(), tt.want) {
			t.Errorf("keys %q: the message stays after the next key", tt.keys)
		}
	}
}

// TestVimVisualSelection checks that VISUAL selects the character under the
// cursor and V-LINE whole lines, for the highlight and for copying.
func TestVimVisualSelection(t *testing.T) {
	e := vimRun([]string{"abcdef"}, "lvl")
	if from, to, ok := e.selectionOnLine(0); !ok || from != 1 || to != 3 {
		t.Errorf("highlight = [%d, %d) %v, want [1, 3) true", from, to, ok)
	}
	if got, _ := e.SelectedText(); got != "bc" {
		t.Errorf("SelectedText = %q, want %q", got, "bc")
	}
	if got, _ := vimRun([]string{"abcd"}, "$vh").SelectedText(); got != "cd" {
		t.Errorf("backwards: SelectedText = %q, want %q", got, "cd")
	}
	if got, _ := vimRun([]string{"ab", "cd", "ef"}, "lVj").SelectedText(); got != "ab\ncd" {
		t.Errorf("V-LINE: SelectedText = %q, want %q", got, "ab\ncd")
	}
	if e := vimRun([]string{"abc"}, "vl<esc>"); e.vimMode != vimNormal || e.selecting {
		t.Errorf("after esc: mode %v selecting %v, want NORMAL without selection", e.vimMode, e.selecting)
	}
}

// TestVimFormatting checks the formatting palette in vim mode: in VISUAL it
// wraps the whole selection and goes back to NORMAL mode.
func TestVimFormatting(t *testing.T) {
	e := vimRun([]string{"abcd"}, "vll").ApplyFormat("b")
	if got := text(e); got != "**abc**d" {
		t.Errorf("bold in VISUAL = %q, want %q", got, "**abc**d")
	}
	if e.vimMode != vimNormal || e.selecting {
		t.Errorf("after bold: mode %v selecting %v, want NORMAL without selection", e.vimMode, e.selecting)
	}

	e = vimRun([]string{"a", "b"}, "Vj").ApplyFormat("u")
	if got := text(e); got != "- a\n- b" {
		t.Errorf("list in V-LINE = %q, want %q", got, "- a\n- b")
	}
	if e.vimMode != vimNormal {
		t.Errorf("after list: mode %v, want NORMAL", e.vimMode)
	}

	e = vimRun([]string{"ab"}, "$").ApplyFormat("b")
	if got := text(e); got != "a****b" {
		t.Errorf("bold in NORMAL = %q, want %q", got, "a****b")
	}
	if n := runeLen(e.currentLine()); e.cursorCol >= n {
		t.Errorf("cursor col %d past the last character (line of %d)", e.cursorCol, n)
	}
	if e = press(e, vimKeys("u")...); text(e) != "ab" {
		t.Errorf("u after formatting = %q, want %q", text(e), "ab")
	}
}

// TestVimMouse checks that mouse selections become VISUAL ones in NORMAL
// mode, select the same text, and that clicks and scrolling leave VISUAL.
func TestVimMouse(t *testing.T) {
	e := vimRun([]string{"abc def"}, "").SetCursor(0, 5).SelectWord()
	if e.vimMode != vimVisual {
		t.Fatalf("double click: mode %v, want VISUAL", e.vimMode)
	}
	if got, _ := e.SelectedText(); got != "def" {
		t.Errorf("double click: SelectedText = %q, want %q", got, "def")
	}
	if got := text(press(e, vimKeys("d")...)); got != "abc " {
		t.Errorf("double click + d = %q, want %q", got, "abc ")
	}

	e = vimRun([]string{"a", "b"}, "").SelectLine()
	if e.vimMode != vimVisualLine {
		t.Fatalf("triple click: mode %v, want V-LINE", e.vimMode)
	}
	if got := text(press(e, vimKeys("d")...)); got != "b" {
		t.Errorf("triple click + d = %q, want %q", got, "b")
	}

	drags := []struct {
		name              string
		al, ac, line, col int
		want              string
	}{
		{"forward", 0, 1, 0, 4, "bcd"},
		{"backward", 0, 4, 0, 1, "bcd"},
		{"across lines", 0, 1, 1, 2, "bcdef\ngh"},
	}
	for _, d := range drags {
		e := vimRun([]string{"abcdef", "ghi"}, "").DragSelection(d.al, d.ac, d.line, d.col)
		if e.vimMode != vimVisual {
			t.Errorf("drag %s: mode %v, want VISUAL", d.name, e.vimMode)
		}
		if got, _ := e.SelectedText(); got != d.want {
			t.Errorf("drag %s: SelectedText = %q, want %q", d.name, got, d.want)
		}
	}

	if e := vimRun([]string{"abc"}, "vl").SetCursor(0, 0); e.vimMode != vimNormal || e.selecting {
		t.Errorf("click in VISUAL: mode %v selecting %v, want NORMAL", e.vimMode, e.selecting)
	}
	if e := vimRun([]string{"a", "b"}, "v").Scroll(1); e.vimMode != vimNormal || e.selecting {
		t.Errorf("scroll in VISUAL: mode %v selecting %v, want NORMAL", e.vimMode, e.selecting)
	}
	if e := vimRun([]string{"abc"}, "d").SetCursor(0, 9); e.cursorCol != 2 || e.vimPending != "" {
		t.Errorf("click past the line end: col %d pending %q, want 2 and none", e.cursorCol, e.vimPending)
	}

	// In INSERT mode a mouse selection stays a plain one: typing replaces it.
	e = vimRun([]string{"abc def"}, "i").SetCursor(0, 5).SelectWord()
	if e.vimMode != vimInsert {
		t.Fatalf("double click in INSERT: mode %v, want INSERT", e.vimMode)
	}
	if got := text(press(e, vimKeys("X")...)); got != "abc X" {
		t.Errorf("double click in INSERT + X = %q, want %q", got, "abc X")
	}
}

// TestVimPaste checks that pasted text is inserted as text in every mode and
// never runs as commands.
func TestVimPaste(t *testing.T) {
	tests := []struct {
		name, keys, paste, want string
	}{
		{"in NORMAL at the cursor", "", "XY", "XYabcd"},
		{"commands are text", "", "dd", "ddabcd"},
		{"several lines", "", "1\n2", "1\n2abcd"},
		{"in VISUAL replaces the selection", "lvl", "X", "aXd"},
		{"in INSERT", "A", "ef", "abcdef"},
	}
	for _, tt := range tests {
		e := press(vimRun([]string{"abcd"}, tt.keys), paste(tt.paste))
		if got := text(e); got != tt.want {
			t.Errorf("%s: text = %q, want %q", tt.name, got, tt.want)
		}
		if e.inVisual() || e.vimPending != "" {
			t.Errorf("%s: still VISUAL or pending after the paste", tt.name)
		}
	}
	if e := press(vimRun([]string{"a"}, ":"), paste("wq"), tea.KeyMsg{Type: tea.KeyEnter}); !e.WantsSave || !e.WantsExit {
		t.Errorf("paste on the command line: WantsSave %v WantsExit %v, want true true", e.WantsSave, e.WantsExit)
	}
}

// TestVimCommandLine checks the ":" commands.
func TestVimCommandLine(t *testing.T) {
	tests := []struct {
		keys             string
		save, exit, quit bool
		line             int
	}{
		{":w<cr>", true, false, false, 0},
		{":q<cr>", false, true, false, 0},
		{":wq<cr>", true, true, false, 0},
		{":x<cr>", true, true, false, 0},
		{":q!<cr>", false, false, true, 0},
		{": w <cr>", true, false, false, 0},
		{":wx<bs><cr>", true, false, false, 0},
		{":w<esc>", false, false, false, 0},
		{":3<cr>", false, false, false, 2},
		{":$<cr>", false, false, false, 3},
		{":99<cr>", false, false, false, 3},
		{"G:0<cr>", false, false, false, 0},
		{":<cr>", false, false, false, 0},
	}
	for _, tt := range tests {
		e := vimRun([]string{"a", "b", "c", "d"}, tt.keys)
		if e.WantsSave != tt.save || e.WantsExit != tt.exit || e.WantsQuit != tt.quit {
			t.Errorf("%q: save %v exit %v quit %v, want %v %v %v",
				tt.keys, e.WantsSave, e.WantsExit, e.WantsQuit, tt.save, tt.exit, tt.quit)
		}
		if e.cursorLine != tt.line {
			t.Errorf("%q: cursor line %d, want %d", tt.keys, e.cursorLine, tt.line)
		}
		if e.vimCmdOn {
			t.Errorf("%q: the command line is still open", tt.keys)
		}
	}

	e := vimRun([]string{"abc"}, ":wq")
	if !strings.Contains(e.Footer(), ":wq") {
		t.Errorf("footer while typing = %q, want it to show :wq", e.Footer())
	}
	if e := vimRun([]string{"abc"}, ":<bs>x"); e.vimCmdOn || text(e) != "bc" {
		t.Errorf("backspace on an empty command line: open %v text %q, want closed and x run", e.vimCmdOn, text(e))
	}
}

// TestVimSystemKeys checks that the Nook keys work in every mode and that
// esc never asks to leave the note.
func TestVimSystemKeys(t *testing.T) {
	for _, prefix := range []string{"", "i", "v", "V", "d", "3"} {
		e := vimRun([]string{"abc"}, prefix+"<C-s>")
		if !e.WantsSave || text(e) != "abc" {
			t.Errorf("%q ctrl+s: WantsSave %v text %q", prefix, e.WantsSave, text(e))
		}
		if e := vimRun([]string{"abc"}, prefix+"<C-q>"); !e.WantsExit {
			t.Errorf("%q ctrl+q: WantsExit false", prefix)
		}
		if e := vimRun([]string{"abc"}, prefix+"<C-t>"); !e.WantsPalette {
			t.Errorf("%q ctrl+t: WantsPalette false", prefix)
		}
		if e := vimRun([]string{"abc"}, prefix+"<esc><esc><esc>"); e.WantsExit || e.VimBusy() {
			t.Errorf("%q esc: WantsExit %v busy %v, want false false", prefix, e.WantsExit, e.VimBusy())
		}
	}
}

// TestVimBusy checks when esc (and ctrl+c) has something to cancel.
func TestVimBusy(t *testing.T) {
	for keys, want := range map[string]bool{
		"": false, "i": true, "v": true, "V": true, "d": true, "2": true, ":": true, "dd": false,
	} {
		if got := vimRun([]string{"abc"}, keys).VimBusy(); got != want {
			t.Errorf("keys %q: VimBusy = %v, want %v", keys, got, want)
		}
	}
	if New([]string{"abc"}, 40, 10).VimBusy() {
		t.Error("VimBusy = true without vim mode")
	}
}

// TestVimFooter checks the mode, the hints and the command being typed in
// the footer, and that it fills the width exactly at any size.
func TestVimFooter(t *testing.T) {
	for keys, want := range map[string]string{
		"": "-- NORMAL --  :w save", "i": "-- INSERT --  esc normal", "v": "-- VISUAL --  d delete",
		"V": "-- V-LINE --", "2d3": "2d3", "5g": "5g", "r": "r",
	} {
		if got := ansi.Strip(vimRun([]string{"abc"}, keys).Footer()); !strings.Contains(got, want) {
			t.Errorf("keys %q: footer = %q, want it to contain %q", keys, got, want)
		}
	}
	for _, keys := range []string{"", "i", "v", ":wq", "2d", ":nope<cr>"} {
		for w := 1; w <= 70; w++ {
			e := vimRun([]string{"abc"}, keys).SetSize(w, 10)
			if got := ansi.StringWidth(e.Footer()); got != w {
				t.Errorf("keys %q width %d: footer is %d cells wide", keys, w, got)
			}
		}
	}
}

// TestVimResize checks that resizing keeps the cursor and the mode, and that
// the motions use the new height.
func TestVimResize(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	e := vimRun(lines, "10Gvl").SetSize(12, 4)
	if e.cursorLine != 9 || e.vimMode != vimVisual {
		t.Fatalf("after resize: line %d mode %v, want 9 VISUAL", e.cursorLine, e.vimMode)
	}
	e = press(e, vimKeys("<esc><C-d>")...)
	if e.cursorLine != 11 {
		t.Errorf("ctrl+d with height 4: line %d, want 11", e.cursorLine)
	}
	if !strings.Contains(e.View(), "line") {
		t.Errorf("view after resize = %q", e.View())
	}
}

// TestVimOffKeepsUsualKeys checks that without vim mode the editor works as
// before: letters are typed, esc asks to leave, selections exclude their end.
func TestVimOffKeepsUsualKeys(t *testing.T) {
	e := press(New([]string{""}, 40, 10), vimKeys("dd")...)
	if got := e.Lines()[0]; got != "dd" {
		t.Fatalf("Lines()[0] = %q, want %q", got, "dd")
	}
	if e := press(New([]string{"a"}, 40, 10), vimKeys("<esc>")...); !e.WantsExit {
		t.Error("esc without vim mode does not ask to exit")
	}
	if got := ansi.Strip(New([]string{"a"}, 80, 10).Footer()); !strings.HasPrefix(got, edFooterHint) {
		t.Errorf("footer = %q, want the usual hint", got)
	}
	e = New([]string{"abcd"}, 40, 10).SetCursor(0, 0)
	e = press(e, tea.KeyMsg{Type: tea.KeyShiftRight}, tea.KeyMsg{Type: tea.KeyShiftRight})
	if got, _ := e.SelectedText(); got != "ab" {
		t.Errorf("shift+right twice selects %q, want %q", got, "ab")
	}
	if e := New([]string{"abc def"}, 40, 10).SelectWord(); e.inVisual() {
		t.Error("a double click turned on VISUAL without vim mode")
	}
}
