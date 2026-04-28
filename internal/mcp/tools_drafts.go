package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
)

type listDraftsArgs struct {
	Profile    string `json:"profile,omitempty"`
	Dirty      bool   `json:"dirty,omitempty"`
	Conflicted bool   `json:"conflicted,omitempty"`
	WeekStart  string `json:"weekStart,omitempty" jsonschema:"YYYY-MM-DD filter"`
	Archived   bool   `json:"archived,omitempty" jsonschema:"include archived drafts (default false)"`
}

type getDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart" jsonschema:"YYYY-MM-DD any day in target week"`
	Name      string `json:"name,omitempty" jsonschema:"draft name (default: default)"`
}

type previewDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
}

type diffDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
}

// RegisterDraftTools registers read-only week-draft tools.
// Mutating tools are registered separately by RegisterDraftMutatingTools (Task 25).
func RegisterDraftTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_week_drafts",
		Description: "List local week drafts with sync state. Read-only.",
	}, listDraftsHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_week_draft",
		Description: "Load a single draft. Returns full content plus sync state and remote fingerprint. Read-only.",
	}, getDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "preview_push_week_draft",
		Description: `Preview what push_week_draft will do. Returns actions, blockers, and an expectedDiffHash.

Always call this before push_week_draft. The diffHash is required by push_week_draft for race protection — if remote changes between preview and push, the hash will not match and push will refuse.`,
	}, previewDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "diff_week_draft",
		Description: "Diff a draft vs current remote week. Cell-level. Read-only. (MVP: --against remote only.)",
	}, diffDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_week_draft_snapshots",
		Description: "List snapshots for a draft. Read-only.",
	}, listSnapshotsHandler(svcs))
}

func listDraftsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listDraftsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args listDraftsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		list, err := svcs.Drafts.Store().List(profile)
		if err != nil {
			return errorResult(fmt.Sprintf("list drafts: %v", err)), nil, nil
		}

		type item struct {
			WeekStart  string                `json:"weekStart"`
			Name       string                `json:"name"`
			Profile    string                `json:"profile"`
			SyncState  string                `json:"syncState"`
			SyncDetail domain.DraftSyncState `json:"syncDetail"`
			TotalHours float64               `json:"totalHours"`
			PulledAt   string                `json:"pulledAt,omitempty"`
			Archived   bool                  `json:"archived,omitempty"`
		}
		items := make([]item, 0, len(list))
		for _, d := range list {
			if args.WeekStart != "" && d.WeekStart.Format("2006-01-02") != args.WeekStart {
				continue
			}
			pulled, _ := svcs.Drafts.PulledCellsByKey(profile, d.WeekStart, d.Name)
			fp := svcs.Drafts.ProbeRemoteFingerprint(ctx, profile, d.WeekStart)
			state := domain.ComputeSyncState(d, pulled, fp)
			if args.Dirty && state.Sync != domain.SyncDirty {
				continue
			}
			if args.Conflicted && state.Sync != domain.SyncConflicted {
				continue
			}
			it := item{
				WeekStart:  d.WeekStart.Format("2006-01-02"),
				Name:       d.Name,
				Profile:    d.Profile,
				SyncState:  string(state.Sync),
				SyncDetail: state,
				TotalHours: state.TotalHours,
				Archived:   d.Archived,
			}
			if !d.Provenance.PulledAt.IsZero() {
				it.PulledAt = d.Provenance.PulledAt.UTC().Format(time.RFC3339)
			}
			items = append(items, it)
		}

		if !args.Archived {
			out := make([]item, 0, len(items))
			for _, it := range items {
				if !it.Archived {
					out = append(out, it)
				}
			}
			items = out
		}

		return jsonResult(struct {
			Schema string `json:"schema"`
			Drafts []item `json:"drafts"`
		}{Schema: "tdx.v1.weekDraftList", Drafts: items})
	}
}

func getDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args getDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := time.ParseInLocation("2006-01-02", args.WeekStart, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		weekStart = domain.WeekRefContaining(weekStart).StartDate
		name := args.Name
		if name == "" {
			name = "default"
		}

		d, err := svcs.Drafts.Store().Load(profile, weekStart, name)
		if err != nil {
			return errorResult(fmt.Sprintf("load draft: %v", err)), nil, nil
		}
		pulled, _ := svcs.Drafts.PulledCellsByKey(profile, weekStart, name)
		fp := svcs.Drafts.ProbeRemoteFingerprint(ctx, profile, weekStart)
		state := domain.ComputeSyncState(d, pulled, fp)

		return jsonResult(struct {
			Schema                   string                `json:"schema"`
			Draft                    domain.WeekDraft      `json:"draft"`
			SyncState                string                `json:"syncState"`
			SyncDetail               domain.DraftSyncState `json:"syncDetail"`
			CurrentRemoteFingerprint string                `json:"currentRemoteFingerprint,omitempty"`
		}{
			Schema:                   "tdx.v1.weekDraft",
			Draft:                    d,
			SyncState:                string(state.Sync),
			SyncDetail:               state,
			CurrentRemoteFingerprint: fp,
		})
	}
}

func previewDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, previewDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args previewDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := time.ParseInLocation("2006-01-02", args.WeekStart, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		weekStart = domain.WeekRefContaining(weekStart).StartDate
		name := args.Name
		if name == "" {
			name = "default"
		}

		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("auth: %v", err)), nil, nil
		}

		_, diff, err := svcs.Drafts.Reconcile(ctx, profile, weekStart, name, user.UID)
		if err != nil {
			return errorResult(fmt.Sprintf("reconcile: %v", err)), nil, nil
		}
		creates, updates, deletes, skips := diff.CountByKindV2()

		return jsonResult(struct {
			Schema           string           `json:"schema"`
			Actions          []domain.Action  `json:"actions"`
			Blockers         []domain.Blocker `json:"blockers"`
			Creates          int              `json:"creates"`
			Updates          int              `json:"updates"`
			Deletes          int              `json:"deletes"`
			Skips            int              `json:"skips"`
			BlockedCount     int              `json:"blockedCount"`
			ExpectedDiffHash string           `json:"expectedDiffHash"`
		}{
			Schema:           "tdx.v1.weekDraftPreview",
			Actions:          diff.Actions,
			Blockers:         diff.Blockers,
			Creates:          creates,
			Updates:          updates,
			Deletes:          deletes,
			Skips:            skips,
			BlockedCount:     len(diff.Blockers),
			ExpectedDiffHash: diff.DiffHash,
		})
	}
}

func diffDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, diffDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args diffDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := time.ParseInLocation("2006-01-02", args.WeekStart, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		weekStart = domain.WeekRefContaining(weekStart).StartDate
		name := args.Name
		if name == "" {
			name = "default"
		}

		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("auth: %v", err)), nil, nil
		}

		_, diff, err := svcs.Drafts.Reconcile(ctx, profile, weekStart, name, user.UID)
		if err != nil {
			return errorResult(fmt.Sprintf("reconcile: %v", err)), nil, nil
		}

		type entry struct {
			Row      string  `json:"row"`
			Day      string  `json:"day"`
			Kind     string  `json:"kind"`
			After    float64 `json:"after"`
			SourceID int     `json:"sourceID,omitempty"`
		}
		entries := make([]entry, 0, len(diff.Actions))
		for _, a := range diff.Actions {
			e := entry{Row: a.RowID, Day: a.Date.Weekday().String()}
			switch a.Kind {
			case domain.ActionCreate:
				e.Kind, e.After = "add", float64(a.Entry.Minutes)/60.0
			case domain.ActionUpdate:
				e.Kind, e.SourceID = "update", a.ExistingID
				if a.Patch.Minutes != nil {
					e.After = float64(*a.Patch.Minutes) / 60.0
				}
			case domain.ActionDelete:
				e.Kind, e.SourceID = "delete", a.DeleteEntryID
			case domain.ActionSkip:
				e.Kind, e.SourceID = "match", a.ExistingID
			}
			entries = append(entries, e)
		}

		return jsonResult(struct {
			Schema  string  `json:"schema"`
			Entries []entry `json:"entries"`
		}{Schema: "tdx.v1.weekDraftDiff", Entries: entries})
	}
}

