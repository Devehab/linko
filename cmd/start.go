package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/cloudflared"
	"github.com/Devehab/linko/internal/naming"
	"github.com/Devehab/linko/internal/target"
	"github.com/Devehab/linko/internal/ui"
)

type startOptions struct {
	name     string
	keep     bool
	replace  bool
	yes      bool
	verbose  bool
	logLevel string
}

func newStartCmd() *cobra.Command {
	opts := &startOptions{}

	cmd := &cobra.Command{
		Use:     "start <port|host:port|url>",
		Aliases: []string{"run", "up"},
		Short:   "Publish a local service on a public HTTPS URL",
		Long: `start publishes a locally running service through your Cloudflare tunnel.

  linko 3000                    random subdomain -> http://localhost:3000
  linko start 3000              the same thing, written out
  linko 3000 --name crm         crm.<base domain> -> http://localhost:3000
  linko 8080 --name api --keep  keep the hostname after you quit

The tunnel stays up until you press Ctrl+C.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runStart(ctx, args[0], opts)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&opts.name, "name", "n", "", "subdomain to use (default: random)")
	f.BoolVar(&opts.keep, "keep", false, "keep the hostname after the tunnel stops")
	f.BoolVar(&opts.replace, "replace", false, "replace the hostname if it already points somewhere else")
	f.BoolVarP(&opts.yes, "yes", "y", false, "do not ask questions")
	f.BoolVarP(&opts.verbose, "verbose", "v", false, "stream cloudflared logs")
	f.StringVar(&opts.logLevel, "loglevel", "info", "cloudflared log level: debug, info, warn, error, fatal")

	return cmd
}

func runStart(ctx context.Context, rawTarget string, opts *startOptions) error {
	cfg, client, err := loadClient()
	if err != nil {
		return err
	}

	service, port, err := target.Parse(rawTarget)
	if err != nil {
		return err
	}

	// Pick the subdomain label.
	label := strings.ToLower(strings.TrimSpace(opts.name))
	ephemeral := label == ""
	if ephemeral {
		label, err = naming.Random(naming.DefaultLength)
		if err != nil {
			return err
		}
	}
	if err := naming.ValidateLabel(label); err != nil {
		return err
	}
	hostname := cfg.Hostname(label)

	// What is already published under this hostname?
	tunnelCfg, err := client.GetTunnelConfig(ctx, cfg.TunnelID)
	if err != nil {
		return fmt.Errorf("reading the tunnel configuration: %w", err)
	}
	if existing, ok := cloudflare.FindIngress(tunnelCfg.Ingress, hostname); ok && existing.Service != service {
		action, err := resolveConflict(hostname, existing.Service, service, opts)
		if err != nil {
			return err
		}
		switch action {
		case conflictCancel:
			ui.Info("Cancelled.")
			return nil
		case conflictNewHostname:
			suffix, rerr := naming.Random(4)
			if rerr != nil {
				return rerr
			}
			label = label + "-" + suffix
			hostname = cfg.Hostname(label)
			ephemeral = true
			ui.Info("Using %s instead", hostname)
		case conflictReplace:
		}
	}

	// DNS
	dnsTarget := cloudflare.TunnelCNAMETarget(cfg.TunnelID)
	rec, created, err := client.EnsureCNAME(ctx, hostname, dnsTarget)
	if err != nil {
		explainDNSFailure(err, cfg.Domain)
		return fmt.Errorf("setting up DNS for %s: %w", hostname, err)
	}
	if created {
		ui.Success("DNS record created (%s)", hostname)
	}

	// Ingress
	tunnelCfg.Ingress = cloudflare.UpsertIngress(tunnelCfg.Ingress, hostname, service)
	if err := client.PutTunnelConfig(ctx, cfg.TunnelID, tunnelCfg); err != nil {
		return fmt.Errorf("updating the tunnel routes: %w", err)
	}
	ui.Success("Route published (%s -> %s)", hostname, service)

	route := config.Route{
		Name:      label,
		Hostname:  hostname,
		Service:   service,
		Port:      port,
		Ephemeral: ephemeral && !opts.keep,
	}
	if rec != nil {
		route.DNSRecordID = rec.ID
	}
	cfg.UpsertRoute(route)
	if err := cfg.Save(); err != nil {
		ui.Warn("could not update %s: %v", config.Path(), err)
	}

	// Clean up the ephemeral hostname when we exit.
	if route.Ephemeral {
		defer cleanupRoute(client, cfg, route)
	}

	// cloudflared
	mgr := cloudflared.New(config.BinDir())
	binary, err := mgr.Ensure(ctx, os.Stdout)
	if err != nil {
		return err
	}

	return runTunnel(ctx, mgr, binary, cfg, route, opts)
}

func runTunnel(ctx context.Context, mgr *cloudflared.Manager, binary string, cfg *config.Config, route config.Route, opts *startOptions) error {
	cmd := mgr.Command(ctx, binary, cloudflared.RunOptions{
		Token:    cfg.TunnelToken,
		LogLevel: opts.logLevel,
	})

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting cloudflared: %w", err)
	}

	watcher := newLogWatcher(opts.verbose, func() {
		printBanner(route, cfg)
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); watcher.scan(stdout) }()
	go func() { defer wg.Done(); watcher.scan(stderr) }()

	// Drain both pipes before Wait(): Wait closes them, which would truncate
	// cloudflared's output and race with the scanners.
	wg.Wait()
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		ui.Blank()
		ui.Info("Tunnel stopped.")
		return nil
	}
	if waitErr != nil {
		if tail := watcher.tail(); tail != "" {
			ui.Blank()
			fmt.Fprintln(os.Stderr, ui.Dim(tail))
		}
		return fmt.Errorf("cloudflared exited: %w", waitErr)
	}
	return nil
}

func printBanner(route config.Route, cfg *config.Config) {
	ui.Blank()
	ui.Success("Tunnel connected")
	ui.Blank()
	ui.Line("  %s  %s", ui.Dim("Public URL"), ui.Bold(ui.Cyan("https://"+route.Hostname)))
	ui.Line("  %s     %s", ui.Dim("Forwards"), route.Service)
	ui.Line("  %s       %s", ui.Dim("Tunnel"), cfg.TunnelName)
	if route.Ephemeral {
		ui.Line("  %s    %s", ui.Dim("Lifetime"), "removed when you quit (use --keep to persist)")
	}
	ui.Blank()
	ui.Line("  %s", ui.Dim("Press Ctrl+C to stop"))
	ui.Blank()
}

// logWatcher turns cloudflared's chatty output into a single "connected" line,
// while keeping the last few lines around in case it dies.
type logWatcher struct {
	verbose   bool
	onConnect func()

	mu        sync.Mutex
	announced bool
	recent    []string
}

func newLogWatcher(verbose bool, onConnect func()) *logWatcher {
	return &logWatcher{verbose: verbose, onConnect: onConnect}
}

func (w *logWatcher) scan(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		w.mu.Lock()
		w.recent = append(w.recent, line)
		if len(w.recent) > 15 {
			w.recent = w.recent[len(w.recent)-15:]
		}
		connected := false
		if !w.announced && isConnectedLine(line) {
			w.announced = true
			connected = true
		}
		verbose := w.verbose
		w.mu.Unlock()

		if verbose {
			fmt.Fprintln(os.Stderr, ui.Dim(line))
		}
		if connected && w.onConnect != nil {
			w.onConnect()
		}
	}
}

func (w *logWatcher) tail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.recent, "\n")
}

func isConnectedLine(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "registered tunnel connection") ||
		strings.Contains(l, "connection established") ||
		strings.Contains(l, "connection registered")
}

// explainDNSFailure turns Cloudflare's opaque "Authentication error (code
// 10000)" into the one thing the user actually has to change. Finding the zone
// only needs Zone:Read, so a token can get all the way through `linko init`
// and still be unable to write a record.
func explainDNSFailure(err error, domain string) {
	var apiErr *cloudflare.Error
	if !errors.As(err, &apiErr) || !apiErr.IsAuth() {
		return
	}
	ui.Blank()
	ui.Fail("Cloudflare refused to create the DNS record.")
	ui.Info("The token can read this zone but not change it.")
	ui.Info("Open https://dash.cloudflare.com/profile/api-tokens, edit the token and set:")
	ui.Info("  Zone → DNS → %s   (not Read)", ui.Bold("Edit"))
	ui.Info("  Zone Resources → Include → Specific zone → %s", domain)
	ui.Blank()
}

type conflictAction int

const (
	conflictReplace conflictAction = iota
	conflictNewHostname
	conflictCancel
)

func resolveConflict(hostname, current, next string, opts *startOptions) (conflictAction, error) {
	if opts.replace {
		return conflictReplace, nil
	}
	ui.Blank()
	ui.Warn("Hostname already in use: %s", ui.Bold(hostname))
	ui.Line("  %s %s", ui.Dim("currently ->"), current)
	ui.Line("  %s %s", ui.Dim("      new ->"), next)
	ui.Blank()

	if opts.yes {
		return conflictReplace, nil
	}

	p := ui.NewPrompter()
	choice, err := p.Choose("What would you like to do?", []string{
		"Replace the existing route",
		"Create another hostname",
		"Cancel",
	})
	if err != nil {
		return conflictCancel, err
	}
	return conflictAction(choice), nil
}

// cleanupRoute removes an ephemeral hostname from the tunnel and DNS.
func cleanupRoute(client *cloudflare.Client, cfg *config.Config, route config.Route) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := removeRoute(ctx, client, cfg, route); err != nil {
		ui.Warn("could not clean up %s: %v", route.Hostname, err)
		ui.Info("Remove it later with: linko remove %s", route.Name)
		return
	}
	if err := cfg.Save(); err != nil {
		ui.Warn("could not update %s: %v", config.Path(), err)
	}
	ui.Info("Removed %s", route.Hostname)
}

// removeRoute deletes the ingress rule, the DNS record and the local entry.
func removeRoute(ctx context.Context, client *cloudflare.Client, cfg *config.Config, route config.Route) error {
	tunnelCfg, err := client.GetTunnelConfig(ctx, cfg.TunnelID)
	if err != nil {
		return err
	}
	tunnelCfg.Ingress = cloudflare.RemoveIngress(tunnelCfg.Ingress, route.Hostname)
	if err := client.PutTunnelConfig(ctx, cfg.TunnelID, tunnelCfg); err != nil {
		return err
	}

	recordID := route.DNSRecordID
	if recordID == "" {
		rec, ferr := client.FindDNSRecord(ctx, route.Hostname)
		if ferr != nil {
			return ferr
		}
		if rec != nil && cloudflare.IsTunnelTarget(rec.Content) {
			recordID = rec.ID
		}
	}
	if recordID != "" {
		if err := client.DeleteDNSRecord(ctx, recordID); err != nil {
			return err
		}
	}

	cfg.RemoveRoute(route.Hostname)
	return nil
}
