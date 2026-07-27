package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/target"
	"github.com/Devehab/linko/internal/ui"
)

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Keep a tunnel running across reboots",
		Long: `service registers a tunnel with your operating system so it starts
automatically at login and restarts if it ever stops.

  linko service install 3000 --name crm    always publish crm
  linko service list
  linko service uninstall crm

macOS uses a launchd agent in ~/Library/LaunchAgents.
Linux uses a systemd user unit in ~/.config/systemd/user.`,
	}
	cmd.AddCommand(newServiceInstallCmd(), newServiceUninstallCmd(), newServiceListCmd())
	return cmd
}

func newServiceInstallCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "install <port|host:port|url>",
		Short: "Start this tunnel automatically at login",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}

			service, _, err := target.Parse(args[0])
			if err != nil {
				return err
			}

			label := strings.ToLower(strings.TrimSpace(name))
			if label == "" {
				if r := cfg.FindRouteByService(service); r != nil {
					label = r.Name
				} else {
					return fmt.Errorf("give the tunnel a name: linko service install %s --name myapp", args[0])
				}
			}

			return installService(label, args[0])
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "subdomain to publish (default: this port's current one)")
	return cmd
}

func newServiceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "uninstall <name>",
		Aliases: []string{"remove", "rm"},
		Short:   "Stop starting this tunnel at login",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return uninstallService(args[0])
		},
	}
}

func newServiceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show tunnels registered to start at login",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := listServices()
			if err != nil {
				return err
			}
			if len(names) == 0 {
				ui.Info("No tunnels are set to start at login.")
				ui.Info("Add one with: linko service install 3000 --name myapp")
				return nil
			}
			ui.Line("Starting at login:")
			for _, n := range names {
				ui.Line("  %s %s", ui.Green("·"), n)
			}
			ui.Blank()
			ui.Info("unit files: %s", serviceDir())
			return nil
		},
	}
}

// ---------------------------------------------------------------- platform

func serviceDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents")
	}
	return filepath.Join(home, ".config", "systemd", "user")
}

func serviceFile(name string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(serviceDir(), "com.linko."+name+".plist")
	}
	return filepath.Join(serviceDir(), "linko-"+name+".service")
}

func installService(name, rawTarget string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("linko service is not supported on Windows yet — use Task Scheduler with: linko %s --name %s", rawTarget, name)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding the linko binary: %w", err)
	}
	if err := os.MkdirAll(serviceDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(config.LogDir(), 0o700); err != nil {
		return err
	}

	path := serviceFile(name)
	var content string

	if runtime.GOOS == "darwin" {
		content = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.linko.%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>start</string>
    <string>%s</string>
    <string>--name</string>
    <string>%s</string>
    <string>--yes</string>
    <string>--no-color</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, name, exe, rawTarget, name, config.LogFile(name), config.LogFile(name))
	} else {
		content = fmt.Sprintf(`[Unit]
Description=linko tunnel for %s
After=network-online.target

[Service]
ExecStart=%s start %s --name %s --yes --no-color
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, name, exe, rawTarget, name)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	ui.Success("Wrote %s", path)

	if runtime.GOOS == "darwin" {
		_ = exec.Command("launchctl", "unload", path).Run() // ignore: may not be loaded
		if out, lerr := exec.Command("launchctl", "load", "-w", path).CombinedOutput(); lerr != nil {
			ui.Warn("launchctl load failed: %v", lerr)
			if s := strings.TrimSpace(string(out)); s != "" {
				ui.Info("%s", s)
			}
			ui.Info("Load it yourself with: launchctl load -w %s", path)
			return nil
		}
	} else {
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		unit := filepath.Base(path)
		if out, serr := exec.Command("systemctl", "--user", "enable", "--now", unit).CombinedOutput(); serr != nil {
			ui.Warn("systemctl enable failed: %v", serr)
			if s := strings.TrimSpace(string(out)); s != "" {
				ui.Info("%s", s)
			}
			ui.Info("Enable it yourself with: systemctl --user enable --now %s", unit)
			return nil
		}
		ui.Info("Survive logout with: sudo loginctl enable-linger $USER")
	}

	ui.Success("%s will start automatically at login", name)
	ui.Info("logs: %s", config.LogFile(name))
	ui.Info("remove it with: linko service uninstall %s", name)
	return nil
}

func uninstallService(name string) error {
	path := serviceFile(name)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%q is not registered to start at login", name)
	}

	if runtime.GOOS == "darwin" {
		_ = exec.Command("launchctl", "unload", "-w", path).Run()
	} else {
		unit := filepath.Base(path)
		_ = exec.Command("systemctl", "--user", "disable", "--now", unit).Run()
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	ui.Success("%s will no longer start at login", name)
	return nil
}

func listServices() ([]string, error) {
	entries, err := os.ReadDir(serviceDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	names := []string{}
	for _, e := range entries {
		n := e.Name()
		switch {
		case strings.HasPrefix(n, "com.linko.") && strings.HasSuffix(n, ".plist"):
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(n, "com.linko."), ".plist"))
		case strings.HasPrefix(n, "linko-") && strings.HasSuffix(n, ".service"):
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(n, "linko-"), ".service"))
		}
	}
	return names, nil
}
