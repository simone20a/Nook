package app

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// debugKeysEnv è la variabile d'ambiente che abilita la diagnostica dei tasti.
// Vale "1" per attivarla; qualunque altro valore (o l'assenza) la disattiva.
const debugKeysEnv = "MDEDIT_DEBUG_KEYS"

// logKey appende una riga di diagnostica su ~/.mdedit/keys.log quando
// MDEDIT_DEBUG_KEYS vale "1", così da vedere cosa invia davvero il terminale.
// Se la diagnostica è disattivata non fa nulla e non crea alcun file.
// Gli errori di I/O vengono ignorati: la diagnostica non deve mai bloccare
// l'editor.
func logKey(k tea.KeyMsg) {
	if os.Getenv(debugKeysEnv) != "1" {
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	dir := filepath.Join(home, ".mdedit")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, "keys.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "Type=%v Runes=%q Alt=%v Paste=%v String=%q\n",
		k.Type, k.Runes, k.Alt, k.Paste, k.String())
}

// knownMsg indica se msg è uno dei messaggi che l'applicazione già conosce e
// gestisce: il ridimensionamento, i tasti, il mouse e i tick che spengono i
// toast di lista ed editor. Tutto il resto (per esempio una CSI sconosciuta che
// bubbletea recapita grezza) è un messaggio non riconosciuto.
func knownMsg(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.WindowSizeMsg, tea.KeyMsg, tea.MouseMsg, listToastClear, editorToastClear:
		return true
	}
	return false
}

// logMsg appende una riga di diagnostica su ~/.mdedit/keys.log quando
// MDEDIT_DEBUG_KEYS vale "1", ma SOLO per i messaggi che l'applicazione non
// conosce: serve a vedere le sequenze di escape (CSI) che il terminale invia e
// che nessuno dei casi gestiti intercetta. I tasti restano registrati da logKey.
// Come logKey non fa nulla con la diagnostica disattivata e ignora gli errori
// di I/O, perché la diagnostica non deve mai bloccare l'editor.
func logMsg(msg tea.Msg) {
	if os.Getenv(debugKeysEnv) != "1" {
		return
	}
	if knownMsg(msg) {
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	dir := filepath.Join(home, ".mdedit")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, "keys.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "Type=%T Val=%q\n", msg, fmt.Sprintf("%v", msg))
}
