package editor

import (
	"strings"

	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/markdown"
	"nook/internal/theme"
)

// View draws the editor: one row for each screen row, separated by "\n".
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

// edFooterHint is the footer text: the essential keys.
const edFooterHint = "ctrl+s save · esc exit · ctrl+t format · ctrl+/ help"

// Footer renders the last row of the editor screen: a single row
// truncated to the width, in dimmed colour via theme.DimText and padded
// with spaces on the right. It is not part of View: the screen reserves a
// row for it, plus an empty one above, and gives the editor a content height
// reduced by two.
func (e Editor) Footer() string {
	if e.width <= 0 {
		return ""
	}
	plain := truncate(edFooterHint, e.width)
	pad := e.width - ansi.StringWidth(plain)
	if pad < 0 {
		pad = 0
	}
	return theme.DimText.Render(plain) + strings.Repeat(" ", pad)
}

// renderRow draws a screen row: the focus row shows the raw text with the cursor,
// the others show the markdown segments rendered with the dimmed palette
// of theme and truncated to the editor width. The dimming of the
// rows without focus is fixed: it does not depend on the distance from the centre.
func (e Editor) renderRow(r int) string {
	if r == e.focusRow() {
		return e.renderFocusRow()
	}
	idx := e.docIndexForRow(r)
	if idx < 0 || idx >= len(e.lines) {
		return ""
	}
	raw := e.lines[idx]
	inside := isInsideFence(e.lines, idx)
	segs := markdown.RenderLine(raw, inside)

	// A heading is highlighted: its background colours the text and a few
	// spaces after it, and the inline styles inside it keep the background.
	headStyle, isHead := markdown.HeadingStyle(raw)
	isHead = isHead && !inside

	// Rows without focus show rendered markdown; if the selection crosses
	// them, the selected part is highlighted.
	selFrom, selTo, hasSel := e.selectionOnLine(idx)
	fromByte, toByte := runeRangeToBytes(raw, selFrom, selTo)

	var b strings.Builder
	for _, seg := range segs {
		st := theme.DimStyles[seg.Style]
		if isHead {
			st = st.Inherit(theme.DimStyles[headStyle])
		}
		if hasSel {
			b.WriteString(renderSegmentWithSelection(seg, fromByte, toByte, st))
			continue
		}
		b.WriteString(st.Render(seg.Text))
	}
	if n := len(segs); n > 0 {
		fill := e.width - ansi.StringWidth(b.String())
		if isHead {
			// The highlight ends a little after the text, not at the edge of
			// the screen; the title itself keeps its place.
			if fill > headingPad {
				fill = headingPad
			}
			if fill > 0 {
				b.WriteString(theme.DimStyles[headStyle].Render(strings.Repeat(" ", fill)))
			}
		} else {
			b.WriteString(blockFill(segs[n-1].Style, fill))
		}
	}
	return truncate(b.String(), e.width)
}

// headingPad is the number of spaces that the highlight of a heading
// extends past the end of its text.
const headingPad = 2

// blockFill extends rules and code fences with "─" up to the width of the
// editor, and code lines with spaces, so that a code block reads as a panel.
// It returns "" for every other style or when there is no room left.
func blockFill(sty markdown.Style, n int) string {
	if n <= 0 {
		return ""
	}
	switch sty {
	case markdown.StyleRule, markdown.StyleFence:
		return theme.DimStyles[sty].Render(strings.Repeat("─", n))
	case markdown.StyleCodeBlock:
		return theme.DimStyles[sty].Render(strings.Repeat(" ", n))
	}
	return ""
}

// truncate shortens s to the width w in cells, adding "…" if it cuts.
// It is aware of wide cells and ANSI sequences.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// isFenceLine reports whether the line opens or closes a code block.
func isFenceLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// isInsideFence reports whether line idx is inside a code block: it counts the
// fence lines that precede it, an odd number means an open fence.
func isInsideFence(lines []string, idx int) bool {
	count := 0
	for i := 0; i < idx && i < len(lines); i++ {
		if isFenceLine(lines[i]) {
			count++
		}
	}
	return count%2 == 1
}

// renderFocusRow draws the cursor's row: raw text, horizontal window
// starting from hScroll, cursor highlighted in reverse. The block prefix
// is compensated with spaces so as not to shift the text on screen.
// All the text shown is rendered with theme.Selected (bold), except the cursor
// character, which stays in reverse.
func (e Editor) renderFocusRow() string {
	raw := ""
	if e.cursorLine >= 0 && e.cursorLine < len(e.lines) {
		raw = e.lines[e.cursorLine]
	}

	// The removed block prefix is compensated: spaces for the markers, but
	// the vertical bar for quotes, so the bar stays continuous even
	// across the Focus Line.
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
	selectedStyle := false // true when the current text is inside the selection
	pending := false       // true when there is text to render
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
