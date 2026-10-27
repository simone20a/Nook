package theme

import (
	"strings"
	"testing"

	lipgloss "github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"nook/internal/markdown"
)

// setTrueColorProfile forces an explicit colour profile on the lipgloss renderer: in
// tests the output is not a terminal and the detected profile would be Ascii, with
// which every style would render the empty string and the comparisons would say
// nothing about the appearance of the styles.
func setTrueColorProfile() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// TestDimStylesDiffer checks that every full style has a dimmed rendering
// that is different: lines without focus must not look like those in focus.
func TestDimStylesDiffer(t *testing.T) {
	setTrueColorProfile()

	for k, full := range Styles {
		dim, ok := DimStyles[k]
		if !ok {
			t.Errorf("DimStyles does not have the key %v", k)
			continue
		}
		if dim.Render("x") == full.Render("x") {
			t.Errorf("DimStyles[%v] e Styles[%v] have the same rendering %q",
				k, k, full.Render("x"))
		}
	}
}

// TestServiceStyles checks that the utility styles Rev and Toast produce
// a non-empty rendering.
func TestServiceStyles(t *testing.T) {
	setTrueColorProfile()

	if got := Rev.String(); got == "" {
		t.Error("Rev.String() = \"\", want non-empty rendering")
	}
	if got := Toast.String(); got == "" {
		t.Error("Toast.String() = \"\", want non-empty rendering")
	}
}

// TestFolderStyles checks that the selected folder is not underlined and
// that it still stands out from unselected ones.
func TestFolderStyles(t *testing.T) {
	setTrueColorProfile()

	if FolderSel.GetUnderline() {
		t.Error("FolderSel has underlining, want none")
	}
	if got, want := FolderSel.Render("folder/"), FolderDim.Render("folder/"); got == want {
		t.Errorf("selected and unselected folder have the same rendering: %q", got)
	}
	if !FolderSel.GetBold() {
		t.Error("FolderSel is not bold")
	}

	// The rendering must not contain the underline attribute (SGR 4).
	if strings.Contains(FolderSel.Render("x"), "\x1b[4m") {
		t.Errorf("the rendering of the selected folder contains underlining: %q", FolderSel.Render("x"))
	}
}

// TestQuoteStyles checks that the quote text is in italics and that the
// bar has a style of its own, distinct from the text.
func TestQuoteStyles(t *testing.T) {
	setTrueColorProfile()

	if !Styles[markdown.StyleQuote].GetItalic() {
		t.Error("Styles[StyleQuote] is not in italics")
	}
	if !DimStyles[markdown.StyleQuote].GetItalic() {
		t.Error("DimStyles[StyleQuote] is not in italics")
	}

	// The bar is bold and has a rendering different from the quote text,
	// both full and dimmed.
	if !Styles[markdown.StyleQuoteBar].GetBold() {
		t.Error("Styles[StyleQuoteBar] is not bold")
	}
	if got, want := Styles[markdown.StyleQuoteBar].Render("▌"), Styles[markdown.StyleQuote].Render("▌"); got == want {
		t.Errorf("bar and quote text have the same full rendering: %q", got)
	}
	if got, want := DimStyles[markdown.StyleQuoteBar].Render("▌"), DimStyles[markdown.StyleQuote].Render("▌"); got == want {
		t.Errorf("bar and quote text have the same dimmed rendering: %q", got)
	}
}

// TestQuoteItalicRendered checks that the italic attribute (SGR 3) appears
// really in the rendering of the quote text.
func TestQuoteItalicRendered(t *testing.T) {
	setTrueColorProfile()

	if got := Styles[markdown.StyleQuote].Render("x"); !strings.Contains(got, "\x1b[3") {
		t.Errorf("the rendering of the quote text does not contain italics: %q", got)
	}
}
