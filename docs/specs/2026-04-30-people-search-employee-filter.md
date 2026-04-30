# `tdx time report status` selector fixes — `IsEmployee` + `--resource-pool`

**Date:** 2026-04-30
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Bug fix + small feature follow-up to v0.9.0
**Target tag:** v0.9.1

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | TD's `/api/people/search` returns mostly Clients (portal end-users — 92K of 100K records on UFL), not staff. The `MustReportTime`, `ShouldReportTime`, and `ResourcePoolIDs` filters are silently ignored in the request body. Only `IsEmployee` is honored — and using it returns ~1080 records on UFL, matching the count visible in the TD UI's user filter. |
| Q2 | All four staff selectors (`--manager`, `--account`, `--all`, new `--resource-pool`) target time-reporting staff, not portal Clients. Hardcode `IsEmployee=true` on the search call for those paths. |
| Q3 | Bump default `MaxResults` for these paths to 5000 (1080 + headroom). Stays well under the 10K behavior cap and finishes in <1s. |
| Q4 | Expose `Employee *bool` on `domain.UserFilter` for caller control. Default behavior (`Employee == nil`) keeps the current "no filter" semantics, so external callers of `peoplesvc.SearchUsers` aren't surprised. |
| Q5 | Add `--resource-pool NAME` as a fifth top-level selector, mutually exclusive with `--user`/`--manager`/`--account`/`--all`. Pool is identified by name (the only thing the TD UI exposes). |
| Q6 | Pool name → ID resolution: client-side. Call `POST /api/resourcepools/search` with `{}`, trim trailing whitespace from each `Name` (TD data has stray tabs), exact case-insensitive match. Error on 0 matches (with the closest candidates) or >1 matches. |
| Q7 | People filtering by pool: client-side. TD silently ignores `ResourcePoolIDs` in people search. After fetching `IsEmployee=true, MaxResults=5000`, filter rows where `u.ResourcePoolID == targetID`. |

---

## 1. Goal

1. Make `tdx time report status --manager me --week ...` return your actual direct reports (12 on UFL), not zero. Same fix applies to `--account NAME` and `--all`.
2. Add `--resource-pool NAME` as a new selector so users can run the report against a TD resource pool (mirroring the TD UI's filter dropdown).

---

## 2. Root cause (existing bug)

`runner.go`'s `--manager` branch:

```go
all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{Limit: hardLimit})
// filter all by ReportsToUID == mgrUID
```

`peoplesvc.SearchUsers` defaults `IsActive=true, UserType=User, MaxResults=100`. TD's "User" type includes Clients (92% of UFL's user records), so the alphabetical-first-100 are almost all Clients with self-referential `ReportsToUID` — and none of them match the user's UID.

Even bumping `MaxResults` to 100K returns only 5 of the user's 12 actual direct reports because TD caps the response and the alphabetical order can split direct reports across the cap.

`IsEmployee=true` filters to ~1080 staff, all 12 direct reports included.

---

## 3. Findings (TD API probe, 2026-04-30)

- `POST /api/resourcepools/search` with `{}` body returns all pools (76 on UFL). Each row: `ID`, `Name`, `IsActive`, `RequiresApproval`, `ManagerUID`, `ManagerFullName`, `CreatedDate`, `ModifiedDate`, `ResourceCount` (often -1).
- `GET /api/resourcepools/{id}` returns `405` (method not allowed). No per-pool members endpoint.
- `POST /api/people/search` with `ResourcePoolIDs: [N]` returns the **same** 1080 employees regardless of pool — silently ignored.
- People search response includes `ResourcePoolID int` and `ResourcePoolName string` for each user.
- Pool names in TD data carry trailing whitespace (tab characters) — must be trimmed before display or matching.
- Each user has exactly one pool (single int field, not an array).

---

## 4. Changes

### 4.1 `internal/domain/user_filter.go`

Add an `Employee *bool` field:

```go
type UserFilter struct {
	Active      *bool
	Employee    *bool  // NEW: nil = no filter; true/false = filter by IsEmployee
	UserType    string
	AccountID   int
	AccountName string
	NameLike    string
	Limit       int
}
```

### 4.2 `internal/domain/user.go`

Extend `domain.User`:

```go
type User struct {
	// ... existing fields ...
	ResourcePoolID   int    // NEW
	ResourcePoolName string // NEW
}
```

