package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"nook/internal/app"
	"nook/internal/store"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fatal(err)
	}
	root := os.Getenv("NOOK_DIR")
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
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
