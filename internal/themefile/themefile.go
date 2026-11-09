// Package themefile reads Nook's theme files: the two built-in themes (dark
// and light, embedded) and custom ones written by the user. A theme file is
// plain data (see themes/dark.toml); turning it into terminal styles is the
// job of package theme.
package themefile

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"nook/internal/ini"
	"nook/internal/markdown"
)

//go:embed themes/*.toml
var builtinFS embed.FS

// Spec is one style: colours (an ANSI 256 index "0".."255" or "#rrggbb",
// empty = unchanged) and attributes.
type Spec struct {
	FG, BG    string
	Bold      bool
	Italic    bool
	Strike    bool
	Underline bool
	Faint     bool
}

// Theme is a whole palette.
type Theme struct {
	Name string
	Full map[markdown.Style]Spec // full-strength markdown styles
	Dim  map[markdown.Style]Spec // markdown styles of the lines out of focus
	UI   map[string]Spec         // interface styles, by UINames
}

// StyleNames maps the key used in theme files to each markdown style.
var StyleNames = map[string]markdown.Style{
	"heading1":     markdown.StyleHeading1,
	"heading2":     markdown.StyleHeading2,
	"heading3":     markdown.StyleHeading3,
	"heading4":     markdown.StyleHeading4,
	"heading5":     markdown.StyleHeading5,
	"heading6":     markdown.StyleHeading6,
	"bold":         markdown.StyleBold,
	"italic":       markdown.StyleItalic,
	"code":         markdown.StyleCode,
	"strike":       markdown.StyleStrike,
	"link":         markdown.StyleLink,
	"image":        markdown.StyleImage,
	"quote":        markdown.StyleQuote,
	"quote_bar":    markdown.StyleQuoteBar,
	"bullet":       markdown.StyleBullet,
	"checkbox":     markdown.StyleCheckbox,
	"fence":        markdown.StyleFence,
	"code_block":   markdown.StyleCodeBlock,
	"rule":         markdown.StyleRule,
	"table_border": markdown.StyleTableBorder,
	"table_head":   markdown.StyleTableHead,
	"text":         markdown.StyleNone,
}

// UINames are the interface styles a theme defines.
var UINames = []string{
	"error", "toast", "toast_error", "banner", "selected", "dim_text",
	"folder_dim", "folder_sel", "selection",
}

// BuiltinNames are the themes that ship with Nook.
var BuiltinNames = []string{"dark", "light"}

// Source returns the text of a built-in theme file.
func Source(name string) (string, bool) {
	b, err := builtinFS.ReadFile("themes/" + name + ".toml")
	if err != nil {
		return "", false
	}
	return string(b), true
}

// Builtin returns a built-in theme.
func Builtin(name string) (Theme, bool) {
	src, ok := Source(name)
	if !ok {
		return Theme{}, false
	}
	t, err := Parse(src, nil)
	if err != nil {
		panic("themefile: built-in theme " + name + ": " + err.Error())
	}
	return t, true
}

// Parse reads a theme file. With a base function the file only needs the
// styles it changes: the rest comes from the theme named by base = "..." (the
// dark one when the key is missing). With a nil base the file must define
// every style, which is how the built-in themes are written.
func Parse(src string, base func(name string) (Theme, bool)) (Theme, error) {
	f, err := ini.Parse(src)
	if err != nil {
		return Theme{}, err
	}
	top := f.Section("")
	for _, k := range ini.Keys(top) {
		if k != "name" && k != "base" {
			return Theme{}, fmt.Errorf("line %d: unknown setting %q (expected name or base)", top[k].Line, k)
		}
	}
	for _, s := range f.Names() {
		if s != "" && s != "full" && s != "dim" && s != "ui" {
			return Theme{}, fmt.Errorf("unknown section [%s] (expected [full], [dim] or [ui])", s)
		}
	}

	t := Theme{
		Full: map[markdown.Style]Spec{},
		Dim:  map[markdown.Style]Spec{},
		UI:   map[string]Spec{},
	}
	if base != nil {
		name := "dark"
		if v, ok := top["base"]; ok {
			name = v.Text
		}
		b, ok := base(name)
		if !ok {
			return Theme{}, fmt.Errorf("base theme %q not found", name)
		}
		for k, v := range b.Full {
			t.Full[k] = v
		}
		for k, v := range b.Dim {
			t.Dim[k] = v
		}
		for k, v := range b.UI {
			t.UI[k] = v
		}
	}
	if v, ok := top["name"]; ok {
		t.Name = v.Text
	}

	for _, sec := range []struct {
		name string
		set  func(key string, sp Spec) bool
	}{
		{"full", func(k string, sp Spec) bool {
			st, ok := StyleNames[k]
			if ok {
				t.Full[st] = sp
			}
			return ok
		}},
		{"dim", func(k string, sp Spec) bool {
			st, ok := StyleNames[k]
			if ok {
				t.Dim[st] = sp
			}
			return ok
		}},
		{"ui", func(k string, sp Spec) bool {
			if k == "trash_dim" || k == "trash_sel" {
				return true // from when Nook had a trash: accepted and ignored
			}
			for _, n := range UINames {
				if n == k {
					t.UI[k] = sp
					return true
				}
			}
			return false
		}},
	} {
		keys := f.Section(sec.name)
		for _, k := range ini.Keys(keys) {
			v := keys[k]
			sp, err := ParseSpec(v.Text)
			if err != nil {
				return Theme{}, fmt.Errorf("line %d: [%s] %s: %v", v.Line, sec.name, k, err)
			}
			if !sec.set(k, sp) {
				return Theme{}, fmt.Errorf("line %d: [%s] unknown style %q", v.Line, sec.name, k)
			}
		}
	}

	if base == nil {
		return t, t.complete()
	}
	return t, nil
}

