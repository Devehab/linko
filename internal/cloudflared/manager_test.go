package cloudflared

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssetNameKnownPlatforms(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		tarball      bool
	}{
		{"darwin", "arm64", "cloudflared-darwin-arm64.tgz", true},
		{"darwin", "amd64", "cloudflared-darwin-amd64.tgz", true},
		{"linux", "amd64", "cloudflared-linux-amd64", false},
		{"linux", "arm64", "cloudflared-linux-arm64", false},
		{"windows", "amd64", "cloudflared-windows-amd64.exe", false},
	}
	for _, c := range cases {
		got, tarball, err := AssetName(c.goos, c.goarch)
		if err != nil {
			t.Errorf("AssetName(%s,%s) = %v", c.goos, c.goarch, err)
			continue
		}
		if got != c.want || tarball != c.tarball {
			t.Errorf("AssetName(%s,%s) = (%q,%v), want (%q,%v)", c.goos, c.goarch, got, tarball, c.want, c.tarball)
		}
	}
}

func TestAssetNameUnsupportedPlatform(t *testing.T) {
	if _, _, err := AssetName("plan9", "mips"); err == nil {
		t.Fatal("expected an error for an unsupported platform")
	}
}

func TestCurrentPlatformIsSupported(t *testing.T) {
	if _, _, err := AssetName(runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("the platform running the tests is unsupported: %v", err)
	}
}

func TestLocateFindsLocalCopyFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bits differ on windows")
	}
	dir := t.TempDir()
	m := New(dir)

	fake := filepath.Join(dir, BinaryName())
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := m.Locate()
	if err != nil {
		t.Fatalf("Locate() = %v", err)
	}
	if got != fake {
		t.Fatalf("Locate() = %q, want the local copy %q", got, fake)
	}
}

func TestLocateIgnoresNonExecutableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bits differ on windows")
	}
	dir := t.TempDir()
	m := New(dir)
	if err := os.WriteFile(m.Path(), []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := m.Locate()
	if err == nil && got == m.Path() {
		t.Fatal("Locate() returned a file without the executable bit")
	}
}

// TestDownloadInstallsBinary exercises the real code path for the platform the
// tests run on: a bare binary on Linux/Windows, a .tgz on macOS.
func TestDownloadInstallsBinary(t *testing.T) {
	payload := []byte("fake-cloudflared-binary")

	_, tarball, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("unsupported platform: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "cloudflared-") {
			t.Errorf("unexpected asset path %q", r.URL.Path)
		}
		if tarball {
			w.Write(makeTarball(t, payload))
			return
		}
		w.Write(payload)
	}))
	defer srv.Close()

	dir := t.TempDir()
	m := New(dir)
	m.BaseURL = srv.URL

	path, derr := m.Download(context.Background())
	if derr != nil {
		t.Fatalf("Download() = %v", derr)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("downloaded content = %q", got)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(path)
		if st.Mode()&0o111 == 0 {
			t.Fatal("downloaded binary is not executable")
		}
	}
}

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := New(t.TempDir())
	m.BaseURL = srv.URL
	if _, err := m.Download(context.Background()); err == nil {
		t.Fatal("expected an error on HTTP 404")
	}
}

func TestBinaryFromTarball(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	payload := []byte("inner-binary")
	// A decoy entry before the real one.
	writeTarFile(t, tw, "README.md", []byte("hello"))
	writeTarFile(t, tw, "cloudflared", payload)

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := binaryFromTarball(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("binaryFromTarball() = %v", err)
	}
	got := new(bytes.Buffer)
	if _, err := got.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), payload) {
		t.Fatalf("extracted %q, want %q", got.Bytes(), payload)
	}
}

func TestBinaryFromTarballMissingEntry(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarFile(t, tw, "something-else", []byte("x"))
	tw.Close()
	gz.Close()

	if _, err := binaryFromTarball(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected an error when the archive has no cloudflared entry")
	}
}

// makeTarball builds a gzipped tar containing a single cloudflared entry.
func makeTarball(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarFile(t, tw, "cloudflared", payload)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name string, content []byte) {
	t.Helper()
	hdr := &tar.Header{
		Name:     name,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
}

func TestCommandPassesTokenViaEnvironment(t *testing.T) {
	m := New(t.TempDir())
	cmd := m.Command(context.Background(), "/usr/bin/true", RunOptions{
		Token:    "super-secret",
		LogLevel: "warn",
	})

	for _, a := range cmd.Args {
		if strings.Contains(a, "super-secret") {
			t.Fatal("the tunnel token leaked into the command line (visible in `ps`)")
		}
	}
	found := false
	for _, e := range cmd.Env {
		if e == "TUNNEL_TOKEN=super-secret" {
			found = true
		}
	}
	if !found {
		t.Fatal("TUNNEL_TOKEN was not set in the environment")
	}

	joined := strings.Join(cmd.Args, " ")
	for _, want := range []string{"--no-autoupdate", "--loglevel warn", "tunnel run"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
}

func TestCommandDefaultsLogLevel(t *testing.T) {
	m := New(t.TempDir())
	cmd := m.Command(context.Background(), "/usr/bin/true", RunOptions{Token: "t"})
	if !strings.Contains(strings.Join(cmd.Args, " "), "--loglevel info") {
		t.Fatalf("args = %v", cmd.Args)
	}
}
