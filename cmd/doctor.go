package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ibtkrgo/linko/cloudflare"
	"github.com/ibtkrgo/linko/config"
	"github.com/ibtkrgo/linko/internal/cloudflared"
	"github.com/ibtkrgo/linko/internal/ui"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that everything linko needs is in place",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runDoctor(ctx)
		},
	}
}

type checkResult struct {
	failed  int
	warned  int
	skipped bool
}

func (r *checkResult) ok(format string, a ...any)   { ui.Success(format, a...) }
func (r *checkResult) warn(format string, a ...any) { r.warned++; ui.Warn(format, a...) }
func (r *checkResult) bad(format string, a ...any)  { r.failed++; ui.Fail(format, a...) }

func runDoctor(ctx context.Context) error {
	res := &checkResult{}
	ui.Header("linko doctor")

	// 1. cloudflared
	mgr := cloudflared.New(config.BinDir())
	binary, lerr := mgr.Locate()
	if lerr != nil {
		res.bad("cloudflared not installed — it will be downloaded on first use")
	} else {
		version, verr := mgr.Version(ctx, binary)
		if verr != nil {
			res.warn("cloudflared found at %s but did not run: %v", binary, verr)
		} else {
			res.ok("cloudflared installed (%s)", firstLine(version))
		}
	}

	// 2. config
	cfg, cerr := config.Load()
	if cerr != nil {
		res.bad("%v", cerr)
		return summarize(res)
	}
	if verr := cfg.Validate(); verr != nil {
		res.bad("%v", verr)
		return summarize(res)
	}
	res.ok("config valid (%s)", config.Path())

	client := cloudflare.New(cfg.APIToken)
	client.AccountID = cfg.AccountID
	client.ZoneID = cfg.ZoneID

	// 3. token
	if _, terr := client.VerifyToken(ctx); terr != nil {
		res.bad("API token rejected: %v", terr)
		return summarize(res)
	}
	res.ok("API token valid")

	// 4. zone
	zone, zerr := client.FindZone(ctx, cfg.Domain)
	if zerr != nil {
		res.bad("DNS zone unreachable: %v", zerr)
	} else if zone.ID != cfg.ZoneID {
		res.warn("zone id changed for %s — re-run `linko init`", cfg.Domain)
	} else {
		res.ok("DNS zone reachable (%s)", zone.Name)
	}

	// 5. tunnel
	tunnel, terr := client.GetTunnel(ctx, cfg.TunnelID)
	if terr != nil {
		res.bad("tunnel not found: %v", terr)
		return summarize(res)
	}
	res.ok("tunnel exists (%s)", tunnel.Name)

	// 6. routes
	tunnelCfg, tcerr := client.GetTunnelConfig(ctx, cfg.TunnelID)
	if tcerr != nil {
		res.bad("tunnel configuration unreadable: %v", tcerr)
	} else {
		rules := cloudflare.HostnameRules(tunnelCfg.Ingress)
		res.ok("tunnel configuration readable (%s)", plural(len(rules), "route"))

		// 7. DNS records for the routes we track
		missing := []string{}
		for _, r := range cfg.Routes {
			rec, ferr := client.FindDNSRecord(ctx, r.Hostname)
			if ferr != nil || rec == nil {
				missing = append(missing, r.Hostname)
				continue
			}
			if !cloudflare.IsTunnelTarget(rec.Content) {
				missing = append(missing, r.Hostname+" (points at "+rec.Content+")")
			}
		}
		switch {
		case len(cfg.Routes) == 0:
			ui.Info("no routes published yet")
		case len(missing) == 0:
			res.ok("DNS configured for %s", plural(len(cfg.Routes), "route"))
		default:
			res.warn("DNS missing or wrong for: %s", strings.Join(missing, ", "))
		}
	}

	// 8. live connection
	if tunnel.ActiveConnections() > 0 {
		res.ok("connection active (%s)", plural(tunnel.ActiveConnections(), "edge connection"))
	} else {
		ui.Info("tunnel is not running right now (start one with `linko 3000`)")
	}

	return summarize(res)
}

func summarize(res *checkResult) error {
	ui.Blank()
	if res.failed == 0 && res.warned == 0 {
		ui.Line("%s", ui.Green("Everything looks good."))
		return nil
	}
	if res.failed == 0 {
		ui.Line("%s", ui.Yellow(fmt.Sprintf("%d warning(s).", res.warned)))
		return nil
	}
	return fmt.Errorf("%d check(s) failed", res.failed)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
