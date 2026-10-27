package editor

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// press consegna i KeyMsg all'editor uno dopo l'altro, ciascuno al risultato
// del precedente, e restituisce l'Editor finale.
func press(e Editor, msgs ...tea.KeyMsg) Editor {
	for _, m := range msgs {
		out, _ := e.Update(m)
		next, ok := out.(Editor)
		if !ok {
			panic(fmt.Sprintf("Update ha restituito %T, atteso Editor", out))
		}
		e = next
	}
	return e
}

// TestSpaceKeyInserts verifica che lo spazio arrivi come tea.KeySpace (Type
// diverso da KeyRunes) e inserisca comunque uno spazio nel testo.
func TestSpaceKeyInserts(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	if got := e.Lines()[0]; got != "a " {
		t.Fatalf("Lines()[0] = %q, atteso %q", got, "a ")
	}
}

// TestPasteKeepsAllRunes verifica che un paste conservi tutte le rune, spazi
// compresi, del testo incollato.
func TestPasteKeepsAllRunes(t *testing.T) {
	e := New([]string{""}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab cd"), Paste: true})

	if got := e.Lines()[0]; got != "ab cd" {
		t.Fatalf("Lines()[0] = %q, atteso %q", got, "ab cd")
	}
}

// TestPasteMultilineSplits verifica che un paste multilinea spezzi la riga sul
// '\n' invece di inserirne il carattere.
func TestPasteMultilineSplits(t *testing.T) {
	e := New([]string{""}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true})

	got := e.Lines()
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("Lines() = %q, atteso %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Lines() = %q, atteso %q", got, want)
		}
	}
}

// TestHomeEnd verifica che KeyHome e KeyEnd portino il cursore a inizio e fine
// riga.
func TestHomeEnd(t *testing.T) {
	e := New([]string{"abc"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyHome})
	if got := e.CursorCol1(); got != 1 {
		t.Fatalf("CursorCol1() dopo home = %d, atteso 1", got)
	}

	e = press(e, tea.KeyMsg{Type: tea.KeyEnd})
	if got := e.CursorCol1(); got != 4 {
		t.Fatalf("CursorCol1() dopo end = %d, atteso 4", got)
	}
}

// TestCtrlUpDownDoc verifica che KeyCtrlUp e KeyCtrlDown portino il cursore a
// inizio e fine documento.
func TestCtrlUpDownDoc(t *testing.T) {
	e := New([]string{"a", "b", "c"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlUp})
	if got := e.CursorRow(); got != 1 {
		t.Fatalf("CursorRow() dopo ctrl+up = %d, atteso 1", got)
	}

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlDown})
	if got := e.CursorRow(); got != 3 {
		t.Fatalf("CursorRow() dopo ctrl+down = %d, atteso 3", got)
	}
}

// TestFormatPrefixApplies verifica che ctrl+t seguito da "b" applichi il
// marcatore del grassetto usato da toggleBold (leggi format.go) e che il tasto
// non aggiunga anche il proprio testo alla riga.
func TestFormatPrefixApplies(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlT}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})

	if got := e.Lines()[0]; got != "a****" {
		t.Fatalf("Lines()[0] = %q, atteso %q (marker di toggleBold)", got, "a****")
	}
	if strings.Contains(e.Lines()[0], "b") {
		t.Fatalf("Lines()[0] = %q, la lettera del prefisso non deve finire nel testo", e.Lines()[0])
	}
}

// TestSystemKeyAfterPrefix verifica che un tasto di sistema dopo ctrl+t annulli
// il prefisso e venga gestito normalmente, senza formattazione.
func TestSystemKeyAfterPrefix(t *testing.T) {
	e := New([]string{"a"}, 10, 5)

	e = press(e, tea.KeyMsg{Type: tea.KeyCtrlT}, tea.KeyMsg{Type: tea.KeyCtrlS})

	if !e.WantsSave {
		t.Fatal("WantsSave = false, atteso true")
	}
	if got := e.Lines()[0]; got != "a" {
		t.Fatalf("Lines()[0] = %q, atteso %q (nessuna formattazione)", got, "a")
	}
}

// TestCtrlQWantsExit verifica che ctrl+q chieda di uscire.
func TestCtrlQWantsExit(t *testing.T) {
	e := press(New([]string{"a"}, 10, 5), tea.KeyMsg{Type: tea.KeyCtrlQ})

	if !e.WantsExit {
		t.Fatal("WantsExit = false, atteso true")
	}
}

// TestCtrlSWantsSave verifica che ctrl+s chieda di salvare.
func TestCtrlSWantsSave(t *testing.T) {
	e := press(New([]string{"a"}, 10, 5), tea.KeyMsg{Type: tea.KeyCtrlS})

	if !e.WantsSave {
		t.Fatal("WantsSave = false, atteso true")
	}
}

