// Package markdown scompone una riga di markdown in segmenti di testo stilati.
// Il package non conosce colori né codici ANSI: si limita a decidere lo stile
// di ogni porzione di testo, lasciando al chiamante la resa grafica.
package markdown

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Style identifica lo stile di un Segment.
type Style int

// Stili riconosciuti dal renderer. StyleHeading1..StyleHeading6 sono
// contigui: lo stile di un'intestazione è StyleHeading1 + (livello - 1).
const (
	StyleNone Style = iota
	StyleHeading1
	StyleHeading2
	StyleHeading3
	StyleHeading4
	StyleHeading5
	StyleHeading6
	StyleBold
	StyleItalic
	StyleCode
	StyleStrike
	StyleLink
	StyleImage
	StyleQuote
	StyleBullet
	StyleCheckbox
	StyleFence
	StyleCodeBlock
	// StyleQuoteBar è il segmento della barra verticale che affianca una
	// citazione; il testo della citazione usa StyleQuote.
	StyleQuoteBar
)

// Segment è una porzione di riga con il suo stile.
//
// RawFrom e RawTo sono offset in byte nel testo grezzo (raw) passato a
// RenderLine, con RawTo escluso: indicano il tratto di sorgente che il segmento
// rende. Per il testo semplice è lo span esatto dei propri byte, per il
// contenuto inline è lo span del contenuto fra i delimitatori (delimitatori
// esclusi), per i segmenti sintetizzati (☐/☑, •) è lo span del marcatore grezzo
// che sostituiscono.
type Segment struct {
	Text    string
	Style   Style
	RawFrom int
	RawTo   int
}

var (
	// Intestazione: 1-6 "#" seguiti da almeno uno spazio o tab.
	headingRe = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)
	// Citazione: ">" seguito, opzionalmente, da uno spazio o tab.
	quoteRe = regexp.MustCompile(`^>[ \t]?(.*)$`)
	// Checkbox: "- [ ]" o "- [x]"/"- [X]" (valutata prima delle liste).
	checkboxRe = regexp.MustCompile(`^(\s*)- \[([ xX])\](.*)$`)
	// Lista non ordinata: -, * o + seguiti da almeno uno spazio.
	bulletRe = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	// Lista ordinata: solo per il calcolo della larghezza del prefisso.
	orderedRe = regexp.MustCompile(`^(\s*)\d+\.\s+`)

	// Sostituzioni inline, in ordine di priorità decrescente.
	imageRe            = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	linkRe             = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	boldRe             = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	strikeRe           = regexp.MustCompile(`~~([^~]+)~~`)
	codeRe             = regexp.MustCompile("`([^`]+)`")
	italicStarRe       = regexp.MustCompile(`\*([^*]+)\*`)
	italicUnderscoreRe = regexp.MustCompile(`_([^_]+)_`)
)

// inlinePattern associa un pattern inline allo stile del suo contenuto.
type inlinePattern struct {
	re  *regexp.Regexp
	sty Style
}

// inlinePatterns è ordinato per priorità: a parità di posizione vince il primo.
var inlinePatterns = []inlinePattern{
	{imageRe, StyleImage},
	{linkRe, StyleLink},
	{boldRe, StyleBold},
	{strikeRe, StyleStrike},
	{codeRe, StyleCode},
	{italicStarRe, StyleItalic},
	{italicUnderscoreRe, StyleItalic},
}

// isFenceLine indica se la riga apre o chiude un blocco di codice.
func isFenceLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// RenderLine scompone una riga markdown nei suoi segmenti stilati. Quando
// insideFence è vero la riga appartiene a un blocco di codice: viene restituita
// intatta, senza alcuna sostituzione.
func RenderLine(raw string, insideFence bool) []Segment {
	if insideFence {
		return []Segment{{Text: raw, Style: StyleCodeBlock, RawFrom: 0, RawTo: len(raw)}}
	}
	if isFenceLine(raw) {
		return []Segment{{Text: raw, Style: StyleFence, RawFrom: 0, RawTo: len(raw)}}
	}

	prefix, rest, base, off := splitPrefix(raw)
	segments := make([]Segment, 0, 4)
	if prefix != nil {
		segments = append(segments, *prefix)
	}
	return append(segments, renderInline(rest, base, off)...)
}

// splitPrefix riconosce il prefisso di blocco di raw e restituisce l'eventuale
// segmento già emesso per il prefisso (con i suoi offset nel grezzo), il testo
// che rimane da tokenizzare, lo stile base di quel testo e l'offset in byte di
// rest dentro raw. Il contenuto inizia sempre dopo il prefisso rimosso, quindi
// l'offset di rest è len(raw)-len(rest).
func splitPrefix(raw string) (prefix *Segment, rest string, base Style, off int) {
	rest, base, off = raw, StyleNone, 0

	// 1. Intestazione.
	if m := headingRe.FindStringSubmatch(rest); m != nil {
		return nil, m[2], Style(int(StyleHeading1) + len(m[1]) - 1), len(raw) - len(m[2])
	}
	// 2. Citazione. Il marcatore ">" (e l'eventuale spazio) diventa una barra
	// verticale larga quanto il marcatore rimosso, così il testo resta
	// allineato fra la Focus Line e le righe rese, e le citazioni consecutive
	// disegnano una barra continua.
	if m := quoteRe.FindStringSubmatch(rest); m != nil {
		end := len(raw) - len(m[1])
		return &Segment{Text: quoteBar(end), Style: StyleQuoteBar, RawFrom: 0, RawTo: end},
			m[1], StyleQuote, end
	}
	// 3. Checkbox, prima delle liste. Il segmento sintetizzato copre lo span del
	// marcatore grezzo sostituito, separatore compreso.
	if m := checkboxRe.FindStringSubmatch(rest); m != nil {
		mark := "☐"
		if m[2] == "x" || m[2] == "X" {
			mark = "☑"
		}
		body := strings.TrimLeft(m[3], " \t")
		end := len(raw) - len(m[3])
		return &Segment{Text: m[1] + mark + " ", Style: StyleCheckbox, RawFrom: 0, RawTo: end},
			body, StyleNone, len(raw) - len(body)
	}
	// 4. Lista non ordinata.
	if m := bulletRe.FindStringSubmatch(rest); m != nil {
		end := len(raw) - len(m[2])
		return &Segment{Text: m[1] + "• ", Style: StyleBullet, RawFrom: 0, RawTo: end},
			m[2], StyleNone, end
	}
	// 5. Nessun prefisso.
	return nil, raw, StyleNone, 0
}

