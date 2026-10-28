package policy

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Parsing. The tokenizer follows `zsh -f` word rules closely enough to see the arguments a
// command will actually receive: quotes, backslashes, $NAME, leading ~, and unquoted globs are
// expanded here. Constructs it does not model (brace expansion, special parameters, a leading
// `=`) make the command need approval.

// Unsupported is a construct the tokenizer does not model; the command needs approval.
type Unsupported struct{ Reason string }

func (u *Unsupported) Error() string { return u.Reason }

func unsupported(reason string) error { return &Unsupported{reason} }

// Token is an operator or a word with its expanded words.
type Token struct {
	Op         bool
	Text       string   // operator text
	Words      []string // expanded words
	AdjacentOp bool     // a word immediately followed by an operator
}

var variableName = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)`)

func variable(command []rune, i int) (string, int, error) {
	m := variableName.FindStringSubmatch(string(command[i:]))
	if m == nil {
		return "", i, unsupported("uses a special shell parameter")
	}
	return os.Getenv(m[1]), i + len([]rune(m[0])), nil
}

func isOperator(c rune) bool { return strings.ContainsRune(operatorChars, c) }

// globEscape escapes text for a pattern, as Python's glob.escape: magic characters in brackets.
func globEscape(text string) string {
	var b strings.Builder
	for _, c := range text {
		if c == '*' || c == '?' || c == '[' {
			b.WriteRune('[')
			b.WriteRune(c)
			b.WriteRune(']')
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func word(command []rune, i int) (Token, int, error) {
	n := len(command)
	var literal, pattern strings.Builder
	magic := false
	start := i
	for i < n && command[i] != ' ' && command[i] != '\t' && !isOperator(command[i]) {
		c := command[i]
		switch {
		case c == '\'':
			end := -1
			for j := i + 1; j < n; j++ {
				if command[j] == '\'' {
					end = j
					break
				}
			}
			if end < 0 {
				return Token{}, i, unsupported("has an unterminated quote")
			}
			text := string(command[i+1 : end])
			i = end + 1
			literal.WriteString(text)
			pattern.WriteString(globEscape(text))
		case c == '"':
			i++
			var buf strings.Builder
			for {
				if i >= n {
					return Token{}, i, unsupported("has an unterminated quote")
				}
				c = command[i]
				if c == '"' {
					i++
					break
				}
				if c == '\\' && i+1 < n && strings.ContainsRune("\"\\$`", command[i+1]) {
					buf.WriteRune(command[i+1])
					i += 2
				} else if c == '$' {
					value, next, err := variable(command, i)
					if err != nil {
						return Token{}, i, err
					}
					buf.WriteString(value)
					i = next
				} else {
					buf.WriteRune(c)
					i++
				}
			}
			literal.WriteString(buf.String())
			pattern.WriteString(globEscape(buf.String()))
		case c == '\\':
			if i+1 >= n {
				return Token{}, i, unsupported("ends with a backslash")
			}
			literal.WriteRune(command[i+1])
			pattern.WriteString(globEscape(string(command[i+1])))
			i += 2
		case c == '$':
			value, next, err := variable(command, i) // zsh neither splits nor globs the value
			if err != nil {
				return Token{}, i, err
			}
			literal.WriteString(value)
			pattern.WriteString(globEscape(value))
			i = next
		case c == '{' && i+1 < n && command[i+1] == '}':
			literal.WriteString("{}") // empty braces are literal in zsh: find -exec ... {} +
			pattern.WriteString("{}")
			i += 2
		case c == '{' || c == '}':
			return Token{}, i, unsupported("uses brace expansion")
		case c == '#' && i == start:
			return Token{}, i, unsupported("contains a comment")
		case c == '=' && literal.Len() == 0:
			return Token{}, i, unsupported("uses = expansion") // also after empty quotes: ""=ls is =ls
		case c == '~' && i != start && literal.Len() == 0:
			// zsh drops the empty quotes first, so ""~ is the home directory (SECTEST F4)
			return Token{}, i, unsupported("uses ~ after empty quotes")
		default:
			magic = magic || c == '*' || c == '?' || c == '['
			literal.WriteRune(c)
			pattern.WriteRune(c)
			i++
		}
	}
	text, pat := literal.String(), pattern.String()
	if command[start] == '~' { // unquoted leading tilde
		head, tail, found := strings.Cut(text, "/")
		home := expandUser(head)
		sep := ""
		if found {
			sep = "/"
		}
		text = home + sep + tail
		pat = globEscape(home) + pat[len(head):]
	}
	var words []string
	if magic {
		matches, err := glob(pat)
		if err != nil {
			return Token{}, i, err
		}
		words = matches // no match: zsh reports an error and runs nothing
	} else {
		words = []string{text}
	}
	return Token{Words: words, AdjacentOp: i < n && isOperator(command[i])}, i, nil
}

