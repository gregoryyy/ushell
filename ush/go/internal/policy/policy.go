// Package policy decides which shell commands ush may run without asking the user (ushpy
// policy.py, ported line by line; the shared corpus in ush/conform/policy.json keeps the two
// equal).
//
// The policy is a convenience gate, not a sandbox. A command line runs without a prompt only if
// ush can parse it completely, every simple command in it is a known read-only invocation, and no
// file-content reader touches a known secret location. Everything else needs explicit approval.
// Unknown syntax is never auto-approved.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SystemDirs: commands from these directories may be named by absolute path.
var SystemDirs = []string{"/bin", "/usr/bin", "/sbin", "/usr/sbin", "/usr/local/bin", "/opt/homebrew/bin"}

// Separators between simple commands.
var Separators = set("|", "|&", "&&", "||", ";")

var outputRedirects = set(">", ">>", ">|", "&>", "&>>")

const operatorChars = "();<>|&"

var forbiddenFragments = []struct{ fragment, what string }{
	{"`", "command substitution"}, {"$(", "command substitution"}, {"${", "parameter expansion"},
	{"$'", "ANSI-C quoting"}, {"$[", "arithmetic expansion"}, {"<(", "process substitution"},
	{">(", "process substitution"}, {"=(", "process substitution"},
}

// MaxGlobMatches: a pattern matching more files than this needs approval.
const MaxGlobMatches = 2000

// Verdict says whether a command line may run without approval, and why not.
type Verdict struct {
	Auto   bool
	Reason string
	Sudo   bool
	Rule   string // the user's allow rules that let commands run, if any did
}

func reject(reason string) Verdict { return Verdict{Reason: reason} }

// ---------------------------------------------------------------------------
// Secret locations. Reading their contents (not listing them) requires approval.

var secretComponents = set(".ssh", ".gnupg", ".aws", ".azure", ".kube", ".docker", ".password-store",
	"Keychains", ".netrc", ".pgpass", ".git-credentials", ".npmrc", ".pypirc", ".vault-token",
	"1Password", "Bitwarden", ".terraform.d", "gcloud")

var secretPathParts = []string{"/.config/ush", "/.config/anthropic", "/.config/gh", "/.local/state/ush"}

var secretNamePatterns = regexp.MustCompile(`(?i)^(id_[a-z0-9_]+|.*\.(pem|key|p12|pfx|kdbx|keychain(-db)?|gpg|asc)|\.env(\..*)?` +
	`|.*(secret|token|passw|credential).*|master\.passwd|shadow|gshadow|sudoers|ssh_host_.*_key)$`)

var secretAbsolute = []string{"/etc/sudoers.d", "/private/etc/sudoers.d", "/var/db/dslocal",
	"/private/var/db/dslocal", "/Library/Keychains", "/var/root", "/root", "/etc/ssh", "/private/etc/ssh"}

var homeParents = []string{"/Users", "/home"}

