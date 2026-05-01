package draftsvc

import (
	"fmt"
	"strings"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// PickChoice is "local" or "remote".
type PickChoice string

const (
	PickLocal  PickChoice = "local"
	PickRemote PickChoice = "remote"
)

// Pick selects which side wins for one (rowID, day) conflict.
type Pick struct {
	RowID  string
	Day    time.Weekday
	Choice PickChoice
}

// Conflict describes one unresolved conflicted cell. Returned by ListConflicts
// for status output and produced for the resolve JSON envelope.
type Conflict struct {
	RowID         string
	RowLabel      string
	Day           time.Weekday
	LocalHours    float64
	LocalSrcID    int
	RemoteHours   float64
	RemoteSrcID   int
	PulledHours   float64
	RemoteDeletes bool // true when remote candidate is "delete" (hours=0, srcID=0)
}

// ResolveResult reports what happened during a Resolve call.
type ResolveResult struct {
	PicksApplied        int
	ConflictsRemaining  int
	PickedLocal         int
	PickedRemote        int
	DroppedDeletedCells int // cells removed because remote intent was delete
}

// ListConflicts returns every conflicted cell in the named draft.
// Returns an empty slice when the draft is unconflicted.
func (s *Service) ListConflicts(profile string, weekStart time.Time, name string) ([]Conflict, error) {
	if name == "" {
		name = "default"
	}
	draft, err := s.store.Load(profile, weekStart, name)
	if err != nil {
		return nil, err
	}
	out := []Conflict{}
	for _, row := range draft.Rows {
		for _, c := range row.Cells {
			if c.Conflict == nil {
				continue
			}
			out = append(out, Conflict{
				RowID:         row.ID,
				RowLabel:      row.Label,
				Day:           c.Day,
				LocalHours:    c.Hours,
				LocalSrcID:    c.SourceEntryID,
				RemoteHours:   c.Conflict.Hours,
				RemoteSrcID:   c.Conflict.SourceEntryID,
				PulledHours:   c.Conflict.PulledHours,
				RemoteDeletes: c.Conflict.Hours == 0 && c.Conflict.SourceEntryID == 0,
			})
		}
	}
	return out, nil
}

// Resolve applies the picks to the named draft. allowDelete is required
// (mirrors push's --allow-deletes) when any --pick remote would drop a cell
// because the remote candidate is "delete on remote." Returns a result
// describing what was applied and how many conflicts remain.
//
// If pickAllLocal or pickAllRemote is set, it overrides the per-cell picks
// list and applies the named choice to every conflicted cell.
//
// Takes an OpPreResolve snapshot before saving.
func (s *Service) Resolve(profile string, weekStart time.Time, name string, pickAllLocal, pickAllRemote bool, picks []Pick, allowDelete bool) (ResolveResult, error) {
	if name == "" {
		name = "default"
	}
	if pickAllLocal && pickAllRemote {
		return ResolveResult{}, fmt.Errorf("pickAllLocal and pickAllRemote are mutually exclusive")
	}

	draft, err := s.store.Load(profile, weekStart, name)
	if err != nil {
		return ResolveResult{}, err
	}

	// Build the effective picks list. Bulk shortcuts win over the slice.
	if pickAllLocal || pickAllRemote {
		choice := PickLocal
		if pickAllRemote {
			choice = PickRemote
		}
		picks = picks[:0]
		for _, row := range draft.Rows {
			for _, c := range row.Cells {
				if c.Conflict == nil {
					continue
				}
				picks = append(picks, Pick{RowID: row.ID, Day: c.Day, Choice: choice})
			}
		}
	}

	if len(picks) == 0 {
		// No picks: count remaining conflicts and return.
		remaining := countConflictsInDraft(draft)
		return ResolveResult{ConflictsRemaining: remaining}, nil
	}

	// Pre-flight: check delete safety.
	if !allowDelete {
		for _, p := range picks {
			if p.Choice != PickRemote {
				continue
			}
			cell := findCell(draft, p.RowID, p.Day)
			if cell == nil || cell.Conflict == nil {
				continue
			}
			if cell.Conflict.Hours == 0 && cell.Conflict.SourceEntryID == 0 {
				return ResolveResult{}, fmt.Errorf("--pick remote on %s/%s would drop the cell (remote intent: delete); pass --yes to confirm",
					p.RowID, p.Day.String())
			}
		}
	}

	result := ResolveResult{}
	applied := map[string]struct{}{} // (rowID,day) → picked

	for _, p := range picks {
		key := pickKey(p.RowID, p.Day)
		if _, dup := applied[key]; dup {
			continue
		}
		ok, dropped := applyPick(&draft, p)
		if !ok {
			continue
		}
		applied[key] = struct{}{}
		result.PicksApplied++
		switch p.Choice {
		case PickLocal:
			result.PickedLocal++
		case PickRemote:
			result.PickedRemote++
			if dropped {
				result.DroppedDeletedCells++
			}
		}
	}

	if result.PicksApplied == 0 {
		// No picks matched any conflicted cell. Don't snapshot or save.
		result.ConflictsRemaining = countConflictsInDraft(draft)
		return result, nil
	}

	// Drop emptied rows (a row whose only cell was a "deleted on remote"
	// pick remote becomes a row with no cells, which is invalid).
	draft.Rows = filterEmptyRows(draft.Rows)
	draft.ModifiedAt = time.Now().UTC()

	if _, err := s.snapshots.Take(draft, OpPreResolve, ""); err != nil {
		return ResolveResult{}, fmt.Errorf("resolve: pre-resolve snapshot: %w", err)
	}
	if err := s.store.Save(draft); err != nil {
		return ResolveResult{}, fmt.Errorf("resolve: save: %w", err)
	}

	result.ConflictsRemaining = countConflictsInDraft(draft)
	return result, nil
}

// applyPick mutates draft in place applying p. Returns ok=true when a cell
// was found and a pick was applied. dropped=true means the cell was removed
// (remote-intent delete + pick remote).
func applyPick(draft *domain.WeekDraft, p Pick) (ok, dropped bool) {
	for ri, row := range draft.Rows {
		if row.ID != p.RowID {
			continue
		}
		for ci, c := range row.Cells {
			if c.Day != p.Day || c.Conflict == nil {
				continue
			}
			switch p.Choice {
			case PickLocal:
				draft.Rows[ri].Cells[ci].Conflict = nil
				return true, false
			case PickRemote:
				if c.Conflict.Hours == 0 && c.Conflict.SourceEntryID == 0 {
					// Drop the cell entirely.
					draft.Rows[ri].Cells = append(row.Cells[:ci], row.Cells[ci+1:]...)
					return true, true
				}
				draft.Rows[ri].Cells[ci].Hours = c.Conflict.Hours
				draft.Rows[ri].Cells[ci].SourceEntryID = c.Conflict.SourceEntryID
				draft.Rows[ri].Cells[ci].Conflict = nil
				return true, false
			}
		}
	}
	return false, false
}

func filterEmptyRows(rows []domain.DraftRow) []domain.DraftRow {
	out := make([]domain.DraftRow, 0, len(rows))
	for _, row := range rows {
		if len(row.Cells) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func countConflictsInDraft(d domain.WeekDraft) int {
	n := 0
	for _, row := range d.Rows {
		for _, c := range row.Cells {
			if c.Conflict != nil {
				n++
			}
		}
	}
	return n
}

func findCell(d domain.WeekDraft, rowID string, day time.Weekday) *domain.DraftCell {
	for ri, row := range d.Rows {
		if row.ID != rowID {
			continue
		}
		for ci, c := range row.Cells {
			if c.Day == day {
				return &d.Rows[ri].Cells[ci]
			}
		}
	}
	return nil
}

func pickKey(rowID string, day time.Weekday) string {
	return rowID + ":" + day.String()
}

// ParsePickChoice accepts "local" / "remote" case-insensitively.
func ParsePickChoice(s string) (PickChoice, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "local":
		return PickLocal, nil
	case "remote":
		return PickRemote, nil
	default:
		return "", fmt.Errorf("--pick must be local|remote, got %q", s)
	}
}

// ParseWeekday accepts case-insensitive weekday names.
func ParseWeekday(s string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sunday":
		return time.Sunday, nil
	case "monday":
		return time.Monday, nil
	case "tuesday":
		return time.Tuesday, nil
	case "wednesday":
		return time.Wednesday, nil
	case "thursday":
		return time.Thursday, nil
	case "friday":
		return time.Friday, nil
	case "saturday":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unknown weekday %q (want sunday..saturday)", s)
	}
}
