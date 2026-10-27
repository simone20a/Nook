package app

import (
	"strings"
	"testing"

	ansi "github.com/charmbracelet/x/ansi"
)

// TestContentArea checks that the content area loses the margins only when
// the screen reaches the minimum thresholds.
func TestContentArea(t *testing.T) {
	if got := contentW(80); got != 76 {
		t.Errorf("contentW(80) = %d, want 76", got)
	}
	if got := contentH(24); got != 22 {
		t.Errorf("contentH(24) = %d, want 22", got)
	}
	if got := contentW(29); got != 29 {
		t.Errorf("contentW(29) = %d, want 29", got)
	}
	if got := contentH(9); got != 9 {
		t.Errorf("contentH(9) = %d, want 9", got)
	}
}

// TestTruncatePlain checks truncation of plain text to a known width.
func TestTruncatePlain(t *testing.T) {
	if got := truncatePlain("abcdefghij", 5); got != "abcd…" {
		t.Errorf("truncatePlain(%q, 5) = %q, want %q", "abcdefghij", got, "abcd…")
	}
	if got := truncatePlain("a", -1); got != "" {
		t.Errorf("truncatePlain(%q, -1) = %q, want %q", "a", got, "")
	}
}

// TestWithMarginsCount checks that withMargins raises the number of rows to the
// total h and that every row starts with marginX spaces.
func TestWithMarginsCount(t *testing.T) {
	const w, h = 80, 10

	line := strings.Repeat("x", contentW(w))
	lines := make([]string, 0, contentH(h))
	for i := 0; i < contentH(h); i++ {
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")

	got := withMargins(content, w, h)
	rows := strings.Split(got, "\n")
	if len(rows) != h {
		t.Fatalf("withMargins has %d rows, want %d", len(rows), h)
	}

	indent := strings.Repeat(" ", marginX)
	for i, row := range rows {
		if !strings.HasPrefix(row, indent) {
			t.Errorf("row %d = %q, want a prefix of %d spaces", i, row, marginX)
		}
	}
}

// TestListContentH checks that the height of the list rows leaves space
// for the banner and the toast row, with a minimum of one row.
func TestListContentH(t *testing.T) {
	cases := []struct {
		ch, bannerH, want int
	}{
		{22, 3, 18},
		{10, 1, 8},
		{2, 3, 1}, // below the minimum: one row remains
	}
	for _, c := range cases {
		if got := listContentH(c.ch, c.bannerH); got != c.want {
			t.Errorf("listContentH(%d, %d) = %d, want %d", c.ch, c.bannerH, got, c.want)
		}
	}
}

// TestListCardW checks the width of the card: at most 60 columns and never
// wider than the available space.
func TestListCardW(t *testing.T) {
	cases := []struct{ cw, want int }{
		{120, 60},
		{76, 60},
		{60, 60},
		{45, 45},
		{20, 20},
		{0, 0},
	}
	for _, c := range cases {
		if got := listCardW(c.cw); got != c.want {
			t.Errorf("listCardW(%d) = %d, want %d", c.cw, got, c.want)
		}
	}
}

// TestCardLeft checks the offset that centres the card on the width
// of the content area.
func TestCardLeft(t *testing.T) {
	cases := []struct {
		cw, cardW, want int
	}{
		{76, 60, 8},
		{120, 60, 30},
		{45, 45, 0},
		{20, 20, 0},
	}
	for _, c := range cases {
		if got := cardLeft(c.cw, c.cardW); got != c.want {
			t.Errorf("cardLeft(%d, %d) = %d, want %d", c.cw, c.cardW, got, c.want)
		}
	}
}

// newListTestScreen creates a store with three notes (body "two words") and the
// list screen loaded, with the middle note selected.
func newListTestScreen(t *testing.T, w, h int) listScreen {
	t.Helper()

	st := newKeysTestStore(t)
	for _, title := range []string{"first", "second", "third"} {
		n, err := st.Create(title)
		if err != nil {
			t.Fatalf("creating note %q: %v", title, err)
		}
		if err := st.Save(n.ID, title, "two words"); err != nil {
			t.Fatalf("saving note %q: %v", title, err)
		}
	}

	s := newListScreen(st, w, h)
	s.load()
	if len(s.notes) < 3 {
		t.Fatalf("the list has %d notes, want at least 3", len(s.notes))
	}
	// The middle note: entries of the plane remain above and below, so the separators
	// and the centring are really visible.
	s.selected = 1
	return s
}

// TestNoteLayoutRows checks the shape of the list plane: the
// selected note on two rows (title with bar and metadata), the others on one row
// with the title only, a separator row between notes and all rows as wide
// as the card.
func TestNoteLayoutRows(t *testing.T) {
	const w, h = 80, 24
	s := newListTestScreen(t, w, h)

	cw := contentW(w)
	cardW := listCardW(cw)
	plane, selFrom, selTo := s.listPlane(cw)

	// Two rows for each note (title and metadata) plus a separator row between
	// one note and the next.
	wantRows := 2*len(s.notes) + (len(s.notes) - 1)
	if len(plane) != wantRows {
		t.Fatalf("the plane has %d rows, want %d: %q", len(plane), wantRows, plane)
	}

	// Every row of the plane is exactly as wide as the card.
	for r, row := range plane {
		if got := ansi.StringWidth(ansi.Strip(row)); got != cardW {
			t.Errorf("row %d of the plane %d columns wide, want %d: %q", r, got, cardW, row)
		}
	}

	// The selected one is the only note in bold and in any case occupies two rows,
	// title and metadata.
	if got := selTo - selFrom + 1; got != 2 {
		t.Fatalf("the selection occupies %d rows (from %d to %d), want 2", got, selFrom, selTo)
	}
	sel := s.notes[s.selected]
	wantDetail := noteDetail(sel)
	if !strings.Contains(wantDetail, "2 words") {
		t.Fatalf("noteDetail = %q, want the count %q", wantDetail, "2 words")
	}
	if !strings.Contains(wantDetail, sel.UpdatedAt.Format("02/01/2006")) {
		t.Fatalf("noteDetail = %q, want the date %q", wantDetail, sel.UpdatedAt.Format("02/01/2006"))
	}

	// Expected plane, row by row: the title of each note and its metadata
	// right below, both centred in the card and without any mark on the left,
	// plus a separator row between the notes.
	wantPlane := make([]string, 0, wantRows)
	for i := range s.notes {
		wantPlane = append(wantPlane, s.notes[i].Title, noteDetail(s.notes[i]))
		if i < len(s.notes)-1 {
			wantPlane = append(wantPlane, strings.Repeat("─", cardW))
		}
	}

	separators := 0
	for r, want := range wantPlane {
		plain := ansi.Strip(plane[r])
		if strings.HasPrefix(want, "─") {
			if strings.TrimRight(plain, " ") != want {
				t.Errorf("separator row %d = %q, want %q", r, strings.TrimRight(plain, " "), want)
			}
			separators++
			if r == 0 || r == len(wantPlane)-1 {
				t.Errorf("separator row %d at the head or tail of the plane", r)
			}
			continue
		}
		// The text is the expected one, without prefixes, and starts after (cardW-width)/2
		// spaces: that is, it is centred in the card.
		if got := strings.TrimSpace(plain); got != want {
			t.Errorf("row %d of the plane = %q, want %q", r, got, want)
		}
		wantLead := (cardW - ansi.StringWidth(want)) / 2
		if got := leadingSpaces(plane[r]); got != wantLead {
			t.Errorf("row %d of the plane starts after %d spaces, want %d (text not centred): %q",
				r, got, wantLead, plain)
		}
	}
	if want := len(s.notes) - 1; separators != want {
		t.Errorf("the plane has %d separator rows, want %d", separators, want)
	}

	// Title and metadata of the selected one are on the rows of the range.
	if got := strings.TrimSpace(ansi.Strip(plane[selFrom])); got != sel.Title {
		t.Errorf("title row of the selected one = %q, want %q", got, sel.Title)
	}
	if got := strings.TrimSpace(ansi.Strip(plane[selTo])); got != wantDetail {
		t.Errorf("metadata row of the selected one = %q, want %q", got, wantDetail)
	}

	// The list window vertically centres the range of the selection:
	// the free rows above and below the selection differ by at most one.
	listH := listContentH(contentH(h), 1)
	start := listWindowStart(selFrom, selTo, listH)
	above := selFrom - start
	below := (start + listH - 1) - selTo
	if above < 0 || below < 0 {
		t.Fatalf("the selection [%d..%d] goes out of the window of %d rows starting at %d", selFrom, selTo, listH, start)
	}
	if above-below > 1 || below-above > 1 {
		t.Errorf("selection not centred: %d free rows above, %d below (window of %d rows)", above, below, listH)
	}

	window := listWindow(plane, selFrom, selTo, listH)
	if len(window) != listH {
		t.Fatalf("the window has %d rows, want %d", len(window), listH)
	}
	if got := strings.TrimSpace(ansi.Strip(window[above])); got != sel.Title {
		t.Errorf("window, row %d = %q, want the title %q of the selected one", above, got, sel.Title)
	}
	if got := strings.TrimSpace(ansi.Strip(window[above+1])); got != wantDetail {
		t.Errorf("window, row %d = %q, want the metadata %q of the selected one", above+1, got, wantDetail)
	}
	for r := 0; r < listH; r++ {
		i := start + r
		if i >= 0 && i < len(plane) {
			continue
		}
		if got := strings.TrimRight(ansi.Strip(window[r]), " "); got != "" {
			t.Errorf("window, row %d outside the plane = %q, want empty", r, got)
		}
	}
}

// leadingSpaces counts the leading spaces of a row, ignoring ANSI codes.
func leadingSpaces(row string) int {
	plain := ansi.Strip(row)
	return len(plain) - len(strings.TrimLeft(plain, " "))
}

// TestListCenteredHorizontally checks that the card rows are offset
// to the right by cardLeft columns (plus the margin): the note titles and the
// separator rows all start at the same column, except title and footer
// which stay centred on the whole content area.
func TestListCenteredHorizontally(t *testing.T) {
	cases := []struct{ w, h int }{
		{80, 24},  // card of 60 on 76 columns: offset 8 + margin 2
		{160, 24}, // card of 60 on 156 columns: offset 48 + margin 2
		{40, 24},  // full-width card: no offset, only the margin
	}
	for _, c := range cases {
		s := newListTestScreen(t, c.w, c.h)
		cw := contentW(c.w)
		cardW := listCardW(cw)
		wantIndent := 0
		if marginsActive(c.w, c.h) {
			wantIndent = marginX
		}
		cardCol := wantIndent + cardLeft(cw, cardW)

		rows := strings.Split(s.View(), "\n")
		titles := 0
		for _, row := range rows {
			plain := ansi.Strip(row)
			for _, n := range s.notes {
				// The title is centred in the card: it starts at cardCol plus half
				// of the free space.
				if i := strings.Index(plain, n.Title); i >= 0 {
					col := ansi.StringWidth(plain[:i])
					want := cardCol + (cardW-ansi.StringWidth(n.Title))/2
					if col != want {
						t.Errorf("w=%d: the title %q starts at column %d, want %d:\n%s",
							c.w, n.Title, col, want, s.View())
					}
					titles++
				}
			}
			if i := strings.Index(plain, "─"); i >= 0 {
				if col := ansi.StringWidth(plain[:i]); col != cardCol {
					t.Errorf("w=%d: the separator row starts at column %d, want %d:\n%s",
						c.w, col, cardCol, s.View())
				}
			}
		}
		if titles == 0 {
			t.Fatalf("w=%d: no title found in the view:\n%s", c.w, s.View())
		}

		// The card is inside the screen: card start + width do not exceed w.
		if limit := cardCol + cardW; limit > c.w {
			t.Errorf("w=%d: the card ends at column %d, beyond the edge %d", c.w, limit, c.w)
		}
	}
}

// TestListHitOutsideCard checks that clicks outside the width of the card
// hit no note, while a click inside it selects it.
func TestListHitOutsideCard(t *testing.T) {
	const w, h = 120, 24
	s := newListTestScreen(t, w, h)

	cw := contentW(w)
	left := marginX + cardLeft(cw, listCardW(cw))
	right := left + listCardW(cw)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			inside := x >= left && x < right
			if _, ok := s.listHitNote(x, y); ok && !inside {
				t.Fatalf("the click (%d,%d) outside the card [%d,%d) hit a note", x, y, left, right)
			}
		}
	}

	hit := false
	for y := 0; y < h && !hit; y++ {
		_, hit = s.listHitNote(left+2, y)
	}
	if !hit {
		t.Errorf("no click inside the card hit a note")
	}
}
