package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"nook/internal/theme"

	ansi "github.com/charmbracelet/x/ansi"
)

// TestCenterPopup checks that the box overlaid by centerPopup appears
// above the base.
func TestCenterPopup(t *testing.T) {
	const base = "aaaa\nbbbb\ncccc"

	got := centerPopup(base, 4, 3, "X")
	if !strings.Contains(ansi.Strip(got), "X") {
		t.Errorf("centerPopup(base, 4, 3, \"X\") = %q, want containing \"X\"", got)
	}
}

// TestToastLine checks that the toast is a single row and does not exceed the
// width of the content area.
func TestToastLine(t *testing.T) {
	const w = 60

	line := toastLine("err", w)
	if strings.Contains(line, "\n") {
		t.Errorf("toastLine(\"err\", %d) = %q, want a single row", w, line)
	}
	plain := ansi.Strip(line)
	if got := ansi.StringWidth(plain); got > contentW(w) {
		t.Errorf("toastLine(\"err\", %d) = %q, wide %d, want <= %d",
			w, plain, got, contentW(w))
	}
}

// TestOverlayAt checks that overlayAt writes the block starting from the row and
// the requested column, leaving the rest of the base unchanged.
func TestOverlayAt(t *testing.T) {
	const base = "aaaa\nbbbb\ncccc"

	got := overlayAt(base, 4, 3, 1, 1, "ZZ")
	rows := strings.Split(got, "\n")
	if len(rows) != 3 {
		t.Fatalf("overlayAt has %d rows, want 3:\n%s", len(rows), got)
	}
	if !strings.Contains(rows[1], "ZZ") {
		t.Errorf("second row = %q, want containing \"ZZ\"", rows[1])
	}
	if rows[0] != "aaaa" || rows[2] != "cccc" {
		t.Errorf("uncovered rows modified: %q", got)
	}
}

// TestToastStyleByMessage checks that error messages use the
// red style and confirmations the accent style, and that the toast no longer has a
// badge background.
func TestToastStyleByMessage(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	ok := toastLine("saved", 76)
	if !strings.Contains(ok, theme.Toast.Render("saved")) {
		t.Errorf("the confirmation toast does not use theme.Toast: %q", ok)
	}
	if strings.Contains(ok, theme.ToastError.Render("saved")) {
		t.Errorf("the confirmation toast uses the error style: %q", ok)
	}

	bad := toastLine("error: something", 76)
	if !strings.Contains(bad, theme.ToastError.Render("error: something")) {
		t.Errorf("the error toast does not use theme.ToastError: %q", bad)
	}

	// Neither style paints the background: the rendering does not contain the
	// background SGR code (48;).
	for name, st := range map[string]lipgloss.Style{"Toast": theme.Toast, "ToastError": theme.ToastError} {
		if got := st.Render("x"); strings.Contains(got, "[48;") {
			t.Errorf("%s paints the background: %q", name, got)
		}
	}
}