// complete reports the first style a theme without a base leaves out.
func (t Theme) complete() error {
	var missing []string
	for name, st := range StyleNames {
		if _, ok := t.Full[st]; !ok {
			missing = append(missing, "[full] "+name)
		}
		if _, ok := t.Dim[st]; !ok {
			missing = append(missing, "[dim] "+name)
		}
	}
	for _, n := range UINames {
		if _, ok := t.UI[n]; !ok {
			missing = append(missing, "[ui] "+n)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing styles: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ParseSpec reads a style such as "fg=69 bg=#102030 bold". The empty string
// is a style with no colours and no attributes.
func ParseSpec(s string) (Spec, error) {
	var sp Spec
	for _, tok := range strings.Fields(s) {
		switch {
		case strings.HasPrefix(tok, "fg="):
			c, err := parseColour(tok[3:])
			if err != nil {
				return sp, err
			}
			sp.FG = c
		case strings.HasPrefix(tok, "bg="):
			c, err := parseColour(tok[3:])
			if err != nil {
				return sp, err
			}
			sp.BG = c
		case tok == "bold":
			sp.Bold = true
		case tok == "italic":
			sp.Italic = true
		case tok == "strike":
			sp.Strike = true
		case tok == "underline":
			sp.Underline = true
		case tok == "faint":
			sp.Faint = true
		default:
			return sp, fmt.Errorf("unknown word %q (use fg=, bg=, bold, italic, strike, underline, faint)", tok)
		}
	}
	return sp, nil
}

// parseColour accepts 0-255 or #rrggbb.
func parseColour(s string) (string, error) {
	if strings.HasPrefix(s, "#") {
		if _, err := strconv.ParseUint(s[1:], 16, 32); err != nil || len(s) != 7 {
			return "", fmt.Errorf("bad colour %q (expected #rrggbb)", s)
		}
		return strings.ToLower(s), nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 255 {
		return "", fmt.Errorf("bad colour %q (expected 0-255 or #rrggbb)", s)
	}
	return strconv.Itoa(n), nil
}

// Load finds and reads the theme called name. "dark" and "light" are
// built in. Any other name is a file: a bare word is looked up as
// <dir>/themes/<name>.toml; anything with a "/" or ending in ".toml" is a
// path ("~/" is the home directory, a relative path starts at dir).
func Load(name, dir string) (Theme, error) {
	if t, ok := Builtin(name); ok {
		return t, nil
	}
	path := name
	switch {
	case strings.HasPrefix(name, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return Theme{}, err
		}
		path = filepath.Join(home, name[2:])
	case strings.ContainsAny(name, `/\`) || strings.HasSuffix(name, ".toml"):
		if !filepath.IsAbs(name) {
			path = filepath.Join(dir, name)
		}
	default:
		path = filepath.Join(dir, "themes", name+".toml")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Theme{}, fmt.Errorf("theme %q not found (looked for %s; the built-in themes are dark and light)", name, path)
		}
		return Theme{}, err
	}
	t, err := Parse(string(b), func(n string) (Theme, bool) {
		if t, ok := Builtin(n); ok {
			return t, true
		}
		return Theme{}, false
	})
	if err != nil {
		return Theme{}, fmt.Errorf("%s: %v", path, err)
	}
	if t.Name == "" {
		t.Name = strings.TrimSuffix(filepath.Base(path), ".toml")
	}
	return t, nil
}
