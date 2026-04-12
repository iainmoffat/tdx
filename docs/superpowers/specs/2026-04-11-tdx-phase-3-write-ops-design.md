# tdx Phase 3 — Mutating Time Entry Operations Design Spec

**Goal:** Add `tdx time entry add | update | delete` commands that create, modify, and remove time entries via the TD Web API. Include pre-write validation (locked days, report status), `--dry-run` across all three commands, batch auto-split at 50 for multi-delete, partial-success reporting, and walkthrough script extension.

**Spec basis:** Framework spec §Phase 3 (lines 639–652), TD Web API Time reference (`ReferenceMaterial/Time`), existing Phase 2 read-ops patterns in `internal/svc/timesvc/` and `internal/cli/time/entry/`.

---

## 1. Scope

### In scope

| Feature | Detail |
|---|---|
| `tdx time entry add` | Create a single time entry via `POST /api/time` (1-element array) |
| `tdx time entry update <id>` | Update mutable fields on a single entry via `PUT /api/time/{id}` |
| `tdx time entry delete <id> [<id>...]` | Delete 1+ entries; single via `DELETE /api/time/{id}`, multi via `POST /api/time/delete` with 50-item auto-split |
| Pre-write validation | Check locked days and week report status before any write |
| `--dry-run` | All three commands: validate + preview without writing |
| Partial-success reporting | Multi-delete returns exit 2 when some IDs succeed and others fail |
| `encodeTarget` | Reverse of `decodeTarget`: domain `Target` → TD wire fields (Component + ID fields) |
| Walkthrough script extension | Add write/update/delete steps to `scripts/walkthrough.sh` with cleanup |
| JSON output | All write commands support `--json` with `tdx.v1.*` schema envelopes |

### Out of scope (deferred)

| Feature | Reason |
|---|---|
| Batch add from file/stdin | Future enhancement; single-entry add is sufficient for Phase 3 |
| Interactive confirmation prompts | Phase 3.5 TUI concern |
| Limited time account cap enforcement | Requires probing an additional TD endpoint (`/api/time/types/limits`); add in a follow-up if needed |
| `--force` to bypass locked-day checks | Dangerous; not needed for v1 |
| Template-based entry creation | Phase 4 (`tdx time week edit`) |
| Week-level batch operations | Phase 4 |

---

## 2. CLI Surface

### 2.1 `tdx time entry add`

Creates a single time entry.

**Flags:**

| Flag | Type | Required | Description |
|---|---|---|---|
| `--date` | string (YYYY-MM-DD) | yes | Entry date |
| `--hours` | float64 | exactly one of `--hours`/`--minutes` | Duration in hours (e.g. `2.5`) |
| `--minutes` | int | exactly one of `--hours`/`--minutes` | Duration in minutes (e.g. `150`) |
| `--type` | string | yes | Time type by name (resolved via `ListTimeTypes`, case-insensitive) |
| `--ticket` | int | exactly one target flag | Ticket ID → `TargetTicket` (requires `--app`) |
| `--project` | int | exactly one target flag | Project ID → `TargetProject` |
| `--workspace` | int | exactly one target flag | Workspace ID → `TargetWorkspace` |
| `--app` | int | required with `--ticket` | Application ID |
| `--task` | int | optional | Promotes `--ticket` → `TargetTicketTask`, or `--project` → `TargetProjectTask` |
| `--issue` | int | optional | Promotes `--project` → `TargetProjectIssue` |
| `--description` / `-d` | string | no | Free text description |
| `--dry-run` | bool | no | Validate and preview without writing |
| `--json` | bool | no | JSON output |
| `--profile` | string | no | Profile name (defaults to active) |

**Target resolution from flags:**

| Flags provided | TargetKind | Required IDs |
|---|---|---|
| `--ticket` | `ticket` | AppID=`--app`, ItemID=`--ticket` |
| `--ticket --task` | `ticketTask` | AppID=`--app`, ItemID=`--ticket`, TaskID=`--task` |
| `--project` | `project` | ItemID=`--project` |
| `--project --task` | `projectTask` | ItemID=`--project`, TaskID=`--task` |
| `--project --issue` | `projectIssue` | ItemID=`--project`, TaskID=`--issue` |
| `--workspace` | `workspace` | ItemID=`--workspace` |

