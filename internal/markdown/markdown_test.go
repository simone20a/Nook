package markdown

import (
	"testing"
	"unicode/utf8"
)

// checkSegments confronta i segmenti attesi con quelli prodotti dal renderer,
// verificando il numero di segmenti, il testo e lo stile di ognuno.
func checkSegments(t *testing.T, raw string, got, want []Segment) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("RenderLine(%q): %d segmenti, want %d\ngot  = %#v\nwant = %#v",
			raw, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i].Text != want[i].Text {
			t.Errorf("RenderLine(%q) segmento %d: testo = %q, want %q",
				raw, i, got[i].Text, want[i].Text)
		}
		if got[i].Style != want[i].Style {
			t.Errorf("RenderLine(%q) segmento %d: stile = %v, want %v",
				raw, i, got[i].Style, want[i].Style)
		}
	}
}

// TestRenderLine verifica i casi esatti della specifica: prefissi di blocco,
// sostituzioni inline, fence e righe dentro un blocco di codice.
func TestRenderLine(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		insideFence bool
		want        []Segment
	}{
		{
			name: "intestazione h1",
			raw:  "# Titolo",
			want: []Segment{{Text: "Titolo", Style: StyleHeading1}},
		},
		{
			name: "intestazione h3",
			raw:  "### Sotto",
			want: []Segment{{Text: "Sotto", Style: StyleHeading3}},
		},
		{
			name: "grassetto e corsivo",
			raw:  "**grassetto** e *corsivo*",
			want: []Segment{
				{Text: "grassetto", Style: StyleBold},
				{Text: " e ", Style: StyleNone},
				{Text: "corsivo", Style: StyleItalic},
			},
		},
		{
			name: "codice e barrato",
			raw:  "`codice` e ~~barrato~~",
			want: []Segment{
				{Text: "codice", Style: StyleCode},
				{Text: " e ", Style: StyleNone},
				{Text: "barrato", Style: StyleStrike},
			},
		},
		{
			name: "link",
			raw:  "[link](https://x.it)",
			want: []Segment{{Text: "link", Style: StyleLink}},
		},
		{
			name: "immagine",
			raw:  "![alt](img.png)",
			want: []Segment{{Text: "alt", Style: StyleImage}},
		},
		{
			name: "citazione",
			raw:  "> citazione",
			want: []Segment{
				{Text: "▌ ", Style: StyleQuoteBar},
				{Text: "citazione", Style: StyleQuote},
			},
		},
		{
			name: "lista",
			raw:  "- voce",
			want: []Segment{
				{Text: "• ", Style: StyleBullet},
				{Text: "voce", Style: StyleNone},
			},
		},
		{
			name: "checkbox fatta",
			raw:  "- [x] fatto",
			want: []Segment{
				{Text: "☑ ", Style: StyleCheckbox},
				{Text: "fatto", Style: StyleNone},
			},
		},
		{
			name: "checkbox da fare",
			raw:  "- [ ] da fare",
			want: []Segment{
				{Text: "☐ ", Style: StyleCheckbox},
				{Text: "da fare", Style: StyleNone},
			},
		},
		{
			name: "apertura fence",
			raw:  "```go",
			want: []Segment{{Text: "```go", Style: StyleFence}},
		},
		{
			name: "riga semplice",
			raw:  "riga semplice",
			want: []Segment{{Text: "riga semplice", Style: StyleNone}},
		},
		{
			name:        "dentro il fence",
			raw:         "**x**",
			insideFence: true,
			want:        []Segment{{Text: "**x**", Style: StyleCodeBlock}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkSegments(t, tc.raw, RenderLine(tc.raw, tc.insideFence), tc.want)
		})
	}
}

// TestSegmentRawOffsets verifica che ogni segmento di "a **b** *c*" riporti gli
// offset in byte del tratto di sorgente che rende, con RawTo escluso: il testo
// semplice occupa i propri byte, il contenuto inline i byte fra i delimitatori.
func TestSegmentRawOffsets(t *testing.T) {
	const raw = "a **b** *c*"
	want := []Segment{
		{Text: "a ", Style: StyleNone, RawFrom: 0, RawTo: 2},
		{Text: "b", Style: StyleBold, RawFrom: 4, RawTo: 5},
		{Text: " ", Style: StyleNone, RawFrom: 7, RawTo: 8},
		{Text: "c", Style: StyleItalic, RawFrom: 9, RawTo: 10},
	}

	got := RenderLine(raw, false)
	checkSegments(t, raw, got, want)

	for i := range want {
		if got[i].RawFrom != want[i].RawFrom || got[i].RawTo != want[i].RawTo {
			t.Errorf("RenderLine(%q) segmento %d: offset = %d..%d, want %d..%d",
				raw, i, got[i].RawFrom, got[i].RawTo, want[i].RawFrom, want[i].RawTo)
			continue
		}
		// Gli offset devono ritagliare esattamente il testo del segmento.
		if sliced := raw[got[i].RawFrom:got[i].RawTo]; sliced != got[i].Text {
			t.Errorf("RenderLine(%q) segmento %d: raw[%d:%d] = %q, want %q",
				raw, i, got[i].RawFrom, got[i].RawTo, sliced, got[i].Text)
		}
	}
}

