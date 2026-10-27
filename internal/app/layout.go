package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
)

// Geometry of the application margins: marginX columns and marginY rows
// on all sides, no borders. Below the smallW/smallH thresholds the margins are
// zeroed so as not to squeeze the content.
const (
	marginX = 2
	marginY = 1
	smallW  = 30
	smallH  = 10
)

// marginsActive reports whether the screen is large enough for the margins.
func marginsActive(w, h int) bool { return w >= smallW && h >= smallH }

// contentW is the width of the content area for a width w.
func contentW(w int) int {
	if w >= smallW {
		return w - marginX*2
	}
	return w
}

// contentH is the height of the content area for a height h.
func contentH(h int) int {
	if h >= smallH {
		return h - marginY*2
	}
	return h
}

// truncatePlain truncates s to the width w, adding "…" if needed.
func truncatePlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// footerLine renders text as a single footer row w columns wide: the text
// is truncated to w, rendered with the style st and padded with spaces on the right.
func footerLine(text string, w int, st lipgloss.Style) string {
	if w <= 0 {
		return ""
	}
	plain := truncatePlain(text, w)
	return st.Render(plain) + strings.Repeat(" ", w-ansi.StringWidth(plain))
}

// withMargins applies the margins to the content of the content area.
// The content must have exactly contentH(h) rows, each already contentW(w)
// wide; with inactive margins the content is returned unchanged.
func withMargins(content string, w, h int) string {
	if !marginsActive(w, h) {
		return content
	}
	return lipgloss.NewStyle().Padding(marginY, marginX).Render(content)
}
