# Multi-value selectors for `tdx time report status`

**Date:** 2026-05-04
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Small feature + breaking change to CLI/MCP/JSON shape
**Target tag:** v0.14.0

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | `--manager`, `--account`, `--resource-pool` all become slice flags (`StringSliceVar`), matching `--user`'s existing shape. Both `--flag A --flag B` and `--flag A,B` are accepted. Singular CLI flag name preserved. |
| Q2 | Multi-X is **union** semantics: "direct reports of A *or* B", not intersection. The only sensible meaning. |
| Q3 | Selector types remain mutually exclusive (can't mix `--manager X --resource-pool Y`). Different selector types compose differently and would need explicit set-operation flags; out of scope here. |
| Q4 | JSON envelope and MCP inputs switch to plural arrays — `manager`/`account`/`resourcePool` → `managers`/`accounts`/`resourcePools`. Plural-only, no dual shape. Breaking. |
| Q5 | Schema name `tdx.v1.timeStatusReport` unchanged. The shape change for the filter block is the breaking part; the rest of the envelope is stable. |

---

## 1. Goal

Let users target multiple managers, accounts, or resource pools in one report invocation:

```
tdx time report status --manager me --manager uid-2 --week 2026-04-12
tdx time report status --resource-pool "Pool A" --resource-pool "Pool B" --week 2026-04-12
tdx time report status --account "Dept A" --account "Dept B" --week 2026-04-12
```

---

## 2. Surface

### CLI

```
--user UID         repeatable / comma-separated   (unchanged)
--manager UID      repeatable / comma-separated   (changed: was single-value)
--account NAME     repeatable / comma-separated   (changed)
--resource-pool NAME  repeatable / comma-separated   (changed)
```

`"me"` continues to be valid as a manager value and is resolved per-element via WhoAmI.

### JSON envelope (breaking)

Filter block before:

```json
"filter": {
  "selector": "manager",
  "manager": "me",
  ...
}
```

After:

```json
"filter": {
  "selector": "manager",
  "managers": ["me", "uid-2"],
  ...
}
```

Same for `account` → `accounts`, `resourcePool` → `resourcePools`. `users` is unchanged (already plural).

### MCP `get_time_status_report` (breaking)

```go
type getTimeStatusReportArgs struct {
    // ... unchanged ...
    UserUIDs      []string  // unchanged
    Managers      []string  // was Manager string
    Accounts      []string  // was Account string
    ResourcePools []string  // was ResourcePool string
    // ...
}
```

`MCPInputs` follows the same rename.

---

## 3. Implementation

### 3.1 `internal/cli/time/report/status.go`

```go
type statusFlags struct {
    // ... unchanged ...
    users         []string
    managers      []string  // was: manager string
    accounts      []string  // was: account string
    resourcePools []string  // was: resourcePool string
    all           bool
    // ...
}
```

Flag registration:

```go
cmd.Flags().StringSliceVar(&f.managers, "manager", nil, "manager UIDs (repeatable / comma-separated; \"me\" = authenticated user)")
cmd.Flags().StringSliceVar(&f.accounts, "account", nil, "account/department names (repeatable / comma-separated)")
cmd.Flags().StringSliceVar(&f.resourcePools, "resource-pool", nil, "TD resource pool names (repeatable / comma-separated)")
```

Selector validation: count distinct selector *types* in use (one type can have multiple values).

### 3.2 `internal/cli/time/report/runner.go`

`resolveUsers` branches:

**Manager:** resolve each (`"me"` → WhoAmI; cache the result for repeated `"me"`), build set of UIDs, fetch employees once with `IsEmployee=true`, keep rows where `ReportsToUID ∈ managerUIDs`.

**Account:** resolve each name → ID via `ResolveAccountByName`, build set, pass `AccountIDs: [N1, N2, ...]` to people search. TD honors the slice server-side (verified during v0.10.0 probes).

**Resource pool:** resolve each name → ID, fetch employees, keep rows where `ResourcePoolID ∈ poolIDs` (client-side; TD silently ignores `ResourcePoolIDs` per the existing reference memory).

Helper for the de-duplication: build a `map[int]struct{}` for IDs or `map[string]struct{}` for UIDs, walk results once.

`MCPInputs` plural rename + field plumbing in `RunForMCP`.

### 3.3 `internal/cli/time/report/print.go`

`filterJSON` rename:

```go
type filterJSON struct {
    Selector      string   `json:"selector"`
    Users         []string `json:"users,omitempty"`
    Managers      []string `json:"managers,omitempty"`      // renamed from Manager
    Accounts      []string `json:"accounts,omitempty"`      // renamed
    ResourcePools []string `json:"resourcePools,omitempty"` // renamed
    Incomplete    bool     `json:"incomplete,omitempty"`
    Threshold     float64  `json:"threshold,omitempty"`
    From          string   `json:"from"`
    To            string   `json:"to"`
}
```

`selectorOf` updates to check slice non-emptiness.

### 3.4 `internal/mcp/tools_report.go`

Args struct rename + `MCPInputs` mapping:

```go
type getTimeStatusReportArgs struct {
    // ... unchanged ...
    UserUIDs      []string `json:"userUIDs,omitempty"`
    Managers      []string `json:"managers,omitempty"`       // was: Manager string
    Accounts      []string `json:"accounts,omitempty"`       // was: Account string
    ResourcePools []string `json:"resourcePools,omitempty"`  // was: ResourcePool string
    // ...
}
```

### 3.5 Docs

- README: update flag list to mention slice semantics on the three selectors; add multi-manager and multi-pool example.
- `docs/guide.md`: same in the Time Status Report section.

---

## 4. Tests

- Runner:
  - `TestRunner_MultiManagerUnion` — two manager UIDs, three users (one reports to A, one to B, one to C); assert exactly the A-and-B users come through.
  - `TestRunner_MultiAccountServerSide` — assert `AccountIDs` slice in `lastFilter` carries both resolved IDs.
  - `TestRunner_MultiResourcePoolUnion` — two pool IDs, mix of users; assert union.
  - `TestRunner_DuplicateValuesDeduped` — `--manager me --manager me` doesn't double-fetch or duplicate output.
- Status:
  - `TestStatus_FlagsRegistered` — confirm slice types.
  - `TestStatus_MultiManagerValidationOK` — selector validation accepts the multi-value form.
  - `TestStatus_SelectorTypesStillMutex` — `--manager X --resource-pool Y` errors.
- Print:
  - `TestPrintJSON_PluralManagersField` — JSON has `managers: [...]`, not `manager: ...`.
  - `TestPrintJSON_OmitsUnusedSelectors` — only the active selector's plural field appears.

---

## 5. Side-effect audit

| Concern | Result |
|---|---|
| Existing `--manager me` (single value) | Continues to work; StringSliceVar accepts a single value. |
| `--user` flag | Unchanged; was already `StringSliceVar`. |
| MCP backwards compat | **Breaking.** Renames in args struct. Documented in release notes. |
| JSON envelope backwards compat | **Breaking.** `manager` → `managers` (etc) field rename. |
| Account-IDs server-side filter | Already accepts a slice (verified in v0.10.0 probe). No new endpoint behavior. |
| Resource-pool client-side filter | TD silently ignores `ResourcePoolIDs`; we keep client-side filtering with the new set. |
| Manager `"me"` resolution under multi-manager | Resolve `"me"` once via WhoAmI, then build the set including the resolved UID. |
| Mutex of selector types | Validation counts distinct selector types in use, not values. |

---

## 6. Out of scope

- Allowing different selector *types* to combine (`--manager X --resource-pool Y`). Different concern (intersection vs union ambiguity); future request if it comes up.
- A `--manager-from-file path` form for very large manager sets. YAGNI.
- Renaming the singular CLI flags to `--managers`/`--accounts`/`--resource-pools`. Cobra accepts repeated singular flags; matches `--user`. Singular wins for ergonomics.

---

## 7. Estimated work

3 commits, target tag v0.14.0:

1. **CLI + runner:** convert flags to slices; runner branches handle multi-value union; tests.
2. **JSON envelope + MCP:** plural field renames; tests.
3. **Docs + live verify on the test tenant + PR + tag.**

Inline execution.
