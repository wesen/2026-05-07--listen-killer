package main

import (
	"fmt"
	"os"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/spf13/cobra"

	listcmd "github.com/wesen/listen-killer/cmd/listen-killer/cmds/tui"
)

var rootCmd = &cobra.Command{
	Use:   "listen-killer",
	Short: "Structured CLI and TUI for managing listening TCP daemons",
	Long: `Listen Killer shows all TCP listening sockets with process metadata.
The list command is script-friendly by default and emits Markdown unless another
Glazed output format is requested.

Run without arguments to print a Markdown listener table:
  listen-killer

For the interactive dashboard:
  listen-killer tui

For CLI/scripting and LLM-agent mode:
  listen-killer list --output json
  listen-killer kill --port 3000 --dry-run --output json
  listen-killer kill --port 3000 --signal TERM --yes`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromCobra(cmd)
	},
}

func main() {
	// Add logging flags (--log-level, --log-format)
	if err := logging.AddLoggingSectionToRootCommand(rootCmd, "listen-killer"); err != nil {
		fmt.Fprintf(os.Stderr, "Error adding logging: %v\n", err)
		os.Exit(1)
	}

	// Register the "list" subcommand (structured Glazed output; never launches TUI)
	listCmd, err := listcmd.NewListCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating list command: %v\n", err)
		os.Exit(1)
	}

	cobraListCmd, err := cli.BuildCobraCommand(listCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "listen-killer",
			ShortHelpSections: []string{schema.DefaultSlug},
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building cobra list command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraListCmd)

	// Register the explicit TUI subcommand. The list command never auto-forks the TUI.
	rootCmd.AddCommand(&cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive Bubbletea dashboard",
		Long: `Launch the interactive Bubbletea dashboard for browsing, marking,
opening, and killing listening TCP server processes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listcmd.RunTUI()
		},
	})

	// Register the "kill" subcommand (structured, scriptable process killer)
	killCmd, err := listcmd.NewKillCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating kill command: %v\n", err)
		os.Exit(1)
	}

	cobraKillCmd, err := cli.BuildCobraCommand(killCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "listen-killer",
			ShortHelpSections: []string{schema.DefaultSlug},
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building cobra kill command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraKillCmd)

	// Help system
	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	// Default action: if no args, run "list"
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "list")
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
