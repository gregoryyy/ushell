package execute

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func opts() Options { return Options{Timeout: 20 * time.Second, Limit: 32000} }

func TestOutputAndExitStatus(t *testing.T) {
	r, err := RunCommand(context.Background(), "echo out; echo err >&2; exit 3", opts())
	if err != nil {
		t.Fatal(err)
	}
	if *r.ExitCode != 3 || !strings.Contains(r.Output, "out") || !strings.Contains(r.Output, "err") {
		t.Errorf("%+v", r)
	}
	if r.Summary() != "exit 3" {
		t.Error(r.Summary())
	}
}

func TestPipefailAndNoRcFiles(t *testing.T) {
	r, _ := RunCommand(context.Background(), "false | true", opts())
	if *r.ExitCode == 0 {
		t.Error("pipefail is off")
	}
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".zshrc"), []byte("alias ls='echo HIJACKED'\n"), 0o644)
	o := opts()
	o.Env = append(CommandEnv(nil), "HOME="+home, "ZDOTDIR="+home)
	r, _ = RunCommand(context.Background(), "ls /", o)
	if strings.Contains(r.Output, "HIJACKED") {
		t.Error("rc files were read")
	}
}

func TestOutputIsBoundedKeepingHeadAndTail(t *testing.T) {
	o := opts()
	o.Limit = 1000
	r, _ := RunCommand(context.Background(), "echo START; seq 1 20000; echo END", o)
	if !r.Truncated || !strings.HasPrefix(r.Output, "START") || !strings.HasSuffix(strings.TrimSpace(r.Output), "END") {
		t.Errorf("truncated %v: %q…%q", r.Truncated, r.Output[:20], r.Output[len(r.Output)-20:])
	}
	if !strings.Contains(r.Output, "bytes omitted") || len(r.Output) > 1100 {
		t.Errorf("%d bytes", len(r.Output))
	}
}

func TestTimeoutStopsTheWholePipeline(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "survived")
	o := opts()
	o.Timeout = time.Second
	start := time.Now()
	r, _ := RunCommand(context.Background(), "trap '' TERM; (sleep 30; touch "+marker+") & sleep 30", o)
	if !r.TimedOut || time.Since(start) > 10*time.Second {
		t.Errorf("%+v after %v", r, time.Since(start))
	}
	time.Sleep(500 * time.Millisecond)
	if out, _ := exec.Command("pgrep", "-f", "touch "+marker).Output(); len(out) > 0 {
		t.Error("a process of the pipeline survived")
	}
}

func TestCancelKillsAndReports(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	start := time.Now()
	_, err := RunCommand(ctx, "sleep 30", opts())
	if !errors.Is(err, ErrCancelled) || time.Since(start) > 10*time.Second {
		t.Errorf("%v after %v", err, time.Since(start))
	}
}

func TestCredentialsAreNotPassedAndPagersAreOff(t *testing.T) {
	env := CommandEnv([]string{"ANTHROPIC_API_KEY=x", "USH_API_KEY=y", "PATH=/bin", "PAGER=less"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "ANTHROPIC") || strings.Contains(joined, "USH_API_KEY") || strings.Contains(joined, "PAGER=less") {
		t.Error(joined)
	}
	if !strings.Contains(joined, "PATH=/bin") || !strings.Contains(joined, "PAGER=cat") {
		t.Error(joined)
	}
}

func TestBinaryAndInvalidUTF8(t *testing.T) {
	r, _ := RunCommand(context.Background(), `printf '\377\376\000ok\200'`, opts())
	if !strings.Contains(r.Output, "ok") || !strings.Contains(r.Output, "�") {
		t.Errorf("%q", r.Output)
	}
	if Decode([]byte{0xff, 'a', 0xfe}) != "�a�" {
		t.Error("one replacement per byte")
	}
}

func TestScripts(t *testing.T) {
	for _, lang := range []string{"sh", "zsh", "python3"} {
		src := "echo hi-" + lang
		if lang == "python3" {
			src = "print('hi-python3')"
		}
		r, err := RunScript(context.Background(), lang, src, opts())
		if err != nil || !strings.Contains(r.Output, "hi-"+lang) {
			t.Errorf("%s: %v %q", lang, err, r.Output)
		}
	}
	if _, err := RunScript(context.Background(), "perl", "1", opts()); err == nil {
		t.Error("perl accepted")
	}
}
