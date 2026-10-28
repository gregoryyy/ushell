// Command release builds the release archives of the Go ush: one static binary per target, packed
// with the zsh plugin, the installer and a README into ush-VERSION-OS-ARCH.tar.gz, plus SHA256SUMS.
// The output depends only on the source, the version and -epoch: no build IDs, no paths, fixed
// timestamps, owners and order, so the same tag gives the same bytes on any machine (TODO 2.1).
//
//	go run ./tools/release -version 0.1.0 -epoch "$(git log -1 --format=%ct)" -out dist
//	go run ./tools/release -version 0.1.0 -check     builds twice and compares
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Targets are the supported platforms (GAPS A2).
var Targets = []string{"darwin/arm64", "darwin/amd64", "linux/arm64", "linux/amd64"}

var semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

// File is one archive member.
type File struct {
	Name string
	Mode int64
	Data []byte
}

func main() {
	version := flag.String("version", "", "release version, e.g. 0.1.0")
	epoch := flag.Int64("epoch", 0, "timestamp of every archive member (seconds; the commit time)")
	out := flag.String("out", "dist", "output directory")
	check := flag.Bool("check", false, "build twice into temporary directories and compare")
	flag.Parse()
	if err := run(*version, *epoch, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func run(version string, epoch int64, out string, check bool) error {
	if !semver.MatchString(version) {
		return fmt.Errorf("version %q is not X.Y.Z or X.Y.Z-suffix", version)
	}
	if !check {
		sums, err := Build(version, epoch, out)
		if err == nil {
			fmt.Print(sums)
		}
		return err
	}
	var results [2]string
	for i := range results {
		dir, err := os.MkdirTemp("", "ush-release-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		if results[i], err = Build(version, epoch, dir); err != nil {
			return err
		}
	}
	if results[0] != results[1] {
		return fmt.Errorf("two builds differ:\n%s\n%s", results[0], results[1])
	}
	fmt.Print(results[0])
	fmt.Println("reproducible: two builds are identical")
	return nil
}

// Build writes one archive per target and SHA256SUMS into out, and returns SHA256SUMS.
func Build(version string, epoch int64, out string) (string, error) {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	shared, err := sharedFiles(root)
	if err != nil {
		return "", err
	}
	var sums strings.Builder
	for _, target := range Targets {
		goos, goarch, _ := strings.Cut(target, "/")
		bin, err := compile(root, version, goos, goarch)
		if err != nil {
			return "", fmt.Errorf("%s: %w", target, err)
		}
		files := append([]File{{"ush", 0o755, bin}}, shared...)
		data, err := Pack(fmt.Sprintf("ush-%s-%s-%s", version, goos, goarch), files, epoch)
		if err != nil {
			return "", err
		}
		name := fmt.Sprintf("ush-%s-%s-%s.tar.gz", version, goos, goarch)
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return "", err
		}
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(data), name)
	}
	return sums.String(), os.WriteFile(filepath.Join(out, "SHA256SUMS"), []byte(sums.String()), 0o644)
}

// sharedFiles are the members every archive has besides the binary. The zsh plugin and the script
// that adds it to ~/.zshrc are ushpy's: one plugin for both implementations of the command.
func sharedFiles(root string) ([]File, error) {
	var files []File
	for _, f := range []struct {
		name, path string
		mode       int64
	}{
		{"install.sh", "packaging/install.sh", 0o755},
		{"README.txt", "packaging/README.txt", 0o644},
		{"ush.plugin.zsh", "../ushpy/shell/ush.plugin.zsh", 0o644},
		{"zshrc.sh", "../ushpy/shell/zshrc.sh", 0o755},
		{"LICENSE", "../LICENSE", 0o644},
	} {
		data, err := os.ReadFile(filepath.Join(root, f.path))
		if err != nil {
			return nil, err
		}
		files = append(files, File{f.name, f.mode, data})
	}
	return files, nil
}

// compile builds a static binary for one target: no cgo, no paths (-trimpath), no build ID, no
// symbol tables, the version stamped in; Go downloads no toolchain and no module.
func compile(root, version, goos, goarch string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "ush-build-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	bin := filepath.Join(dir, "ush")
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false",
		"-ldflags", "-s -w -buildid= -X main.version="+version, "-o", bin, "./cmd/ush")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch,
		"GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("go build: %v\n%s", err, output)
	}
	return os.ReadFile(bin)
}

// moduleRoot is ushgo/, the directory with go.mod at or above the working directory.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory; run from ushgo/")
		}
		dir = parent
	}
}

// Pack writes files under dir/ into a gzip-compressed tar, deterministically: members sorted by
// name, every timestamp set to epoch, owner root, no user or group names, no gzip name or time.
func Pack(dir string, files []File, epoch int64) ([]byte, error) {
	if strings.ContainsAny(dir, "/\\") || dir == "" {
		return nil, errors.New("archive directory must be one plain name")
	}
	sorted := append([]File(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	mtime := time.Unix(epoch, 0).UTC()
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(gz)
	header := func(h *tar.Header) error {
		h.ModTime, h.Uid, h.Gid, h.Format = mtime, 0, 0, tar.FormatUSTAR
		return tw.WriteHeader(h)
	}
	if err := header(&tar.Header{Typeflag: tar.TypeDir, Name: dir + "/", Mode: 0o755}); err != nil {
		return nil, err
	}
	for _, f := range sorted {
		if f.Name == "" || strings.ContainsAny(f.Name, "/\\") {
			return nil, fmt.Errorf("member name %q must be one plain name", f.Name)
		}
		h := &tar.Header{Typeflag: tar.TypeReg, Name: dir + "/" + f.Name, Mode: f.Mode,
			Size: int64(len(f.Data))}
		if err := header(h); err != nil {
			return nil, err
		}
		if _, err := io.Copy(tw, bytes.NewReader(f.Data)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
