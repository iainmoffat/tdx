package template

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	webeditor "github.com/iainmoffat/tdx/internal/web/editor"
)

func newEditCmd() *cobra.Command {
	var (
		webFlag     bool
		profileFlag string
	)

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit template hours in an interactive grid",
		Long:  "Edit template hours in an interactive grid.\nUse --web to open the editor in your browser instead of the terminal.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			tmpl, err := store.Load(profile, args[0])
			if err != nil {
				return err
			}

			if webFlag {
				return runWebEditor(cmd, profile, tmpl, store)
			}
			return runTUIEditor(cmd, profile, tmpl, store)
		},
	}

	cmd.Flags().BoolVar(&webFlag, "web", false, "open the editor in your browser")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTUIEditor(cmd *cobra.Command, profile string, tmpl domain.Template, store *tmplsvc.Store) error {
	m := editor.New(tmpl.Name, tmpl.Rows)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	final, _ := result.(editor.Model)
	if !final.Saved() {
		return nil
	}

	tmpl.Rows = final.Rows()
	tmpl.ModifiedAt = time.Now().UTC()
	if err := store.Save(profile, tmpl); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	return nil
}

func runWebEditor(cmd *cobra.Command, profile string, tmpl domain.Template, store *tmplsvc.Store) error {
	saveFn := func(t domain.Template) error {
		return store.Save(profile, t)
	}

	res, err := webeditor.Run(tmpl, saveFn)
	if err != nil {
		return fmt.Errorf("web editor: %w", err)
	}

	if res.Saved {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	}
	return nil
}
