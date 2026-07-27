package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ibtkrgo/linko/config"
	"github.com/ibtkrgo/linko/internal/ui"
)

func newRemoveCmd() *cobra.Command {
	var all bool
	var yes bool

	cmd := &cobra.Command{
		Use:     "remove <name...>",
		Aliases: []string{"rm", "delete"},
		Short:   "Delete a published hostname (route + DNS record)",
		Args:    cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return routeNames(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(cmd.Context())
			defer cancel()
			return runRemove(ctx, args, all, yes)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "remove every route linko knows about")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

func runRemove(ctx context.Context, names []string, all, yes bool) error {
	cfg, client, err := loadClient()
	if err != nil {
		return err
	}

	var targets []config.Route
	switch {
	case all:
		targets = cfg.SortedRoutes()
	case len(names) == 0:
		return fmt.Errorf("give a name to remove, or use --all\n\nSee what is published with: linko list")
	default:
		for _, n := range names {
			r := cfg.FindRoute(n)
			if r == nil {
				r = cfg.FindRouteByHostname(cfg.Hostname(n))
			}
			if r == nil {
				return fmt.Errorf("no route named %q — run `linko list` to see them", n)
			}
			targets = append(targets, *r)
		}
	}

	if len(targets) == 0 {
		ui.Info("Nothing to remove.")
		return nil
	}

	if !yes {
		ui.Line("This will delete:")
		for _, r := range targets {
			ui.Line("  %s %s %s", ui.Bold(r.Hostname), ui.Dim("->"), r.Service)
		}
		if !ui.NewPrompter().Confirm("Continue?", false) {
			ui.Info("Cancelled.")
			return nil
		}
	}

	var failures int
	for _, r := range targets {
		if err := removeRoute(ctx, client, cfg, r); err != nil {
			ui.Fail("%s: %v", r.Hostname, err)
			failures++
			continue
		}
		ui.Success("Removed %s", r.Hostname)
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("%d route(s) could not be removed", failures)
	}
	return nil
}
