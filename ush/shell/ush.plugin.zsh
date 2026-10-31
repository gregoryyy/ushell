# ush zsh integration. Source this file from ~/.zshrc:
#
#   source /path/to/ush/shell/ush.plugin.zsh
#
# Natural-language requests contain apostrophes, question marks, and other
# characters that zsh would otherwise parse ("don't" opens a quote, "why?" is a
# glob). Before zsh parses an accepted line, this plugin quotes the request part of
#
#   ush //REQUEST            sudo [OPTIONS] ush //REQUEST            // REQUEST
#
# so ush receives it verbatim. Under sudo, `ush` is replaced by its absolute path
# because sudo's secure_path usually does not include the install directory.
#
# In propose mode, ush's proposal is put on the next command line, and what you then
# run there is recorded for `//` to continue with (see the hooks at the end).
#
# Ctrl-N takes notes on command output and files with the model (see _ush_notes below).
#
# Settings (set before sourcing):
#   USH_BARE_SLASH=0   do not treat a line starting with `//` as `ush //...`
#   USH_BIN=PATH       ush executable to use (default: `ush` found on PATH)
#   USH_NOTES_KEY=KEYS the notes key, as bindkey writes it (default: ^N; empty: none)

[[ -o interactive ]] || return 0
(( ${+_ush_plugin_loaded} )) && return 0
typeset -g _ush_plugin_loaded=1

