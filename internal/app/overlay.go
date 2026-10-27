package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/theme"
)

// Caratteri del bordo dei riquadri sovrapposti: angoli e lati.
const (
	boxTopLeft     = "┌"
	boxTopRight    = "┐"
	boxBottomLeft  = "└"
	boxBottomRight = "┘"
	boxHorizontal  = "─"
	boxVertical    = "│"
)

// maxPopupW è la larghezza massima del testo di un popup per una larghezza di
// terminale w: all'area contenuto vanno tolti bordo e padding del riquadro.
// Restano comunque almeno 10 colonne di testo.
func maxPopupW(w int) int {
	c := contentW(w) - 4
	if c < 10 {
		return 10
	}
	return c
}

// maxPopupH è l'altezza massima del testo di un popup per un'altezza di
// terminale h: all'area contenuto vanno tolti bordo e padding del riquadro.
// Restano comunque almeno 3 righe di testo.
func maxPopupH(h int) int {
	c := contentH(h) - 4
	if c < 3 {
		return 3
	}
	return c
}

// centerPopup centra il riquadro costruito da popup sopra base, sull'area
// contentW(w) x contentH(h). base deve avere esattamente contentH(h) righe
// contigue, ciascuna larga contentW(w); le righe non coperte dal riquadro
// restano invariate.
//
// Il riquadro viene sempre disegnato: il minimo di 10 colonne e 3 righe
// garantito da maxPopupW e maxPopupH rende sempre possibile comporre il bordo
// con il padding, quindi non esiste un'area troppo piccola che giustifichi il
// ritorno della base invariata (la soglia è già applicata dentro gli helper,
// non sui valori grezzi di contentW/contentH).
func centerPopup(base string, w, h int, popup string) string {
	cw, ch := contentW(w), contentH(h)
	rows := splitRows(base, ch)

	lines := strings.Split(popup, "\n")

	// Larghezza interna del riquadro: la riga più lunga del popup, senza
	// superare maxPopupW(w). Ogni riga viene poi troncata a questa misura.
	textW := 0
	for _, l := range lines {
		if lw := ansi.StringWidth(l); lw > textW {
			textW = lw
		}
	}
	if limit := maxPopupW(w); textW > limit {
		textW = limit
	}

	// Altezza interna del riquadro: le righe del popup, senza superare
	// maxPopupH(h).
	textH := len(lines)
	if limit := maxPopupH(h); textH > limit {
		textH = limit
	}

	box := make([]string, 0, textH+4)
	box = append(box, boxTopLeft+strings.Repeat(boxHorizontal, textW+2)+boxTopRight)
	box = append(box, boxVertical+strings.Repeat(" ", textW+2)+boxVertical)
	for _, l := range lines[:textH] {
		box = append(box, boxVertical+" "+padPlain(truncatePlain(l, textW), textW)+" "+boxVertical)
	}
	box = append(box, boxVertical+strings.Repeat(" ", textW+2)+boxVertical)
	box = append(box, boxBottomLeft+strings.Repeat(boxHorizontal, textW+2)+boxBottomRight)

	boxW, boxH := ansi.StringWidth(box[0]), len(box)
	top := (ch - boxH) / 2
	if top < 0 {
		top = 0
	}
	left := (cw - boxW) / 2
	if left < 0 {
		left = 0
	}
	for i, boxRow := range box {
		r := top + i
		if r < 0 || r >= ch {
			continue
		}
		rows[r] = spliceRow(rows[r], left, boxW, boxRow, cw)
	}
	return strings.Join(rows, "\n")
}

// toastStyle sceglie lo stile del toast: quello di errore per i messaggi che
// cominciano con "errore", quello normale per tutto il resto (conferme come
// "salvato" o "nota creata").
func toastStyle(msg string) lipgloss.Style {
	if strings.HasPrefix(msg, "errore") {
		return theme.ToastError
	}
	return theme.Toast
}

// toastLine rende msg come unica riga di toast: testo con lo stile del toast,
// troncato alla larghezza massima del popup e centrato con spazi su contentW(w)
// colonne.
func toastLine(msg string, w int) string {
	cw := contentW(w)
	if cw <= 0 {
		return ""
	}
	if strings.ContainsAny(msg, "\n\r") {
		msg = strings.Join(strings.Fields(msg), " ")
	}
	text := truncatePlain(toastStyle(msg).Render(msg), maxPopupW(w))
	tw := ansi.StringWidth(text)
	if tw > cw {
		tw = cw
		text = truncatePlain(text, cw)
	}
	left := (cw - tw) / 2
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", cw-tw-left)
}

// overlayAt sovrappone block (righe separate da \n) su base a partire da
// row,col, tagliando tutto ciò che esce dall'area contentW(w) x contentH(h).
func overlayAt(base string, w, h, row, col int, block string) string {
	cw, ch := contentW(w), contentH(h)
	if cw <= 0 || ch <= 0 {
		return base
	}
	rows := splitRows(base, ch)
	for i, line := range strings.Split(block, "\n") {
		r := row + i
		if r < 0 || r >= ch {
			continue
		}
		c := col
		if c < 0 {
			line = ansi.Cut(line, -c, ansi.StringWidth(line))
			c = 0
		}
		if c >= cw {
			continue
		}
		line = ansi.Truncate(line, cw-c, "")
		if lw := ansi.StringWidth(line); lw == 0 {
			continue
		}
		rows[r] = spliceRow(rows[r], c, ansi.StringWidth(line), line, cw)
	}
	return strings.Join(rows, "\n")
}

// splitRows divide base nelle righe dell'area contenuto: toglie un eventuale a
// capo finale e completa con righe vuote quelle mancanti.
func splitRows(base string, ch int) []string {
	if ch <= 0 {
		return nil
	}
	if base == "" {
		return make([]string, ch)
	}
	rows := strings.Split(strings.TrimSuffix(base, "\n"), "\n")
	for len(rows) < ch {
		rows = append(rows, "")
	}
	return rows
}

// spliceRow sostituisce in row il segmento che parte da col e occupa cutW
// colonne con segment, lasciando invariate le parti esterne. row viene portata
// alla larghezza w prima del taglio, così il segmento cade a col anche quando
// la riga era più corta.
func spliceRow(row string, col, cutW int, segment string, w int) string {
	if col < 0 {
		segment = ansi.Cut(segment, -col, ansi.StringWidth(segment))
		col = 0
	}
	row = padPlain(row, w)
	prefix := ansi.Cut(row, 0, col)
	suffix := ansi.Cut(row, col+cutW, ansi.StringWidth(row))
	return prefix + segment + suffix
}

// padPlain porta s alla larghezza visiva w aggiungendo spazi a destra.
func padPlain(s string, w int) string {
	if cur := ansi.StringWidth(s); cur < w {
		return s + strings.Repeat(" ", w-cur)
	}
	return s
}
