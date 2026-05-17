# Per-user threshold via WorkableHours Implementation Plan (v0.16.5)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `tdx time report status --incomplete` defaults to per-user `WorkableHours` thresholds (FT=40, PT=32, etc.) instead of a global 40. Explicit `--threshold N` still applies globally.

**Architecture:** Additive domain/wire change for `WorkableHours float64`. Runner gains a `thresholdSet bool` (populated from `cmd.Flags().Changed("threshold")` on the CLI side and from `Threshold > 0` on the MCP side) to distinguish "global mode" from "per-user mode". Each row's threshold is computed inline; JSON envelope grows `filter.thresholdMode` + per-row `threshold` for transparency.

**Tech Stack:** Go 1.26.2; cobra. No new dependencies.

**Spec:** [`docs/specs/2026-05-11-per-user-threshold-workable-hours.md`](../specs/2026-05-11-per-user-threshold-workable-hours.md)

---

## File Structure

After this plan completes:

```
internal/
├── domain/
│   ├── user.go                              # MODIFY: add WorkableHours float64
│   └── user_test.go                         # MODIFY: smoke test for the new field
├── svc/
│   └── peoplesvc/
│       ├── types.go                         # MODIFY: wireUser gains WorkableHours
│       ├── users.go                         # MODIFY: decodeUser plumbs it through
│       └── users_test.go                    # MODIFY: 1 new wire-decode test
└── cli/
    └── time/
        └── report/
            ├── status.go                    # MODIFY: populate thresholdSet from cmd.Flags().Changed()
            ├── runner.go                    # MODIFY: per-user threshold logic + JSON-shaped row data
            ├── print.go                     # MODIFY: per-row threshold + filter.thresholdMode in JSON envelope
            └── runner_test.go               # MODIFY: ~5 new tests
docs/
└── guide/
    └── time.md                              # MODIFY: document per-user behavior under tdx time report status
```

No new files. No README/tree changes.

## Branch + Versioning

- Branch: `per-user-threshold` (Task 0)
- Version: v0.16.5 (no source change; tagged after merge)

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. (Main is at `fa89233` v0.16.4.)

- [ ] **Step 2: Create branch**

```bash
git checkout -b per-user-threshold
```

Expected: `Switched to a new branch 'per-user-threshold'`.

---

## Task 1: Probe + domain + wire + decoder + tests

**Files:**
- Modify: `internal/domain/user.go`
- Modify: `internal/domain/user_test.go`
- Modify: `internal/svc/peoplesvc/types.go`
- Modify: `internal/svc/peoplesvc/users.go`
- Modify: `internal/svc/peoplesvc/users_test.go`

### Step 1: Probe TD live for `WorkableHours`

**Before writing wire types**, refresh auth and probe the actual response shape:

```bash
tdx auth login --sso   # refresh token; user may need to do this interactively
tdx auth status        # confirm valid
```

Then dump the field-list on the authenticated user:

```bash
TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
MY_UID=$(./tdx auth status 2>&1 | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' || curl -s -H "Authorization: Bearer $TOKEN" "https://demotemplate.teamdynamix.com/TDWebApi/api/auth/getuser" | python3 -c "import json,sys; print(json.load(sys.stdin)['UID'])")
curl -s -H "Authorization: Bearer $TOKEN" "https://demotemplate.teamdynamix.com/TDWebApi/api/people/$MY_UID" | python3 -c "
import json,sys
d = json.load(sys.stdin)
matches = {k: v for k, v in d.items() if any(s in k.lower() for s in ['workable','hours','capacit','expected'])}
print('candidate fields:', matches if matches else '(none — dump all field names below)')
if not matches:
    for k in sorted(d.keys()): print(' ', k)
"
```

**Decision tree based on probe result:**

- **Field is named `WorkableHours` and is a number:** proceed with the spec as-is.
- **Field has a different name (e.g. `WeeklyCapacity`, `WorkingHours`):** use the actual name in the wire tag below; everything else (domain field `WorkableHours`, runner logic) stays unchanged.
- **Field is completely absent on `/api/people/{uid}`:** check `POST /api/people/search` too. If still absent, escalate — this is a tenant-data issue, not an implementation issue. The feature can still ship: with `WorkableHours == 0` for every user, the per-user mode falls back to 40 for everyone, which is the current behavior. Document the gap and tag anyway; users on tenants with the field populated benefit, others see no regression.

