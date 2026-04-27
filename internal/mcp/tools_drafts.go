package mcp

import (
	"context"
	"fmt"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/domain"
)

type listDraftsArgs struct {
	Profile    string `json:"profile,omitempty"`
	Dirty      bool   `json:"dirty,omitempty"`
	Conflicted bool   `json:"conflicted,omitempty"`
	WeekStart  string `json:"weekStart,omitempty" jsonschema:"YYYY-MM-DD filter"`
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
			}
			if !d.Provenance.PulledAt.IsZero() {
				it.PulledAt = d.Provenance.PulledAt.UTC().Format(time.RFC3339)
			}
			items = append(items, it)
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
