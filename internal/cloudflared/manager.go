// Package cloudflared locates, downloads and runs the cloudflared binary.
package cloudflared

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ReleaseBase is where the official binaries live.
const ReleaseBase = "https://github.com/cloudflare/cloudflared/releases/latest/download/"

// ErrNotFound means no cloudflared binary is available yet.
var ErrNotFound = errors.New("cloudflared was not found")

// Manager owns linko's private copy of cloudflared.
type Manager struct {
	BinDir  string
	BaseURL string
	HTTP    *http.Client
}

// New returns a manager storing binaries in binDir.
func New(binDir string) *Manager {
	return &Manager{
		BinDir:  binDir,
		BaseURL: ReleaseBase,
		HTTP:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// BinaryName is the platform-specific file name.
func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "cloudflared.exe"
	}
	return "cloudflared"
}

// Path is where linko keeps its own copy.
func (m *Manager) Path() string { return filepath.Join(m.BinDir, BinaryName()) }

// AssetName returns the release asset for the current platform and whether it
// is a gzipped tarball rather than a bare binary.
func AssetName(goos, goarch string) (name string, tarball bool, err error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "amd64":
			return "cloudflared-darwin-amd64.tgz", true, nil
		case "arm64":
			return "cloudflared-darwin-arm64.tgz", true, nil
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "cloudflared-linux-amd64", false, nil
		case "arm64":
			return "cloudflared-linux-arm64", false, nil
		case "arm":
			return "cloudflared-linux-arm", false, nil
		case "386":
			return "cloudflared-linux-386", false, nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "cloudflared-windows-amd64.exe", false, nil
		case "386":
			return "cloudflared-windows-386.exe", false, nil
		}
	}
	return "", false, fmt.Errorf("no cloudflared build for %s/%s — install it manually from https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/", goos, goarch)
}

func isExecutableFile(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return st.Mode()&0o111 != 0
}

// Locate returns a usable cloudflared: linko's own copy first, then $PATH.
func (m *Manager) Locate() (string, error) {
	if p := m.Path(); isExecutableFile(p) {
		return p, nil
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p, nil
	}
	return "", ErrNotFound
}

// Ensure returns a cloudflared path, downloading it if necessary.
func (m *Manager) Ensure(ctx context.Context, progress io.Writer) (string, error) {
	if p, err := m.Locate(); err == nil {
		return p, nil
	}
	if progress != nil {
		fmt.Fprintf(progress, "Downloading cloudflared into %s …\n", m.BinDir)
	}
	return m.Download(ctx)
}

// Download fetches the current release for this platform.
func (m *Manager) Download(ctx context.Context) (string, error) {
	asset, tarball, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(m.BinDir, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", m.BinDir, err)
	}

	url := strings.TrimSuffix(m.BaseURL, "/") + "/" + asset
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := m.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading cloudflared: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading cloudflared: HTTP %d from %s", resp.StatusCode, url)
	}

	var src io.Reader = resp.Body
	if tarball {
		src, err = binaryFromTarball(resp.Body)
		if err != nil {
			return "", err
		}
	}

	tmp, err := os.CreateTemp(m.BinDir, ".cloudflared-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return "", fmt.Errorf("writing cloudflared: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil && runtime.GOOS != "windows" {
		return "", err
	}
	dest := m.Path()
	_ = os.Remove(dest)
	if err := os.Rename(tmpName, dest); err != nil {
		return "", fmt.Errorf("installing cloudflared to %s: %w", dest, err)
	}
	return dest, nil
}

func binaryFromTarball(r io.Reader) (io.Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("reading cloudflared archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("cloudflared binary not found inside the downloaded archive")
		}
		if err != nil {
			return nil, fmt.Errorf("reading cloudflared archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) == "cloudflared" {
			return tr, nil
		}
	}
}

// Version runs `cloudflared --version` and returns the trimmed output.
func (m *Manager) Version(ctx context.Context, path string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, path, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("running %s --version: %w", path, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunOptions configures a cloudflared run.
type RunOptions struct {
	// Token is the tunnel run token. It is passed via the environment so it
	// does not show up in the process list.
	Token string
	// LogLevel is cloudflared's --loglevel (debug, info, warn, error, fatal).
	LogLevel string
	// ExtraArgs are appended after `run`.
	ExtraArgs []string
}

// Command builds the exec.Cmd that runs the tunnel.
func (m *Manager) Command(ctx context.Context, path string, opts RunOptions) *exec.Cmd {
	level := opts.LogLevel
	if level == "" {
		level = "info"
	}
	args := []string{"--no-autoupdate", "--loglevel", level, "tunnel", "run"}
	args = append(args, opts.ExtraArgs...)

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(), "TUNNEL_TOKEN="+opts.Token)
	return cmd
}
