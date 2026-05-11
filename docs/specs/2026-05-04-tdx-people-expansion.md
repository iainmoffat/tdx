# `tdx people` expansion: search, show, accounts list + MCP read tools

**Date:** 2026-05-04
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Feature — fills the read-only people-discovery gap
**Target tag:** v0.15.0 (minor — three new CLI verbs + four new MCP tools)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | Ship all four components together: `tdx people search`, `tdx people show <UID>`, `tdx people accounts list`, plus MCP `search_people` / `get_person` / `list_accounts` / `list_resource_pools`. |
| Q2 | `tdx people search` defaults to **staff-only** (`IsEmployee=true` client-side filter). `--include-clients` adds portal users back. Reasoning: same as v0.9.0's report-status fix — Sample is 92% portal clients, the typical "find a coworker" workflow doesn't want them in the result. |
| Q3 | Use `GET /api/people/lookup?searchText=&maxResults=` for partial-name search. The `NameLike` field on `POST /api/people/search` is silently ignored (existing reference memory). `/lookup` honors searchText and returns the same row shape. |
| Q4 | `tdx people show <UID>` reuses the existing `peoplesvc.GetUser`. No new endpoint. |
| Q5 | `tdx people accounts list` was deferred in v0.10.0 (spec §6, "out of scope: 6404 rows is a lot to scroll, --account is exact-match-only"). Ship it now since the same pattern works for pools and the inventory of `tdx people` is growing. |
| Q6 | Default sort = name ascending for both accounts and search results. Stable, predictable. |
| Q7 | New MCP tools are read-only; no `confirm` required. Matches the precedent of `list_week_drafts`, `get_week_draft`, etc. |
| Q8 | Add `IsEmployee bool` to `domain.User` and `wireUser`. Required for the staff filter; harmless addition for existing callers. |

---

## 1. Surface

### CLI

```
tdx people search QUERY [flags]
  --limit N                cap results (default 25, max 100)
  --include-clients        include portal client users (default: staff only)
  --json                   tdx.v1.peopleSearchResult envelope
  --profile NAME

tdx people show <UID> [flags]
  --json                   tdx.v1.person envelope
  --profile NAME

tdx people accounts list [flags]
  --json                   tdx.v1.accountList envelope
  --profile NAME

tdx people pools list                                  (existing)
```

### MCP

```
search_people:
  searchText      string  required
  maxResults      int     default 25
  includeClients  bool    default false (staff only)
  → tdx.v1.peopleSearchResult

get_person:
  uid             string  required
  → tdx.v1.person

list_accounts:
  (no inputs beyond profile)
  → tdx.v1.accountList

list_resource_pools:
  (no inputs beyond profile)
  → tdx.v1.resourcePoolList   (existing schema, used by `tdx people pools list --json`)
```

MCP tool count: 40 → 44.

### Output schemas

```json
// tdx.v1.peopleSearchResult
{
  "schema": "tdx.v1.peopleSearchResult",
  "query": "Smith",
  "people": [
    { "uid": "...", "fullName": "...", "email": "...", "account": "...",
      "manager": "...", "isEmployee": true, "isActive": true, "title": "..." }
  ]
}

// tdx.v1.person  (single)
{
  "schema": "tdx.v1.person",
  "person": { ...full domain.User... }
}

// tdx.v1.accountList
{
  "schema": "tdx.v1.accountList",
  "accounts": [
    { "id": 866, "name": "...", "isActive": true,
      "managerUID": "...", "managerFullName": "..." }
  ]
}
```

---

## 2. Domain + service additions

### 2.1 `internal/domain/user.go`

```go
type User struct {
    // ... existing fields ...
    IsEmployee bool `json:"isEmployee,omitempty" yaml:"isEmployee,omitempty"`
}
```

### 2.2 `internal/svc/peoplesvc/types.go`

Add `IsEmployee bool` to `wireUser`.

### 2.3 `internal/svc/peoplesvc/users.go`

`decodeUser` populates `IsEmployee: w.IsEmployee`.

