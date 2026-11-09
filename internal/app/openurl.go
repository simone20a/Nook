package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// openURL opens a link in the default browser or mail client; tests replace it.
var openURL = openInBrowser

// openInBrowser runs the system opener. Only web and mail links are opened:
// a note must not be able to launch files or programs.
func openInBrowser(url string) error {
	lower := strings.ToLower(url)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "mailto:") {
		return fmt.Errorf("not a web link: %s", url)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
