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

func newTokenCmd() *cobra.Command {
	var value string

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Replace the stored Cloudflare API token",
		Long: `token swaps the API token linko uses, without touching anything else.

  linko token                  paste a new one
  linko token --token cfut_…   non-interactive

The new token is checked against Cloudflare before it is saved, so a typo
cannot lock you out of your own configuration. linko then re-verifies that
it can still reach your domain and your tunnel.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runToken(ctx, value)
		},
	}
	cmd.Flags().StringVar(&value, "token", "", "the new token (or set LINKO_API_TOKEN)")
	return cmd
}

func runToken(ctx context.Context, value string) error {
	cfg, client, err := loadClient()
	if err != nil {
		return err
	}

	token := strings.TrimSpace(value)
	if token == "" {
		ui.Blank()
		printTokenSteps()
		p := ui.NewPrompter()
		token, err = p.AskSecret("New Cloudflare API token:")
		if err != nil {
			return err
		}
	}
	if token == "" {
		return fmt.Errorf("no token given")
	}
	if token == cfg.APIToken {
		ui.Info("That is the token already in use — nothing changed.")
		return nil
	}

	// Prove it works before overwriting the one that might still be good.
	probe := cloudflare.New(token)
	probe.AccountID = cfg.AccountID
	probe.ZoneID = cfg.ZoneID
	if _, verr := probe.VerifyToken(ctx); verr != nil {
		ui.Fail("Cloudflare rejected that token — the stored one is untouched.")
		ui.Info("%v", verr)
		return verr
	}

	cfg.APIToken = token
	client.Token = token
	if serr := cfg.Save(); serr != nil {
		return fmt.Errorf("saving the new token: %w", serr)
	}
	ui.Success("Token updated and saved to %s", config.Path())

	// A replacement token often has different permissions. Say so now rather
	// than at the next `linko 3000`.
	ui.Blank()
	if _, zerr := client.FindZone(ctx, cfg.Domain); zerr != nil {
		ui.Fail("It cannot reach %s", cfg.Domain)
		explainZoneListFailure(zerr)
		return zerr
	}
	ui.Success("Can reach %s", cfg.Domain)

	if _, terr := client.GetTunnel(ctx, cfg.TunnelID); terr != nil {
		ui.Fail("It cannot reach the tunnel %q", cfg.TunnelName)
		explainTunnelFailure(terr)
		return terr
	}
	ui.Success("Can reach the tunnel %s", cfg.TunnelName)

	ui.Blank()
	ui.Line("%s", ui.Bold("All good."))
	return nil
}
