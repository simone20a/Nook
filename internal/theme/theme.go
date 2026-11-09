// Package theme collects the application's shared palette: the full styles
// used for highlighted text, the dimmed ones used for lines that are not in
// focus, plus the utility styles (reverse, error, toast, banner, selection,
// dimmed text and folders).
//
// The colours come from a theme file (see package themefile and the files in
// themes/): dark and light are built in, and the user can write more. The
// package starts with the dark theme; main calls Use once with the theme the
// configuration asks for, before anything is drawn.
package theme

import (
	lipgloss "github.com/charmbracelet/lipgloss"

	"nook/internal/markdown"
	"nook/internal/themefile"
)

// Styles maps every markdown style to its full rendering.
var Styles map[markdown.Style]lipgloss.Style

// DimStyles maps every markdown style to its dimmed rendering: lines that do
// not have focus do not use Faint, but a duller Foreground.
var DimStyles map[markdown.Style]lipgloss.Style

var (
	// Rev highlights the character under the cursor.
	Rev = lipgloss.NewStyle().Reverse(true)
	// Error is the style of error messages.
	Error lipgloss.Style
	// Toast is the style of the temporary overlaid message: accent text,
	// bold, no background or box, consistent with the rest of the interface.
	Toast lipgloss.Style
	// ToastError is the style of the error toast, same format but in red.
	ToastError lipgloss.Style
	// BannerAcc is the style of the banner accent.
	BannerAcc lipgloss.Style
	// Selected is the style of the selected row.
	Selected lipgloss.Style
	// DimText is the style of dimmed text: the dimming is in the colour only.
	DimText lipgloss.Style
	// FolderDim is the style of unselected folders: a muted tint, so the
	// selected folder stands out even without underlining.
	FolderDim lipgloss.Style
	// FolderSel is the style of the selected folder: a brighter colour and
	// bold, no underline.
	FolderSel lipgloss.Style
	// Selection is the style of selected text in the editor.
	Selection lipgloss.Style
)

func init() {
	t, ok := themefile.Builtin("dark")
	if !ok {
		panic("theme: no built-in dark theme")
	}
	Use(t)
}

// Resolve finds the theme called name: "dark", "light", "auto" (dark or light
// according to the terminal's background), or a custom theme file; dir is the
// configuration directory. See themefile.Load for the file lookup.
func Resolve(name, dir string) (themefile.Theme, error) {
	if name == "auto" {
		name = "light"
		if lipgloss.HasDarkBackground() {
			name = "dark"
		}
	}
	return themefile.Load(name, dir)
}

// Use makes t the theme of the whole application.
func Use(t themefile.Theme) {
	Styles = buildMarkdown(t.Full)
	DimStyles = buildMarkdown(t.Dim)
	ui := func(name string) lipgloss.Style { return style(t.UI[name]) }
	Error = ui("error")
	Toast = ui("toast")
	ToastError = ui("toast_error")
	BannerAcc = ui("banner")
	Selected = ui("selected")
	DimText = ui("dim_text")
	FolderDim = ui("folder_dim")
	FolderSel = ui("folder_sel")
	Selection = ui("selection")
}

// buildMarkdown turns the specs of a theme into lipgloss styles.
func buildMarkdown(specs map[markdown.Style]themefile.Spec) map[markdown.Style]lipgloss.Style {
	out := make(map[markdown.Style]lipgloss.Style, len(specs))
	for st, s := range specs {
		out[st] = style(s)
	}
	return out
}

// style turns one spec into a lipgloss style.
func style(s themefile.Spec) lipgloss.Style {
	st := lipgloss.NewStyle()
	if s.FG != "" {
		st = st.Foreground(lipgloss.Color(s.FG))
	}
	if s.BG != "" {
		st = st.Background(lipgloss.Color(s.BG))
	}
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Strike {
		st = st.Strikethrough(true)
	}
	if s.Underline {
		st = st.Underline(true)
	}
	if s.Faint {
		st = st.Faint(true)
	}
	return st
}
