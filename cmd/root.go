// Package cmd wires up linko's command line interface.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/cloudflare"
	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/target"
	"github.com/Devehab/linko/internal/ui"
)

// Version is set from main at build time.
var Version = "dev"

var noColor bool

// NewRootCmd builds the command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "linko",
		Short: "Expose a local port on a public HTTPS URL via Cloudflare Tunnel",
		Long: `linko turns a service running on your machine into a public HTTPS URL
using your own Cloudflare account and domain.

  linko init          one-time setup
  linko 3000          publish localhost:3000 on a random subdomain
  linko 3000 -n crm   publish it on crm.<your base domain>`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				ui.SetColor(false)
			}
		},
	}

	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable coloured output")
	root.SetVersionTemplate("linko {{.Version}}\n")

	root.AddCommand(
		newInitCmd(),
		newStartCmd(),
		newListCmd(),
		newRemoveCmd(),
		newStatusCmd(),
		newDoctorCmd(),
	)
	return root
}

// Execute runs linko and converts errors into a clean exit.
func Execute(version string) {
	Version = version
	cloudflare.UserAgent = "linko/" + version

	root := NewRootCmd()
	root.SetArgs(NormalizeArgs(os.Args[1:], commandNames(root)))

	if err := root.Execute(); err != nil {
		ui.Blank()
		fmt.Fprintf(os.Stderr, "%s %v\n", ui.Red("Error:"), err)
		if errors.Is(err, config.ErrNotInitialized) {
			fmt.Fprintf(os.Stderr, "\nRun %s to get started.\n", ui.Bold("linko init"))
		}
		os.Exit(1)
	}
}

func commandNames(root *cobra.Command) map[string]bool {
	names := map[string]bool{
		"help":       true,
		"completion": true,
	}
	for _, c := range root.Commands() {
		names[c.Name()] = true
		for _, a := range c.Aliases {
			names[a] = true
		}
	}
	return names
}

// NormalizeArgs rewrites `linko 3000 …` into `linko start 3000 …` so a port can
// be used as the default command.
func NormalizeArgs(args []string, known map[string]bool) []string {
	if len(args) == 0 {
		return args
	}
	first := args[0]
	if known[first] {
		return args
	}
	if len(first) > 0 && first[0] == '-' {
		return args
	}
	if target.Looks(first) {
		return append([]string{"start"}, args...)
	}
	return args
}

// signalContext cancels when the user hits Ctrl+C.
func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(ch)
	}()
	return ctx, cancel
}

// loadClient loads the config and returns a ready Cloudflare client.
func loadClient() (*config.Config, *cloudflare.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	client := cloudflare.New(cfg.APIToken)
	client.AccountID = cfg.AccountID
	client.ZoneID = cfg.ZoneID
	return cfg, client, nil
}
