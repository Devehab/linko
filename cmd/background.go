package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/ui"
)

// envDetached marks the re-executed child so it does not try to detach again.
const envDetached = "LINKO_DETACHED"

// Background is a tunnel started with -d and still running.
type Background struct {
	Name     string
	PID      int
	Hostname string
	Service  string
	Started  time.Time
}

// startDetached re-executes linko without -d, with its output going to a log
// file, and returns as soon as the child reports a working URL.
func startDetached(rawTarget, label, hostname string, opts *startOptions) error {
	if already, _ := readPID(label); already > 0 && processAlive(already) {
		ui.Warn("%s is already running in the background (pid %d)", hostname, already)
		ui.Info("Stop it with: linko stop %s", label)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding the linko binary: %w", err)
	}

	for _, dir := range []string{config.RunDir(), config.LogDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	logPath := config.LogFile(label)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating %s: %w", logPath, err)
	}
	defer logFile.Close()

	// Pin the name so a restart cannot drift onto a different hostname.
	args := []string{"start", rawTarget, "--name", label, "--yes", "--no-color"}
	if opts.temp {
		args = append(args, "--temp")
	}
	if opts.replace {
		args = append(args, "--replace")
	}
	if opts.logLevel != "" {
		args = append(args, "--loglevel", opts.logLevel)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), envDetached+"=1")
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting the background tunnel: %w", err)
	}

	if err := writePID(label, cmd.Process.Pid); err != nil {
		ui.Warn("could not record the pid: %v", err)
	}

	// Do not leave a zombie behind if the child dies immediately.
	go func() { _ = cmd.Wait() }()

	ui.Info("Starting in the background …")
	if !waitForLog(logPath, "Tunnel connected", 90*time.Second, cmd.Process.Pid) {
		ui.Fail("The tunnel did not come up within 90s.")
		ui.Info("Log: %s", logPath)
		_ = removePID(label)
		_ = cmd.Process.Kill()
		return fmt.Errorf("background tunnel failed to start")
	}

	url := "https://" + hostname
	ui.Blank()
	ui.Success("Running in the background (pid %d)", cmd.Process.Pid)
	ui.Blank()
	ui.Line("  %s  %s", ui.Dim("Public URL"), ui.Link(url, ui.Bold(ui.Cyan(url))))
	ui.Line("  %s        %s", ui.Dim("Logs"), logPath)
	ui.Blank()
	ui.Line("  %s   %s", ui.Cyan("linko stop "+label), ui.Dim("stop it"))
	ui.Line("  %s              %s", ui.Cyan("linko ps"), ui.Dim("see what is running"))
	ui.Blank()

	if opts.open {
		openBrowser(url) //nolint:errcheck // the URL is already confirmed live
	}
	return nil
}

// waitForLog tails a log file until it contains marker, the process dies, or
// the deadline passes.
func waitForLog(path, marker string, timeout time.Duration, pid int) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f, err := os.Open(path); err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if strings.Contains(sc.Text(), marker) {
					f.Close()
					return true
				}
			}
			f.Close()
		}
		if pid > 0 && !processAlive(pid) {
			return false
		}
		time.Sleep(time.Second)
	}
	return false
}

func writePID(name string, pid int) error {
	if err := os.MkdirAll(config.RunDir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(config.PIDFile(name), []byte(strconv.Itoa(pid)+"\n"), 0o600)
}

func readPID(name string) (int, error) {
	b, err := os.ReadFile(config.PIDFile(name))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func removePID(name string) error {
	err := os.Remove(config.PIDFile(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// processAlive reports whether a pid is still running. Signal 0 performs the
// permission and existence checks without delivering anything.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// runningTunnels lists background tunnels, cleaning up stale pid files.
func runningTunnels(cfg *config.Config) []Background {
	entries, err := os.ReadDir(config.RunDir())
	if err != nil {
		return nil
	}

	out := []Background{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".pid" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".pid")
		pid, err := readPID(name)
		if err != nil {
			continue
		}
		if !processAlive(pid) {
			_ = removePID(name) // the process is gone; forget it
			continue
		}

		bg := Background{Name: name, PID: pid}
		if info, ierr := e.Info(); ierr == nil {
			bg.Started = info.ModTime()
		}
		if cfg != nil {
			if r := cfg.FindRoute(name); r != nil {
				bg.Hostname = r.Hostname
				bg.Service = r.Service
			}
		}
		out = append(out, bg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func stopTunnel(name string) error {
	pid, err := readPID(name)
	if err != nil {
		return fmt.Errorf("%q is not running in the background", name)
	}
	if !processAlive(pid) {
		_ = removePID(name)
		return fmt.Errorf("%q is not running (stale pid %d cleaned up)", name, pid)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := p.Signal(syscall.SIGINT); err != nil {
		return fmt.Errorf("signalling pid %d: %w", pid, err)
	}

	// Give it a moment to clean up its own routes, then insist.
	for i := 0; i < 20; i++ {
		if !processAlive(pid) {
			return removePID(name)
		}
		time.Sleep(500 * time.Millisecond)
	}
	_ = p.Kill()
	return removePID(name)
}
