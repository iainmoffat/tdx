package config

import (
	"fmt"

	"github.com/ipm/tdx/internal/config"
	"github.com/spf13/cobra"
)

// NewCmd returns the `tdx config` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and initialise tdx configuration",
	}
	cmd.AddCommand(newPathCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newInitCmd())
	return cmd
}

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print resolved tdx config paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(w, "root:         %s\n", p.Root)
			_, _ = fmt.Fprintf(w, "config:       %s\n", p.ConfigFile)
			_, _ = fmt.Fprintf(w, "credentials:  %s\n", p.CredentialsFile)
			_, _ = fmt.Fprintf(w, "templates:    %s\n", p.TemplatesDir)
			return nil
		},
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the tdx config directory if it does not exist",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			if err := p.EnsureRoot(); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "initialised %s\n", p.Root)
			return nil
		},
	}
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current profile configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			store := config.NewProfileStore(p)
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if len(cfg.Profiles) == 0 {
				_, _ = fmt.Fprintln(w, "no profiles configured")
				_, _ = fmt.Fprintln(w, "run 'tdx auth login' to create one")
				return nil
			}
			_, _ = fmt.Fprintf(w, "default profile: %s\n", cfg.DefaultProfile)
			_, _ = fmt.Fprintln(w, "profiles:")
			for _, prof := range cfg.Profiles {
				_, _ = fmt.Fprintf(w, "  - %s  %s\n", prof.Name, prof.TenantBaseURL)
			}
			return nil
		},
	}
}
