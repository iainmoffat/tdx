# tdx Ticket Update Spec (Phase D.2a, v0.16.3)

**Date:** 2026-05-09
**Goal:** Add `tdx ticket update <id>` for generic ticket-field updates beyond the existing dedicated `status`/`assign`/`comment`/`log` commands. Ship as v0.16.3.

## Motivation

After v0.16.2, `tdx ticket` covers the daily read-heavy flows plus four targeted mutations (`comment`, `status`, `assign`, `log`). What's still missing: changing other fields on an existing ticket — title, description, type, priority, account, requestor, responsibility group. Today these require leaving the CLI for the web UI. This patch closes that gap with a single thin PATCH wrapper.

`tdx ticket create` (the other half of "Phase D.2") is split out and deferred — Sample probing on 2026-05-09 confirmed the canonical IT Tickets app gates `POST /tickets` and `GET /tickets/forms` for typical (non-admin) API users. Shipping the safer half first preserves momentum while the create-permission question gets sorted.

## Decisions

Settled during brainstorming on 2026-05-09:

1. **One new command:** `tdx ticket update <id>`. Mutating, requires `--yes`.
2. **Thin PATCH wrapper.** Build `[]ticketsvc.PatchOp` from CLI flags. One PATCH call per invocation. No client-side validation of TD's required-field constraints — surface TD's response verbatim if rejected.
3. **Field set:** title, description, type, account, requestor, responsibility-group, priority-id, optional accompanying comment.
4. **Reuse existing resolvers** for type/account/requestor/group name → ID conversion. Priority is numeric-only this round (no priority-name resolver yet; add later if asked).
5. **Status/assignee excluded** from `update` — they already have dedicated commands. Don't duplicate.
6. **One new MCP tool:** `update_ticket`, requires `confirm: true`. Tool count: 62 → 63.
7. **`tdx ticket create` deferred** to v0.17.0 (separate spec) along with custom-attribute support.

## Command surface

### `tdx ticket update <id>`

| Flag | TD field | Resolution path |
|---|---|---|
| `--title "<text>"` | `Title` | as-is string |
| `--description "<text>"` | `Description` | as-is string (replaces full Description) |
| `--type <name\|id>` | `TypeID` | numeric → ID; non-numeric → `ticketsvc.ResolveTypeByName` (case-insensitive exact; ambiguous → candidate-list error) |
| `--account <name\|id>` | `AccountID` | numeric → ID; non-numeric → `peoplesvc.ResolveAccountByName` (existing helper from v0.10.0) |
| `--requestor <uid\|email\|me>` | `RequestorUid` | existing `resolvePrincipal` (UID heuristic / `me` / email lookup) |
| `--responsibility-group <name\|id>` | `ResponsibleGroupID` | numeric → ID; non-numeric → `ticketsvc.ResolveGroupByName` (existing from v0.16.1) |
| `--priority-id <N>` | `PriorityID` | numeric only |
| `--comment "<text>"` | (separate feed POST) | posted via `AddFeed` after the PATCH succeeds |
| `--app <id>` | — | overrides profile default |
| `--yes` | — | required to mutate |
| `--profile <name>` | — | profile override |

**Validation:**
- At least one field flag must be set (else "nothing to update" error before the PATCH).
- `--yes` required (else fail-fast with the standard message pattern).
- Each name resolution emits a clear error on no-match / ambiguous match.

**Output (text):**
```
ticket #12345 updated: title="New title", description=<changed>, requestor=Alice
(comment posted: feed entry 1234)   # only if --comment was supplied
```

