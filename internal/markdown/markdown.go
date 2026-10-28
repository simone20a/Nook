// Package markdown breaks a line of markdown into styled text segments.
// The package knows nothing about colours or ANSI codes: it only decides the style
// of each piece of text, leaving the rendering to the caller.
package markdown

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Style identifies the style of a Segment.
type Style int

// Styles recognised by the renderer. StyleHeading1..StyleHeading6 are
// contiguous: the style of a heading is StyleHeading1 + (level - 1).
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
	// StyleQuoteBar is the segment of the vertical bar that runs alongside a
	// quote; the text of the quote uses StyleQuote.
	StyleQuoteBar
	// StyleRule is a horizontal rule (---, ***, ___).
	StyleRule
	// StyleTableBorder is a border of a table (│, ├─┼─┤).
	StyleTableBorder
	// StyleTableHead is the text of the header row of a table.
	StyleTableHead
)

// Segment is a portion of a line with its style.
//
// RawFrom and RawTo are byte offsets into the raw text passed to
// RenderLine, with RawTo exclusive: they mark the stretch of source that the
// segment renders. For plain text it is the exact span of its own bytes, for
// inline content it is the span of the content between the delimiters
// (delimiters excluded), for synthesised segments (☐/☑, •) it is the span of the
// raw marker they replace.
type Segment struct {
	Text    string
	Style   Style
	RawFrom int
	RawTo   int
}

var (
	// Heading: 1-6 "#" followed by at least one space or tab.
	headingRe = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)
	// Quote: ">" optionally followed by a space or tab.
	quoteRe = regexp.MustCompile(`^>[ \t]?(.*)$`)
	// Checkbox: "- [ ]" or "- [x]"/"- [X]" (evaluated before lists).
	checkboxRe = regexp.MustCompile(`^(\s*)- \[([ xX])\](.*)$`)
	// Unordered list: -, * or + followed by at least one space.
	bulletRe = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	// Ordered list: only used to compute the width of the prefix.
	orderedRe = regexp.MustCompile(`^(\s*)\d+\.\s+`)

	// Inline replacements, in decreasing order of priority.
	imageRe            = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	linkRe             = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	boldRe             = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	strikeRe           = regexp.MustCompile(`~~([^~]+)~~`)
	codeRe             = regexp.MustCompile("`([^`]+)`")
	italicStarRe       = regexp.MustCompile(`\*([^*]+)\*`)
	italicUnderscoreRe = regexp.MustCompile(`_([^_]+)_`)
)

// inlinePattern associates an inline pattern with the style of its content.
type inlinePattern struct {
	re  *regexp.Regexp
	sty Style
}

// inlinePatterns is ordered by priority: at equal position the first one wins.
var inlinePatterns = []inlinePattern{
	{imageRe, StyleImage},
	{linkRe, StyleLink},
	{boldRe, StyleBold},
	{strikeRe, StyleStrike},
	{codeRe, StyleCode},
	{italicStarRe, StyleItalic},
	{italicUnderscoreRe, StyleItalic},
}

// isFenceLine reports whether the line opens or closes a code block.
func isFenceLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// isRule reports whether the line is a horizontal rule: at least three
// "-", "*" or "_" (all the same), optionally separated by spaces or tabs.
func isRule(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" || strings.IndexByte("-*_", t[0]) < 0 {
		return false
	}
	n := 0
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case t[0]:
			n++
		case ' ', '\t':
		default:
			return false
		}
	}
	return n >= 3
}

// fenceLabel returns the text of the line that opens or closes a code
// block: the language after the backticks, trimmed.
func fenceLabel(raw string) string {
	t := strings.TrimSpace(raw)
	return strings.TrimSpace(strings.TrimLeft(t, "`~"))
}

// RenderLine breaks a markdown line into its styled segments. When
// insideFence is true the line belongs to a code block: it is returned
// untouched, without any replacement. A fence line becomes the top
// (┌─ language) or bottom (└─) edge of the block, and a horizontal rule
// becomes a line of "─"; the caller extends both to the width of the screen.
func RenderLine(raw string, insideFence bool) []Segment {
	if isFenceLine(raw) {
		text := "└─"
		if !insideFence {
			text = "┌─"
			if lang := fenceLabel(raw); lang != "" {
				text += " " + lang + " "
			}
		}
		return []Segment{{Text: text, Style: StyleFence, RawFrom: 0, RawTo: len(raw)}}
	}
	if insideFence {
		return []Segment{{Text: raw, Style: StyleCodeBlock, RawFrom: 0, RawTo: len(raw)}}
	}
	if isRule(raw) {
		return []Segment{{Text: "───", Style: StyleRule, RawFrom: 0, RawTo: len(raw)}}
	}

	prefix, rest, base, off := splitPrefix(raw)
	segments := make([]Segment, 0, 4)
	if prefix != nil {
		segments = append(segments, *prefix)
	}
	return append(segments, renderInline(rest, base, off)...)
}

