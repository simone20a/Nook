package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestDebugKeyLog verifica la diagnostica dei tasti: con MDEDIT_DEBUG_KEYS=1
// logKey appende il KeyMsg a ~/.mdedit/keys.log, mentre con la variabile
// disattivata non crea alcun file.
func TestDebugKeyLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".mdedit")
	path := filepath.Join(dir, "keys.log")

	t.Setenv(debugKeysEnv, "1")
	logKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lettura %s: %v", path, err)
	}
	content := string(data)
	if !strings.Contains(content, "Type=") {
		t.Fatalf("keys.log = %q, atteso contenere %q", content, "Type=")
	}
	if !strings.Contains(content, "String=") {
		t.Fatalf("keys.log = %q, atteso contenere %q", content, "String=")
	}

	// Con la diagnostica disattivata non deve nascere nessun file: si parte
	// da una HOME pulita, senza la directory creata dal primo logKey.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("rimozione %s: %v", dir, err)
	}

	t.Setenv(debugKeysEnv, "0")
	logKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("keys.log esiste con la diagnostica disattivata (stat err = %v)", err)
	}
}
