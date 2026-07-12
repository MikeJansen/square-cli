package cli

import (
	"fmt"
	"sort"

	"github.com/MikeJansen/square-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect square-cli configuration",
		Long:  "Commands for inspecting the resolved configuration and available profiles.",
	}
	cmd.AddCommand(newConfigPathCommand())
	cmd.AddCommand(newConfigProfilesCommand())
	cmd.AddCommand(newConfigCurrentCommand())
	return cmd
}

func newConfigPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path in use",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("config")
			if path == "" {
				path = config.DefaultConfigPath()
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
}

func newConfigProfilesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List profiles defined in the config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("config")
			if path == "" {
				path = config.DefaultConfigPath()
			}
			file, err := config.LoadFile(path)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(file.Profiles))
			for name := range file.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				marker := "  "
				if name == file.CurrentProfile {
					marker = "* "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", marker, name)
			}
			return nil
		},
	}
}

func newConfigCurrentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the resolved configuration (access token redacted)",
		RunE: func(cmd *cobra.Command, args []string) error {
			gf := collectGlobalFlags(cmd)
			cfg, err := config.Resolve(gf)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "profile:      %s\n", cfg.Profile)
			fmt.Fprintf(out, "environment:  %s\n", cfg.Environment)
			fmt.Fprintf(out, "base_url:     %s\n", cfg.BaseURL)
			if cfg.APIVersion != "" {
				fmt.Fprintf(out, "api_version:  %s\n", cfg.APIVersion)
			} else {
				fmt.Fprintf(out, "api_version:  (server default)\n")
			}
			fmt.Fprintf(out, "access_token: %s\n", redact(cfg.AccessToken))
			return nil
		},
	}
}

func collectGlobalFlags(cmd *cobra.Command) config.Overrides {
	get := func(name string) string {
		v, _ := cmd.Flags().GetString(name)
		return v
	}
	return config.Overrides{
		AccessToken: get("access-token"),
		Environment: get("environment"),
		APIVersion:  get("api-version"),
		BaseURL:     get("base-url"),
		Profile:     get("profile"),
		ConfigPath:  get("config"),
	}
}

func redact(token string) string {
	switch {
	case token == "":
		return "(not set)"
	case len(token) <= 8:
		return "****"
	default:
		return token[:4] + "…" + token[len(token)-4:]
	}
}