Exactly one of `--ticket`, `--project`, `--workspace` must be set. Providing none or more than one is a validation error.

**Billable resolution:** Derived from the resolved `TimeType.Billable` flag. Not exposed as a CLI flag (YAGNI — users who need to override billable can use the TD web UI).

**Validation order:**
1. Parse and validate flags (date format, exactly one duration, exactly one target, required companions like `--app`)
2. Resolve time type name → ID via `ListTimeTypes`
3. Check locked days for the target date via `GetLockedDays`
4. Check week report status via `GetWeekReport` for the target date's week
5. If `--dry-run`: print preview and exit 0
6. Execute `POST /api/time` with the 1-element array
7. Fetch the created entry by returned ID (to get the full domain object with resolved names)
8. Print result

**Human output on success:**

```
created entry 12345
date:         2026-04-11
hours:        2.50
minutes:      150
type:         Regular Time
target:       #67890 Some Ticket Title
description:  Worked on feature X
status:       open
billable:     true
```

**JSON output:** `{"schema": "tdx.v1.entryAdd", "entry": {...}}`

**--dry-run output:**

```
DRY RUN — would create:
date:         2026-04-11
hours:        2.50
type:         Regular Time
target:       ticket #67890 (app 999)
description:  Worked on feature X
```

**Exit codes:** 0 success, 1 failure.

### 2.2 `tdx time entry update <id>`

Updates mutable fields on a single entry. Positional arg: the entry ID.

**Flags:**

| Flag | Type | Description |
|---|---|---|
| `--date` | string (YYYY-MM-DD) | New date |
| `--hours` | float64 | New duration in hours |
| `--minutes` | int | New duration in minutes |
| `--type` | string | New time type by name |
| `--description` / `-d` | string | New description (pass empty string `""` to clear; flag presence without value is an error) |
| `--dry-run` | bool | Validate and preview without writing |
| `--json` | bool | JSON output |
| `--profile` | string | Profile name |

At least one of `--date`, `--hours`, `--minutes`, `--type`, `--description` must be set. If none, error: `"nothing to update — specify at least one field to change"`.

`--hours` and `--minutes` are mutually exclusive (same as `add`).

**No target flags.** Target and owner are immutable per TD docs. If the user needs to change the target, the error guidance is: `"target is immutable — use 'tdx time entry delete <id>' then 'tdx time entry add' with the new target"`.

**Validation order:**
1. Parse and validate flags, parse positional ID
2. Fetch existing entry via `GetEntry` (also verifies it exists)
3. If `--type` set, resolve time type name → ID via `ListTimeTypes`
4. If `--date` changes the date: check locked days for both the old and new dates
5. Check week report status for affected week(s)
6. If `--dry-run`: print old→new diff and exit 0
7. Build write payload merging changes into existing entry
8. Execute `PUT /api/time/{id}`
9. Decode returned `wireTimeEntry` → domain `TimeEntry`
10. Print result

**Human output on success:** Same `show` format as `add`, with the updated values.

**JSON output:** `{"schema": "tdx.v1.entryUpdate", "entry": {...}}`

**--dry-run output:**

```
DRY RUN — would update entry 12345:
  date:         2026-04-11 → 2026-04-12
  description:  "old text" → "new text"
  (hours, type unchanged)
```

**Exit codes:** 0 success, 1 failure.

### 2.3 `tdx time entry delete <id> [<id>...]`

Deletes one or more entries. Positional args: entry IDs (at least one required).

**Flags:**

| Flag | Type | Description |
|---|---|---|
| `--dry-run` | bool | Preview without deleting |
| `--json` | bool | JSON output |
| `--profile` | string | Profile name |

**Single-ID path (1 positional arg):**
- Uses `DELETE /api/time/{id}` — no response body, just HTTP status
- 404 → `"entry <id> not found"` (exit 1)
- Human: `deleted entry <id>`
- JSON: `{"schema": "tdx.v1.entryDelete", "deleted": [<id>]}`
- Exit 0 or 1

