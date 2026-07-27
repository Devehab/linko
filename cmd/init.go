package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/cloudflared"
	"github.com/Devehab/linko/internal/naming"
	"github.com/Devehab/linko/internal/ui"
)

type initOptions struct {
	token      string
	domain     string
	base       string
	tunnelName string
	force      bool
	yes        bool
	skipDL     bool
}

func newInitCmd() *cobra.Command {
	opts := &initOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "One-time setup: connect your Cloudflare account and create the tunnel",
		Long: `init walks through the one-time setup:

  1. store a Cloudflare API token
  2. find the DNS zone for your domain
  3. create (or reuse) a Zero Trust tunnel
  4. save everything to ` + config.Path(),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runInit(ctx, opts)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opts.token, "token", "", "Cloudflare API token (or set LINKO_API_TOKEN)")
	f.StringVar(&opts.domain, "domain", "", "domain managed by Cloudflare, e.g. example.com")
	f.StringVar(&opts.base, "base", "", "base subdomain for generated URLs, e.g. demo.example.com")
	f.StringVar(&opts.tunnelName, "tunnel", "", "tunnel name (default <domain>-linko-tunnel)")
	f.BoolVar(&opts.force, "force", false, "overwrite an existing configuration")
	f.BoolVarP(&opts.yes, "yes", "y", false, "non-interactive: fail instead of prompting")
	f.BoolVar(&opts.skipDL, "skip-download", false, "do not download cloudflared")

	return cmd
}