// TestMapCellToRaw verifica la mappatura cella di visualizzazione → offset in
// byte nel raw per "**ab** cd": le celle sono quelle della riga resa, che non
// mostra i delimitatori, quindi dopo "ab" la mappatura salta i due "*" finali.
func TestMapCellToRaw(t *testing.T) {
	const raw = "**ab** cd"
	segs := RenderLine(raw, false)

	// La riga resa è "ab cd": 'a' → 2, 'b' → 3, poi la cella dello spazio → 6
	// (i due "*" di chiusura non occupano celle), 'c' → 7, 'd' → 8. La cella 5
	// cade oltre tutti i segmenti e vale la lunghezza totale del raw (9).
	cells := []struct {
		cell int
		want int
	}{
		{cell: 0, want: 2},
		{cell: 1, want: 3},
		{cell: 2, want: 6},
		{cell: 3, want: 7},
		{cell: 4, want: 8},
		{cell: 5, want: len(raw)},
	}

	for _, c := range cells {
		if got := MapCellToRaw(segs, c.cell); got != c.want {
			t.Errorf("MapCellToRaw(%q, %d) = %d, want %d", raw, c.cell, got, c.want)
		}
	}
}

// TestPrefixWidth verifica la larghezza in celle del prefisso di blocco rimosso.
func TestPrefixWidth(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"# T", 2},
		{"### S", 4},
		{"> c", 2},
		{"  - v", 4},
		{"- [ ] d", 6},
		{"1. v", 3},
		{"plain", 0},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if got := PrefixWidth(tc.raw); got != tc.want {
				t.Errorf("PrefixWidth(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

// TestQuoteBar verifica che il marcatore ">" diventi una barra verticale larga
// quanto il marcatore rimosso, così il contenuto resta allineato e le citazioni
// consecutive disegnano una barra continua.
func TestQuoteBar(t *testing.T) {
	cases := []struct {
		raw      string
		wantBar  string
		wantText string
	}{
		{"> citazione", "▌ ", "citazione"},
		{">citazione", "▌", "citazione"},
		{">", "▌", ""},
		{"> ", "▌ ", ""},
	}
	for _, c := range cases {
		segs := RenderLine(c.raw, false)
		if len(segs) == 0 || segs[0].Style != StyleQuoteBar {
			t.Errorf("RenderLine(%q): primo segmento = %+v, want StyleQuoteBar", c.raw, segs)
			continue
		}
		if segs[0].Text != c.wantBar {
			t.Errorf("RenderLine(%q): barra = %q, want %q", c.raw, segs[0].Text, c.wantBar)
		}
		// La barra occupa esattamente il marcatore rimosso: le celle del
		// prefisso coincidono con quelle del testo grezzo tolto.
		if got, want := utf8.RuneCountInString(segs[0].Text), PrefixWidth(c.raw); got != want {
			t.Errorf("RenderLine(%q): barra di %d celle, want %d (larghezza del prefisso rimosso)",
				c.raw, got, want)
		}
		joined := ""
		for _, s := range segs[1:] {
			joined += s.Text
		}
		if joined != c.wantText {
			t.Errorf("RenderLine(%q): testo = %q, want %q", c.raw, joined, c.wantText)
		}
	}
}

// TestIsQuote verifica il riconoscimento delle righe di citazione.
func TestIsQuote(t *testing.T) {
	for _, raw := range []string{">", "> x", ">x", ">  x"} {
		if !IsQuote(raw) {
			t.Errorf("IsQuote(%q) = false, want true", raw)
		}
	}
	for _, raw := range []string{"", "x", "x > y", "# > x", "- > x"} {
		if IsQuote(raw) {
			t.Errorf("IsQuote(%q) = true, want false", raw)
		}
	}
}

// TestQuoteInline verifica che il markdown inline dentro una citazione venga
// comunque riconosciuto.
func TestQuoteInline(t *testing.T) {
	segs := RenderLine("> con **grassetto**", false)
	if len(segs) < 3 {
		t.Fatalf("RenderLine = %+v, want barra, testo e grassetto", segs)
	}
	found := false
	for _, s := range segs {
		if s.Style == StyleBold && s.Text == "grassetto" {
			found = true
		}
	}
	if !found {
		t.Errorf("il grassetto dentro la citazione non è riconosciuto: %+v", segs)
	}
}
