package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/ui"
)

// This file handles the situations where the world changed underneath us: the
// API token stopped working, the tunnel was deleted from the dashboard, or a
// DNS record was removed by hand. In every case linko explains what happened
// in plain terms and then fixes it, rather than printing a Cloudflare error
// code and giving up.

func isAuthError(err error) bool {
	var apiErr *cloudflare.Error
	return errors.As(err, &apiErr) && apiErr.IsAuth()
}

func isNotFound(err error) bool {
	var apiErr *cloudflare.Error
	return errors.As(err, &apiErr) && apiErr.IsNotFound()
}

// preflight makes sure the credentials still work and the tunnel still exists
// before we try to publish anything, repairing whichever is broken.
func preflight(ctx context.Context, cfg *config.Config, client *cloudflare.Client, yes bool) error {
	for attempt := 0; attempt < 3; attempt++ {
		tunnel, err := client.GetTunnel(ctx, cfg.TunnelID)

		switch {
		case err == nil && tunnel != nil && tunnel.DeletedAt == nil:
			return nil

		case isAuthError(err):
			if rerr := reauthenticate(ctx, cfg, client, yes); rerr != nil {
				return rerr
			}

		case err == nil || isNotFound(err):
			// Either Cloudflare says the tunnel is gone, or it answered with a
			// tunnel that has been deleted.
			if rerr := repairTunnel(ctx, cfg, client, yes); rerr != nil {
				return rerr
			}

		default:
			return err
		}
	}
	return fmt.Errorf("could not get the tunnel into a working state — run `linko doctor`")
}

// reauthenticate explains that the stored token stopped working, takes a new
// one, verifies it against Cloudflare before trusting it, and saves it.
func reauthenticate(ctx context.Context, cfg *config.Config, client *cloudflare.Client, yes bool) error {
	ui.Blank()
	ui.Fail("Cloudflare is no longer accepting the stored API token.")
	ui.Info("It was most likely deleted, edited, or it expired.")
	ui.Blank()

	if yes {
		ui.Info("Provide a working token and run the command again:")
		ui.Info("  export %s='…'", config.EnvToken)
		ui.Info("Create one at %s", TokenURL)
		return fmt.Errorf("the stored API token is no longer valid")
	}

	printTokenSteps()

	p := ui.NewPrompter()
	for attempt := 0; attempt < 3; attempt++ {
		token, err := p.AskSecret("New Cloudflare API token:")
		if err != nil {
			return err
		}
		if token == "" {
			ui.Warn("a token is required")
			continue
		}

		// Verify before saving, so a typo cannot lock the user out of their
		// own configuration.
		probe := cloudflare.New(token)
		probe.AccountID = cfg.AccountID
		probe.ZoneID = cfg.ZoneID
		if _, verr := probe.VerifyToken(ctx); verr != nil {
			ui.Fail("Cloudflare rejected that one too.")
			ui.Info("%v", verr)
			continue
		}

		cfg.APIToken = token
		client.Token = token
		if serr := cfg.Save(); serr != nil {
			return fmt.Errorf("saving the new token: %w", serr)
		}
		ui.Success("Token updated and saved to %s", config.Path())
		return nil
	}
	return fmt.Errorf("no working token was supplied")
}

// repairTunnel handles a tunnel that disappeared from the account. It adopts
// one of the same name if it exists, otherwise recreates it, and then
// republishes every URL — including re-pointing the DNS records, because a new
// tunnel means a new CNAME target.
func repairTunnel(ctx context.Context, cfg *config.Config, client *cloudflare.Client, yes bool) error {
	ui.Blank()
	ui.Warn("The tunnel %q is no longer in your Cloudflare account.", cfg.TunnelName)
	ui.Info("It was probably deleted from the Zero Trust dashboard.")
	ui.Blank()

	if !yes {
		if !ui.NewPrompter().Confirm("Recreate it and republish your URLs?", true) {
			return fmt.Errorf("tunnel %q no longer exists", cfg.TunnelName)
		}
	}

	tunnel, err := client.FindTunnel(ctx, cfg.TunnelName)
	if err != nil {
		explainTunnelFailure(err)
		return err
	}
	if tunnel == nil {
		tunnel, err = client.CreateTunnel(ctx, cfg.TunnelName)
		if err != nil {
			ui.Fail("Could not recreate the tunnel")
			explainTunnelFailure(err)
			return err
		}
		ui.Success("Tunnel recreated (%s)", tunnel.Name)
	} else {
		ui.Success("Found a tunnel named %q already — adopting it", tunnel.Name)
	}

	cfg.TunnelID = tunnel.ID
	token, err := client.TunnelToken(ctx, tunnel.ID)
	if err != nil {
		return fmt.Errorf("fetching the new tunnel token: %w", err)
	}
	cfg.TunnelToken = token
	if err := cfg.Save(); err != nil {
		return err
	}

	if len(cfg.Routes) > 0 {
		fixed, rerr := repairRoutes(ctx, cfg, client)
		if rerr != nil {
			return rerr
		}
		ui.Success("Republished %s onto the new tunnel", plural(fixed, "URL"))
	}
	return nil
}

// repairRoutes puts every known URL back: the DNS record pointing at the
// current tunnel, and the ingress rule pointing at the local port. Safe to run
// when nothing is broken — it only reports what it actually changed.
func repairRoutes(ctx context.Context, cfg *config.Config, client *cloudflare.Client) (int, error) {
	tunnelCfg, err := client.GetTunnelConfig(ctx, cfg.TunnelID)
	if err != nil {
		return 0, fmt.Errorf("reading the tunnel configuration: %w", err)
	}

	target := cloudflare.TunnelCNAMETarget(cfg.TunnelID)
	changed := 0

	for i := range cfg.Routes {
		r := &cfg.Routes[i]

		rec, created, derr := client.EnsureCNAME(ctx, r.Hostname, target)
		if derr != nil {
			explainDNSFailure(derr, cfg.Domain)
			return changed, fmt.Errorf("restoring DNS for %s: %w", r.Hostname, derr)
		}
		if rec != nil {
			r.DNSRecordID = rec.ID
		}
		if created {
			ui.Info("DNS record restored for %s", r.Hostname)
			changed++
		}

		existing, ok := cloudflare.FindIngress(tunnelCfg.Ingress, r.Hostname)
		if !ok || existing.Service != r.Service {
			ui.Info("Route restored for %s -> %s", r.Hostname, r.Service)
			changed++
		}
		tunnelCfg.Ingress = cloudflare.UpsertIngress(tunnelCfg.Ingress, r.Hostname, r.Service)
	}

	if err := client.PutTunnelConfig(ctx, cfg.TunnelID, tunnelCfg); err != nil {
		return changed, fmt.Errorf("updating the tunnel routes: %w", err)
	}
	if err := cfg.Save(); err != nil {
		return changed, err
	}
	return changed, nil
}