### 2.4 `internal/svc/peoplesvc/lookup.go` (NEW)

```go
// LookupPeople searches by partial name/email/ID via the /api/people/lookup
// autocomplete endpoint. The /search endpoint silently ignores most filter
// params; /lookup is the right tool for "find this person."
//
// maxResults <= 0 defaults to 25; capped at 100 per call.
func (s *Service) LookupPeople(ctx context.Context, profileName, searchText string, maxResults int) ([]domain.User, error)
```

URL: `GET /TDWebApi/api/people/lookup?searchText=...&maxResults=N`. Response is `[]wireUser` (same shape as search).

### 2.5 No changes to `peoplesvc.SearchAccounts` / `SearchPools`

Both already return public `Account` / `ResourcePool` types and are reused by the new CLI/MCP commands.

---

## 3. CLI implementation

### 3.1 `internal/cli/people/search.go` (NEW)

Cobra command. Mirrors `pools.go`'s extract-pure-runner pattern (so tests don't go through `config.ResolvePaths` and break in CI):

```go
func newSearchCmd(svc peoplesvcAPI) *cobra.Command { ... }
func runPeopleSearch(ctx context.Context, w io.Writer, svc peoplesvcAPI,
    profile, searchText string, limit int, includeClients, jsonOut bool) error
```

Filtering: after fetch, drop rows where `!u.IsEmployee` unless `includeClients`. No-results case prints `"no people match \"<query>\" (use --include-clients to broaden)"`.

Columns (text): `UID-PREFIX  NAME  EMAIL  ACCOUNT  MANAGER  TITLE`. Print first 8 chars of UID for compactness; full UID in `--json`.

### 3.2 `internal/cli/people/show.go` (NEW)

```go
func newShowCmd(svc peoplesvcAPI) *cobra.Command { ... }
func runPeopleShow(ctx context.Context, w io.Writer, svc peoplesvcAPI,
    profile, uid string, jsonOut bool) error
```

Text output (key:value pairs):

```
UID:           aaaaaaaa-1234-5678-9abc-def012345678
Name:          Sample User
Email:         sample@example.com
Active:        yes
Employee:      yes
Account:       999999 (Sample Department)
Manager:       John Toner <john.toner@example.com>
Resource pool: Sample Leaders
```

JSON: `{"schema": "tdx.v1.person", "person": {...}}`.

### 3.3 `internal/cli/people/accounts.go` (NEW)

Same shape as `pools.go`. List sorted by name. Columns: `ID NAME MANAGER ACTIVE`.

`--limit N` is **not** added here — `pools list` doesn't have one and the user just pages with their terminal. If 6404 turns out to be too much in practice, add it then.

### 3.4 `internal/cli/people/people.go`

Wire the new subcommands:

```go
cmd.AddCommand(newPoolsCmd(svc))
cmd.AddCommand(newAccountsCmd(svc))
cmd.AddCommand(newSearchCmd(svc))
cmd.AddCommand(newShowCmd(svc))
```

### 3.5 `peoplesvcAPI` interface (CLI test surface)

Extend with the methods needed by the new commands:

```go
type peoplesvcAPI interface {
    SearchPools(ctx, profile) ([]peoplesvc.ResourcePool, error)
    SearchAccounts(ctx, profile) ([]peoplesvc.Account, error)
    LookupPeople(ctx, profile, query string, max int) ([]domain.User, error)
    GetUser(ctx, profile, uid string) (domain.User, error)
}
```

---

## 4. MCP implementation

### 4.1 `internal/mcp/tools_people.go` (NEW)

Four read-only tool registrations + handlers. Pattern mirrors `tools_report.go` (no `confirmGate`, no mutation).

Args structs:

```go
type searchPeopleArgs struct {
    Profile        string `json:"profile,omitempty"`
    SearchText     string `json:"searchText"`
    MaxResults     int    `json:"maxResults,omitempty"`
    IncludeClients bool   `json:"includeClients,omitempty"`
}

type getPersonArgs struct {
    Profile string `json:"profile,omitempty"`
    UID     string `json:"uid"`
}

type listAccountsArgs   struct { Profile string `json:"profile,omitempty"` }
type listResourcePoolsArgs struct { Profile string `json:"profile,omitempty"` }
```

