package auth

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage tdx auth profiles",
	}
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileAddCmd())
	cmd.AddCommand(newProfileRemoveCmd())
	cmd.AddCommand(newProfileUseCmd())
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if len(cfg.Profiles) == 0 {
				_, _ = fmt.Fprintln(w, "no profiles configured")
				return nil
			}
			for _, p := range cfg.Profiles {
				marker := "  "
				if p.Name == cfg.DefaultProfile {
					marker = "* "
				}
				_, _ = fmt.Fprintf(w, "%s%s  %s\n", marker, p.Name, p.TenantBaseURL)
			}
			return nil
		},
	}
}

func newProfileAddCmd() *cobra.Command {
	var url string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			p := domain.Profile{Name: args[0], TenantBaseURL: url}
			if err := store.AddProfile(p); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "added profile %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "tenant base URL (e.g. https://yourorg.teamdynamix.com/)")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func newProfileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			if err := store.RemoveProfile(args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed profile %q\n", args[0])
			return nil
		},
	}
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			if err := store.SetDefault(args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default profile set to %q\n", args[0])
			return nil
		},
	}
}

// newProfileStore resolves config paths fresh each call so tests can flip
// TDX_CONFIG_HOME between invocations.
func newProfileStore() (*config.ProfileStore, error) {
	p, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}
	return config.NewProfileStore(p), nil
}
