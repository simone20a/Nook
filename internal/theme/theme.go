// Package theme raccoglie la palette condivisa dell'applicazione: gli stili
// pieni usati per il testo in evidenza, quelli attenuati usati per le righe
// non in focus, oltre agli stili di servizio (reverse, errore, toast, banner,
// selezione, testo attenuato e cestino).
package theme

import (
	lipgloss "github.com/charmbracelet/lipgloss"

	"nook/internal/markdown"
)

// ac costruisce un colore adaptive: Light per i terminali chiari, Dark per
// quelli scuri.
func ac(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// spec descrive uno stile del markdown in forma di dati: i colori sono indici
// ANSI 256 (stringa vuota = nessun colore), gli attributi sono flag. La tabella
// serve a generare sia gli stili pieni sia le scale attenuate e dissolte, senza
// duplicare a mano ogni variante.
type spec struct {
	light, dark     string // primo piano
	bgLight, bgDark string // sfondo
	bold            bool
	italic          bool
	strike          bool
	underline       bool
	faint           bool
}

// fullSpecs è la tabella degli stili pieni.
var fullSpecs = map[markdown.Style]spec{
	markdown.StyleHeading1:  {light: "33", dark: "69", bold: true},
	markdown.StyleHeading2:  {light: "40", dark: "75", bold: true},
	markdown.StyleHeading3:  {light: "31", dark: "81", bold: true},
	markdown.StyleHeading4:  {light: "67", dark: "110"},
	markdown.StyleHeading5:  {light: "97", dark: "146"},
	markdown.StyleHeading6:  {light: "95", dark: "145"},
	markdown.StyleBold:      {light: "55", dark: "141", bold: true},
	markdown.StyleItalic:    {light: "29", dark: "79", italic: true},
	markdown.StyleCode:      {light: "88", dark: "210", bgLight: "253", bgDark: "237"},
	markdown.StyleStrike:    {light: "94", dark: "180", strike: true},
	markdown.StyleLink:      {light: "25", dark: "38", underline: true},
	markdown.StyleImage:     {faint: true},
	markdown.StyleQuote:     {light: "250", dark: "246", italic: true},
	markdown.StyleQuoteBar:  {light: "103", dark: "60", bold: true},
	markdown.StyleBullet:    {light: "208", dark: "214"},
	markdown.StyleCheckbox:  {light: "65", dark: "71"},
	markdown.StyleFence:     {faint: true},
	markdown.StyleCodeBlock: {light: "248", dark: "245"},
	markdown.StyleNone:      {},
}

// dimSpecs è la tabella degli stili attenuati: le righe senza focus non usano
// solo Faint, ma un primo piano più spento.
var dimSpecs = map[markdown.Style]spec{
	markdown.StyleHeading1:  {light: "117", dark: "24", bold: true, faint: true},
	markdown.StyleHeading2:  {light: "122", dark: "28", bold: true, faint: true},
	markdown.StyleHeading3:  {light: "117", dark: "31", bold: true, faint: true},
	markdown.StyleHeading4:  {light: "145", dark: "58", faint: true},
	markdown.StyleHeading5:  {light: "147", dark: "92", faint: true},
	markdown.StyleHeading6:  {light: "147", dark: "92", faint: true},
	markdown.StyleBold:      {light: "60", dark: "103", bold: true},
	markdown.StyleItalic:    {light: "65", dark: "108", italic: true},
	markdown.StyleCode:      {light: "66", dark: "110", bgLight: "254", bgDark: "236"},
	markdown.StyleStrike:    {light: "67", dark: "102", strike: true},
	markdown.StyleLink:      {light: "68", dark: "59", underline: true},
	markdown.StyleImage:     {light: "250", dark: "241", faint: true},
	markdown.StyleQuote:     {light: "248", dark: "245", italic: true, faint: true},
	markdown.StyleQuoteBar:  {light: "145", dark: "240", bold: true},
	markdown.StyleBullet:    {light: "137", dark: "94", faint: true},
	markdown.StyleCheckbox:  {light: "108", dark: "65", faint: true},
	markdown.StyleFence:     {light: "250", dark: "241", faint: true},
	markdown.StyleCodeBlock: {light: "250", dark: "240", faint: true},
	markdown.StyleNone:      {light: "250", dark: "241", faint: true},
}

// Styles associa ogni stile del markdown alla sua resa piena.
var Styles = build(fullSpecs)

// DimStyles associa ogni stile del markdown alla sua resa attenuata: le righe
// che non hanno il focus non usano Faint, ma un Foreground più spento.
var DimStyles = build(dimSpecs)

// build traduce una tabella di spec in stili lipgloss.
func build(specs map[markdown.Style]spec) map[markdown.Style]lipgloss.Style {
	out := make(map[markdown.Style]lipgloss.Style, len(specs))
	for st, s := range specs {
		style := lipgloss.NewStyle()
		if s.light != "" || s.dark != "" {
			style = style.Foreground(ac(s.light, s.dark))
		}
		if s.bgLight != "" || s.bgDark != "" {
			style = style.Background(ac(s.bgLight, s.bgDark))
		}
		if s.bold {
			style = style.Bold(true)
		}
		if s.italic {
			style = style.Italic(true)
		}
		if s.strike {
			style = style.Strikethrough(true)
		}
		if s.underline {
			style = style.Underline(true)
		}
		if s.faint {
			style = style.Faint(true)
		}
		out[st] = style
	}
	return out
}

var (
	// Rev evidenzia il carattere sotto il cursore.
	Rev = lipgloss.NewStyle().Reverse(true)
	// Error è lo stile dei messaggi di errore.
	Error = lipgloss.NewStyle().Foreground(ac("124", "9"))
	// Toast è lo stile del messaggio temporaneo sovrapposto: testo in accento,
	// grassetto, senza sfondo né riquadro, coerente con il resto dell'interfaccia.
	Toast = lipgloss.NewStyle().Bold(true).Foreground(ac("25", "81"))
	// ToastError è lo stile del toast di errore, nello stesso formato ma in rosso.
	ToastError = lipgloss.NewStyle().Bold(true).Foreground(ac("124", "9"))
	// BannerAcc è lo stile dell'accento del banner.
	BannerAcc = lipgloss.NewStyle().Bold(true).Foreground(ac("25", "69"))
	// Selected è lo stile della riga selezionata.
	Selected = lipgloss.NewStyle().Bold(true)
	// DimText è lo stile del testo attenuato: l'attenuazione è solo nel colore.
	DimText = lipgloss.NewStyle().Foreground(ac("250", "241"))
	// TrashDim è lo stile delle note nel cestino, non selezionate.
	TrashDim = lipgloss.NewStyle().Foreground(ac("124", "88")).Faint(true)
	// TrashSel è lo stile della nota del cestino selezionata.
	TrashSel = lipgloss.NewStyle().Foreground(ac("124", "9")).Bold(true)
	// FolderDim è lo stile delle cartelle non selezionate: tinta spenta, così la
	// cartella selezionata si distingue anche senza sottolineatura.
	FolderDim = lipgloss.NewStyle().Foreground(ac("103", "60")).Bold(true)
	// FolderSel è lo stile della cartella selezionata: colore più acceso e
	// grassetto, senza sottolineatura.
	FolderSel = lipgloss.NewStyle().Foreground(ac("25", "117")).Bold(true)
	// Selection è lo stile del testo selezionato nell'editor.
	Selection = lipgloss.NewStyle().Foreground(ac("235", "232")).Background(ac("223", "238"))
)
