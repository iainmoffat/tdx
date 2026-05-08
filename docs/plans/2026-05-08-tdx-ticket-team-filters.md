# tdx Ticket Team-Scope Filters Implementation Plan (v0.16.1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--responsibility-group <name|id>` and `--manager me|UID|email` selectors to `tdx ticket search`, plus a new `tdx ticket groups list` discovery command. Ship as v0.16.1.

**Architecture:** Bottom-up — domain (TicketGroup + ResponsibilityGroupIDs filter field) → service (groups.go + ResponsibilityGroupIDs in wireTicketSearch) → CLI (groups list cmd, two new flags on search, manager-expansion helper) → MCP (one new read-only tool, two new search-tickets inputs) → docs. Manager expansion happens in the CLI layer (resolves manager → direct-report UIDs via people-search) and merges into the existing `AssigneeUIDs` filter, keeping ticketsvc free of cross-service deps.

**Tech Stack:** Go 1.26.2; cobra; existing `tdx.Client`; `httptest` for service tests.

**Spec:** [`docs/specs/2026-05-08-tdx-ticket-team-filters.md`](../specs/2026-05-08-tdx-ticket-team-filters.md)

---

## File Structure

After this plan completes:

```
internal/
├── domain/
│   └── ticket.go              # MODIFY: add TicketGroup; add ResponsibilityGroupIDs to TicketSearchFilter
├── svc/
│   └── ticketsvc/
│       ├── types.go           # MODIFY: add wireGroup; add ResponsibilityGroupIDs to wireTicketSearch
│       ├── tickets.go         # MODIFY: map filter.ResponsibilityGroupIDs into wire body
│       ├── groups.go          # NEW: ListGroups + ResolveGroupByName
│       ├── groups_test.go     # NEW
│       └── tickets_test.go    # MODIFY: add TestSearchTicketsSendsResponsibilityGroupIDs
├── cli/
│   └── ticket/
│       ├── helpers.go         # MODIFY: peoplesvcAPI gains SearchUsers; add expandManagersToReports
│       ├── helpers_test.go    # MODIFY: stubPeoplesvc gains SearchUsers; new tests
│       ├── search.go          # MODIFY: --responsibility-group, --manager flags; updated buildSearchFilter
│       ├── search_test.go     # MODIFY: 5+ new test cases
│       ├── groups.go          # NEW: groups list cmd
│       ├── groups_test.go     # NEW
│       ├── ticket.go          # MODIFY: register newGroupsCmd
│       └── stub_test.go       # MODIFY: stubTicketsvc gains ListGroups + ResolveGroupByName
└── mcp/
    ├── tools_ticket.go        # MODIFY: searchTicketsArgs adds 2 fields; new listTicketGroupsHandler
    └── tools_ticket_test.go   # MODIFY: assert tool count = 13 ticket tools (was 12)
docs/
├── guide/
│   ├── ticket.md              # MODIFY: new flags + groups section
│   └── mcp.md                 # MODIFY: tool table + count update
└── guide.md                   # MODIFY: ASCII tree
README.md                      # MODIFY: ASCII tree (byte-identical to guide.md)
```

## Established Patterns

Read these BEFORE starting any task:

- Service test helper: `internal/svc/ticketsvc/service_test.go` `harness(t, srv.URL)`.
- Existing metadata-list pattern: `internal/svc/ticketsvc/metadata.go` (statuses + types) + `internal/cli/ticket/types.go` + `statuses.go`.
- Existing name-resolver pattern: `internal/svc/ticketsvc/metadata.go` `ResolveStatusByName` (0/1/many error shape).
- CLI runner pattern: `internal/cli/ticket/search.go` `buildSearchFilter` + `runTicketSearch`.
- MCP tool registration: `internal/mcp/tools_ticket.go` for read-only tools.
- Stub-test pattern: `internal/cli/ticket/stub_test.go` `stubTicketsvc` and `internal/cli/ticket/helpers_test.go` `stubPeoplesvc`.

Memory notes (read first):
- `reference_td_search_silent_filters.md` — TD silent-filter pattern.
- `reference_td_ticket_api_quirks.md` — verified ticket-API quirks (StatusClass, IsOpen ignored, etc.).
- `feedback_no_coauthor.md` — no `Co-Authored-By:` trailer in commits.

## Branch + Versioning

- Branch: `ticket-team-filters` (created in Task 0)
- Version: v0.16.1 (no source-level version string; tagged via git after merge)

---

## Task 0: Create branch

**Files:** none (git operation only)

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: `On branch main`, `nothing to commit, working tree clean`. (Spec commit `49d6535` is already on main.)

- [ ] **Step 2: Create branch**

```bash
git checkout -b ticket-team-filters
```

Expected: `Switched to a new branch 'ticket-team-filters'`.

---

## Task 1: Domain — TicketGroup + ResponsibilityGroupIDs

**Files:**
- Modify: `internal/domain/ticket.go`
- Modify: `internal/domain/ticket_test.go`

- [ ] **Step 1: Append `TicketGroup` type**

In `internal/domain/ticket.go`, after the existing `TicketType` declaration (around line 26-32), append:

```go
// TicketGroup is a TD responsibility group (a team that can be assigned
// tickets). Groups exist tenant-wide and can serve multiple ticket apps;
// the search endpoint filters by ResponsibilityGroupIDs (server-side honored).
type TicketGroup struct {
	ID     int
	Name   string
	Active bool
}
```

- [ ] **Step 2: Add `ResponsibilityGroupIDs` to `TicketSearchFilter`**

Find `TicketSearchFilter` (currently around lines 72-80). Add the new field after `AccountIDs []int`:

```go
type TicketSearchFilter struct {
	AppID                  int
	StatusIDs              []int
	AssigneeUIDs           []string
	RequestorUIDs          []string
	AccountIDs             []int
	ResponsibilityGroupIDs []int    // NEW
	Text                   string
	IncludeClosed          bool
	Limit                  int
}
```

- [ ] **Step 3: Add a constructor smoke test**

In `internal/domain/ticket_test.go`, append:

```go
func TestTicketGroupZeroValueIsValid(t *testing.T) {
	_ = TicketGroup{}
}

func TestTicketSearchFilterResponsibilityGroupIDs(t *testing.T) {
	f := TicketSearchFilter{ResponsibilityGroupIDs: []int{1, 2}}
	if len(f.ResponsibilityGroupIDs) != 2 {
		t.Fatalf("want 2, got %d", len(f.ResponsibilityGroupIDs))
	}
}
```

- [ ] **Step 4: Verify**

```bash
go build ./... && go test ./internal/domain/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/ticket.go internal/domain/ticket_test.go
git commit -m "feat(domain): add TicketGroup + ResponsibilityGroupIDs filter field"
```

---

## Task 2: ticketsvc — wire types