func isSecret(path string) bool {
	for _, p := range secretAbsolute {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	for _, p := range secretPathParts {
		if strings.Contains(path+"/", p+"/") {
			return true
		}
	}
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	for _, p := range parts {
		if secretComponents[p] {
			return true
		}
	}
	return len(parts) > 0 && secretNamePatterns.MatchString(parts[len(parts)-1])
}

// coversSecrets: recursively reading directory would reach a secret location.
func coversSecrets(directory string) bool {
	d := strings.TrimRight(directory, "/")
	if d == "" {
		d = "/"
	}
	if d == "/" || isSecret(d) {
		return true
	}
	roots := append(append([]string{}, secretAbsolute...), homeParents...)
	roots = append(roots, expandUser("~"))
	for _, h := range homeParents {
		entries, err := os.ReadDir(h)
		if err != nil {
			continue
		}
		for _, e := range entries {
			roots = append(roots, filepath.Join(h, e.Name()))
		}
	}
	for _, r := range roots {
		if r == d || strings.HasPrefix(r, d+"/") {
			return true
		}
	}
	return false
}

// checkPaths checks expanded arguments that are paths: existing files or words with a '/'.
// Other words (search patterns, keys) are skipped, so `grep token f` is not mistaken for reading
// a file named "token".
func checkPaths(words []string, recursive bool) string {
	for _, word := range words {
		// A dotfile name that looks secret (.env, .env.local) counts even before it exists.
		named := strings.HasPrefix(word, ".") && (secretNamePatterns.MatchString(word) || secretComponents[word])
		if word == "" || strings.HasPrefix(word, "-") || !(strings.Contains(word, "/") || lexists(word) || named) {
			continue
		}
		path := realpath(word)
		if isSecret(path) {
			return fmt.Sprintf("reads a secret location (%s)", path)
		}
		if info, err := os.Stat(path); recursive && err == nil && info.IsDir() && coversSecrets(path) {
			return fmt.Sprintf("reads %s recursively, which includes secret locations", path)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Known commands. A validator returns "" if the arguments are acceptable, or the reason the
// invocation needs approval.

type validator func(args []string) string

type kind int

const (
	meta kind = iota
	content
	recursiveKind
)

type entry struct {
	kind  kind
	check validator
}

func anyArgs([]string) string { return "" }

func noArgs(args []string) string {
	if len(args) > 0 {
		return "arguments change its behavior"
	}
	return ""
}

func shortFlags(args []string) map[rune]bool {
	flags := map[rune]bool{}
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && len(a) > 1 {
			for _, c := range a[1:] {
				flags[c] = true
			}
		}
	}
	return flags
}

func forbid(letters string, options ...string) validator {
	opts := set(options...)
	return func(args []string) string {
		for _, a := range args {
			base := strings.SplitN(a, "=", 2)[0]
			if opts[base] {
				return fmt.Sprintf("option %s may change the system", base)
			}
		}
		flags := shortFlags(args)
		var bad []string
		for _, c := range letters {
			if flags[c] {
				bad = append(bad, string(c))
			}
		}
		if len(bad) > 0 {
			sort.Strings(bad)
			return fmt.Sprintf("option -%s may change the system", strings.Join(bad, ""))
		}
		return ""
	}
}

func firstArgIn(allowed ...string) validator {
	ok := set(allowed...)
	return func(args []string) string {
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				if ok[a] {
					return ""
				}
				break
			}
		}
		return "only some subcommands are known to be read-only"
	}
}

func prefixIn(allowed ...[]string) validator {
	return func(args []string) string {
		for _, p := range allowed {
			if hasPrefix(args, p) {
				return ""
			}
		}
		return "only some subcommands are known to be read-only"
	}
}

func findCheck(args []string) string {
	bad := set("-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0", "-fprintf", "-fls")
	for _, a := range args {
		if bad[a] {
			return fmt.Sprintf("find %s runs commands or changes files", a)
		}
	}
	return ""
}

var sedPrint = regexp.MustCompile(`^((\d+|\$)(,(\d+|\$))?|/[^/]*/)?[pdq]$`)

// sedSubst: s/re/replacement/flags with any delimiter that isn't a word character, space or
// backslash; flags from gIip and digits (ushpy's SED_SUBST, without backreferences).
func sedSubst(s string) bool {
	r := []rune(s)
	if len(r) < 4 || r[0] != 's' {
		return false
	}
	d := r[1]
	if d == '\\' || d == ' ' || d == '\t' || d == '\n' || isWordRune(d) {
		return false
	}
	i, parts := 2, 0
	for i < len(r) && parts < 2 {
		switch {
		case r[i] == '\\':
			if i+1 >= len(r) {
				return false
			}
			i += 2
		case r[i] == d:
			parts++
			i++
		default:
			i++
		}
	}
	if parts < 2 {
		return false
	}
	for _, c := range r[i:] {
		if !strings.ContainsRune("gIip0123456789", c) {
			return false
		}
	}
	return true
}

func sedCheck(args []string) string {
	var scripts, positional []string
	expectScript := false
	for _, a := range args {
		switch {
		case expectScript:
			scripts = append(scripts, a)
			expectScript = false
		case a == "-e" || a == "--expression":
			expectScript = true
		case a == "--quiet" || a == "--silent" || a == "--regexp-extended" ||
			(strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && len(a) > 1 && onlyRunes(a[1:], "nEru")):
		case strings.HasPrefix(a, "-"):
			return fmt.Sprintf("sed option %s is not known to be read-only", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(scripts) == 0 {
		if len(positional) == 0 {
			return "sed without a script"
		}
		scripts = positional[:1]
	}
	for _, script := range scripts {
		for _, part := range strings.Split(script, ";") {
			part = pyStrip(part)
			if !sedPrint.MatchString(part) && !sedSubst(part) {
				return "sed script is not a simple print or substitution"
			}
		}
	}
	return ""
}

func awkCheck(args []string) string {
	program := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			program = a
			break
		}
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-f") {
			return "awk programs from files are not checked"
		}
	}
	for _, word := range []string{"system", "getline", "|", ">", "close", "fflush", "ENVIRON", "printf >"} {
		if strings.Contains(program, word) {
			return fmt.Sprintf("awk program uses %s", pyRepr(word))
		}
	}
	return ""
}

func jqCheck(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && (strings.Contains(a, "env") || strings.Contains(a, "$ENV") ||
			strings.Contains(a, "input_filename")) {
			return "jq program reads the environment"
		}
	}
	return ""
}

