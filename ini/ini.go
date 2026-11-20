// Package ini reads the small subset of TOML that Nook's configuration and
// theme files use: "key = value" lines, "[section]" headers and "#" comments.
// A value is a quoted string ("..." with \\ \" \n \t escapes, or '...'
// without escapes) or a bare word such as true, false or a number. Arrays,
// inline tables, multi-line strings and dotted keys are not supported: every
// file stays valid TOML, but not every TOML file is accepted.
package ini

import (
	"fmt"
	"sort"
	"strings"
)

// Value is the right-hand side of a "key = value" line.
type Value struct {
	Text   string // the value, unquoted and unescaped
	Quoted bool   // written between quotes
	Line   int    // 1-based line in the source
}

// Bool reads a bare true or false.
func (v Value) Bool() (bool, error) {
	if !v.Quoted {
		switch v.Text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, fmt.Errorf("line %d: expected true or false, got %q", v.Line, v.Text)
}

// File is a parsed file: the keys of each section, with the keys before the
// first header under the section "".
type File struct {
	Sections map[string]map[string]Value
}

// Section returns the keys of a section (nil when it does not exist).
func (f *File) Section(name string) map[string]Value { return f.Sections[name] }

// Names returns the section names, sorted.
func (f *File) Names() []string {
	names := make([]string, 0, len(f.Sections))
	for n := range f.Sections {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Keys returns the keys of a section, sorted.
func Keys(sec map[string]Value) []string {
	keys := make([]string, 0, len(sec))
	for k := range sec {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Parse reads src. The error names the first bad line.
func Parse(src string) (*File, error) {
	f := &File{Sections: map[string]map[string]Value{"": {}}}
	cur := ""
	for i, raw := range strings.Split(src, "\n") {
		n := i + 1
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if n == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				return nil, fmt.Errorf("line %d: missing ]", n)
			}
			if rest := strings.TrimSpace(line[end+1:]); rest != "" && rest[0] != '#' {
				return nil, fmt.Errorf("line %d: unexpected text after ]", n)
			}
			name := strings.TrimSpace(line[1:end])
			if !validName(name) {
				return nil, fmt.Errorf("line %d: bad section name %q", n, name)
			}
			if _, dup := f.Sections[name]; dup {
				return nil, fmt.Errorf("line %d: section [%s] appears twice", n, name)
			}
			f.Sections[name] = map[string]Value{}
			cur = name
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return nil, fmt.Errorf("line %d: expected key = value", n)
		}
		key := strings.TrimSpace(line[:eq])
		if !validName(key) {
			return nil, fmt.Errorf("line %d: bad key %q", n, key)
		}
		val, err := parseValue(strings.TrimSpace(line[eq+1:]), n)
		if err != nil {
			return nil, err
		}
		if _, dup := f.Sections[cur][key]; dup {
			return nil, fmt.Errorf("line %d: %q is set twice", n, key)
		}
		f.Sections[cur][key] = val
	}
	return f, nil
}

// validName accepts letters, digits, "_" and "-".
func validName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// parseValue reads the text after the "=".
func parseValue(s string, line int) (Value, error) {
	v := Value{Line: line}
	if s == "" {
		return v, fmt.Errorf("line %d: missing value", line)
	}
	switch s[0] {
	case '"':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			switch c := s[i]; c {
			case '\\':
				i++
				if i >= len(s) {
					return v, fmt.Errorf("line %d: unfinished escape", line)
				}
				switch s[i] {
				case '\\', '"':
					b.WriteByte(s[i])
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					return v, fmt.Errorf("line %d: unknown escape \\%c", line, s[i])
				}
			case '"':
				if err := trailing(s[i+1:], line); err != nil {
					return v, err
				}
				v.Text, v.Quoted = b.String(), true
				return v, nil
			default:
				b.WriteByte(c)
			}
		}
		return v, fmt.Errorf("line %d: string is not closed", line)
	case '\'':
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return v, fmt.Errorf("line %d: string is not closed", line)
		}
		if err := trailing(s[end+2:], line); err != nil {
			return v, err
		}
		v.Text, v.Quoted = s[1:end+1], true
		return v, nil
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if s == "" {
		return v, fmt.Errorf("line %d: missing value", line)
	}
	v.Text = s
	return v, nil
}

// trailing checks that only a comment follows a closing quote.
func trailing(rest string, line int) error {
	if rest = strings.TrimSpace(rest); rest != "" && rest[0] != '#' {
		return fmt.Errorf("line %d: unexpected text after the string", line)
	}
	return nil
}
