# Server-side `--account` filter + `tdx people pools list` discovery

**Date:** 2026-05-01
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** v0.9.1 follow-ups (out-of-scope items 2a + 2b from the v0.9.1 spec)
**Target tag:** v0.10.0 (minor — adds new top-level command parent)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | TD's `POST /api/accounts/search` returns all accounts when sent `{}`. `NameLike` is silently ignored (same anti-pattern as people/resource-pool search). Sample has 6404 accounts; payload is ~3 MB and sub-second. |
| Q2 | TD's `POST /api/people/search` **does** honor `AccountIDs`. Probe: 168 vs 1077 employees with vs without on the test tenant. So `--account` becomes server-side once we have the ID. |
| Q3 | Account resolution is client-side (fetch all 6404, exact case-insensitive match on `Name`). Exact match preserves existing UX semantics; ambiguous names error out. |
| Q4 | Accounts live in `peoplesvc` package alongside pools. They're TD entities related to the `/api/people` surface (each user has `DefaultAccountID`/`Name`); keeping them in one package mirrors the pools layout shipped in v0.9.1. |
| Q5 | Add new `tdx people` parent command. v0.9.1 introduced `peoplesvc` as plumbing only; this surfaces it as a discoverable command surface. First (and currently only) subcommand: `pools list`. |
| Q6 | `tdx people accounts list` is out of scope. 6404 rows is a lot to scroll, and `--account` is exact-match-only — discoverability is less valuable. Reconsider when we have a real use case. |

---

## 1. Goal

1. **2a — discovery:** `tdx people pools list` prints the resource-pool catalog so users can see exact pool names without hopping to the TD web UI.
2. **2b — correctness/efficiency:** `tdx time report status --account NAME` resolves `NAME` to an account ID and lets TD do the filtering server-side, instead of fetching every employee and filtering by `DefaultAccountName` in client code.

---

## 2. Findings (TD API probe, 2026-05-01)

- `POST /api/accounts/search` with `{}` returns 6404 rows on the test tenant. `NameLike` filter ignored.
- Single-account GET `/api/accounts/{id}` works but isn't useful for our flow (we have a name, not an ID).
- People-search `AccountIDs: [N]` IS honored (168 results when set, 1077 without on the test tenant — confirms server-side filtering works).
- Account names on the test tenant are like `"999999 (Sample Department)"` — numeric prefix + parenthesized human name. No trailing whitespace quirks observed (unlike resource pools).

---

## 3. Changes

### 3.1 `internal/svc/peoplesvc/accounts.go` (NEW)

```go
// Account is the public domain shape for a TD account/department.
type Account struct {
	ID              int
	Name            string
	IsActive        bool
	ParentID        int
	ParentName      string
	Code            string
	ManagerUID      string
	ManagerFullName string
}

func (s *Service) SearchAccounts(ctx context.Context, profileName string) ([]Account, error)
func (s *Service) ResolveAccountByName(ctx context.Context, profileName, name string) (Account, error)
```

`ResolveAccountByName` mirrors `ResolvePoolByName`:
- Empty name → error.
- Case-insensitive match (after trimming whitespace, in case TD has the same data quirk).
- 0 matches → `account %q not found among N accounts` error.
- >1 matches → `account %q is ambiguous (matched IDs %s)` error.

### 3.2 `internal/svc/peoplesvc/types.go`

Add wire shape:

```go
type wireAccount struct {
	ID              int    `json:"ID"`
	Name            string `json:"Name"`
	IsActive        bool   `json:"IsActive"`
	ParentID        int    `json:"ParentID,omitempty"`
	ParentName      string `json:"ParentName,omitempty"`
	Code            string `json:"Code"`
	ManagerUID      string `json:"ManagerUID"`
	ManagerFullName string `json:"ManagerFullName"`
}
```

### 3.3 `internal/cli/time/report/runner.go`

Update `peoplesvcAPI` interface to add `ResolveAccountByName`. Update `--account` branch:

```go
case f.account != "":
	acct, err := deps.People.ResolveAccountByName(ctx, deps.Profile, f.account)
	if err != nil {
		return nil, err
	}
	return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
		Employee:  &trueVal,
		AccountID: acct.ID,
		Limit:     employeeLimit,
	})
```

This drops the existing client-side `AccountName` post-filter (TD now filters by ID server-side). The `domain.UserFilter.AccountName` field stays for backwards compat but is no longer set by this branch.