The summary lists each field that was changed (NOT each field that was set, since they're the same). For long values like description, the summary just says `<changed>` rather than echoing the full text. JSON output is the full updated ticket.

**Output (JSON):** envelope `tdx.v1.ticket` (the full updated `domain.Ticket`, same shape as `tdx ticket show <id> --json`).

### Behavior nuances

- **Atomicity:** the PATCH is a single TD call with all the ops batched. If TD rejects (e.g. invalid type for app, permission denied), nothing applied. The `--comment` POST runs only if PATCH succeeded; if the comment fails, the patch stays applied and we print a warning.
- **Empty-string handling:** `--description ""` IS a valid clear operation (set Description to empty). The "at least one field" check uses `cmd.Flags().Changed(name)`, not value emptiness — so passing `--description ""` counts as a real intent.
- **Order in the PATCH body:** doesn't matter to TD; we append ops in flag-iteration order.

## MCP

New tool `update_ticket` in `internal/mcp/tools_ticket_mutating.go`:

```go
type updateTicketArgs struct {
    Profile               string `json:"profile,omitempty"`
    AppID                 int    `json:"appID,omitempty"`
    ID                    int    `json:"id"`
    Title                 string `json:"title,omitempty"`
    Description           string `json:"description,omitempty"`
    TypeID                int    `json:"typeID,omitempty"`
    TypeName              string `json:"typeName,omitempty"`
    AccountID             int    `json:"accountID,omitempty"`
    AccountName           string `json:"accountName,omitempty"`
    RequestorUID          string `json:"requestorUID,omitempty"`
    ResponsibilityGroupID int    `json:"responsibilityGroupID,omitempty"`
    PriorityID            int    `json:"priorityID,omitempty"`
    Comment               string `json:"comment,omitempty"`
    Confirm               bool   `json:"confirm" jsonschema:"set true to actually update"`
}
```

Description: "Update a ticket's editable fields (title/description/type/account/requestor/group/priority). Excludes status and assignee — use update_ticket_status and update_ticket_assignee for those. Requires confirm=true."

**One nuance for the MCP variant:** unlike the CLI, MCP doesn't accept `me` for requestor (no interactive auth context). The handler accepts `requestorUID` only (no `requestorEmail`); callers must pre-resolve via `search_people` if needed. Same pattern as `update_ticket_assignee`.

For type/account name resolution: if `typeName` is set, the handler calls `ResolveTypeByName`; same for `accountName`. If both `typeID` and `typeName` are set, `typeID` wins (consistent with CLI's "numeric beats name").

Tool count: 62 → 63.

## Domain layer

No changes. `domain.TicketSearchFilter` and `domain.Ticket` already have all the fields we patch.

## Service layer

No new service methods. `ticketsvc.PatchTicket` (already exists) handles the PATCH; the CLI builds the `[]ticketsvc.PatchOp` slice from flags and calls it.

If `internal/svc/peoplesvc/` lacks a name → ID resolver for accounts (`ResolveAccountByName`), we'd need to add one — but per `project_tdx_current_state.md` v0.10.0 release notes, that helper exists. Verify by grep before assuming.

## CLI layer

New file `internal/cli/ticket/update.go`:

- `newUpdateCmd(svc ticketsvcAPI) *cobra.Command` — cobra wrapper with all flags
- `runTicketUpdate(ctx, w, svc, people, profile, appID, id, fields, yesFlag) error` — pure runner, takes a `ticketUpdateFields` struct holding the resolved IDs
- `buildTicketPatchOps(fields ticketUpdateFields) []ticketsvc.PatchOp` — pure builder, easy to test
- `resolveUpdateFields(ctx, svc, people, profile, appID, raw rawUpdateFlags) (ticketUpdateFields, error)` — pure resolver

Where:

```go
type rawUpdateFlags struct {
    title           string
    titleSet        bool   // from cmd.Flags().Changed("title")
    description     string
    descriptionSet  bool
    typeArg         string // can be name or id
    accountArg      string
    requestorArg    string
    groupArg        string
    priorityID      int
    comment         string
}

type ticketUpdateFields struct {
    title            *string
    description      *string
    typeID           *int
    accountID        *int
    requestorUID     *string
    groupID          *int
    priorityID       *int
}
```

Pointer fields cleanly distinguish "set to value X" (including empty string for title/description) from "don't touch this field." `buildTicketPatchOps` walks the struct and emits a PatchOp per non-nil field.

`ticketsvcAPI` interface (in `ticket.go`) already has `PatchTicket` and the resolver methods we need — no widening required.

`peoplesvcAPI` (in `helpers.go`) gains `ResolveAccountByName(ctx, profile, name) (domain.Account, error)` if not already present. (Probable; check via grep.)

Wired in `internal/cli/ticket/ticket.go` `New()`:

```go
cmd.AddCommand(newUpdateCmd(nil))
```

## Tests

Per the established CLI pattern:

1. **Pure helper tests** (no cobra):
   - `TestBuildTicketPatchOpsTitleOnly` — sets title; verifies one op with path=/Title.
   - `TestBuildTicketPatchOpsAllFields` — sets every field; verifies 7 ops.
   - `TestBuildTicketPatchOpsEmptyDescription` — `description=""` (pointer non-nil) emits an op.
   - `TestBuildTicketPatchOpsNoFields` — all nil → empty ops slice.
2. **Resolver tests:**
   - `TestResolveUpdateFieldsTypeByName` — stub returns matching type → typeID is set.
   - `TestResolveUpdateFieldsTypeByNumericArg` — `--type 7` → typeID=7 without service call.
   - `TestResolveUpdateFieldsRequestorMe` — stub authedUID → requestorUID.
   - Similar for `--account`, `--responsibility-group`.
3. **Runner tests** (with stubs):
   - `TestRunTicketUpdateSuccess` — happy path; verifies stub captured patch ops + output text.
   - `TestRunTicketUpdateWithComment` — patch + comment both posted.
   - `TestRunTicketUpdateCommentFailDoesNotPanic` — patch succeeds, comment errors → output mentions "comment failed" but no error returned.
4. **Cobra-level tests:**
   - `TestNewUpdateCmdRequiresYes` — running without `--yes` errors before any service call.
   - `TestNewUpdateCmdRejectsAllEmpty` — no field flags set → "nothing to update" error.

## Documentation

- `docs/guide/ticket.md` — add `## tdx ticket update` section after `## tdx ticket assign` (alphabetical-ish, fits the existing "comment / status / assign / log" cluster — placement: after `assign`, before `log`).
- `docs/guide.md` ASCII tree — update the `ticket` branch to add `update`. Currently the line reads `comment / status / assign / log`; becomes `comment / status / assign / update / log`.
- `README.md` ASCII tree — same change (must stay byte-identical).
- `docs/guide/mcp.md` — add `update_ticket` row to "Tickets (Phase D — mutating)" table; bump tool count from 62 → 63 (read 12 unchanged; mutating 6 → 7).

## Live verification

Re-auth required first: `tdx auth login --sso`.

Use test ticket 542034:

```bash
./tdx ticket update 542034 --title "v0.16.3 verify" --yes
./tdx ticket show 542034 | head -2   # verify title changed
./tdx ticket update 542034 --title "Test ticket for tdx cli testing" --description "Test ticket for tdx cli testing" --comment "rolled back" --yes
./tdx ticket update 542034 --type "IT Tickets - Default" --yes  # type by name
./tdx ticket update 542034 --priority-id <N> --yes  # discover N from web UI or `tdx ticket show --json`
./tdx ticket update 542034 --requestor me --yes  # me is requestor
# Expect each --yes to succeed; permission errors surface verbatim.
```

If any wire-format issue surfaces (e.g. PATCH op path differs from `/Title`), fix and re-test.

**Specifically watch for:** TD's PATCH op path conventions. We've already verified `/StatusID` and `/ResponsibleUid` work. The other paths (`/Title`, `/Description`, `/TypeID`, `/AccountID`, `/RequestorUid`, `/ResponsibleGroupID`, `/PriorityID`) are reasonable extrapolations — but per `feedback_probe_wire_formats_early.md`, never trust extrapolation without verifying live. The fix is mechanical: change the path string in `buildTicketPatchOps`.

## Acceptance criteria

1. `tdx ticket update <id> --title "..." --yes` flips Title; verified via `tdx ticket show`.
2. `tdx ticket update <id> --type "<name>" --yes` resolves and applies.
3. `tdx ticket update <id> --requestor me --yes` resolves to authed UID.
4. `tdx ticket update <id> --comment "..." [other field flags] --yes` posts the comment AFTER the PATCH (only if PATCH succeeded).
5. `tdx ticket update <id> --yes` (no field flags) errors fast: "nothing to update".
6. `tdx ticket update <id>` (no `--yes`) errors fast: "pass --yes".
7. New MCP tool `update_ticket` registered; requires `confirm:true`; tool count = 63.
8. Docs updated: ASCII tree (byte-identical between guide.md and README.md), `## tdx ticket update` section in guide/ticket.md, mcp.md tool table.
9. `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` all green.
10. Live-verified against the test tenant on ticket 542034 (title + description + comment + at least one of type/priority).
11. Released as v0.16.3 (PR + squash + tag + Goreleaser).

## Risks and mitigations

- **PATCH path-name mismatches.** TD's JSON-Patch op paths are based on the JSON-serialized field names; we've extrapolated from a small set. Mitigation: live-verify each path before tagging; fix mechanically if any are wrong.
- **Permission gating per app.** Some users may have read but not patch permission on certain ticket apps. Surface TD's 401/403 verbatim — same pattern as v0.16.2 task delete.
- **Empty-string semantics for Title.** TD might reject `Title=""` even though it's a valid PATCH op. If so, we let TD's error bubble up — not our problem to anticipate.
- **`peoplesvc.ResolveAccountByName` may not exist.** Verify before coding; if missing, add it in this task.
- **Cobra `Changed()` for empty-string detection.** Confirmed: `cmd.Flags().Changed("description")` returns true for `--description ""` even though the value is empty. This is what we want.

## Out of scope

- `tdx ticket create` — deferred to v0.17.0 (separate spec). Bigger surface, gated by per-app create permission, often needs custom-attribute support.
- Tag management (`--add-tag`, `--remove-tag`) — TD has dedicated `/tags` endpoints. Own command later.
- Custom-attribute updates — TD's PATCH supports `/Attributes/<id>` but the schema discovery is form-aware. Defer.
- Bulk update across multiple tickets — niche.
- `--priority "name"` (priority name → ID resolver) — defer; numeric only this round.
- Status/assignee — already have dedicated commands; do not duplicate.
- Move ticket between apps (TD has `/application` PATCH) — defer.