**Multi-ID path (2+ positional args):**
- Uses `POST /api/time/delete` with auto-split at 50 items
- Returns `BulkOperationResults` per batch; results aggregated across batches
- Human on full success: `deleted N entries`
- Human on partial success: table of successes + failures
- JSON: `{"schema": "tdx.v1.entryDelete", "deleted": [...], "failed": [{"id": N, "message": "..."}]}`
- Exit 0 (full), 2 (partial), 1 (total failure)

**Validation (both paths):**
1. Parse positional IDs
2. Fetch each entry via `GetEntry` to verify existence and get dates for locked-day check
3. Check locked days for affected dates
4. Check week report status for affected weeks
5. If `--dry-run`: print what would be deleted and exit 0

**--dry-run output (multi):**

```
DRY RUN — would delete 3 entries:
  12345  2026-04-11  2.50h  Regular Time  #67890 Some Ticket
  12346  2026-04-11  1.00h  Regular Time  #67890 Some Ticket
  12347  2026-04-12  3.00h  Overtime      project/42 My Project
```

---

## 3. Architecture

### 3.1 New files

| File | Contents |
|---|---|
| `internal/domain/batch.go` | `BatchResult`, `BatchFailure` types |
| `internal/domain/entry_input.go` | `EntryInput` type |
| `internal/svc/timesvc/write.go` | `AddEntry`, `UpdateEntry`, `DeleteEntry`, `DeleteEntries` methods |
| `internal/svc/timesvc/encode.go` | `encodeTarget` (reverse of `decodeTarget`), `encodeEntryWrite` |
| `internal/svc/timesvc/write_test.go` | Tests for write methods |
| `internal/svc/timesvc/encode_test.go` | Tests for target encoding |
| `internal/cli/time/entry/add.go` | `newAddCmd()` |
| `internal/cli/time/entry/add_test.go` | Tests for add command |
| `internal/cli/time/entry/update.go` | `newUpdateCmd()` |
| `internal/cli/time/entry/update_test.go` | Tests for update command |
| `internal/cli/time/entry/delete.go` | `newDeleteCmd()` |
| `internal/cli/time/entry/delete_test.go` | Tests for delete command |

### 3.2 Modified files

| File | Change |
|---|---|
| `internal/domain/errors.go` | Add `ErrDayLocked`, `ErrWeekSubmitted` sentinels |
| `internal/svc/timesvc/types.go` | Add `wireTimeEntryWrite`, `wireBulkResult`, `wireBulkSuccess`, `wireBulkFailure` wire types |
| `internal/cli/time/entry/entry.go` | Wire `newAddCmd()`, `newUpdateCmd()`, `newDeleteCmd()` into parent |
| `scripts/walkthrough.sh` | Add write/update/delete walkthrough steps with cleanup |

### 3.3 Service layer

All write methods live in `internal/svc/timesvc/write.go`, following the read-ops pattern from `entries.go`:

```go
func (s *Service) AddEntry(ctx context.Context, profileName string, input domain.EntryInput) (domain.TimeEntry, error)
```

- Calls `clientFor(profileName)` to get an authenticated client
- Encodes `input` → `wireTimeEntryWrite` via `encodeEntryWrite`
- POSTs `[wireTimeEntryWrite]` (1-element array) to `/TDWebApi/api/time`
- Decodes `wireBulkResult` response
- If failure: return error with TD's error message
- If success: fetch created entry by returned ID via `GetEntry` (to get full domain object with resolved names, consistent with existing show/list output)
- Returns the domain `TimeEntry`

```go
func (s *Service) UpdateEntry(ctx context.Context, profileName string, id int, input domain.EntryInput) (domain.TimeEntry, error)
```

- PUTs `wireTimeEntryWrite` to `/TDWebApi/api/time/{id}`
- Decodes returned `wireTimeEntry` via existing `decodeTimeEntry`
- Resolves type names via existing `resolveTimeTypeNames`
- Returns domain `TimeEntry`

