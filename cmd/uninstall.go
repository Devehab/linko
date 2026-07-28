package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/ui"
)

func newUninstallCmd() *cobra.Command {
	var yes bool
	var keepCloud bool
	var keepBinary bool

	cmd := &cobra.Command{
		Use:     "uninstall",
		Aliases: []string{"remove-self", "purge"},
		Short:   "Remove linko from this machine, and its traces from Cloudflare",
		Long: `uninstall undoes everything linko ever did, in order:

  1. stop tunnels running in the background
  2. remove the services that start them at login
  3. delete your published URLs and their DNS records
  4. delete the tunnel from your Cloudflare account
  5. delete ~/.linko  (config, logs, the cloudflared copy)
  6. delete the linko binary itself

Your Cloudflare account, your domain and your API token are left alone —
revoke the token yourself if you no longer want it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runUninstall(ctx, yes, keepCloud, keepBinary)
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	f.BoolVar(&keepCloud, "keep-cloud", false, "leave the URLs, DNS records and tunnel on Cloudflare")
	f.BoolVar(&keepBinary, "keep-binary", false, "leave the linko binary in place")
	return cmd
}

func runUninstall(ctx context.Context, yes, keepCloud, keepBinary bool) error {
	p := ui.NewPrompter()
	cfg, _ := config.Load()

	ui.Header("Uninstall linko")
	ui.Line("This will remove:")
	ui.Line("  %s background tunnels and the services that start them", ui.Dim("·"))
	if !keepCloud && cfg != nil {
		ui.Line("  %s %s and their DNS records", ui.Dim("·"), plural(len(cfg.Routes), "published URL"))
		if cfg.TunnelName != "" {
			ui.Line("  %s the Cloudflare tunnel %q", ui.Dim("·"), cfg.TunnelName)
		}
	}
	ui.Line("  %s %s", ui.Dim("·"), config.Dir())
	if !keepBinary {
		if exe, err := os.Executable(); err == nil {
			ui.Line("  %s %s", ui.Dim("·"), exe)
		}
	}
	ui.Blank()
	ui.Line("It will %s touch your Cloudflare account, your domain, or your API token.",
		ui.Bold("not"))
	ui.Blank()

	if !yes && !p.Confirm("Go ahead?", false) {
		ui.Info("Cancelled — nothing was removed.")
		return nil
	}

	// ── 1. background tunnels ────────────────────────────────────────────
	ui.Blank()
	running := runningTunnels(cfg)
	for _, bg := range running {
		if err := stopTunnel(bg.Name); err != nil {
			ui.Warn("could not stop %s: %v", bg.Name, err)
		} else {
			ui.Success("Stopped %s", bg.Name)
		}
	}
	if len(running) == 0 {
		ui.Info("Nothing was running in the background")
	}

	// ── 2. login services ────────────────────────────────────────────────
	if names, err := listServices(); err == nil {
		for _, n := range names {
			if uerr := uninstallService(n); uerr != nil {
				ui.Warn("could not remove the %s service: %v", n, uerr)
			}
		}
		if len(names) == 0 {
			ui.Info("No login services were installed")
		}
	}

	// ── 3 & 4. Cloudflare ────────────────────────────────────────────────
	if !keepCloud && cfg != nil && cfg.Validate() == nil {
		_, client, cerr := loadClient()
		if cerr != nil {
			ui.Warn("skipping the Cloudflare cleanup: %v", cerr)
		} else {
			for _, r := range cfg.SortedRoutes() {
				if rerr := removeRoute(ctx, client, cfg, r); rerr != nil {
					ui.Warn("could not remove %s: %v", r.Hostname, rerr)
				} else {
					ui.Success("Removed %s", r.Hostname)
				}
			}
			if cfg.TunnelID != "" {
				if derr := client.DeleteTunnel(ctx, cfg.TunnelID); derr != nil {
					ui.Warn("could not delete the tunnel: %v", derr)
					ui.Info("Delete it yourself in the Zero Trust dashboard if you want it gone.")
				} else {
					ui.Success("Deleted the tunnel %s", cfg.TunnelName)
				}
			}
		}
	} else if keepCloud {
		ui.Info("Leaving your URLs and tunnel on Cloudflare (--keep-cloud)")
	}

	// ── 5. local state ───────────────────────────────────────────────────
	dir := config.Dir()
	if err := os.RemoveAll(dir); err != nil {
		ui.Fail("could not remove %s: %v", dir, err)
	} else {
		ui.Success("Removed %s", dir)
	}

	// ── 6. the binary ────────────────────────────────────────────────────
	if !keepBinary {
		if err := removeSelf(); err != nil {
			ui.Warn("%v", err)
		}
	}

	ui.Blank()
	ui.Line("%s", ui.Bold("linko is gone."))
	ui.Line("  %s", ui.Dim("Revoke the API token at "+TokenURL+" if you no longer need it."))
	ui.Line("  %s", ui.Dim("Reinstall any time: curl -fsSL "+InstallURL+" | bash"))
	ui.Blank()
	return nil
}

// removeSelf deletes the running binary. Unix lets a running executable be
// unlinked; Windows does not, so there we explain instead of failing.
func removeSelf() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate the linko binary: %w", err)
	}

	if runtime.GOOS == "windows" {
		ui.Info("Windows will not delete a running program.")
		ui.Info("Remove it yourself: %s", exe)
		return nil
	}

	if err := os.Remove(exe); err != nil {
		if os.IsPermission(err) {
			ui.Info("%s needs elevated permission to delete.", exe)
			ui.Info("Remove it with: sudo rm %s", exe)
			return nil
		}
		return fmt.Errorf("could not remove %s: %w", exe, err)
	}
	ui.Success("Removed %s", exe)
	return nil
}
