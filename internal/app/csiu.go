package app

import (
	"reflect"
	"strconv"
	"strings"
)

// Actions that a terminal can ask for with a "CSI u" key sequence.
const (
	csiUndo = "undo"
	csiRedo = "redo"
	csiCopy = "copy"
)

// csiAction decodes the key sequences that Bubble Tea does not know. Terminals
// with the kitty keyboard protocol (kitty, WezTerm, Ghostty, iTerm2...) send
// ctrl+shift+z as "ESC [ 122 ; 6 u", which arrives as an unknown-sequence
// message. Here the ones that matter are turned into actions:
// ctrl+shift+z redoes, ctrl+z undoes, ctrl+y redoes, ctrl+shift+c copies.
func csiAction(msg any) (string, bool) {
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem().Kind() != reflect.Uint8 ||
		v.Type().Name() != "unknownCSISequenceMsg" {
		return "", false
	}
	seq := strings.TrimPrefix(string(v.Bytes()), "\x1b[")
	return parseCSIu(seq)
}

// parseCSIu reads a "code;modifiers u" sequence (without the introducer) and
// returns the action bound to that key, if any.
func parseCSIu(seq string) (string, bool) {
	if !strings.HasSuffix(seq, "u") {
		return "", false
	}
	parts := strings.Split(strings.TrimSuffix(seq, "u"), ";")
	if len(parts) != 2 {
		return "", false
	}
	code, err := strconv.Atoi(strings.Split(parts[0], ":")[0])
	if err != nil {
		return "", false
	}
	mod, err := strconv.Atoi(strings.Split(parts[1], ":")[0])
	if err != nil || mod < 1 {
		return "", false
	}
	// The modifiers are sent as 1 + a bit mask: shift 1, alt 2, ctrl 4.
	bits := mod - 1
	shift, alt, ctrl := bits&1 != 0, bits&2 != 0, bits&4 != 0
	if !ctrl || alt {
		return "", false
	}
	switch code {
	case 'z', 'Z':
		if shift {
			return csiRedo, true
		}
		return csiUndo, true
	case 'y', 'Y':
		if !shift {
			return csiRedo, true
		}
	case 'c', 'C':
		if shift {
			return csiCopy, true
		}
	}
	return "", false
}
