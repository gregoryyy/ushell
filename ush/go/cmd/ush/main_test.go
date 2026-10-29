package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregoryyy/ushell/ush/go/internal/agent"
	"github.com/gregoryyy/ushell/ush/go/internal/config"
	"github.com/gregoryyy/ushell/ush/go/internal/ui"
)

// The command line's behaviour is tested from outside by ushpy's black-box tests against the
// built binary (USH_TEST_BIN, ushgo TODO 1.10); these tests cover what needs no provider.

func TestVersionAndHelp(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--version"}, "ush 0.0.0-dev (go)\n"},
		{[]string{"--help"}, usage}, {[]string{"-h"}, usage}, {[]string{"help"}, usage},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tc.args, &stdout, &stderr); code != 0 || stdout.String() != tc.want || stderr.Len() != 0 {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", tc.args, code, stdout.String(), stderr.String())
		}
	}
	if !strings.Contains(usage, "ush explain COMMAND...") || !strings.Contains(usage, "-k, --checker MODEL") {
		t.Error("usage text")
	}
}

func TestUsageErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for _, args := range [][]string{{"frobnicate"}, {"-e", "ultra", "//x"}, {"--bogus"}, {"explain"}, {"ai", "frob"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("%v: exit %d, want 2; %s", args, code, stderr.String())
		}
	}
}

func TestBriefTakesThePartsAndAccess(t *testing.T) {
	explanation := "Summary: creates a file.\nParts:\n- `touch FILE`: creates FILE if missing.\nAccess:\n- files: writes FILE\n- network: none\nNotes: harmless."
	if got := brief(explanation); got != "Parts:\n- `touch FILE`: creates FILE if missing.\nAccess:\n- files: writes FILE\n- network: none" {
		t.Errorf("%q", got)
	}
	if brief("Parts:\n- a\nSummary: late") != "Parts:\n- a" || brief("free text") != "free text" ||
		brief("Notes: x\nAccess: none\nParts:\n- a") != "Parts:\n- a\nAccess: none" {
		t.Error("brief cases")
	}
}

// ush notes never runs as root: it would send what only root can read (TODO 2.25).
func TestNotesNeverAsRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "app.log")
	os.WriteFile(file, []byte("x"), 0o600)
	root := config.Identity{UID: 0, Name: "root", Home: "/var/root", OwnerUID: 0, OwnerName: "root", OwnerHome: "/var/root"}
	code, err := notesCommand([]string{file}, ui.New(), root, config.Settings{}, nil, agent.Options{})
	if code != 2 || err == nil || !strings.Contains(err.Error(), "doesn't run as root") {
		t.Errorf("exit %d, %v", code, err)
	}
}

// The built binary: version stamping, and a request without a key is a configuration error.
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
	cmd := exec.Command(bin, "//hello")
	home := t.TempDir()
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME=" + home, "XDG_STATE_HOME=" + home,
		"USH_PROVIDER=ollama", "USH_OLLAMA_URL=http://127.0.0.1:9"}
	var exit *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("request without a model: %v, want exit status 2", err)
	}
}
