# tdx Ticket Team-Scope Filters Spec (v0.16.1)

**Date:** 2026-05-08
**Goal:** Add `--responsibility-group <name|id>` and `--manager me|UID|email` selectors to `tdx ticket search`, plus a new `tdx ticket groups list` discovery command. Ship as v0.16.1.

## Motivation

After v0.16.0 shipped, `tdx ticket search` covered "tickets assigned to me" via the default `--assignee me`, but had no clean answer for "tickets assigned to my team." The only workarounds were running a TD saved search (only works if one exists), enumerating individual `--assignee` flags (clunky past 3-4 people), or pulling direct-reports UIDs from `tdx time report status` and pasting them back in. This patch closes that gap.

## Decisions

Settled during brainstorming on 2026-05-08, after live probing Sample:

1. **`ResponsibilityGroupIDs` IS honored server-side** on `POST /api/{appId}/tickets/search`. Live-verified: filtering by group 103 returned exactly 5 tickets, all with `ResponsibleGroupID==103`.
2. **`/api/groups/search` (POST) returns the tenant-wide group list.** `GET /api/groups` is 405. Group rows have `ID`, `Name`, `IsActive`, `PlatformApplications`, etc. Filter params on this endpoint may be silently ignored (per the established TD silent-filter pattern); the implementation reads the full list and filters client-side for name resolution.
3. **`ReportsToUid` is silently ignored on `/api/people/search`** (live re-verified — 1093 staff returned with or without the filter). `--manager` therefore expands client-side: fetch all staff with `IsEmployee=true`, filter to `ReportsToUID == <managerUID>`, send those UIDs as `ResponsibilityUids` (which IS honored).
4. **All four selector flags combine with union semantics.** `--assignee`, `--requestor`, `--responsibility-group`, `--manager` are orthogonal filter dimensions that can be mixed freely. No mutual exclusion.
5. **Name resolution is case-insensitive exact match** (same pattern as status/type/saved-search resolvers). Numeric args bypass name resolution.
6. **Direct reports only** — no transitive manager walk. YAGNI.
7. **MCP exposure** — `search_tickets` gains two inputs; one new read-only tool `list_ticket_groups`. Tool count 56 → 57.

## Command surface changes

### `tdx ticket search` — two new flags

```
--responsibility-group <name|id>   filter by ticket group (repeatable; numeric or exact name)
--manager me|UID|email             filter to tickets assigned to direct reports of <person> (repeatable)
```

Existing flags unchanged. The default behavior (no flags → `--assignee me` open) is unchanged.

Combination examples:

```bash
# Tickets assigned to my team (direct reports), open only
tdx ticket search --manager me

# Tickets assigned to a specific group
tdx ticket search --responsibility-group "Linux Platform Services"

# Mixed: my open tickets PLUS Linux Team's
tdx ticket search --assignee me --responsibility-group "Linux Platform Services"

# Direct reports of Alice (by email lookup)
tdx ticket search --manager alice@uf.edu
```

When **no individual selector** is provided (no `--assignee`, no `--requestor`, no `--responsibility-group`, no `--manager`), the existing default `--assignee me` still applies — keeps "open tickets in my queue" as the no-flag answer.

### `tdx ticket groups list` — new metadata command

```
tdx ticket groups list [--json]
```

Mirrors `tdx ticket types list` and `tdx ticket statuses list` in shape. Output: `ID | NAME | ACTIVE` table. JSON envelope `tdx.v1.ticketGroupList`. Tenant-wide (groups can serve multiple ticket apps); `--app <id>` is accepted but ignored for compatibility with sibling commands.

## Domain types

Additive changes only:

```go
// internal/domain/ticket.go (additive)

// TicketGroup is a TD responsibility group (a team that can be assigned
// tickets). Groups exist tenant-wide and can serve multiple ticket apps.
type TicketGroup struct {
    ID     int
    Name   string
    Active bool
}

type TicketSearchFilter struct {
    // ... existing fields unchanged ...
    ResponsibilityGroupIDs []int // NEW — server-side filter (honored on the test tenant)
}
```

`--manager` does NOT add a field to the domain filter. Manager expansion to direct-report UIDs happens in the CLI layer (which has access to `peoplesvc`); the resolved UIDs are appended to the filter's existing `AssigneeUIDs`. This keeps the service layer free of cross-service deps.

Alternative considered: put expansion in the service layer. Rejected because it would force `ticketsvc` to import `peoplesvc`, creating an awkward dependency. CLI-layer expansion is cleaner.

## Service layer

New file `internal/svc/ticketsvc/groups.go`:

```go
// ListGroups returns all tenant groups via POST /api/groups/search.
// The endpoint may silently ignore body filter params; we send {} and
// filter client-side for name resolution.
func (s *Service) ListGroups(ctx context.Context, profileName string) ([]domain.TicketGroup, error)

// ResolveGroupByName finds a group by case-insensitive exact match.
// Returns an error with candidates if 0 or >1 match.
func (s *Service) ResolveGroupByName(ctx context.Context, profileName string, name string) (domain.TicketGroup, error)
```

