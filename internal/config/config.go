// Package config reads Nook's configuration file, ~/.config/Nook/config.toml.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nook/internal/ini"
)

// Example is the documented configuration file that "nook config init"
// writes. It holds the default values.
//
//go:embed example.toml
var Example string

// Config is the configuration after the file and the environment are applied.
type Config struct {
	Theme    string // "dark", "light", "auto", a theme name or a path
	Vim      bool   // vim-style modal editing
	NotesDir string // where the notes live; "" = ~/.nook
	Dir      string // the configuration directory, ~/.config/Nook
	File     string // the configuration file, which may not exist
}

// Default returns the settings used when nothing is configured.
func Default() Config { return Config{Theme: "dark"} }

// Dir returns Nook's configuration directory: $XDG_CONFIG_HOME/Nook, or
// ~/.config/Nook.
func Dir() (string, error) { return dir(os.Getenv, os.UserHomeDir) }

func dir(getenv func(string) string, home func() (string, error)) (string, error) {
	if x := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "Nook"), nil
	}
	h, err := home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", "Nook"), nil
}

// Load reads the configuration file (a missing file is fine: the defaults
// apply) and then the environment variables NOOK_THEME, NOOK_VIM and
// NOOK_DIR, which win over it. NOOK_CONFIG points at another file.
func Load() (Config, error) { return load(os.Getenv, os.UserHomeDir) }

func load(getenv func(string) string, home func() (string, error)) (Config, error) {
	cfg := Default()
	d, err := dir(getenv, home)
	if err != nil {
		return cfg, err
	}
	cfg.Dir = d
	cfg.File = filepath.Join(d, "config.toml")
	if p := getenv("NOOK_CONFIG"); p != "" {
		cfg.File = p
		cfg.Dir = filepath.Dir(p)
	}

	b, err := os.ReadFile(cfg.File)
	switch {
	case err == nil:
		if err := cfg.apply(string(b), home); err != nil {
			return cfg, fmt.Errorf("%s: %v", cfg.File, err)
		}
	case !os.IsNotExist(err):
		return cfg, err
	}

	if v := getenv("NOOK_THEME"); v != "" {
		cfg.Theme = v
	}
	if v := getenv("NOOK_VIM"); v != "" {
		cfg.Vim = v == "1"
	}
	if v := getenv("NOOK_DIR"); v != "" {
		cfg.NotesDir = v
	}
	return cfg, nil
}

// apply reads the text of a configuration file.
func (c *Config) apply(src string, home func() (string, error)) error {
	f, err := ini.Parse(src)
	if err != nil {
		return err
	}
	for _, s := range f.Names() {
		if s != "" {
			return fmt.Errorf("unknown section [%s]", s)
		}
	}
	top := f.Section("")
	for _, k := range ini.Keys(top) {
		v := top[k]
		switch k {
		case "theme":
			if strings.TrimSpace(v.Text) == "" {
				return fmt.Errorf("line %d: theme is empty", v.Line)
			}
			c.Theme = v.Text
		case "vim":
			b, err := v.Bool()
			if err != nil {
				return err
			}
			c.Vim = b
		case "notes_dir":
			p, err := expandHome(v.Text, home)
			if err != nil {
				return fmt.Errorf("line %d: %v", v.Line, err)
			}
			if !filepath.IsAbs(p) {
				return fmt.Errorf("line %d: notes_dir must be an absolute path or start with ~/", v.Line)
			}
			c.NotesDir = p
		default:
			return fmt.Errorf("line %d: unknown setting %q (known: theme, vim, notes_dir)", v.Line, k)
		}
	}
	return nil
}

// expandHome turns a leading "~" into the home directory.
func expandHome(p string, home func() (string, error)) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	h, err := home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, strings.TrimPrefix(p, "~")), nil
}

// Init writes the documented default configuration file unless one already
// exists, and returns its path.
func Init() (string, error) {
	cfg, err := Load()
	if err != nil {
		// A broken file is exactly the one we must not overwrite.
		return cfg.File, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.File), 0o755); err != nil {
		return cfg.File, err
	}
	f, err := os.OpenFile(cfg.File, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return cfg.File, fmt.Errorf("%s already exists", cfg.File)
		}
		return cfg.File, err
	}
	defer f.Close()
	_, err = f.WriteString(Example)
	return cfg.File, err
}
