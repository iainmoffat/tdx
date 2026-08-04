# `tdx time entry add --time-off` — Design

**Date:** 2026-06-30
**Status:** Approved (brainstorm), ready for implementation plan
**Scope:** CLI flag + MCP parity (no week-draft `add-row`)

## Goal

Let users log a TeamDynamix **time-off / leave** entry directly from the CLI and
MCP. Today `tdx time entry add` only accepts ticket / project / workspace
targets, so the `timeoff` target — which the push encoder already supports — has
no creation path. The only way leave reaches tdx now is by entering it in the
TD web UI and pulling it down. This closes that gap.

## Background (current behavior)

- `domain.TargetTimeOff` ("timeoff") exists and `timesvc/encode.go` encodes it:
  `Component = 17` (TimeOff) and `wire.ProjectID = target.ItemID`. Decode mirrors
  this (`target.ItemID = wire.ProjectID`, falling back to `wire.ItemID`).
- A time-off entry on the UF tenant carries `ItemID = 52` and Time Type
  `{id: 3, name: "Leave"}` (TD: "Sick Leave, PTO, UF holidays and admin days").
- `tdx time entry add` validates "exactly one of `--ticket`, `--project`, or
  `--workspace`" and `buildTarget` has no time-off case
  (`internal/cli/time/entry/add.go`).
- The wire time-type struct already parses `IsTimeOffTimeType`
  (`internal/svc/timesvc/types.go`), but `domain.TimeType` does not surface it,
  and `tdx time type list` does not expose it.
- The time-off `ItemID` (52) is **not** carried on any time-type listing, so it
  must be sourced separately.

## Design

### 1. CLI: new flags on `tdx time entry add`

Add to `addFlags`:

- `--time-off` (bool) — selects the time-off target.
- `--time-off-id` (int) — optional override for the time-off ItemID.

`--time-off` becomes a **fourth mutually-exclusive target selector**. Validation
in `runAdd` changes from "exactly one of `--ticket`/`--project`/`--workspace`"
to "exactly one of `--ticket`/`--project`/`--workspace`/`--time-off`". The
`--time-off` boolean counts toward `targetCount`.

Additional validation:

- `--time-off-id` set without `--time-off` → error
  (`--time-off-id requires --time-off`).
- `--time-off` is incompatible with `--app`, `--plan`, `--task`, `--issue`
  (those describe other target kinds). Reuse the existing companion-flag checks;
  add a guard that none of them are set with `--time-off`.

`buildTarget` gains:

```go
case f.timeOff:
    return domain.Target{
        Kind:   domain.TargetTimeOff,
        ItemID: f.timeOffID, // resolved before buildTarget is called
    }
```

`targetSummary` gains a `TargetTimeOff` case → `"time-off (id N)"`.

### 2. Time-off ItemID resolution

New service method:

```go
// ResolveTimeOffItemID returns the tenant's time-off ItemID for the user.
// If override > 0 it is returned verbatim (no API call). Otherwise the user's
// recent time-off entries are searched and the most recent one's ItemID is
// returned. Returns a sentinel error if none can be found.
func (s *Service) ResolveTimeOffItemID(ctx context.Context, profile, userUID string, override int) (int, error)
```

Behavior:

1. If `override > 0` → return it.
2. Else search the user's own entries via the existing `/api/time/search` path
   over a lookback window (default **180 days** ending today): `PersonUIDs =
   [userUID]`, `EntryDateFrom`/`EntryDateTo` set. These filters are
   live-honored (see `reference_td_search_silent_filters` / project quirks).
3. Filter results to time-off entries (`Target.Kind == TargetTimeOff`)
   client-side, since the search wire ProjectID filter is unreliable.
4. Return the ItemID of the most recent matching entry.
5. If none found → `ErrTimeOffIDUnknown` with message:
   *"couldn't determine your time-off ID — log one leave entry in the TD web UI
   first, or pass --time-off-id N."*

The CLI calls this after resolving the user (it needs `user.UID`) and before
`buildTarget`, then sets `f.timeOffID` to the resolved value.

