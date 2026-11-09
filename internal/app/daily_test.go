package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func dailyModel(t *testing.T, day string) (Model, string) {
	t.Helper()
	d, err := time.Parse(dailyLayout, day)
	if err != nil {
		t.Fatal(err)
	}
	old := dailyNow
	dailyNow = func() time.Time { return d }
	t.Cleanup(func() { dailyNow = old })
	st := newKeysTestStore(t)
	m := New(st)
	m.width, m.height = 80, 24
	return m, st.Root()
}

func press(m Model, k tea.KeyMsg) Model {
	out, _ := m.Update(k)
	return out.(Model)
}

func TestDailyDraftIsWrittenOnlyOnSave(t *testing.T) {
	m, root := dailyModel(t, "2026-11-10")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.screen != screenEditor || !m.editor.draft {
		t.Fatalf("screen %v draft %v", m.screen, m.editor.draft)
	}
	if _, err := os.Stat(filepath.Join(root, "Daily")); !os.IsNotExist(err) {
		t.Fatalf("Daily created before saving (err = %v)", err)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	b, err := os.ReadFile(filepath.Join(root, "Daily", "2026-11-10.md"))
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if string(b) != "# 2026-11-10\n\nhello" {
		t.Fatalf("body %q", b)
	}
	if m.editor.draft {
		t.Fatal("still a draft after saving")
	}
	// A second save updates the same file.
	m = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	b, _ = os.ReadFile(filepath.Join(root, "Daily", "2026-11-10.md"))
	if string(b) != "# 2026-11-10\n\nhello!" {
		t.Fatalf("body after second save %q", b)
	}
}

func TestDailyDiscardCreatesNothing(t *testing.T) {
	m, root := dailyModel(t, "2026-11-10")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	m = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = press(m, tea.KeyMsg{Type: tea.KeyRight}) // Discard
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenList {
		t.Fatalf("screen %v, want the list", m.screen)
	}
	if _, err := os.Stat(filepath.Join(root, "Daily")); !os.IsNotExist(err) {
		t.Fatalf("Daily created after discarding (err = %v)", err)
	}
}

func TestDailyOpensExistingNote(t *testing.T) {
	m, root := dailyModel(t, "2026-11-10")
	if err := os.MkdirAll(filepath.Join(root, "Daily"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Daily", "2026-11-10.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.screen != screenEditor || m.editor.draft {
		t.Fatalf("screen %v draft %v", m.screen, m.editor.draft)
	}
	if got := m.editor.ed.Lines()[0]; got != "old" {
		t.Fatalf("opened %q", got)
	}
}

func TestDailyNeverOverwrites(t *testing.T) {
	m, root := dailyModel(t, "2026-11-10")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if err := os.MkdirAll(filepath.Join(root, "Daily"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Daily", "2026-11-10.md"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	b, _ := os.ReadFile(filepath.Join(root, "Daily", "2026-11-10.md"))
	if string(b) != "other" {
		t.Fatalf("overwritten: %q", b)
	}
	if !m.editor.draft || m.editor.toast == "" {
		t.Fatal("expected the draft to be kept with an error toast")
	}
}