func runInit(ctx context.Context, opts *initOptions) error {
	p := ui.NewPrompter()

	existing, _ := config.Load()
	if config.Exists() && !opts.force {
		ui.Warn("A configuration already exists at %s", config.Path())
		if opts.yes {
			return fmt.Errorf("refusing to overwrite an existing config (use --force)")
		}
		if !p.Confirm("Reconfigure linko?", false) {
			ui.Info("Nothing changed.")
			return nil
		}
	}

	cfg := &config.Config{}
	if existing != nil {
		// Keep the routes we already published.
		cfg.Routes = existing.Routes
	}

	ui.Header("Cloudflare credentials")

	// 1. API token: --token, then $LINKO_API_TOKEN, then whatever is stored.
	token := resolveToken(opts.token, existing)
	if token == "" {
		if opts.yes {
			return fmt.Errorf("no API token: pass --token or set %s", config.EnvToken)
		}
		ui.Blank()
		printTokenSteps()
		var err error
		token, err = p.AskSecret("Cloudflare API token:")
		if err != nil {
			return err
		}
	}
	if token == "" {
		return fmt.Errorf("an API token is required")
	}
	cfg.APIToken = token

	client := cloudflare.New(token)
	if _, err := client.VerifyToken(ctx); err != nil {
		ui.Fail("Cloudflare rejected the token")
		return err
	}
	ui.Success("Cloudflare connected")

	// 2. Domain / zone
	ui.Header("Domain")
	domain := strings.ToLower(strings.TrimSpace(opts.domain))
	if domain == "" && existing != nil {
		domain = existing.Domain
	}
	if domain == "" {
		if opts.yes {
			return fmt.Errorf("--domain is required in non-interactive mode")
		}
		zones, err := client.ListZones(ctx)
		if err == nil && len(zones) > 0 {
			names := make([]string, 0, len(zones))
			for _, z := range zones {
				names = append(names, z.Name)
			}
			ui.Info("Zones on this account: %s", strings.Join(names, ", "))
			domain = names[0]
		}
		domain, err = p.AskRequired("Domain:", domain, func(s string) error {
			return naming.ValidateHostname(s)
		})
		if err != nil {
			return err
		}
	}

	zone, err := client.FindZone(ctx, domain)
	if err != nil {
		ui.Fail("DNS zone not found")
		explainZoneFailure(ctx, client, domain)
		return err
	}
	cfg.Domain = zone.Name
	cfg.ZoneID = zone.ID
	cfg.AccountID = zone.Account.ID
	cfg.AccountName = zone.Account.Name
	client.ZoneID = zone.ID
	client.AccountID = zone.Account.ID
	ui.Success("DNS zone found (%s)", zone.Name)
	if cfg.AccountID == "" {
		accounts, aerr := client.ListAccounts(ctx)
		if aerr != nil || len(accounts) == 0 {
			return fmt.Errorf("could not determine the Cloudflare account id — make sure the token has Account → Cloudflare Tunnel → Edit")
		}
		cfg.AccountID = accounts[0].ID
		cfg.AccountName = accounts[0].Name
		client.AccountID = accounts[0].ID
	}

	// 3. Base subdomain
	base := strings.ToLower(strings.TrimSpace(opts.base))
	if base == "" && existing != nil && strings.HasSuffix(existing.BaseDomain, cfg.Domain) {
		base = existing.BaseDomain
	}
	if base == "" {
		base = "demo." + cfg.Domain
		if !opts.yes {
			base, err = p.AskRequired("Base subdomain:", base, func(s string) error {
				return validateBase(s, cfg.Domain)
			})
			if err != nil {
				return err
			}
		}
	}
	base = expandBase(base, cfg.Domain)
	if err := validateBase(base, cfg.Domain); err != nil {
		return err
	}
	cfg.BaseDomain = base
	ui.Success("URLs will look like https://abc12.%s", base)
	warnCertificateDepth(base, cfg.Domain)

	// 4. Tunnel
	ui.Header("Tunnel")
	tunnelName := strings.TrimSpace(opts.tunnelName)
	if tunnelName == "" && existing != nil {
		tunnelName = existing.TunnelName
	}
	if tunnelName == "" {
		tunnelName = defaultTunnelName(cfg.Domain)
		if !opts.yes {
			tunnelName, err = p.AskRequired("Tunnel name:", tunnelName, nil)
			if err != nil {
				return err
			}
		}
	}

	tunnel, err := client.FindTunnel(ctx, tunnelName)
	if err != nil {
		ui.Fail("Could not list the tunnels on this account")
		explainTunnelFailure(err)
		return err
	}
	if tunnel == nil {
		tunnel, err = client.CreateTunnel(ctx, tunnelName)
		if err != nil {
			ui.Fail("Could not create the tunnel")
			explainTunnelFailure(err)
			return err
		}
		ui.Success("Tunnel created (%s)", tunnel.Name)
	} else {
		ui.Success("Tunnel reused (%s)", tunnel.Name)
	}
	cfg.TunnelID = tunnel.ID
	cfg.TunnelName = tunnel.Name

	tunnelToken, err := client.TunnelToken(ctx, tunnel.ID)
	if err != nil {
		return fmt.Errorf("fetching the tunnel token: %w", err)
	}
	cfg.TunnelToken = tunnelToken

	// Make sure the remote config has a valid catch-all rule.
	tunnelCfg, err := client.GetTunnelConfig(ctx, tunnel.ID)
	if err != nil {
		return fmt.Errorf("reading the tunnel configuration: %w", err)
	}
	if err := client.PutTunnelConfig(ctx, tunnel.ID, tunnelCfg); err != nil {
		return fmt.Errorf("initialising the tunnel configuration: %w", err)
	}
	ui.Success("Tunnel configuration ready")

	// 5. Save
	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Success("Configuration saved to %s", config.Path())

	// 6. cloudflared
	if !opts.skipDL {
		mgr := cloudflared.New(config.BinDir())
		if path, lerr := mgr.Locate(); lerr == nil {
			ui.Success("cloudflared found (%s)", path)
		} else {
			dctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			path, derr := mgr.Ensure(dctx, nil)
			cancel()
			if derr != nil {
				ui.Warn("cloudflared could not be downloaded: %v", derr)
				ui.Info("linko will try again the first time you run it.")
			} else {
				ui.Success("cloudflared installed (%s)", path)
			}
		}
	}

	ui.Blank()
	ui.Line("%s", ui.Bold("You're ready."))
	ui.Line("  %s   publish localhost:3000 on a random subdomain", ui.Cyan("linko 3000"))
	ui.Line("  %s   publish it on crm.%s", ui.Cyan("linko 3000 -n crm"), cfg.BaseDomain)
	ui.Blank()
	ui.Line("  %s %s", ui.Dim("Guide:"), ui.Cyan(DocsURL))
	ui.Line("  %s", ui.Dim("or run: linko docs"))
	ui.Blank()
	return nil
}

