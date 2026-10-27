package editor

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// press delivers the KeyMsgs to the editor one after the other, each to the result
// of the previous one, and returns the final Editor.
func press(e Editor, msgs ...tea.KeyMsg) Editor {
	for _, m := range msgs {
		out, _ := e.Update(m)
		next, ok := out.(Editor)
		if !ok {
			panic(fmt.Sprintf("Update returned %T, want Editor", out))
		}
		e = next
	}
	return e
}

// TestSpaceKeyInserts checks that space arrives as tea.KeySpace (Type
// different from KeyRunes) and still inserts a space in the text.
func TestSpaceKeyInserts(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	if got := e.Lines()[0]; got != "a " {
		t.Fatalf("Lines()[0] = %q, want %q", got, "a ")
	}
}

// TestPasteKeepsAllRunes checks that a paste keeps all the runes, spaces
// included, of the pasted text.
func TestPasteKeepsAllRunes(t *testing.T) {
	e := New([]string{""}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab cd"), Paste: true})

	if got := e.Lines()[0]; got != "ab cd" {
		t.Fatalf("Lines()[0] = %q, want %q", got, "ab cd")
	}
}

// TestPasteMultilineSplits checks that a multi-line paste splits the line on
// '\n' instead of inserting the character.
func TestPasteMultilineSplits(t *testing.T) {
	e := New([]string{""}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true})

	got := e.Lines()
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("Lines() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Lines() = %q, want %q", got, want)
		}
	}
}

// TestHomeEnd checks that KeyHome and KeyEnd bring the cursor to the start and end of
// the line.
func TestHomeEnd(t *testing.T) {
	e := New([]string{"abc"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyHome})
	if got := e.CursorCol1(); got != 1 {
		t.Fatalf("CursorCol1() after home = %d, want 1", got)
	}

	e = press(e, tea.KeyMsg{Type: tea.KeyEnd})
	if got := e.CursorCol1(); got != 4 {
		t.Fatalf("CursorCol1() after end = %d, want 4", got)
	}
}

// TestCtrlUpDownDoc checks that KeyCtrlUp and KeyCtrlDown bring the cursor to the
// start and end of the document.
func TestCtrlUpDownDoc(t *testing.T) {
	e := New([]string{"a", "b", "c"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlUp})
	if got := e.CursorRow(); got != 1 {
		t.Fatalf("CursorRow() after ctrl+up = %d, want 1", got)
	}

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlDown})
	if got := e.CursorRow(); got != 3 {
		t.Fatalf("CursorRow() after ctrl+down = %d, want 3", got)
	}
}

// TestFormatPrefixApplies checks that ctrl+t followed by "b" applies the
// bold marker used by toggleBold (see format.go) and that the key
// does not also add its own text to the line.
func TestFormatPrefixApplies(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlT}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})

	if got := e.Lines()[0]; got != "a****" {
		t.Fatalf("Lines()[0] = %q, want %q (toggleBold marker)", got, "a****")
	}
	if strings.Contains(e.Lines()[0], "b") {
		t.Fatalf("Lines()[0] = %q, the prefix letter must not end up in the text", e.Lines()[0])
	}
}

// TestSystemKeyAfterPrefix checks that a system key after ctrl+t cancels
// the prefix and is handled normally, without formatting.
func TestSystemKeyAfterPrefix(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlT}, tea.KeyMsg{Type: tea.KeyCtrlS})

	if !e.WantsSave {
		t.Fatal("WantsSave = false, want true")
	}
	if got := e.Lines()[0]; got != "a" {
		t.Fatalf("Lines()[0] = %q, want %q (no formatting)", got, "a")
	}
}

// TestCtrlQWantsExit checks that ctrl+q asks to exit.
func TestCtrlQWantsExit(t *testing.T) {
	e := press(New([]string{"a"}, 10, 5), tea.KeyMsg{Type: tea.KeyCtrlQ})

	if !e.WantsExit {
		t.Fatal("WantsExit = false, want true")
	}
}

// TestCtrlSWantsSave checks that ctrl+s asks to save.
func TestCtrlSWantsSave(t *testing.T) {
	e := press(New([]string{"a"}, 10, 5), tea.KeyMsg{Type: tea.KeyCtrlS})

	if !e.WantsSave {
		t.Fatal("WantsSave = false, want true")
	}
}

