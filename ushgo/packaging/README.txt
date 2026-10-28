ush, the you shell: ask in natural language at the zsh prompt, get answers from real commands.
This archive holds the Go implementation of the command `ush`, built for one platform.

  ush              the command, a static binary
  install.sh       installs it: ./install.sh, or sudo ./install.sh --prefix /usr/local
  ush.plugin.zsh   the zsh plugin for unquoted `ush //...` and `// ...`
  README.txt       this file

Check the archive against SHA256SUMS from the same release before installing:
  shasum -a 256 -c SHA256SUMS --ignore-missing

macOS: a file downloaded with a browser carries a quarantine attribute, and the binary is not
signed, so macOS refuses to run it. Download with curl, or remove the attribute after checking the
checksum: xattr -d com.apple.quarantine ush

Then: ush ai configure, and ush //why is my disk full?
Documentation: https://github.com/gregoryyy/ushell