func xargsSplit(args []string) ([]string, []string) {
	takesValue := set("-n", "-I", "-P", "-L", "-s", "-d", "-E", "-a", "--max-args", "--max-procs",
		"--delimiter", "--arg-file")
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		if takesValue[args[i]] {
			i += 2
		} else {
			i++
		}
	}
	if i > len(args) {
		i = len(args)
	}
	return args[:i], args[i:]
}

func psCheck(args []string) string {
	for _, a := range args {
		if (strings.HasPrefix(a, "-") && strings.Contains(a[1:], "E")) || (!strings.HasPrefix(a, "-") && strings.Contains(a, "e")) {
			return "ps would print process environments"
		}
	}
	return ""
}

func commandCheck(args []string) string {
	if len(args) > 0 && (args[0] == "-v" || args[0] == "-V") {
		return ""
	}
	return "command runs another command"
}

func topCheck(args []string) string {
	if contains(args, "-l") || contains(args, "-b") {
		return ""
	}
	return "top needs -l (macOS) or -b (Linux) to exit"
}

func dateCheck(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "+") && a != "-u" && a != "-R" && a != "-I" && a != "--utc" {
			return "date with arguments may set the clock"
		}
	}
	return ""
}

func sysctlCheck(args []string) string {
	if shortFlags(args)['w'] {
		return "sysctl would change a kernel setting"
	}
	for _, a := range args {
		if strings.Contains(a, "=") {
			return "sysctl would change a kernel setting"
		}
	}
	return ""
}

func ifconfigCheck(args []string) string {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "-a" || args[0] == "-l" || !strings.HasPrefix(args[0], "-"))) {
		return ""
	}
	return "ifconfig with arguments may reconfigure an interface"
}

func ipCheck(args []string) string {
	mutating := set("add", "del", "delete", "change", "replace", "flush", "set", "append",
		"prepend", "exec", "save", "restore", "up", "down")
	for _, a := range args {
		if mutating[a] {
			return "ip would change network configuration"
		}
	}
	return ""
}

// dnsCheck: a name looked up reaches the resolver and the name's own servers, so it can carry
// data out (SECTEST F2): a lookup needs approval. Options alone (dig -v) don't look anything up.
func dnsCheck(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "+") {
			return "a DNS lookup sends the name to outside servers; the name can carry data"
		}
	}
	return ""
}

func positionalAtMost(n int) validator {
	return func(args []string) string {
		count := 0
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				count++
			}
		}
		if count > n {
			return "an extra file argument would be written"
		}
		return ""
	}
}

func all(validators ...validator) validator {
	return func(args []string) string {
		for _, v := range validators {
			if reason := v(args); reason != "" {
				return reason
			}
		}
		return ""
	}
}

func onlyIf(ok func(a []string) bool, reason string) validator {
	return func(args []string) string {
		if ok(args) {
			return ""
		}
		return reason
	}
}

func first(a []string) string {
	if len(a) > 0 {
		return a[0]
	}
	return ""
}