```go
func (s *Service) DeleteEntry(ctx context.Context, profileName string, id int) error
```

- Sends `DELETE /TDWebApi/api/time/{id}`
- 404 → `ErrEntryNotFound`; other errors propagated
- No response body to decode

```go
func (s *Service) DeleteEntries(ctx context.Context, profileName string, ids []int) (domain.BatchResult, error)
```

- Splits `ids` into chunks of 50
- POSTs each chunk as `[]int` to `/TDWebApi/api/time/delete`
- Decodes `wireBulkResult` for each chunk
- Aggregates all successes/failures into a single `domain.BatchResult`
- Returns aggregate result (never returns error for partial success — caller inspects `BatchResult`)

### 3.4 Target encoding

New `encodeTarget` function in `internal/svc/timesvc/encode.go` — the reverse of `decodeTarget`:

```go
func encodeTarget(t domain.Target) (component int, ticketID, projectID, planID, portfolioID, itemID int, err error)
```

Maps `domain.TargetKind` → TD component enum and distributes `Target.ItemID`/`Target.TaskID` into the correct wire fields:

| TargetKind | Component | TicketID | ProjectID | ItemID | PlanID |
|---|---|---|---|---|---|
| `ticket` | 9 | ItemID | | | |
| `ticketTask` | 25 | ItemID | | TaskID | |
| `project` | 1 | | ItemID | | |
| `projectTask` | 2 | | | TaskID | ItemID (=PlanID) |
| `projectIssue` | 3 | | | ItemID | |
| `workspace` | 45 | | ItemID | | |
| `timeoff` | 17 | | ItemID | | |
| `portfolio` | 23 | | | | ItemID |

Note: `projectTask` encoding maps `Target.ItemID` → `PlanID` and `Target.TaskID` → `ItemID`, mirroring the decode logic in `decodeTarget` where `componentTaskTime` sets `ItemID = w.PlanID` and `TaskID = w.ItemID`.

### 3.5 Wire write types

```go
// wireTimeEntryWrite is the request body for POST /api/time and PUT /api/time/{id}.
// Field names and types must match TD's TeamDynamix.Api.Time.TimeEntry exactly.
// NOTE: exact field names need probing (Step 0) — the read response uses these
// names; the write request should use the same shape per TD convention.
type wireTimeEntryWrite struct {
    TimeID      int     `json:"TimeID,omitempty"`  // 0/omitted for add; positive for edit
    TimeDate    string  `json:"TimeDate"`           // "YYYY-MM-DDT00:00:00" (no zone)
    Minutes     float64 `json:"Minutes"`
    TimeTypeID  int     `json:"TimeTypeID"`
    Component   int     `json:"Component"`
    TicketID    int     `json:"TicketID,omitempty"`
    ProjectID   int     `json:"ProjectID,omitempty"`
    PlanID      int     `json:"PlanID,omitempty"`
    PortfolioID int     `json:"PortfolioID,omitempty"`
    ItemID      int     `json:"ItemID,omitempty"`
    AppID       int     `json:"AppID,omitempty"`
    Description string  `json:"Description"`
    Billable    bool    `json:"Billable"`
}

// wireBulkResult matches the response from POST /api/time and POST /api/time/delete.
// NOTE: exact field names need probing (Step 0).
type wireBulkResult struct {
    Successes []wireBulkSuccess `json:"Successes"`
    Failures  []wireBulkFailure `json:"Failures"`
}

type wireBulkSuccess struct {
    ID int `json:"ID"`
}

type wireBulkFailure struct {
    ID           int    `json:"ID"`
    ErrorCode    string `json:"ErrorCode"`
    ErrorMessage string `json:"ErrorMessage"`
}
```

---

## 4. Pre-Write Validation

Every mutating command checks for blockers before writing:

1. **Locked days:** Call `GetLockedDays(ctx, profileName, date, date)` for each affected date. If the date appears in the locked list, abort with a clear message: `"cannot write to <date>: day is locked by your organization"`.