### 4.3 `internal/svc/peoplesvc/types.go`

Add `IsEmployee` to `wireUserSearch`, add `ResourcePoolID/Name` to `wireUser`, add new `wireResourcePool`:

```go
type wireUserSearch struct {
	NameLike   string `json:"NameLike,omitempty"`
	IsActive   *bool  `json:"IsActive,omitempty"`
	IsEmployee *bool  `json:"IsEmployee,omitempty"`  // NEW
	AccountIDs []int  `json:"AccountIDs,omitempty"`
	UserType   string `json:"UserType,omitempty"`
	MaxResults int    `json:"MaxResults,omitempty"`
}

type wireUser struct {
	// ... existing fields ...
	ResourcePoolID   int    `json:"ResourcePoolID"`   // NEW
	ResourcePoolName string `json:"ResourcePoolName"` // NEW
}

// wireResourcePool matches POST /api/resourcepools/search response rows.
type wireResourcePool struct {
	ID               int    `json:"ID"`
	Name             string `json:"Name"`
	IsActive         bool   `json:"IsActive"`
	RequiresApproval bool   `json:"RequiresApproval"`
	ManagerUID       string `json:"ManagerUID"`
	ManagerFullName  string `json:"ManagerFullName"`
}
```

### 4.4 `internal/svc/peoplesvc/users.go`

Wire `filter.Employee` into the request body and propagate `ResourcePoolID/Name` through `decodeUser`:

```go
if filter.Employee != nil {
	req.IsEmployee = filter.Employee
}
// ... in decodeUser:
ResourcePoolID:   w.ResourcePoolID,
ResourcePoolName: strings.TrimSpace(w.ResourcePoolName),
```

### 4.5 `internal/svc/peoplesvc/pools.go` (NEW)

```go
// ResourcePool is the public domain shape for a TD resource pool.
type ResourcePool struct {
	ID               int
	Name             string
	IsActive         bool
	RequiresApproval bool
	ManagerUID       string
	ManagerFullName  string
}

// SearchPools lists all resource pools visible to the profile.
func (s *Service) SearchPools(ctx context.Context, profileName string) ([]ResourcePool, error) { ... }

// ResolvePoolByName looks up a single pool by exact case-insensitive name
// (after trimming trailing whitespace from TD's data). Returns an error
// listing the available candidates when no match or >1 matches are found.
func (s *Service) ResolvePoolByName(ctx context.Context, profileName, name string) (ResourcePool, error) { ... }
```

`ResolvePoolByName` returns:
- 0 matches → `fmt.Errorf("resource pool %q not found (got %d pools)", name, len(pools))`. Optionally include up to 5 nearest names by simple substring match.
- >1 matches → error listing all matching IDs.

### 4.6 `internal/cli/time/report/flags.go`

Add `resourcePool` to `statusFlags`. Update validation: exactly one of the five selectors required (`--user`, `--manager`, `--account`, `--all`, `--resource-pool`).

### 4.7 `internal/cli/time/report/runner.go`

Update `peoplesvcAPI` interface to add `ResolvePoolByName`. Update `resolveUsers`:

```go
trueVal := true
const employeeLimit = 5000

case f.manager != "":
	// ... resolve mgrUID ...
	all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
		Employee: &trueVal,
		Limit:    employeeLimit,
	})
	// ... filter by ReportsToUID ...

case f.account != "":
	return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
		Employee:    &trueVal,
		AccountName: f.account,
		Limit:       employeeLimit,
	})

case f.all:
	return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
		Employee: &trueVal,
		Limit:    employeeLimit,
	})

case f.resourcePool != "":
	pool, err := deps.People.ResolvePoolByName(ctx, deps.Profile, f.resourcePool)
	if err != nil {
		return nil, err
	}
	all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
		Employee: &trueVal,
		Limit:    employeeLimit,
	})
	if err != nil {
		return nil, err
	}
	out := []domain.User{}
	for _, u := range all {
		if u.ResourcePoolID == pool.ID {
			out = append(out, u)
		}
	}
	return out, nil
```