// explainZoneFailure turns "no zone named X" into something actionable by
// showing what the token can actually see.
func explainZoneFailure(ctx context.Context, client *cloudflare.Client, wanted string) {
	zones, err := client.ListZones(ctx)
	if err != nil {
		ui.Info("This token cannot list zones at all (%v).", err)
		ui.Info("It is missing the Zone permission. Edit the token and add:")
		ui.Info("  Zone → DNS → Edit,  with Zone Resources including %s", wanted)
		return
	}
	if len(zones) == 0 {
		ui.Info("This token can authenticate but sees zero zones.")
		ui.Info("Its Zone Resources are empty — edit the token and include %s.", wanted)
		return
	}

	names := make([]string, 0, len(zones))
	for _, z := range zones {
		names = append(names, z.Name)
	}
	ui.Info("Zones this token can see: %s", strings.Join(names, ", "))

	// A very common mistake: passing a subdomain where the zone apex is wanted.
	if z, ok := cloudflare.ZoneNameFor(wanted, zones); ok {
		ui.Info("%q sits inside the zone %q — pass --domain %s instead.", wanted, z.Name, z.Name)
	}
}

// extraLabels counts how many labels base adds on top of the zone apex.
//
//	extraLabels("example.com", "example.com")      = 0
//	extraLabels("demo.example.com", "example.com") = 1
func extraLabels(base, domain string) int {
	base = strings.ToLower(strings.Trim(strings.TrimSpace(base), "."))
	domain = strings.ToLower(strings.Trim(strings.TrimSpace(domain), "."))
	if base == "" || domain == "" || base == domain {
		return 0
	}
	prefix := strings.TrimSuffix(base, "."+domain)
	if prefix == base {
		return 0 // not inside the zone at all
	}
	return strings.Count(prefix, ".") + 1
}

// warnCertificateDepth catches the failure that costs the most time to
// diagnose. Cloudflare's free Universal SSL covers exactly example.com and
// *.example.com — one level. A base of demo.example.com therefore produces
// abc.demo.example.com, which no free certificate matches, and the browser
// fails the handshake outright with ERR_SSL_VERSION_OR_CIPHER_MISMATCH. DNS
// and the tunnel look perfectly healthy while this happens.
func warnCertificateDepth(base, domain string) {
	if extraLabels(base, domain) == 0 {
		return
	}
	ui.Blank()
	ui.Warn("Your URLs will be two labels deep: %s", ui.Bold("<name>."+base))
	ui.Info("Cloudflare's free Universal SSL only covers %s and *.%s,", domain, domain)
	ui.Info("so HTTPS will fail with a certificate error even though DNS and")
	ui.Info("the tunnel are fine.")
	ui.Info("Fix it either way:")
	ui.Info("  · re-run with %s   (URLs become <name>.%s)", ui.Bold("--base "+domain), domain)
	ui.Info("  · or enable Total TLS / Advanced Certificate Manager on %s", domain)
	ui.Blank()
}

// explainTunnelFailure covers the other half of the permission story: a token
// scoped only to DNS gets through zone lookup and then fails here, again with
// a bare "Authentication error (code 10000)".
func explainTunnelFailure(err error) {
	var apiErr *cloudflare.Error
	if !errors.As(err, &apiErr) || !apiErr.IsAuth() {
		return
	}
	ui.Blank()
	ui.Info("This token can reach your DNS but not Zero Trust tunnels.")
	ui.Info("linko needs BOTH permissions on the SAME token:")
	ui.Info("  Zone    → DNS               → Edit")
	ui.Info("  Account → Cloudflare Tunnel → Edit")
	ui.Info("Edit it at https://dash.cloudflare.com/profile/api-tokens.")
	ui.Blank()
}

// resolveToken picks the Cloudflare API token from, in order: the --token
// flag, the LINKO_API_TOKEN environment variable, and the stored config.
//
// Reading the environment here matters: config.Load applies the same override,
// but only when a config file already exists — so without this, `linko init
// --yes` on a fresh machine would reject a perfectly good LINKO_API_TOKEN.
func resolveToken(flagValue string, existing *config.Config) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(config.EnvToken)); v != "" {
		return v
	}
	if existing != nil {
		return strings.TrimSpace(existing.APIToken)
	}
	return ""
}

// expandBase turns "demo" into "demo.example.com".
func expandBase(base, domain string) string {
	base = strings.ToLower(strings.Trim(strings.TrimSpace(base), "."))
	if base == "" || base == "@" {
		return domain
	}
	if base == domain || strings.HasSuffix(base, "."+domain) {
		return base
	}
	return base + "." + domain
}

func validateBase(base, domain string) error {
	base = expandBase(base, domain)
	if base != domain && !strings.HasSuffix(base, "."+domain) {
		return fmt.Errorf("%q must be inside %s", base, domain)
	}
	return naming.ValidateHostname(base)
}

func defaultTunnelName(domain string) string {
	label := strings.SplitN(domain, ".", 2)[0]
	if label == "" {
		label = "linko"
	}
	return label + "-linko-tunnel"
}
