package app

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// debugKeysEnv is the environment variable that enables key diagnostics.
// "1" activates it; any other value (or its absence) deactivates it.
const debugKeysEnv = "NOOK_DEBUG_KEYS"

// logKey appends a diagnostic line to ~/.nook/keys.log when
// NOOK_DEBUG_KEYS is "1", so as to see what the terminal really sends.
// If diagnostics are off it does nothing and creates no file.
// I/O errors are ignored: diagnostics must never block
// the editor.
func logKey(k tea.KeyMsg) {
	if os.Getenv(debugKeysEnv) != "1" {
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	dir := filepath.Join(home, ".nook")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, "keys.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "Type=%v Runes=%q Alt=%v Paste=%v String=%q\n",
		k.Type, k.Runes, k.Alt, k.Paste, k.String())
}

// knownMsg reports whether msg is one of the messages the application already knows and
// handles: resizing, keys, the mouse and the ticks that turn off the
// list and editor toasts. Everything else (for example an unknown CSI that
// bubbletea delivers raw) is an unrecognised message.
func knownMsg(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.WindowSizeMsg, tea.KeyMsg, tea.MouseMsg, listToastClear, editorToastClear:
		return true
	}
	return false
}

// logMsg appends a diagnostic line to ~/.nook/keys.log when
// NOOK_DEBUG_KEYS is "1", but ONLY for messages the application does not
// know: it serves to see the escape sequences (CSI) the terminal sends and
// that none of the handled cases intercepts. Keys stay recorded by logKey.
// Like logKey it does nothing when diagnostics are off and ignores
// I/O errors, because diagnostics must never block the editor.
func logMsg(msg tea.Msg) {
	if os.Getenv(debugKeysEnv) != "1" {
		return
	}
	if knownMsg(msg) {
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	dir := filepath.Join(home, ".nook")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, "keys.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "Type=%T Val=%q\n", msg, fmt.Sprintf("%v", msg))
}
