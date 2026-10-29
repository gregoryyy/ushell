#!/bin/sh
# Print the Homebrew formula for a published tag (ushgo TODO 2.7). Homebrew builds ush from the
# tag's source tarball with Go, so the formula needs that tarball's checksum: run this after the
# tag is on GitHub, and commit the output as Formula/ush.rb in the tap repository
# gregoryyy/homebrew-ush (created by the user). Users then install with:
#   brew install gregoryyy/ush/ush
#
#   scripts/formula.sh 0.2.0-rc1 > ush.rb
set -eu
version=${1:?usage: scripts/formula.sh VERSION}
case "$version" in *[!0-9A-Za-z.-]*) echo "formula.sh: bad version $version" >&2; exit 2 ;; esac
url="https://github.com/gregoryyy/ushell/archive/refs/tags/v$version.tar.gz"
sha=$(curl -fsSL "$url" | shasum -a 256 | cut -d' ' -f1)
cat <<FORMULA
class Ush < Formula
  desc "The you shell: ask at the zsh prompt in plain language, get the commands"
  homepage "https://github.com/gregoryyy/ushell"
  url "$url"
  sha256 "$sha"
  version "$version"
  license "MIT"

  depends_on "go" => :build

  def install
    cd "ush/go" do
      ENV["CGO_ENABLED"] = "0"
      system "go", "build", "-trimpath", "-buildvcs=false",
             "-ldflags", "-s -w -X main.version=#{version}", "-o", bin/"ush", "./cmd/ush"
    end
    (share/"ush").install "ush/shell/ush.plugin.zsh"
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
  end
end
FORMULA
