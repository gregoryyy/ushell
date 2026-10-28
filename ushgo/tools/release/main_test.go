package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func members(t *testing.T, data []byte) []*tar.Header {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if gz.Name != "" || !gz.ModTime.IsZero() {
		t.Errorf("gzip header carries name %q or time %v", gz.Name, gz.ModTime)
	}
	tr := tar.NewReader(gz)
	var headers []*tar.Header
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return headers
		}
		if err != nil {
			t.Fatal(err)
		}
		headers = append(headers, h)
	}
}

func TestPackIsDeterministicAndOrdered(t *testing.T) {
	files := []File{{"ush", 0o755, []byte("binary")}, {"README.txt", 0o644, []byte("read me")}}
	reversed := []File{files[1], files[0]}
	a, err := Pack("ush-1.0.0-linux-amd64", files, 1700000000)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Pack("ush-1.0.0-linux-amd64", reversed, 1700000000)
	if !bytes.Equal(a, b) {
		t.Fatal("input order changes the archive")
	}
	if c, _ := Pack("ush-1.0.0-linux-amd64", files, 1700000001); bytes.Equal(a, c) {
		t.Fatal("epoch has no effect")
	}
	var names []string
	for _, h := range members(t, a) {
		names = append(names, h.Name)
		if h.Uid != 0 || h.Gid != 0 || h.Uname != "" || h.Gname != "" || h.ModTime.Unix() != 1700000000 {
			t.Errorf("%s: owner %d/%d %q/%q, time %v", h.Name, h.Uid, h.Gid, h.Uname, h.Gname, h.ModTime)
		}
	}
	want := "ush-1.0.0-linux-amd64/ ush-1.0.0-linux-amd64/README.txt ush-1.0.0-linux-amd64/ush"
	if strings.Join(names, " ") != want {
		t.Errorf("members %v", names)
	}
}

func TestPackRefusesPathsInNames(t *testing.T) {
	for _, bad := range []string{"../evil", "a/b", ""} {
		if _, err := Pack("d", []File{{bad, 0o644, nil}}, 0); err == nil {
			t.Errorf("member %q accepted", bad)
		}
	}
	if _, err := Pack("../d", nil, 0); err == nil {
		t.Error("directory with a path accepted")
	}
}

func TestVersionsAreChecked(t *testing.T) {
	for _, v := range []string{"", "v1.0.0", "1.0", "1.0.0;rm", "1.0.0 "} {
		if err := run(v, 0, t.TempDir(), false); err == nil {
			t.Errorf("version %q accepted", v)
		}
	}
}

// The full build for every target: archives with the expected members, a static binary that runs
// on this machine and reports the version, checksums that match, and the same bytes twice.
func TestBuildIsReproducibleAndRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("builds four targets")
	}
	first, second := t.TempDir(), t.TempDir()
	sums, err := Build("1.2.3", 1700000000, first)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Build("1.2.3", 1700000000, second)
	if err != nil {
		t.Fatal(err)
	}
	if sums != again {
		t.Fatalf("two builds differ:\n%s\n%s", sums, again)
	}
	if strings.Count(sums, "\n") != len(Targets) {
		t.Fatalf("SHA256SUMS: %q", sums)
	}
	host := "ush-1.2.3-" + runtime.GOOS + "-" + runtime.GOARCH
	out := t.TempDir()
	tarball := filepath.Join(first, host+".tar.gz")
	if output, err := exec.Command("tar", "-xzf", tarball, "-C", out).CombinedOutput(); err != nil {
		t.Fatalf("tar: %v %s", err, output)
	}
	for _, name := range []string{"ush", "install.sh", "README.txt", "ush.plugin.zsh"} {
		if _, err := os.Stat(filepath.Join(out, host, name)); err != nil {
			t.Error(err)
		}
	}
	version, err := exec.Command(filepath.Join(out, host, "ush"), "--version").Output()
	if err != nil || string(version) != "ush 1.2.3 (go)\n" {
		t.Fatalf("--version: %q %v", version, err)
	}
	// The installer puts the binary and the plugin where ushpy's installer does.
	prefix := t.TempDir()
	install := exec.Command("sh", filepath.Join(out, host, "install.sh"), "--prefix", prefix)
	if output, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install.sh: %v %s", err, output)
	}
	for _, path := range []string{"bin/ush", "lib/ush/ush.plugin.zsh"} {
		info, err := os.Stat(filepath.Join(prefix, path))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o022 != 0 {
			t.Errorf("%s is writable by others: %v", path, info.Mode())
		}
	}
}