// ---------------------------------------------------------------------------
// Mutating tools (Task 25)
// ---------------------------------------------------------------------------

type createDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
	From      string `json:"from,omitempty" jsonschema:"blank | template:<n> | draft:<date>[/<n>]"`
	ShiftDays int    `json:"shiftDays,omitempty"`
	Confirm   bool   `json:"confirm"`
}

type pullDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart" jsonschema:"YYYY-MM-DD any day in target week"`
	Name      string `json:"name,omitempty"`
	Force     bool   `json:"force,omitempty" jsonschema:"overwrite a dirty draft (auto-snapshots first)"`
	Confirm   bool   `json:"confirm" jsonschema:"must be true to execute"`
}

type updateDraftEdit struct {
	RowID       string  `json:"rowID"`
	Day         string  `json:"day" jsonschema:"sun|mon|tue|wed|thu|fri|sat"`
	Hours       float64 `json:"hours" jsonschema:"0 with sourceEntryID = delete on push"`
	Description string  `json:"description,omitempty"`
}

type updateDraftArgs struct {
	Profile            string            `json:"profile,omitempty"`
	WeekStart          string            `json:"weekStart"`
	Name               string            `json:"name,omitempty"`
	Edits              []updateDraftEdit `json:"edits"`
	ExpectedModifiedAt string            `json:"expectedModifiedAt,omitempty" jsonschema:"RFC3339 ModifiedAt from prior get_week_draft (optimistic concurrency)"`
	Confirm            bool              `json:"confirm"`
}

type deleteDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
	Confirm   bool   `json:"confirm"`
}

type pushDraftArgs struct {
	Profile          string `json:"profile,omitempty"`
	WeekStart        string `json:"weekStart"`
	Name             string `json:"name,omitempty"`
	ExpectedDiffHash string `json:"expectedDiffHash" jsonschema:"hash from preview_push_week_draft"`
	AllowDeletes     bool   `json:"allowDeletes,omitempty" jsonschema:"required true if preview contains any delete actions"`
	Confirm          bool   `json:"confirm"`
}

// RegisterDraftMutatingTools registers the mutating week-draft tools.
// All require confirm=true; push additionally requires expectedDiffHash.
func RegisterDraftMutatingTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "create_week_draft",
		Description: `Create a new week draft. From values:
  blank             - empty draft
  template:<name>   - seed rows from a template
  draft:<date>      - clone from another draft (default name)
  draft:<date>/<n>  - clone from a specifically-named draft

Optional shiftDays adjusts the source's WeekStart when from=draft:<...>.
Requires confirm=true. Refuses to overwrite an existing draft at the same
(profile, weekStart, name).`,
	}, createDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "pull_week_draft",
		Description: "Pull a live TD week into a local draft. Refuses to overwrite a dirty draft unless force=true (auto-snapshots first). Requires confirm=true.",
	}, pullDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "update_week_draft",
		Description: `Apply per-cell edits to a draft. Requires confirm=true.

To delete a pulled entry: set hours: 0 on a cell that already has sourceEntryID. Push will then issue an ActionDelete.
To add a new cell on an existing row: include {rowID, day, hours} for a row that does not yet have a cell on that day.

For multi-turn editing: cache modifiedAt from get_week_draft and pass it as expectedModifiedAt to detect concurrent edits.`,
	}, updateDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "delete_week_draft",
		Description: "Delete a local draft. Auto-snapshots first. Requires confirm=true.",
	}, deleteDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "push_week_draft",
		Description: `Push a draft to TD. Requires confirm=true and expectedDiffHash from preview_push_week_draft.

Recipe:
  1. preview_push_week_draft -> capture diffHash and check whether actions include any deletes.
  2. If deletes are present, set allowDeletes=true and ideally surface them to the user before confirming.
  3. push_week_draft -> on hash mismatch, do NOT retry the same hash. Call diff_week_draft and re-preview.`,
	}, pushDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "copy_week_draft",
		Description: "Clone a draft from src to dst. Cells are dimensionless so cross-week copies work without rewrites. Requires confirm=true. Refuses if dst already exists.",
	}, copyDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "rename_week_draft",
		Description: "Rename a draft (preserves snapshot history). Auto-snapshots before any file motion. Requires confirm=true.",
	}, renameDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "reset_week_draft",
		Description: "Discard local edits and re-pull from TD. Auto-snapshots first. Requires confirm=true.",
	}, resetDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "archive_week_draft",
		Description: "Hide a draft from default `list_week_drafts` output. Soft-archive via the `archived: true` flag — fully reversible via `unarchive_week_draft`. Requires confirm=true.",
	}, archiveDraftHandler(svcs, true))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "unarchive_week_draft",
		Description: "Show a previously archived draft in default `list_week_drafts` output. Requires confirm=true.",
	}, archiveDraftHandler(svcs, false))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "snapshot_week_draft",
		Description: "Take a manual snapshot of a draft. Optional `keep=true` pins it (exempt from auto-prune). Requires confirm=true.",
	}, snapshotDraftHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "restore_week_draft_snapshot",
		Description: "Restore a draft from a prior snapshot by sequence number. Auto-snapshots the current state as pre-restore first. Requires confirm=true.",
	}, restoreSnapshotHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "prune_week_draft_snapshots",
		Description: "Drop unpinned snapshots. olderThanDays>0 drops by age; 0 prunes to the retention cap. Requires confirm=true.",
	}, pruneSnapshotsHandler(svcs))
}

