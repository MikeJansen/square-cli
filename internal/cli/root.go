// Package cli wires together the root command, global flags, configuration
// resolution, and the generated command tree.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/MikeJansen/square-cli/internal/client"
	"github.com/MikeJansen/square-cli/internal/commands"
	"github.com/MikeJansen/square-cli/internal/config"
	"github.com/MikeJansen/square-cli/internal/output"
	"github.com/spf13/cobra"
)

// Build metadata, overridable via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type globalFlags struct {
	profile     string
	configPath  string
	accessToken string
	environment string
	apiVersion  string
	baseURL     string
	outputFmt   string
	verbose     bool
}

// NewRootCommand constructs the top-level cobra command.
func NewRootCommand() *cobra.Command {
	gf := &globalFlags{}

	root := &cobra.Command{
		Use:   "square-cli",
		Short: "A comprehensive command-line interface for the Square API",
		Long: `square-cli is an exhaustive, scriptable wrapper around the Square Connect API.

Every command mirrors a Square API endpoint one-to-one: its flags are the
endpoint's parameters, and its output is the raw JSON the API returns. This
makes it well suited to shell scripts, automation, and AI assistants.

Authentication resolves from (lowest to highest precedence): the config file
profile, environment variables (SQUARE_ACCESS_TOKEN, SQUARE_ENVIRONMENT,
SQUARE_VERSION), and global flags.

Append --docs to any endpoint command to print a link to its online reference.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return setupRuntime(cmd, gf)
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&gf.profile, "profile", "", "Config profile to use")
	pf.StringVar(&gf.configPath, "config", "", "Path to config file (default: $XDG_CONFIG_HOME/square-cli/config.yaml)")
	pf.StringVar(&gf.accessToken, "access-token", "", "Square access token (overrides env and config)")
	pf.StringVar(&gf.environment, "environment", "", "Square environment: sandbox or production")
	pf.StringVar(&gf.apiVersion, "api-version", "", "Square-Version header to send (e.g. 2026-05-20)")
	pf.StringVar(&gf.baseURL, "base-url", "", "Override the API base URL")
	pf.StringVarP(&gf.outputFmt, "output", "o", "json", "Output format: json, compact, or raw")
	pf.BoolVarP(&gf.verbose, "verbose", "v", false, "Log requests to stderr")

	// Flags consumed directly by generated endpoint commands.
	pf.Bool(commands.FlagDocs, false, "Print the online documentation URL for this endpoint and exit")
	pf.Bool(commands.FlagTyped, false, "Render the response through the typed model (normalized field order)")

	root.SetVersionTemplate("square-cli {{.Version}}\n")

	// Static commands.
	root.AddCommand(newConfigCommand())

	// Generated command tree (all Square API namespaces).
	commands.AddCommands(root)

	return root
}

func setupRuntime(cmd *cobra.Command, gf *globalFlags) error {
	// The config and completion commands do not need a client.
	if cmd.Name() == "help" || cmd.Name() == "config" || cmd.Name() == "completion" {
		return nil
	}
	if p := cmd.Parent(); p != nil && p.Name() == "config" {
		return nil
	}

	cfg, err := config.Resolve(config.Overrides{
		AccessToken: gf.accessToken,
		Environment: gf.environment,
		APIVersion:  gf.apiVersion,
		BaseURL:     gf.baseURL,
		Profile:     gf.profile,
		ConfigPath:  gf.configPath,
	})
	if err != nil {
		return err
	}

	fmtVal, err := output.ParseFormat(gf.outputFmt)
	if err != nil {
		return err
	}

	c := client.New(client.Options{
		BaseURL:     cfg.BaseURL,
		AccessToken: cfg.AccessToken,
		APIVersion:  cfg.APIVersion,
		UserAgent:   "square-cli/" + version,
		Verbose:     gf.verbose,
	})
	if gf.verbose {
		c.SetLogger(func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "[square-cli] "+format+"\n", args...)
		})
	}

	rt := &commands.Runtime{
		Client: c,
		Output: output.Options{Writer: cmd.OutOrStdout(), Format: fmtVal},
	}
	cmd.SetContext(commands.WithRuntime(cmd.Context(), rt))
	return nil
}

// Execute runs the root command against ctx.
func Execute(ctx context.Context) error {
	return NewRootCommand().ExecuteContext(ctx)
}
