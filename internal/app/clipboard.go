package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// copyText copies text to the system clipboard; tests replace it.
var copyText = copyToClipboard

// copyToClipboard writes text to the clipboard in two ways: the OSC 52 escape
// sequence, which the terminal turns into a clipboard write (also over ssh and
// inside tmux, but some terminals ignore it or ask for permission), and the
// clipboard command of the system. It fails only if no command could be run.
func copyToClipboard(text string) error {
	fmt.Fprintf(os.Stdout, "\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(text)))

	var last error
	for _, c := range clipboardCommands() {
		path, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c[1:]...)
		cmd.Stdin = strings.NewReader(text)
		// pbcopy reads text in the encoding of the locale.
		cmd.Env = append(os.Environ(), "LC_CTYPE=UTF-8")
		if last = cmd.Run(); last == nil {
			return nil
		}
	}
	if last != nil {
		return last
	}
	if len(clipboardCommands()) > 0 {
		return errors.New("no clipboard command found; relying on the terminal (OSC 52)")
	}
	return nil
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
