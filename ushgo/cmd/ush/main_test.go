package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionAndHelp(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--version"}, "ush 0.0.0-dev (go)\n"},
		{[]string{"--help"}, usage},
		{[]string{"-h"}, usage},
		{[]string{"help"}, usage},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tc.args, &stdout, &stderr); code != 0 {
			t.Errorf("%v: exit %d, want 0", tc.args, code)
		}
		if stdout.String() != tc.want || stderr.Len() != 0 {
			t.Errorf("%v: stdout %q, stderr %q", tc.args, stdout.String(), stderr.String())
		}
	}
}

// Until the port answers requests, every request is a usage error, so nothing runs by accident.
func TestRequestsAreRefusedAsUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"//why is my disk full?"}, {"--version", "x"}, {"ai", "status"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), "does not answer requests yet") {
			t.Errorf("%v: stdout %q, stderr %q", args, stdout.String(), stderr.String())
		}
	}
}

// The built binary, as the conformance tests will run it: version stamping and exit status.
func TestBuiltBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "ush")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=9.9.9", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "--version").Output()
	if err != nil || string(out) != "ush 9.9.9 (go)\n" {
		t.Fatalf("--version: %q, %v", out, err)
	}
	var exit *exec.ExitError
	if err := exec.Command(bin, "//hello").Run(); !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("request: %v, want exit status 2", err)
	}
}
