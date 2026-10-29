#!/bin/sh
# Install ush (the Go implementation) from this release archive.
#
#   ./install.sh                          for you: ~/.local/bin/ush, ~/.local/share/ush/
#   sudo ./install.sh --prefix /usr/local root-owned: /usr/local/bin/ush, /usr/local/lib/ush/
#
# The manual page goes to PREFIX/share/man/man1/ush.1 in both cases.
#
# The same places as the Python installer (ush/py/install.sh), so the zsh plugin line in ~/.zshrc
# stays valid when you switch implementations. Use the root-owned install for `sudo ush`.
# --zshrc adds the zsh plugin to ~/.zshrc without asking, --no-zshrc leaves it alone; by default
# the installer asks when run from a terminal (zshrc.sh).
set -eu

prefix="$HOME/.local" zshrc=ask
while [ $# -gt 0 ]; do
    case "$1" in
        --prefix) prefix=${2:?--prefix needs a directory}; shift 2 ;;
        --zshrc) zshrc=yes; shift ;;
        --no-zshrc) zshrc=no; shift ;;
        -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
        *) echo "install.sh: unknown option $1" >&2; exit 2 ;;
    esac
done
if [ "$prefix" = "$HOME/.local" ]; then lib="$prefix/share/ush"; else lib="$prefix/lib/ush"; fi
here=$(cd "$(dirname "$0")" && pwd)

umask 022
mkdir -p "$prefix/bin" "$lib" "$prefix/share/man/man1"
# Copy to a temporary name and rename, so a running ush is never overwritten in place.
cp "$here/ush" "$prefix/bin/.ush.new"
chmod 755 "$prefix/bin/.ush.new"
mv -f "$prefix/bin/.ush.new" "$prefix/bin/ush"
cp "$here/ush.plugin.zsh" "$lib/ush.plugin.zsh"
chmod 644 "$lib/ush.plugin.zsh"
cp "$here/ush.1" "$prefix/share/man/man1/ush.1"
chmod 644 "$prefix/share/man/man1/ush.1"

echo "installed $prefix/bin/ush ($("$prefix/bin/ush" --version))"
case ":$PATH:" in *":$prefix/bin:"*) ;; *) echo "add $prefix/bin to PATH" ;; esac
sh "$here/zshrc.sh" "$lib/ush.plugin.zsh" "$zshrc"
