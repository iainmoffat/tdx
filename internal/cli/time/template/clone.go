package template

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

func newCloneCmd() *cobra.Command {
	var profileFlag string

	cmd := &cobra.Command{
		Use:   "clone <source> <dest>",
		Short: "Clone a template under a new name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)

			tmpl, err := store.Load(profile, src)
			if err != nil {
				return err
			}
			if store.Exists(profile, dst) {
				return fmt.Errorf("template %q already exists", dst)
			}

			now := time.Now().UTC().Truncate(time.Second)
			tmpl.Name = dst
			tmpl.DerivedFrom = nil
			tmpl.CreatedAt = now
			tmpl.ModifiedAt = now

			if err := store.Save(profile, tmpl); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cloned %q → %q\n", src, dst)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}
