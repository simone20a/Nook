package markdown

import (
	"testing"
	"unicode/utf8"
)

// checkSegments compares the expected segments with those produced by the renderer,
// checking the number of segments, the text and the style of each.
func checkSegments(t *testing.T, raw string, got, want []Segment) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("RenderLine(%q): %d segments, want %d\ngot  = %#v\nwant = %#v",
			raw, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i].Text != want[i].Text {
			t.Errorf("RenderLine(%q) segment %d: text = %q, want %q",
				raw, i, got[i].Text, want[i].Text)
		}
		if got[i].Style != want[i].Style {
			t.Errorf("RenderLine(%q) segment %d: style = %v, want %v",
				raw, i, got[i].Style, want[i].Style)
		}
	}
}

// TestRenderLine checks the exact cases of the specification: block prefixes,
// inline replacements, fences and lines inside a code block.
func TestRenderLine(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		insideFence bool
		want        []Segment
	}{
		{
			name: "heading h1",
			raw:  "# Title",
			want: []Segment{{Text: "Title", Style: StyleHeading1}},
		},
		{
			name: "heading h3",
			raw:  "### Sub",
			want: []Segment{{Text: "Sub", Style: StyleHeading3}},
		},
		{
			name: "bold and italic",
			raw:  "**bold** and *italic*",
			want: []Segment{
				{Text: "bold", Style: StyleBold},
				{Text: " and ", Style: StyleNone},
				{Text: "italic", Style: StyleItalic},
			},
		},
		{
			name: "code and strikethrough",
			raw:  "`code` and ~~strike~~",
			want: []Segment{
				{Text: "code", Style: StyleCode},
				{Text: " and ", Style: StyleNone},
				{Text: "strike", Style: StyleStrike},
			},
		},
		{
			name: "link",
			raw:  "[link](https://x.it)",
			want: []Segment{{Text: "link", Style: StyleLink}},
		},
		{
			name: "image",
			raw:  "![alt](img.png)",
			want: []Segment{{Text: "alt", Style: StyleImage}},
		},
		{
			name: "quote",
			raw:  "> quote",
			want: []Segment{
				{Text: "▌ ", Style: StyleQuoteBar},
				{Text: "quote", Style: StyleQuote},
			},
		},
		{
			name: "list",
			raw:  "- item",
			want: []Segment{
				{Text: "• ", Style: StyleBullet},
				{Text: "item", Style: StyleNone},
			},
		},
		{
			name: "checked checkbox",
			raw:  "- [x] done",
			want: []Segment{
				{Text: "☑ ", Style: StyleCheckbox},
				{Text: "done", Style: StyleNone},
			},
		},
		{
			name: "unchecked checkbox",
			raw:  "- [ ] to do",
			want: []Segment{
				{Text: "☐ ", Style: StyleCheckbox},
				{Text: "to do", Style: StyleNone},
			},
		},
		{
			name: "opening fence",
			raw:  "```go",
			want: []Segment{{Text: "┌─ go ", Style: StyleFence}},
		},
		{
			name:        "closing fence",
			raw:         "```",
			insideFence: true,
			want:        []Segment{{Text: "└─", Style: StyleFence}},
		},
		{
			name: "opening fence without language",
			raw:  "```",
			want: []Segment{{Text: "┌─", Style: StyleFence}},
		},
		{
			name: "rule dashes",
			raw:  "---",
			want: []Segment{{Text: "───", Style: StyleRule}},
		},
		{
			name: "rule spaced stars",
			raw:  "* * *",
			want: []Segment{{Text: "───", Style: StyleRule}},
		},
		{
			name: "two dashes are text",
			raw:  "--",
			want: []Segment{{Text: "--", Style: StyleNone}},
		},
		{
			name:        "rule inside a fence is code",
			raw:         "---",
			insideFence: true,
			want:        []Segment{{Text: "---", Style: StyleCodeBlock}},
		},
		{
			name: "simple line",
			raw:  "simple line",
			want: []Segment{{Text: "simple line", Style: StyleNone}},
		},
		{
			name:        "inside the fence",
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

// TestSegmentRawOffsets checks that every segment of "a **b** *c*" reports the
// byte offsets of the stretch of source it renders, with RawTo excluded: plain
// text occupies its own bytes, inline content the bytes between the delimiters.
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
			t.Errorf("RenderLine(%q) segment %d: offset = %d..%d, want %d..%d",
				raw, i, got[i].RawFrom, got[i].RawTo, want[i].RawFrom, want[i].RawTo)
			continue
		}
		// The offsets must cut out exactly the text of the segment.
		if sliced := raw[got[i].RawFrom:got[i].RawTo]; sliced != got[i].Text {
			t.Errorf("RenderLine(%q) segmento %d: raw[%d:%d] = %q, want %q",
				raw, i, got[i].RawFrom, got[i].RawTo, sliced, got[i].Text)
		}
	}
}

