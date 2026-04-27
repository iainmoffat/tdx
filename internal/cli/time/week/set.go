package week

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

var dayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

type cellWrite struct {
	RowID string
	Day   time.Weekday
	Hours float64
}

func parseCellWrite(s string) (cellWrite, error) {
	colon := strings.Index(s, ":")
	if colon < 0 {
		return cellWrite{}, fmt.Errorf("expected row:day=hours, got %q", s)
	}
	eq := strings.Index(s, "=")
	if eq < 0 || eq < colon {
		return cellWrite{}, fmt.Errorf("expected row:day=hours, got %q", s)
	}
	rowID := strings.TrimSpace(s[:colon])
	dayStr := strings.TrimSpace(s[colon+1 : eq])
	hoursStr := strings.TrimSpace(s[eq+1:])
	day, ok := dayNames[strings.ToLower(dayStr)]
	if !ok {
		return cellWrite{}, fmt.Errorf("unknown day %q (use sun, mon, tue, wed, thu, fri, sat)", dayStr)
	}
	h, err := strconv.ParseFloat(hoursStr, 64)
	if err != nil {
		return cellWrite{}, fmt.Errorf("invalid hours %q: %w", hoursStr, err)
	}
	return cellWrite{RowID: rowID, Day: day, Hours: h}, nil
}

type setFlags struct {
	profile string
}

func newSetCmd() *cobra.Command {
	var f setFlags
	cmd := &cobra.Command{
		Use:   "set <date>[/<name>] <row>:<day>=<hours> ...",
		Short: "Non-interactive cell write (repeatable)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSet(cmd, f, args)
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	return cmd
}

func runSet(cmd *cobra.Command, f setFlags, args []string) error {
	weekStart, name, err := ParseDraftRef(args[0])
	if err != nil {
		return err
	}

	writes := make([]cellWrite, 0, len(args)-1)
	for _, tok := range args[1:] {
		w, err := parseCellWrite(tok)
		if err != nil {
			return err
		}
		writes = append(writes, w)
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

	for _, w := range writes {
		if !applyCellWrite(&d, w) {
			return fmt.Errorf("row %q not found in draft (Phase B will add `add-row` for creating rows)", w.RowID)
		}
	}
	d.ModifiedAt = time.Now().UTC()
	if err := drafts.Store().Save(d); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %d cells in draft %s/%s.\n",
		len(writes), weekStart.Format("2006-01-02"), name)
	return nil
}

// applyCellWrite mutates d to set the named row's day cell to hours.
// Returns true on success; false if the row was not found.
func applyCellWrite(d *domain.WeekDraft, w cellWrite) bool {
	for ri := range d.Rows {
		if d.Rows[ri].ID != w.RowID {
			continue
		}
		for ci := range d.Rows[ri].Cells {
			if d.Rows[ri].Cells[ci].Day == w.Day {
				d.Rows[ri].Cells[ci].Hours = w.Hours
				return true
			}
		}
		d.Rows[ri].Cells = append(d.Rows[ri].Cells, domain.DraftCell{
			Day:   w.Day,
			Hours: w.Hours,
		})
		return true
	}
	return false
}
