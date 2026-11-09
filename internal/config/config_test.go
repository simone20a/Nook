package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// env builds a getenv function over a fixed map.
func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func home(h string) func() (string, error) { return func() (string, error) { return h, nil } }

// write puts a config file in a fresh home and returns the home.
func write(t *testing.T, content string) string {
	t.Helper()
	h := t.TempDir()
	d := filepath.Join(h, ".config", "Nook")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestDefaultsWithoutFile(t *testing.T) {
	h := t.TempDir()
	cfg, err := load(env(nil), home(h))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "dark" || cfg.Vim || cfg.Reading || cfg.NotesDir != "" {
		t.Errorf("defaults = %+v", cfg)
	}
	if want := filepath.Join(h, ".config", "Nook", "config.toml"); cfg.File != want {
		t.Errorf("File = %q, want %q", cfg.File, want)
	}
}

func TestExampleIsTheDefaults(t *testing.T) {
	h := write(t, Example)
	cfg, err := load(env(nil), home(h))
	if err != nil {
		t.Fatal(err)
	}
	if def := Default(); cfg.Theme != def.Theme || cfg.Vim != def.Vim || cfg.Reading != def.Reading || cfg.NotesDir != "" {
		t.Errorf("example config = %+v", cfg)
	}
}

func TestReadsFile(t *testing.T) {
	h := write(t, "theme = \"light\"\nvim = true\nstart_in_reading = true\nnotes_dir = \"~/Notes\"\n")
	cfg, err := load(env(nil), home(h))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "light" || !cfg.Vim || !cfg.Reading || cfg.NotesDir != filepath.Join(h, "Notes") {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestEnvironmentWins(t *testing.T) {
	h := write(t, "theme = \"light\"\nvim = true\nstart_in_reading = true\nnotes_dir = \"/a\"\n")
	cfg, err := load(env(map[string]string{"NOOK_THEME": "auto", "NOOK_VIM": "0", "NOOK_READING": "0", "NOOK_DIR": "/b"}), home(h))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "auto" || cfg.Vim || cfg.Reading || cfg.NotesDir != "/b" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestXDGAndNookConfig(t *testing.T) {
	x := t.TempDir()
	if err := os.MkdirAll(filepath.Join(x, "Nook"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(x, "Nook", "config.toml"), []byte("vim = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := load(env(map[string]string{"XDG_CONFIG_HOME": x}), home("/nowhere"))
	if err != nil || !cfg.Vim || cfg.Dir != filepath.Join(x, "Nook") {
		t.Errorf("XDG: %+v, %v", cfg, err)
	}

	other := filepath.Join(t.TempDir(), "c.toml")
	if err := os.WriteFile(other, []byte("theme = \"light\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = load(env(map[string]string{"NOOK_CONFIG": other}), home("/nowhere"))
	if err != nil || cfg.Theme != "light" || cfg.Dir != filepath.Dir(other) {
		t.Errorf("NOOK_CONFIG: %+v, %v", cfg, err)
	}
}

func TestErrorsNameFileAndLine(t *testing.T) {
	cases := map[string]string{
		"colour = \"red\"":      "unknown setting",
		"vim = \"yes\"":         "expected true or false",
		"start_in_reading = 1":  "expected true or false",
		"theme = \"\"":          "theme is empty",
		"notes_dir = \"notes\"": "absolute",
		"[extra]":               "unknown section",
		"theme = ":              "missing value",
	}
	for src, want := range cases {
		h := write(t, "# c\n"+src+"\n")
		_, err := load(env(nil), home(h))
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "config.toml") {
			t.Errorf("%q: error = %v, want %q", src, err, want)
		}
	}
}

func TestInit(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("NOOK_CONFIG", "")
	t.Setenv("NOOK_THEME", "")
	t.Setenv("NOOK_VIM", "")
	t.Setenv("NOOK_READING", "")
	t.Setenv("NOOK_DIR", "")
	path, err := Init()
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != Example {
		t.Error("written file is not the example")
	}
	if _, err := Init(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second Init: %v", err)
	}
}