2. **Report status:** Call `GetWeekReport(ctx, profileName, date)` for each affected week. If the report status is `submitted` or `approved`, abort with: `"cannot write to week of <start>: report is <status>"`.

3. **For `update` with `--date` change:** Check both the old date's week and the new date's week.

4. **For multi-delete:** Aggregate all affected dates, deduplicate, batch the locked-day and report-status checks.

Validation runs in both the normal and `--dry-run` paths. The only difference is that `--dry-run` stops after validation + preview.

---

## 5. Error Handling

### TD API errors → domain errors

| Scenario | HTTP status | Domain error |
|---|---|---|
| Entry not found (GET/DELETE/PUT) | 404 | `ErrEntryNotFound` |
| Day locked (POST /api/time) | In `BulkOperationResults.Failures` | `ErrDayLocked` (surfaced in CLI) |
| Report submitted/approved | In `BulkOperationResults.Failures` | `ErrWeekSubmitted` (surfaced in CLI) |
| Token expired | 401 | `ErrUnauthorized` (existing) |
| Rate limited | 429 | Retry (existing client behavior) |
| Immutable field change attempt | In `BulkOperationResults.Failures` | Surface TD's error message directly |

### Exit codes

| Code | Meaning | When |
|---|---|---|
| 0 | Full success | All operations completed |
| 1 | Total failure | Validation error, all writes rejected, entry not found, auth error |
| 2 | Partial success | Multi-delete: some IDs succeeded, some failed |

Exit 2 is only possible from `tdx time entry delete` with multiple IDs. Single-entry operations are always 0 or 1.

### Immutable field guidance

If the user tries to change target-related fields on `update`, the CLI rejects it locally with:
```
target is immutable — use 'tdx time entry delete <id>' then 'tdx time entry add' with the new target
```
This is a local validation error, not an API round-trip.

---

## 6. Testing Strategy

### Step 0: Probe TD write endpoints (before coding)

Probe the live UFL tenant to verify wire shapes. Run these against a real token:

1. **POST /api/time** — create a test entry (for a known date/type/target), capture the request and response to verify `wireTimeEntryWrite` field names and `wireBulkResult` shape.
2. **PUT /api/time/{id}** — update the test entry, verify the response is a single `wireTimeEntry` (same shape as GET).
3. **DELETE /api/time/{id}** — delete the test entry, capture HTTP status code (200 vs 204).
4. **POST /api/time/delete** — if easily testable, probe with a single ID to verify `wireBulkResult` shape matches POST /api/time's response.
5. **Write to locked day** — if a locked day exists, attempt a write and capture the error shape in `wireBulkResult.Failures`.

Record all captured responses and use them to finalize the wire type definitions before writing tests.

### Unit tests

- `encodeTarget` — round-trip test: `decodeTarget(encodeTarget(target)) == target` for each supported kind
- `encodeEntryWrite` — verify wire struct field mapping for each target kind
- `AddEntry` — httptest stub returning canned `wireBulkResult` success
- `UpdateEntry` — httptest stub returning canned `wireTimeEntry`
- `DeleteEntry` — httptest stub returning 200/204
- `DeleteEntries` — httptest stubs for: all succeed, all fail, partial success, auto-split at 50+

### CLI integration tests

Per command, following the existing Phase 2 pattern (`cobra.Execute` + `TDX_CONFIG_HOME=t.TempDir()` + httptest):

- `add` — success, missing required flags, locked day rejection, dry-run output
- `update` — success, nothing-to-update error, entry not found, dry-run diff
- `delete` — single success, multi success, partial failure (exit 2), entry not found, dry-run preview

### Manual walkthrough (via scripts/walkthrough.sh)

Extended with write steps that create, verify, update, verify, delete, verify. The test entry is always cleaned up (delete in a trap).

---

## 7. Decision Log

