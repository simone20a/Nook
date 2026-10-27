package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestDebugKeyLog checks key diagnostics: with NOOK_DEBUG_KEYS=1
// logKey appends the KeyMsg to ~/.nook/keys.log, while with the variable
// off it creates no file.
func TestDebugKeyLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".nook")
	path := filepath.Join(dir, "keys.log")

	t.Setenv(debugKeysEnv, "1")
	logKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	content := string(data)
	if !strings.Contains(content, "Type=") {
		t.Fatalf("keys.log = %q, want it to contain %q", content, "Type=")
	}
	if !strings.Contains(content, "String=") {
		t.Fatalf("keys.log = %q, want it to contain %q", content, "String=")
	}

	// With diagnostics off no file must appear: we start
	// from a clean HOME, without the directory created by the first logKey.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("removing %s: %v", dir, err)
	}

	t.Setenv(debugKeysEnv, "0")
	logKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("keys.log exists with diagnostics off (stat err = %v)", err)
	}
}
