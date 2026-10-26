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
    print -r -- "$lead$sudo_part${(q-)bin}"
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
    'help:show usage'
  )
  if (( CURRENT == 2 )); then
    _describe -t commands 'ush command' subcommands
    _arguments '(-n --dry-run)'{-n,--dry-run}'[show commands without running them]' \
      '(-c --confirm)'{-c,--confirm}'[confirm every command]' \
      '(-1 --once)'{-1,--once}'[answer once, no follow-up questions]' \
      '(-v --verbose)'{-v,--verbose}'[show complete command output]'
  elif [[ $words[2] == ai ]]; then
    _values 'ai command' configure status clear-key
  fi
}
(( ${+functions[compdef]} )) && compdef _ush ush