var dayNamesMCP = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

func parseWeekStart(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, domain.EasternTZ)
	if err != nil {
		return time.Time{}, err
	}
	return domain.WeekRefContaining(t).StartDate, nil
}

func pullDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, pullDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args pullDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to pull (creates or refreshes a local draft)."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}

		d, err := svcs.Drafts.Pull(ctx, profile, weekStart, name, args.Force)
		if err != nil {
			return errorResult(fmt.Sprintf("pull: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema string           `json:"schema"`
			Draft  domain.WeekDraft `json:"draft"`
		}{Schema: "tdx.v1.weekDraftPullResult", Draft: d})
	}
}

func updateDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args updateDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to update the draft."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}

		d, err := svcs.Drafts.Store().Load(profile, weekStart, name)
		if err != nil {
			return errorResult(fmt.Sprintf("load draft: %v", err)), nil, nil
		}

		// Optimistic concurrency check.
		if args.ExpectedModifiedAt != "" {
			actual := d.ModifiedAt.UTC().Format(time.RFC3339)
			wantT, e1 := time.Parse(time.RFC3339, args.ExpectedModifiedAt)
			gotT, e2 := time.Parse(time.RFC3339, actual)
			if e1 == nil && e2 == nil && !wantT.Equal(gotT) {
				return errorResult(fmt.Sprintf(
					"draft was modified since you read it (expected %s, got %s); call get_week_draft and retry",
					args.ExpectedModifiedAt, actual)), nil, nil
			}
		}

		// Apply edits.
		for _, e := range args.Edits {
			day, ok := dayNamesMCP[e.Day]
			if !ok {
				return errorResult(fmt.Sprintf("invalid day %q (use sun|mon|tue|wed|thu|fri|sat)", e.Day)), nil, nil
			}
			if !applyMCPEdit(&d, e.RowID, day, e.Hours, e.Description) {
				return errorResult(fmt.Sprintf("row %q not found in draft", e.RowID)), nil, nil
			}
		}
		d.ModifiedAt = time.Now().UTC()
		if err := svcs.Drafts.Store().Save(d); err != nil {
			return errorResult(fmt.Sprintf("save: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema string           `json:"schema"`
			Draft  domain.WeekDraft `json:"draft"`
		}{Schema: "tdx.v1.weekDraft", Draft: d})
	}
}

