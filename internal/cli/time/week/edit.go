package week

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	webeditor "github.com/iainmoffat/tdx/internal/web/editor"
)

type editFlags struct {
	profile string
	web     bool
}

func newEditCmd() *cobra.Command {
	var f editFlags
	cmd := &cobra.Command{
		Use:   "edit [date[/name]]",
		Short: "Edit a draft in an interactive grid (defaults to the current week)",
		Long: `Edit a draft's hours in an interactive grid (the same editor used by ` +
			"`tdx time template edit`" + `).

Use --web to open the editor in your browser instead of the terminal.

Only hours within existing rows can be edited. To add or remove rows, use
` + "`tdx time week new --from-template`" + ` or ` + "`tdx time week set`" + `.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) > 0 {
				ref = args[0]
			}
			return runEdit(cmd, f, ref)
		},
	}
	cmd.Flags().BoolVar(&f.web, "web", false, "open the editor in your browser")
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	return cmd
}

func runEdit(cmd *cobra.Command, f editFlags, ref string) error {
	weekStart, name, err := ResolveWeekRef(ref)
	if err != nil {
		return err
	}

	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)
	drafts := draftsvc.NewService(paths, tsvc)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	d, err := drafts.Store().Load(profileName, weekStart, name)
	if err != nil {
		return err
	}

	if f.web {
		return runWebEditor(cmd, drafts, d)
	}
	return runTUIEditor(cmd, drafts, d)
}

func runTUIEditor(cmd *cobra.Command, drafts *draftsvc.Service, d domain.WeekDraft) error {
	sheet := draftToSheet(d)
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

	applySheetToDraft(final.Sheet(), &d)
	d.ModifiedAt = time.Now().UTC()
	if err := drafts.Store().Save(d); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved draft %s/%s.\n",
		d.WeekStart.Format("2006-01-02"), d.Name)
	return nil
}

func runWebEditor(cmd *cobra.Command, drafts *draftsvc.Service, d domain.WeekDraft) error {
	sheet := draftToSheet(d)
	saveFn := func(s editor.Sheet) error {
		applySheetToDraft(s, &d)
		d.ModifiedAt = time.Now().UTC()
		return drafts.Store().Save(d)
	}
	res, err := webeditor.Run(sheet, saveFn)
	if err != nil {
		return fmt.Errorf("web editor: %w", err)
	}
	if res.Saved {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved draft %s/%s.\n",
			d.WeekStart.Format("2006-01-02"), d.Name)
	}
	return nil
}

// draftToSheet builds a Sheet from a draft, with rows in display order
// (group, then label) and dense WeekHours assembled from sparse cells.
func draftToSheet(d domain.WeekDraft) editor.Sheet {
	rows := make([]editor.SheetRow, 0, len(d.Rows))
	for _, r := range d.Rows {
		var h domain.WeekHours
		for _, c := range r.Cells {
			h.SetDay(c.Day, c.Hours)
		}
		rows = append(rows, editor.SheetRow{
			ID:         r.ID,
			Label:      r.Label,
			GroupName:  r.Target.GroupName,
			DisplayRef: r.Target.DisplayRef,
			TypeName:   r.TimeType.Name,
			Hours:      h,
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
	return editor.Sheet{Name: d.Name, Rows: rows}
}

// applySheetToDraft writes hours from sheet rows back into draft cells,
// preserving SourceEntryID and PerCell metadata. Per spec §8:
//   - cell exists, hours unchanged → no-op
//   - cell exists with SourceEntryID, hours=0 → set to 0 (delete-on-push)
//   - cell exists without SourceEntryID, hours=0 → drop the cell
//   - cell exists, hours>0 → update Hours (preserves PerCell)
//   - cell absent, hours=0 → no-op
//   - cell absent, hours>0 → add new {Day, Hours, SourceEntryID=0}
//
// Cells in each row are sorted by Day after the apply.
func applySheetToDraft(sheet editor.Sheet, d *domain.WeekDraft) {
	hoursByID := make(map[string]domain.WeekHours, len(sheet.Rows))
	for _, r := range sheet.Rows {
		hoursByID[r.ID] = r.Hours
	}

	for ri := range d.Rows {
		row := &d.Rows[ri]
		newHours, ok := hoursByID[row.ID]
		if !ok {
			continue
		}

		cellsByDay := make(map[time.Weekday]int, len(row.Cells))
		for ci, c := range row.Cells {
			cellsByDay[c.Day] = ci
		}

		var rebuilt []domain.DraftCell
		for ci := 0; ci < 7; ci++ {
			wd := time.Weekday(ci)
			h := newHours.ForDay(wd)
			idx, exists := cellsByDay[wd]
			switch {
			case exists:
				cell := row.Cells[idx]
				if h == 0 && cell.SourceEntryID == 0 {
					continue // drop local-only cell
				}
				cell.Hours = h
				rebuilt = append(rebuilt, cell)
			case !exists && h > 0:
				rebuilt = append(rebuilt, domain.DraftCell{Day: wd, Hours: h})
			}
		}
		sort.SliceStable(rebuilt, func(i, j int) bool {
			return rebuilt[i].Day < rebuilt[j].Day
		})
		row.Cells = rebuilt
	}
}
