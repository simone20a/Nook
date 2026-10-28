package app

import "testing"

// TestParseCSIu checks the decoding of the "CSI u" sequences that terminals
// send for ctrl+shift combinations.
func TestParseCSIu(t *testing.T) {
	cases := []struct {
		seq    string
		action string
		ok     bool
	}{
		{"122;6u", csiUndo, true}, // ctrl+shift+z, as seen in a keys.log
		{"121;6u", csiRedo, true}, // ctrl+shift+y
		{"99;6u", csiCopy, true},  // ctrl+shift+c
		{"122;5u", "", false},     // ctrl+z alone is not bound
		{"99;5u", "", false},      // plain ctrl+c is a normal key (quit)
		{"122;2u", "", false},     // shift+z only
		{"122;7u", "", false},     // ctrl+alt+z
		{"122u", "", false},
		{"x;6u", "", false},
		{"122;6~", "", false},
	}
	for _, tc := range cases {
		got, ok := parseCSIu(tc.seq)
		if got != tc.action || ok != tc.ok {
			t.Errorf("parseCSIu(%q) = %q, %v; want %q, %v", tc.seq, got, ok, tc.action, tc.ok)
		}
	}
}