// splitPrefix recognises the block prefix of raw and returns the segment already
// emitted for the prefix, if any (with its offsets in the raw text), the text
// left to tokenise, the base style of that text and the byte offset of
// rest inside raw. The content always starts after the removed prefix, so
// the offset of rest is len(raw)-len(rest).
func splitPrefix(raw string) (prefix *Segment, rest string, base Style, off int) {
	rest, base, off = raw, StyleNone, 0

	// 1. Heading.
	if m := headingRe.FindStringSubmatch(rest); m != nil {
		return nil, m[2], Style(int(StyleHeading1) + len(m[1]) - 1), len(raw) - len(m[2])
	}
	// 2. Quote. The ">" marker (and the optional space) becomes a vertical bar
	// as wide as the removed marker, so the text stays
	// aligned between the Focus Line and the rendered lines, and consecutive
	// quotes draw a continuous bar.
	if m := quoteRe.FindStringSubmatch(rest); m != nil {
		end := len(raw) - len(m[1])
		return &Segment{Text: quoteBar(end), Style: StyleQuoteBar, RawFrom: 0, RawTo: end},
			m[1], StyleQuote, end
	}
	// 3. Checkbox, before lists. The synthesised segment covers the span of the
	// raw marker it replaces, separator included.
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
	// 4. Unordered list.
	if m := bulletRe.FindStringSubmatch(rest); m != nil {
		end := len(raw) - len(m[2])
		return &Segment{Text: m[1] + "• ", Style: StyleBullet, RawFrom: 0, RawTo: end},
			m[2], StyleNone, end
	}
	// 5. No prefix.
	return nil, raw, StyleNone, 0
}

// HeadingStyle reports whether raw is a heading line and, if so, the style of
// its level.
func HeadingStyle(raw string) (Style, bool) {
	if m := headingRe.FindStringSubmatch(raw); m != nil {
		return Style(int(StyleHeading1) + len(m[1]) - 1), true
	}
	return StyleNone, false
}

// QuoteBar is the character of the vertical bar that runs alongside quotes.
const QuoteBar = "▌"

// quoteBar builds the vertical bar width bytes wide, that is as many bytes
// as the removed ">" marker took: the bar plus one space for every byte
// after the first. So "> text" becomes "▌ text" and ">text" becomes
// "▌text", keeping the content aligned with the other lines.
func quoteBar(width int) string {
	if width <= 0 {
		return ""
	}
	return QuoteBar + strings.Repeat(" ", width-1)
}

// IsQuote reports whether raw is a quote line (it starts with ">").
func IsQuote(raw string) bool {
	return quoteRe.MatchString(raw)
}

// renderInline tokenises rest, giving the base style to ordinary text and the
// specific style to the content of every inline match. off is the byte offset of
// rest inside the raw line: the offsets of the produced segments derive from it.
func renderInline(rest string, base Style, off int) []Segment {
	var segments []Segment
	for rest != "" {
		bestIdx, bestLoc := -1, []int(nil)

		// Leftmost match; at equal position the first pattern wins.
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
		// The inline content covers the bytes between the delimiters, excluded.
		segments = appendText(segments, rest[bestLoc[2]:bestLoc[3]], inlinePatterns[bestIdx].sty, off+bestLoc[2])
		rest = rest[bestLoc[1]:]
		off += bestLoc[1]
	}
	return segments
}

// appendText adds a segment, skipping empty text. off is the byte offset of
// text inside the raw line; ordinary text is copied intact, so the
// segment occupies exactly its own bytes.
func appendText(segments []Segment, text string, sty Style, off int) []Segment {
	if text == "" {
		return segments
	}
	return append(segments, Segment{Text: text, Style: sty, RawFrom: off, RawTo: off + len(text)})
}

// MapCellToRaw converts a column in display cells into the byte column of the
// raw line the segments were produced from. If cell falls beyond all the
// segments it returns the total length of the raw line.
//
// assumed: rune = cell in the current renderer
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
			// The segment renders itself byte for byte only when the text
			// matches the raw span; synthesised segments (☐/☑, •)
			// stand for the marker they replace.
			if len(s.Text) != s.RawTo-s.RawFrom {
				return s.RawFrom
			}
			return s.RawFrom + byteOffsetOfRune(s.Text, cell-col)
		}
		col += n
	}
	return total
}

// byteOffsetOfRune returns the byte offset of the rune at index idx inside
// s. Past the last rune it returns len(s).
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

// PrefixWidth returns how many cells the block prefix removed from the line
// occupies. All recognised prefixes are ASCII, so the number of
// runes equals the number of cells.
func PrefixWidth(raw string) int {
	if isRule(raw) || isFenceLine(raw) {
		return 0
	}
	prefix, rest, _, _ := splitPrefix(raw)
	removed := utf8.RuneCountInString(raw) - utf8.RuneCountInString(rest)
	if prefix != nil || removed > 0 {
		return removed
	}
	// Ordered list: not removed by RenderLine, but its prefix has to be measured.
	if m := orderedRe.FindStringSubmatch(raw); m != nil {
		return utf8.RuneCountInString(m[0])
	}
	return 0
}
