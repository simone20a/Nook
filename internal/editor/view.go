package editor

import (
	"strings"

	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/markdown"
	"nook/internal/theme"
)

// View disegna l'editor: una riga per ogni riga dello schermo, separate da "\n".
func (e Editor) View() string {
	out := ""
	for r := 0; r < e.height; r++ {
		if r > 0 {
			out += "\n"
		}
		out += e.renderRow(r)
	}
	return out
}

// Testi del footer dell'editor: i tasti disponibili, oppure l'attesa del tasto
// che applica la formattazione (ctrl+t già premuto).
const (
	edFooterHint   = "F2 salva · esc esci · F3 formatta · F1 aiuto"
	edFooterFormat = "ctrl+t …"
)

// Footer rende l'ultima riga della schermata dell'editor: una sola riga
// troncata alla larghezza, in colore attenuato via theme.DimText e completata
// con spazi a destra. Mentre l'editor aspetta il tasto di formattazione mostra
// "ctrl+t …". Non fa parte di View: la schermata gli riserva una riga e gli dà
// all'editor un'altezza di contenuto ridotta di uno.
func (e Editor) Footer() string {
	if e.width <= 0 {
		return ""
	}
	text := edFooterHint
	if e.awaitingFormat {
		text = edFooterFormat
	}
	plain := truncate(text, e.width)
	pad := e.width - ansi.StringWidth(plain)
	if pad < 0 {
		pad = 0
	}
	return theme.DimText.Render(plain) + strings.Repeat(" ", pad)
}

// renderRow disegna una riga dello schermo: la riga di focus mostra il testo
// grezzo col cursore, le altre mostrano i segmenti markdown resi con la palette
// attenuata di theme e troncati alla larghezza dell'editor. L'attenuazione delle
// righe senza focus è fissa: non dipende dalla distanza dal centro.
func (e Editor) renderRow(r int) string {
	if r == e.focusRow() {
		return e.renderFocusRow()
	}
	idx := e.docIndexForRow(r)
	if idx < 0 || idx >= len(e.lines) {
		return ""
	}
	raw := e.lines[idx]
	segs := markdown.RenderLine(raw, isInsideFence(e.lines, idx))

	// Le righe senza focus mostrano il markdown reso; se la selezione le
	// attraversa, la parte selezionata viene evidenziata.
	selFrom, selTo, hasSel := e.selectionOnLine(idx)
	fromByte, toByte := runeRangeToBytes(raw, selFrom, selTo)

	var b strings.Builder
	for _, seg := range segs {
		st := theme.DimStyles[seg.Style]
		if hasSel {
			b.WriteString(renderSegmentWithSelection(seg, fromByte, toByte, st))
			continue
		}
		b.WriteString(st.Render(seg.Text))
	}
	return truncate(b.String(), e.width)
}

// truncate accorcia s alla larghezza w in celle, aggiungendo "…" se taglia.
// È consapevole delle celle larghe e delle sequenze ANSI.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// isFenceLine indica se la riga apre o chiude un blocco di codice.
func isFenceLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// isInsideFence indica se la riga idx sta dentro un blocco di codice: conta le
// righe di fence che la precedono, un numero dispari significa fence aperto.
func isInsideFence(lines []string, idx int) bool {
	count := 0
	for i := 0; i < idx && i < len(lines); i++ {
		if isFenceLine(lines[i]) {
			count++
		}
	}
	return count%2 == 1
}

// renderFocusRow disegna la riga del cursore: testo grezzo, finestra
// orizzontale a partire da hScroll, cursore evidenziato in reverse. Il prefisso
// di blocco viene compensato come spazio per non spostare il testo a video.
// Tutto il testo mostrato è reso con theme.Selected (bold), tranne il carattere
// del cursore che resta in reverse.
func (e Editor) renderFocusRow() string {
	raw := ""
	if e.cursorLine >= 0 && e.cursorLine < len(e.lines) {
		raw = e.lines[e.cursorLine]
	}

	// Il prefisso di blocco rimosso viene compensato: spazi per i marcatori, ma
	// la barra verticale per le citazioni, così la barra resta continua anche
	// attraverso la Focus Line.
	padW := markdown.PrefixWidth(raw)
	gutter := strings.Repeat(" ", padW)
	if padW > 0 && markdown.IsQuote(raw) {
		gutter = theme.Styles[markdown.StyleQuoteBar].Render(markdown.QuoteBar) +
			strings.Repeat(" ", padW-1)
	}

	rawW := e.width - padW
	if rawW < 0 {
		rawW = 0
	}

	runes := []rune(raw)
	start := e.hScroll
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	end := start + rawW
	if end > len(runes) {
		end = len(runes)
	}

	cursorPos := padW + (e.cursorCol - e.hScroll)

	selFrom, selTo, hasSel := e.selectionOnLine(e.cursorLine)

	var b strings.Builder
	var run strings.Builder
	selectedStyle := false // true quando il testo in corso è dentro la selezione
	pending := false       // true quando c'è testo da rendere
	flush := func() {
		if !pending {
			return
		}
		if selectedStyle {
			b.WriteString(theme.Selection.Render(run.String()))
		} else {
			b.WriteString(theme.Selected.Render(run.String()))
		}
		run.Reset()
		pending = false
	}
	add := func(text string, sel bool) {
		if pending && sel != selectedStyle {
			flush()
		}
		selectedStyle = sel
		run.WriteString(text)
		pending = true
	}

	if gutter != "" {
		b.WriteString(gutter)
	}
	for i := start; i < end; i++ {
		ch := string(runes[i])
		if padW+(i-start) == cursorPos {
			flush()
			b.WriteString(theme.Rev.Render(ch))
			continue
		}
		add(ch, hasSel && i >= selFrom && i < selTo)
	}
	if cursorPos >= padW+(end-start) && cursorPos < e.width {
		flush()
		b.WriteString(theme.Rev.Render(" "))
	}
	flush()
	return b.String()
}
