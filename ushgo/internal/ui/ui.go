// Package ui: terminal interaction (ushpy ui.py). Status goes to stderr, answers to stdout.
// Prompts read from the terminal even when stdin is a pipe; without a terminal ush cannot ask,
// so commands that need approval are declined.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gregoryyy/ushell/ushgo/internal/check"
	"github.com/gregoryyy/ushell/ushgo/internal/execute"
)

// hidden: what a terminal would act on instead of showing: C0 controls but tab and newline, DEL,
// C1 controls, and format characters that reorder or hide text (SECTEST F1).
var hidden = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f-\x{9f}\x{61c}\x{200b}-\x{200f}\x{2028}-\x{202e}\x{2060}-\x{2069}\x{feff}]`)

// Visible writes control and format characters out (^[ for ESC, ^M for a carriage return,
// <U+202E> for a bidi override), so the terminal never acts on them.
func Visible(text string) string {
	return hidden.ReplaceAllStringFunc(text, func(m string) string {
		c := []rune(m)[0]
		switch {
		case c < 0x20:
			return "^" + string(rune(c+0x40))
		case c == 0x7f:
			return "^?"
		case c <= 0x9f:
			return fmt.Sprintf("<0x%02x>", c)
		}
		return fmt.Sprintf("<U+%04X>", c)
	})
}

// Choice is the user's answer to an approval prompt.
type Choice struct {
	Action string // yes | no | edit | abort
	Text   string // edited command, or the reason for declining
}

// Terminal writes status and answers and reads approvals.
type Terminal struct {
	Out, Err io.Writer
	Color    bool
	Verbose  bool     // -v: explanations from the line editor in full
	History  []string // session lines, for Up in the line editor
	UshArgv  []string // how the line editor runs `ush explain`
	// Ask, if set, answers approval prompts instead of the terminal (tests).
	Ask        func(reason string, editable bool) Choice
	mu         sync.Mutex
	answerOpen bool
	tty        *os.File
	ttyReader  *bufio.Reader
	stdin      *bufio.Reader
}

// New writes to stdout and stderr; colours if stderr is a terminal and NO_COLOR is unset.
func New() *Terminal {
	self, _ := os.Executable()
	return &Terminal{Out: os.Stdout, Err: os.Stderr, Color: IsTerminal(os.Stderr) && os.Getenv("NO_COLOR") == "",
		UshArgv: []string{self}}
}

func (t *Terminal) style(code, text string) string {
	if t.Color {
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	return text
}

func (t *Terminal) dim(text string) string  { return t.style("2", text) }
func (t *Terminal) bold(text string) string { return t.style("1", text) }

func (t *Terminal) emit(text string) {
	t.endAnswer()
	fmt.Fprintln(t.Err, text)
}

func (t *Terminal) Info(text string)  { t.emit(t.dim(Visible(text))) }
func (t *Terminal) Warn(text string)  { t.emit(t.style("33", "ush: "+Visible(text))) }
func (t *Terminal) Error(text string) { t.emit(t.style("31", "ush: "+Visible(text))) }

// Progress is a thinking note between tool calls.
func (t *Terminal) Progress(text string) {
	t.emit(t.dim("… " + Visible(strings.Join(strings.Fields(text), " "))))
}

// Answer streams answer text to stdout.
func (t *Terminal) Answer(chunk string) {
	t.mu.Lock()
	t.answerOpen = true
	t.mu.Unlock()
	io.WriteString(t.Out, Visible(chunk))
}

func (t *Terminal) endAnswer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.answerOpen {
		t.answerOpen = false
		io.WriteString(t.Out, "\n")
	}
}

// FinishAnswer ends an open answer with a newline.
func (t *Terminal) FinishAnswer() { t.endAnswer() }

// ShowCommand shows a command in agent mode.
func (t *Terminal) ShowCommand(command, purpose, note string) {
	line := t.style("36", "$ ") + t.bold(Visible(command))
	if purpose != "" {
		line += t.dim("   # " + Visible(purpose))
	}
	t.emit(line)
	if note != "" {
		t.emit(t.dim("  " + Visible(note)))
	}
}

// ShowProposal shows a proposed command: the user runs it, not ush. Warnings in yellow.
func (t *Terminal) ShowProposal(command, purpose string, notes []string) {
	line := t.style("36", "→ ") + t.bold(Visible(command))
	if purpose != "" {
		line += t.dim("   # " + Visible(purpose))
	}
	t.emit(line)
	for _, n := range notes {
		t.emit(t.style("33", "  "+Visible(n)))
	}
}

// ShowScript shows a script with line numbers.
func (t *Terminal) ShowScript(language, source, purpose, note string) {
	line := t.style("36", "$ "+language+" script")
	if purpose != "" {
		line += t.dim("   # " + Visible(purpose))
	}
	t.emit(line)
	for n, l := range strings.Split(source, "\n") {
		t.emit(t.dim(fmt.Sprintf("%4d │ ", n+1)) + Visible(l))
	}
	if note != "" {
		t.emit(t.dim("  " + Visible(note)))
	}
}

// ShowCheck shows the checker's verdict under the proposal: dim when it has no objection.
func (t *Terminal) ShowCheck(model string, v check.Verdict) {
	if v.OK() {
		t.emit(t.dim(fmt.Sprintf("  checker %s: no objection", model)))
		return
	}
	label := "no verdict"
	if v.Status == "flag" {
		label = "flagged"
	}
	t.emit(t.style("33", fmt.Sprintf("  checker %s: %s: %s", model, label, Visible(v.Reason))))
}

// ShowOutput shows the first lines of a command's output and its status.
func (t *Terminal) ShowOutput(r execute.Result, lines int) {
	text := strings.TrimRight(r.Output, "\n")
	shown := strings.Split(text, "\n")
	if lines > 0 && len(shown) > lines {
		hiddenLines := len(shown) - lines
		shown = append(shown[:lines], fmt.Sprintf("… %d more lines (use -v to see all)", hiddenLines))
	} else if lines == 0 {
		shown = nil
	}
	for _, l := range shown {
		t.emit(t.dim("  │ " + Visible(l)))
	}
	status := fmt.Sprintf("  └ %s, %.1fs", r.Summary(), r.Duration)
	if r.TimedOut || (r.ExitCode != nil && *r.ExitCode != 0) {
		t.emit(t.style("33", status))
	} else {
		t.emit(t.dim(status))
	}
}

// terminal is the input terminal: stdin if it is one, else /dev/tty; nil without a terminal.
func (t *Terminal) terminal() *os.File {
	if IsTerminal(os.Stdin) {
		return os.Stdin
	}
	if t.tty == nil {
		f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return nil
		}
		t.tty = f
	}
	return t.tty
}

// CanPrompt: a terminal is available to ask.
func (t *Terminal) CanPrompt() bool { return t.Ask != nil || t.terminal() != nil }

// ReadLine reads one line; ok false at end of input. A session line (`ush>`) is read with zsh's
// line editor: the session's earlier lines as history, a proposal pre-filled, and the plugin's
// keys (Down explains, Up or Esc closes the explanation, Esc clears).
func (t *Terminal) ReadLine(prompt, prefill string, session bool) (string, bool) {
	tty := t.terminal()
	if tty == nil {
		return "", false
	}
	t.endAnswer()
	if session || prefill != "" {
		history := t.History
		if len(history) > 500 {
			history = history[len(history)-500:]
		}
		if used, line, ok := vared(prompt, prefill, tty, t.Verbose, history, t.UshArgv); used {
			if ok && session && strings.TrimSpace(line) != "" {
				t.History = append(t.History, line)
			}
			return line, ok
		}
	}
	var reader *bufio.Reader
	if tty == os.Stdin {
		io.WriteString(t.Out, prompt)
		if t.stdin == nil {
			t.stdin = bufio.NewReader(os.Stdin)
		}
		reader = t.stdin
	} else {
		io.WriteString(t.Err, prompt)
		if t.ttyReader == nil {
			t.ttyReader = bufio.NewReader(tty)
		}
		reader = t.ttyReader
	}
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimRight(line, "\n"), true
}

// Confirm asks whether to run a command.
func (t *Terminal) Confirm(reason string, editable bool) Choice {
	if reason != "" {
		t.emit(t.style("33", "  needs approval: "+Visible(reason)))
	}
	if t.Ask != nil {
		return t.Ask(reason, editable)
	}
	options := "[y]es / [n]o / [a]bort"
	if editable {
		options = "[y]es / [n]o / [e]dit / [a]bort"
	}
	for {
		reply, ok := t.ReadLine("  run it? "+options+": ", "", false)
		if !ok {
			return Choice{Action: "abort"}
		}
		switch strings.ToLower(strings.TrimSpace(reply)) {
		case "y", "yes":
			return Choice{Action: "yes"}
		case "a", "abort", "q", "quit":
			return Choice{Action: "abort"}
		case "n", "no", "":
			why, _ := t.ReadLine("  reason, for the assistant (optional): ", "", false)
			return Choice{Action: "no", Text: strings.TrimSpace(why)}
		case "e", "edit":
			if editable {
				if edited, ok := t.ReadLine("  command: ", "", false); ok && strings.TrimSpace(edited) != "" {
					return Choice{Action: "edit", Text: strings.TrimSpace(edited)}
				}
			}
		}
	}
}

// varedScript is ushpy's VARED: zsh's vared edits a session line, pre-filled with a proposal or
// empty, with earlier lines as history; the result comes back on the descriptor in $3. Exit 2: no
// zle; exit 1: end of input.
const varedScript = `
zmodload zsh/zle 2>/dev/null || exit 2
line=$1 prompt=$2 out=$3 offered=$4 n=$5
shift 5
HISTSIZE=1000
local h
for h in "${@[1,n]}"; do
  print -s -r -- "$h"
