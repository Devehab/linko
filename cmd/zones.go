package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/internal/ui"
)

// chooseZone works out which Cloudflare domain to use.
//
// Typing a domain name by hand is the slowest and most error-prone part of
// setup — the token already tells us exactly which domains are available, so
// we list them and take a number instead.
//
//	wanted != ""      use it, no questions
//	one domain only   pick it silently
//	several           show a numbered list
func chooseZone(ctx context.Context, client *cloudflare.Client, p *ui.Prompter,
	wanted string, yes bool) (*cloudflare.Zone, error) {

	wanted = strings.ToLower(strings.TrimSpace(wanted))

	zones, err := client.ListZones(ctx)
	if err != nil {
		// A token may be allowed to read one zone without being allowed to
		// list them, so a named lookup can still succeed.
		if wanted != "" {
			return client.FindZone(ctx, wanted)
		}
		ui.Fail("Could not read the domains on this account")
		explainZoneListFailure(err)
		return nil, err
	}

	if len(zones) == 0 {
		ui.Fail("This token cannot see any domain")
		ui.Info("Edit it at %s and set:", TokenURL)
		ui.Info("  Zone → DNS → %s", ui.Bold("Edit"))
		ui.Info("  Zone Resources → Include → Specific zone → your domain")
		return nil, fmt.Errorf("no domains visible to this token")
	}

	if wanted != "" {
		for i := range zones {
			if strings.EqualFold(zones[i].Name, wanted) {
				return &zones[i], nil
			}
		}
		names := zoneNames(zones)
		ui.Fail("No domain named %q on this account", wanted)
		ui.Info("This token can see: %s", strings.Join(names, ", "))
		return nil, fmt.Errorf("unknown domain %q", wanted)
	}

	if len(zones) == 1 {
		ui.Success("Domain: %s", zones[0].Name)
		return &zones[0], nil
	}

	if yes {
		return nil, fmt.Errorf("this account has %d domains — pass --domain to pick one (%s)",
			len(zones), strings.Join(zoneNames(zones), ", "))
	}

	idx, err := p.Choose("Which domain should linko use?", zoneLabels(zones, ""))
	if err != nil {
		return nil, err
	}
	return &zones[idx], nil
}

func zoneNames(zones []cloudflare.Zone) []string {
	out := make([]string, 0, len(zones))
	for _, z := range zones {
		out = append(out, z.Name)
	}
	return out
}

// zoneLabels renders the menu, marking the one currently in use and flagging
// any domain that is not fully active on Cloudflare yet.
func zoneLabels(zones []cloudflare.Zone, current string) []string {
	out := make([]string, 0, len(zones))
	for _, z := range zones {
		label := z.Name
		if current != "" && strings.EqualFold(z.Name, current) {
			label += "  " + ui.Dim("(current)")
		}
		if z.Status != "" && !strings.EqualFold(z.Status, "active") {
			label += "  " + ui.Yellow("("+z.Status+")")
		}
		out = append(out, label)
	}
	return out
}

func explainZoneListFailure(err error) {
	if !isAuthError(err) {
		ui.Info("%v", err)
		return
	}
	ui.Info("The token is not allowed to read your domains.")
	ui.Info("Edit it at %s and add:", TokenURL)
	ui.Info("  Zone → DNS → %s", ui.Bold("Edit"))
	ui.Info("  Zone Resources → Include → Specific zone → your domain")
}
