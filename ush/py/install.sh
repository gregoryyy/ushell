#!/bin/sh
# Install ush into a private virtual environment with hash-pinned dependencies.
#
#   ./install.sh                          for you: ~/.local/share/ush, ~/.local/bin/ush
#   sudo ./install.sh --prefix /usr/local root-owned: /usr/local/lib/ush, /usr/local/bin/ush
#
# Use the root-owned install if you run `sudo ush`: root then executes only files
# that root owns. Set PYTHON to choose the interpreter (3.10 or newer).
# --zshrc adds the zsh plugin to ~/.zshrc without asking, --no-zshrc leaves it alone;
# by default the installer asks when run from a terminal (../shell/zshrc.sh).
set -eu

prefix="$HOME/.local" zshrc=ask
while [ $# -gt 0 ]; do
    case "$1" in
        --prefix) prefix=${2:?--prefix needs a directory}; shift 2 ;;
        --zshrc) zshrc=yes; shift ;;
        --no-zshrc) zshrc=no; shift ;;
        -h|--help) sed -n '2,11p' "$0"; exit 0 ;;
        *) echo "install.sh: unknown option $1" >&2; exit 2 ;;
    esac
done
case "$prefix" in *"'"*) echo "install.sh: prefix must not contain a quote" >&2; exit 2 ;; esac
if [ "$prefix" = "$HOME/.local" ]; then lib="$prefix/share/ush"; else lib="$prefix/lib/ush"; fi
src=$(cd "$(dirname "$0")" && pwd)

umask 022
mkdir -p "$lib" "$prefix/bin"

# Use $PYTHON, or the newest Python 3.10+ that can create a virtual environment.
candidates=${PYTHON:-"python3.14 python3.13 python3.12 python3.11 python3.10 python3"}
python=
for candidate in $candidates; do
    command -v "$candidate" >/dev/null 2>&1 || continue
    "$candidate" -c 'import sys; sys.exit(sys.version_info < (3, 10))' 2>/dev/null || continue
    if "$candidate" -m venv --clear "$lib/venv" >/dev/null 2>&1; then
        python=$(command -v "$candidate")
        break
    fi
done
[ -n "$python" ] || { echo "install.sh: ush needs Python 3.10 or newer with venv; set PYTHON" >&2; exit 1; }
echo "using $python"
"$lib/venv/bin/pip" install -q --disable-pip-version-check --require-hashes \
    -r "$src/requirements.lock"
# Copy the package instead of building it, so no unpinned build tools are fetched.
site=$("$lib/venv/bin/python" -c 'import sysconfig; print(sysconfig.get_paths()["purelib"])')
rm -rf "$site/ush"
cp -R "$src/src/ush" "$site/ush"
find "$site/ush" -name __pycache__ -prune -exec rm -rf {} +
cp "$src/../shell/ush.plugin.zsh" "$lib/ush.plugin.zsh"

cat > "$prefix/bin/ush" <<LAUNCHER
#!/bin/sh
# -I: ignore PYTHON* variables, the user site directory, and the current directory.
exec '$lib/venv/bin/python' -I -m ush "\$@"
LAUNCHER
chmod 755 "$prefix/bin/ush"

echo "installed $prefix/bin/ush"
case ":$PATH:" in *":$prefix/bin:"*) ;; *) echo "add $prefix/bin to PATH" ;; esac
sh "$src/../shell/zshrc.sh" "$lib/ush.plugin.zsh" "$zshrc"
echo "then choose a model: ush ai configure"
