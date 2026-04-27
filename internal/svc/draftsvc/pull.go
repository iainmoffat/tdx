package draftsvc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

type rowGroupKey struct {
	kind     domain.TargetKind
	appID    int
	itemID   int
	taskID   int
	typeID   int
	billable bool
}

// buildDraftFromReport converts a live WeekReport into a WeekDraft.
// Entries with the same (target, timeType, billable) collapse into one row;
// each entry becomes a cell on its weekday.
func buildDraftFromReport(profile, name string, report domain.WeekReport) domain.WeekDraft {
	weekStart := report.WeekRef.StartDate
	groups := map[rowGroupKey]*domain.DraftRow{}
	var order []rowGroupKey

	for _, e := range report.Entries {
		k := rowGroupKey{
			kind: e.Target.Kind, appID: e.Target.AppID, itemID: e.Target.ItemID,
			taskID: e.Target.TaskID, typeID: e.TimeType.ID, billable: e.Billable,
		}
		row, ok := groups[k]
		if !ok {
			row = &domain.DraftRow{
				Target: e.Target, TimeType: e.TimeType, Billable: e.Billable,
				Description: e.Description,
				Label:       displayLabel(e.Target),
				ResolverHints: domain.ResolverHints{
					TargetDisplayName: e.Target.DisplayName, TimeTypeName: e.TimeType.Name,
				},
			}
			groups[k] = row
			order = append(order, k)
		}
		// DST-safe calendar-day arithmetic.
		ey, em, ed := e.Date.Date()
		ry, rm, rd := weekStart.Date()
		entryDay := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
		refDay := time.Date(ry, rm, rd, 0, 0, 0, 0, time.UTC)
		dayIdx := int(entryDay.Sub(refDay).Hours() / 24)
		if dayIdx < 0 || dayIdx >= 7 {
			continue
		}
		wd := weekStart.AddDate(0, 0, dayIdx).Weekday()

		row.Cells = append(row.Cells, domain.DraftCell{
			Day: wd, Hours: float64(e.Minutes) / 60.0, SourceEntryID: e.ID,
		})
	}

	rows := make([]domain.DraftRow, 0, len(order))
	for _, k := range order {
		rows = append(rows, *groups[k])
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return totalHours(rows[i]) > totalHours(rows[j])
	})
	for i := range rows {
		rows[i].ID = fmt.Sprintf("row-%02d", i+1)
		sort.SliceStable(rows[i].Cells, func(a, b int) bool {
			return rows[i].Cells[a].Day < rows[i].Cells[b].Day
		})
	}

	now := time.Now().UTC()
	return domain.WeekDraft{
		SchemaVersion: 1,
		Profile:       profile,
		WeekStart:     weekStart,
		Name:          name,
		Provenance: domain.DraftProvenance{
			Kind:              domain.ProvenancePulled,
			PulledAt:          now,
			RemoteFingerprint: computeRemoteFingerprint(report),
			RemoteStatus:      report.Status,
		},
		CreatedAt:  now,
		ModifiedAt: now,
		Rows:       rows,
	}
}

func totalHours(r domain.DraftRow) float64 {
	var sum float64
	for _, c := range r.Cells {
		sum += c.Hours
	}
	return sum
}

func displayLabel(t domain.Target) string {
	if t.DisplayName != "" {
		return t.DisplayName
	}
	return t.DisplayRef
}

// computeRemoteFingerprint produces a stable hash of the remote week,
// ignoring fields TD touches automatically (CreatedAt, ModifiedAt). Two
// reports with the same canonical entry set produce the same fingerprint
// regardless of entry order.
func computeRemoteFingerprint(r domain.WeekReport) string {
	type fpEntry struct {
		ID           int    `json:"id"`
		Date         string `json:"date"`
		Minutes      int    `json:"minutes"`
		TimeTypeID   int    `json:"timeTypeID"`
		Billable     bool   `json:"billable"`
		TargetKind   string `json:"targetKind"`
		TargetAppID  int    `json:"targetAppID"`
		TargetItemID int    `json:"targetItemID"`
		TargetTaskID int    `json:"targetTaskID"`
		Description  string `json:"description"`
	}
	out := make([]fpEntry, 0, len(r.Entries))
	for _, e := range r.Entries {
		out = append(out, fpEntry{
			ID: e.ID, Date: e.Date.Format("2006-01-02"), Minutes: e.Minutes,
			TimeTypeID: e.TimeType.ID, Billable: e.Billable,
			TargetKind: string(e.Target.Kind), TargetAppID: e.Target.AppID,
			TargetItemID: e.Target.ItemID, TargetTaskID: e.Target.TaskID,
			Description: e.Description,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Date < out[j].Date
	})
	canonical := struct {
		WeekStart string    `json:"weekStart"`
		Status    string    `json:"status"`
		Entries   []fpEntry `json:"entries"`
	}{
		WeekStart: r.WeekRef.StartDate.Format("2006-01-02"),
		Status:    string(r.Status),
		Entries:   out,
	}
	data, _ := json.Marshal(canonical)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
