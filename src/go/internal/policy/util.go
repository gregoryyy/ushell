package policy

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func hasPrefix(words, prefix []string) bool {
	if len(words) < len(prefix) {
		return false
	}
	for i := range prefix {
		if words[i] != prefix[i] {
			return false
		}
	}
	return true
}

func subset(args []string, allowed ...string) bool {
	ok := set(allowed...)
	for _, a := range args {
		if !ok[a] {
			return false
		}
	}
	return true
}

func onlyRunes(s, allowed string) bool {
	for _, c := range s {
		if !strings.ContainsRune(allowed, c) {
			return false
		}
	}
	return true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func isWordRune(c rune) bool {
	return c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c)
}

// pyStrip strips whitespace like Python's str.strip().
func pyStrip(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

// pyRepr quotes like Python's repr() of a string without quotes in it.
func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

// pyDirname and pyBasename split like Python's os.path: "/bin/ls" → "/bin", "ls".
func pyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	head := p[:i+1]
	if strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head
}

func pyBasename(p string) string { return p[strings.LastIndex(p, "/")+1:] }

func lexists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// realpath resolves like Python's os.path.realpath: symlinks as far as the path exists, the
// rest as written, made absolute and clean.
func realpath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	dir, rest := abs, ""
	for {
		parent, base := filepath.Dir(dir), filepath.Base(dir)
		if rest == "" {
			rest = base
		} else {
			rest = filepath.Join(base, rest)
		}
		if parent == dir {
			return abs
		}
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(resolved, rest)
		}
		dir = parent
	}
}
