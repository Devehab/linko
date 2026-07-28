package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/ui"
)

func newDomainCmd() *cobra.Command {
	var list bool
	var yes bool

	cmd := &cobra.Command{
		Use:     "domain [name]",
		Aliases: []string{"domains"},
		Short:   "Change the domain your URLs are published under",
		Long: `domain switches which of your Cloudflare domains linko publishes to.

  linko domain               pick from a numbered list
  linko domain example.com   switch straight to one
  linko domain --list        just show what is available

Everything already published stays where it is; new URLs use the new
domain. linko offers to clean the old ones up.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()

			wanted := ""
			if len(args) == 1 {
				wanted = args[0]
			}
			return runDomain(ctx, wanted, list, yes)
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "list the domains this token can see and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask before switching")
	return cmd
}

func runDomain(ctx context.Context, wanted string, listOnly, yes bool) error {
	cfg, client, err := loadClient()
	if err != nil {
		return err
	}
	p := ui.NewPrompter()

	if listOnly {
		zones, lerr := client.ListZones(ctx)
		if lerr != nil {
			explainZoneListFailure(lerr)
			return lerr
		}
		ui.Line("Domains this token can see:")
		for _, z := range zones {
			mark := " "
			if strings.EqualFold(z.Name, cfg.Domain) {
				mark = ui.Green("·")
			}
			ui.Line("  %s %s %s", mark, z.Name, ui.Dim(z.Status))
		}
		ui.Blank()
		ui.Info("current: %s", cfg.Domain)
		return nil
	}

	ui.Line("Currently publishing to %s", ui.Bold(cfg.BaseDomain))
	ui.Blank()

	zone, err := pickDomain(ctx, client, p, wanted, cfg.Domain, yes)
	if err != nil {
		return err
	}

	if strings.EqualFold(zone.Name, cfg.Domain) && strings.EqualFold(cfg.BaseDomain, zone.Name) {
		ui.Info("Already using %s — nothing changed.", zone.Name)
		return nil
	}

	// Anything already published lives in the old zone. It keeps working —
	// the DNS records still point at the same tunnel — but linko would no
	// longer be able to manage it, so offer to clear it out first.
	if len(cfg.Routes) > 0 {
		ui.Blank()
		ui.Warn("You have %s on %s:", plural(len(cfg.Routes), "URL"), cfg.Domain)
		for _, r := range cfg.SortedRoutes() {
			ui.Line("    https://%s %s %s", r.Hostname, ui.Dim("->"), r.Service)
		}
		ui.Blank()

		remove := yes
		if !yes {
			remove = p.Confirm("Delete them before switching?", true)
		}
		if remove {
			for _, r := range cfg.SortedRoutes() {
				if rerr := removeRoute(ctx, client, cfg, r); rerr != nil {
					ui.Fail("%s: %v", r.Hostname, rerr)
				} else {
					ui.Success("Removed %s", r.Hostname)
				}
			}
			_ = cfg.Save()
		} else {
			ui.Info("Leaving them in place — linko will no longer manage them.")
		}
	}

	previousAccount := cfg.AccountID

	cfg.Domain = zone.Name
	cfg.ZoneID = zone.ID
	cfg.BaseDomain = zone.Name
	if zone.Account.ID != "" {
		cfg.AccountID = zone.Account.ID
		cfg.AccountName = zone.Account.Name
	}
	client.ZoneID = cfg.ZoneID
	client.AccountID = cfg.AccountID

	// A domain in a different Cloudflare account means the stored tunnel is
	// not reachable any more; make one there instead.
	if cfg.AccountID != previousAccount {
		ui.Info("That domain is in a different Cloudflare account — setting up a tunnel there.")
		if terr := adoptOrCreateTunnel(ctx, cfg, client); terr != nil {
			return terr
		}
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	ui.Blank()
	ui.Success("Now publishing to %s", cfg.BaseDomain)
	ui.Line("  %s   %s", ui.Cyan("linko 3000"), ui.Dim("→ https://<name>."+cfg.BaseDomain))
	warnCertificateDepth(cfg.BaseDomain, cfg.Domain)
	return nil
}

// pickDomain is chooseZone with the current domain marked in the menu.
func pickDomain(ctx context.Context, client *cloudflare.Client, p *ui.Prompter,
	wanted, current string, yes bool) (*cloudflare.Zone, error) {

	if strings.TrimSpace(wanted) != "" || yes {
		return chooseZone(ctx, client, p, wanted, yes)
	}

	zones, err := client.ListZones(ctx)
	if err != nil {
		explainZoneListFailure(err)
		return nil, err
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("this token cannot see any domain")
	}
	if len(zones) == 1 {
		ui.Info("This token only has access to %s.", zones[0].Name)
		return &zones[0], nil
	}

	idx, err := p.Choose("Which domain should linko publish to?", zoneLabels(zones, current))
	if err != nil {
		return nil, err
	}
	return &zones[idx], nil
}

// adoptOrCreateTunnel makes sure a usable tunnel exists in the current
// account and stores its id and token.
func adoptOrCreateTunnel(ctx context.Context, cfg *config.Config, client *cloudflare.Client) error {
	name := cfg.TunnelName
	if strings.TrimSpace(name) == "" {
		name = defaultTunnelName(cfg.Domain)
	}

	tunnel, err := client.FindTunnel(ctx, name)
	if err != nil {
		explainTunnelFailure(err)
		return err
	}
	if tunnel == nil {
		tunnel, err = client.CreateTunnel(ctx, name)
		if err != nil {
			ui.Fail("Could not create a tunnel in that account")
			explainTunnelFailure(err)
			return err
		}
		ui.Success("Tunnel created (%s)", tunnel.Name)
	} else {
		ui.Success("Tunnel reused (%s)", tunnel.Name)
	}

	token, err := client.TunnelToken(ctx, tunnel.ID)
	if err != nil {
		return fmt.Errorf("fetching the tunnel token: %w", err)
	}
	cfg.TunnelID = tunnel.ID
	cfg.TunnelName = tunnel.Name
	cfg.TunnelToken = token
	return nil
}