func applyMCPEdit(d *domain.WeekDraft, rowID string, day time.Weekday, hours float64, description string) bool {
	for ri := range d.Rows {
		if d.Rows[ri].ID != rowID {
			continue
		}
		if description != "" {
			d.Rows[ri].Description = description
		}
		for ci := range d.Rows[ri].Cells {
			if d.Rows[ri].Cells[ci].Day == day {
				d.Rows[ri].Cells[ci].Hours = hours
				return true
			}
		}
		d.Rows[ri].Cells = append(d.Rows[ri].Cells, domain.DraftCell{Day: day, Hours: hours})
		return true
	}
	return false
}

func deleteDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, deleteDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args deleteDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to delete the draft (auto-snapshots first)."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}

		d, err := svcs.Drafts.Store().Load(profile, weekStart, name)
		if err != nil {
			return errorResult(fmt.Sprintf("load draft: %v", err)), nil, nil
		}
		if _, err := svcs.Drafts.Snapshots().Take(d, draftsvc.OpPreDelete, ""); err != nil {
			return errorResult(fmt.Sprintf("auto-snapshot: %v", err)), nil, nil
		}
		if err := svcs.Drafts.Store().Delete(profile, weekStart, name); err != nil {
			return errorResult(fmt.Sprintf("delete: %v", err)), nil, nil
		}
		return textResult(fmt.Sprintf("Deleted draft %s/%s.", weekStart.Format("2006-01-02"), name))
	}
}

func pushDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, pushDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args pushDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Call preview_push_week_draft first, then set confirm=true and pass the expectedDiffHash."); !ok {
			return r, nil, nil
		}
		if args.ExpectedDiffHash == "" {
			return errorResult("expectedDiffHash is required (call preview_push_week_draft first)"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}

		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("auth: %v", err)), nil, nil
		}

		res, err := svcs.Drafts.Apply(ctx, profile, weekStart, name, args.ExpectedDiffHash, args.AllowDeletes, user.UID)
		if err != nil {
			if strings.Contains(err.Error(), "hash mismatch") {
				return errorResult("week changed since preview (hash mismatch). Call preview_push_week_draft again to get an updated diffHash."), nil, nil
			}
			if strings.Contains(err.Error(), "delete actions") {
				return errorResult("draft contains delete actions; pass allowDeletes=true to confirm."), nil, nil
			}
			return errorResult(fmt.Sprintf("apply: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema  string `json:"schema"`
			Created int    `json:"created"`
			Updated int    `json:"updated"`
			Deleted int    `json:"deleted"`
			Skipped int    `json:"skipped"`
			Failed  any    `json:"failed,omitempty"`
		}{
			Schema:  "tdx.v1.weekDraftPushResult",
			Created: res.Created, Updated: res.Updated, Deleted: res.Deleted, Skipped: res.Skipped,
			Failed: res.Failed,
		})
	}
}

func createDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, createDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args createDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to create the draft."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}

		var draft domain.WeekDraft
		switch {
		case args.From == "" || args.From == "blank":
			draft, err = svcs.Drafts.NewBlank(profile, weekStart, name)
		case strings.HasPrefix(args.From, "template:"):
			tname := strings.TrimPrefix(args.From, "template:")
			tmpl, terr := svcs.Template.Store().Load(profile, tname)
			if terr != nil {
				return errorResult(fmt.Sprintf("load template: %v", terr)), nil, nil
			}
			draft, err = svcs.Drafts.NewFromTemplate(profile, weekStart, name, tmpl)
		case strings.HasPrefix(args.From, "draft:"):
			ref := strings.TrimPrefix(args.From, "draft:")
			srcDate, srcName, perr := parseDraftRefMCP(ref)
			if perr != nil {
				return errorResult(fmt.Sprintf("from: %v", perr)), nil, nil
			}
			if args.ShiftDays != 0 {
				srcDate = srcDate.AddDate(0, 0, -args.ShiftDays)
			}
			draft, err = svcs.Drafts.NewFromDraft(profile, weekStart, name, profile, srcDate, srcName)
		default:
			return errorResult(fmt.Sprintf("unknown from value: %q", args.From)), nil, nil
		}
		if err != nil {
			return errorResult(fmt.Sprintf("create: %v", err)), nil, nil
		}

		return jsonResult(struct {
			Schema string           `json:"schema"`
			Draft  domain.WeekDraft `json:"draft"`
		}{Schema: "tdx.v1.weekDraftCreateResult", Draft: draft})
	}
}