Handlers reuse the same envelope schemas (`tdx.v1.peopleSearchResult`, `tdx.v1.person`, `tdx.v1.accountList`, `tdx.v1.resourcePoolList`) so CLI `--json` and MCP outputs are identical.

### 4.2 `internal/mcp/server.go`

`RegisterPeopleTools(srv, svcs)` called from `NewServer`. Tool count assertion in `server_test.go`: 40 → 44.

### 4.3 `internal/mcp` Services struct

Already has `svcs.People *peoplesvc.Service`. Reuse.

---

## 5. Tests

- **Domain:**
  - `TestUser_IsEmployeeRoundTripJSON` — confirms field round-trips.

- **peoplesvc:**
  - `TestLookupPeople_Decode` — fixture covers full row shape including `IsEmployee`.
  - `TestLookupPeople_QueryParams` — assert URL has `searchText=` and `maxResults=`.
  - `TestLookupPeople_DefaultMaxResults` — `0` → 25 in URL.
  - `TestLookupPeople_CapsMaxResults` — `>100` → 100.

- **CLI:**
  - `TestPeopleSearch_Text_DefaultStaffOnly` — three results in fixture (2 employees + 1 client); default output shows 2.
  - `TestPeopleSearch_Text_IncludeClients` — same fixture, `--include-clients`, output shows 3.
  - `TestPeopleSearch_JSON_Envelope` — schema + query field + people array.
  - `TestPeopleSearch_NoMatches` — friendly empty message + hint.
  - `TestPeopleShow_Text_KeyValueLayout` — key fields appear, in expected order.
  - `TestPeopleShow_JSON_Envelope` — schema + person field.
  - `TestAccountsList_Text_SortedByName` — fixture with 3 accounts in non-name order; output is sorted.
  - `TestAccountsList_JSON_Envelope` — schema + accounts array.

- **MCP:**
  - `TestSearchPeople_FiltersClients` — handler with `IncludeClients=false` strips clients.
  - `TestGetPerson_Shape` — single-person envelope.
  - `TestListAccounts` / `TestListResourcePools` — count + schema.
  - `TestNewServer_RegistersAllTools` — bumped to 44.

---

## 6. Side-effect audit

| Concern | Result |
|---|---|
| Existing `peoplesvc.SearchUsers` callers | Unchanged. New `LookupPeople` is additive. |
| Existing `tdx people pools list` | Unchanged. New subcommands are siblings. |
| `domain.User` shape change | Adds optional `IsEmployee bool`. Existing serializations (week drafts, time-status JSON envelope) don't include this field today; adding it is additive. |
| `wireUser` shape change | Adds field; ignored by code paths that don't read it. |
| MCP tool count | 40 → 44. Server test asserts the count. |
| `tdx auth status` workflow | Unchanged. |

---

## 7. Out of scope

- Admin operations (create/update/delete user, group/app assignments). TD supports these but tdx is read-focused; not asked for.
- Email-exact lookup. `/api/people/lookup` already searches name+email+ID broadly; user can grep on the result. Add a `--email` flag later if it's actually needed.
- `tdx people whoami`. `tdx auth status` already covers it.
- Per-user threshold lookup via `WorkableHours` (deferred from the v0.12.0 incomplete-filter spec). Different concern.

---

## 8. Estimated work

6 commits, target tag v0.15.0:

1. **Domain + peoplesvc:** `IsEmployee` field, `wireUser` extension, `LookupPeople` + tests.
2. **CLI search + show:** new commands + tests.
3. **CLI accounts list:** new command + tests.
4. **MCP tools:** four new read-only tools + tests + server count bump.
5. **Docs:** README, guide.md.
6. **Live verify on the test tenant + PR + tag.**

Inline execution.