// Tokens splits one command line into operators and expanded words.
func Tokens(command string) ([]Token, error) {
	r := []rune(command)
	var tokens []Token
	i, n := 0, len(r)
	for i < n {
		switch {
		case r[i] == ' ' || r[i] == '\t':
			i++
		case isOperator(r[i]):
			j := i
			for j < n && isOperator(r[j]) {
				j++
			}
			tokens = append(tokens, Token{Op: true, Text: string(r[i:j])})
			i = j
		default:
			t, next, err := word(r, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, t)
			i = next
		}
	}
	return tokens, nil
}

// expandUser expands ~ and ~name like Python's os.path.expanduser.
func expandUser(head string) string {
	if head == "~" {
		if home := os.Getenv("HOME"); home != "" {
			return home
		}
		if u, err := user.Current(); err == nil {
			return u.HomeDir
		}
		return head
	}
	if u, err := user.Lookup(head[1:]); err == nil {
		return strings.TrimRight(u.HomeDir, "/")
	}
	return head
}

// glob expands a pattern as Python's glob.glob(recursive="**" in pattern) does, which matches
// zsh's defaults: sorted, no hidden names unless the pattern names a dot, ** for any depth.
func glob(pattern string) ([]string, error) {
	recursive := strings.Contains(pattern, "**")
	absolute := strings.HasPrefix(pattern, "/")
	parts := strings.Split(strings.Trim(pattern, "/"), "/")
	starts := []string{""}
	if absolute {
		starts = []string{"/"}
	}
	matches := starts
	for idx, part := range parts {
		last := idx == len(parts)-1
		var next []string
		for _, base := range matches {
			dir := base
			if dir == "" {
				dir = "."
			}
			switch {
			case recursive && part == "**":
				found := []string{base}
				found = append(found, walkDirs(base, last)...)
				next = append(next, found...)
			case !hasMagic(part):
				name := filepath.Join(base, unescape(part))
				if base == "" {
					name = unescape(part)
				}
				if _, err := os.Lstat(name); err == nil {
					next = append(next, name)
				}
			default:
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, e := range entries {
					name := e.Name()
					if strings.HasPrefix(name, ".") && !strings.HasPrefix(part, ".") {
						continue
					}
					if ok, _ := filepath.Match(toGoPattern(part), name); ok {
						if !last && !isDir(filepath.Join(dir, name)) {
							continue
						}
						if base == "" {
							next = append(next, name)
						} else {
							next = append(next, filepath.Join(base, name))
						}
					}
				}
			}
			if len(next) > MaxGlobMatches {
				return nil, unsupported("uses a pattern that matches too many files to check")
			}
		}
		matches = next
	}
	if len(parts) == 1 && parts[0] == "" {
		matches = nil
	}
	out := dedupe(matches)
	if strings.HasSuffix(pattern, "/") {
		for i, m := range out {
			out[i] = m + "/"
		}
	}
	sort.Strings(out)
	if len(out) > MaxGlobMatches {
		return nil, unsupported("uses a pattern that matches too many files to check")
	}
	return out, nil
}

// walkDirs lists what ** matches below base: every non-hidden directory, and files too if ** is
// the last part.
func walkDirs(base string, files bool) []string {
	root := base
	if root == "" {
		root = "."
	}
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || files {
			rel := path
			if base == "" {
				rel = strings.TrimPrefix(path, "./")
			}
			out = append(out, rel)
		}
		if len(out) > MaxGlobMatches {
			return errors.New("too many")
		}
		return nil
	})
	return out
}

