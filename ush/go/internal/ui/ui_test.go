package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gregoryyy/ushell/ush/go/internal/check"
	"github.com/gregoryyy/ushell/ush/go/internal/execute"
)

const attack = "echo safe\r\x1b[2K\x1b]0;pwned\x07\x1b]52;c;aGk=\x07\u009b31m‮rm -rf ~"

func TestVisibleWritesControlsOut(t *testing.T) { // SECTEST F1, ushpy's cases
	got := Visible("a\rb\x1b[2K\x07\x7f\u009bc‮d​e\tf\ng")
	if got != "a^Mb^[[2K^G^?<0x9b>c<U+202E>d<U+200B>e\tf\ng" {
		t.Errorf("%q", got)
	}
}

func TestTheTerminalNeverWritesControlCharacters(t *testing.T) {
	var out, errOut bytes.Buffer
	term := &Terminal{Out: &out, Err: &errOut}
	term.ShowCommand(attack, "purpose\x1b[2K", "note\r")
	term.ShowProposal(attack, "purpose", []string{"note\x1b]52;c;aGk=\x07"})
	term.ShowScript("sh", "echo a\x1b]52;c;aGk=\x07\nrm x\r", "p", "")
	code := 0
	term.ShowOutput(execute.Result{ExitCode: &code, Output: "ok\x1b]0;pwned\x07\n" + attack}, 100)
	term.ShowCheck("checker", check.Verdict{Status: "flag", Reason: "reason\x1b[2J"})
	term.Info("info\x1b[2J")
	term.Warn("warn\x1b[2J")
	term.Error("error\x1b[2J")
	term.Answer("answer \x1b]52;c;aGk=\x07 ‮ done")
	term.FinishAnswer()
	all := out.String() + errOut.String()
	for _, c := range []string{"\x1b", "\r", "\x07", "\u009b", "‮"} {
		if strings.Contains(all, c) {
			t.Errorf("%q reached the terminal", c)
		}
	}
	if !strings.Contains(errOut.String(), "echo safe^M^[[2K^[]0;pwned^G") || !strings.Contains(errOut.String(), "<U+202E>rm -rf ~") {
		t.Error(errOut.String())
	}
}

func TestOutputIsShortenedToShownLines(t *testing.T) {
	var errOut bytes.Buffer
	term := &Terminal{Out: &bytes.Buffer{}, Err: &errOut}
	code := 1
	term.ShowOutput(execute.Result{ExitCode: &code, Output: "1\n2\n3\n4\n", Duration: 0.25}, 2)
	if !strings.Contains(errOut.String(), "… 2 more lines (use -v to see all)") || !strings.Contains(errOut.String(), "└ exit 1, 0.2s") {
		t.Error(errOut.String())
	}
}
