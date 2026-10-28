// Package execute runs shell commands and auxiliary scripts with time and output bounds (ushpy
// execute.py).
package execute

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// Environment variables never passed to commands: provider credentials.
var secretEnvPrefixes = []string{"ANTHROPIC_", "USH_API_KEY", "CLAUDE_API_KEY", "OPENAI_API_KEY"}

var nonInteractiveEnv = map[string]string{"PAGER": "cat", "GIT_PAGER": "cat", "SYSTEMD_PAGER": "",
	"NO_COLOR": "1", "HOMEBREW_NO_AUTO_UPDATE": "1"}

// ScriptLanguages are the languages of auxiliary scripts.
var ScriptLanguages = []string{"sh", "zsh", "python3"}

// ErrCancelled: the user interrupted a running command.
var ErrCancelled = errors.New("cancelled")

// Result of a command.
type Result struct {
	ExitCode  *int    // nil if it didn't exit normally
	Output    string  // stdout and stderr, interleaved and bounded
	Truncated bool    // the middle of the output was dropped
	TimedOut  bool    // stopped at the time limit
	Duration  float64 // seconds
}

// Summary is "timed out" or "exit N".
func (r Result) Summary() string {
	if r.TimedOut {
		return "timed out"
	}
	if r.ExitCode == nil {
		return "exit None"
	}
	return fmt.Sprintf("exit %d", *r.ExitCode)
}

// FindShell returns the shell command prefix: zsh -f -o pipefail -c, else sh -c.
func FindShell() []string {
	candidates := []string{"/bin/zsh", "/usr/bin/zsh"}
	if p, err := exec.LookPath("zsh"); err == nil {
		candidates = append(candidates, p)
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.Mode()&0o111 != 0 {
			return []string{p, "-f", "-o", "pipefail", "-c"}
		}
	}
	return []string{"/bin/sh", "-c"}
}

// CommandEnv is the environment of commands: without provider credentials, pagers off.
func CommandEnv(base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	var env []string
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		secret := false
		for _, p := range secretEnvPrefixes {
			if strings.HasPrefix(name, p) {
				secret = true
			}
		}
		if _, set := nonInteractiveEnv[name]; !secret && !set {
			env = append(env, kv)
		}
	}
	for k, v := range nonInteractiveEnv {
		env = append(env, k+"="+v)
	}
	return env
}

// Collector keeps the head and tail of a stream within a limit of bytes.
type Collector struct {
	mu                   sync.Mutex
	headLimit, tailLimit int
	head, tail           []byte
	dropped              int
	echo                 func([]byte)
}

// NewCollector keeps 3/4 of limit from the start and the rest from the end.
func NewCollector(limit int, echo func([]byte)) *Collector {
	return &Collector{headLimit: limit * 3 / 4, tailLimit: limit - limit*3/4, echo: echo}
}

// Feed adds a chunk.
func (c *Collector) Feed(chunk []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.echo != nil {
		c.echo(chunk)
	}
	if room := c.headLimit - len(c.head); room > 0 {
		n := min(room, len(chunk))
		c.head = append(c.head, chunk[:n]...)
		chunk = chunk[n:]
	}
	if len(chunk) > 0 {
		c.tail = append(c.tail, chunk...)
		if excess := len(c.tail) - c.tailLimit; excess > 0 {
			c.dropped += excess
			c.tail = append([]byte(nil), c.tail[excess:]...)
		}
	}
}

// Text returns the kept output, decoded, and whether bytes were dropped.
func (c *Collector) Text() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dropped == 0 {
		return Decode(append(append([]byte(nil), c.head...), c.tail...)), false
	}
	marker := fmt.Sprintf("\n[... %d bytes omitted ...]\n", c.dropped)
	return Decode(append(append(append([]byte(nil), c.head...), marker...), c.tail...)), true
}

// Write lets a Collector be an io.Writer.
func (c *Collector) Write(p []byte) (int, error) {
	c.Feed(append([]byte(nil), p...))
	return len(p), nil
}

// Decode is UTF-8 with one replacement character per invalid byte, as Python's "replace".
func Decode(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var s strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			s.WriteRune(utf8.RuneError)
		} else {
			s.Write(b[:size])
		}
		b = b[size:]
	}
	return s.String()
}