// TestWordMovement copre i movimenti e le cancellazioni per parola e la
// selezione parola/riga, con KeyMsg reali e con riga e cursore attesi espliciti.
//
// Riga di prova "foo bar baz": le parole cominciano alle colonne 0, 4 e 8
// (0-based) e finiscono alle colonne 3, 7 e 11; il cursore di New parte in fondo
// alla riga, cioè alla colonna 11.
func TestWordMovement(t *testing.T) {
	t.Run("ctrl+left va a inizio della parola corrente o precedente", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5)

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorCol1(); got != 9 {
			t.Fatalf("dopo ctrl+left CursorCol1() = %d, atteso 9 (inizio di %q)", got, "baz")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorCol1(); got != 5 {
			t.Fatalf("dopo il secondo ctrl+left CursorCol1() = %d, atteso 5 (inizio di %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorCol1(); got != 1 {
			t.Fatalf("dopo il terzo ctrl+left CursorCol1() = %d, atteso 1 (inizio di %q)", got, "foo")
		}
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("CursorRow() = %d, atteso 1", got)
		}
		if got := e.Lines()[0]; got != "foo bar baz" {
			t.Fatalf("Lines()[0] = %q, atteso %q (il movimento non tocca il testo)", got, "foo bar baz")
		}
	})

	t.Run("alt+left si comporta come ctrl+left", func(t *testing.T) {
		e := press(New([]string{"foo bar baz"}, 20, 5), tea.KeyMsg{Type: tea.KeyLeft, Alt: true})

		if got := e.CursorCol1(); got != 9 {
			t.Fatalf("dopo alt+left CursorCol1() = %d, atteso 9 (inizio di %q)", got, "baz")
		}
	})

	t.Run("ctrl+right va a fine della parola corrente o successiva", func(t *testing.T) {
		e := press(New([]string{"foo bar baz"}, 20, 5), tea.KeyMsg{Type: tea.KeyHome})

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("dopo il primo ctrl+right CursorCol1() = %d, atteso 4 (fine di %q)", got, "foo")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorCol1(); got != 8 {
			t.Fatalf("dopo il secondo ctrl+right CursorCol1() = %d, atteso 8 (fine di %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorCol1(); got != 12 {
			t.Fatalf("dopo il terzo ctrl+right CursorCol1() = %d, atteso 12 (fine riga)", got)
		}
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("CursorRow() = %d, atteso 1", got)
		}
	})

	t.Run("alt+right si comporta come ctrl+right", func(t *testing.T) {
		e := press(New([]string{"foo bar baz"}, 20, 5),
			tea.KeyMsg{Type: tea.KeyHome},
			tea.KeyMsg{Type: tea.KeyRight, Alt: true},
		)

		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("dopo alt+right CursorCol1() = %d, atteso 4 (fine di %q)", got, "foo")
		}
	})

	t.Run("i movimenti per parola restano nella riga", func(t *testing.T) {
		// Il cursore parte in fondo alla prima riga ("foo", colonna 3).
		e := New([]string{"foo", "bar"}, 20, 5)

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlRight})
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("dopo ctrl+right CursorRow() = %d, atteso 1 (nessun scavalcamento di riga)", got)
		}
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("dopo ctrl+right CursorCol1() = %d, atteso 4", got)
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlLeft})
		if got := e.CursorRow(); got != 1 {
			t.Fatalf("dopo ctrl+left CursorRow() = %d, atteso 1", got)
		}
		if got := e.CursorCol1(); got != 1 {
			t.Fatalf("dopo ctrl+left CursorCol1() = %d, atteso 1", got)
		}
	})

	t.Run("alt+backspace cancella fino a inizio parola", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5) // cursore a fine riga, colonna 11

		e = press(e, tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
		if got := e.Lines()[0]; got != "foo bar " {
			t.Fatalf("Lines()[0] dopo alt+backspace = %q, atteso %q", got, "foo bar ")
		}
		if got := e.CursorCol1(); got != 9 {
			t.Fatalf("CursorCol1() dopo alt+backspace = %d, atteso 9", got)
		}
		if !e.Dirty {
			t.Fatal("Dirty = false dopo alt+backspace, atteso true")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
		if got := e.Lines()[0]; got != "foo " {
			t.Fatalf("Lines()[0] dopo il secondo alt+backspace = %q, atteso %q", got, "foo ")
		}
		if got := e.CursorCol1(); got != 5 {
			t.Fatalf("CursorCol1() dopo il secondo alt+backspace = %d, atteso 5", got)
		}
	})

	t.Run("ctrl+u cancella dal cursore a inizio riga", func(t *testing.T) {
		e := New([]string{"foo bar"}, 20, 5) // cursore a fine riga, colonna 7

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlU})
		if got := e.Lines()[0]; got != "" {
			t.Fatalf("Lines()[0] dopo ctrl+u = %q, atteso %q (la riga si svuota)", got, "")
		}
		if got := e.CursorCol1(); got != 1 {
			t.Fatalf("CursorCol1() dopo ctrl+u = %d, atteso 1", got)
		}
		if !e.Dirty {
			t.Fatal("Dirty = false dopo ctrl+u, atteso true")
		}
	})

	t.Run("ctrl+k cancella dal cursore a fine riga", func(t *testing.T) {
		e := New([]string{"foo bar"}, 20, 5)
		e = press(e,
			tea.KeyMsg{Type: tea.KeyHome},
			tea.KeyMsg{Type: tea.KeyRight},
			tea.KeyMsg{Type: tea.KeyRight},
			tea.KeyMsg{Type: tea.KeyRight},
		)
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("CursorCol1() prima di ctrl+k = %d, atteso 4", got)
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyCtrlK})
		if got := e.Lines()[0]; got != "foo" {
			t.Fatalf("Lines()[0] dopo ctrl+k = %q, atteso %q", got, "foo")
		}
		if got := e.CursorCol1(); got != 4 {
			t.Fatalf("CursorCol1() dopo ctrl+k = %d, atteso 4", got)
		}
		if !e.Dirty {
			t.Fatal("Dirty = false dopo ctrl+k, atteso true")
		}
	})

	// I tasti "shift+ctrl+left"/"shift+ctrl+right" arrivano a withSelection: qui
	// la stessa funzione è chiamata direttamente, così il test non dipende dal
	// nome che bubbletea assegna al tasto, e la selezione prodotta è verificata
	// dal testo che una runa digitata lascia dietro di sé.
	t.Run("la selezione estesa a sinistra copre la parola prima", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5) // cursore a fine riga, colonna 11

		e = withSelection(e, Editor.moveWordLeft)
		if !e.selecting {
			t.Fatal("selecting = false dopo l'estensione, atteso true")
		}
		if got := e.anchorCol; got != 11 {
			t.Fatalf("anchorCol = %d, atteso 11 (ancora al cursore di partenza)", got)
		}
		if got := e.cursorCol; got != 8 {
			t.Fatalf("cursorCol = %d, atteso 8 (inizio di %q)", got, "baz")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "foo bar x" {
			t.Fatalf("Lines()[0] = %q, atteso %q (la selezione era %q)", got, "foo bar x", "baz")
		}
	})

	t.Run("la selezione estesa a destra copre la parola dopo", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5).SetCursor(0, 4) // inizio di "bar"

		e = withSelection(e, Editor.moveWordRight)
		if got := e.anchorCol; got != 4 {
			t.Fatalf("anchorCol = %d, atteso 4", got)
		}
		if got := e.cursorCol; got != 7 {
			t.Fatalf("cursorCol = %d, atteso 7 (fine di %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "foo x baz" {
			t.Fatalf("Lines()[0] = %q, atteso %q (la selezione era %q)", got, "foo x baz", "bar")
		}
	})

	t.Run("SelectWord seleziona la parola sotto il cursore", func(t *testing.T) {
		e := New([]string{"foo bar baz"}, 20, 5).SetCursor(0, 5) // dentro "bar"

		e = e.SelectWord()
		if !e.selecting {
			t.Fatal("selecting = false, atteso true")
		}
		if got := e.anchorLine; got != 0 {
			t.Fatalf("anchorLine = %d, atteso 0", got)
		}
		if got := e.anchorCol; got != 4 {
			t.Fatalf("anchorCol = %d, atteso 4 (inizio di %q)", got, "bar")
		}
		if got := e.cursorCol; got != 7 {
			t.Fatalf("cursorCol = %d, atteso 7 (fine di %q)", got, "bar")
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "foo x baz" {
			t.Fatalf("Lines()[0] = %q, atteso %q (la selezione era %q)", got, "foo x baz", "bar")
		}
	})

	t.Run("SelectWord a fine parola prende la parola che precede", func(t *testing.T) {
		e := New([]string{"foo bar"}, 20, 5) // cursore a fine riga, subito dopo "bar"

		e = e.SelectWord()
		if got := e.anchorCol; got != 4 {
			t.Fatalf("anchorCol = %d, atteso 4 (inizio di %q)", got, "bar")
		}
		if got := e.cursorCol; got != 7 {
			t.Fatalf("cursorCol = %d, atteso 7 (fine di %q)", got, "bar")
		}
	})

	t.Run("SelectWord senza parola azzera la selezione", func(t *testing.T) {
		e := New([]string{"   "}, 20, 5) // solo separatori

		e = e.SelectWord()
		if e.selecting {
			t.Fatal("selecting = true, atteso false (nessuna parola sotto il cursore)")
		}
		if got := e.cursorCol; got != 3 {
			t.Fatalf("cursorCol = %d, atteso 3 (cursore invariato)", got)
		}
	})

	t.Run("SelectLine seleziona l'intera riga", func(t *testing.T) {
		e := New([]string{"ciao mondo"}, 20, 5).SetCursor(0, 4)

		e = e.SelectLine()
		if !e.selecting {
			t.Fatal("selecting = false, atteso true")
		}
		if got := e.anchorCol; got != 0 {
			t.Fatalf("anchorCol = %d, atteso 0", got)
		}
		if got := e.cursorCol; got != 10 {
			t.Fatalf("cursorCol = %d, atteso 10 (fine riga)", got)
		}

		e = press(e, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if got := e.Lines()[0]; got != "x" {
			t.Fatalf("Lines()[0] = %q, atteso %q (la riga era tutta selezionata)", got, "x")
		}
	})
}