Rationale for discovery-over-config: the feature's audience has logged leave in
the web UI before (that's how they know they take leave), so discovery is
zero-config in the common case; the `--time-off-id` override covers the
first-timer and any tenant where discovery returns nothing.

### 3. Time-type handling

- Add `IsTimeOff bool` to `domain.TimeType`; populate it from
  `wireTimeType.IsTimeOffTimeType` wherever wire time types are converted to
  domain (the existing `ListTimeTypes` decode path).
- Add helper:

```go
// DefaultTimeOffType returns the single time-off-flagged time type, or an
// error if there are zero or more than one (caller must then require --type).
func DefaultTimeOffType(types []TimeType) (TimeType, error)
```

- In `runAdd`, time-type selection becomes:
  - If `--type` is given → resolve by name as today, then **validate**
    `tt.IsTimeOff` is true when `--time-off` is set (error otherwise:
    *"time type %q is not a time-off type"*). This prevents logging PTO against a
    work type.
  - If `--type` is omitted and `--time-off` is set → call
    `DefaultTimeOffType(types)`. On success use it; on the zero/multiple error,
    surface *"--type is required (could not pick a default time-off type)"*.
  - If `--time-off` is not set → `--type` remains required exactly as today.

### 4. MCP parity (`create_time_entry`)

**Correction after code review:** the tool is named `create_time_entry` (not
`add_time_entry`), and it already builds its target generically —
`domain.Target{Kind: domain.TargetKind(args.Kind), ItemID: args.ItemID, ...}`
(`internal/mcp/tools_entry.go`). So `kind: "timeoff"` with a known `itemID`
**already works today**; `domain.Target.Validate` accepts timeoff without an
AppID. No new boolean inputs are needed, and adding `timeOff`/`timeOffId` would
duplicate the existing generic shape.

The real MCP gaps are discoverability and the ID:

- `kind` is documented as `"target kind (ticket/project/workspace)"` — update the
  jsonschema description to include `timeoff` (and the other supported kinds) so
  agents know it exists.
- When `kind == "timeoff"` and `itemID == 0`, call `ResolveTimeOffItemID` to
  auto-discover, matching the CLI's zero-config behavior. A non-zero `itemID` is
  used verbatim (the override path).
- The existing `confirm: true` gate is unchanged.

This yields exact parity: CLI `--time-off` ≡ MCP `kind:"timeoff"` with no
`itemID`; CLI `--time-off-id N` ≡ MCP `kind:"timeoff", itemID:N`.

### 5. Error handling (summary)

| Condition | Result |
|---|---|
| Zero or >1 of the four target selectors | "exactly one of --ticket, --project, --workspace, or --time-off is required" |
| `--time-off-id` without `--time-off` | "--time-off-id requires --time-off" |
| `--app`/`--plan`/`--task`/`--issue` with `--time-off` | "--time-off cannot be combined with --app/--plan/--task/--issue" |
| `--type` names a non-time-off type with `--time-off` | "time type %q is not a time-off type" |
| `--type` omitted, no/ambiguous time-off type | "--type is required (could not pick a default time-off type)" |
| ID not discoverable and no override | ErrTimeOffIDUnknown with web-UI / --time-off-id guidance |

Existing guards (locked day → `ErrDayLocked`, submitted week →
`ErrWeekSubmitted`) already run on the shared path and need no change.

### 6. Data flow

```
runAdd
  ├─ validate flags (4-way target mutex, companion flags)
  ├─ resolve profile, user (WhoAmI), time types (ListTimeTypes)
  ├─ if --time-off:
  │     itemID = ResolveTimeOffItemID(ctx, profile, user.UID, f.timeOffID)
  │     tt     = (--type given ? resolve+validate IsTimeOff : DefaultTimeOffType)
  │     f.timeOffID = itemID
  ├─ target = buildTarget(f)            // TargetTimeOff{ItemID: itemID}
  ├─ locked-day / submitted-week checks (unchanged)
  ├─ dry-run? print and return
  └─ AddEntry  →  encode (Component=17, ProjectID=ItemID)  →  TD
```

## Testing

Unit tests, mirroring the existing `add.go` patterns (pure helpers + stubbed
service), in `internal/cli/time/entry/` and `internal/svc/timesvc/`:

- `buildTarget` returns `TargetTimeOff{ItemID}` when `f.timeOff` is set.
- Four-way mutual-exclusivity validation: 0 selectors errors; 2 selectors
  (e.g. `--time-off` + `--ticket`) errors; `--time-off` alone passes.
- `--time-off-id` without `--time-off` errors; `--app`/`--plan`/`--task`/`--issue`
  with `--time-off` errors.
- `ResolveTimeOffItemID`: override returns verbatim (no search call); discovery
  returns the most-recent time-off entry's ItemID from a stubbed search;
  no-match returns `ErrTimeOffIDUnknown`.
- `DefaultTimeOffType`: single flagged type returns it; zero/multiple error.
- Type validation: non-time-off `--type` with `--time-off` errors; time-off
  `--type` passes.
- `targetSummary` renders the time-off case for dry-run.
- MCP: `add_time_entry` with `timeOff:true` builds the same target (table-style
  test alongside existing MCP add tests).

A live end-to-end check (on the UF tenant) is the acceptance gate, not a unit
test: `tdx time entry add --time-off --date <d> --hours 2 --dry-run` previews a
`time-off (id 52)` target, and without `--dry-run` creates a Leave entry that
`tdx time entry list --week <d> --json` shows; roll back with
`tdx time entry delete <id> --yes`.

## Out of scope

- `tdx time week set` add-row (the deferred week-draft "add-row" capability).
- Caching the discovered ItemID in profile config (YAGNI; the override flag and
  on-demand discovery are sufficient).
- Time-off categories beyond what `IsTimeOff`-flagged time types cover.

## Files touched (anticipated)

- `internal/cli/time/entry/add.go` — flags, validation, resolution, buildTarget,
  targetSummary.
- `internal/domain/timetype.go` — `IsTimeOff` field, `DefaultTimeOffType`.
- `internal/svc/timesvc/` — `ResolveTimeOffItemID`, `ErrTimeOffIDUnknown`,
  populate `IsTimeOff` in the time-type decode.
- `internal/mcp/tools_entry.go` — `kind` jsonschema description includes
  `timeoff`; auto-resolve `itemID` when `kind=="timeoff"` and `itemID==0`.
- Docs: `docs/guide/time.md` (entry add reference), `README.md` if the command
  tree notes targets.
- Tests across the above packages.
