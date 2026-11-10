package notes

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFilesAreWordsNamingPaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	os.WriteFile("app.log", []byte("x"), 0o600)
	files, instruction := SplitWords([]string{"why", "app.log", "", "502?"})
	if strings.Join(files, ",") != "app.log" || instruction != "why 502?" {
		t.Errorf("%q %q", files, instruction)
	}
}

func TestReadFileSendsTextAndSaysWhyNot(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o700)
		os.WriteFile(path, data, 0o600)
		return path
	}
	text := write("app.log", []byte("ERROR x\n\xff"))
	if got, reason := ReadFile(text, Limit); got != "ERROR x\n�" || reason != "" {
		t.Errorf("%q %q", got, reason)
	}
	secret := write(".ssh/config", []byte("Host x"))
	link := filepath.Join(dir, "plain.txt")
	os.Symlink(secret, link)
	broken := filepath.Join(dir, "broken.txt")
	os.Symlink(filepath.Join(dir, ".ssh", "gone"), broken)
	unreadable := write("locked.log", []byte("x"))
	os.Chmod(unreadable, 0)
	fifo := filepath.Join(dir, "fifo")
	syscall.Mkfifo(fifo, 0o600)
	for _, tc := range []struct{ path, want string }{
		{write(".env", []byte("A=1")), "a secret location"},
		{secret, "a secret location"},
		{link, "a secret location"},   // by what the link resolves to
		{broken, "a secret location"}, // also when it's gone
		{write("bin.dat", []byte("\x7fELF\x00")), "binary"},
		{write("big.log", []byte(strings.Repeat("x", Limit+1))), "too large (200 KB; at most 200 KB in all)"},
		{dir, "a directory; name the files in it"},
		{fifo, "not a regular file"},
		{filepath.Join(dir, "missing"), "can't read it (No such file or directory)"},
	} {
		if got, reason := ReadFile(tc.path, Limit); got != "" || reason != tc.want {
			t.Errorf("%s: %q %q, want %q", tc.path, got, reason, tc.want)
		}
	}
	if os.Geteuid() != 0 {
		if _, reason := ReadFile(unreadable, Limit); reason != "can't read it (Permission denied)" {
			t.Error(reason)
		}
	}
	if _, reason := ReadFile(text, 3); reason != "too large (9 bytes; at most 200 KB in all)" {
		t.Error("the budget left:", reason)
	}
}

func TestSizeText(t *testing.T) {
	for size, want := range map[int]string{18: "18 bytes", 1234: "1.2 KB", 150_000: "150 KB", 200_001: "200 KB"} {
		if got := SizeText(size); got != want {
			t.Errorf("%d: %q, want %q", size, got, want)
		}
	}
}