// QuoteBar è il carattere della barra verticale che affianca le citazioni.
const QuoteBar = "▌"

// quoteBar costruisce la barra verticale larga width byte, cioè quanti byte
// occupava il marcatore ">" rimosso: la barra più uno spazio per ogni byte
// successivo al primo. Così "> testo" diventa "▌ testo" e ">testo" diventa
// "▌testo", mantenendo l'allineamento del contenuto con le altre righe.
func quoteBar(width int) string {
	if width <= 0 {
		return ""
	}
	return QuoteBar + strings.Repeat(" ", width-1)
}

// IsQuote indica se raw è una riga di citazione (comincia con ">").
func IsQuote(raw string) bool {
	return quoteRe.MatchString(raw)
}

// renderInline tokenizza rest assegnando lo stile base al testo comune e lo
// stile specifico al contenuto di ogni match inline. off è l'offset in byte di
// rest dentro la riga grezza: gli offset dei segmenti prodotti ne derivano.
func renderInline(rest string, base Style, off int) []Segment {
	var segments []Segment
	for rest != "" {
		bestIdx, bestLoc := -1, []int(nil)

		// Match più a sinistra; a parità di posizione vince il primo pattern.
		for i, p := range inlinePatterns {
			loc := p.re.FindStringSubmatchIndex(rest)
			if loc == nil {
				continue
			}
			if bestIdx < 0 || loc[0] < bestLoc[0] {
				bestIdx, bestLoc = i, loc
			}
		}
		if bestIdx < 0 {
			segments = appendText(segments, rest, base, off)
			break
		}
		if bestLoc[0] > 0 {
			segments = appendText(segments, rest[:bestLoc[0]], base, off)
		}
		// Il contenuto inline copre i byte fra i delimitatori, esclusi.
		segments = appendText(segments, rest[bestLoc[2]:bestLoc[3]], inlinePatterns[bestIdx].sty, off+bestLoc[2])
		rest = rest[bestLoc[1]:]
		off += bestLoc[1]
	}
	return segments
}

// appendText aggiunge un segmento, saltando il testo vuoto. off è l'offset in
// byte di text dentro la riga grezza; il testo comune è copiato intatto, quindi
// il segmento occupa esattamente i propri byte.
func appendText(segments []Segment, text string, sty Style, off int) []Segment {
	if text == "" {
		return segments
	}
	return append(segments, Segment{Text: text, Style: sty, RawFrom: off, RawTo: off + len(text)})
}

// MapCellToRaw converte una colonna in celle di visualizzazione nella colonna in
// byte della riga grezza da cui i segmenti sono stati prodotti. Se cell cade
// oltre tutti i segmenti restituisce la lunghezza totale della riga grezza.
//
// assumed: runa=cella nel renderer attuale
func MapCellToRaw(segs []Segment, cell int) int {
	if cell < 0 {
		return 0
	}
	total := 0
	for _, s := range segs {
		if s.RawTo > total {
			total = s.RawTo
		}
	}
	col := 0
	for _, s := range segs {
		n := utf8.RuneCountInString(s.Text)
		if cell < col+n {
			// Il segmento rende se stesso byte per byte solo quando il testo
			// coincide con lo span grezzo; i segmenti sintetizzati (☐/☑, •)
			// valgono per il marcatore che sostituiscono.
			if len(s.Text) != s.RawTo-s.RawFrom {
				return s.RawFrom
			}
			return s.RawFrom + byteOffsetOfRune(s.Text, cell-col)
		}
		col += n
	}
	return total
}

// byteOffsetOfRune restituisce l'offset in byte della runa di indice idx dentro
// s. Oltre l'ultima runa restituisce len(s).
func byteOffsetOfRune(s string, idx int) int {
	i := 0
	for off := range s {
		if i == idx {
			return off
		}
		i++
	}
	return len(s)
}

// PrefixWidth restituisce quante celle occupa il prefisso di blocco rimosso
// dalla riga. Tutti i prefissi riconosciuti sono ASCII, quindi il numero di
// rune coincide con il numero di celle.
func PrefixWidth(raw string) int {
	prefix, rest, _, _ := splitPrefix(raw)
	removed := utf8.RuneCountInString(raw) - utf8.RuneCountInString(rest)
	if prefix != nil || removed > 0 {
		return removed
	}
	// Lista ordinata: non rimossa da RenderLine, ma il cui prefisso va misurato.
	if m := orderedRe.FindStringSubmatch(raw); m != nil {
		return utf8.RuneCountInString(m[0])
	}
	return 0
}
