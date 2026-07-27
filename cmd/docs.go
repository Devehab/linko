package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Devehab/linko/internal/ui"
)

// DocsURL is printed after installation, during `linko init`, and by
// `linko docs`. It points at a Markdown file so it renders on GitHub without
// any Pages setup.
const DocsURL = "https://github.com/Devehab/linko/blob/main/GUIDE.md"

// TokenURL is where Cloudflare API tokens are created.
const TokenURL = "https://dash.cloudflare.com/profile/api-tokens"

func newDocsCmd() *cobra.Command {
	var open bool

	cmd := &cobra.Command{
		Use:     "docs",
		Aliases: []string{"guide", "help-me"},
		Short:   "Show the setup guide, including how to create the Cloudflare token",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			printGuide()
			if open {
				if err := openBrowser(DocsURL); err != nil {
					ui.Warn("could not open a browser: %v", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&open, "open", "o", false, "open the full guide in your browser")
	return cmd
}

// printTokenSteps is the condensed version shown inline by `linko init`, where
// the user is one keystroke away from needing exactly this.
func printTokenSteps() {
	ui.Line("  %s", ui.Bold("Create a Cloudflare API token"))
	ui.Blank()
	ui.Line("  1. %s", ui.Cyan(TokenURL))
	ui.Line("     Create Token → Create Custom Token")
	ui.Blank()
	ui.Line("  2. Add %s permission rows (+ Add more):", ui.Bold("both"))
	ui.Line("       Zone     →  DNS               →  %s", ui.Bold("Edit"))
	ui.Line("       Account  →  Cloudflare Tunnel →  %s", ui.Bold("Edit"))
	ui.Blank()
	ui.Line("  3. Zone Resources → Include → Specific zone → your domain")
	ui.Line("     %s", ui.Dim("leave this empty and the token sees no domains at all"))
	ui.Blank()
	ui.Line("  %s %s", ui.Dim("Full guide:"), ui.Cyan(DocsURL))
	ui.Blank()
}

func printGuide() {
	ui.Header("linko — setup guide")

	printTokenSteps()

	ui.Line("  %s", ui.Bold("Set it up"))
	ui.Blank()
	ui.Line("    linko init")
	ui.Line("    %s", ui.Dim("when asked for \"Base subdomain\", answer with your bare domain"))
	ui.Line("    %s", ui.Dim("example.com — not demo.example.com (free certificates cover one level)"))
	ui.Blank()

	ui.Line("  %s", ui.Bold("Everyday use"))
	ui.Blank()
	ui.Line("    linko 3000                 %s", ui.Dim("random URL, removed when you quit"))
	ui.Line("    linko 3000 --name crm      %s", ui.Dim("https://crm.<your domain>, kept"))
	ui.Line("    linko 3000 --keep          %s", ui.Dim("keep the random one too"))
	ui.Blank()
	ui.Line("    linko list                 %s", ui.Dim("what is published"))
	ui.Line("    linko status               %s", ui.Dim("tunnel connection state"))
	ui.Line("    linko remove crm           %s", ui.Dim("delete a URL and its DNS record"))
	ui.Line("    linko doctor               %s", ui.Dim("check the whole setup"))
	ui.Blank()

	ui.Line("  %s", ui.Bold("Stuck?"))
	ui.Blank()
	ui.Line("    linko doctor               %s", ui.Dim("says exactly which step fails"))
	ui.Line("    %s", ui.Cyan(DocsURL))
	ui.Blank()
	ui.Line("  %s", ui.Dim("Open it in a browser:  linko docs --open"))
	ui.Blank()
}

// openBrowser opens a URL with the platform's default handler.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening %s: %w", url, err)
	}
	return nil
}
