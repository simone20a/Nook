package app

import (
	"strings"

	ansi "github.com/charmbracelet/x/ansi"

	"nook/internal/theme"
)

// helpRow is one row of a help page: a key and its description, or a
// section heading when heading is true (only desc is used).
type helpRow struct {
	key, desc string
	heading   bool
}

// helpPage is one tab of the help popup.
type helpPage struct {
	title string
	rows  []helpRow
}

// hh builds a heading row and hk a key row.
func hh(title string) helpRow     { return helpRow{desc: title, heading: true} }
func hk(key, desc string) helpRow { return helpRow{key: key, desc: desc} }

// Indexes of the pages that open by default on each screen.
const (
	helpTabList   = 0
	helpTabEditor = 1
)

// helpWidth is the fixed text width of the popup, so that switching tab
// never resizes the box (it shrinks only on narrow terminals).
const helpWidth = 68

// helpKeyW is the width of the key column.
const helpKeyW = 24

// helpFooter explains how to move around the help itself.
const helpFooter = "←/→ section · ↑/↓ scroll · esc close"

var helpPages = []helpPage{
	{"List", []helpRow{
		hh("Browse"),
		hk("↑ / ↓", "move the selection"),
		hk("enter / →", "open the note, or enter the folder"),
		hk("← / esc", "go up one folder"),
		hk("f / ctrl+f", "filter by title"),
		hh("Create and change"),
		hk("n / ctrl+n", "new note in this folder"),
		hk("c / ctrl+k", "new folder"),
		hk("r / ctrl+r", "rename"),
		hk("d / ctrl+d", "move to the trash"),
		hh("Trash"),
		hk("o / ctrl+o", "switch between list and trash"),
		hk("u", "restore (in the trash)"),
		hk("x / ctrl+x", "delete forever (in the trash)"),
		hh("Popups"),
		hk("enter / esc", "confirm / cancel an input"),
		hk("y or enter / n or esc", "confirm / cancel a question"),
	}},
	{"Editor", []helpRow{
		hh("Files"),
		hk("ctrl+s (or F2)", "save"),
		hk("esc / ctrl+q", "back to the list (asks if unsaved)"),
		hh("Moving"),
		hk("arrows", "move the cursor"),
		hk("home / end", "start / end of the line"),
		hk("ctrl+home / ctrl+end", "start / end of the document"),
		hk("pgup / pgdn", "one page up / down"),
		hk("ctrl+← / ctrl+→", "previous / next word (also alt)"),
		hh("Selecting"),
		hk("shift + arrows", "extend the selection"),
		hk("shift+home / shift+end", "select to start / end of line"),
		hk("shift+ctrl+← / →", "select by word"),
		hk("ctrl+c", "copy the selection (ctrl+shift+c in some terminals)"),
		hh("Undo"),
		hk("ctrl+z", "undo"),
		hk("ctrl+y", "redo (ctrl+shift+z in some terminals)"),
		hh("Deleting"),
		hk("backspace / delete", "delete before / after the cursor"),
		hk("alt+backspace", "delete the previous word"),
		hk("ctrl+u / ctrl+k", "delete to start / end of line"),
	}},
	{"Formatting", []helpRow{
		hh("Palette"),
		hk("ctrl+t", "open the formatting palette"),
		hk("a letter", "apply that format at once"),
		hk("↑ / ↓, enter, esc", "or choose, apply, close"),
		hh("Letters in the palette"),
		hk("b", "bold"),
		hk("i", "italic"),
		hk("s", "strikethrough"),
		hk("k", "code"),
		hk("l", "link"),
		hk("q", "quote"),
		hk("u", "bullet list"),
		hk("x", "checkbox"),
		hk("h", "heading level (# → ## → ###)"),
		hk("0", "remove the heading"),
		hh("Good to know"),
		hk("with a selection", "the format wraps the selected text"),
		hk("on the focus line", "markdown is raw: type it by hand"),
	}},
	{"Mouse", []helpRow{
		hh("List"),
		hk("click", "select an entry"),
		hk("double click", "open the note or folder"),
		hh("Editor"),
		hk("click", "move the Focus Line there on release"),
		hk("double click", "select the word"),
		hk("triple click", "select the line"),
		hk("wheel / two fingers", "scroll the document under the Focus Line"),
		hk("drag", "select; nothing moves, even on release"),
		hk("shift + drag", "native terminal selection"),
	}},
	{"General", []helpRow{
		hh("Help"),
		hk("ctrl+/ (or F1)", "open or close this help"),
		hk("?", "open help from the list"),
		hk("esc", "close this help"),
		hh("Quit"),
		hk("ctrl+c", "quit now (copies if text is selected)"),
		hk("q / ctrl+q", "quit from the list"),
		hh("Data"),
		hk("~/.nook", "notes are .md files, folders are folders"),
		hk(".nook/.trash", "where deleted notes wait"),
		hk("NOOK_DIR", "environment variable to change the folder"),
	}},
}