type copyDraftArgs struct {
	Profile string `json:"profile,omitempty"`
	Src     string `json:"src" jsonschema:"<date>[/<name>]"`
	Dst     string `json:"dst" jsonschema:"<date>[/<name>]"`
	Confirm bool   `json:"confirm"`
}

type renameDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	OldName   string `json:"oldName"`
	NewName   string `json:"newName"`
	Confirm   bool   `json:"confirm"`
}

type resetDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
	Confirm   bool   `json:"confirm"`
}

type archiveDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
	Confirm   bool   `json:"confirm"`
}

func copyDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, copyDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args copyDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to copy the draft."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		srcDate, srcName, err := parseDraftRefMCP(args.Src)
		if err != nil {
			return errorResult(fmt.Sprintf("src: %v", err)), nil, nil
		}
		dstDate, dstName, err := parseDraftRefMCP(args.Dst)
		if err != nil {
			return errorResult(fmt.Sprintf("dst: %v", err)), nil, nil
		}
		d, err := svcs.Drafts.Copy(profile, srcDate, srcName, profile, dstDate, dstName)
		if err != nil {
			return errorResult(fmt.Sprintf("copy: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema string           `json:"schema"`
			Draft  domain.WeekDraft `json:"draft"`
		}{Schema: "tdx.v1.weekDraftCopyResult", Draft: d})
	}
}

func renameDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, renameDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args renameDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to rename the draft."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		if args.OldName == "" || args.NewName == "" {
			return errorResult("oldName and newName are required"), nil, nil
		}
		if err := svcs.Drafts.Rename(profile, weekStart, args.OldName, args.NewName); err != nil {
			return errorResult(fmt.Sprintf("rename: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema    string `json:"schema"`
			WeekStart string `json:"weekStart"`
			OldName   string `json:"oldName"`
			NewName   string `json:"newName"`
		}{
			Schema:    "tdx.v1.weekDraftRenameResult",
			WeekStart: weekStart.Format("2006-01-02"),
			OldName:   args.OldName, NewName: args.NewName,
		})
	}
}

func resetDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, resetDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args resetDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to reset the draft (discard local edits)."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}
		if err := svcs.Drafts.Reset(ctx, profile, weekStart, name); err != nil {
			return errorResult(fmt.Sprintf("reset: %v", err)), nil, nil
		}
		return textResult(fmt.Sprintf("Reset draft %s/%s.", weekStart.Format("2006-01-02"), name))
	}
}

// parseDraftRefMCP duplicates the cli/week ParseDraftRef without that import.
func parseDraftRefMCP(s string) (time.Time, string, error) {
	var dateStr, name string
	if i := strings.IndexByte(s, '/'); i >= 0 {
		dateStr, name = s[:i], s[i+1:]
		if name == "" {
			return time.Time{}, "", fmt.Errorf("empty name after slash")
		}
	} else {
		dateStr, name = s, "default"
	}
	d, err := time.ParseInLocation("2006-01-02", dateStr, domain.EasternTZ)
	if err != nil {
		return time.Time{}, "", err
	}
	return domain.WeekRefContaining(d).StartDate, name, nil
}

func archiveDraftHandler(svcs Services, archive bool) func(context.Context, *sdkmcp.CallToolRequest, archiveDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args archiveDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		verb := "archive"
		if !archive {
			verb = "unarchive"
		}
		if r, ok := confirmGate(args.Confirm, fmt.Sprintf("Set confirm=true to %s the draft.", verb)); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}
		if err := svcs.Drafts.SetArchived(profile, weekStart, name, archive); err != nil {
			return errorResult(fmt.Sprintf("%s: %v", verb, err)), nil, nil
		}
		return jsonResult(struct {
			Schema    string `json:"schema"`
			WeekStart string `json:"weekStart"`
			Name      string `json:"name"`
			Archived  bool   `json:"archived"`
		}{
			Schema:    "tdx.v1.weekDraftArchiveResult",
			WeekStart: weekStart.Format("2006-01-02"),
			Name:      name,
			Archived:  archive,
		})
	}
}

