package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/config"
	"github.com/Devehab/linko/internal/ui"
)

func newStopCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "stop [name...]",
		Short: "Stop a tunnel running in the background",
		Long: `stop shuts down tunnels started with -d.

  linko stop crm     stop one
  linko stop --all   stop every background tunnel

The URLs stay published unless they were started with --temp, so starting
the same port again hands back the same address.`,
		Args: cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			cfg, _ := config.Load()
			names := []string{}
			for _, bg := range runningTunnels(cfg) {
				names = append(names, bg.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(args, all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "stop every background tunnel")
	return cmd
}

func runStop(names []string, all bool) error {
	cfg, _ := config.Load()
	running := runningTunnels(cfg)

	if all {
		names = names[:0]
		for _, bg := range running {
			names = append(names, bg.Name)
		}
	}

	if len(names) == 0 {
		if len(running) == 0 {
			ui.Info("Nothing is running in the background.")
			return nil
		}
		ui.Line("Running in the background:")
		for _, bg := range running {
			ui.Line("  %s %s", ui.Bold(bg.Name), ui.Dim(fmt.Sprintf("(pid %d)", bg.PID)))
		}
		ui.Blank()
		return fmt.Errorf("give a name to stop, or use --all")
	}

	var failed int
	for _, name := range names {
		if err := stopTunnel(name); err != nil {
			ui.Fail("%v", err)
			failed++
			continue
		}
		ui.Success("Stopped %s", name)
	}
	if failed > 0 {
		return fmt.Errorf("%d tunnel(s) could not be stopped", failed)
	}
	return nil
}

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ps",
		Aliases: []string{"running"},
		Short:   "Show tunnels running in the background",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			running := runningTunnels(cfg)
			if len(running) == 0 {
				ui.Info("Nothing is running in the background.")
				ui.Info("Start one with: linko 3000 -d")
				return nil
			}

			rows := make([][]string, 0, len(running))
			for _, bg := range running {
				url := "-"
				if bg.Hostname != "" {
					url = "https://" + bg.Hostname
					url = ui.Link("https://"+bg.Hostname, url)
				}
				target := bg.Service
				if target == "" {
					target = "-"
				}
				rows = append(rows, []string{
					bg.Name,
					url,
					"->",
					target,
					fmt.Sprintf("pid %d", bg.PID),
				})
			}
			ui.Table(os.Stdout, []string{"NAME", "URL", "", "TARGET", "PROCESS"}, rows)
			ui.Blank()
			ui.Info("logs: %s", config.LogDir())
			printClickHint()
			return nil
		},
	}
}