// TestWordMovement covers movements and deletions by word and
// word/line selection, with real KeyMsgs and with explicit expected line and cursor.
//
// Test line "foo bar baz": the words start at columns 0, 4 and 8
// (0-based) and end at columns 3, 7 and 11; the cursor of New starts at the end of
// the line, that is at column 11.
func TestWordMovement(t *testing.T) {
	t.Run("ctrl+left goes to the start of the current or previous word", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5)

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorCol1(); got != 9 {
			t.Fatalf("after ctrl+left CursorCol1() = %d, want 9 (start of %q)", got, "baz")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorCol1(); got != 5 {
			t.Fatalf("after the second ctrl+left CursorCol1() = %d, want 5 (start of %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorCol1(); got != 1 {
			t.Fatalf("after the third ctrl+left CursorCol1() = %d, want 1 (start of %q)", got, "foo")
		}
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("CursorRow() = %d, want 1", got)
		}
		if got := e.Lines()[0]; got != "foo bar baz" {
			t.Fatalf("Lines()[0] = %q, want %q (the movement does not touch the text)", got, "foo bar baz")
		}
	})

	t.Run("alt+left behaves like ctrl+left", func(t *testing.T) {
		e := press(New([]string{"foo bar baz"}, 20, 5), tea.KeyMsg{Type: tea.KeyLeft, Alt: true})

		if got := e.CursorCol1(); got != 9 {
			t.Fatalf("after alt+left CursorCol1() = %d, want 9 (start of %q)", got, "baz")
		}
	})

	t.Run("ctrl+right goes to the end of the current or next word", func(t *testing.T) {
		e := press(New([]string{"foo bar baz"}, 20, 5), tea.KeyMsg{Type: tea.KeyHome})

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("after the first ctrl+right CursorCol1() = %d, want 4 (end of %q)", got, "foo")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorCol1(); got != 8 {
			t.Fatalf("after the second ctrl+right CursorCol1() = %d, want 8 (end of %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorCol1(); got != 12 {
			t.Fatalf("after the third ctrl+right CursorCol1() = %d, want 12 (end of line)", got)
		}
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("CursorRow() = %d, want 1", got)
		}
	})

	t.Run("alt+right behaves like ctrl+right", func(t *testing.T) {
		e := press(New([]string{"foo bar baz"}, 20, 5),
			tea.KeyMsg{Type: tea.KeyHome},
			tea.KeyMsg{Type: tea.KeyRight, Alt: true},
		)

		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("after alt+right CursorCol1() = %d, want 4 (end of %q)", got, "foo")
		}
	})

	t.Run("word movements stay on the line", func(t *testing.T) {
		// The cursor starts at the end of the first line ("foo", column 3).
		e := New([]string{"foo", "bar"}, 20, 5)

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("after ctrl+right CursorRow() = %d, want 1 (no crossing of lines)", got)
		}
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("after ctrl+right CursorCol1() = %d, want 4", got)
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("after ctrl+left CursorRow() = %d, want 1", got)
		}
		if got := e.CursorCol1(); got != 1 {
			t.Fatalf("after ctrl+left CursorCol1() = %d, want 1", got)
		}
	})

	t.Run("alt+backspace deletes up to the start of the word", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5) // cursor at the end of the line, column 11

		e = press(e, tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
		if got := e.Lines()[0]; got != "foo bar " {
			t.Fatalf("Lines()[0] after alt+backspace = %q, want %q", got, "foo bar ")
		}
		if got := e.CursorCol1(); got != 9 {
			t.Fatalf("CursorCol1() after alt+backspace = %d, want 9", got)
		}
		if !e.Dirty {
			t.Fatal("Dirty = false after alt+backspace, want true")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
		if got := e.Lines()[0]; got != "foo " {
			t.Fatalf("Lines()[0] after the second alt+backspace = %q, want %q", got, "foo ")
		}
		if got := e.CursorCol1(); got != 5 {
			t.Fatalf("CursorCol1() after the second alt+backspace = %d, want 5", got)
		}
	})

	t.Run("ctrl+u deletes from the cursor to the start of the line", func(t *testing.T) {
		e := New([]string{"foo bar"}, 20, 5) // cursor at the end of the line, column 7

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlU})
		if got := e.Lines()[0]; got != "" {
			t.Fatalf("Lines()[0] after ctrl+u = %q, want %q (the line becomes empty)", got, "")
		}
		if got := e.CursorCol1(); got != 1 {
			t.Fatalf("CursorCol1() after ctrl+u = %d, want 1", got)
		}
		if !e.Dirty {
			t.Fatal("Dirty = false after ctrl+u, want true")
		}
	})

	t.Run("ctrl+k deletes from the cursor to the end of the line", func(t *testing.T) {
		e := New([]string{"foo bar"}, 20, 5)
		e = press(e,
			tea.KeyMsg{Type: tea.KeyHome},
			tea.KeyMsg{Type: tea.KeyRight},
			tea.KeyMsg{Type: tea.KeyRight},
			tea.KeyMsg{Type: tea.KeyRight},
		)
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("CursorCol1() before ctrl+k = %d, want 4", got)
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlK})
		if got := e.Lines()[0]; got != "foo" {
			t.Fatalf("Lines()[0] after ctrl+k = %q, want %q", got, "foo")
		}
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("CursorCol1() after ctrl+k = %d, want 4", got)
		}
		if !e.Dirty {
			t.Fatal("Dirty = false after ctrl+k, want true")
		}
	})

	// The "shift+ctrl+left"/"shift+ctrl+right" keys reach withSelection: here
	// the same function is called directly, so the test does not depend on the
	// name that bubbletea gives to the key, and the produced selection is verified
	// by the text that a typed rune leaves behind.
	t.Run("the selection extended to the left covers the word before", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5) // cursor at the end of the line, column 11

		e = withSelection(e, Editor.moveWordLeft)
		if !e.selecting {
			t.Fatal("selecting = false after the extension, want true")
		}
		if got := e.anchorCol; got != 11 {
			t.Fatalf("anchorCol = %d, want 11 (anchor at the starting cursor)", got)
		}
		if got := e.cursorCol; got != 8 {
			t.Fatalf("cursorCol = %d, want 8 (start of %q)", got, "baz")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "foo bar x" {
			t.Fatalf("Lines()[0] = %q, want %q (the selection was %q)", got, "foo bar x", "baz")
		}
	})

	t.Run("the selection extended to the right covers the word after", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5).SetCursor(0, 4) // start of "bar"

		e = withSelection(e, Editor.moveWordRight)
		if got := e.anchorCol; got != 4 {
			t.Fatalf("anchorCol = %d, want 4", got)
		}
		if got := e.cursorCol; got != 7 {
			t.Fatalf("cursorCol = %d, want 7 (end of %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "foo x baz" {
			t.Fatalf("Lines()[0] = %q, want %q (the selection was %q)", got, "foo x baz", "bar")
		}
	})

	t.Run("SelectWord selects the word under the cursor", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5).SetCursor(0, 5) // dentro "bar"

		e = e.SelectWord()
		if !e.selecting {
			t.Fatal("selecting = false, want true")
		}
		if got := e.anchorLine; got != 0 {
			t.Fatalf("anchorLine = %d, want 0", got)
		}
		if got := e.anchorCol; got != 4 {
			t.Fatalf("anchorCol = %d, want 4 (start of %q)", got, "bar")
		}
		if got := e.cursorCol; got != 7 {
			t.Fatalf("cursorCol = %d, want 7 (end of %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "foo x baz" {
			t.Fatalf("Lines()[0] = %q, want %q (the selection was %q)", got, "foo x baz", "bar")
		}
	})

	t.Run("SelectWord at the end of a word takes the preceding word", func(t *testing.T) {
		e := New([]string{"foo bar"}, 20, 5) // cursor at the end of the line, right after "bar"

		e = e.SelectWord()
		if got := e.anchorCol; got != 4 {
			t.Fatalf("anchorCol = %d, want 4 (start of %q)", got, "bar")
		}
		if got := e.cursorCol; got != 7 {
			t.Fatalf("cursorCol = %d, want 7 (end of %q)", got, "bar")
		}
	})

	t.Run("SelectWord without a word clears the selection", func(t *testing.T) {
		e := New([]string{"   "}, 20, 5) // only separators

		e = e.SelectWord()
		if e.selecting {
			t.Fatal("selecting = true, want false (no word under the cursor)")
		}
		if got := e.cursorCol; got != 3 {
			t.Fatalf("cursorCol = %d, want 3 (cursor unchanged)", got)
		}
	})

	t.Run("SelectLine selects the whole line", func(t *testing.T) {
		e := New([]string{"ciao mondo"}, 20, 5).SetCursor(0, 4)

		e = e.SelectLine()
		if !e.selecting {
			t.Fatal("selecting = false, want true")
		}
		if got := e.anchorCol; got != 0 {
			t.Fatalf("anchorCol = %d, want 0", got)
		}
		if got := e.cursorCol; got != 10 {
			t.Fatalf("cursorCol = %d, want 10 (end of line)", got)
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "x" {
			t.Fatalf("Lines()[0] = %q, want %q (the whole line was selected)", got, "x")
		}
	})
}
