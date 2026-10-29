#!/bin/sh
# Print the Homebrew formula for a published tag (ushgo TODO 2.7). Homebrew builds ush from the
# tag's source tarball with Go, so the formula needs that tarball's checksum: run this after the
# tag is on GitHub, and commit the output as Formula/ush.rb in the tap repository
# gregoryyy/homebrew-ush (created by the user). Users then install with:
#   brew install gregoryyy/ush/ush
#
#   scripts/formula.sh 0.2.2 > ush.rb
#
# The paths are those of the ush/go layout (#30). Tags before it, v0.2.0 among them, have ushgo/
# and ushpy/shell/: their formula comes from formula.sh at that tag.
set -eu
version=${1:?usage: scripts/formula.sh VERSION}
case "$version" in *[!0-9A-Za-z.-]*) echo "formula.sh: bad version $version" >&2; exit 2 ;; esac
url="https://github.com/gregoryyy/ushell/archive/refs/tags/v$version.tar.gz"
sha=$(curl -fsSL "$url" | shasum -a 256 | cut -d' ' -f1)
cat <<FORMULA
class Ush < Formula
  desc "Shell assistant for zsh: ask in plain language, get the commands"
  homepage "https://github.com/gregoryyy/ushell"
  url "$url"
  version "$version"
  sha256 "$sha"
  license "MIT"

  depends_on "go" => :build

  def install
    cd "ush/go" do
      ENV["CGO_ENABLED"] = "0"
      system "go", "build", "-trimpath", "-buildvcs=false",
             "-ldflags", "-s -w -X main.version=#{version}", "-o", bin/"ush", "./cmd/ush"
    end
    (share/"ush").install "ush/shell/ush.plugin.zsh"
    man1.install "ush/man/ush.1"
  end

  def caveats
    <<~EOS
      For proposals on your command line and unquoted // requests, add to ~/.zshrc:
        source #{opt_share}/ush/ush.plugin.zsh
      Then open a new terminal and run: ush ai configure
    EOS
  end

  test do
    assert_match "#{version} (go)", shell_output("#{bin}/ush --version")
    assert_match "usage:", shell_output("#{bin}/ush help")
    assert_path_exists man1/"ush.1"
  end
end
FORMULA
