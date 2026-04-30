# `--manager` / `--account` / `--all` — apply `IsEmployee` filter

**Date:** 2026-04-30
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Bug fix follow-up to v0.9.0

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | TD's `/api/people/search` returns mostly Clients (portal end-users — 92K of 100K records on UFL), not staff. The `MustReportTime` and `ShouldReportTime` filters are silently ignored in the request body. Only `IsEmployee` is honored — and using it returns ~1080 records on UFL, matching the count visible in the TD UI's user filter. |
| Q2 | All three selectors (`--manager`, `--account`, `--all`) target time-reporting staff, not portal Clients. Hardcode `IsEmployee=true` on the search call for those paths. |
| Q3 | Bump default `MaxResults` for these paths to 5000 (1080 + headroom). Stays well under the 10K behavior cap and finishes in <1s. |
| Q4 | Expose `Employee *bool` on `domain.UserFilter` for caller control. Default behavior (`Employee == nil`) keeps the current "no filter" semantics, so external callers of `peoplesvc.SearchUsers` aren't surprised. |

---

## 1. Goal

Make `tdx time report status --manager me --week ...` return your actual direct reports (12 on UFL), not zero. Same fix works for `--account NAME` and `--all`.

---

## 2. Root cause

`runner.go`'s `--manager` branch:
```go
all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{Limit: hardLimit})
// filter all by ReportsToUID == mgrUID
```

`peoplesvc.SearchUsers` defaults `IsActive=true, UserType=User, MaxResults=100`. TD's "User" type includes Clients (92% of UFL's user records), so the alphabetical-first-100 are almost all Clients with self-referential ReportsToUID — and none of them match the user's UID.

Even bumping `MaxResults` to 100K returns only 5 of the user's 12 actual direct reports because TD caps the response and the alphabetical order can split direct reports across the cap.

`IsEmployee=true` filters to ~1080 staff, all 12 direct reports included.

---

## 3. Change

### 3.1 `internal/domain/user_filter.go`

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

### 3.2 `internal/svc/peoplesvc/types.go`

Add `IsEmployee *bool` to `wireUserSearch`:

```go
type wireUserSearch struct {
	NameLike   string `json:"NameLike,omitempty"`
	IsActive   *bool  `json:"IsActive,omitempty"`
	IsEmployee *bool  `json:"IsEmployee,omitempty"`  // NEW
	AccountIDs []int  `json:"AccountIDs,omitempty"`
	UserType   string `json:"UserType,omitempty"`
	MaxResults int    `json:"MaxResults,omitempty"`
}
```

### 3.3 `internal/svc/peoplesvc/users.go`

Wire `filter.Employee` into the request body:

```go
if filter.Employee != nil {
	req.IsEmployee = filter.Employee
}
```

### 3.4 `internal/cli/time/report/runner.go`

In `resolveUsers`, change the search calls for `--manager`, `--account`, `--all` to set `Employee` and a higher `Limit`:

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
```

`--user UIDs` remains unchanged (it doesn't use search).

---

## 4. Tests

- New `TestSearchUsers_EmployeeFilter` in `internal/svc/peoplesvc/users_test.go`: assert that when `filter.Employee=&true`, the wire body contains `"IsEmployee":true`.
- New `TestRunner_ManagerSearchesByEmployee` in `internal/cli/time/report/runner_test.go`: assert `mockPeoplesvc.SearchUsers` is invoked with `Employee != nil && *Employee == true` for `--manager me`.

---

## 5. Side-effect audit

| Concern | Result |
|---|---|
| `--user UID` flow | Unchanged; doesn't use search. |
| Other peoplesvc callers | None today — peoplesvc was added in v0.9.0 only for this feature. |
| `MaxResults=5000` cost | ~1080 results on UFL. Under 1 second. Negligible. |
| Account-name client-side filter | Still applies after `Employee=true` filters server-side. Combined behavior is what we want. |

---

## 6. Out of scope

- Looking up account ID by name to pass `AccountIDs` to TD (would make `--account` server-side). Useful follow-up but not blocking.
- Pagination for tenants larger than 5000 employees. UFL has 1080 — well under cap.
- Per-profile saved team list (the v0.9.0 fallback option B). Not needed once `--manager` works.

---

## 7. Estimated work

3 commits:
1. Domain: `Employee *bool` on `UserFilter`.
2. peoplesvc: wire `IsEmployee` through types + users.go + tests.
3. Runner: set `Employee=&true` on selector searches + tests + live verification.

Inline execution. Tag v0.9.1.