// TestMapCellToRaw checks the mapping display cell → offset in
// bytes in the raw text for "**ab** cd": the cells are those of the rendered line, which does not
// show the delimiters, so after "ab" the mapping skips the two trailing "*".
func TestMapCellToRaw(t *testing.T) {
	const raw = "**ab** cd"
	segs := RenderLine(raw, false)

	// The rendered line is "ab cd": 'a' → 2, 'b' → 3, then the space cell → 6
	// (the two closing "*" occupy no cells), 'c' → 7, 'd' → 8. Cell 5
	// falls beyond all the segments and equals the total length of the raw text (9).
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

// TestPrefixWidth checks the width in cells of the removed block prefix.
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
		{"---", 0},
		{"* * *", 0},
		{"```go", 0},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if got := PrefixWidth(tc.raw); got != tc.want {
				t.Errorf("PrefixWidth(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

// TestQuoteBar checks that the ">" marker becomes a vertical bar as wide
// as the removed marker, so the content stays aligned and consecutive
// quotes draw a continuous bar.
func TestQuoteBar(t *testing.T) {
	cases := []struct {
		raw      string
		wantBar  string
		wantText string
	}{
		{"> quote", "▌ ", "quote"},
		{">quote", "▌", "quote"},
		{">", "▌", ""},
		{"> ", "▌ ", ""},
	}
	for _, c := range cases {
		segs := RenderLine(c.raw, false)
		if len(segs) == 0 || segs[0].Style != StyleQuoteBar {
			t.Errorf("RenderLine(%q): first segment = %+v, want StyleQuoteBar", c.raw, segs)
			continue
		}
		if segs[0].Text != c.wantBar {
			t.Errorf("RenderLine(%q): bar = %q, want %q", c.raw, segs[0].Text, c.wantBar)
		}
		// The bar occupies exactly the removed marker: the cells of the
		// prefix match those of the raw text removed.
		if got, want := utf8.RuneCountInString(segs[0].Text), PrefixWidth(c.raw); got != want {
			t.Errorf("RenderLine(%q): bar of %d cells, want %d (width of the removed prefix)",
				c.raw, got, want)
		}
		joined := ""
		for _, s := range segs[1:] {
			joined += s.Text
		}
		if joined != c.wantText {
			t.Errorf("RenderLine(%q): text = %q, want %q", c.raw, joined, c.wantText)
		}
	}
}

// TestIsQuote checks the recognition of quote lines.
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

// TestQuoteInline checks that inline markdown inside a quote is
// still recognised.
func TestQuoteInline(t *testing.T) {
	segs := RenderLine("> with **bold**", false)
	if len(segs) < 3 {
		t.Fatalf("RenderLine = %+v, want bar, text and bold", segs)
	}
	found := false
	for _, s := range segs {
		if s.Style == StyleBold && s.Text == "bold" {
			found = true
		}
	}
	if !found {
		t.Errorf("bold inside the quote is not recognised: %+v", segs)
	}
}
