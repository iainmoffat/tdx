# `--incomplete` filter for `tdx time report status`

**Date:** 2026-05-04
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Small feature on top of v0.9.0's Time Status Report
**Target tag:** v0.12.0 (minor — adds CLI flag, MCP input, JSON envelope filter field)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | Threshold is configurable via `--threshold N` (default 40). PT staff can pass `--threshold 32`. Per-user from TD's `WorkableHours` is a future option, not now. |
| Q2 | Threshold filter is **independent of submission status**. A `submitted` 38h holiday week WILL appear under `--incomplete` if 38 < threshold. Status filtering, if needed later, ships as its own flag (`--status open` or similar) — keep functionality separate. |
| Q3 | Permission-denied rows are dropped under the filter (we can't classify zero hours we couldn't read; including them is noise). Without `--incomplete`, current behavior unchanged. |
| Q4 | Filter applies to displayed rows; subtotals and OVERALL totals reflect the filtered set. Mirrors how `--include-zero=false` already works. |
| Q5 | Two flags, not one. `--incomplete` (bool) is the trigger; `--threshold N` (default 40) is the dial. `--threshold` set without `--incomplete` errors out — explicit beats implicit. |

---

## 1. Goal

Make `tdx time report status --manager me --week 2026-04-19 --incomplete` show only the direct reports who haven't logged a full week, so the user can chase down submissions without scanning the full team list.

---

## 2. Surface

### CLI

```
--incomplete       keep only rows whose totalHours < threshold (default 40)
                   and whose status is not permission-denied
--threshold N      hours threshold (default 40); requires --incomplete
```

Mutual exclusivity / dependency:
- `--threshold` set alone → error `"--threshold requires --incomplete"`.
- Both flags compose with all five selectors and with `--include-zero`.

### MCP

`get_time_status_report` gains:

```json
{
  "incomplete": false,
  "threshold": 40
}
```

`incomplete=true` is the trigger. `threshold` defaults to 40.

### JSON envelope

`filter` block additive:

```json
{
  "filter": {
    "selector": "manager",
    "manager": "me",
    "incomplete": true,
    "threshold": 40,
    "from": "2026-04-19",
    "to": "2026-04-25"
  },
  ...
}
```

`incomplete` and `threshold` fields appear only when the filter is active.

---

## 3. Implementation

### 3.1 `internal/cli/time/report/status.go`

Add to `statusFlags`:

```go
type statusFlags struct {
    // ... existing fields ...
    incomplete bool
    threshold  float64
}
```

Register flags:

```go
cmd.Flags().BoolVar(&f.incomplete, "incomplete", false, "filter to rows with totalHours < --threshold")
cmd.Flags().Float64Var(&f.threshold, "threshold", 40, "hours threshold for --incomplete (default 40)")
```

Add to `validateStatusFlags`:

```go
if !f.incomplete {
    // Threshold without --incomplete is meaningless. Reject when user
    // passed something other than the default 40 to surface the mistake.
    // (We can't tell "default" from "explicit 40" without flag.Changed,
    // so use cmd.Flags().Changed("threshold") in the cobra RunE wrapper.)
}
```

Detection of explicit `--threshold` without `--incomplete` happens at the cobra layer (uses `cmd.Flags().Changed("threshold")`):

```go
RunE: func(cmd *cobra.Command, args []string) error {
    if cmd.Flags().Changed("threshold") && !f.incomplete {
        return fmt.Errorf("--threshold requires --incomplete")
    }
    if err := validateStatusFlags(f); err != nil { return err }
    return runStatus(cmd, f)
},
```

### 3.2 `internal/cli/time/report/runner.go`

In `assembleReport`, after the existing `--include-zero` filter and before the sort:

```go
// Apply --incomplete filter.
if f.incomplete {
    threshold := f.threshold
    if threshold <= 0 {
        threshold = 40
    }
    filtered := results[:0]
    for _, r := range results {
        if r.Status == domain.ReportStatus("permission-denied") {
            continue
        }
        if r.TotalHours() >= threshold {
            continue
        }
        filtered = append(filtered, r)
    }
    results = filtered
}
```