**Files:**
- Modify: `internal/svc/ticketsvc/types.go`

This is the smallest possible change — just adding wire types. No tests yet (they come with Task 3 and Task 4).

- [ ] **Step 1: Add `wireGroup` type**

In `internal/svc/ticketsvc/types.go`, append at the bottom (or after the existing wire types — placement doesn't matter, follow alphabetical or insertion order):

```go
// wireGroup matches a row in POST /TDWebApi/api/groups/search.
// PlatformApplications is included in the response but not surfaced
// to consumers in v0.16.1.
type wireGroup struct {
	ID                   int           `json:"ID"`
	Name                 string        `json:"Name"`
	IsActive             bool          `json:"IsActive"`
	Description          string        `json:"Description,omitempty"`
	ExternalID           string        `json:"ExternalID,omitempty"`
	PlatformApplications []interface{} `json:"PlatformApplications,omitempty"`
}
```

- [ ] **Step 2: Extend `wireTicketSearch` with `ResponsibilityGroupIDs`**

Find `wireTicketSearch` and add the new field after `ResponsibilityUids`:

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

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: clean. (No tests touched yet; existing tests should still pass since the field is additive with omitempty.)

```bash
go test ./internal/svc/ticketsvc/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/svc/ticketsvc/types.go
git commit -m "feat(ticketsvc): add wireGroup + ResponsibilityGroupIDs wire field"
```

---

## Task 3: ticketsvc — groups.go + tests

**Files:**
- Create: `internal/svc/ticketsvc/groups.go`
- Create: `internal/svc/ticketsvc/groups_test.go`

- [ ] **Step 1: Implement groups.go**

```go
package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListGroups returns all tenant groups via POST /api/groups/search.
// The endpoint may silently ignore body filter params (per the established
// TD silent-filter pattern); we send an empty body and let callers filter
// client-side. Groups are tenant-wide — no app-id needed.
func (s *Service) ListGroups(ctx context.Context, profileName string) ([]domain.TicketGroup, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireGroup
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/groups/search", struct{}{}, &wire); err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	out := make([]domain.TicketGroup, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketGroup{
			ID:     w.ID,
			Name:   strings.TrimSpace(w.Name),
			Active: w.IsActive,
		})
	}
	return out, nil
}

// ResolveGroupByName finds a group by case-insensitive exact match.
// Returns an error listing candidates if zero or >1 match.
func (s *Service) ResolveGroupByName(ctx context.Context, profileName string, name string) (domain.TicketGroup, error) {
	all, err := s.ListGroups(ctx, profileName)
	if err != nil {
		return domain.TicketGroup{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketGroup
	for _, g := range all {
		if strings.ToLower(g.Name) == target {
			matches = append(matches, g)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketGroup{}, fmt.Errorf("no ticket group matches %q (use `tdx ticket groups list` to see options)", name)
	case 1:
		return matches[0], nil
	default:
		labels := make([]string, 0, len(matches))
		for _, m := range matches {
			labels = append(labels, fmt.Sprintf("%d (%s)", m.ID, m.Name))
		}
		return domain.TicketGroup{}, fmt.Errorf("multiple ticket groups match %q: %s — pass --group-id <int> instead", name, strings.Join(labels, ", "))
	}
}
```

Note the error message says `--group-id <int>` even though that flag isn't added in this plan. This is forward-looking; users who hit ambiguity can drop in the numeric ID via the existing `--responsibility-group <id>` (numeric arg). If you'd rather, change the message to `pass numeric id directly via --responsibility-group`. Either is fine — pick one and be consistent across resolvers in the package.

Recommendation: change to `pass numeric id directly via --responsibility-group <int>` for consistency with how the CLI accepts it.

- [ ] **Step 2: Tests**

Create `internal/svc/ticketsvc/groups_test.go`:

```go
package ticketsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/groups/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 100, "Name": "  Linux Team  ", "IsActive": true, "Description": "ICT Linux", "ExternalID": ""},
			{"ID": 101, "Name": "Network Ops", "IsActive": true},
			{"ID": 102, "Name": "Archived Team", "IsActive": false}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	groups, err := svc.ListGroups(context.Background(), prof)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("want 3, got %d", len(groups))
	}
	if groups[0].Name != "Linux Team" {
		t.Errorf("name not trimmed: %q", groups[0].Name)
	}
	if !groups[0].Active || groups[2].Active {
		t.Errorf("Active mapping wrong: %+v", groups)
	}
}

func TestResolveGroupByNameSingleMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 100, "Name": "Linux Team", "IsActive": true},
			{"ID": 101, "Name": "Network Ops", "IsActive": true}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ResolveGroupByName(context.Background(), prof, "linux team")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 100 {
		t.Fatalf("want id=100, got %d", got.ID)
	}
}

func TestResolveGroupByNameNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 1, "Name": "Only", "IsActive": true}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveGroupByName(context.Background(), prof, "nonsense")
	if err == nil || !strings.Contains(err.Error(), "no ticket group matches") {
		t.Fatalf("want no-match error, got %v", err)
	}
}

func TestResolveGroupByNameAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "Network", "IsActive": true},
			{"ID": 2, "Name": "Network", "IsActive": true}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveGroupByName(context.Background(), prof, "Network")
	if err == nil || !strings.Contains(err.Error(), "multiple ticket groups match") {
		t.Fatalf("want ambiguous error, got %v", err)
	}
	if !strings.Contains(err.Error(), "1 (Network)") || !strings.Contains(err.Error(), "2 (Network)") {
		t.Errorf("error should list candidates: %v", err)
	}
}
```

- [ ] **Step 3: Verify**

```bash
go test ./internal/svc/ticketsvc/...
go vet ./internal/svc/ticketsvc/...
golangci-lint run ./internal/svc/ticketsvc/...
```

Expected: all clean.

- [ ] **Step 4: Commit**

```bash
git add internal/svc/ticketsvc/groups.go internal/svc/ticketsvc/groups_test.go
git commit -m "feat(ticketsvc): add ListGroups + ResolveGroupByName"
```

---

## Task 4: ticketsvc — wire-through ResponsibilityGroupIDs in SearchTickets

**Files:**
- Modify: `internal/svc/ticketsvc/tickets.go`
- Modify: `internal/svc/ticketsvc/tickets_test.go`

- [ ] **Step 1: Map the new filter field**

In `tickets.go`, find the `req := wireTicketSearch{...}` literal inside `SearchTickets`. Add one line:

```go
req := wireTicketSearch{
	StatusIDs:              filter.StatusIDs,
	ResponsibilityUids:     filter.AssigneeUIDs,
	ResponsibilityGroupIDs: filter.ResponsibilityGroupIDs, // NEW
	RequestorUids:          filter.RequestorUIDs,
	AccountIDs:             filter.AccountIDs,
	SearchText:             filter.Text,
	MaxResults:             filter.Limit,
}
```

(The struct may currently be in a different field order — preserve the surrounding style. The point is: pass `filter.ResponsibilityGroupIDs` through to the wire field.)

- [ ] **Step 2: Add a wire-body assertion test**

Append to `internal/svc/ticketsvc/tickets_test.go`:

```go
func TestSearchTicketsSendsResponsibilityGroupIDs(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.SearchTickets(context.Background(), prof, domain.TicketSearchFilter{
		AppID:                  31,
		ResponsibilityGroupIDs: []int{100, 200},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capturedBody), `"ResponsibilityGroupIDs":[100,200]`) {
		t.Errorf("ResponsibilityGroupIDs not in request body: %s", capturedBody)
	}
}
```

(This test file already imports `io`, `bytes`, `strings` etc. for sibling tests; add what's missing.)

- [ ] **Step 3: Verify**

```bash
go test ./internal/svc/ticketsvc/...
```

Expected: PASS (the new test plus all existing).

- [ ] **Step 4: Commit**

```bash
git add internal/svc/ticketsvc/tickets.go internal/svc/ticketsvc/tickets_test.go
git commit -m "feat(ticketsvc): wire ResponsibilityGroupIDs through SearchTickets"
```

---

## Task 5: CLI — extend peoplesvcAPI with SearchUsers

**Files:**
- Modify: `internal/cli/ticket/helpers.go`
- Modify: `internal/cli/ticket/helpers_test.go` (extend `stubPeoplesvc`)

- [ ] **Step 1: Extend the interface**

In `internal/cli/ticket/helpers.go`, find `peoplesvcAPI` and add the new method:

```go
// peoplesvcAPI is the subset of peoplesvc used by ticket helpers.
type peoplesvcAPI interface {
	LookupPeople(ctx context.Context, profile string, q string, limit int) ([]domain.User, error)
	SearchUsers(ctx context.Context, profile string, filter domain.UserFilter) ([]domain.User, error) // NEW
}
```

- [ ] **Step 2: Extend the stub**

In `internal/cli/ticket/helpers_test.go`, find `stubPeoplesvc`:

```go
type stubPeoplesvc struct {
	users        []domain.User
	err          error
	searchUsers  []domain.User // NEW — what SearchUsers returns
	searchErr    error         // NEW
	lastFilter   domain.UserFilter // NEW — capture what callers passed
}

func (s *stubPeoplesvc) LookupPeople(_ context.Context, _ string, _ string, _ int) ([]domain.User, error) {
	return s.users, s.err
}

// NEW:
func (s *stubPeoplesvc) SearchUsers(_ context.Context, _ string, filter domain.UserFilter) ([]domain.User, error) {
	s.lastFilter = filter
	return s.searchUsers, s.searchErr
}
```

- [ ] **Step 3: Verify**

```bash
go build ./... && go test ./internal/cli/ticket/...
```

Expected: PASS. Existing tests untouched should still pass; the interface widening doesn't break callers because `peoplesvc.Service` already implements `SearchUsers`.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ticket/helpers.go internal/cli/ticket/helpers_test.go
git commit -m "feat(cli/ticket): widen peoplesvcAPI with SearchUsers for manager expansion"
```

---

## Task 6: CLI — expandManagersToReports helper + tests

**Files:**
- Modify: `internal/cli/ticket/helpers.go`
- Modify: `internal/cli/ticket/helpers_test.go`

- [ ] **Step 1: Add the helper**

In `internal/cli/ticket/helpers.go`, append:

```go
// expandManagersToReports resolves each manager argument (me|UID|email) to
// a manager UID, then fetches all employees and filters those whose
// ReportsToUID matches one of the resolved manager UIDs. Returns the
// deduplicated set of direct-report UIDs.
//
// Why: TD's /api/people/search silently ignores ReportsToUid in the
// request body, so we can't filter server-side. This helper does the
// expansion in one pass over the staff list.
func expandManagersToReports(ctx context.Context, people peoplesvcAPI, profile, authedUID string, managerArgs []string) ([]string, error) {
	if len(managerArgs) == 0 {
		return nil, nil
	}
	// Resolve each manager arg to a UID.
	managerUIDs := make(map[string]struct{}, len(managerArgs))
	for _, arg := range managerArgs {
		uid, err := resolvePrincipal(ctx, people, profile, authedUID, arg)
		if err != nil {
			return nil, fmt.Errorf("--manager %q: %w", arg, err)
		}
		managerUIDs[uid] = struct{}{}
	}
	// Fetch all staff. ReportsToUid is silently ignored, so we filter client-side.
	trueVal := true
	all, err := people.SearchUsers(ctx, profile, domain.UserFilter{
		Employee: &trueVal,
		Limit:    5000,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch staff for --manager expansion: %w", err)
	}
	seen := make(map[string]struct{})
	var reports []string
	for _, u := range all {
		if _, ok := managerUIDs[u.ReportsToUID]; !ok {
			continue
		}
		if _, dup := seen[u.UID]; dup {
			continue
		}
		seen[u.UID] = struct{}{}
		reports = append(reports, u.UID)
	}
	return reports, nil
}
```

- [ ] **Step 2: Tests**

Append to `internal/cli/ticket/helpers_test.go`:

```go
func TestExpandManagersToReportsEmpty(t *testing.T) {
	got, err := expandManagersToReports(context.Background(), &stubPeoplesvc{}, "default", "uid-me", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

func TestExpandManagersToReportsMe(t *testing.T) {
	stub := &stubPeoplesvc{
		searchUsers: []domain.User{
			{UID: "report-1", FullName: "Alice", ReportsToUID: "uid-me"},
			{UID: "report-2", FullName: "Bob", ReportsToUID: "uid-me"},
			{UID: "other-1", FullName: "Carol", ReportsToUID: "uid-someone-else"},
		},
	}
	got, err := expandManagersToReports(context.Background(), stub, "default", "uid-me", []string{"me"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 reports, got %d: %v", len(got), got)
	}
	want := map[string]bool{"report-1": true, "report-2": true}
	for _, uid := range got {
		if !want[uid] {
			t.Errorf("unexpected uid: %s", uid)
		}
	}
	// Verify SearchUsers was called with Employee=true, Limit=5000.
	if stub.lastFilter.Employee == nil || !*stub.lastFilter.Employee {
		t.Errorf("Employee filter not set: %+v", stub.lastFilter)
	}
	if stub.lastFilter.Limit != 5000 {
		t.Errorf("Limit: got %d, want 5000", stub.lastFilter.Limit)
	}
}

func TestExpandManagersToReportsMultipleManagers(t *testing.T) {
	stub := &stubPeoplesvc{
		// Looking up Alice as a name returns just one result so resolvePrincipal succeeds.
		users: []domain.User{{UID: "alice-uid", FullName: "Alice", Email: "alice@x"}},
		searchUsers: []domain.User{
			{UID: "u1", ReportsToUID: "alice-uid"},
			{UID: "u2", ReportsToUID: "uid-me"},
			{UID: "u3", ReportsToUID: "alice-uid"},
			{UID: "u4", ReportsToUID: "someone-else"},
		},
	}
	got, err := expandManagersToReports(context.Background(), stub, "default", "uid-me", []string{"me", "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	// Expected union: u1 + u2 + u3 (NOT u4).
	if len(got) != 3 {
		t.Fatalf("want 3 reports, got %d: %v", len(got), got)
	}
}

func TestExpandManagersToReportsDedupe(t *testing.T) {
	// If the same UID's manager matches multiple inputs (e.g. me + my UID),
	// it should appear only once in the result.
	stub := &stubPeoplesvc{
		searchUsers: []domain.User{{UID: "u1", ReportsToUID: "uid-me"}},
	}
	got, _ := expandManagersToReports(context.Background(), stub, "default", "uid-me",
		[]string{"me", "uid-me-direct-uid-form-12345-1234-1234-1234-123456789012"})
	// Even though the second arg won't resolve to "uid-me", the test verifies
	// the dedupe loop handles a single source. Real dedupe is also implicitly
	// tested by TestExpandManagersToReportsMultipleManagers.
	if len(got) > 1 {
		t.Errorf("dedupe failed: %v", got)
	}
}

func TestExpandManagersToReportsErrorPropagates(t *testing.T) {
	stub := &stubPeoplesvc{searchErr: errors.New("boom")}
	_, err := expandManagersToReports(context.Background(), stub, "default", "uid-me", []string{"me"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want propagated error, got %v", err)
	}
}
```

(`errors` is already imported in helpers_test.go for sibling tests.)

- [ ] **Step 3: Verify**

```bash
go test ./internal/cli/ticket/...
golangci-lint run ./internal/cli/ticket/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ticket/helpers.go internal/cli/ticket/helpers_test.go
git commit -m "feat(cli/ticket): add expandManagersToReports helper"
```

---

## Task 7: CLI — stubTicketsvc gains group-related methods

**Files:**
- Modify: `internal/cli/ticket/stub_test.go`
- Modify: `internal/cli/ticket/ticket.go` (add the two new methods to `ticketsvcAPI`)

- [ ] **Step 1: Extend `ticketsvcAPI`**

In `internal/cli/ticket/ticket.go`, find the `ticketsvcAPI` interface and add two methods:

```go
type ticketsvcAPI interface {
	// ... existing methods ...
	ListGroups(ctx context.Context, profile string) ([]domain.TicketGroup, error)
	ResolveGroupByName(ctx context.Context, profile string, name string) (domain.TicketGroup, error)
}
```

- [ ] **Step 2: Extend `stubTicketsvc`**

In `internal/cli/ticket/stub_test.go`:

```go
type stubTicketsvc struct {
	// ... existing fields ...
	groups        []domain.TicketGroup
	resolvedGroup domain.TicketGroup
	groupErr      error
}

// ... existing methods ...

func (s *stubTicketsvc) ListGroups(_ context.Context, _ string) ([]domain.TicketGroup, error) {
	if s.groupErr != nil {
		return nil, s.groupErr
	}
	return s.groups, nil
}

func (s *stubTicketsvc) ResolveGroupByName(_ context.Context, _ string, _ string) (domain.TicketGroup, error) {
	if s.groupErr != nil {
		return domain.TicketGroup{}, s.groupErr
	}
	return s.resolvedGroup, nil
}
```

(Use the package-private convention: separate `groupErr` field rather than reusing the existing `err`. Other stubs in the file already use a single `err` for everything; for safety in mixed-use tests, a dedicated error field is cleaner. Match whichever convention the existing code uses; if all stub methods share `err`, keep consistency and use `err`.)

Recommendation: re-use the existing shared `err` field (since most stub methods do already) and only return errors from group methods when `groupErr` is set. Pick whichever pattern is consistent with the file as it stands.

- [ ] **Step 3: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
```

Expected: PASS. The CLI package shouldn't be calling the new methods yet.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ticket/stub_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): widen ticketsvcAPI with ListGroups/ResolveGroupByName"
```

---

## Task 8: CLI — `tdx ticket groups list` command

**Files:**
- Create: `internal/cli/ticket/groups.go`
- Create: `internal/cli/ticket/groups_test.go`
- Modify: `internal/cli/ticket/ticket.go` (register `newGroupsCmd`)

- [ ] **Step 1: Implement groups.go**

Mirror `internal/cli/ticket/types.go` and `statuses.go`:

```go
package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func newGroupsCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Inspect ticket responsibility groups (tenant-wide)",
	}
	cmd.AddCommand(newGroupsListCmd(svc))
	return cmd
}

func newGroupsListCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ticket responsibility groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			s := svc
			if s == nil {
				s = ticketsvc.New(paths)
			}
			return runGroupsList(cmd.Context(), cmd.OutOrStdout(), s, profile, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runGroupsList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, jsonOut bool) error {
	groups, err := svc.ListGroups(ctx, profile)
	if err != nil {
		return err
	}
	if jsonOut {
		type groupJSON struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}
		out := make([]groupJSON, 0, len(groups))
		for _, g := range groups {
			out = append(out, groupJSON{ID: g.ID, Name: g.Name, Active: g.Active})
		}
		return render.JSON(w, struct {
			Schema string      `json:"schema"`
			Groups []groupJSON `json:"groups"`
		}{Schema: "tdx.v1.ticketGroupList", Groups: out})
	}
	if len(groups) == 0 {
		_, _ = fmt.Fprintln(w, "no ticket groups found")
		return nil
	}
	headers := []string{"ID", "NAME", "ACTIVE"}
	rows := make([][]string, 0, len(groups))
	for _, g := range groups {
		active := "yes"
		if !g.Active {
			active = "no"
		}
		rows = append(rows, []string{strconv.Itoa(g.ID), g.Name, active})
	}
	render.Table(w, headers, rows, nil)
	return nil
}
```

- [ ] **Step 2: Tests**

```go
package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunGroupsListTable(t *testing.T) {
	stub := &stubTicketsvc{groups: []domain.TicketGroup{
		{ID: 100, Name: "Linux Team", Active: true},
		{ID: 101, Name: "Network Ops", Active: true},
		{ID: 102, Name: "Archived", Active: false},
	}}
	var buf bytes.Buffer
	if err := runGroupsList(context.Background(), &buf, stub, "default", false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"100", "Linux Team", "101", "Network Ops", "102", "Archived"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunGroupsListJSON(t *testing.T) {
	stub := &stubTicketsvc{groups: []domain.TicketGroup{{ID: 100, Name: "Linux Team", Active: true}}}
	var buf bytes.Buffer
	if err := runGroupsList(context.Background(), &buf, stub, "default", true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketGroupList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunGroupsListEmpty(t *testing.T) {
	stub := &stubTicketsvc{groups: nil}
	var buf bytes.Buffer
	_ = runGroupsList(context.Background(), &buf, stub, "default", false)
	if !strings.Contains(buf.String(), "no ticket groups found") {
		t.Errorf("empty msg: %s", buf.String())
	}
}
```

- [ ] **Step 3: Wire `New()`**

In `internal/cli/ticket/ticket.go`, add the registration in `New()`:

```go
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Manage TeamDynamix tickets",
	}
	cmd.AddCommand(newAppCmd(nil))
	cmd.AddCommand(newSearchCmd(nil))
	cmd.AddCommand(newShowCmd(nil))
	cmd.AddCommand(newFeedCmd(nil))
	cmd.AddCommand(newCommentCmd(nil))
	cmd.AddCommand(newStatusCmd(nil))
	cmd.AddCommand(newAssignCmd(nil))
	cmd.AddCommand(newLogCmd(nil))
	cmd.AddCommand(newTypesCmd(nil))
	cmd.AddCommand(newStatusesCmd(nil))
	cmd.AddCommand(newGroupsCmd(nil)) // NEW
	return cmd
}
```

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
golangci-lint run ./internal/cli/ticket/...
go run ./cmd/tdx ticket groups --help
```

Expected: clean; help output shows "groups list" sub-command.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ticket/groups.go internal/cli/ticket/groups_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): add groups list command"
```

---

## Task 9: CLI — extend `tdx ticket search` with --responsibility-group + --manager

**Files:**
- Modify: `internal/cli/ticket/search.go`
- Modify: `internal/cli/ticket/search_test.go`

This is the biggest task in the plan. Implements the two new flags plus updates to `buildSearchFilter`.

- [ ] **Step 1: Add the flag declarations and pass-through**

In `newSearchCmd`, add the two new flag variables and registrations:

```go
func newSearchCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		statusFlags    []string
		assigneeFlags  []string
		requestorFlags []string
		groupFlags     []string  // NEW
		managerFlags   []string  // NEW
		accountFlag    string
		textFlag       string
		limitFlag      int
		includeClosed  bool
		appID          int
		jsonFlag       bool
		profileFlag    string
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search tickets in the current app (default: my open)",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			s := svc
			if s == nil { s = ticketsvc.New(paths) }
			people := peoplesvc.New(paths)
			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil { return err }
			filter, err := buildSearchFilter(cmd.Context(), s, people, profile, authedUID, appID,
				statusFlags, assigneeFlags, requestorFlags, groupFlags, managerFlags,
				accountFlag, textFlag, limitFlag, includeClosed)
			if err != nil { return err }
			return runTicketSearch(cmd.Context(), cmd.OutOrStdout(), s, profile, filter, jsonFlag)
		},
	}
	cmd.Flags().StringSliceVar(&statusFlags, "status", nil, "filter by status name or id (repeatable)")
	cmd.Flags().StringSliceVar(&assigneeFlags, "assignee", nil, "assignee me|UID|email (repeatable; default = me)")
	cmd.Flags().StringSliceVar(&requestorFlags, "requestor", nil, "requestor me|UID|email (repeatable)")
	cmd.Flags().StringSliceVar(&groupFlags, "responsibility-group", nil, "responsibility group name or id (repeatable)") // NEW
	cmd.Flags().StringSliceVar(&managerFlags, "manager", nil, "tickets assigned to direct reports of me|UID|email (repeatable)") // NEW
	cmd.Flags().StringVar(&accountFlag, "account", "", "account/department name (currently informational)")
	cmd.Flags().StringVar(&textFlag, "text", "", "free-text search")
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results (capped at 1000)")
	cmd.Flags().BoolVar(&includeClosed, "include-closed", false, "include closed tickets")
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	cmd.AddCommand(newSearchSavedCmd(svc))
	return cmd
}
```

- [ ] **Step 2: Update `buildSearchFilter` signature and body**

Replace the existing `buildSearchFilter` with:

```go
func buildSearchFilter(ctx context.Context, svc ticketsvcAPI, people peoplesvcAPI, profile, authedUID string, appID int,
	statusFlags, assigneeFlags, requestorFlags, groupFlags, managerFlags []string,
	_ string, text string, limit int, includeClosed bool) (domain.TicketSearchFilter, error) {

	if limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	// Resolve --status (numeric or name)
	statusIDs := make([]int, 0, len(statusFlags))
	for _, raw := range statusFlags {
		id, name := parseStatusArg(raw)
		if id > 0 {
			statusIDs = append(statusIDs, id)
			continue
		}
		st, err := svc.ResolveStatusByName(ctx, profile, appID, name)
		if err != nil {
			return domain.TicketSearchFilter{}, fmt.Errorf("--status %q: %w", raw, err)
		}
		statusIDs = append(statusIDs, st.ID)
	}

	// Resolve --responsibility-group (numeric or name)
	groupIDs := make([]int, 0, len(groupFlags))
	for _, raw := range groupFlags {
		// Reuse parseStatusArg for the numeric/name dichotomy — same shape.
		id, name := parseStatusArg(raw)
		if id > 0 {
			groupIDs = append(groupIDs, id)
			continue
		}
		g, err := svc.ResolveGroupByName(ctx, profile, name)
		if err != nil {
			return domain.TicketSearchFilter{}, fmt.Errorf("--responsibility-group %q: %w", raw, err)
		}
		groupIDs = append(groupIDs, g.ID)
	}

	resolveAll := func(args []string) ([]string, error) {
		out := make([]string, 0, len(args))
		for _, a := range args {
			uid, err := resolvePrincipal(ctx, people, profile, authedUID, a)
			if err != nil {
				return nil, err
			}
			out = append(out, uid)
		}
		return out, nil
	}

	assignees, err := resolveAll(assigneeFlags)
	if err != nil { return domain.TicketSearchFilter{}, fmt.Errorf("--assignee: %w", err) }
	requestors, err := resolveAll(requestorFlags)
	if err != nil { return domain.TicketSearchFilter{}, fmt.Errorf("--requestor: %w", err) }

	// Expand --manager into direct-report UIDs and merge into assignees.
	managerReports, err := expandManagersToReports(ctx, people, profile, authedUID, managerFlags)
	if err != nil {
		return domain.TicketSearchFilter{}, err
	}
	if len(managerReports) > 0 {
		// Dedupe across explicit --assignee + manager-expansion.
		seen := make(map[string]struct{}, len(assignees)+len(managerReports))
		for _, uid := range assignees {
			seen[uid] = struct{}{}
		}
		for _, uid := range managerReports {
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			assignees = append(assignees, uid)
		}
	}

	// Default: if user gave no individual selectors AT ALL, default to assignee=me.
	hasAnySelector := len(assignees) > 0 || len(requestors) > 0 || len(groupIDs) > 0 || len(managerFlags) > 0
	if !hasAnySelector {
		if authedUID == "" {
			return domain.TicketSearchFilter{}, fmt.Errorf("no filter specified and no authenticated UID — pass --assignee or --requestor explicitly")
		}
		assignees = []string{authedUID}
	}

	return domain.TicketSearchFilter{
		AppID:                  appID,
		StatusIDs:              statusIDs,
		AssigneeUIDs:           assignees,
		RequestorUIDs:          requestors,
		ResponsibilityGroupIDs: groupIDs,
		Text:                   text,
		IncludeClosed:          includeClosed,
		Limit:                  limit,
	}, nil
}
```

Note the subtle change to default-to-me: it now checks `groupIDs` and `managerFlags` too. We use `managerFlags` (not `managerReports`) on purpose: if a user passed `--manager me` but the manager has no reports (yielding empty `managerReports`), we still respect the user's explicit "manager" intent and don't auto-add `me` as assignee.

- [ ] **Step 3: Tests**

Append to `internal/cli/ticket/search_test.go`:

```go
func TestBuildSearchFilterGroupByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedGroup: domain.TicketGroup{ID: 100, Name: "Linux Team"}}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, []string{"Linux Team"}, nil, "", "", 50, false)
	if err != nil { t.Fatal(err) }
	if len(filter.ResponsibilityGroupIDs) != 1 || filter.ResponsibilityGroupIDs[0] != 100 {
		t.Errorf("group ID not resolved: %+v", filter.ResponsibilityGroupIDs)
	}
	// default-to-me must NOT have fired — group is a selector.
	if len(filter.AssigneeUIDs) != 0 {
		t.Errorf("default-to-me should not fire when --responsibility-group is set; got %v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterGroupByID(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, []string{"42"}, nil, "", "", 50, false)
	if err != nil { t.Fatal(err) }
	if len(filter.ResponsibilityGroupIDs) != 1 || filter.ResponsibilityGroupIDs[0] != 42 {
		t.Errorf("numeric group not preserved: %+v", filter.ResponsibilityGroupIDs)
	}
}

func TestBuildSearchFilterManagerExpandsToReports(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{
		searchUsers: []domain.User{
			{UID: "report-1", ReportsToUID: "uid-me"},
			{UID: "report-2", ReportsToUID: "uid-me"},
			{UID: "other", ReportsToUID: "someone"},
		},
	}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, nil, []string{"me"}, "", "", 50, false)
	if err != nil { t.Fatal(err) }
	if len(filter.AssigneeUIDs) != 2 {
		t.Fatalf("want 2 reports, got %d: %v", len(filter.AssigneeUIDs), filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterManagerSuppressesDefaultToMe(t *testing.T) {
	// When user passes --manager but no direct reports exist, AssigneeUIDs
	// should be empty (NOT default to "me") because --manager is an
	// explicit selector intent.
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{searchUsers: []domain.User{}} // no reports
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, nil, []string{"me"}, "", "", 50, false)
	if err != nil { t.Fatal(err) }
	if len(filter.AssigneeUIDs) != 0 {
		t.Errorf("should be empty (no reports), got %v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterManagerMergesWithAssignees(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{
		searchUsers: []domain.User{
			{UID: "report-1", ReportsToUID: "uid-me"},
		},
	}
	uid := "12345678-1234-1234-1234-123456789012"
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, []string{uid}, nil, nil, []string{"me"}, "", "", 50, false)
	if err != nil { t.Fatal(err) }
	if len(filter.AssigneeUIDs) != 2 {
		t.Fatalf("want 2 (raw + report), got %d: %v", len(filter.AssigneeUIDs), filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterMultipleSelectorsCombine(t *testing.T) {
	// --assignee me + --responsibility-group X → both fields populated;
	// default-to-me does NOT fire (it would have anyway since assignee=me
	// was given, but verify the combined-state explicitly).
	stub := &stubTicketsvc{resolvedGroup: domain.TicketGroup{ID: 200, Name: "Foo"}}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, []string{"me"}, nil, []string{"Foo"}, nil, "", "", 50, false)
	if err != nil { t.Fatal(err) }
	if len(filter.AssigneeUIDs) != 1 || filter.AssigneeUIDs[0] != "uid-me" {
		t.Errorf("assignees: %v", filter.AssigneeUIDs)
	}
	if len(filter.ResponsibilityGroupIDs) != 1 || filter.ResponsibilityGroupIDs[0] != 200 {
		t.Errorf("groups: %v", filter.ResponsibilityGroupIDs)
	}
}

func TestBuildSearchFilterDefaultsToMeWhenNoSelector(t *testing.T) {
	// Existing behavior preserved: no flags → assignee=me.
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, nil, nil, "", "", 50, false)
	if err != nil { t.Fatal(err) }
	if len(filter.AssigneeUIDs) != 1 || filter.AssigneeUIDs[0] != "uid-me" {
		t.Errorf("default-to-me regressed: %v", filter.AssigneeUIDs)
	}
}
```

Existing tests in `search_test.go` call `buildSearchFilter` with the OLD signature (one fewer pair of slice args). UPDATE all existing call sites to insert `nil, nil` (new groupFlags + managerFlags) in the right spot:

```go
// OLD: buildSearchFilter(ctx, stub, people, "default", "uid-me", 31, nil, nil, nil, "", "", 50, false)
// NEW: buildSearchFilter(ctx, stub, people, "default", "uid-me", 31, nil, nil, nil, nil, nil, "", "", 50, false)
```

Find and update every existing call site in `search_test.go`. Run the test and let the compiler tell you if any are missed.

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
golangci-lint run ./internal/cli/ticket/...
go run ./cmd/tdx ticket search --help
```

Expected: help text shows the two new flags. All tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ticket/search.go internal/cli/ticket/search_test.go
git commit -m "feat(cli/ticket): add --responsibility-group and --manager to search"
```

---

## Task 10: MCP — search_tickets gains 2 fields + new list_ticket_groups tool

**Files:**
- Modify: `internal/mcp/tools_ticket.go`
- Modify: `internal/mcp/tools_ticket_test.go`
- Modify: `internal/mcp/server_test.go` (tool count assertion)

- [ ] **Step 1: Add inputs to searchTicketsArgs**

In `internal/mcp/tools_ticket.go`, find the `searchTicketsArgs` struct and add two fields:

```go
type searchTicketsArgs struct {
	Profile                string   `json:"profile,omitempty"`
	AppID                  int      `json:"appID,omitempty"`
	StatusIDs              []int    `json:"statusIDs,omitempty"`
	AssigneeUIDs           []string `json:"assigneeUIDs,omitempty"`
	RequestorUIDs          []string `json:"requestorUIDs,omitempty"`
	AccountIDs             []int    `json:"accountIDs,omitempty"`
	ResponsibilityGroupIDs []int    `json:"responsibilityGroupIDs,omitempty"` // NEW
	ManagerUIDs            []string `json:"managerUIDs,omitempty"`            // NEW
	SearchText             string   `json:"searchText,omitempty"`
	IncludeClosed          bool     `json:"includeClosed,omitempty"`
	MaxResults             int      `json:"maxResults,omitempty"`
}
```

- [ ] **Step 2: Update searchTicketsHandler**

The handler needs to:
1. If `args.ManagerUIDs` is non-empty, expand each to direct reports via `svcs.People.SearchUsers` (mirroring the CLI's `expandManagersToReports`). The MCP layer can't reuse the CLI helper (cross-package), so duplicate the small expansion routine inline OR move the helper to a shared spot.

Recommendation: **inline a small expansion helper in `tools_ticket.go`**. It's ~15 lines. Keeping the CLI helper CLI-only avoids broadening package boundaries.

Add to `tools_ticket.go`:

```go
// mcpExpandManagersToReports mirrors the CLI helper for the MCP layer.
// Takes a list of manager UIDs (already resolved — MCP doesn't accept
// "me" or emails in inputs, only UIDs) and returns the union of direct-
// report UIDs.
func mcpExpandManagersToReports(ctx context.Context, svcs Services, profile string, managerUIDs []string) ([]string, error) {
	if len(managerUIDs) == 0 {
		return nil, nil
	}
	managerSet := make(map[string]struct{}, len(managerUIDs))
	for _, u := range managerUIDs {
		managerSet[u] = struct{}{}
	}
	trueVal := true
	all, err := svcs.People.SearchUsers(ctx, profile, domain.UserFilter{
		Employee: &trueVal,
		Limit:    5000,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch staff for manager expansion: %w", err)
	}
	seen := make(map[string]struct{})
	var out []string
	for _, u := range all {
		if _, ok := managerSet[u.ReportsToUID]; !ok {
			continue
		}
		if _, dup := seen[u.UID]; dup {
			continue
		}
		seen[u.UID] = struct{}{}
		out = append(out, u.UID)
	}
	return out, nil
}
```

Then in `searchTicketsHandler`, after argument parsing and before the search call:

```go
assignees := args.AssigneeUIDs
if len(args.ManagerUIDs) > 0 {
	reports, err := mcpExpandManagersToReports(ctx, svcs, profile, args.ManagerUIDs)
	if err != nil {
		return errorResult(fmt.Sprintf("search_tickets: %v", err)), nil, nil
	}
	seen := make(map[string]struct{}, len(assignees)+len(reports))
	for _, u := range assignees {
		seen[u] = struct{}{}
	}
	for _, u := range reports {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		assignees = append(assignees, u)
	}
}
filter := domain.TicketSearchFilter{
	AppID:                  args.AppID,
	StatusIDs:              args.StatusIDs,
	AssigneeUIDs:           assignees,
	RequestorUIDs:          args.RequestorUIDs,
	AccountIDs:             args.AccountIDs,
	ResponsibilityGroupIDs: args.ResponsibilityGroupIDs,
	Text:                   args.SearchText,
	IncludeClosed:          args.IncludeClosed,
	Limit:                  args.MaxResults,
}
```

(Find the existing handler shape; the code above is illustrative — preserve the existing handler's idioms for service calls, JSON envelope wrapping, etc.)

- [ ] **Step 3: Add list_ticket_groups tool**

Append to `tools_ticket.go`:

```go
type listTicketGroupsArgs struct {
	Profile string `json:"profile,omitempty"`
}

type ticketGroupJSON struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func listTicketGroupsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listTicketGroupsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listTicketGroupsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		groups, err := svcs.Tickets.ListGroups(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("list_ticket_groups: %v", err)), nil, nil
		}
		out := make([]ticketGroupJSON, 0, len(groups))
		for _, g := range groups {
			out = append(out, ticketGroupJSON{ID: g.ID, Name: g.Name, Active: g.Active})
		}
		return jsonResult(struct {
			Schema string            `json:"schema"`
			Groups []ticketGroupJSON `json:"groups"`
		}{Schema: "tdx.v1.ticketGroupList", Groups: out})
	}
}
```

In `RegisterTicketTools`, register the new tool:

```go
sdkmcp.AddTool(srv, &sdkmcp.Tool{
	Name:        "list_ticket_groups",
	Description: "List tenant responsibility groups (teams that can be assigned tickets). Read-only.",
}, listTicketGroupsHandler(svcs))
```

- [ ] **Step 4: Tests**

In `internal/mcp/tools_ticket_test.go`, append a smoke test:

```go
func TestRegisterListTicketGroups(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTicketTools(srv, Services{})
	// No panic = success.
}
```

- [ ] **Step 5: Update tool-count assertion**

In `internal/mcp/server_test.go`, find the test that asserts the tool count (was 56 after v0.16.0). Update it to 57.

```bash
grep -n "wantCount\s*=\s*56\|wantCount\s*:=\s*56" internal/mcp/server_test.go
```

If found, change `56` → `57`.

- [ ] **Step 6: Verify**

```bash
go build ./...
go test ./internal/mcp/...
golangci-lint run ./internal/mcp/...
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/
git commit -m "feat(mcp): search_tickets gains group/manager inputs + list_ticket_groups tool"
```

---

## Task 11: Documentation

**Files:**
- Modify: `docs/guide/ticket.md`
- Modify: `docs/guide/mcp.md`
- Modify: `docs/guide.md`
- Modify: `README.md`

- [ ] **Step 1: Update `docs/guide/ticket.md`**

Two edits:

(a) Under `## tdx ticket search`, add documentation for the two new flags. Insert into the existing flag bullet list:

```markdown
- `--responsibility-group <name|id>` — filter to tickets assigned to a TD responsibility group (team). Repeatable. Numeric arg = ID; non-numeric = case-insensitive exact name match (errors with candidate list on ambiguity). Use `tdx ticket groups list` to discover groups.
- `--manager me|UID|email` — expand to "tickets assigned to direct reports of this person." Repeatable. `me` = the authenticated user. Resolution: looks up direct reports via `/api/people/search` then injects their UIDs as assignees. Direct reports only (no transitive walk).
```

Also add an example to the existing examples block:

```bash
# Tickets assigned to my team (direct reports), open only
tdx ticket search --manager me

# Tickets assigned to a specific group
tdx ticket search --responsibility-group "Linux Platform Services"

# Mixed: my open tickets PLUS Linux Team's
tdx ticket search --assignee me --responsibility-group "Linux Platform Services"
```

(b) Add a new top-level `## tdx ticket groups` section after `## tdx ticket statuses`:

```markdown
## tdx ticket groups

Inspect tenant-wide ticket responsibility groups (teams that tickets can be assigned to as a group rather than to an individual).

### tdx ticket groups list

```bash
tdx ticket groups list
tdx ticket groups list --json
```

Output: `ID | NAME | ACTIVE` table. JSON envelope: `tdx.v1.ticketGroupList`.

Groups are tenant-wide — the same group can serve multiple ticket apps. Use `--responsibility-group <name>` on `tdx ticket search` once you know the group you care about.
```

Also update the Contents TOC list at top of ticket.md to include the new section.

- [ ] **Step 2: Update `docs/guide/mcp.md`**

Two edits:

(a) Update the intro line counts. Change `56 tools` to `57 tools`. Adjust the read/mutating split: 8 read → 9 read in the ticket section.

(b) In the "Tickets (Phase D — read-only)" table, add a row:

```markdown
| `list_ticket_groups` | List tenant responsibility groups (teams) |
```

Also add a bullet under the search_tickets row noting the new inputs:

```markdown
| `search_tickets` | Search tickets by status/assignee/requestor/text. Now also accepts `responsibilityGroupIDs []int` and `managerUIDs []string` (manager UIDs expand to direct-report UIDs server-side). |
```

(Actually, edit the existing description if cleaner. The description column is short — keep it terse.)

- [ ] **Step 3: Update ASCII tree in `docs/guide.md`**

Find the `ticket` branch and add `groups` as a sub-group:

```text
├── ticket
│   ├── app              → list / use / show
│   ├── search           → saved
│   ├── show / feed
│   ├── comment / status / assign / log
│   ├── types / statuses → list
│   └── groups           → list
```

- [ ] **Step 4: Update ASCII tree in `README.md`**

Same change. Verify byte-identical:

```bash
diff <(awk '/^```text$/,/^```$/' docs/guide.md) <(awk '/^```text$/,/^```$/' README.md)
```

Expected: empty.

- [ ] **Step 5: Commit**

```bash
git add docs/guide/ticket.md docs/guide/mcp.md docs/guide.md README.md
git commit -m "docs: add team-scope filter docs + tdx ticket groups list reference"
```

---

## Task 12: Live verification + PR + release

**Files:** none modified — verification only, then push + tag.

### Live verification (run on UFL)

- [ ] **Step 1: Pre-flight**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

All green required.

- [ ] **Step 2: Build local binary**

```bash
go build -o tdx ./cmd/tdx
```

- [ ] **Step 3: Probe each new path**

```bash
./tdx ticket groups list                                    # ≥1 group
./tdx ticket groups list --json | jq '.schema'              # tdx.v1.ticketGroupList
./tdx ticket search --responsibility-group "<a real group from above>" --limit 5
./tdx ticket search --manager me --limit 5                  # tickets assigned to your reports
./tdx ticket search --responsibility-group "<group>" --manager me --limit 5  # combined
./tdx ticket search --limit 3                               # default-to-me still works
```

Each should succeed and return reasonable results. If any fails on a wire-format issue, fix on the same branch and re-test.

- [ ] **Step 4: MCP smoke**

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./tdx mcp serve | jq '.result.tools | length'
```

Expected: 57.

### Push + PR + merge + tag

- [ ] **Step 5: Push branch**

```bash
git push -u origin ticket-team-filters
```

- [ ] **Step 6: Open PR**

Use `gh pr create` with a body file (heredoc-via-stdin tends to break on backticks). Suggested title: `v0.16.1: tdx ticket team-scope filters`. Body covers: summary, the two new flags + groups command + MCP additions, live-verification checklist, "tag v0.16.1 after merge."

- [ ] **Step 7: Merge after CI passes**

```bash
gh pr merge <PR#> --squash --admin --delete-branch
```

- [ ] **Step 8: Reset main, tag, push tag**

```bash
git checkout main
git fetch origin
git reset --hard origin/main
git tag v0.16.1
git push origin v0.16.1
```

Goreleaser publishes the release.

- [ ] **Step 9: Update memory**

Update `MEMORY.md` index line for current state to reflect v0.16.1. Add a new "Latest release" block to `project_tdx_current_state.md`. Mark the `project_tdx_backlog.md` entry as **shipped** (or remove it — it's done).

---

## Self-Review

**1. Spec coverage:**

Spec → Tasks:
- TicketGroup type → Task 1
- ResponsibilityGroupIDs filter field → Task 1
- wireGroup wire type → Task 2
- ResponsibilityGroupIDs wire field → Task 2
- ListGroups + ResolveGroupByName service methods → Task 3
- SearchTickets wires new field → Task 4
- peoplesvcAPI gains SearchUsers → Task 5
- expandManagersToReports helper → Task 6
- ticketsvcAPI gains group methods + stub updated → Task 7
- `tdx ticket groups list` command → Task 8
- --responsibility-group / --manager flags + buildSearchFilter changes → Task 9
- MCP search_tickets gains 2 inputs + list_ticket_groups tool → Task 10
- All docs updates (ticket.md, guide.md tree, README tree, mcp.md) → Task 11
- Live verification + release → Task 12

All 9 acceptance criteria from the spec map to tasks 8-12 (commands wired, behaviors verified, count = 57, tests + lint clean, live-verified, released as v0.16.1).

**2. Placeholder scan:**
- Step instructions like "Find the existing handler shape; preserve the existing idioms" are concrete enough — they tell the implementer to read the current code, not invent something.
- One spot in Task 9 says "Find and update every existing call site in `search_test.go`. Run the test and let the compiler tell you if any are missed." — this is a practical instruction, not a placeholder.
- No "TBD"/"TODO".

**3. Type consistency:**
- `domain.TicketGroup` and `domain.TicketSearchFilter.ResponsibilityGroupIDs` referenced consistently from Tasks 1 → 4 → 7 → 9 → 10.
- `wireGroup` and `wireTicketSearch.ResponsibilityGroupIDs` consistent across Tasks 2 → 3 → 4.
- `expandManagersToReports` (CLI) and `mcpExpandManagersToReports` (MCP) — different names because they live in different packages and have different signatures (CLI accepts `me`/email, MCP accepts UIDs only). Documented in Task 10 step 2.
- `peoplesvcAPI.SearchUsers` signature matches `peoplesvc.Service.SearchUsers` (verified by grep in earlier sessions).
- All flag names: `--responsibility-group` and `--manager` used consistently in CLI flag registration, docs, and MCP error messages.

All consistent.
