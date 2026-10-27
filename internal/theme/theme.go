// Package theme collects the application's shared palette: the full styles
// used for highlighted text, the dimmed ones used for lines that are not in
// focus, plus the utility styles (reverse, error, toast, banner, selection,
// dimmed text and trash).
package theme

import (
	lipgloss "github.com/charmbracelet/lipgloss"

	"nook/internal/markdown"
)

// ac builds an adaptive colour: Light for light terminals, Dark for dark
// ones.
func ac(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// spec describes a markdown style as data: colours are ANSI 256 indices
// (empty string = no colour), attributes are flags. The table is used to
// generate both the full styles and the dimmed and faded scales, without
// duplicating every variant by hand.
type spec struct {
	light, dark     string // foreground
	bgLight, bgDark string // background
	bold            bool
	italic          bool
	strike          bool
	underline       bool
	faint           bool
}

// fullSpecs is the table of full styles.
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

// dimSpecs is the table of dimmed styles: lines without focus do not just use
// Faint, they use a duller foreground.
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

// Styles maps every markdown style to its full rendering.
var Styles = build(fullSpecs)

// DimStyles maps every markdown style to its dimmed rendering: lines that do
// not have focus do not use Faint, but a duller Foreground.
var DimStyles = build(dimSpecs)

// build turns a spec table into lipgloss styles.
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
	// Rev highlights the character under the cursor.
	Rev = lipgloss.NewStyle().Reverse(true)
	// Error is the style of error messages.
	Error = lipgloss.NewStyle().Foreground(ac("124", "9"))
	// Toast is the style of the temporary overlaid message: accent text,
	// bold, no background or box, consistent with the rest of the interface.
	Toast = lipgloss.NewStyle().Bold(true).Foreground(ac("25", "81"))
	// ToastError is the style of the error toast, same format but in red.
	ToastError = lipgloss.NewStyle().Bold(true).Foreground(ac("124", "9"))
	// BannerAcc is the style of the banner accent.
	BannerAcc = lipgloss.NewStyle().Bold(true).Foreground(ac("25", "69"))
	// Selected is the style of the selected row.
	Selected = lipgloss.NewStyle().Bold(true)
	// DimText is the style of dimmed text: the dimming is in the colour only.
	DimText = lipgloss.NewStyle().Foreground(ac("250", "241"))
	// TrashDim is the style of unselected notes in the trash.
	TrashDim = lipgloss.NewStyle().Foreground(ac("124", "88")).Faint(true)
	// TrashSel is the style of the selected trash note.
	TrashSel = lipgloss.NewStyle().Foreground(ac("124", "9")).Bold(true)
	// FolderDim is the style of unselected folders: a muted tint, so the
	// selected folder stands out even without underlining.
	FolderDim = lipgloss.NewStyle().Foreground(ac("103", "60")).Bold(true)
	// FolderSel is the style of the selected folder: a brighter colour and
	// bold, no underline.
	FolderSel = lipgloss.NewStyle().Foreground(ac("25", "117")).Bold(true)
	// Selection is the style of selected text in the editor.
	Selection = lipgloss.NewStyle().Foreground(ac("235", "232")).Background(ac("223", "238"))
)
