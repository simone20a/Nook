package ini

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	src := "\ufeff# comment\ntheme = \"dark\"  # trailing\nvim = true\n\n[full]\nbold = 'a # b'\npath = \"C:\\\\x \\\"q\\\"\"\n"
	f, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	top := f.Section("")
	if v := top["theme"]; v.Text != "dark" || !v.Quoted || v.Line != 2 {
		t.Errorf("theme = %+v", v)
	}
	if b, err := top["vim"].Bool(); err != nil || !b {
		t.Errorf("vim = %v, %v", b, err)
	}
	full := f.Section("full")
	if got := full["bold"].Text; got != "a # b" {
		t.Errorf("bold = %q", got)
	}
	if got := full["path"].Text; got != `C:\x "q"` {
		t.Errorf("path = %q", got)
	}
}

func TestBoolRejectsOthers(t *testing.T) {
	for _, s := range []string{`vim = "true"`, `vim = yes`, `vim = 1`} {
		f, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Section("")["vim"].Bool(); err == nil {
			t.Errorf("%s: Bool accepted it", s)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"novalue":              "line 1",
		"a = ":                 "missing value",
		"a = \"x":              "not closed",
		"a = \"x\" y":          "unexpected text",
		"a = 1\na = 2":         "line 2",
		"[a]\n[a]":             "twice",
		"[a":                   "missing ]",
		"[a] x":                "after ]",
		"bad key = 1":          "bad key",
		"a = \"\\q\"":          "unknown escape",
		"a = # only a comment": "missing value",
	}
	for src, want := range cases {
		_, err := Parse(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%q) error = %v, want it to contain %q", src, err, want)
		}
	}
}

func TestNamesAndKeysSorted(t *testing.T) {
	f, err := Parse("[b]\nz = 1\na = 2\n[a]\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(f.Names(), ","); got != ",a,b" {
		t.Errorf("Names = %q", got)
	}
	if got := strings.Join(Keys(f.Section("b")), ","); got != "a,z" {
		t.Errorf("Keys = %q", got)
	}
}
