package template

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
	"github.com/ipm/tdx/internal/tui/editor"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit template hours in an interactive grid",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)
			tmpl, err := store.Load(args[0])
			if err != nil {
				return err
			}

			m := editor.New(tmpl.Name, tmpl.Rows)
			p := tea.NewProgram(m, tea.WithAltScreen())
			result, err := p.Run()
			if err != nil {
				return fmt.Errorf("editor: %w", err)
			}

			final := result.(editor.Model)
			if !final.Saved() {
				return nil
			}

			tmpl.Rows = final.Rows()
			tmpl.ModifiedAt = time.Now().UTC()
			if err := store.Save(tmpl); err != nil {
				return fmt.Errorf("save: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
			return nil
		},
	}
}