### 3.4 `internal/svc/peoplesvc/users.go`

Drop the dead client-side `AccountName` post-filter loop:

```go
// REMOVE:
// if filter.AccountName != "" && u.AccountName != filter.AccountName { continue }
```

The `AccountName` field on `domain.UserFilter` was only ever consumed there. Delete it from the struct (no other callers per `grep`).

### 3.5 `internal/domain/user_filter.go`

Remove `AccountName` field (now unused after 3.4).

### 3.6 `internal/cli/people/people.go` (NEW) + `internal/cli/people/pools.go` (NEW)

```go
// internal/cli/people/people.go
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "people",
		Short: "Browse TD users, accounts, and resource pools",
	}
	cmd.AddCommand(newPoolsCmd())
	return cmd
}

// internal/cli/people/pools.go
func newPoolsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "pools", Short: "Resource pools"}
	cmd.AddCommand(newPoolsListCmd())
	return cmd
}
```

`tdx people pools list`:
- Default human table: `ID  NAME  MANAGER  REQ-APPROVAL  ACTIVE`.
- Sorted by `Name` ascending.
- `--json` emits envelope `{"schema":"tdx.v1.resourcePoolList","pools":[...]}`.
- `--profile` flag for non-default profiles.
- Tests cover: text output, JSON envelope, no-pools case (returns count 0 cleanly).

### 3.7 `internal/cli/root.go`

Wire the new parent:

```go
rootCmd.AddCommand(people.NewCmd())
```

### 3.8 Docs

- `README.md`: add `tdx people pools list` row to the commands table; mention that `--account` is now exact-match against TD's account name and benefits from server-side filtering.
- `docs/guide.md`: add a "People discovery" subsection covering `pools list`; update the report-status section's API call list to include `/api/accounts/search`.

---

## 4. Tests

- `peoplesvc/accounts_test.go` — `TestSearchAccounts_Decode`, `TestResolveAccountByName_{Match,NotFound,Ambiguous,EmptyName}`. Reuses the test harness pattern from `pools_test.go`.
- `cli/time/report/runner_test.go` — extend mock to record `ResolveAccountByName` calls; add `TestRunner_AccountSelectorResolvesToServerSideID` asserting that `--account NAME` invokes `ResolveAccountByName` once and passes `AccountID: <resolved>` to `SearchUsers` (and **not** `AccountName`).
- `cli/people/pools_test.go` — `TestPoolsList_TextOutput`, `TestPoolsList_JSONEnvelope`, `TestPoolsList_EmptyResult`. Use a stub `peoplesvcAPI` interface scoped to the people-cli package.

---

## 5. Side-effect audit

| Concern | Result |
|---|---|
| Existing `--account NAME` callers | Same UX. Lookup is still exact-match by name, just server-side now. Error messages get more specific (cite the account count, say "ambiguous" if applicable). |
| `domain.UserFilter.AccountName` removal | `grep` confirmed no other callers. Safe to drop. |
| Other `peoplesvc.SearchUsers` callers | None today (only the report runner uses it). |
| Account-search payload size | ~3 MB on the test tenant. One sub-second call per `--account` invocation. Negligible vs the per-user weekly report fan-out that follows. |
| MCP `get_time_status_report` | Unchanged surface — `account` input still takes a string name. Internal resolution happens transparently. |
| `tdx people` parent | New top-level verb. No collision with existing subcommands (`auth`, `completion`, `config`, `mcp`, `time`, `version`). |

---

## 6. Out of scope

- `tdx people accounts list` (6404 rows; `--account` is exact-match-only, so discoverability adds little value today).
- `tdx people users search` or similar people-discovery commands.
- Caching the accounts/pools list across invocations. Each `--account` run does one fresh fetch; under a second.
- Pagination for very large tenants. Sample has 6404 accounts in one shot; if a tenant ever returns truncated data we'll add `MaxResults` then.
- Refactoring `peoplesvc` into smaller packages (per-entity). Keep one package; entities are small and related.

---

## 7. Estimated work

5 commits:
1. peoplesvc: `accounts.go` + `wireAccount` + tests.
2. Runner: switch `--account` to server-side ID lookup; drop client-side `AccountName` filter; drop `domain.UserFilter.AccountName`.
3. CLI: new `tdx people` parent + `pools list` subcommand + tests.
4. Docs: README + guide updates.
5. Live verification on the test tenant; PR; merge; tag v0.10.0.

Inline execution.
