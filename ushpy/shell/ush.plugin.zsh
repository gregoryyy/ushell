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
# Settings (set before sourcing):
#   USH_BARE_SLASH=0   do not treat a line starting with `//` as `ush //...`
#   USH_BIN=PATH       ush executable to use (default: `ush` found on PATH)

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
zmodload -F zsh/zselect b:zselect 2>/dev/null

_ush_precmd() {
  local st=$? content
  _ush_keys_restore
  if [[ $_ush_state == running ]]; then
    _ush_end_capture
    print -r -- "$st"$'\n'"$_ush_line" >| $_ush_dir/ran
  fi
  _ush_state=
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
# - Esc removes it, to start from scratch; `//` then hears that it didn't run. Only in the emacs
#   keymap: in vi mode Esc switches to command mode (Ctrl-U clears the line there).
# - Down, past the last line of the proposal, explains it (`ush explain`, with the model); the
#   proposal stays on the line. Elsewhere Down does what it did before.
_ush_discard() {
  _ush_state=
  BUFFER= CURSOR=0
  _ush_keys_restore
  zle -M "ush: proposal removed"
}

_ush_explain() {
  if [[ $_ush_state == offered && $HISTNO == $HISTCMD && $RBUFFER != *$'\n'* &&
        -n ${BUFFER//[[:space:]]/} ]]; then
    zle -I
    print
    command ${USH_BIN:-ush} explain -- "$BUFFER" </dev/null
  else
    local widget=${_ush_orig[$KEYS]:-down-line-or-history}
    [[ $widget == undefined-key ]] || zle $widget
  fi
}

_ush_keys_bind() {
  (( ${#_ush_saved} )) && return 0
  local key line
  _ush_saved=("$(bindkey -M emacs -L $'\e')")
  bindkey -M emacs $'\e' _ush_discard
  for key in $'\e[B' $'\eOB'; do
    line=$(bindkey -M main -L $key)
    _ush_saved+=("$line")
    _ush_orig[$key]=${${(z)line}[-1]}
    bindkey -M main $key _ush_explain
  done
}

_ush_keys_restore() {
  (( ${#_ush_saved} )) || return 0
  local line
  bindkey -M emacs -r $'\e'
  bindkey -M main -r $'\e[B'
  bindkey -M main -r $'\eOB'
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
fi
