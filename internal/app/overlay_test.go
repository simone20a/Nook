package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"nook/internal/theme"

	ansi "github.com/charmbracelet/x/ansi"
)

// TestCenterPopup verifica che il riquadro sovrapposto da centerPopup compaia
// sopra la base.
func TestCenterPopup(t *testing.T) {
	const base = "aaaa\nbbbb\ncccc"

	got := centerPopup(base, 4, 3, "X")
	if !strings.Contains(ansi.Strip(got), "X") {
		t.Errorf("centerPopup(base, 4, 3, \"X\") = %q, want contenente \"X\"", got)
	}
}

// TestToastLine verifica che il toast sia una sola riga e che non superi la
// larghezza dell'area contenuto.
func TestToastLine(t *testing.T) {
	const w = 60

	line := toastLine("err", w)
	if strings.Contains(line, "\n") {
		t.Errorf("toastLine(\"err\", %d) = %q, want una sola riga", w, line)
	}
	plain := ansi.Strip(line)
	if got := ansi.StringWidth(plain); got > contentW(w) {
		t.Errorf("toastLine(\"err\", %d) = %q, largo %d, want <= %d",
			w, plain, got, contentW(w))
	}
}

// TestOverlayAt verifica che overlayAt scriva il blocco a partire dalla riga e
// dalla colonna richieste, lasciando il resto della base invariato.
func TestOverlayAt(t *testing.T) {
	const base = "aaaa\nbbbb\ncccc"

	got := overlayAt(base, 4, 3, 1, 1, "ZZ")
	rows := strings.Split(got, "\n")
	if len(rows) != 3 {
		t.Fatalf("overlayAt ha %d righe, want 3:\n%s", len(rows), got)
	}
	if !strings.Contains(rows[1], "ZZ") {
		t.Errorf("seconda riga = %q, want contenente \"ZZ\"", rows[1])
	}
	if rows[0] != "aaaa" || rows[2] != "cccc" {
		t.Errorf("righe non coperte modificate: %q", got)
	}
}

// TestToastStyleByMessage verifica che i messaggi di errore usino lo stile
// rosso e le conferme lo stile d'accento, e che il toast non abbia più uno
// sfondo a badge.
func TestToastStyleByMessage(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	ok := toastLine("salvato", 76)
	if !strings.Contains(ok, theme.Toast.Render("salvato")) {
		t.Errorf("il toast di conferma non usa theme.Toast: %q", ok)
	}
	if strings.Contains(ok, theme.ToastError.Render("salvato")) {
		t.Errorf("il toast di conferma usa lo stile d'errore: %q", ok)
	}

	bad := toastLine("errore: qualcosa", 76)
	if !strings.Contains(bad, theme.ToastError.Render("errore: qualcosa")) {
		t.Errorf("il toast d'errore non usa theme.ToastError: %q", bad)
	}

	// Nessuno dei due stili dipinge lo sfondo: nella resa non compare il codice
	// SGR di sfondo (48;).
	for name, st := range map[string]lipgloss.Style{"Toast": theme.Toast, "ToastError": theme.ToastError} {
		if got := st.Render("x"); strings.Contains(got, "[48;") {
			t.Errorf("%s dipinge lo sfondo: %q", name, got)
		}
	}
}