done
shift $n
typeset -ga ush=("$@")
typeset -gi explained=0
typeset -g kept_line= kept_text=  # the last explanation and the line it explains
_ush_init() {
  PREDISPLAY=$prompt
}
_ush_close() {
  explained=0
  PREDISPLAY=$prompt
}
_ush_esc() {
  if (( explained )); then
    _ush_close
  elif [[ -n $offered ]]; then
    BUFFER= CURSOR=0
    offered=
  fi
}
_ush_up() {
  if (( explained )); then
    _ush_close
  else
    zle up-line-or-history
  fi
}
_ush_down() {
  if [[ -n $offered && $HISTNO == $HISTCMD && $RBUFFER != *$'\n'* &&
        -n ${BUFFER//[[:space:]]/} ]]; then
    if [[ -z $kept_text || $BUFFER != $kept_line ]]; then
      PREDISPLAY="ush: explaining…"$'\n'"→ "
      zle -R
      local text
      if text=$("${ush[@]}" explain --inline -- "$BUFFER" 2>&1 </dev/null); then
        kept_line=$BUFFER kept_text=$text
      else
        kept_line= kept_text=
      fi
    else
      local text=$kept_text
    fi
    explained=1
    PREDISPLAY="$text"$'\n'"→ "
  else
    zle down-line-or-history
  fi
}
_ush_finish() {
  PREDISPLAY=$prompt
}
zle -N _ush_esc
zle -N _ush_up
zle -N _ush_down
zle -N zle-line-init _ush_init
zle -N zle-line-finish _ush_finish
bindkey -e
bindkey $'\e' _ush_esc
bindkey $'\e[A' _ush_up
bindkey $'\eOA' _ush_up
bindkey $'\e[B' _ush_down
bindkey $'\eOB' _ush_down
vared -h -e -p '' line || exit 1
print -rn -- "$line" >&$out
`

// vared edits a line with zsh; used is false if zsh can't edit here.
func vared(prompt, prefill string, tty *os.File, verbose bool, history, ushArgv []string) (used bool, line string, ok bool) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		return false, "", false
	}
	read, write, err := os.Pipe()
	if err != nil {
		return false, "", false
	}
	defer read.Close()
	ush := append([]string{}, ushArgv...)
	if verbose {
		ush = append(ush, "-v")
	}
	offered := ""
	if prefill != "" {
		offered = "1"
	}
	args := []string{"-f", "-c", varedScript, "zsh", prefill, prompt, "3", offered, strconv.Itoa(len(history))}
	args = append(append(args, history...), ush...)
	cmd := exec.Command(zsh, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.ExtraFiles = []*os.File{write} // descriptor 3
	if err := cmd.Start(); err != nil {
		write.Close()
		return false, "", false
	}
	write.Close()
	data, _ := io.ReadAll(read)
	err = cmd.Wait()
	code := 0
	if exit, isExit := err.(*exec.ExitError); isExit {
		code = exit.ExitCode()
	}
	if code == 2 {
		return false, "", false
	}
	if code != 0 {
		return true, "", false
	}
	return true, execute.Decode(data), true
}

// Input reads answers for `ush ai configure` from stdin, as Python's input(): the prompt on
// stdout, EOF as an empty answer.
func (t *Terminal) Input(prompt string) string {
	io.WriteString(t.Out, prompt)
	if t.stdin == nil {
		t.stdin = bufio.NewReader(os.Stdin)
	}
	line, _ := t.stdin.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

// Secret reads a line without echo when stdin is a terminal (Python's getpass); the prompt goes
// to stderr.
func (t *Terminal) Secret(prompt string) string {
	io.WriteString(t.Err, prompt)
	if IsTerminal(os.Stdin) {
		restore := setEcho(os.Stdin, false)
		defer func() { restore(); io.WriteString(t.Err, "\n") }()
	}
	if t.stdin == nil {
		t.stdin = bufio.NewReader(os.Stdin)
	}
	line, _ := t.stdin.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}