# Print the rewritten form of $1, or nothing if the line is not an ush request.
_ush_rewrite() {
  emulate -L zsh
  local line=$1 lead sudo_part req bin
  [[ $line == *$'\n'* ]] && return 1
  if [[ $line =~ '^([[:space:]]*)(sudo([[:space:]]+-[^[:space:]]+)*[[:space:]]+)?ush[[:space:]]+(//.*)$' ]]; then
    lead=$match[1] sudo_part=$match[2] req=$match[4]
  elif [[ ${USH_BARE_SLASH:-1} != 0 && $line =~ '^([[:space:]]*)(//.*)$' ]]; then
    lead=$match[1] sudo_part= req=$match[2]
  else
    return 1
  fi
  bin=${USH_BIN:-ush}
  if [[ -n $sudo_part ]]; then
    # sudo resets PATH; pass the absolute path of the ush found here.
    [[ $bin == /* ]] || bin=${commands[$bin]:-$bin}
  fi
  if [[ $req == // ]]; then
    print -r -- "$lead$sudo_part${(q-)bin} //"
  else
    print -r -- "$lead$sudo_part${(q-)bin} ${(qq)req}"
  fi
}

_ush_line_finish() {
  local rewritten
  (( _ush_explained )) && PREDISPLAY=
  rewritten=$(_ush_rewrite "$BUFFER") && BUFFER=$rewritten
}

autoload -Uz add-zle-hook-widget
add-zle-hook-widget zle-line-finish _ush_line_finish

# Completion for subcommands and options.
_ush() {
  local -a subcommands
  subcommands=(
    'ai:configure, inspect, or clear the AI provider key'
    'history:list recent requests and the commands they ran'
    'replay:run the commands of an earlier request again, without AI'
    'explain:explain a command line with the model, without running it'
    'notes:notes on files or piped input with the model'
    'allow:list, add, or remove commands that may run without approval'
    'help:show usage'
  )
  if (( CURRENT == 2 )); then
    _describe -t commands 'ush command' subcommands
    _arguments '(-r --run)'{-r,--run}'[agent mode: ush runs the commands]' \
      '(-N --new)'{-N,--new}'[start a new conversation]' \
      '(-n --dry-run)'{-n,--dry-run}'[show commands without running them]' \
      '(-c --confirm)'{-c,--confirm}'[confirm every command]' \
      '(-1 --once)'{-1,--once}'[answer once, no follow-up questions]' \
      '(-v --verbose)'{-v,--verbose}'[show complete command output]'
  elif [[ $words[2] == ai ]]; then
    _values 'ai command' configure status clear-key
  fi
}
(( ${+functions[compdef]} )) && compdef _ush ush

# Propose mode (ARCH). ush leaves its proposal in a private directory (USH_HANDOFF, made
# here, mode 0700, removed when the shell exits); before the next prompt it goes onto the
# command line. Whatever runs from that line is recorded there for `//`: the command line,
# its exit status, and, unless ush marked the proposal interactive, its output. The output
# goes through tee, so while that one command runs its stdout and stderr are a pipe.
# Not as root: root doesn't get proposals on its command line.
typeset -g _ush_dir= _ush_state= _ush_mode= _ush_line= _ush_tee= _ush_out= _ush_err=
typeset -ga _ush_saved=()   # the key bindings a proposal replaced, as `bindkey -L` prints them
typeset -gA _ush_orig=()    # key sequence -> the widget it had
typeset -gi _ush_explained=0  # an explanation is shown above the proposal
typeset -g _ush_kept_line= _ush_kept_text=  # the last explanation and the line it explains
typeset -gi _ush_noting=0     # Ctrl-N ran the command: notes on it before the next prompt
typeset -g _ush_notes_ask=    # and the question for them
zmodload -F zsh/zselect b:zselect 2>/dev/null

_ush_precmd() {
  local st=$? content
  _ush_keys_restore
  if [[ $_ush_state == running ]]; then
    _ush_end_capture
    print -r -- "$st"$'\n'"$_ush_line" >| $_ush_dir/ran
    if (( _ush_noting )); then
      local -a ask=(--last)
      [[ -n $_ush_notes_ask ]] && ask+=(-- $_ush_notes_ask)
      command ${USH_BIN:-ush} notes $ask
    fi
  fi
  _ush_state= _ush_noting=0 _ush_notes_ask=
  [[ -f $_ush_dir/proposal && ! -L $_ush_dir/proposal ]] || return 0
  content=$(<$_ush_dir/proposal)
  command rm -f -- $_ush_dir/proposal
  _ush_mode=${content%%$'\n'*}
  print -rz -- "${content#*$'\n'}"
  _ush_state=offered
  _ush_keys_bind
}

_ush_preexec() {
  _ush_keys_restore
  [[ $_ush_state == offered ]] || return 0
  _ush_state=
  # A new request instead of the proposal: nothing of it ran.
  [[ $1 =~ '^[[:space:]]*(sudo([[:space:]]+-[^[:space:]]+)*[[:space:]]+)?([^[:space:]]*/)?ush([[:space:]]|$)' ]] &&
    return 0
  _ush_state=running _ush_line=$1
  command rm -f -- $_ush_dir/output $_ush_dir/fifo
  [[ $_ush_mode == capture ]] || return 0
  command mkfifo -m 600 -- $_ush_dir/fifo || return 0
  command tee -- $_ush_dir/output < $_ush_dir/fifo &!
  _ush_tee=$!
  exec {_ush_out}>&1 {_ush_err}>&2 >$_ush_dir/fifo 2>&1
  command rm -f -- $_ush_dir/fifo
}

_ush_end_capture() {
  [[ -n $_ush_tee ]] || return 0
  exec 1>&$_ush_out 2>&$_ush_err {_ush_out}>&- {_ush_err}>&-
  local i
  for i in {1..200}; do  # until tee has written everything, at most 2 s
    kill -0 $_ush_tee 2>/dev/null || break
    zselect -t 1 2>/dev/null || sleep 0.01
  done
  _ush_tee=
}

# Two keys act on a proposal while it is on the command line, and only then; their own bindings
# come back as soon as the line is run, cleared, or abandoned:
# - Down, past the last line of the proposal, explains it (`ush explain`, with the model). The
#   explanation shows above the command, as display-only text (PREDISPLAY); the cursor stays in
#   the command, to edit it or run it with Enter. Elsewhere Down does what it did before. Down
#   again on the same line shows the same explanation; only an edited line is explained anew.
# - Up closes the explanation; without one it is history, as always.
# - Esc closes the explanation, or else removes the proposal, to start from scratch; `//` then
#   hears that it didn't run. Only in the emacs keymap: in vi mode Esc switches to command mode
#   (Ctrl-U clears the line there).
_ush_discard() {
  if (( _ush_explained )); then
    _ush_explained=0
    PREDISPLAY=
    return
  fi
  _ush_state=
  BUFFER= CURSOR=0
  _ush_keys_restore
  zle -M "ush: proposal removed"
}

_ush_up() {
  if (( _ush_explained )); then
    _ush_explained=0
    PREDISPLAY=
  else
    local widget=${_ush_orig[$KEYS]:-up-line-or-history}
    [[ $widget == undefined-key ]] || zle $widget
  fi
}

_ush_explain() {
  if [[ $_ush_state == offered && $HISTNO == $HISTCMD && $RBUFFER != *$'\n'* &&
        -n ${BUFFER//[[:space:]]/} ]]; then
    local text
    if [[ -z $_ush_kept_text || $BUFFER != $_ush_kept_line ]]; then
      PREDISPLAY=$'\n'"ush: explaining…"$'\n'"→ "
      zle -R
      if text=$(command ${USH_BIN:-ush} explain --inline -- "$BUFFER" 2>&1 </dev/null); then
        _ush_kept_line=$BUFFER _ush_kept_text=$text
      else
        _ush_kept_line= _ush_kept_text=
      fi
    else
      text=$_ush_kept_text
    fi
    _ush_explained=1
    PREDISPLAY=$'\n'"$text"$'\n'"→ "
  else
    local widget=${_ush_orig[$KEYS]:-down-line-or-history}
    [[ $widget == undefined-key ]] || zle $widget
  fi
}

# Ctrl-N: notes with the model (`ush notes`, TODO 2.25), printed before the next prompt, so
# they stay in the scrollback. A comment ending the line is the question: `make # why?`.
# - On an empty line, or one with only a question: notes on the output of the last command the
#   plugin recorded.
# - On a command line: the command runs with its output recorded, as a proposal runs (through a
#   pipe, so no colours or pagers), and then gets notes.
# - On another line, the words that name files are sent, and the rest is the question:
#   `app.log nginx.conf why the 502?`.
# Lines run as `ush notes ...` go into the history as such, showing what was sent. Not on sudo
# or ush lines.
_ush_notes() {
  emulate -L zsh
  setopt extended_glob
  local -a words args
  local comment= command=$BUFFER word first bin=${USH_BIN:-ush} files=0
  words=(${(Z+c+)BUFFER})
  if [[ $words[-1] == '#'* ]]; then
    comment=${${words[-1]#\#}##[[:space:]]#}
    command=${${${BUFFER%%[[:space:]]#}%"$words[-1]"}%%[[:space:]]#}
    words[-1]=()
  fi
  first=${(Q)words[1]}
  [[ $first == '~/'* ]] && first=$HOME/${first#\~/}
  if [[ $first == (sudo|doas|ush|*/ush|//*) ]]; then
    zle -M "ush: no notes on sudo or ush lines"
    return
  fi
  if (( ${#words} )) && whence -- $first >/dev/null; then
    _ush_noting=1 _ush_notes_ask=$comment
    _ush_state=offered _ush_mode=capture
    BUFFER=$command
    zle accept-line
    return
  fi
  for word in $words; do
    word=${(Q)word}
    [[ $word == '~/'* ]] && word=$HOME/${word#\~/}
    [[ -e $word || -L $word ]] && files=1
    args+=($word)
  done
  [[ -n $comment ]] && args+=($comment)
  if (( files )); then
    BUFFER="${(q-)bin} notes -- ${(j: :)${(@q-)args}}"
  else
    BUFFER="${(q-)bin} notes --last${args:+ -- ${(q-)${(j: :)args}}}"
  fi
  zle accept-line
}

_ush_keys_bind() {
  (( ${#_ush_saved} )) && return 0
  local key line
  _ush_saved=("$(bindkey -M emacs -L $'\e')")
  bindkey -M emacs $'\e' _ush_discard
  for key in $'\e[B' $'\eOB' $'\e[A' $'\eOA'; do
    line=$(bindkey -M main -L $key)
    _ush_saved+=("$line")
    _ush_orig[$key]=${${(z)line}[-1]}
    if [[ $key == *B ]]; then
      bindkey -M main $key _ush_explain
    else
      bindkey -M main $key _ush_up
    fi
  done
}

_ush_keys_restore() {
  _ush_explained=0 _ush_kept_line= _ush_kept_text=
  (( ${#_ush_saved} )) || return 0
  local line
  bindkey -M emacs -r $'\e'
  bindkey -M main -r $'\e[B'
  bindkey -M main -r $'\eOB'
  bindkey -M main -r $'\e[A'
  bindkey -M main -r $'\eOA'
  for line in $_ush_saved; do
    [[ -n $line ]] && eval "$line"
  done
  _ush_saved=()
}

_ush_exit() {
  _ush_keys_restore
  _ush_end_capture
  [[ -n $_ush_dir ]] && command rm -rf -- $_ush_dir
}

if (( EUID != 0 )) && _ush_dir=$(command mktemp -d "${${TMPDIR:-/tmp}%/}"/ush.XXXXXXXX 2>/dev/null)
then
  export USH_HANDOFF=$_ush_dir
  precmd_functions=(_ush_precmd ${precmd_functions:#_ush_precmd})  # first: it needs $?
  autoload -Uz add-zsh-hook
  add-zsh-hook preexec _ush_preexec
  add-zsh-hook zshexit _ush_exit
  zle -N _ush_discard
  zle -N _ush_explain
  zle -N _ush_up
  zle -N _ush_notes
  # The notes key, in emacs and viins: Ctrl-N where it still has zsh's own meaning (emacs:
  # down-line-or-history, which the Down arrow also does; viins: it inserts a literal ^N), or
  # another USH_NOTES_KEY where that is free. A key the user bound is never taken.
  if [[ -n ${USH_NOTES_KEY-'^N'} ]]; then
    for _ush_map in emacs viins; do
      _ush_was=${${(z)"$(bindkey -M $_ush_map -- ${USH_NOTES_KEY-'^N'})"}[-1]}  # the widget
      if [[ $_ush_was == undefined-key ]] || [[ ${USH_NOTES_KEY-'^N'} == '^N' &&
            ${_ush_map}:${_ush_was} == (emacs:down-line-or-history|viins:self-insert) ]]; then
        bindkey -M $_ush_map -- ${USH_NOTES_KEY-'^N'} _ush_notes
      fi
    done
    unset _ush_map _ush_was
  fi
fi