var commands = map[string]entry{
	// metadata and system state
	"ls": {meta, anyArgs}, "du": {meta, anyArgs}, "df": {meta, anyArgs}, "stat": {meta, anyArgs},
	"file": {meta, forbid("C", "--compile")}, "wc": {meta, anyArgs}, "find": {meta, findCheck},
	"tree": {meta, anyArgs}, "which": {meta, anyArgs}, "whereis": {meta, anyArgs}, "type": {meta, anyArgs},
	"basename": {meta, anyArgs}, "dirname": {meta, anyArgs}, "realpath": {meta, anyArgs},
	"readlink": {meta, anyArgs}, "pwd": {meta, anyArgs}, "cd": {meta, anyArgs},
	"command": {meta, commandCheck}, "whoami": {meta, anyArgs},
	"id": {meta, anyArgs}, "groups": {meta, anyArgs}, "hostname": {meta, noArgs},
	"uname": {meta, anyArgs}, "sw_vers": {meta, anyArgs}, "uptime": {meta, anyArgs},
	"date": {meta, dateCheck}, "sysctl": {meta, sysctlCheck}, "vm_stat": {meta, anyArgs},
	"vmstat": {meta, anyArgs}, "iostat": {meta, anyArgs}, "ps": {meta, psCheck},
	"pgrep": {meta, anyArgs}, "top": {meta, topCheck}, "lsof": {meta, anyArgs},
	"mount": {meta, noArgs}, "nproc": {meta, anyArgs}, "free": {meta, anyArgs},
	"lsblk": {meta, anyArgs}, "findmnt": {meta, anyArgs}, "lscpu": {meta, anyArgs},
	"lspci": {meta, anyArgs}, "lsusb": {meta, anyArgs}, "lsmod": {meta, anyArgs},
	"getconf": {meta, anyArgs}, "locale": {meta, anyArgs}, "echo": {meta, anyArgs},
	"printf": {meta, anyArgs}, "true": {meta, anyArgs}, "false": {meta, anyArgs},
	"test": {meta, anyArgs}, "[": {meta, anyArgs}, "sleep": {meta, anyArgs},
	"md5": {meta, anyArgs}, "md5sum": {meta, anyArgs}, "shasum": {meta, anyArgs},
	"sha256sum": {meta, anyArgs}, "cksum": {meta, anyArgs},
	"netstat": {meta, anyArgs}, "ss": {meta, anyArgs}, "ip": {meta, ipCheck},
	"ifconfig": {meta, ifconfigCheck}, "dig": {meta, dnsCheck}, "host": {meta, dnsCheck},
	"nslookup": {meta, dnsCheck},
	// macOS
	"diskutil": {meta, prefixIn([]string{"list"}, []string{"info"}, []string{"apfs", "list"},
		[]string{"apfs", "listSnapshots"}, []string{"apfs", "listVolumeGroups"})},
	"tmutil": {meta, firstArgIn("listlocalsnapshots", "listlocalsnapshotdates", "listbackups",
		"destinationinfo", "status", "latestbackup", "machinedirectory", "isexcluded")},
	"log":             {content, firstArgIn("show")},
	"system_profiler": {meta, anyArgs},
	"pmset":           {meta, onlyIf(func(a []string) bool { return first(a) == "-g" }, "only pmset -g is read-only")},
	"launchctl":       {meta, firstArgIn("list", "print", "print-disabled", "blame")},
	"defaults":        {content, firstArgIn("read", "read-type", "domains")},
	"mdutil": {meta, onlyIf(func(a []string) bool {
		return subset(a, "-s", "-a", "-sa", "-as") || first(a) == "-s"
	}, "only mdutil -s is read-only")},
	"mdfind": {meta, anyArgs}, "csrutil": {meta, firstArgIn("status")},
	"spctl": {meta, onlyIf(func(a []string) bool {
		return len(a) > 0 && (a[0] == "--status" || a[0] == "--assess" || a[0] == "-a")
	}, "only spctl --status/--assess are read-only")},
	"ioreg": {meta, anyArgs},
	"networksetup": {meta, onlyIf(func(a []string) bool {
		return len(a) > 0 && (strings.HasPrefix(a[0], "-get") || strings.HasPrefix(a[0], "-list"))
	}, "only -get*/-list* are read-only")},
	"scutil": {meta, onlyIf(func(a []string) bool {
		return len(a) > 0 && (a[0] == "--dns" || a[0] == "--proxy" || a[0] == "--nwi" || a[0] == "--get")
	}, "only scutil --dns/--proxy/--nwi/--get are read-only")},
	"fdesetup": {meta, firstArgIn("status", "isactive")},
	"kmutil":   {meta, firstArgIn("showloaded")},
	"softwareupdate": {meta, onlyIf(func(a []string) bool {
		return len(a) > 0 && (a[0] == "-l" || a[0] == "--list" || a[0] == "--history")
	}, "only softwareupdate --list/--history are read-only")},
	"pkgutil": {meta, onlyIf(func(a []string) bool {
		return len(a) > 0 && set("--pkgs", "--pkg-info", "--files", "--file-info", "--pkgs-plist")[a[0]]
	}, "only pkgutil queries are read-only")},
	"xcode-select": {meta, onlyIf(func(a []string) bool {
		return len(a) == 1 && (a[0] == "-p" || a[0] == "--print-path" || a[0] == "-v")
	}, "only xcode-select -p is read-only")},
	"nvram": {meta, onlyIf(func(a []string) bool {
		return len(a) > 0 && subset(a, "-p", "-x", "-xp", "-px")
	}, "nvram with arguments may change firmware variables")},
	"plutil": {content, onlyIf(func(a []string) bool {
		return len(a) > 0 && (a[0] == "-p" || a[0] == "-lint")
	}, "only plutil -p/-lint are read-only")},
	// Linux services
	"journalctl": {content, forbid("f", "--vacuum-size", "--vacuum-time", "--vacuum-files", "--rotate",
		"--flush", "--sync", "--relinquish-var", "--setup-keys", "--follow")},
	"systemctl": {meta, firstArgIn("status", "list-units", "list-unit-files", "list-timers", "show",
		"is-active", "is-enabled", "is-failed", "cat", "list-dependencies")},
	"dmesg": {content, forbid("cCDEnw", "--clear", "--read-clear", "--console-off", "--console-on",
		"--console-level", "--follow")},
	// tools with read-only subcommands
	"docker": {meta, prefixIn([]string{"system", "df"}, []string{"ps"}, []string{"images"},
		[]string{"image", "ls"}, []string{"volume", "ls"}, []string{"container", "ls"},
		[]string{"info"}, []string{"version"})},
	"brew": {meta, onlyIf(func(a []string) bool {
		return len(a) > 0 && (set("list", "ls", "info", "config", "--cache", "--prefix", "--cellar",
			"--repository", "outdated", "leaves", "deps", "uses", "doctor", "--version")[a[0]] ||
			(a[0] == "cleanup" && (contains(a, "-n") || contains(a, "--dry-run"))))
	}, "this brew command changes packages")},
	"xcrun": {meta, prefixIn([]string{"simctl", "list"})},
	// file contents
	"cat": {content, anyArgs}, "head": {content, anyArgs},
	"tail": {content, forbid("fF", "--follow")},
	"grep": {content, anyArgs}, "egrep": {content, anyArgs}, "fgrep": {content, anyArgs},
	"rg": {recursiveKind, forbid("", "--pre", "--pre-glob")}, "strings": {content, anyArgs},
	"od": {content, anyArgs}, "hexdump": {content, anyArgs},
	"xxd":  {content, all(forbid("r"), positionalAtMost(1))},
	"sort": {content, forbid("o", "--output", "--compress-program")},
	"uniq": {content, positionalAtMost(1)},
	"cut":  {content, anyArgs}, "tr": {content, anyArgs}, "column": {content, anyArgs},
	"nl": {content, anyArgs}, "fold": {content, anyArgs}, "paste": {content, anyArgs},
	"comm": {content, anyArgs}, "diff": {content, anyArgs}, "cmp": {content, anyArgs},
	"base64": {content, forbid("o", "--output")},
	"sed":    {content, sedCheck}, "awk": {content, awkCheck}, "jq": {content, jqCheck},
}

func isRecursive(name string, args []string) bool {
	if name == "rg" {
		return true
	}
	if name == "grep" || name == "egrep" || name == "fgrep" {
		flags := shortFlags(args)
		if flags['r'] || flags['R'] {
			return true
		}
		for _, a := range args {
			if a == "--recursive" || a == "--dereference-recursive" || a == "--directories=recurse" {
				return true
			}
		}
	}
	return false
}
