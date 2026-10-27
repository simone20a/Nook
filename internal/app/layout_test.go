package app

import (
	"strings"
	"testing"

	ansi "github.com/charmbracelet/x/ansi"
)

// TestContentArea verifica che l'area contenuto perda i margini solo quando
// lo schermo raggiunge le soglie minime.
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

// TestTruncatePlain verifica il troncamento a larghezza nota del testo piano.
func TestTruncatePlain(t *testing.T) {
	if got := truncatePlain("abcdefghij", 5); got != "abcd…" {
		t.Errorf("truncatePlain(%q, 5) = %q, want %q", "abcdefghij", got, "abcd…")
	}
	if got := truncatePlain("a", -1); got != "" {
		t.Errorf("truncatePlain(%q, -1) = %q, want %q", "a", got, "")
	}
}

// TestWithMarginsCount verifica che withMargins alzi il numero di righe al
// totale h e che ogni riga cominci con marginX spazi.
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
		t.Fatalf("withMargins ha %d righe, want %d", len(rows), h)
	}

	indent := strings.Repeat(" ", marginX)
	for i, row := range rows {
		if !strings.HasPrefix(row, indent) {
			t.Errorf("riga %d = %q, want prefisso di %d spazi", i, row, marginX)
		}
	}
}

// TestListContentH verifica che l'altezza delle righe della lista lasci spazio
// al banner e alla riga del toast, con il minimo di una riga.
func TestListContentH(t *testing.T) {
	cases := []struct {
		ch, bannerH, want int
	}{
		{22, 3, 18},
		{10, 1, 8},
		{2, 3, 1}, // sotto il minimo: resta una riga
	}
	for _, c := range cases {
		if got := listContentH(c.ch, c.bannerH); got != c.want {
			t.Errorf("listContentH(%d, %d) = %d, want %d", c.ch, c.bannerH, got, c.want)
		}
	}
}

// TestListCardW verifica la larghezza della card: al massimo 60 colonne e mai
// più larga dello spazio disponibile.
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

// TestCardLeft verifica lo scostamento che centra la card sulla larghezza
// dell'area contenuto.
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

// newListTestScreen crea uno store con tre note (corpo "due parole") e la
// schermata lista caricata, con la nota di mezzo selezionata.
func newListTestScreen(t *testing.T, w, h int) listScreen {
	t.Helper()

	st := newKeysTestStore(t)
	for _, title := range []string{"prima", "seconda", "terza"} {
		n, err := st.Create(title)
		if err != nil {
			t.Fatalf("creazione nota %q: %v", title, err)
		}
		if err := st.Save(n.ID, title, "due parole"); err != nil {
			t.Fatalf("salvataggio nota %q: %v", title, err)
		}
	}

	s := newListScreen(st, w, h)
	s.load()
	if len(s.notes) < 3 {
		t.Fatalf("la lista ha %d note, attese almeno 3", len(s.notes))
	}
	// La nota di mezzo: sopra e sotto restano voci del piano, così i separatori
	// e la centratura si vedono davvero.
	s.selected = 1
	return s
}