`MCPInputs` gains `Incomplete bool` and `Threshold float64`. `RunForMCP` pipes them through.

### 3.3 `internal/cli/time/report/print.go`

Extend `filterJSON`:

```go
type filterJSON struct {
    // ... existing ...
    Incomplete bool    `json:"incomplete,omitempty"`
    Threshold  float64 `json:"threshold,omitempty"`
}
```

Populate when `f.incomplete`:

```go
if f.incomplete {
    filter.Incomplete = true
    filter.Threshold = f.threshold
    if filter.Threshold <= 0 {
        filter.Threshold = 40
    }
}
```

### 3.4 `internal/mcp/tools_report.go`

Extend args struct + pipe through:

```go
type getTimeStatusReportArgs struct {
    // ... existing ...
    Incomplete bool    `json:"incomplete,omitempty"`
    Threshold  float64 `json:"threshold,omitempty"`
}
```

### 3.5 Docs

- `README.md`: add `--incomplete`/`--threshold` to the status row and to the status section's example block.
- `docs/guide.md`: add a one-paragraph "Incomplete filter" subsection under the Time Status Report section, plus a concrete example.

---

## 4. Tests

- `runner_test.go`:
  - `TestRunner_IncompleteFiltersBelowThreshold` — three users (38, 40, 42 hours), `--incomplete` keeps the 38h row.
  - `TestRunner_IncompleteWithCustomThreshold` — `--threshold 32`, kept rows are < 32.
  - `TestRunner_IncompleteDropsPermissionDenied` — a permission-denied row is filtered out, even though its TotalHours = 0.
  - `TestRunner_IncompleteFiltersAffectTotals` — overall totals reflect the filtered set (sum of just incomplete rows).
- `status_test.go`:
  - `TestStatus_ThresholdRequiresIncomplete` — passing `--threshold 32` alone errors.
  - `TestStatus_FlagsRegistered` — adds `incomplete`, `threshold`.
- `print_test.go` (or where envelope is tested): JSON envelope includes `incomplete` and `threshold` when set, omits them when not.

---

## 5. Side-effect audit

| Concern | Result |
|---|---|
| Existing CLI consumers | No change unless `--incomplete` is passed. |
| `--include-zero=false` interaction | `--include-zero` runs before `--incomplete`. A user-week with totalHours=0 is dropped by `--include-zero`, so `--incomplete` never sees it. Pass `--include-zero` to combine: zero-hour rows count as incomplete iff also visible. |
| Selector compatibility | All five selectors (`--user`/`--manager`/`--account`/`--resource-pool`/`--all`) compose with the new filter. |
| Output format compatibility | Filter applies at the row level before format dispatch; text/JSON/CSV/XLSX all respect it without per-format code. |
| MCP backwards compat | New optional inputs default to `false` / `0`. Existing callers unaffected. |
| JSON envelope schema | Schema name `tdx.v1.timeStatusReport` unchanged. New optional `filter.incomplete` and `filter.threshold` fields are additive. |

---

## 6. Out of scope

- Per-user thresholds via TD's `WorkableHours` field. Useful when teams mix FT/PT but adds complexity (e.g. how to display "incomplete vs Sarah's threshold of 32"). Defer to v0.13.0 if requested.
- A `--status open` filter (excludes already-submitted weeks). Separate flag, separate concern; the user explicitly asked to keep these separate.
- Color/marker for incomplete rows in non-filtered runs (e.g. always show a `⚠` next to <40h rows). Could come later.
- Slack/email notification hooks for incomplete weeks. Out of scope for this CLI; that's an automation layer above tdx.

---

## 7. Estimated work

3 commits, target tag v0.12.0:

1. **CLI + runner:** add flags, validation, filter loop in `assembleReport`; tests.
2. **JSON envelope + MCP:** plumb through `print.go` and `tools_report.go`; tests.
3. **Docs + live verify on UFL + PR + tag.**

Inline execution.