| # | Decision | Rationale |
|---|---|---|
| D1 | `add` creates exactly one entry (no batch add from CLI) | YAGNI — single-entry add covers the common case; batch add from file/stdin is a future enhancement. Service layer uses the batch endpoint internally (1-element array) to get `BulkOperationResults` handling for free. |
| D2 | `update` uses `PUT /api/time/{id}`, not `POST /api/time` | PUT returns the modified `wireTimeEntry` directly, giving us the full updated state for display. POST returns `BulkOperationResults` which only has success/failure IDs — we'd need a follow-up GET. |
| D3 | `delete` single-ID uses `DELETE`, multi-ID uses `POST /api/time/delete` | Single DELETE is simpler (no response body to parse). Multi uses the batch endpoint for efficiency + partial-success handling. The service layer makes the choice transparent to the CLI. |
| D4 | Billable derived from TimeType, not exposed as a CLI flag | TD's UI sets billable from the time type. Overriding it via CLI adds complexity for a rare use case. Users who need to override can use the TD web UI. |
| D5 | Pre-write validation is eager (check before write) | Better UX than letting TD reject and parsing error responses. Also required for --dry-run to work. Extra API calls (GetLockedDays, GetWeekReport) are cheap and cacheable within a command. |
| D6 | `update` fetches existing entry before writing | Needed to: (a) verify entry exists, (b) supply immutable fields in the PUT body, (c) show old→new diff in --dry-run. One extra GET per update is acceptable. |
| D7 | `AddEntry` re-fetches the created entry by ID after POST | POST /api/time returns BulkOperationResults (just IDs), not the full entry. Re-fetching via GetEntry gives us the complete domain object with resolved type names, consistent with show/list output. One extra GET per add. |
| D8 | Limited time account cap enforcement deferred | Requires probing a separate TD endpoint for per-user-per-type limits. The core write operations work without it — TD will reject writes that exceed limits and we surface the error. |
| D9 | No interactive confirmation prompt in Phase 3 | Framework spec mentions it but it's a Phase 3.5/TUI concern. --dry-run gives the preview capability; the user runs without --dry-run when they're ready. |
| D10 | `encodeTarget` lives in `internal/svc/timesvc/encode.go` | Pairs with `decodeTarget` in `entries.go`. Both are timesvc-internal mapping functions. |

---

## 8. Open Questions (Resolved During Step 0 Probe)

These will be answered by probing the live TD API before writing the implementation plan:

1. **`wireBulkResult` exact field names** — Is it `Successes`/`Failures`? What fields do `Success` and `Failure` objects have? (`ID`? `TimeID`? `ErrorCode`? `ErrorMessage`? `Message`?)
2. **`wireTimeEntryWrite` required fields** — Which fields must be present for a create? For an edit? Is `Uid` required for "self" writes or inferred from the auth token?
3. **`DELETE /api/time/{id}` response** — Status 200 or 204? Empty body or message?
4. **`TimeDate` write format** — Does TD accept `"2026-04-11T00:00:00"` (no zone, matching the read response shape) or does it need `"2026-04-11T00:00:00Z"` or another format?
5. **Error shape for locked-day writes** — HTTP 400/403/other? Or success with `BulkOperationResults.Failures`?

---

## 9. Walkthrough Script Extension

Add these steps to `scripts/walkthrough.sh` after the existing read-only steps:

```bash
# ---------- write steps (create, verify, update, verify, delete, verify) ----------
# Uses a known date and type. Creates a test entry, verifies it,
# updates it, verifies the update, deletes it, verifies deletion.
# All write steps use a cleanup trap to ensure the test entry is deleted
# even on failure.
```

**Steps:**
1. `tdx time entry add --date <date> --hours 0.25 --type "<known type>" --ticket <id> --app <id> -d "walkthrough test entry"` → captures the new entry ID from output
2. `tdx time entry list --week <date>` → verify the new entry appears
3. `tdx time entry update <id> -d "updated walkthrough description"` → verify success
4. `tdx time entry show <id>` → verify description changed
5. `tdx time entry delete <id>` → verify success
6. `tdx time entry show <id>` → verify "entry not found" (exit 1)

The walkthrough date/type/target will be configurable via env vars (like the existing `TDX_WALKTHROUGH_WEEK`), with defaults for the UFL tenant.