// TestNoteLayoutRows verifica la forma del piano della lista: la nota
// selezionata su due righe (titolo con barra e metadati), le altre su una riga
// con il solo titolo, una riga di separazione fra le note e tutte le righe larghe
// quanto la card.
func TestNoteLayoutRows(t *testing.T) {
	const w, h = 80, 24
	s := newListTestScreen(t, w, h)

	cw := contentW(w)
	cardW := listCardW(cw)
	plane, selFrom, selTo := s.listPlane(cw)

	// Due righe per ogni nota (titolo e metadati) più una riga di separazione fra
	// una nota e la successiva.
	wantRows := 2*len(s.notes) + (len(s.notes) - 1)
	if len(plane) != wantRows {
		t.Fatalf("il piano ha %d righe, want %d: %q", len(plane), wantRows, plane)
	}

	// Ogni riga del piano è larga esattamente quanto la card.
	for r, row := range plane {
		if got := ansi.StringWidth(ansi.Strip(row)); got != cardW {
			t.Errorf("riga %d del piano larga %d colonne, want %d: %q", r, got, cardW, row)
		}
	}

	// La selezionata è la sola nota in grassetto e occupa comunque due righe,
	// titolo e metadati.
	if got := selTo - selFrom + 1; got != 2 {
		t.Fatalf("la selezione occupa %d righe (da %d a %d), want 2", got, selFrom, selTo)
	}
	sel := s.notes[s.selected]
	wantDetail := noteDetail(sel)
	if !strings.Contains(wantDetail, "2 parole") {
		t.Fatalf("noteDetail = %q, atteso il conteggio %q", wantDetail, "2 parole")
	}
	if !strings.Contains(wantDetail, sel.UpdatedAt.Format("02/01/2006")) {
		t.Fatalf("noteDetail = %q, attesa la data %q", wantDetail, sel.UpdatedAt.Format("02/01/2006"))
	}

	// Piano atteso, riga per riga: il titolo di ogni nota e i suoi metadati
	// subito sotto, entrambi centrati nella card e senza alcun segno a sinistra,
	// più una riga di separazione fra le note.
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
				t.Errorf("riga di separazione %d = %q, want %q", r, strings.TrimRight(plain, " "), want)
			}
			separators++
			if r == 0 || r == len(wantPlane)-1 {
				t.Errorf("riga di separazione %d in testa o in coda al piano", r)
			}
			continue
		}
		// Il testo è quello atteso, senza prefissi, e comincia dopo (cardW-larghezza)/2
		// spazi: cioè è centrato nella card.
		if got := strings.TrimSpace(plain); got != want {
			t.Errorf("riga %d del piano = %q, want %q", r, got, want)
		}
		wantLead := (cardW - ansi.StringWidth(want)) / 2
		if got := leadingSpaces(plane[r]); got != wantLead {
			t.Errorf("riga %d del piano comincia dopo %d spazi, want %d (testo non centrato): %q",
				r, got, wantLead, plain)
		}
	}
	if want := len(s.notes) - 1; separators != want {
		t.Errorf("il piano ha %d righe di separazione, want %d", separators, want)
	}

	// Titolo e metadati della selezionata stanno sulle righe dell'intervallo.
	if got := strings.TrimSpace(ansi.Strip(plane[selFrom])); got != sel.Title {
		t.Errorf("riga titolo della selezionata = %q, want %q", got, sel.Title)
	}
	if got := strings.TrimSpace(ansi.Strip(plane[selTo])); got != wantDetail {
		t.Errorf("riga metadati della selezionata = %q, want %q", got, wantDetail)
	}

	// La finestra della lista centra verticalmente l'intervallo della selezione:
	// le righe libere sopra e sotto la selezione differiscono al più di una.
	listH := listContentH(contentH(h), 1)
	start := listWindowStart(selFrom, selTo, listH)
	above := selFrom - start
	below := (start + listH - 1) - selTo
	if above < 0 || below < 0 {
		t.Fatalf("la selezione [%d..%d] esce dalla finestra di %d righe che parte da %d", selFrom, selTo, listH, start)
	}
	if above-below > 1 || below-above > 1 {
		t.Errorf("selezione non centrata: %d righe libere sopra, %d sotto (finestra di %d righe)", above, below, listH)
	}

	window := listWindow(plane, selFrom, selTo, listH)
	if len(window) != listH {
		t.Fatalf("la finestra ha %d righe, want %d", len(window), listH)
	}
	if got := strings.TrimSpace(ansi.Strip(window[above])); got != sel.Title {
		t.Errorf("finestra, riga %d = %q, want il titolo %q della selezionata", above, got, sel.Title)
	}
	if got := strings.TrimSpace(ansi.Strip(window[above+1])); got != wantDetail {
		t.Errorf("finestra, riga %d = %q, want i metadati %q della selezionata", above+1, got, wantDetail)
	}
	for r := 0; r < listH; r++ {
		i := start + r
		if i >= 0 && i < len(plane) {
			continue
		}
		if got := strings.TrimRight(ansi.Strip(window[r]), " "); got != "" {
			t.Errorf("finestra, riga %d fuori dal piano = %q, attesa vuota", r, got)
		}
	}
}

// leadingSpaces conta gli spazi iniziali di una riga, ignorando i codici ANSI.
func leadingSpaces(row string) int {
	plain := ansi.Strip(row)
	return len(plain) - len(strings.TrimLeft(plain, " "))
}

// TestListCenteredHorizontally verifica che le righe della card siano scostate
// a destra di cardLeft colonne (più il margine): i titoli delle note e le righe
// di separazione cominciano tutti alla stessa colonna, tranne titolo e footer
// che restano centrati sull'intera area contenuto.
func TestListCenteredHorizontally(t *testing.T) {
	cases := []struct{ w, h int }{
		{80, 24},  // card di 60 su 76 colonne: scostamento 8 + margine 2
		{160, 24}, // card di 60 su 156 colonne: scostamento 48 + margine 2
		{40, 24},  // card a larghezza piena: nessuno scostamento, solo il margine
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
				// Il titolo è centrato nella card: comincia a cardCol più metà
				// dello spazio libero.
				if i := strings.Index(plain, n.Title); i >= 0 {
					col := ansi.StringWidth(plain[:i])
					want := cardCol + (cardW-ansi.StringWidth(n.Title))/2
					if col != want {
						t.Errorf("w=%d: il titolo %q comincia alla colonna %d, want %d:\n%s",
							c.w, n.Title, col, want, s.View())
					}
					titles++
				}
			}
			if i := strings.Index(plain, "─"); i >= 0 {
				if col := ansi.StringWidth(plain[:i]); col != cardCol {
					t.Errorf("w=%d: la riga di separazione comincia alla colonna %d, want %d:\n%s",
						c.w, col, cardCol, s.View())
				}
			}
		}
		if titles == 0 {
			t.Fatalf("w=%d: nessun titolo trovato nella view:\n%s", c.w, s.View())
		}

		// La card sta dentro lo schermo: inizio card + larghezza non superano w.
		if limit := cardCol + cardW; limit > c.w {
			t.Errorf("w=%d: la card finisce alla colonna %d, oltre il bordo %d", c.w, limit, c.w)
		}
	}
}

// TestListHitOutsideCard verifica che i click fuori dalla larghezza della card
// non colpiscano nessuna nota, mentre un click al suo interno la seleziona.
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
				t.Fatalf("il click (%d,%d) fuori dalla card [%d,%d) ha colpito una nota", x, y, left, right)
			}
		}
	}

	hit := false
	for y := 0; y < h && !hit; y++ {
		_, hit = s.listHitNote(left+2, y)
	}
	if !hit {
		t.Errorf("nessun click dentro la card ha colpito una nota")
	}
}
