package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/theme"
)

// Border characters of the overlaid boxes: corners and sides.
const (
	boxTopLeft     = "┌"
	boxTopRight    = "┐"
	boxBottomLeft  = "└"
	boxBottomRight = "┘"
	boxHorizontal  = "─"
	boxVertical    = "│"
)

// maxPopupW is the maximum text width of a popup for a terminal width
// w: the border and padding of the box have to be subtracted from the content area.
// There are always at least 10 columns of text left.
func maxPopupW(w int) int {
	c := contentW(w) - 4
	if c < 10 {
		return 10
	}
	return c
}

// maxPopupH is the maximum text height of a popup for a terminal height
// h: the border and padding of the box have to be subtracted from the content area.
// There are always at least 3 rows of text left.
func maxPopupH(h int) int {
	c := contentH(h) - 4
	if c < 3 {
		return 3
	}
	return c
}

// centerPopup centres the box built by popup over base, on the area
// contentW(w) x contentH(h). base must have exactly contentH(h) contiguous
// rows, each contentW(w) wide; the rows not covered by the box
// stay unchanged.
//
// The box is always drawn: the minimum of 10 columns and 3 rows
// guaranteed by maxPopupW and maxPopupH always makes it possible to compose the border
// with the padding, so there is no area too small that would justify
// returning the base unchanged (the threshold is already applied inside the helpers,
// not on the raw values of contentW/contentH).
func centerPopup(base string, w, h int, popup string) string {
	cw, ch := contentW(w), contentH(h)
	rows := splitRows(base, ch)

	lines := strings.Split(popup, "\n")

	// Inner width of the box: the longest row of the popup, without
	// exceeding maxPopupW(w). Every row is then truncated to this size.
	textW := 0
	for _, l := range lines {
		if lw := ansi.StringWidth(l); lw > textW {
			textW = lw
		}
	}
	if limit := maxPopupW(w); textW > limit {
		textW = limit
	}

	// Inner height of the box: the rows of the popup, without exceeding
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

// toastStyle chooses the toast style: the error one for messages that
// start with "error", the normal one for everything else (confirmations like
// "saved" or "note created").
func toastStyle(msg string) lipgloss.Style {
	if strings.HasPrefix(msg, "error") {
		return theme.ToastError
	}
	return theme.Toast
}

// toastLine renders msg as a single toast row: text with the toast style,
// truncated to the maximum popup width and centred with spaces over contentW(w)
// columns.
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

// overlayAt overlays block (rows separated by \n) on base starting at
// row,col, cutting everything that goes outside the area contentW(w) x contentH(h).
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

// splitRows splits base into the rows of the content area: it removes a possible
// trailing newline and pads the missing ones with empty rows.
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

// spliceRow replaces in row the segment that starts at col and occupies cutW
// columns with segment, leaving the outer parts unchanged. row is brought
// to the width w before the cut, so the segment falls at col even when
// the row was shorter.
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

// padPlain brings s to the visual width w by adding spaces on the right.
func padPlain(s string, w int) string {
	if cur := ansi.StringWidth(s); cur < w {
		return s + strings.Repeat(" ", w-cur)
	}
	return s
}
