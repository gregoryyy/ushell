#!/bin/sh
# Install ush (the Go implementation) from this release archive.
#
#   ./install.sh                          for you: ~/.local/bin/ush, ~/.local/share/ush/
#   sudo ./install.sh --prefix /usr/local root-owned: /usr/local/bin/ush, /usr/local/lib/ush/
#
# The same places as the Python installer (ushpy/install.sh), so the zsh plugin line in ~/.zshrc
# stays valid when you switch implementations. Use the root-owned install for `sudo ush`.
set -eu

prefix="$HOME/.local"
while [ $# -gt 0 ]; do
    case "$1" in
        --prefix) prefix=${2:?--prefix needs a directory}; shift 2 ;;
        -h|--help) sed -n '2,8p' "$0"; exit 0 ;;
        *) echo "install.sh: unknown option $1" >&2; exit 2 ;;
    esac
done
if [ "$prefix" = "$HOME/.local" ]; then lib="$prefix/share/ush"; else lib="$prefix/lib/ush"; fi
here=$(cd "$(dirname "$0")" && pwd)

umask 022
mkdir -p "$prefix/bin" "$lib"
# Copy to a temporary name and rename, so a running ush is never overwritten in place.
cp "$here/ush" "$prefix/bin/.ush.new"
chmod 755 "$prefix/bin/.ush.new"
mv -f "$prefix/bin/.ush.new" "$prefix/bin/ush"
cp "$here/ush.plugin.zsh" "$lib/ush.plugin.zsh"
chmod 644 "$lib/ush.plugin.zsh"

echo "installed $prefix/bin/ush ($("$prefix/bin/ush" --version))"
case ":$PATH:" in *":$prefix/bin:"*) ;; *) echo "add $prefix/bin to PATH" ;; esac
echo "for unquoted // requests, add to ~/.zshrc:"
echo "  source '$lib/ush.plugin.zsh'"