// openHelp opens the help on the page that fits the active screen.
func (m Model) openHelp() Model {
	m.helpOpen = true
	m.helpScroll = 0
	if m.screen == screenEditor {
		m.helpTab = helpTabEditor
	} else {
		m.helpTab = helpTabList
	}
	return m
}

// helpBodyH is the fixed number of rows of the scrollable part of the popup:
// the tallest page, limited by what fits in a terminal of height h (the tab
// bar, the footer and two blank rows take the other four).
func helpBodyH(h int) int {
	tallest := 0
	for _, p := range helpPages {
		if len(p.rows) > tallest {
			tallest = len(p.rows)
		}
	}
	body := maxPopupH(h) - 4
	if body > tallest {
		body = tallest
	}
	if body < 1 {
		body = 1
	}
	return body
}

// updateHelpKey handles a key while help is open: it moves between pages,
// scrolls, or closes it. Unknown keys are ignored so that the popup is
// not closed by accident.
func (m Model) updateHelpKey(key string) Model {
	n := len(helpPages)
	body := helpBodyH(m.height)
	switch key {
	case "esc", "q", "?", "f1", "ctrl+/", "ctrl+_":
		m.helpOpen = false
		return m
	case "right", "tab", "l":
		m.helpTab, m.helpScroll = (m.helpTab+1)%n, 0
	case "left", "shift+tab", "h":
		m.helpTab, m.helpScroll = (m.helpTab+n-1)%n, 0
	case "down", "j":
		m.helpScroll++
	case "up", "k":
		m.helpScroll--
	case "pgdown", " ":
		m.helpScroll += body
	case "pgup":
		m.helpScroll -= body
	case "home":
		m.helpScroll = 0
	case "end":
		m.helpScroll = len(helpPages[m.helpTab].rows)
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			if i := int(key[0] - '1'); i < n {
				m.helpTab, m.helpScroll = i, 0
			}
		}
	}
	maxScroll := len(helpPages[m.helpTab].rows) - body
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.helpScroll > maxScroll {
		m.helpScroll = maxScroll
	}
	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
	return m
}

// helpTabBar renders the row of tabs, with the active one highlighted.
func helpTabBar(active int) string {
	parts := make([]string, len(helpPages))
	for i, p := range helpPages {
		label := " " + p.title + " "
		if i == active {
			parts[i] = theme.Rev.Render(label)
		} else {
			parts[i] = theme.DimText.Render(label)
		}
	}
	return strings.Join(parts, " ")
}

// renderHelp builds the content of the help popup for a terminal w x h: a
// tab bar, the visible rows of the active page and a footer. Every row has
// the same width so the box keeps its size when the page changes.
func renderHelp(w, h, tab, scroll int) string {
	if tab < 0 || tab >= len(helpPages) {
		tab = 0
	}
	textW := helpWidth
	if limit := maxPopupW(w); textW > limit {
		textW = limit
	}
	body := helpBodyH(h)
	rows := helpPages[tab].rows
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(rows)-body {
		scroll = len(rows) - body
	}
	if scroll < 0 {
		scroll = 0
	}

	fit := func(s string) string {
		return padPlain(truncatePlain(s, textW), textW)
	}

	lines := []string{fit(helpTabBar(tab)), fit("")}
	for i := 0; i < body; i++ {
		idx := scroll + i
		if idx >= len(rows) {
			lines = append(lines, fit(""))
			continue
		}
		r := rows[idx]
		if r.heading {
			lines = append(lines, fit(theme.BannerAcc.Render(r.desc)))
			continue
		}
		key := padPlain(truncatePlain(r.key, helpKeyW), helpKeyW)
		lines = append(lines, fit(theme.Selected.Render(key)+"  "+r.desc))
	}

	footer := helpFooter
	if scroll > 0 {
		footer += " · ▲"
	}
	if scroll+body < len(rows) {
		footer += " · ▼"
	}
	lines = append(lines, fit(""), fit(theme.DimText.Render(footer)))
	return strings.Join(lines, "\n")
}

// helpTextWidth is the widest visible row of a rendered help, used by tests.
func helpTextWidth(s string) int {
	widest := 0
	for _, l := range strings.Split(s, "\n") {
		if lw := ansi.StringWidth(l); lw > widest {
			widest = lw
		}
	}
	return widest
}
