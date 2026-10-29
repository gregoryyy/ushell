ush, the you shell: ask in natural language at the zsh prompt, get the shell commands that answer it.
This archive holds the Go implementation of the command `ush`, built for one platform.

  ush              the command, a static binary
  install.sh       installs it: ./install.sh, or sudo ./install.sh --prefix /usr/local
  ush.plugin.zsh   the zsh plugin: proposals on your command line, unquoted `// ...`
  zshrc.sh         used by install.sh to add the plugin to ~/.zshrc (it asks first)
  LICENSE          MIT
  README.txt       this file

Check the archive against SHA256SUMS from the same release before installing:
  shasum -a 256 -c SHA256SUMS --ignore-missing

macOS: a file downloaded with a browser carries a quarantine attribute, and the binary is not
signed, so macOS refuses to run it. Download with curl, or remove the attribute after checking the
checksum: xattr -d com.apple.quarantine ush

Then, in a new terminal: ush ai configure, and // why is my disk full?
ush proposes a command on your command line: Enter runs it, Down explains it, Esc removes it,
and // continues with what you ran. `ush --run //...` lets ush run read-only commands itself.
Documentation: https://github.com/gregoryyy/ushell