func hasMagic(part string) bool {
	plain := strings.NewReplacer("[*]", "", "[?]", "", "[[]", "").Replace(part)
	return strings.ContainsAny(plain, "*?[")
}

// unescape undoes globEscape for a part without magic.
func unescape(part string) string {
	return strings.NewReplacer("[*]", "*", "[?]", "?", "[[]", "[").Replace(part)
}

// toGoPattern turns a Python glob part into a filepath.Match pattern: backslashes are literal in
// Python's fnmatch, escapes in Go's.
func toGoPattern(part string) string {
	return strings.ReplaceAll(part, `\`, `\\`)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// The user's allowlist: rules that let commands the table doesn't know run without approval.
// A rule is a command name and leading arguments ("git status", "kubectl get"); it matches a
// simple command whose words start with it. Rules extend the table and never override it.

var ruleWord = regexp.MustCompile(`^[A-Za-z0-9._:@%+=,-]+$`)

// RuleWords is the most words a rule may have.
const RuleWords = 8

var runners = set("sh", "bash", "zsh", "dash", "ksh", "mksh", "fish", "csh", "tcsh", "busybox",
	"env", "sudo", "doas", "su", "xargs", "nice", "nohup", "timeout", "time", "exec", "eval",
	"source", ".", "command", "builtin", "watch", "parallel", "script", "expect", "chroot",
	"ssh", "osascript", "open", "launchctl", "at", "batch", "crontab", "screen", "tmux",
	"python", "perl", "ruby", "node", "deno", "bun", "php", "lua", "tclsh", "R", "Rscript",
	"java", "go", "make", "npm", "npx", "pip", "uv", "uvx", "git-shell")

var interpreter = regexp.MustCompile(`^(python|perl|ruby|node|php|lua|pip)[0-9.]*$`)

// Commands whose purpose is to change or send something; ush asks for them every time.
var changers = set("rm", "rmdir", "mv", "cp", "ln", "dd", "install", "mkfs", "newfs", "fdisk",
	"truncate", "shred", "srm", "chmod", "chown", "chgrp", "chflags", "xattr", "touch", "mkdir",
	"tee", "kill", "killall", "pkill", "shutdown", "reboot", "halt", "systemctl", "service",
	"rsync", "scp", "sftp", "ftp", "curl", "wget", "nc", "ncat", "socat", "telnet", "mail",
	"sendmail", "security", "passwd", "useradd", "userdel", "usermod", "dscl", "visudo")

// ParseRule returns the rule's words, or an error with the reason text can't be an allow rule.
func ParseRule(text string) ([]string, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, errors.New("an allow rule needs a command name")
	}
	if len(words) > RuleWords {
		return nil, fmt.Errorf("an allow rule has at most %d words", RuleWords)
	}
	for _, w := range words {
		if !ruleWord.MatchString(w) {
			return nil, fmt.Errorf("%s: allow rules are plain words, without paths, quotes, globs, "+
				"variables, or shell syntax", pyRepr(w))
		}
	}
	name := words[0]
	if strings.HasPrefix(name, "-") {
		return nil, errors.New("an allow rule starts with a command name")
	}
	if runners[name] || interpreter.MatchString(name) {
		return nil, fmt.Errorf("%s runs other commands or code, so allowing it would allow anything", name)
	}
	if changers[name] {
		return nil, fmt.Errorf("%s changes or sends data; ush asks before it every time", name)
	}
	if _, ok := commands[name]; ok {
		return nil, fmt.Errorf("%s is in ush's own read-only table; its check decides, and a rule "+
			"can't widen it", name)
	}
	return words, nil
}

func allowed(name string, args []string, rules [][]string) string {
	words := append([]string{name}, args...)
	for _, rule := range rules {
		if hasPrefix(words, rule) {
			return strings.Join(rule, " ")
		}
	}
	return ""
}

var envAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func simpleCommand(words []string, rules [][]string) Verdict {
	if len(words) == 0 {
		return reject("empty command in a pipeline or list")
	}
	if envAssignment.MatchString(words[0]) {
		return reject("sets environment variables for the command")
	}
	name, args := words[0], words[1:]
	if name == "sudo" {
		return Verdict{Reason: "runs a command with sudo", Sudo: true}
	}
	if strings.Contains(name, "/") {
		if !contains(SystemDirs, pyDirname(name)) {
			return reject(fmt.Sprintf("runs %s, which is not a known system command", name))
		}
		name = pyBasename(name)
	}
	if name == "xargs" {
		_, rest := xargsSplit(args)
		if len(rest) == 0 {
			return reject("xargs without a command")
		}
		inner := simpleCommand(rest, nil)
		if !inner.Auto {
			return inner
		}
		if e, ok := commands[pyBasename(rest[0])]; !ok || e.kind != meta {
			return reject("xargs passes unchecked paths to a file reader")
		}
		return Verdict{Auto: true}
	}
	e, ok := commands[name]
	if !ok {
		rule := allowed(name, args, rules)
		if rule == "" {
			return reject(fmt.Sprintf("%s is not a known read-only command", name))
		}
		if reason := checkPaths(args, false); reason != "" {
			return reject(reason)
		}
		return Verdict{Auto: true, Rule: rule}
	}
	if reason := e.check(args); reason != "" {
		return reject(reason)
	}
	if e.kind == content || e.kind == recursiveKind {
		if reason := checkPaths(args, e.kind == recursiveKind || isRecursive(name, args)); reason != "" {
			return reject(reason)
		}
	}
	return Verdict{Auto: true}
}

// Classify returns whether command (one shell command line) may run without approval. rules is
// the user's allowlist (ParseRule); a root session passes none.
func Classify(command string, rules [][]string) Verdict {
	if strings.TrimSpace(command) == "" {
		return reject("empty command")
	}
	if strings.ContainsAny(command, "\n\r") {
		return reject("spans several lines")
	}
	for _, f := range forbiddenFragments {
		if strings.Contains(command, f.fragment) {
			return reject("uses " + f.what)
		}
	}
	tokens, err := Tokens(command)
	if err != nil {
		return reject(err.Error())
	}
	cmds := [][]string{{}}
	appended := -1 // the last word token added to a command (not taken as a redirect target)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.Op {
			cmds[len(cmds)-1] = append(cmds[len(cmds)-1], tok.Words...)
			appended = i
			continue
		}
		var target *Token
		if i+1 < len(tokens) && !tokens[i+1].Op {
			target = &tokens[i+1]
		}
		switch {
		case Separators[tok.Text]:
			cmds = append(cmds, []string{})
		case outputRedirects[tok.Text] || tok.Text == ">&":
			if i > 0 {
				prev := tokens[i-1]
				if appended == i-1 && prev.AdjacentOp && len(prev.Words) == 1 && isDigits(prev.Words[0]) {
					last := cmds[len(cmds)-1]
					cmds[len(cmds)-1] = last[:len(last)-1] // the file descriptor of "2>" (not a target: "< 2>x", F7)
				}
			}
			if target == nil {
				return reject("has a redirection without a target")
			}
			if !(len(target.Words) == 1 && target.Words[0] == "/dev/null") &&
				!(tok.Text == ">&" && len(target.Words) == 1 && isDigits(target.Words[0])) {
				return reject("redirects output to a file")
			}
			i++
		case tok.Text == "<":
			if target == nil {
				return reject("has a redirection without a target")
			}
			if reason := checkPaths(target.Words, false); reason != "" {
				return reject(reason)
			}
			i++
		default:
			return reject(fmt.Sprintf("uses shell syntax %s", pyRepr(tok.Text)))
		}
	}
	var used []string
	sudo := false
	for _, words := range cmds {
		v := simpleCommand(words, rules)
		sudo = sudo || v.Sudo
		if !v.Auto {
			return Verdict{Reason: v.Reason, Sudo: sudo}
		}
		if v.Rule != "" && !contains(used, v.Rule) {
			used = append(used, v.Rule)
		}
	}
	return Verdict{Auto: true, Rule: strings.Join(used, ", ")}
}
