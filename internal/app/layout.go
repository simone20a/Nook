package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
)

// Geometria dei margini dell'applicazione: marginX colonne e marginY righe
// su tutti i lati, senza bordi. Sotto le soglie smallW/smallH i margini sono
// azzerati per non comprimere il contenuto.
const (
	marginX = 2
	marginY = 1
	smallW  = 30
	smallH  = 10
)

// marginsActive indica se lo schermo è abbastanza grande per i margini.
func marginsActive(w, h int) bool { return w >= smallW && h >= smallH }

// contentW è la larghezza dell'area contenuto per una larghezza w.
func contentW(w int) int {
	if w >= smallW {
		return w - marginX*2
	}
	return w
}

// contentH è l'altezza dell'area contenuto per un'altezza h.
func contentH(h int) int {
	if h >= smallH {
		return h - marginY*2
	}
	return h
}

// truncatePlain tronca s alla larghezza w, aggiungendo "…" se serve.
func truncatePlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// footerLine rende text come unica riga di footer larga w colonne: il testo
// viene troncato a w, reso con lo stile st e completato con spazi a destra.
func footerLine(text string, w int, st lipgloss.Style) string {
	if w <= 0 {
		return ""
	}
	plain := truncatePlain(text, w)
	return st.Render(plain) + strings.Repeat(" ", w-ansi.StringWidth(plain))
}

// withMargins applica i margini al contenuto dell'area contenuto.
// Il content deve avere esattamente contentH(h) righe, ciascuna già larga
// contentW(w); con margini inattivi il content viene restituito invariato.
func withMargins(content string, w, h int) string {
	if !marginsActive(w, h) {
		return content
	}
	return lipgloss.NewStyle().Padding(marginY, marginX).Render(content)
}
