package themefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nook/internal/markdown"
)

func TestBuiltinsAreComplete(t *testing.T) {
	for _, name := range BuiltinNames {
		th, ok := Builtin(name)
		if !ok {
			t.Fatalf("no built-in theme %q", name)
		}
		if th.Name != name {
			t.Errorf("%s: name = %q", name, th.Name)
		}
		if len(th.Full) != len(StyleNames) || len(th.Dim) != len(StyleNames) || len(th.UI) != len(UINames) {
			t.Errorf("%s: %d/%d/%d styles", name, len(th.Full), len(th.Dim), len(th.UI))
		}
	}
}

// The dark theme keeps the colours Nook has always had.
func TestDarkValues(t *testing.T) {
	th, _ := Builtin("dark")
	if got := th.Dim[markdown.StyleHeading1]; got != (Spec{FG: "153", BG: "24", Bold: true}) {
		t.Errorf("dim heading1 = %+v", got)
	}
	if got := th.UI["selection"]; got != (Spec{FG: "232", BG: "238"}) {
		t.Errorf("selection = %+v", got)
	}
	l, _ := Builtin("light")
	if got := l.Full[markdown.StyleCode]; got != (Spec{FG: "88", BG: "253"}) {
		t.Errorf("light code = %+v", got)
	}
}

func TestParseSpec(t *testing.T) {
	sp, err := ParseSpec("fg=69 bg=#AABBCC bold italic strike underline faint")
	if err != nil {
		t.Fatal(err)
	}
	want := Spec{FG: "69", BG: "#aabbcc", Bold: true, Italic: true, Strike: true, Underline: true, Faint: true}
	if sp != want {
		t.Errorf("got %+v", sp)
	}
	if sp, err := ParseSpec(""); err != nil || sp != (Spec{}) {
		t.Errorf("empty: %+v, %v", sp, err)
	}
	for _, bad := range []string{"fg=256", "fg=#12345", "bg=red", "blink", "fg=-1"} {
		if _, err := ParseSpec(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestCustomThemeOverridesBase(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "base = \"light\"\n[dim]\nheading1 = \"fg=#ff0000 bold\"\n[ui]\nerror = \"fg=1\"\n"
	if err := os.WriteFile(filepath.Join(dir, "themes", "mine.toml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	th, err := Load("mine", dir)
	if err != nil {
		t.Fatal(err)
	}
	if th.Name != "mine" {
		t.Errorf("name = %q", th.Name)
	}
	if got := th.Dim[markdown.StyleHeading1]; got != (Spec{FG: "#ff0000", Bold: true}) {
		t.Errorf("heading1 = %+v", got)
	}
	light, _ := Builtin("light")
	if th.Dim[markdown.StyleBold] != light.Dim[markdown.StyleBold] {
		t.Error("untouched style does not come from the base")
	}
	// A path works too, relative to the config directory.
	if _, err := Load("themes/mine.toml", dir); err != nil {
		t.Errorf("by path: %v", err)
	}
}

func TestDefaultBaseIsDark(t *testing.T) {
	th, err := Parse("[ui]\nerror = \"fg=1\"\n", func(n string) (Theme, bool) { return Builtin(n) })
	if err != nil {
		t.Fatal(err)
	}
	dark, _ := Builtin("dark")
	if th.Full[markdown.StyleBold] != dark.Full[markdown.StyleBold] {
		t.Error("default base is not dark")
	}
}

func TestErrors(t *testing.T) {
	base := func(n string) (Theme, bool) { return Builtin(n) }
	cases := map[string]string{
		"colour = 1":                 "unknown setting",
		"[nope]":                     "unknown section",
		"[dim]\nheading9 = \"bold\"": "unknown style",
		"[ui]\nerror = \"fg=999\"":   "bad colour",
		"base = \"missing\"":         "not found",
		"[full]\nbold = \"loud\"":    "unknown word",
		"[dim\nbold = \"bold\"":      "missing ]",
	}
	for src, want := range cases {
		if _, err := Parse(src, base); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%q) = %v, want %q", src, err, want)
		}
	}
	if _, err := Parse("name = \"x\"\n", nil); err == nil || !strings.Contains(err.Error(), "missing styles") {
		t.Errorf("incomplete theme without base: %v", err)
	}
	if _, err := Load("ghost", t.TempDir()); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing file: %v", err)
	}
}
