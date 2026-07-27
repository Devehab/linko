package cmd

import (
	"context"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/ui"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the tunnel connection state and route count",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runStatus(ctx)
		},
	}
}

func runStatus(ctx context.Context) error {
	cfg, client, err := loadClient()
	if err != nil {
		return err
	}

	tunnel, err := client.GetTunnel(ctx, cfg.TunnelID)
	if err != nil {
		return err
	}
	tunnelCfg, cfgErr := client.GetTunnelConfig(ctx, cfg.TunnelID)

	ui.Header("Cloudflare Tunnel")
	ui.Line("  %s        %s", ui.Dim("Name"), tunnel.Name)
	ui.Line("  %s          %s", ui.Dim("ID"), tunnel.ID)
	ui.Line("  %s     %s", ui.Dim("Account"), fallback(cfg.AccountName, cfg.AccountID))
	ui.Line("  %s      %s", ui.Dim("Domain"), cfg.BaseDomain)

	active := tunnel.ActiveConnections()
	if active > 0 {
		colos := tunnel.Colos()
		detail := ""
		if len(colos) > 0 {
			detail = " via " + strings.Join(colos, ", ")
		}
		ui.Line("  %s      %s", ui.Dim("Status"), ui.Green("connected")+ui.Dim(" ("+plural(active, "connection")+detail+")"))
	} else {
		ui.Line("  %s      %s", ui.Dim("Status"), ui.Yellow("not running"))
	}

	ui.Header("Routes")
	if cfgErr != nil {
		ui.Warn("could not read the live routes: %v", cfgErr)
	} else {
		rules := cloudflare.HostnameRules(tunnelCfg.Ingress)
		if len(rules) == 0 {
			ui.Info("no routes configured")
		}
		for _, r := range rules {
			mark := ui.Dim("·")
			if cfg.FindRouteByHostname(r.Hostname) != nil {
				mark = ui.Green("·")
			}
			url := "https://" + r.Hostname
			ui.Line("  %s %s %s %s", mark, ui.Link(url, url), ui.Dim("->"), r.Service)
		}
	}

	ui.Blank()
	ui.Info("config: %s", config.Path())
	return nil
}

func fallback(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
