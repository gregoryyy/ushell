# The Linux stand-in for GitHub's ubuntu-latest runner (TODO 6.5): Ubuntu 26.04 with what the
# workflow's Ubuntu jobs install or set up (zsh, mandoc, Python 3.10 and 3.13, Go 1.27), and what
# the local CI needs to run them (git, uv, gh). Built and run by tools/ci/linux.sh.
#
# Pinned: the base image by digest (the multi-platform index, so arm64 and amd64 get the same
# release), the downloads by version and SHA-256. Versions at least two weeks old, as for the
# workflow's tools, except Go 1.27.4, the release toolchain (TODO 5.12).
FROM ubuntu:26.04@sha256:6a06111c3b0a8600feadb270aec771dc239bf407b25625f93f44ab00e11f4d3c

ARG TARGETARCH
ARG GO_VERSION=1.27.4
ARG GO_SHA256_ARM64=3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92
ARG GO_SHA256_AMD64=7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840
ARG UV_VERSION=0.13.5
ARG UV_SHA256_ARM64=4bb69cf95ed13f23713bb7bc09f241896ea0c75b7c557e615a20604b82f9c403
ARG UV_SHA256_AMD64=f68d291c75783122dad58ab77def0e13f3233f358731db5d6d7e8715603b928f
ARG GH_VERSION=2.103.3
ARG GH_SHA256_ARM64=ee32df2323c8847f5fac6cb6b4b69ca75828e9329a8f22f677021f7b3306ce63
ARG GH_SHA256_AMD64=4c806978ffce1cec70853302a56d70ac6da4f19bae7adca4e1a8ab00633dd91d

ENV DEBIAN_FRONTEND=noninteractive LANG=C.UTF-8 LC_ALL=C.UTF-8
# python3 in /usr/bin as on GitHub's runner image: the plugin tests start zsh with
# PATH=…:/usr/bin:/bin, and their fake ush runs python3 from there.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git zsh mandoc locales python3 \
 && rm -rf /var/lib/apt/lists/*

RUN set -eu; \
    case "$TARGETARCH" in \
      arm64) go=$GO_SHA256_ARM64; uv=$UV_SHA256_ARM64; uvarch=aarch64; gh=$GH_SHA256_ARM64 ;; \
      amd64) go=$GO_SHA256_AMD64; uv=$UV_SHA256_AMD64; uvarch=x86_64;  gh=$GH_SHA256_AMD64 ;; \
      *) echo "unsupported architecture $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    cd /tmp; \
    curl -fsSLo go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz"; \
    echo "$go  go.tgz" | sha256sum -c -; \
    tar -C /usr/local -xzf go.tgz; \
    curl -fsSLo uv.tgz "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/uv-${uvarch}-unknown-linux-gnu.tar.gz"; \
    echo "$uv  uv.tgz" | sha256sum -c -; \
    tar -xzf uv.tgz; install -m 0755 uv-*/uv uv-*/uvx /usr/local/bin/; \
    curl -fsSLo gh.tgz "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${TARGETARCH}.tar.gz"; \
    echo "$gh  gh.tgz" | sha256sum -c -; \
    tar -xzf gh.tgz; install -m 0755 gh_*/bin/gh /usr/local/bin/; \
    rm -rf /tmp/*

# A user like GitHub's `runner`, not root: as root, ush ignores USH_* variables and the tests that
# expect a normal user would test the root paths instead. No sudo: CI never uses the real one.
RUN useradd --create-home --shell /bin/bash runner
USER runner
WORKDIR /home/runner
ENV PATH=/usr/local/go/bin:/home/runner/.local/bin:$PATH \
    UV_PYTHON_INSTALL_DIR=/home/runner/.local/share/uv/python
# The Pythons of the matrix, found later by `uv python find` (actions/setup-python's stand-in).
RUN uv python install 3.10 3.13 && uv python find 3.10 && uv python find 3.13 && go version
