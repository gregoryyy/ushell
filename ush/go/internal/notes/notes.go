// Package notes: notes on command output and files (TODO 2.25), `ush notes` and Ctrl-X n with
// the plugin (ushpy notes.py, the same rules).
//
// What is sent, all of it redacted: the last command the plugin recorded (--last), input piped to
// ush, and files named on the command line. A file is not sent if it is in a secret location (the
// policy's list, by the name given and by the file it resolves to), is not a regular file, holds
// binary data, or would take the material past Limit, the limit of piped input.
package notes

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gregoryyy/ushell/ush/go/internal/execute"
	"github.com/gregoryyy/ushell/ush/go/internal/policy"
)

// Limit is the most material sent, the same as for piped input.
const Limit = 200_000

// SplitWords: words naming existing paths are files; the rest is the instruction.
func SplitWords(words []string) (files []string, instruction string) {
	var rest []string
	for _, w := range words {
		if w == "" {
			continue
		}
		if _, err := os.Lstat(w); err == nil {
			files = append(files, w)
		} else {
			rest = append(rest, w)
		}
	}
	return files, strings.TrimSpace(strings.Join(rest, " "))
}

// ReadFile returns the text of a file to send, or "" and why it isn't sent.
func ReadFile(path string, budget int) (string, string) {
	abs, _ := filepath.Abs(path)
	real, err := filepath.EvalSymlinks(path)
	if err == nil {
		real, _ = filepath.Abs(real)
	} else if target, err := os.Readlink(path); err == nil { // a broken link: where it points
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(abs), target)
		}
		real = filepath.Clean(target)
	} else {
		real = abs
	}
	if policy.IsSecret(abs) || policy.IsSecret(real) {
		return "", "a secret location"
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) // a FIFO doesn't block the open
	if err != nil {
		return "", fmt.Sprintf("can't read it (%s)", reason(err))
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Sprintf("can't read it (%s)", reason(err))
	}
	if info.IsDir() {
		return "", "a directory; name the files in it"
	}
	if !info.Mode().IsRegular() {
		return "", "not a regular file"
	}
	if info.Size() > int64(budget) {
		return "", TooLarge(int(info.Size()))
	}
	data, _ := io.ReadAll(io.LimitReader(f, int64(budget)+1))
	if len(data) > budget {
		return "", TooLarge(len(data))
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", "binary"
	}
	return execute.Decode(data), ""
}

// reason is the system's message for a failed open, as Python's strerror gives it.
func reason(err error) string {
	var errno syscall.Errno
	if e, ok := err.(*os.PathError); ok {
		if n, ok := e.Err.(syscall.Errno); ok {
			errno = n
		}
	}
	if errno == 0 {
		return err.Error()
	}
	s := errno.Error()
	return strings.ToUpper(s[:1]) + s[1:]
}

// TooLarge is the reason a file over the limit isn't sent.
func TooLarge(size int) string {
	return fmt.Sprintf("too large (%s; at most %s in all)", SizeText(size), SizeText(Limit))
}

// SizeText: "18 bytes", "1.2 KB", "200 KB".
func SizeText(size int) string {
	if size < 1000 {
		return fmt.Sprintf("%d bytes", size)
	}
	return strings.Replace(strconv.FormatFloat(float64(size)/1000, 'f', 1, 64)+" KB", ".0 KB", " KB", 1)
}
