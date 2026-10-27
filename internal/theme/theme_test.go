package theme

import (
	"strings"
	"testing"

	lipgloss "github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"nook/internal/markdown"
)

// setTrueColorProfile impone al renderer di lipgloss un profilo colore esplicito: in
// test l'output non è un terminale e il profilo rilevato sarebbe Ascii, con il
// quale ogni stile renderebbe la stringa vuota e i confronti non direbbero
// nulla sull'aspetto degli stili.
func setTrueColorProfile() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// TestDimStylesDiffer verifica che ogni stile pieno abbia una resa attenuata
// diversa: le righe senza focus non devono apparire come quelle in focus.
func TestDimStylesDiffer(t *testing.T) {
	setTrueColorProfile()

	for k, full := range Styles {
		dim, ok := DimStyles[k]
		if !ok {
			t.Errorf("DimStyles non ha la chiave %v", k)
			continue
		}
		if dim.Render("x") == full.Render("x") {
			t.Errorf("DimStyles[%v] e Styles[%v] hanno la stessa resa %q",
				k, k, full.Render("x"))
		}
	}
}

// TestServiceStyles verifica che gli stili di servizio Rev e Toast producano
// una resa non vuota.
func TestServiceStyles(t *testing.T) {
	setTrueColorProfile()

	if got := Rev.String(); got == "" {
		t.Error("Rev.String() = \"\", want resa non vuota")
	}
	if got := Toast.String(); got == "" {
		t.Error("Toast.String() = \"\", want resa non vuota")
	}
}

// TestFolderStyles verifica che la cartella selezionata non sia sottolineata e
// che si distingua comunque da quelle non selezionate.
func TestFolderStyles(t *testing.T) {
	setTrueColorProfile()

	if FolderSel.GetUnderline() {
		t.Error("FolderSel ha la sottolineatura, want nessuna")
	}
	if got, want := FolderSel.Render("cartella/"), FolderDim.Render("cartella/"); got == want {
		t.Errorf("cartella selezionata e non selezionata hanno la stessa resa: %q", got)
	}
	if !FolderSel.GetBold() {
		t.Error("FolderSel non è in grassetto")
	}

	// La resa non deve contenere l'attributo di sottolineatura (SGR 4).
	if strings.Contains(FolderSel.Render("x"), "\x1b[4m") {
		t.Errorf("la resa della cartella selezionata contiene la sottolineatura: %q", FolderSel.Render("x"))
	}
}

// TestQuoteStyles verifica che il testo della citazione sia in corsivo e che la
// barra abbia uno stile proprio, distinto dal testo.
func TestQuoteStyles(t *testing.T) {
	setTrueColorProfile()

	if !Styles[markdown.StyleQuote].GetItalic() {
		t.Error("Styles[StyleQuote] non è in corsivo")
	}
	if !DimStyles[markdown.StyleQuote].GetItalic() {
		t.Error("DimStyles[StyleQuote] non è in corsivo")
	}

	// La barra è in grassetto e ha una resa diversa dal testo della citazione,
	// sia piena sia attenuata.
	if !Styles[markdown.StyleQuoteBar].GetBold() {
		t.Error("Styles[StyleQuoteBar] non è in grassetto")
	}
	if got, want := Styles[markdown.StyleQuoteBar].Render("▌"), Styles[markdown.StyleQuote].Render("▌"); got == want {
		t.Errorf("barra e testo della citazione hanno la stessa resa piena: %q", got)
	}
	if got, want := DimStyles[markdown.StyleQuoteBar].Render("▌"), DimStyles[markdown.StyleQuote].Render("▌"); got == want {
		t.Errorf("barra e testo della citazione hanno la stessa resa attenuata: %q", got)
	}
}

// TestQuoteItalicRendered verifica che l'attributo corsivo (SGR 3) compaia
// davvero nella resa del testo di citazione.
func TestQuoteItalicRendered(t *testing.T) {
	setTrueColorProfile()

	if got := Styles[markdown.StyleQuote].Render("x"); !strings.Contains(got, "\x1b[3") {
		t.Errorf("la resa del testo di citazione non contiene il corsivo: %q", got)
	}
}