// Options of a run.
type Options struct {
	Timeout     time.Duration
	Limit       int
	Interactive bool         // keep the terminal: stdin and process group (approved commands)
	Dir         string       // working directory; "" is the current one
	Env         []string     // nil: CommandEnv(nil)
	Echo        func([]byte) // called with each chunk of output
}

// Run runs argv, collecting combined output. Non-interactive commands get /dev/null as stdin and
// their own process group, so a timeout or cancellation stops the whole pipeline. Interactive
// commands keep the terminal's stdin and process group; the terminal delivers Ctrl-C to them.
// Cancelling ctx stops the command and returns ErrCancelled.
func Run(ctx context.Context, argv []string, o Options) (Result, error) {
	start := time.Now()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = o.Dir
	cmd.Env = o.Env
	if cmd.Env == nil {
		cmd.Env = CommandEnv(nil)
	}
	if o.Interactive {
		cmd.Stdin = os.Stdin
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	collector := NewCollector(o.Limit, o.Echo)
	read, write, err := os.Pipe()
	if err != nil {
		return Result{}, err
	}
	cmd.Stdout, cmd.Stderr = write, write
	if err := cmd.Start(); err != nil {
		read.Close()
		write.Close()
		code := 127
		return Result{ExitCode: &code, Output: err.Error() + "\n", Duration: time.Since(start).Seconds()}, nil
	}
	write.Close()
	pumped := make(chan struct{})
	go func() {
		defer close(pumped)
		buf := make([]byte, 65536)
		for {
			n, err := read.Read(buf)
			if n > 0 {
				collector.Feed(append([]byte(nil), buf[:n]...))
			}
			if err != nil {
				return
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(o.Timeout)
	defer timer.Stop()
	timedOut, cancelled := false, false
	select {
	case <-done:
	case <-timer.C:
		timedOut = true
		stop(cmd, o.Interactive, done)
	case <-ctx.Done():
		cancelled = true
		stop(cmd, o.Interactive, done)
	}
	select { // like ushpy's reader.join(2): don't wait for a background process holding the pipe
	case <-pumped:
	case <-time.After(2 * time.Second):
	}
	read.Close()
	output, truncated := collector.Text()
	result := Result{Output: output, Truncated: truncated, TimedOut: timedOut,
		Duration: time.Since(start).Seconds()}
	if cmd.ProcessState != nil {
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			code := status.ExitStatus()
			if status.Signaled() {
				code = -int(status.Signal())
			}
			result.ExitCode = &code
		}
	}
	if cancelled {
		return result, ErrCancelled
	}
	return result, nil
}

// stop sends SIGTERM to the command (its process group unless interactive), and SIGKILL if it
// is still running two seconds later.
func stop(cmd *exec.Cmd, interactive bool, done chan error) {
	send := func(sig syscall.Signal) {
		if interactive {
			_ = cmd.Process.Signal(sig)
		} else {
			_ = syscall.Kill(-cmd.Process.Pid, sig)
		}
	}
	send(syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		send(syscall.SIGKILL)
		<-done
	}
}

// RunCommand runs a command line with FindShell.
func RunCommand(ctx context.Context, command string, o Options) (Result, error) {
	return Run(ctx, append(FindShell(), command), o)
}

// RunScript writes source to a private temporary file and runs it with its interpreter.
func RunScript(ctx context.Context, language, source string, o Options) (Result, error) {
	workdir, err := os.MkdirTemp("", "ush-") // mode 0700
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(workdir)
	path := filepath.Join(workdir, "script")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Result{}, err
	}
	if _, err := io.WriteString(f, source); err != nil {
		f.Close()
		return Result{}, err
	}
	f.Close()
	var argv []string
	switch language {
	case "python3":
		python, err := exec.LookPath("python3")
		if err != nil {
			return Result{}, fmt.Errorf("python3 not found")
		}
		argv = []string{python, "-I", path}
	case "zsh":
		argv = []string{FindShell()[0], "-f", path}
	case "sh":
		argv = []string{"/bin/sh", path}
	default:
		return Result{}, fmt.Errorf("unsupported script language %q", language)
	}
	return Run(ctx, argv, o)
}
