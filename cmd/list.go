package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/ui"
)

func newListCmd() *cobra.Command {
	var remote bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show the hostnames you have published",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runList(ctx, remote)
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "read the live routes from Cloudflare instead of the local config")
	return cmd
}

func runList(ctx context.Context, remote bool) error {
	cfg, client, err := loadClient()
	if err != nil {
		return err
	}

	if remote {
		tunnelCfg, err := client.GetTunnelConfig(ctx, cfg.TunnelID)
		if err != nil {
			return err
		}
		rules := cloudflare.HostnameRules(tunnelCfg.Ingress)
		if len(rules) == 0 {
			ui.Info("No routes configured on tunnel %s.", cfg.TunnelName)
			return nil
		}
		rows := make([][]string, 0, len(rules))
		for _, r := range rules {
			url := "https://" + r.Hostname
			rows = append(rows, []string{ui.Link(url, url), "->", r.Service})
		}
		ui.Table(os.Stdout, []string{"URL", "", "TARGET"}, rows)
		printClickHint()
		return nil
	}

	routes := cfg.SortedRoutes()
	if len(routes) == 0 {
		ui.Info("Nothing published yet. Try: linko 3000")
		return nil
	}

	rows := make([][]string, 0, len(routes))
	for _, r := range routes {
		kind := "persistent"
		if r.Ephemeral {
			kind = "temporary"
		}
		url := "https://" + r.Hostname
		rows = append(rows, []string{
			r.Name,
			ui.Link(url, url),
			"->",
			r.Service,
			kind,
		})
	}
	ui.Table(os.Stdout, []string{"NAME", "URL", "", "TARGET", "KIND"}, rows)
	ui.Blank()
	ui.Info("%d route(s) · tunnel %s · %s", len(routes), cfg.TunnelName, config.Path())
	printClickHint()
	return nil
}

// printClickHint tells the user how to follow the URLs just printed, when the
// terminal makes that non-obvious.
func printClickHint() {
	if hint := ui.ClickHint(); hint != "" {
		ui.Info("%s", hint)
	}
}

// routeNames is used for shell completion of `linko remove`.
func routeNames(prefix string) []string {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	out := []string{}
	for _, r := range cfg.Routes {
		if prefix == "" || strings.HasPrefix(r.Name, prefix) {
			out = append(out, r.Name)
		}
	}
	return out
}
