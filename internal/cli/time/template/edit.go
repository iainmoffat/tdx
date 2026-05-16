package template

import (
	"fmt"
	"sort"
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
			if err := domain.ValidateArtifactName(args[0]); err != nil {
				return err
			}
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
	sheet := templateToSheet(tmpl)
	m := editor.New(sheet)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	final, _ := result.(editor.Model)
	if !final.Saved() {
		return nil
	}

	applySheetToTemplate(final.Sheet(), &tmpl)
	tmpl.ModifiedAt = time.Now().UTC()
	if err := store.Save(profile, tmpl); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	return nil
}

func runWebEditor(cmd *cobra.Command, profile string, tmpl domain.Template, store *tmplsvc.Store) error {
	sheet := templateToSheet(tmpl)

	saveFn := func(s editor.Sheet) error {
		applySheetToTemplate(s, &tmpl)
		tmpl.ModifiedAt = time.Now().UTC()
		return store.Save(profile, tmpl)
	}

	res, err := webeditor.Run(sheet, saveFn)
	if err != nil {
		return fmt.Errorf("web editor: %w", err)
	}

	if res.Saved {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	}
	return nil
}

// templateToSheet builds a Sheet from a template, with rows sorted by
// (GroupName, then Label or DisplayRef) for stable display order.
func templateToSheet(t domain.Template) editor.Sheet {
	rows := make([]editor.SheetRow, 0, len(t.Rows))
	for _, r := range t.Rows {
		rows = append(rows, editor.SheetRow{
			ID:         r.ID,
			Label:      r.Label,
			GroupName:  r.Target.GroupName,
			DisplayRef: r.Target.DisplayRef,
			TypeName:   r.TimeType.Name,
			Hours:      r.Hours,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].GroupName != rows[j].GroupName {
			return rows[i].GroupName < rows[j].GroupName
		}
		li := rows[i].Label
		if li == "" {
			li = rows[i].DisplayRef
		}
		lj := rows[j].Label
		if lj == "" {
			lj = rows[j].DisplayRef
		}
		return li < lj
	})
	return editor.Sheet{Name: t.Name, Rows: rows}
}

// applySheetToTemplate copies hours from sheet rows back into t.Rows
// (matched by row ID). Mutates t in place. Rows in t that aren't in the
// sheet are untouched (defensive — shouldn't happen since the editor can't
// drop rows).
func applySheetToTemplate(sheet editor.Sheet, t *domain.Template) {
	hoursByID := make(map[string]domain.WeekHours, len(sheet.Rows))
	for _, r := range sheet.Rows {
		hoursByID[r.ID] = r.Hours
	}
	for i := range t.Rows {
		if h, ok := hoursByID[t.Rows[i].ID]; ok {
			t.Rows[i].Hours = h
		}
	}
}