`--user UIDs` remains unchanged (it doesn't use search).

### 4.8 `internal/cli/time/report/print.go`

Extend the JSON envelope `filterJSON`:

```go
type filterJSON struct {
	Selector     string   `json:"selector"`
	Users        []string `json:"users,omitempty"`
	Manager      string   `json:"manager,omitempty"`
	Account      string   `json:"account,omitempty"`
	ResourcePool string   `json:"resourcePool,omitempty"` // NEW
	From         string   `json:"from"`
	To           string   `json:"to"`
}
```

Update `selectorOf` to return `"resource-pool"` when `f.resourcePool != ""`.

### 4.9 `internal/cli/mcp` — `get_time_status_report`

Add `resource_pool` input parameter (string, optional). Pipe through `MCPInputs.ResourcePool` → `statusFlags.resourcePool`. Update validation to count it as the fifth selector.

### 4.10 Docs

- `README.md`: add `--resource-pool` to the status command flag list.
- `docs/USER_GUIDE.md` (or equivalent): example invocation `tdx time report status --resource-pool "ICT - DBP - Linux Platform Services LPS" --week 2026-04-12`.
- `CHANGELOG.md`: v0.9.1 entry covering both fixes.

---

## 5. Tests

- `TestSearchUsers_EmployeeFilter` in `peoplesvc/users_test.go`: assert wire body contains `"IsEmployee":true` when `filter.Employee=&true`.
- `TestSearchUsers_DecodesResourcePool` in `peoplesvc/users_test.go`: assert `ResourcePoolID` and trimmed `ResourcePoolName` round-trip through decode.
- `TestSearchPools_Decode` in `peoplesvc/pools_test.go`: assert pool list parses, trailing whitespace in `Name` is trimmed.
- `TestResolvePoolByName_Match`, `_NoMatch`, `_MultipleMatches` in `peoplesvc/pools_test.go`.
- `TestResolvePoolByName_TrimmingMatchesTabbedNames` — feed the wire fixture with `"Foo Pool\t"`, query `"Foo Pool"`, expect match.
- `TestRunner_ManagerSearchesByEmployee` in `runner_test.go`: assert `mockPeoplesvc.SearchUsers` invoked with `Employee != nil && *Employee == true` for `--manager me`.
- `TestRunner_ResourcePoolSelector` in `runner_test.go`: assert `ResolvePoolByName` is called and only users with matching `ResourcePoolID` end up in the result.
- `TestStatusFlags_ResourcePoolSelector` in `flags_test.go`: validation accepts `--resource-pool` as sole selector and rejects combinations.

---

## 6. Side-effect audit

| Concern | Result |
|---|---|
| `--user UID` flow | Unchanged; doesn't use search. |
| Other peoplesvc callers | None today — peoplesvc was added in v0.9.0 only for this feature. |
| `MaxResults=5000` cost | ~1080 results on UFL. Under 1 second. Negligible. |
| Account-name client-side filter | Still applies after `Employee=true` filters server-side. Combined behavior is what we want. |
| Pool-name lookup cost | One extra HTTP call (returns 76 rows on UFL). Single-digit ms. |
| Pool name trailing tabs | Trimmed on decode in both `wireUser.ResourcePoolName` and `wireResourcePool.Name`, so user-facing names are clean. |
| MCP backwards compat | New optional `resource_pool` input. Existing callers unaffected. |
| JSON envelope shape | Adds optional `filter.resourcePool` and a new selector value `"resource-pool"`. Schema name `tdx.v1.timeStatusReport` unchanged (additive). |

---

## 7. Out of scope

- `tdx people pools list` discovery command. Useful but the user can paste names from the TD UI.
- Looking up account ID by name to pass `AccountIDs` to TD (would make `--account` server-side). Useful follow-up but not blocking.
- Pagination for tenants larger than 5000 employees. UFL has 1080 — well under cap.
- Per-profile saved team list (the v0.9.0 fallback option B). Not needed once `--manager` works.
- Multi-pool support (TD models one pool per user; nothing to do).

---

## 8. Estimated work

6 commits:
1. Domain: `Employee *bool` on `UserFilter`, `ResourcePoolID/Name` on `User`.
2. peoplesvc: wire `IsEmployee` through types + users.go decode + tests.
3. peoplesvc: new `pools.go` (SearchPools + ResolvePoolByName) + tests.
4. Runner: set `Employee=&true` on existing selector searches + tests.
5. CLI/MCP: `--resource-pool` flag, validation, runner branch, JSON envelope, MCP input + tests.
6. Docs: README, USER_GUIDE, CHANGELOG; live verification on UFL; tag v0.9.1.

Inline execution.