Capture the field name and JSON type. Use them in Step 2.

### Step 2: Add `WorkableHours` to `domain.User`

Open `internal/domain/user.go`. Find `type User struct`. Add:

```go
// WorkableHours is the user's expected weekly hours from TD. 0.0 means
// unset/unknown; the time-report --incomplete filter falls back to a
// global 40 default when computing per-user thresholds.
WorkableHours float64 `json:"workableHours,omitempty"`
```

(Pick the placement that matches the surrounding fields' style.)

### Step 3: Domain smoke test

Append to `internal/domain/user_test.go`:

```go
func TestUserWorkableHoursZeroValue(t *testing.T) {
	var u User
	if u.WorkableHours != 0 {
		t.Errorf("zero value should be 0.0, got %v", u.WorkableHours)
	}
}

func TestUserWorkableHoursRoundTrip(t *testing.T) {
	u := User{UID: "u1", WorkableHours: 32.5}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var got User
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.WorkableHours != 32.5 {
		t.Errorf("round-trip lost value; got %v", got.WorkableHours)
	}
}
```

(`encoding/json` import: check; the test file likely already imports it for sibling tests. If not, add `"encoding/json"`.)

### Step 4: Add `WorkableHours` to `wireUser`

In `internal/svc/peoplesvc/types.go`, find `type wireUser struct`. Add (with the JSON tag name from your Step 1 probe — use `WorkableHours` if that's what TD returns; otherwise adapt):

```go
WorkableHours float64 `json:"WorkableHours,omitempty"`
```

### Step 5: Plumb through `decodeUser`

In `internal/svc/peoplesvc/users.go`, find `decodeUser` (the wireUser → domain.User mapper). Add the field to the returned `domain.User{...}` literal:

```go
return domain.User{
    // ... existing fields ...
    WorkableHours: w.WorkableHours,
}
```

(Preserve the existing field order; insert the new line at the end before the closing brace.)

### Step 6: Wire decoder test

In `internal/svc/peoplesvc/users_test.go`, find an existing httptest-based test for `GetUser` or `SearchUsers` and add a new one (or extend an existing one) to verify `WorkableHours` round-trips. Example new test:

```go
func TestGetUserDecodesWorkableHours(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"UID": "u1",
			"FullName": "Test User",
			"PrimaryEmail": "test@example.com",
			"WorkableHours": 32.0
		}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.GetUser(context.Background(), prof, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkableHours != 32.0 {
		t.Errorf("WorkableHours: got %v, want 32.0", got.WorkableHours)
	}
}
```

(`harness` is the existing test helper in peoplesvc — if the test file doesn't have it, look at how sibling tests construct a service; replicate that pattern.)

### Step 7: Verify

```bash
go build ./...
go test ./internal/domain/... ./internal/svc/peoplesvc/...
go vet ./internal/domain/... ./internal/svc/peoplesvc/...
gofmt -l internal/domain/ internal/svc/peoplesvc/
golangci-lint run ./internal/domain/... ./internal/svc/peoplesvc/...
```

All clean.

### Step 8: Commit

```bash
git add internal/domain/user.go internal/domain/user_test.go internal/svc/peoplesvc/types.go internal/svc/peoplesvc/users.go internal/svc/peoplesvc/users_test.go
git commit -m "feat(domain,peoplesvc): add User.WorkableHours field"
```

**No `Co-Authored-By:` trailer.**

---

## Task 2: Runner — per-user threshold logic

**Files:**
- Modify: `internal/cli/time/report/status.go` — populate `thresholdSet`
- Modify: `internal/cli/time/report/runner.go` — extend `statusFlags` + `MCPInputs` + replace the `--incomplete` filter block

### Step 1: Add `thresholdSet` to flag struct + MCP inputs

In `internal/cli/time/report/runner.go`, find `type statusFlags struct` (around line 18-30). Add a field:

```go
thresholdSet  bool   // true when --threshold was explicitly passed (CLI: cmd.Flags().Changed("threshold"); MCP: in.Threshold > 0)
```

Find `type MCPInputs struct` (around line 159-175). Add the same:

```go
ThresholdSet  bool   // mirror of statusFlags.thresholdSet — set by handler from `Threshold > 0`
```

In `RunForMCP` (around line 195), wire `ThresholdSet` into the constructed `statusFlags`:

```go
f := statusFlags{
    // ... existing fields ...
    threshold:     in.Threshold,
    thresholdSet:  in.ThresholdSet,
}
```

### Step 2: Update `status.go` cobra wiring to populate `thresholdSet`

In `internal/cli/time/report/status.go`, find the `RunE` function (around line 56-75). Just before constructing the `statusFlags` or before calling `runStatus`, set:

```go
f.thresholdSet = cmd.Flags().Changed("threshold")
```

Place it after the existing `if cmd.Flags().Changed("threshold") && !f.incomplete { ... }` validation block (which already exists at line 60). Use the same cobra check — but assign to the struct field this time so the runner can see it:

```go
if cmd.Flags().Changed("threshold") && !f.incomplete {
    return fmt.Errorf("--threshold requires --incomplete")
}
f.thresholdSet = cmd.Flags().Changed("threshold")
```

### Step 3: Replace the `--incomplete` filter block in runner

In `internal/cli/time/report/runner.go`, find the existing block at lines 125-143:

```go
// Apply --incomplete filter (keeps rows below threshold; drops
// permission-denied since we can't classify hours we couldn't read).
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

Replace with:

```go
// Apply --incomplete filter (keeps rows below threshold; drops
// permission-denied since we can't classify hours we couldn't read).
// Threshold mode:
//   - thresholdSet=true   → global f.threshold (or 40 if <= 0) for every row
//   - thresholdSet=false  → per-user from row.User.WorkableHours; falls
//                           back to defaultThresholdFallback (40) when 0
const defaultThresholdFallback = 40.0
if f.incomplete {
    globalThreshold := f.threshold
    if globalThreshold <= 0 {
        globalThreshold = defaultThresholdFallback
    }
    filtered := results[:0]
    for _, r := range results {
        if r.Status == domain.ReportStatus("permission-denied") {
            continue
        }
        var rowThreshold float64
        if f.thresholdSet {
            rowThreshold = globalThreshold
        } else if r.User.WorkableHours > 0 {
            rowThreshold = r.User.WorkableHours
        } else {
            rowThreshold = defaultThresholdFallback
        }
        if r.TotalHours() >= rowThreshold {
            continue
        }
        filtered = append(filtered, r)
    }
    results = filtered
}
```

### Step 4: Verify build

```bash
go build ./...
```

Expected: clean. The runner now uses `thresholdSet` and the per-user logic; tests in Task 3 will verify the behavior.

### Step 5: Commit

```bash
git add internal/cli/time/report/runner.go internal/cli/time/report/status.go
git commit -m "feat(time/report): per-user --incomplete threshold from User.WorkableHours"
```

**No `Co-Authored-By:` trailer.**

---

## Task 3: Tests for runner per-user logic

**Files:**
- Modify: `internal/cli/time/report/runner_test.go`

- [ ] **Step 1: Append new tests**

Look at how existing tests construct stub users (search `runner_test.go` for `domain.User{`). They likely build `[]domain.User` slices with FullName / UID / etc. Reuse the same shape.

```go
func TestRunStatusPerUserThresholdFromWorkableHours(t *testing.T) {
	// Alice 40h workable, logged 40h → not incomplete.
	// Bob   32h workable, logged 30h → incomplete.
	// Carol 40h workable, logged 35h → incomplete.
	deps := buildTestDepsWithUsersAndWeeks(...) // adapt to the existing test setup
	// Configure the people-svc stub to return users with WorkableHours set:
	//   Alice {WorkableHours: 40}, Bob {WorkableHours: 32}, Carol {WorkableHours: 40}
	// Configure time-svc to return week-reports with TotalMin = 40*60, 30*60, 35*60.
	// Run with --incomplete, no --threshold:

	rep, err := Run(...) // or RunForMCP; whichever the existing tests use
	if err != nil { t.Fatal(err) }

	// Assert: only Bob and Carol in the result. Alice (40>=40) is filtered.
	gotUIDs := []string{}
	for _, r := range rep.Rows {
		gotUIDs = append(gotUIDs, r.User.UID)
	}
	if !contains(gotUIDs, "bob") || !contains(gotUIDs, "carol") {
		t.Errorf("expected Bob and Carol in result; got %v", gotUIDs)
	}
	if contains(gotUIDs, "alice") {
		t.Errorf("Alice 40>=40 should be filtered; got %v", gotUIDs)
	}
}

func TestRunStatusPerUserThresholdFallbackTo40(t *testing.T) {
	// User with WorkableHours=0 and TotalHours=30 → falls back to 40 → 30<40 → incomplete (in result).
}

func TestRunStatusGlobalThresholdOverridesPerUser(t *testing.T) {
	// --threshold 20 set: Alice 40/40 → not incomplete (40>=20); Bob 32/30 → not incomplete (30>=20).
	// Both filtered. Result is empty.
}

func TestRunStatusIncompleteNotSetIgnoresThreshold(t *testing.T) {
	// --incomplete NOT set. Threshold flag is irrelevant; all rows pass through.
}
```

**IMPORTANT:** the exact test scaffolding (function names like `buildTestDepsWithUsersAndWeeks`, mock package names) varies based on the existing tests. Read 50-100 lines of `runner_test.go` first to understand the pattern, then write the new tests in that style. If the existing tests use a `mockPeoplesvc` struct with a list of users, add `WorkableHours` to those mock users.

If `Run` is the actual exported runner function — find it via `grep -n "^func Run\|^func RunForMCP" internal/cli/time/report/runner.go` — and use that.

- [ ] **Step 2: Run new tests**

```bash
go test ./internal/cli/time/report/... -run TestRunStatus -v
```

Expected: all 4 new tests pass.

- [ ] **Step 3: Full package verify**

```bash
go test ./internal/cli/time/report/...
go vet ./internal/cli/time/report/...
gofmt -l internal/cli/time/report/
golangci-lint run ./internal/cli/time/report/...
```

All clean.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/time/report/runner_test.go
git commit -m "test(time/report): per-user threshold filter behavior"
```

**No `Co-Authored-By:` trailer.**

---

## Task 4: JSON envelope — per-row threshold + filter.thresholdMode

**Files:**
- Modify: `internal/domain/time_status_report.go` (if WeekStatusRow needs a `Threshold float64` field for JSON output) — OR
- Modify: `internal/cli/time/report/print.go` (where JSON shaping happens)

Decide which file to modify based on whether the print layer wraps `domain.WeekStatusRow` in a print-specific JSON shape, or just marshals the domain struct directly.

- [ ] **Step 1: Inspect the existing JSON shape**

```bash
grep -n "json:" internal/domain/time_status_report.go | head
grep -nA 20 "func printJSON\|func renderJSON\|printJSONOutput" internal/cli/time/report/print.go | head -60
```

Three likely cases:
- **The domain struct is marshaled directly with its `json:` tags.** Then add `Threshold float64 \`json:"threshold,omitempty"\`` to `WeekStatusRow` and populate it in the runner during the `--incomplete` filter loop. This means `WeekStatusRow` needs to be mutable (it's a struct slice, fine) OR the runner builds a side-table mapping UID → threshold and `printJSON` re-attaches.
- **`print.go` defines its own shaped struct that mirrors `WeekStatusRow`.** Add `Threshold float64 \`json:"threshold,omitempty"\`` to that mirror struct, populate during the print loop.
- **Other.** Adapt.

For cleanliness, prefer modifying `print.go`'s output shape (or whatever struct gets marshaled). Avoid putting display-only fields on the canonical domain type if the existing pattern keeps them separate.

- [ ] **Step 2: Threshold mode at the filter level**

`print.go` already has a filter section in the JSON envelope (per existing `Threshold float64 \`json:"threshold,omitempty"\`` at line 68). Add a sibling field:

```go
ThresholdMode string `json:"thresholdMode,omitempty"`
```

Populate it in the function that builds the filter envelope:

```go
if f.incomplete {
    if f.thresholdSet {
        filter.ThresholdMode = "global"
        filter.Threshold = f.threshold
    } else {
        filter.ThresholdMode = "per-user"
        // Leave filter.Threshold empty in per-user mode (since each row has its own).
        // Or set it to the fallback (40); document the choice.
    }
}
```

Be explicit about whether `filter.Threshold` in per-user mode is omitted (cleaner) or set to the 40-fallback (more compatible with old consumers). Recommendation: **omit in per-user mode** — `json:"threshold,omitempty"` with float64=0 naturally omits. Document this in the user-facing doc.

- [ ] **Step 3: Per-row threshold**

In the runner's `--incomplete` filter loop (Task 2 Step 3), capture each row's threshold for downstream JSON. Two options:

**Option A** (preferred — minimal): attach the threshold to the JSON-shape struct in `print.go`'s row-rendering loop. The runner already computes `rowThreshold` for the filter check — the runner can emit a per-row threshold map (`map[string]float64` keyed by UID), and `print.go` reads it.

But that requires changing the function signature from `Run(...) (TimeStatusReport, error)` to something carrying the side-table. Cleaner: extend the `TimeStatusReport` domain struct to optionally carry a per-row threshold map. Or simplest:

**Option B**: store the threshold on `WeekStatusRow` itself by adding a non-tagged field that `print.go` uses, and not exposed in the JSON envelope from the domain struct directly. This is closest to a clean approach if WeekStatusRow is part of the domain layer's public API.

**Option C (cleanest)**: add `Threshold float64 \`json:"threshold,omitempty"\`` directly to `WeekStatusRow`. It's a display/computed field, populated only when `--incomplete` is on. Other consumers ignore it; it's omitempty so it doesn't pollute non-incomplete output.

Recommendation: **Option C**. One field on `WeekStatusRow`, populated in the runner's filter loop:

```go
// In the filter loop, before the >= comparison:
r.Threshold = rowThreshold
// (Then either filter-out or keep r as before.)
```

Make sure to write `r.Threshold` to the new `results[i]` slice element, not the iteration variable copy. The simplest pattern:

```go
for i := range results {
    r := &results[i]
    if r.Status == ... { continue }
    var rowThreshold float64
    // ... compute rowThreshold as in Task 2 ...
    r.Threshold = rowThreshold
    if r.TotalHours() >= rowThreshold {
        // mark for removal — track an index list
    }
}
```

Or build a new slice as before, but set `r.Threshold` before the `if r.TotalHours() >= rowThreshold` skip:

```go
filtered := results[:0]
for _, r := range results {
    if r.Status == domain.ReportStatus("permission-denied") { continue }
    var rowThreshold float64
    // ... compute ...
    r.Threshold = rowThreshold       // mutates the local copy; appended below
    if r.TotalHours() >= rowThreshold { continue }
    filtered = append(filtered, r)
}
results = filtered
```

(The `r` is a copy from the loop range; appending it carries the modified `Threshold`. This is correct in Go.)

Add to `internal/domain/time_status_report.go`:

```go
type WeekStatusRow struct {
    // ... existing fields ...
    Threshold float64 `json:"threshold,omitempty"`   // set during --incomplete filtering; 0 / omitted otherwise
}
```

- [ ] **Step 4: Test the JSON output**

Add to `internal/cli/time/report/runner_test.go` (or a separate `print_test.go` if one exists):

```go
func TestRunStatusJSONEnvelopeHasThresholdMode(t *testing.T) {
    // --incomplete with no --threshold; assert envelope has filter.thresholdMode == "per-user".
}

func TestRunStatusJSONEnvelopeGlobalMode(t *testing.T) {
    // --incomplete --threshold 30; assert envelope has filter.thresholdMode == "global" and filter.threshold == 30.
}

func TestRunStatusJSONEnvelopePerRowThreshold(t *testing.T) {
    // --incomplete with two users (one 32h workable, one 40h workable); assert each row's threshold field matches the WorkableHours.
}
```

Implementation: render the report via the JSON path (`printJSON` or similar), unmarshal, inspect.

- [ ] **Step 5: Verify**

```bash
go build ./...
go test ./internal/domain/... ./internal/cli/time/report/...
go vet ./...
gofmt -l internal/domain/ internal/cli/time/report/
golangci-lint run ./internal/domain/... ./internal/cli/time/report/...
```

All clean.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/time_status_report.go internal/cli/time/report/print.go internal/cli/time/report/runner.go internal/cli/time/report/runner_test.go
git commit -m "feat(time/report): JSON envelope gains thresholdMode + per-row threshold"
```

(Adjust the file list to only include files actually modified.)

**No `Co-Authored-By:` trailer.**

---

## Task 5: MCP — populate ThresholdSet from input

**Files:**
- Modify: `internal/mcp/tools_report.go` (or whatever file has the `get_time_status_report` handler)

- [ ] **Step 1: Find the MCP handler**

```bash
grep -n "get_time_status_report\|RunForMCP" internal/mcp/*.go | head -5
```

- [ ] **Step 2: Populate `ThresholdSet`**

In the handler that constructs `report.MCPInputs`, after assigning `Threshold: args.Threshold`, add:

```go
ThresholdSet: args.Threshold > 0,
```

(Per the spec: MCP can't distinguish "explicit 0" from "omitted" via JSON unmarshal, so `args.Threshold == 0` always means "use per-user / fallback". A user wanting "global 0" passes `--threshold 0` on the CLI, which is essentially a no-op anyway.)

- [ ] **Step 3: Update MCP tool description (optional)**

Add a line to the description noting the per-user behavior, so agents know what `threshold` omitted does.

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./internal/mcp/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tools_report.go
git commit -m "feat(mcp): get_time_status_report threshold=0 means per-user mode"
```

(Adjust file name if different.)

**No `Co-Authored-By:` trailer.**

---

## Task 6: Documentation

**Files:**
- Modify: `docs/guide/time.md`

- [ ] **Step 1: Find the `tdx time report status` section**

```bash
grep -n "tdx time report status\|--incomplete\|--threshold" docs/guide/time.md | head
```

- [ ] **Step 2: Update the prose**

Find the existing description of `--incomplete` / `--threshold`. Replace with:

```markdown
- `--incomplete` — filter to user-weeks below the threshold.
- `--threshold N` — explicit global threshold (hours). When omitted, each user's TD `WorkableHours` is used (e.g. 40 for FT, 32 for PT); falls back to 40 when `WorkableHours` is unset.

Examples:

```bash
# Per-user threshold (default — uses each user's WorkableHours):
tdx time report status --manager me --week 2026-04-19 --incomplete

# Global threshold of 20 hours for everyone:
tdx time report status --manager me --week 2026-04-19 --incomplete --threshold 20

# Restore pre-v0.16.5 behavior (global 40):
tdx time report status --manager me --week 2026-04-19 --incomplete --threshold 40
```

Output JSON envelope (when `--json` + `--incomplete`):

- Per-row `threshold` shows the value used for that row.
- `filter.thresholdMode` is `"per-user"` or `"global"`.
```

(Adapt to existing style — if the section uses tables vs bullets, match.)

- [ ] **Step 3: Verify**

```bash
grep -c "per-user\|WorkableHours" docs/guide/time.md
```

Should be > 0.

- [ ] **Step 4: Commit**

```bash
git add docs/guide/time.md
git commit -m "docs: per-user --threshold via WorkableHours under tdx time report status"
```

**No `Co-Authored-By:` trailer.**

---

## Task 7: Live verification + PR + release

**Files:** none modified — verification + git operations only.

- [ ] **Step 1: Pre-flight**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

All green.

- [ ] **Step 2: Build + auth check**

```bash
go build -o tdx ./cmd/tdx
./tdx auth status
```

`token: valid` required. If not, re-auth via `tdx auth login --sso`.

- [ ] **Step 3: Live-verify per-user mode**

```bash
# Run with --incomplete, no --threshold — expect per-user thresholds:
./tdx time report status --manager me --week 2026-05-04 --incomplete --json | jq '.filter, .rows[0] // {}'
```

Confirm:
- `filter.thresholdMode == "per-user"`
- At least one row has a non-zero `threshold` field that matches a known WorkableHours value (40, 32, etc.)

If no rows come back, the report data may not have incomplete users for that week — try a different week or a week-range query.

- [ ] **Step 4: Live-verify global override**

```bash
./tdx time report status --manager me --week 2026-05-04 --incomplete --threshold 20 --json | jq '.filter, (.rows | length)'
```

Confirm:
- `filter.thresholdMode == "global"`
- `filter.threshold == 20`
- Rows are filtered by < 20 hours.

- [ ] **Step 5: Push + open PR**

```bash
rm tdx 2>/dev/null
git push -u origin per-user-threshold
```

PR body to `/tmp/pr-body-v0.16.5.md`:

```markdown
## Summary

`tdx time report status --incomplete` now uses each user's TD `WorkableHours` (FT=40, PT=32, etc.) as the threshold by default, instead of a global 40-for-everyone. Explicit `--threshold N` keeps the existing global-override behavior. Closes a deferred item from v0.12.0.

### Backwards compat

Slightly breaking for users who relied on the no-flag default-40 behavior. To restore the pre-v0.16.5 behavior, pass `--threshold 40` explicitly. For most users this is strictly better — fewer false-positives for PT staff at the 40-default.

### Architecture

- New `domain.User.WorkableHours` field; `peoplesvc.wireUser` decodes it.
- Runner now distinguishes "global mode" (`--threshold N` set) from "per-user mode" via a new `thresholdSet` bool on `statusFlags` and `MCPInputs`.
- JSON envelope gains per-row `threshold` and filter-level `thresholdMode`.
- Fallback when `WorkableHours <= 0`: literal 40 default.

### Live-verified on the test tenant

- Per-user mode produces correct thresholds for staff with FT/PT WorkableHours
- Global override still works
- JSON envelope shape confirmed

Spec: `docs/specs/2026-05-11-per-user-threshold-workable-hours.md`
Plan: `docs/plans/2026-05-11-per-user-threshold-workable-hours.md`

## Test plan

- [x] Domain field round-trip
- [x] Wire decode test
- [x] Runner per-user logic (4 cases)
- [x] JSON envelope (thresholdMode + per-row threshold)
- [x] `go test ./... && go vet ./... && golangci-lint run ./...` clean
- [x] Live-verified per-user and global modes on the test tenant

After merge, tag `v0.16.5` to trigger Goreleaser.
```

```bash
gh pr create --title "v0.16.5: per-user --incomplete threshold via TD WorkableHours" --body-file /tmp/pr-body-v0.16.5.md
```

- [ ] **Step 6: Wait for CI; merge**

```bash
gh pr merge <PR#> --squash --admin --delete-branch
```

- [ ] **Step 7: Reset, tag, push tag**

```bash
git checkout main
git fetch origin
git reset --hard origin/main
git tag v0.16.5
git push origin v0.16.5
```

- [ ] **Step 8: Update memory**

`MEMORY.md` index → v0.16.5. `project_tdx_current_state.md` gets a new "Latest release" block. `project_tdx_backlog.md` marks "per-user threshold via WorkableHours" as shipped.

---

## Self-Review

**1. Spec coverage:**
- Spec § Domain types (`User.WorkableHours`) → Task 1
- Spec § Wire format (`wireUser.WorkableHours`) → Task 1
- Spec § Runner logic (per-user / global / fallback) → Task 2
- Spec § JSON envelope (thresholdMode + per-row threshold) → Task 4
- Spec § MCP semantics → Task 5
- Spec § Tests (domain + service + runner + JSON) → Tasks 1, 3, 4
- Spec § Documentation → Task 6
- Spec § Live verification → Task 7
- Spec § Acceptance criteria 1-8 → covered across all tasks; criteria 5 + 6 in Task 7

All requirements have a task.

**2. Placeholder scan:**
- Task 1 step 1 has a probe-decision-tree with concrete fallback strategies — that's adaptive directive, not vague TODO.
- Task 3 step 1 and Task 4 step 3 say "adapt to the existing test/file pattern" with grep hints. Concrete instructions, not placeholders.
- No "TBD" / "fill in details" anywhere.

**3. Type consistency:**
- `WorkableHours float64` on domain.User (Task 1) and wireUser (Task 1) — same type, same name, consistent through.
- `thresholdSet bool` on statusFlags (Task 2) and `MCPInputs.ThresholdSet bool` (Task 2 step 1, used in Task 5) — consistent.
- `Threshold float64` JSON tag `json:"threshold,omitempty"` on WeekStatusRow (Task 4) — consistent with existing `filter.Threshold`.
- `filter.thresholdMode string` (Task 4) — new field; documented in spec + docs.

All consistent.