// ---------------------------------------------------------------------------
// Snapshot tools (Task 22)
// ---------------------------------------------------------------------------

type snapshotDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
	Keep      bool   `json:"keep,omitempty" jsonschema:"pin (exempt from auto-prune)"`
	Note      string `json:"note,omitempty"`
	Confirm   bool   `json:"confirm"`
}

type restoreSnapshotArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
	Sequence  int    `json:"sequence" jsonschema:"snapshot sequence number from list_week_draft_snapshots"`
	Confirm   bool   `json:"confirm"`
}

type pruneSnapshotsArgs struct {
	Profile       string `json:"profile,omitempty"`
	WeekStart     string `json:"weekStart"`
	Name          string `json:"name,omitempty"`
	OlderThanDays int    `json:"olderThanDays,omitempty" jsonschema:"prune snapshots older than N days; 0 = use retention cap"`
	Confirm       bool   `json:"confirm"`
}

type listSnapshotsArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
}

func listSnapshotsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listSnapshotsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args listSnapshotsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}
		list, err := svcs.Drafts.Snapshots().List(profile, weekStart, name)
		if err != nil {
			return errorResult(fmt.Sprintf("list snapshots: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema    string                  `json:"schema"`
			Snapshots []draftsvc.SnapshotInfo `json:"snapshots"`
		}{Schema: "tdx.v1.weekDraftSnapshotList", Snapshots: list})
	}
}

func snapshotDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, snapshotDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args snapshotDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to take a snapshot."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}
		d, err := svcs.Drafts.Store().Load(profile, weekStart, name)
		if err != nil {
			return errorResult(fmt.Sprintf("load draft: %v", err)), nil, nil
		}
		info, err := svcs.Drafts.Snapshots().Take(d, draftsvc.OpManual, args.Note)
		if err != nil {
			return errorResult(fmt.Sprintf("take: %v", err)), nil, nil
		}
		if args.Keep {
			if err := svcs.Drafts.Snapshots().Pin(profile, weekStart, name, info.Sequence, args.Note); err != nil {
				return errorResult(fmt.Sprintf("pin: %v", err)), nil, nil
			}
			info.Pinned = true
		}
		return jsonResult(struct {
			Schema   string                `json:"schema"`
			Snapshot draftsvc.SnapshotInfo `json:"snapshot"`
		}{Schema: "tdx.v1.weekDraftSnapshot", Snapshot: info})
	}
}

func restoreSnapshotHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, restoreSnapshotArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args restoreSnapshotArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to restore the draft from a snapshot."); !ok {
			return r, nil, nil
		}
		if args.Sequence <= 0 {
			return errorResult("sequence is required (use list_week_draft_snapshots to find sequence numbers)"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}
		if err := svcs.Drafts.RestoreSnapshot(profile, weekStart, name, args.Sequence); err != nil {
			return errorResult(fmt.Sprintf("restore: %v", err)), nil, nil
		}
		return textResult(fmt.Sprintf("Restored draft %s/%s from snapshot %d.",
			weekStart.Format("2006-01-02"), name, args.Sequence))
	}
}

func pruneSnapshotsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, pruneSnapshotsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args pruneSnapshotsArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to prune snapshots."); !ok {
			return r, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}
		var pruned int
		if args.OlderThanDays > 0 {
			pruned, err = svcs.Drafts.Snapshots().PruneOlderThan(profile, weekStart, name,
				time.Duration(args.OlderThanDays)*24*time.Hour)
		} else {
			pruned, err = svcs.Drafts.Snapshots().PruneToRetention(profile, weekStart, name)
		}
		if err != nil {
			return errorResult(fmt.Sprintf("prune: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema string `json:"schema"`
			Pruned int    `json:"pruned"`
		}{Schema: "tdx.v1.weekDraftSnapshotPruneResult", Pruned: pruned})
	}
}