Wire type added to `internal/svc/ticketsvc/types.go`:

```go
type wireGroup struct {
    ID                   int    `json:"ID"`
    Name                 string `json:"Name"`
    IsActive             bool   `json:"IsActive"`
    Description          string `json:"Description"`
    ExternalID           string `json:"ExternalID"`
    PlatformApplications []any  `json:"PlatformApplications,omitempty"` // shape varies; not surfaced in v0.16.1
}
```

`wireTicketSearch` extends with one field:

```go
type wireTicketSearch struct {
    StatusIDs              []int    `json:"StatusIDs,omitempty"`
    ResponsibilityUids     []string `json:"ResponsibilityUids,omitempty"`
    ResponsibilityGroupIDs []int    `json:"ResponsibilityGroupIDs,omitempty"` // NEW
    RequestorUids          []string `json:"RequestorUids,omitempty"`
    AccountIDs             []int    `json:"AccountIDs,omitempty"`
    SearchText             string   `json:"SearchText,omitempty"`
    MaxResults             int      `json:"MaxResults,omitempty"`
}
```

`SearchTickets` already maps from `domain.TicketSearchFilter` — adds one line to map `ResponsibilityGroupIDs` through.

## CLI layer

### `internal/cli/ticket/groups.go` (new)

Mirror of `types.go`/`statuses.go`:

```go
func newGroupsCmd(svc ticketsvcAPI) *cobra.Command       // top-level "groups"
func newGroupsListCmd(svc ticketsvcAPI) *cobra.Command   // "groups list"
func runGroupsList(ctx, w, svc, profile, jsonOut) error  // pure runner
```

Wired into `New()` in `ticket.go` like the other metadata sub-commands.

### `internal/cli/ticket/search.go` (extended)

Two new flags:

```go
cmd.Flags().StringSliceVar(&groupFlags, "responsibility-group", nil, "filter by group name or id (repeatable)")
cmd.Flags().StringSliceVar(&managerFlags, "manager", nil, "tickets assigned to direct reports of me|UID|email (repeatable)")
```

Three changes in `buildSearchFilter`:

1. **Group resolution.** For each `--responsibility-group` value: numeric → use as int; non-numeric → call `svc.ResolveGroupByName`. Append to `filter.ResponsibilityGroupIDs`.

2. **Manager expansion.** For each `--manager` value: resolve to manager UID via `resolvePrincipal` (handles `me`/UID/email). Then call `peoplesvc.SearchUsers(IsEmployee=true, Limit=5000)`, filter rows where `ReportsToUID == managerUID`, collect their UIDs. Append all resulting UIDs to `filter.AssigneeUIDs`. Dedupe.

3. **Update default-to-me logic.** The existing "no flags → assignee=me" injection should NOT fire if any of the four selector flags is present. Currently it checks only `assigneeFlags` and `requestorFlags`. Extend to also check `groupFlags` and `managerFlags`.

A new helper:

```go
// expandManagersToReports resolves each manager argument to a UID, then
// fetches direct reports for each via people.SearchUsers and returns the
// union of report UIDs. Errors propagate from people-service failures.
func expandManagersToReports(ctx context.Context, people peoplesvcAPI, profile, authedUID string, managerArgs []string) ([]string, error)
```

The interface `peoplesvcAPI` (already in `helpers.go`) gains `SearchUsers` to support the bulk fetch:

```go
type peoplesvcAPI interface {
    LookupPeople(ctx, profile, q, limit) ([]domain.User, error)
    SearchUsers(ctx, profile, filter domain.UserFilter) ([]domain.User, error)  // NEW
}
```

Update `stubPeoplesvc` in tests.

## MCP layer

`search_tickets` argument struct gains two fields:

```go
type searchTicketsArgs struct {
    // ... existing ...
    ResponsibilityGroupIDs []int    `json:"responsibilityGroupIDs,omitempty"`
    ManagerUIDs            []string `json:"managerUIDs,omitempty"`
}
```

The handler runs the same manager-expansion logic the CLI uses (refactor into a shared helper if cleanest). Or accept the simpler approach: MCP `managerUIDs` are taken at face value by the handler, which calls a new `expandManagersToReports` helper exposed in the ticket package or a shared spot.

New read-only tool `list_ticket_groups`:

```go
type listTicketGroupsArgs struct {
    Profile string `json:"profile,omitempty"`
}
```

Returns `tdx.v1.ticketGroupList` envelope.

Mutating tool count unchanged (still 4). Read tool count: 8 → 9. Total: 56 → 57.

## JSON envelopes (additive)

- New: `tdx.v1.ticketGroupList`
- Existing `tdx.v1.ticketList` unchanged (search results still partial records). Filter information echoed in the envelope for `--json` output stays additive — the existing output adds `responsibilityGroupIDs` and `managerUIDs` arrays when set, omits them when empty.

