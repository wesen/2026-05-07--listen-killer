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
	Short: "Terminal dashboard for managing listening TCP daemons",
	Long: `Listen Killer shows all TCP listening sockets in an interactive table.
You can sort, filter, kill processes, and export data as JSON/YAML/CSV.

Run without arguments to launch the TUI dashboard:
  listen-killer

For CLI/scripting and LLM-agent mode:
  listen-killer list --no-tui --output json
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

	// Register the "list" subcommand (Glazed + Bubbletea)
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
