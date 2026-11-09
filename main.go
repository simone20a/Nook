package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/app"
	"nook/internal/config"
	"nook/internal/store"
	"nook/internal/theme"
	"nook/internal/themefile"
)

const usage = `Usage:
  nook                      open the notes
  nook config init          write a documented ~/.config/Nook/config.toml
  nook config path          print the path of the configuration file
  nook config show          print the settings in effect and where they come from
  nook theme <dark|light>   print a built-in theme, as a starting point for your own
  nook help                 show this text
`

func main() {
	if len(os.Args) > 1 {
		os.Exit(command(os.Args[1:]))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	th, err := theme.Resolve(cfg.Theme, cfg.Dir)
	if err != nil {
		fatal(err)
	}
	theme.Use(th)
	app.SetVim(cfg.Vim)
	app.SetReading(cfg.Reading)

	root := cfg.NotesDir
	if root == "" {
		root = filepath.Join(home, ".nook")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		fatal(err)
	}

	importLegacy(home, root)

	st, err := store.Open(root)
	if err != nil {
		fatal(err)
	}
	defer st.Close()
	p := tea.NewProgram(app.New(st), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fatal(err)
	}
}

// command runs a subcommand and returns the exit status.
func command(args []string) int {
	switch {
	case args[0] == "theme" && len(args) == 2:
		src, ok := themefile.Source(args[1])
		if !ok {
			fmt.Fprintf(os.Stderr, "nook: no built-in theme %q (try dark or light)\n", args[1])
			return 1
		}
		fmt.Print(src)
	case args[0] == "config" && len(args) == 2 && args[1] == "init":
		path, err := config.Init()
		if err != nil {
			fmt.Fprintln(os.Stderr, "nook:", err)
			return 1
		}
		fmt.Println("wrote", path)
	case args[0] == "config" && len(args) == 2 && args[1] == "path":
		cfg, err := config.Load()
		if cfg.File == "" {
			fmt.Fprintln(os.Stderr, "nook:", err)
			return 1
		}
		fmt.Println(cfg.File)
	case args[0] == "config" && len(args) == 2 && args[1] == "show":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "nook:", err)
			return 1
		}
		found := "found"
		if _, err := os.Stat(cfg.File); err != nil {
			found = "not found: defaults apply"
		}
		fmt.Printf("file:             %s (%s)\n", cfg.File, found)
		fmt.Printf("theme:            %s\n", cfg.Theme)
		fmt.Printf("vim:              %v\n", cfg.Vim)
		fmt.Printf("start_in_reading: %v\n", cfg.Reading)
		fmt.Printf("notes_dir:        %q\n", cfg.NotesDir)
		for _, k := range []string{"NOOK_CONFIG", "NOOK_THEME", "NOOK_VIM", "NOOK_READING", "NOOK_DIR"} {
			if v := os.Getenv(k); v != "" {
				fmt.Printf("%s=%s is set and wins over the file\n", k, v)
			}
		}
	case args[0] == "help" || args[0] == "-h" || args[0] == "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	return 0
}

// importLegacy moves the notes of the old SQLite databases (from when Nook
// stored everything in notes.db) into plain files, once: after a successful
// import the database is renamed to notes.db.imported and never read again.
func importLegacy(home, root string) {
	for _, db := range []string{
		filepath.Join(root, "notes.db"),
		filepath.Join(home, ".nook", "notes.db"),
		filepath.Join(home, ".mdedit", "notes.db"),
	} {
		if _, err := os.Stat(db); err != nil {
			continue
		}
		n, err := store.ImportLegacyDB(db, root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nook: could not import %s: %v\n", db, err)
			continue
		}
		if err := os.Rename(db, db+".imported"); err != nil {
			fmt.Fprintf(os.Stderr, "nook: imported %d notes from %s but could not rename it: %v\n", n, db, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "nook: imported %d notes from %s\n", n, db)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "nook:", err)
	os.Exit(1)
}