## Documentation

- `docs/guide/ticket.md` — add the two new flags under `## tdx ticket search`; add a `## tdx ticket groups` section with `### tdx ticket groups list`. Update Contents.
- `docs/guide.md` ASCII tree — add `groups → list` under `ticket`.
- `README.md` ASCII tree — same change (must stay byte-identical).
- `docs/guide/mcp.md` — add `list_ticket_groups` row to "Tickets (Phase D — read-only)" table; bump tool counts (read 8→9, total 56→57). Add a note under `search_tickets` that `responsibilityGroupIDs` and `managerUIDs` inputs are now accepted.

## Testing

Per established patterns:

1. **Service tests** (httptest fixtures):
   - `TestListGroups` — decodes `IsActive`, drops `Description`/`ExternalID` (we don't surface them).
   - `TestResolveGroupByNameSingleMatch` / `NoMatch` / `Ambiguous` — mirror status resolver tests.
   - `TestSearchTicketsSendsResponsibilityGroupIDs` — captures request body, verifies the array is sent.

2. **CLI tests** (stub-based):
   - `TestBuildSearchFilterGroupByName` — stub returns one matching group → filter has correct ID.
   - `TestBuildSearchFilterGroupByID` — numeric arg passes through.
   - `TestBuildSearchFilterGroupAmbiguous` — error propagates with candidate list.
   - `TestBuildSearchFilterManagerMe` — stub returns 3 direct reports → AssigneeUIDs has those 3 UIDs (plus none merged from `--assignee me` since user passed `--manager me` instead).
   - `TestBuildSearchFilterManagerExpandsToReports` — stub returns N users with mixed `ReportsToUID` → AssigneeUIDs = the matching subset.
   - `TestBuildSearchFilterMultipleSelectorsCombine` — `--assignee me --responsibility-group X` → both fields populated; default-to-me does NOT fire.
   - `TestBuildSearchFilterDefaultsToMeWhenNoSelector` — preserves existing behavior.
   - `TestRunGroupsListTable` / `JSON` / `Empty` — mirror types/statuses tests.

3. **MCP tests** — at least one happy-path for `list_ticket_groups` and one for `search_tickets` with manager+group filters. Confirm tool count assertion is updated (server_test.go).

4. **Live verification** before tag (run on the test tenant):
   - `tdx ticket groups list` returns ≥ a handful of groups
   - `tdx ticket search --responsibility-group "<a real group>"` returns matching tickets
   - `tdx ticket search --manager me` returns tickets assigned to my direct reports (or "no tickets matched" if no reports + IT Tickets)
   - Multiple selectors combine without error
   - Default `tdx ticket search` (no flags) still defaults to my assigned tickets

## Out of scope

- Transitive manager walk (manager-of-manager → all descendants). YAGNI.
- "Primary vs secondary responsibility" distinction.
- Watchers, workflow-step assignees, contacts.
- New mutating commands.
- Performance optimization for `--manager me` (the staff-list fetch is ~1093 rows on the test tenant = sub-second). Cache could come later if needed.
- Cross-app group filtering (groups in v0.16.1 are tenant-wide; the search is still per-app). If a group serves multiple apps, the user picks which app to search.

## Risks and mitigations

- **`ResponsibilityGroupIDs` may behave differently on other tenants.** Mitigation: the live probe verified honor on the test tenant; we ship believing it's honored, and users on tenants where it isn't will see empty results — same failure mode as any silent-filter discovery. The wire field stays exported via the filter, and we can add a fallback (client-side group filter using `ResponsibleGroupID` on each row, like we do for IsOpen) in a patch release if needed.
- **`/api/groups/search` may also silently ignore filter params.** We don't rely on its filtering — we always send `{}` and read the full list. Acceptable.
- **Manager expansion fetches all staff (~1k rows on the test tenant).** That's one extra API call per `tdx ticket search --manager` invocation. Acceptable; sub-second.
- **Adding `SearchUsers` to the CLI's `peoplesvcAPI` interface** is an interface-widening change. All existing test stubs must implement it. Spec calls this out so the implementer doesn't miss it.

## Acceptance criteria

1. `tdx ticket search --responsibility-group "<real-name>"` returns tickets assigned to that group on the test tenant.
2. `tdx ticket search --manager me` returns tickets assigned to direct reports of the authenticated user.
3. Multiple selectors (`--assignee me --responsibility-group X --manager Y`) combine with union semantics; default-to-me does NOT fire.
4. `tdx ticket groups list` lists tenant groups; `--json` envelope is `tdx.v1.ticketGroupList`.
5. `search_tickets` MCP tool accepts `responsibilityGroupIDs` and `managerUIDs` inputs.
6. `list_ticket_groups` MCP tool registered; total tool count = 57.
7. `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` all green.
8. Live-verified against the test tenant on at least the four CLI cases above.
9. Released as v0.16.1 (PR + squash + tag + Goreleaser).
