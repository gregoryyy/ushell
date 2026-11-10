#!/bin/sh
# Add the ush zsh plugin to ~/.zshrc (or $ZDOTDIR/.zshrc); used by both installers.
#
#   zshrc.sh PLUGIN [ask|yes|no]
#
# The plugin puts ush's proposals on the command line and lets requests go unquoted. ask (the
# default) asks only when run from a terminal, with yes as the default answer; without a terminal,
# and with no, it only prints the line to add. Never as root, and never twice: a .zshrc that
# already sources an ush.plugin.zsh is left alone.
set -eu

plugin=${1:?zshrc.sh: plugin path missing}
mode=${2:-ask}
rc=${ZDOTDIR:-$HOME}/.zshrc
line="source '$plugin'"

hint() {
    echo "for proposals on your command line and unquoted // requests, add to $rc:"
    echo "  $line"
}

case "$plugin" in *"'"*) echo "zshrc.sh: the plugin path must not contain a quote" >&2; exit 2 ;; esac
if grep -q 'ush\.plugin\.zsh' "$rc" 2>/dev/null; then
    echo "the zsh plugin is already in $rc"
    exit 0
fi
if [ "$(id -u)" = 0 ] || [ "$mode" = no ]; then
    hint
    exit 0
fi
if [ "$mode" = ask ]; then
    if ! [ -t 0 ] || ! [ -t 1 ]; then
        hint
        exit 0
    fi
    printf 'add the zsh plugin to %s? [Y/n] ' "$rc"
    read -r answer || answer=n
    case "$answer" in
        ""|[yY]|[yY][eE][sS]) ;;
        *) hint; exit 0 ;;
    esac
fi
printf '\n# ush: // requests, and proposals on the command line\n%s\n' "$line" >> "$rc"
echo "added the zsh plugin to $rc; it works in new terminals (or now: $line)"
