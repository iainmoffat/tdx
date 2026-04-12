package template

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
)

func newCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <source> <dest>",
		Short: "Clone a template under a new name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)

			tmpl, err := store.Load(src)
			if err != nil {
				return err
			}
			if store.Exists(dst) {
				return fmt.Errorf("template %q already exists", dst)
			}

			now := time.Now().UTC().Truncate(time.Second)
			tmpl.Name = dst
			tmpl.DerivedFrom = nil
			tmpl.CreatedAt = now
			tmpl.ModifiedAt = now

			if err := store.Save(tmpl); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cloned %q → %q\n", src, dst)
			return nil
		},
	}
}
