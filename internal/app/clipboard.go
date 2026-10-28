package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// copyText copies text to the system clipboard; tests replace it.
var copyText = copyToClipboard

// copyToClipboard writes text to the clipboard in two ways, both harmless when
// unsupported: the OSC 52 escape sequence, which the terminal turns into a
// clipboard write (also over ssh and inside tmux), and the clipboard command of
// the system.
func copyToClipboard(text string) {
	fmt.Fprintf(os.Stdout, "\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(text)))

	for _, c := range clipboardCommands() {
		path, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return
		}
	}
}

// clipboardCommands lists the clipboard commands to try on this system.
func clipboardCommands() [][]string {
	switch runtime.GOOS {
	case "darwin":
		return [][]string{{"pbcopy"}}
	case "windows":
		return [][]string{{"clip"}}
	}
	return [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
}
