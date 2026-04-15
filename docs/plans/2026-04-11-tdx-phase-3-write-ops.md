# Phase 3 -- Write Operations Implementation Plan

> **For agentic workers:** execute tasks in numbered order. Each task follows
> strict TDD: write failing test, verify failure, implement, verify pass, commit.
> Never amend commits -- always create new ones. Branch: `phase-3-write-ops`.
> No `go mod tidy` -- Phase 3 adds zero new deps.

**Design spec:** `docs/superpowers/specs/2026-04-11-tdx-phase-3-write-ops-design.md`

## Goal

Add three write commands to tdx -- `tdx time entry add`, `update`, `delete` --
with pre-write validation (locked days, report status), `--dry-run` support,
batch auto-split at 50 for multi-delete, and partial-success reporting (exit 2).
Extend the walkthrough script to exercise the full create-read-update-delete
lifecycle.

## Architecture

```
CLI layer (add.go, update.go, delete.go)
  |-- pre-write validation (GetLockedDays, GetWeekReport)
  |-- --dry-run short-circuit
  |-- calls service layer
  v
Service layer (timesvc/write.go)
  |-- AddEntry   -> POST /api/time (1-element array)
  |-- UpdateEntry -> GET + PUT /api/time/{id}
  |-- DeleteEntry -> DELETE /api/time/{id}
  |-- DeleteEntries -> POST /api/time/delete (batched at 50)
  v
Wire layer (timesvc/encode.go, timesvc/types.go)
  |-- encodeTarget, encodeEntryWrite
  |-- wireTimeEntryWrite, wireBulkResult
```

Pre-write validation lives in the CLI layer, not the service layer. This keeps
service methods simple (just do the write) and makes `--dry-run` trivial (skip
the service call).

## Tech Stack

Same as Phases 1-2: Go 1.24, cobra, stdlib `net/http`, `encoding/json`,
`net/http/httptest` for tests. No new dependencies.

---

## Task 1: Fix Target.Validate + add new error sentinels

Fix `Target.Validate()` so it only requires `AppID > 0` for ticket and
ticketTask kinds. Add `ErrDayLocked` and `ErrWeekSubmitted` sentinels.

### Step 1.1 -- Write failing tests

Create/update the test file for Target.Validate:

```bash
cat > internal/domain/target_test.go << 'GOEOF'
package domain

import (
	"errors"
	"testing"
)

func TestTargetValidate(t *testing.T) {
	tests := []struct {
		name    string
		target  Target
		wantErr error
	}{
		{
			name:    "empty kind",
			target:  Target{},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "unknown kind",
			target:  Target{Kind: TargetKind("bogus")},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "ticket requires appID",
			target:  Target{Kind: TargetTicket, ItemID: 1},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "ticketTask requires appID",
			target:  Target{Kind: TargetTicketTask, AppID: 0, ItemID: 1, TaskID: 2},
			wantErr: ErrInvalidTarget,
		},
		{
			name:   "ticket with appID valid",
			target: Target{Kind: TargetTicket, AppID: 5, ItemID: 1},
		},
		{
			name:   "ticketTask with appID valid",
			target: Target{Kind: TargetTicketTask, AppID: 5, ItemID: 1, TaskID: 2},
		},
		{
			name:    "ticket requires itemID",
			target:  Target{Kind: TargetTicket, AppID: 5},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "ticketTask requires taskID",
			target:  Target{Kind: TargetTicketTask, AppID: 5, ItemID: 1},
			wantErr: ErrInvalidTarget,
		},
		{
			name:   "project does not require appID",
			target: Target{Kind: TargetProject, ItemID: 54},
		},
		{
			name:   "projectTask does not require appID",
			target: Target{Kind: TargetProjectTask, ItemID: 2091, TaskID: 2612},
		},
		{
			name:   "workspace does not require appID",
			target: Target{Kind: TargetWorkspace, ItemID: 10},
		},
		{
			name:   "timeoff does not require appID",
			target: Target{Kind: TargetTimeOff, ItemID: 10},
		},
		{
			name:   "portfolio does not require appID",
			target: Target{Kind: TargetPortfolio, ItemID: 10},
		},
		{
			name:   "projectIssue does not require appID",
			target: Target{Kind: TargetProjectIssue, ItemID: 10},
		},
		{
			name:    "projectTask requires taskID",
			target:  Target{Kind: TargetProjectTask, ItemID: 2091},
			wantErr: ErrInvalidTarget,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.Validate()
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error wrapping %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error wrapping %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}
GOEOF
```

### Step 1.2 -- Verify test failure

```bash
go test ./internal/domain/ -run TestTargetValidate -count=1
```

Expected: several subtests fail because Validate currently requires `AppID > 0`
for all kinds.

### Step 1.3 -- Implement fixes

In `internal/domain/target.go`, replace the Validate method:

```go
func (t Target) Validate() error {
	if t.Kind == "" {
		return fmt.Errorf("%w: kind is required", ErrInvalidTarget)
	}
	if !t.Kind.IsKnown() {
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidTarget, t.Kind)
	}
	// AppID is only meaningful for ticket-based kinds.
	if (t.Kind == TargetTicket || t.Kind == TargetTicketTask) && t.AppID <= 0 {
		return fmt.Errorf("%w: %s requires appID", ErrInvalidTarget, t.Kind)
	}
	if t.ItemID <= 0 {
		return fmt.Errorf("%w: itemID is required", ErrInvalidTarget)
	}
	if (t.Kind == TargetTicketTask || t.Kind == TargetProjectTask) && t.TaskID <= 0 {
		return fmt.Errorf("%w: %s requires taskID", ErrInvalidTarget, t.Kind)
	}
	return nil
}
```

In `internal/domain/errors.go`, add the two new sentinels:

```go
var (
	ErrDayLocked     = errors.New("day is locked")
	ErrWeekSubmitted = errors.New("week already submitted")
)
```

### Step 1.4 -- Verify tests pass

```bash
go test ./internal/domain/ -run TestTargetValidate -count=1
go vet ./internal/domain/
```

### Step 1.5 -- Commit

```bash
git add internal/domain/target.go internal/domain/target_test.go internal/domain/errors.go
git commit -m "fix(domain): Target.Validate only requires AppID for ticket kinds; add write sentinels

AppID is only meaningful for ticket and ticketTask targets. Project,
workspace, timeoff, portfolio, and projectIssue targets legitimately
have AppID=0.

Also adds ErrDayLocked and ErrWeekSubmitted sentinels for Phase 3
pre-write validation.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Add domain types (BatchResult, EntryInput, EntryUpdate)

### Step 2.1 -- Write failing tests

```bash
cat > internal/domain/batch_test.go << 'GOEOF'
package domain

import "testing"

func TestBatchResultFullSuccess(t *testing.T) {
	r := BatchResult{Succeeded: []int{1, 2, 3}}
	if !r.FullSuccess() {
		t.Fatal("expected FullSuccess")
	}
	if r.PartialSuccess() {
		t.Fatal("expected not PartialSuccess")
	}
	if r.TotalFailure() {
		t.Fatal("expected not TotalFailure")
	}
}

func TestBatchResultPartialSuccess(t *testing.T) {
	r := BatchResult{
		Succeeded: []int{1},
		Failed:    []BatchFailure{{ID: 2, Message: "not found"}},
	}
	if r.FullSuccess() {
		t.Fatal("expected not FullSuccess")
	}
	if !r.PartialSuccess() {
		t.Fatal("expected PartialSuccess")
	}
	if r.TotalFailure() {
		t.Fatal("expected not TotalFailure")
	}
}

func TestBatchResultTotalFailure(t *testing.T) {
	r := BatchResult{
		Failed: []BatchFailure{{ID: 1, Message: "locked"}},
	}
	if r.FullSuccess() {
		t.Fatal("expected not FullSuccess")
	}
	if r.PartialSuccess() {
		t.Fatal("expected not PartialSuccess")
	}
	if !r.TotalFailure() {
		t.Fatal("expected TotalFailure")
	}
}

func TestBatchResultEmpty(t *testing.T) {
	r := BatchResult{}
	if r.FullSuccess() {
		t.Fatal("expected not FullSuccess for empty result")
	}
	if r.PartialSuccess() {
		t.Fatal("expected not PartialSuccess for empty result")
	}
	if r.TotalFailure() {
		t.Fatal("expected not TotalFailure for empty result")
	}
}
GOEOF

cat > internal/domain/entry_input_test.go << 'GOEOF'
package domain

import "testing"

func TestEntryUpdateIsEmpty(t *testing.T) {
	u := EntryUpdate{}
	if !u.IsEmpty() {
		t.Fatal("expected IsEmpty for zero EntryUpdate")
	}

	desc := "hello"
	u.Description = &desc
	if u.IsEmpty() {
		t.Fatal("expected not IsEmpty when Description is set")
	}
}

func TestEntryUpdateIsEmptyAllFields(t *testing.T) {
	mins := 30
	typeID := 5
	billable := true
	desc := "test"
	u := EntryUpdate{
		Minutes:     &mins,
		TimeTypeID:  &typeID,
		Billable:    &billable,
		Description: &desc,
	}
	if u.IsEmpty() {
		t.Fatal("expected not IsEmpty when all fields set")
	}
}
GOEOF
```

### Step 2.2 -- Verify test failure

```bash
go test ./internal/domain/ -run "TestBatchResult|TestEntryUpdate" -count=1
```

Expected: compilation failures because the types don't exist yet.

### Step 2.3 -- Implement

```bash
cat > internal/domain/batch.go << 'GOEOF'
package domain

// BatchResult reports the outcome of a batch write operation.
type BatchResult struct {
	Succeeded []int          // IDs that succeeded
	Failed    []BatchFailure // entries that failed
}

// BatchFailure describes a single failed item in a batch operation.
type BatchFailure struct {
	ID      int
	Message string
}

// FullSuccess returns true when all items succeeded and at least one did.
func (r BatchResult) FullSuccess() bool {
	return len(r.Failed) == 0 && len(r.Succeeded) > 0
}

// PartialSuccess returns true when some items succeeded and some failed.
func (r BatchResult) PartialSuccess() bool {
	return len(r.Succeeded) > 0 && len(r.Failed) > 0
}

// TotalFailure returns true when no items succeeded and at least one failed.
func (r BatchResult) TotalFailure() bool {
	return len(r.Succeeded) == 0 && len(r.Failed) > 0
}
GOEOF

cat > internal/domain/entry_input.go << 'GOEOF'
package domain

import "time"

// EntryInput holds the fields required to create a new time entry.
type EntryInput struct {
	UserUID     string
	Date        time.Time
	Minutes     int
	TimeTypeID  int
	Billable    bool
	Target      Target
	ProjectID   int // wire ProjectID for projectTask/projectIssue; 0 for others
	Description string
}

// EntryUpdate holds optional fields for updating an existing time entry.
// Nil pointer fields are left unchanged.
type EntryUpdate struct {
	Date        *time.Time
	Minutes     *int
	TimeTypeID  *int
	Billable    *bool
	Description *string
}

// IsEmpty returns true if no fields are set for update.
func (u EntryUpdate) IsEmpty() bool {
	return u.Date == nil && u.Minutes == nil && u.TimeTypeID == nil && u.Billable == nil && u.Description == nil
}
GOEOF
```

### Step 2.4 -- Verify tests pass

```bash
go test ./internal/domain/ -run "TestBatchResult|TestEntryUpdate" -count=1
go vet ./internal/domain/
```

### Step 2.5 -- Commit

```bash
git add internal/domain/batch.go internal/domain/batch_test.go internal/domain/entry_input.go internal/domain/entry_input_test.go
git commit -m "feat(domain): add BatchResult, EntryInput, EntryUpdate types

BatchResult tracks succeeded/failed IDs for batch operations.
EntryInput carries all fields needed to create a time entry.
EntryUpdate uses pointer fields so nil means 'unchanged'.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Add wire write types to timesvc/types.go

### Step 3.1 -- Add wire types

No separate test needed -- these structs are tested implicitly by Tasks 4-7.
Add the following to the bottom of `internal/svc/timesvc/types.go`:

```go
// wireTimeEntryWrite is the request body for POST/PUT /api/time.
type wireTimeEntryWrite struct {
	TimeID      int     `json:"TimeID,omitempty"`
	Uid         string  `json:"Uid"`
	TimeDate    string  `json:"TimeDate"`
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

// wireBulkResult is the response from batch POST /api/time and POST /api/time/delete.
type wireBulkResult struct {
	Succeeded []wireBulkSuccess `json:"Succeeded"`
	Failed    []wireBulkFailure `json:"Failed"`
}

type wireBulkSuccess struct {
	Index int `json:"Index"`
	ID    int `json:"ID"`
}

type wireBulkFailure struct {
	Index         int    `json:"Index"`
	TimeEntryID   int    `json:"TimeEntryID"`
	ErrorMessage  string `json:"ErrorMessage"`
	ErrorCode     int    `json:"ErrorCode"`
	ErrorCodeName string `json:"ErrorCodeName"`
}
```

### Step 3.2 -- Verify compilation

```bash
go vet ./internal/svc/timesvc/
```

### Step 3.3 -- Commit

```bash
git add internal/svc/timesvc/types.go
git commit -m "feat(timesvc): add wire write types for Phase 3

wireTimeEntryWrite, wireBulkResult, wireBulkSuccess, wireBulkFailure.
Shapes verified against live UFL tenant during Step 0 probing.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: encodeTarget + encodeEntryWrite

### Step 4.1 -- Write failing tests

```bash
cat > internal/svc/timesvc/encode_test.go << 'GOEOF'
package timesvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestEncodeTargetRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		target    domain.Target
		projectID int // for projectTask/projectIssue
	}{
		{
			name:   "ticket",
			target: domain.Target{Kind: domain.TargetTicket, AppID: 5, ItemID: 100},
		},
		{
			name:   "ticketTask",
			target: domain.Target{Kind: domain.TargetTicketTask, AppID: 5, ItemID: 100, TaskID: 200},
		},
		{
			name:   "project",
			target: domain.Target{Kind: domain.TargetProject, ItemID: 54},
		},
		{
			name:      "projectTask",
			target:    domain.Target{Kind: domain.TargetProjectTask, ItemID: 2091, TaskID: 2612},
			projectID: 54,
		},
		{
			name:      "projectIssue",
			target:    domain.Target{Kind: domain.TargetProjectIssue, ItemID: 300},
			projectID: 54,
		},
		{
			name:   "workspace",
			target: domain.Target{Kind: domain.TargetWorkspace, ItemID: 10},
		},
		{
			name:   "timeoff",
			target: domain.Target{Kind: domain.TargetTimeOff, ItemID: 10},
		},
		{
			name:   "portfolio",
			target: domain.Target{Kind: domain.TargetPortfolio, ItemID: 10},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w wireTimeEntryWrite
			err := encodeTarget(tt.target, tt.projectID, &w)
			if err != nil {
				t.Fatalf("encodeTarget: %v", err)
			}

			// Build a wireTimeEntry from the write struct to decode back.
			wire := wireTimeEntry{
				Component:   w.Component,
				TicketID:    w.TicketID,
				ProjectID:   w.ProjectID,
				PlanID:      w.PlanID,
				PortfolioID: w.PortfolioID,
				ItemID:      w.ItemID,
				AppID:       w.AppID,
			}
			got, err := decodeTarget(wire)
			if err != nil {
				t.Fatalf("decodeTarget: %v", err)
			}
			if got.Kind != tt.target.Kind {
				t.Errorf("kind: got %q, want %q", got.Kind, tt.target.Kind)
			}
			if got.ItemID != tt.target.ItemID {
				t.Errorf("itemID: got %d, want %d", got.ItemID, tt.target.ItemID)
			}
			if got.TaskID != tt.target.TaskID {
				t.Errorf("taskID: got %d, want %d", got.TaskID, tt.target.TaskID)
			}
			if got.AppID != tt.target.AppID {
				t.Errorf("appID: got %d, want %d", got.AppID, tt.target.AppID)
			}
		})
	}
}

func TestEncodeTargetUnsupportedKind(t *testing.T) {
	var w wireTimeEntryWrite
	err := encodeTarget(domain.Target{Kind: domain.TargetKind("bogus"), ItemID: 1}, 0, &w)
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func TestEncodeEntryWrite(t *testing.T) {
	input := domain.EntryInput{
		UserUID:     "uid-123",
		Date:        time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Minutes:     90,
		TimeTypeID:  5,
		Billable:    true,
		Target:      domain.Target{Kind: domain.TargetProjectTask, ItemID: 2091, TaskID: 2612},
		ProjectID:   54,
		Description: "did some work",
	}
	w, err := encodeEntryWrite(input)
	if err != nil {
		t.Fatalf("encodeEntryWrite: %v", err)
	}
	if w.Uid != "uid-123" {
		t.Errorf("Uid: got %q, want %q", w.Uid, "uid-123")
	}
	if w.TimeDate != "2026-04-11T00:00:00" {
		t.Errorf("TimeDate: got %q, want %q", w.TimeDate, "2026-04-11T00:00:00")
	}
	if w.Minutes != 90.0 {
		t.Errorf("Minutes: got %f, want 90.0", w.Minutes)
	}
	if w.TimeTypeID != 5 {
		t.Errorf("TimeTypeID: got %d, want 5", w.TimeTypeID)
	}
	if !w.Billable {
		t.Error("Billable: got false, want true")
	}
	if w.Component != componentTaskTime {
		t.Errorf("Component: got %d, want %d", w.Component, componentTaskTime)
	}
	if w.PlanID != 2091 {
		t.Errorf("PlanID: got %d, want 2091", w.PlanID)
	}
	if w.ItemID != 2612 {
		t.Errorf("ItemID: got %d, want 2612", w.ItemID)
	}
	if w.ProjectID != 54 {
		t.Errorf("ProjectID: got %d, want 54", w.ProjectID)
	}
	if w.Description != "did some work" {
		t.Errorf("Description: got %q", w.Description)
	}
}
GOEOF
```

### Step 4.2 -- Verify test failure

```bash
go test ./internal/svc/timesvc/ -run "TestEncodeTarget|TestEncodeEntryWrite" -count=1
```

Expected: compilation failure -- `encodeTarget` and `encodeEntryWrite` don't exist.

### Step 4.3 -- Implement

```bash
cat > internal/svc/timesvc/encode.go << 'GOEOF'
package timesvc

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// encodeTarget maps a domain.Target to wire fields on a wireTimeEntryWrite.
// projectID is the wire ProjectID required for projectTask and projectIssue kinds.
func encodeTarget(t domain.Target, projectID int, w *wireTimeEntryWrite) error {
	switch t.Kind {
	case domain.TargetTicket:
		w.Component = componentTicketTime
		w.TicketID = t.ItemID
		w.AppID = t.AppID
	case domain.TargetTicketTask:
		w.Component = componentTicketTaskTime
		w.TicketID = t.ItemID
		w.ItemID = t.TaskID
		w.AppID = t.AppID
	case domain.TargetProject:
		w.Component = componentProjectTime
		w.ProjectID = t.ItemID
	case domain.TargetProjectTask:
		w.Component = componentTaskTime
		w.ProjectID = projectID
		w.PlanID = t.ItemID
		w.ItemID = t.TaskID
	case domain.TargetProjectIssue:
		w.Component = componentIssueTime
		w.ProjectID = projectID
		w.ItemID = t.ItemID
	case domain.TargetWorkspace:
		w.Component = componentWorkspaceTime
		w.ProjectID = t.ItemID
	case domain.TargetTimeOff:
		w.Component = componentTimeOff
		w.ProjectID = t.ItemID
	case domain.TargetPortfolio:
		w.Component = componentPortfolioTime
		w.PortfolioID = t.ItemID
		w.ItemID = t.ItemID
	default:
		return fmt.Errorf("%w: %s", domain.ErrUnsupportedTargetKind, t.Kind)
	}
	return nil
}

// encodeEntryWrite builds a wireTimeEntryWrite from a domain.EntryInput.
func encodeEntryWrite(input domain.EntryInput) (wireTimeEntryWrite, error) {
	w := wireTimeEntryWrite{
		Uid:         input.UserUID,
		TimeDate:    input.Date.Format("2006-01-02T00:00:00"),
		Minutes:     float64(input.Minutes),
		TimeTypeID:  input.TimeTypeID,
		Description: input.Description,
		Billable:    input.Billable,
	}
	if err := encodeTarget(input.Target, input.ProjectID, &w); err != nil {
		return wireTimeEntryWrite{}, err
	}
	return w, nil
}
GOEOF
```

### Step 4.4 -- Verify tests pass

```bash
go test ./internal/svc/timesvc/ -run "TestEncodeTarget|TestEncodeEntryWrite" -count=1
go vet ./internal/svc/timesvc/
```

### Step 4.5 -- Commit

```bash
git add internal/svc/timesvc/encode.go internal/svc/timesvc/encode_test.go
git commit -m "feat(timesvc): add encodeTarget and encodeEntryWrite

Reverse of decodeTarget -- maps domain targets to wire fields.
Round-trip tested for all 8 supported target kinds.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: AddEntry service method

### Step 5.1 -- Write failing tests

```bash
cat > internal/svc/timesvc/write_test.go << 'GOEOF'
package timesvc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// writeTestConfig creates a minimal config directory pointing at the given URL.
func writeTestConfig(t *testing.T, dir, url string) {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf("defaultProfile: default\nprofiles:\n- name: default\n  tenantBaseURL: %s\n", url)), 0644); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(dir, "credentials.yaml")
	if err := os.WriteFile(credPath, []byte("tokens:\n  default: test-token\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAddEntrySuccess(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time":
			callCount++
			// Verify request body is a 1-element array.
			body, _ := io.ReadAll(r.Body)
			var entries []wireTimeEntryWrite
			if err := json.Unmarshal(body, &entries); err != nil {
				t.Errorf("unmarshal request body: %v", err)
			}
			if len(entries) != 1 {
				t.Errorf("expected 1 entry in array, got %d", len(entries))
			}
			if entries[0].Uid != "uid-abc" {
				t.Errorf("expected Uid uid-abc, got %s", entries[0].Uid)
			}
			if entries[0].Component != componentTaskTime {
				t.Errorf("expected component %d, got %d", componentTaskTime, entries[0].Component)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":999}],"Failed":[]}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":2612,"ItemTitle":"Test Task",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":0,"AppName":"None","Component":2,
				"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
				"PlanID":2091,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":15.0,"Description":"test desc",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":70,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	input := domain.EntryInput{
		UserUID:     "uid-abc",
		Date:        time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Minutes:     15,
		TimeTypeID:  5,
		Billable:    false,
		Target:      domain.Target{Kind: domain.TargetProjectTask, ItemID: 2091, TaskID: 2612},
		ProjectID:   54,
		Description: "test desc",
	}
	entry, err := svc.AddEntry(context.Background(), "default", input)
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}
	if entry.ID != 999 {
		t.Errorf("expected ID 999, got %d", entry.ID)
	}
	if entry.Minutes != 15 {
		t.Errorf("expected 15 minutes, got %d", entry.Minutes)
	}
	if callCount != 1 {
		t.Errorf("expected 1 POST call, got %d", callCount)
	}
}

func TestAddEntryServerFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Succeeded":[],"Failed":[{"Index":0,"TimeEntryID":0,"ErrorMessage":"Day is locked","ErrorCode":40,"ErrorCodeName":"DayLocked"}]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	input := domain.EntryInput{
		UserUID:    "uid-abc",
		Date:       time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Minutes:    15,
		TimeTypeID: 5,
		Target:     domain.Target{Kind: domain.TargetTicket, AppID: 5, ItemID: 100},
	}
	_, err := svc.AddEntry(context.Background(), "default", input)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Day is locked") {
		t.Errorf("expected error to contain 'Day is locked', got: %v", err)
	}
}

// mustResolvePaths is a test helper that resolves config paths or fails the test.
func mustResolvePaths(t *testing.T) configPaths {
	t.Helper()
	p, err := resolveTestPaths()
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	return p
}
GOEOF
```

Note: `mustResolvePaths` and `configPaths` are placeholders -- adjust to match the
actual `config.ResolvePaths()` return type and import. The test file imports and
helper may need minor adjustments to match existing patterns in the codebase.
Look at the existing test files in `internal/svc/timesvc/` for the exact pattern.

### Step 5.2 -- Verify test failure

```bash
go test ./internal/svc/timesvc/ -run "TestAddEntry" -count=1
```

Expected: compilation failure -- `AddEntry` method doesn't exist.

### Step 5.3 -- Implement

Create `internal/svc/timesvc/write.go`:

```go
package timesvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// AddEntry creates a new time entry and returns the full domain entry.
func (s *Service) AddEntry(ctx context.Context, profileName string, input domain.EntryInput) (domain.TimeEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.TimeEntry{}, err
	}

	w, err := encodeEntryWrite(input)
	if err != nil {
		return domain.TimeEntry{}, fmt.Errorf("encode entry: %w", err)
	}

	// POST expects a JSON array (batch of 1).
	payload := []wireTimeEntryWrite{w}
	var result wireBulkResult
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/time", payload, &result); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("create entry: %w", err)
	}

	if len(result.Failed) > 0 {
		return domain.TimeEntry{}, fmt.Errorf("create entry: %s", result.Failed[0].ErrorMessage)
	}
	if len(result.Succeeded) == 0 {
		return domain.TimeEntry{}, fmt.Errorf("create entry: no entry created")
	}

	// Fetch the created entry for the full domain object with resolved type names.
	return s.GetEntry(ctx, profileName, result.Succeeded[0].ID)
}
```

### Step 5.4 -- Verify tests pass

Fix any import/helper issues surfaced in step 5.2, then:

```bash
go test ./internal/svc/timesvc/ -run "TestAddEntry" -count=1
go vet ./internal/svc/timesvc/
```

### Step 5.5 -- Commit

```bash
git add internal/svc/timesvc/write.go internal/svc/timesvc/write_test.go
git commit -m "feat(timesvc): add AddEntry service method

POSTs a 1-element array to /api/time, decodes BulkResult, then
fetches the created entry by ID for full domain resolution.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: UpdateEntry service method

### Step 6.1 -- Write failing tests

Add to `internal/svc/timesvc/write_test.go`:

```go
func TestUpdateEntrySuccess(t *testing.T) {
	requests := make(map[string]string) // method+path -> body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests[r.Method+" "+r.URL.Path] = string(body)

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":2612,"ItemTitle":"Test Task",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":0,"AppName":"None","Component":2,
				"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
				"PlanID":2091,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":15.0,"Description":"original",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":70,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodPut && r.URL.Path == "/TDWebApi/api/time/999":
			// Verify the PUT body has the updated description.
			if !strings.Contains(string(body), `"Description":"updated"`) {
				t.Errorf("PUT body missing updated description: %s", body)
			}
			// Return the updated wire entry.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":2612,"ItemTitle":"Test Task",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":0,"AppName":"None","Component":2,
				"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
				"PlanID":2091,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":15.0,"Description":"updated",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":70,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	desc := "updated"
	update := domain.EntryUpdate{Description: &desc}
	entry, err := svc.UpdateEntry(context.Background(), "default", 999, update)
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}
	if entry.ID != 999 {
		t.Errorf("expected ID 999, got %d", entry.ID)
	}
	if entry.Description != "updated" {
		t.Errorf("expected description 'updated', got %q", entry.Description)
	}
}

func TestUpdateEntryNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`"Time entry not found."`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	desc := "updated"
	update := domain.EntryUpdate{Description: &desc}
	_, err := svc.UpdateEntry(context.Background(), "default", 9999, update)
	if err == nil {
		t.Fatal("expected error")
	}
}
```

### Step 6.2 -- Verify test failure

```bash
go test ./internal/svc/timesvc/ -run "TestUpdateEntry" -count=1
```

Expected: compilation failure -- `UpdateEntry` doesn't exist.

### Step 6.3 -- Implement

Add to `internal/svc/timesvc/write.go`:

```go
// UpdateEntry fetches the raw wire entry, applies the update, PUTs it back,
// and returns the updated domain entry.
func (s *Service) UpdateEntry(ctx context.Context, profileName string, id int, update domain.EntryUpdate) (domain.TimeEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.TimeEntry{}, err
	}

	// Fetch raw wire entry (not domain) so we can re-submit all wire fields.
	var raw wireTimeEntry
	path := fmt.Sprintf("/TDWebApi/api/time/%d", id)
	if err := client.DoJSON(ctx, "GET", path, nil, &raw); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("fetch entry %d: %w", id, err)
	}

	// Apply non-nil update fields.
	if update.Date != nil {
		raw.TimeDate = tdTime(*update.Date)
	}
	if update.Minutes != nil {
		raw.Minutes = float64(*update.Minutes)
	}
	if update.TimeTypeID != nil {
		raw.TimeTypeID = *update.TimeTypeID
	}
	if update.Billable != nil {
		raw.Billable = *update.Billable
	}
	if update.Description != nil {
		raw.Description = *update.Description
	}

	// PUT the modified entry.
	var updated wireTimeEntry
	if err := client.DoJSON(ctx, "PUT", path, raw, &updated); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("update entry %d: %w", id, err)
	}

	// Resolve type names and convert to domain.
	types, err := s.ListTimeTypes(ctx, profileName)
	if err != nil {
		return domain.TimeEntry{}, fmt.Errorf("resolve time types: %w", err)
	}
	entries := []wireTimeEntry{updated}
	domainEntries, err := decodeTimeEntries(entries)
	if err != nil {
		return domain.TimeEntry{}, err
	}
	resolveTimeTypeNames(domainEntries, types)
	return domainEntries[0], nil
}
```

Note: If `decodeTimeEntries` does not exist as a batch helper, you may need to use
`decodeTimeEntry` (singular) and `resolveTimeTypeNames`. Adjust to match the
actual helper signatures in `internal/svc/timesvc/entries.go`. The key pattern
is: decode the wire entry to domain, then resolve type names.

### Step 6.4 -- Verify tests pass

```bash
go test ./internal/svc/timesvc/ -run "TestUpdateEntry" -count=1
go vet ./internal/svc/timesvc/
```

### Step 6.5 -- Commit

```bash
git add internal/svc/timesvc/write.go internal/svc/timesvc/write_test.go
git commit -m "feat(timesvc): add UpdateEntry service method

GET raw wire entry, apply non-nil update fields, PUT back.
Preserves all wire-only fields (ProjectID, FunctionalRoleId, etc.)
that the domain model does not carry.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: DeleteEntry + DeleteEntries service methods

### Step 7.1 -- Write failing tests

Add to `internal/svc/timesvc/write_test.go`:

```go
func TestDeleteEntrySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/TDWebApi/api/time/999" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	err := svc.DeleteEntry(context.Background(), "default", 999)
	if err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}
}

func TestDeleteEntryNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	err := svc.DeleteEntry(context.Background(), "default", 9999)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDeleteEntriesAllSucceed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/TDWebApi/api/time/delete" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1},{"Index":1,"ID":2}],"Failed":[]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	result, err := svc.DeleteEntries(context.Background(), "default", []int{1, 2})
	if err != nil {
		t.Fatalf("DeleteEntries: %v", err)
	}
	if !result.FullSuccess() {
		t.Errorf("expected full success, got %d succeeded %d failed", len(result.Succeeded), len(result.Failed))
	}
}

func TestDeleteEntriesPartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1}],"Failed":[{"Index":1,"TimeEntryID":2,"ErrorMessage":"Could not find a time entry with an ID of 2","ErrorCode":10,"ErrorCodeName":"InvalidTimeEntryID"}]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	svc := New(mustResolvePaths(t))
	result, err := svc.DeleteEntries(context.Background(), "default", []int{1, 2})
	if err != nil {
		t.Fatalf("DeleteEntries: %v", err)
	}
	if !result.PartialSuccess() {
		t.Error("expected partial success")
	}
	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failure, got %d", len(result.Failed))
	}
	if result.Failed[0].ID != 2 {
		t.Errorf("expected failed ID 2, got %d", result.Failed[0].ID)
	}
}

func TestDeleteEntriesAutoSplitAt50(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var ids []int
		if err := json.Unmarshal(body, &ids); err != nil {
			t.Errorf("unmarshal: %v", err)
		}
		if len(ids) > 50 {
			t.Errorf("batch size %d exceeds max 50", len(ids))
		}
		// Build success response for all IDs in this batch.
		var succeeded []wireBulkSuccess
		for i, id := range ids {
			succeeded = append(succeeded, wireBulkSuccess{Index: i, ID: id})
		}
		result := wireBulkResult{Succeeded: succeeded}
		resp, _ := json.Marshal(result)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestConfig(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	// Build 51 IDs.
	ids := make([]int, 51)
	for i := range ids {
		ids[i] = i + 1
	}

	svc := New(mustResolvePaths(t))
	result, err := svc.DeleteEntries(context.Background(), "default", ids)
	if err != nil {
		t.Fatalf("DeleteEntries: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for 51 IDs, got %d", callCount)
	}
	if len(result.Succeeded) != 51 {
		t.Errorf("expected 51 succeeded, got %d", len(result.Succeeded))
	}
}
```

### Step 7.2 -- Verify test failure

```bash
go test ./internal/svc/timesvc/ -run "TestDeleteEntry" -count=1
```

Expected: compilation failure -- `DeleteEntry` and `DeleteEntries` don't exist.

### Step 7.3 -- Implement

Add to `internal/svc/timesvc/write.go`:

```go
const maxBatchSize = 50

// DeleteEntry deletes a single time entry by ID.
func (s *Service) DeleteEntry(ctx context.Context, profileName string, id int) error {
	client, err := s.clientFor(profileName)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/TDWebApi/api/time/%d", id)
	if err := client.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete entry %d: %w", id, err)
	}
	return nil
}

// DeleteEntries deletes multiple time entries in batches of 50.
// Returns a BatchResult with succeeded and failed IDs.
func (s *Service) DeleteEntries(ctx context.Context, profileName string, ids []int) (domain.BatchResult, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.BatchResult{}, err
	}

	var result domain.BatchResult

	for i := 0; i < len(ids); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		var bulk wireBulkResult
		if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/time/delete", chunk, &bulk); err != nil {
			// If the API call itself fails, treat all IDs in this chunk as failed.
			for _, id := range chunk {
				result.Failed = append(result.Failed, domain.BatchFailure{
					ID:      id,
					Message: err.Error(),
				})
			}
			continue
		}

		for _, s := range bulk.Succeeded {
			result.Succeeded = append(result.Succeeded, s.ID)
		}
		for _, f := range bulk.Failed {
			result.Failed = append(result.Failed, domain.BatchFailure{
				ID:      f.TimeEntryID,
				Message: f.ErrorMessage,
			})
		}
	}

	return result, nil
}
```

### Step 7.4 -- Verify tests pass

```bash
go test ./internal/svc/timesvc/ -run "TestDeleteEntry" -count=1
go vet ./internal/svc/timesvc/
```

### Step 7.5 -- Commit

```bash
git add internal/svc/timesvc/write.go internal/svc/timesvc/write_test.go
git commit -m "feat(timesvc): add DeleteEntry and DeleteEntries service methods

DeleteEntry: DELETE /api/time/{id}, 404 -> error.
DeleteEntries: POST /api/time/delete in batches of 50, aggregates
BulkResult across chunks.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Wire add/update/delete into entry.go parent

### Step 8.1 -- Update entry.go

Modify `internal/cli/time/entry/entry.go`:

1. Change `Short` from `"List and inspect time entries"` to `"Manage time entries"`.
2. Add `cmd.AddCommand(newAddCmd())`, `cmd.AddCommand(newUpdateCmd())`, `cmd.AddCommand(newDeleteCmd())`.

The `newAddCmd`, `newUpdateCmd`, and `newDeleteCmd` functions will be created in
Tasks 9-11. For now, create stub files so this compiles.

Create `internal/cli/time/entry/add.go`:

```go
package entry

import "github.com/spf13/cobra"

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // stub -- implemented in Task 9
		},
	}
}
```

Create `internal/cli/time/entry/update.go`:

```go
package entry

import "github.com/spf13/cobra"

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // stub -- implemented in Task 10
		},
	}
}
```

Create `internal/cli/time/entry/delete.go`:

```go
package entry

import "github.com/spf13/cobra"

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id> [<id>...]",
		Short: "Delete one or more time entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // stub -- implemented in Task 11
		},
	}
}
```

### Step 8.2 -- Verify compilation and existing tests still pass

```bash
go vet ./internal/cli/time/entry/
go test ./internal/cli/time/entry/ -count=1
```

### Step 8.3 -- Commit

```bash
git add internal/cli/time/entry/entry.go internal/cli/time/entry/add.go internal/cli/time/entry/update.go internal/cli/time/entry/delete.go
git commit -m "feat(cli): wire add/update/delete stubs into entry command

Updates Short to 'Manage time entries'. Stubs will be replaced with
full implementations in Tasks 9-11.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: CLI `add` command

This is the largest CLI task. It implements the full `tdx time entry add`
command with flag parsing, target resolution, pre-write validation, `--dry-run`,
and human/JSON output.

### Step 9.1 -- Extract shared printEntry helper + write CLI test helper

First, extract the human-format entry printer from `show.go` into a shared
helper so `add.go`, `update.go`, and `show.go` can all use it.

Create `internal/cli/time/entry/print.go`:

```go
package entry

import (
	"fmt"
	"io"

	"github.com/iainmoffat/tdx/internal/domain"
)

// printEntry writes a human-readable entry to w, matching the show command format.
func printEntry(w io.Writer, entry domain.TimeEntry) {
	fmt.Fprintf(w, "entry:        %d\n", entry.ID)
	fmt.Fprintf(w, "date:         %s\n", entry.Date.Format("2006-01-02"))
	fmt.Fprintf(w, "hours:        %.2f\n", entry.Hours())
	fmt.Fprintf(w, "minutes:      %d\n", entry.Minutes)
	fmt.Fprintf(w, "type:         %s\n", entry.TimeType.Name)
	fmt.Fprintf(w, "target:       %s\n", targetLabel(entry.Target))
	if entry.Description != "" {
		fmt.Fprintf(w, "description:  %s\n", entry.Description)
	}
	fmt.Fprintf(w, "status:       %s\n", entry.ReportStatus)
	fmt.Fprintf(w, "billable:     %t\n", entry.Billable)
}
```

Then update `show.go` to call `printEntry(w, entry)` instead of the inline
`fmt.Fprintf` calls.

Create `internal/cli/time/entry/test_helpers_test.go`:

```go
package entry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeTestProfile creates config and credentials files pointing at the given URL.
func writeTestProfile(t *testing.T, dir, url string) {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(
		"defaultProfile: default\nprofiles:\n- name: default\n  tenantBaseURL: %s\n", url,
	)), 0644); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(dir, "credentials.yaml")
	if err := os.WriteFile(credPath, []byte("tokens:\n  default: test-token\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
```

### Step 9.2 -- Write failing tests

Create `internal/cli/time/entry/add_test.go`:

```go
package entry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// addTestServer returns an httptest.Server that handles the endpoints needed
// for the add command: POST /api/time, GET /api/time/{id}, GET /api/time/types,
// GET /auth/getuser, GET /api/time/locked.
func addTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true},{"ID":6,"Name":"Meetings","IsActive":true}]`))

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":999}],"Failed":[]}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":100,"ItemTitle":"Test Ticket",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":5,"AppName":"Test App","Component":9,
				"TicketID":100,"ProjectID":0,"ProjectName":"",
				"PlanID":0,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"did work",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/locked"):
			// No locked days.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			// Week report with no submitted status.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":0}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAddCmdTicketSuccess(t *testing.T) {
	srv := addTestServer(t)
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"add",
		"--date", "2026-04-11",
		"--hours", "1",
		"--type", "Development",
		"--ticket", "100",
		"--app", "5",
		"-d", "did work",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v\nOutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "created entry 999") {
		t.Errorf("expected 'created entry 999' in output, got:\n%s", out.String())
	}
}

func TestAddCmdProjectTaskSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time":
			// Verify the wire body has projectTask fields.
			body, _ := io.ReadAll(r.Body)
			var entries []json.RawMessage
			json.Unmarshal(body, &entries)
			var entry map[string]interface{}
			json.Unmarshal(entries[0], &entry)
			if int(entry["Component"].(float64)) != 2 {
				t.Errorf("expected Component 2, got %v", entry["Component"])
			}
			if int(entry["ProjectID"].(float64)) != 54 {
				t.Errorf("expected ProjectID 54, got %v", entry["ProjectID"])
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":888}],"Failed":[]}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/888":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":888,"ItemID":2612,"ItemTitle":"Task",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":0,"AppName":"None","Component":2,
				"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
				"PlanID":2091,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":30.0,"Description":"task work",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":70,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/locked"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":0}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"add",
		"--date", "2026-04-11",
		"--minutes", "30",
		"--type", "Development",
		"--project", "54",
		"--plan", "2091",
		"--task", "2612",
		"-d", "task work",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v\nOutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "created entry 888") {
		t.Errorf("expected 'created entry 888', got:\n%s", out.String())
	}
}

func TestAddCmdMissingRequiredFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing date",
			args: []string{"add", "--hours", "1", "--type", "Dev", "--ticket", "100", "--app", "5"},
			want: "--date is required",
		},
		{
			name: "missing hours and minutes",
			args: []string{"add", "--date", "2026-04-11", "--type", "Dev", "--ticket", "100", "--app", "5"},
			want: "exactly one of --hours or --minutes is required",
		},
		{
			name: "both hours and minutes",
			args: []string{"add", "--date", "2026-04-11", "--hours", "1", "--minutes", "60", "--type", "Dev", "--ticket", "100", "--app", "5"},
			want: "exactly one of --hours or --minutes is required",
		},
		{
			name: "missing type",
			args: []string{"add", "--date", "2026-04-11", "--hours", "1", "--ticket", "100", "--app", "5"},
			want: "--type is required",
		},
		{
			name: "no target",
			args: []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev"},
			want: "exactly one of --ticket, --project, or --workspace is required",
		},
		{
			name: "ticket without app",
			args: []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev", "--ticket", "100"},
			want: "--app is required with --ticket",
		},
		{
			name: "plan without project and task",
			args: []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev", "--project", "54", "--plan", "2091"},
			want: "--plan requires both --project and --task",
		},
		{
			name: "project task without plan",
			args: []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev", "--project", "54", "--task", "2612"},
			want: "--task with --project requires --plan",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCmd()
			cmd.SetArgs(tt.args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error, got nil. Output:\n%s", out.String())
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestAddCmdDryRun(t *testing.T) {
	postCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time":
			postCalled = true
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/locked"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":0}`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"add",
		"--date", "2026-04-11",
		"--hours", "1",
		"--type", "Development",
		"--ticket", "100",
		"--app", "5",
		"--dry-run",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if postCalled {
		t.Error("expected no POST /api/time call in dry-run mode")
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("expected 'dry run' in output, got:\n%s", out.String())
	}
}
```

### Step 9.3 -- Verify test failure

```bash
go test ./internal/cli/time/entry/ -run "TestAddCmd" -count=1
```

Expected: failures because `newAddCmd` is a stub.

### Step 9.4 -- Implement

Replace the stub `internal/cli/time/entry/add.go` with the full implementation:

```go
package entry

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	authsvc "github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

func newAddCmd() *cobra.Command {
	var (
		dateFlag    string
		hoursFlag   float64
		minutesFlag int
		typeFlag    string
		ticketFlag  int
		projectFlag int
		workspFlag  int
		appFlag     int
		taskFlag    int
		planFlag    int
		issueFlag   int
		descFlag    string
		dryRun      bool
		jsonFlag    bool
		profileFlag string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, addFlags{
				date:    dateFlag,
				hours:   hoursFlag,
				minutes: minutesFlag,
				typeName: typeFlag,
				ticket:  ticketFlag,
				project: projectFlag,
				workspace: workspFlag,
				app:     appFlag,
				task:    taskFlag,
				plan:    planFlag,
				issue:   issueFlag,
				desc:    descFlag,
				dryRun:  dryRun,
				json:    jsonFlag,
				profile: profileFlag,
			})
		},
	}

	cmd.Flags().StringVar(&dateFlag, "date", "", "entry date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&hoursFlag, "hours", 0, "duration in hours (e.g. 1.5)")
	cmd.Flags().IntVar(&minutesFlag, "minutes", 0, "duration in minutes")
	cmd.Flags().StringVar(&typeFlag, "type", "", "time type name (e.g. Development)")
	cmd.Flags().IntVar(&ticketFlag, "ticket", 0, "ticket ID")
	cmd.Flags().IntVar(&projectFlag, "project", 0, "project ID")
	cmd.Flags().IntVar(&workspFlag, "workspace", 0, "workspace ID")
	cmd.Flags().IntVar(&appFlag, "app", 0, "app ID (required with --ticket)")
	cmd.Flags().IntVar(&taskFlag, "task", 0, "task ID")
	cmd.Flags().IntVar(&planFlag, "plan", 0, "plan ID (required with --project --task)")
	cmd.Flags().IntVar(&issueFlag, "issue", 0, "issue ID (with --project)")
	cmd.Flags().StringVarP(&descFlag, "description", "d", "", "description")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without creating")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")

	return cmd
}

type addFlags struct {
	date      string
	hours     float64
	minutes   int
	typeName  string
	ticket    int
	project   int
	workspace int
	app       int
	task      int
	plan      int
	issue     int
	desc      string
	dryRun    bool
	json      bool
	profile   string
}

func runAdd(cmd *cobra.Command, f addFlags) error {
	// --- Flag validation ---
	if f.date == "" {
		return fmt.Errorf("--date is required")
	}
	date, err := time.Parse("2006-01-02", f.date)
	if err != nil {
		return fmt.Errorf("invalid --date %q: expected YYYY-MM-DD", f.date)
	}

	hasHours := f.hours != 0
	hasMins := f.minutes != 0
	if hasHours == hasMins {
		return fmt.Errorf("exactly one of --hours or --minutes is required")
	}
	minutes := f.minutes
	if hasHours {
		minutes = int(math.Round(f.hours * 60))
	}
	if minutes <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	if f.typeName == "" {
		return fmt.Errorf("--type is required")
	}

	// --- Target resolution ---
	targetCount := 0
	if f.ticket > 0 { targetCount++ }
	if f.project > 0 { targetCount++ }
	if f.workspace > 0 { targetCount++ }
	if targetCount != 1 {
		return fmt.Errorf("exactly one of --ticket, --project, or --workspace is required")
	}

	var target domain.Target
	var projectID int

	switch {
	case f.ticket > 0:
		if f.app <= 0 {
			return fmt.Errorf("--app is required with --ticket")
		}
		if f.task > 0 {
			target = domain.Target{Kind: domain.TargetTicketTask, AppID: f.app, ItemID: f.ticket, TaskID: f.task}
		} else {
			target = domain.Target{Kind: domain.TargetTicket, AppID: f.app, ItemID: f.ticket}
		}

	case f.project > 0:
		if f.plan > 0 && f.task <= 0 {
			return fmt.Errorf("--plan requires both --project and --task")
		}
		if f.task > 0 && f.plan <= 0 {
			return fmt.Errorf("--task with --project requires --plan")
		}
		if f.task > 0 {
			// projectTask
			target = domain.Target{Kind: domain.TargetProjectTask, ItemID: f.plan, TaskID: f.task}
			projectID = f.project
		} else if f.issue > 0 {
			target = domain.Target{Kind: domain.TargetProjectIssue, ItemID: f.issue}
			projectID = f.project
		} else {
			target = domain.Target{Kind: domain.TargetProject, ItemID: f.project}
		}

	case f.workspace > 0:
		target = domain.Target{Kind: domain.TargetWorkspace, ItemID: f.workspace}
	}

	// --- Resolve profile, user, type ---
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	user, err := auth.WhoAmI(cmd.Context(), profileName)
	if err != nil {
		return fmt.Errorf("could not resolve current user: %w", err)
	}

	types, err := tsvc.ListTimeTypes(cmd.Context(), profileName)
	if err != nil {
		return fmt.Errorf("lookup time types: %w", err)
	}
	match, ok := domain.FindTimeTypeByName(types, f.typeName)
	if !ok {
		return fmt.Errorf("no time type named %q", f.typeName)
	}

	// --- Pre-write validation ---
	locked, err := tsvc.GetLockedDays(cmd.Context(), profileName, date, date)
	if err != nil {
		return fmt.Errorf("check locked days: %w", err)
	}
	for _, ld := range locked {
		if ld.Date.Equal(date) {
			return fmt.Errorf("%w: %s", domain.ErrDayLocked, date.Format("2006-01-02"))
		}
	}

	weekReport, err := tsvc.GetWeekReport(cmd.Context(), profileName, date)
	if err != nil {
		return fmt.Errorf("check week report: %w", err)
	}
	if weekReport.Status != domain.ReportStatusOpen {
		return fmt.Errorf("%w: week of %s is %s", domain.ErrWeekSubmitted, date.Format("2006-01-02"), weekReport.Status)
	}

	// --- Build input ---
	input := domain.EntryInput{
		UserUID:     user.UID,
		Date:        date,
		Minutes:     minutes,
		TimeTypeID:  match.ID,
		Billable:    false,
		Target:      target,
		ProjectID:   projectID,
		Description: f.desc,
	}

	w := cmd.OutOrStdout()

	// --- Dry run ---
	if f.dryRun {
		fmt.Fprintln(w, "dry run -- no entry created")
		fmt.Fprintf(w, "date:         %s\n", date.Format("2006-01-02"))
		fmt.Fprintf(w, "minutes:      %d\n", minutes)
		fmt.Fprintf(w, "type:         %s\n", match.Name)
		fmt.Fprintf(w, "target:       %s\n", target.Kind)
		if f.desc != "" {
			fmt.Fprintf(w, "description:  %s\n", f.desc)
		}
		return nil
	}

	// --- Create entry ---
	entry, err := tsvc.AddEntry(cmd.Context(), profileName, input)
	if err != nil {
		return fmt.Errorf("add entry: %w", err)
	}

	// --- Output ---
	format := render.ResolveFormat(render.Flags{JSON: f.json})
	if format == render.FormatJSON {
		return render.JSON(w, struct {
			Schema string           `json:"schema"`
			Entry  domain.TimeEntry `json:"entry"`
		}{
			Schema: "tdx.v1.entryAdd",
			Entry:  entry,
		})
	}

	fmt.Fprintf(w, "created entry %d\n", entry.ID)
	printEntry(w, entry)
	return nil
}
```

Note: adjust imports and function names to match actual codebase patterns. The
key patterns are from the existing list.go and show.go commands.

### Step 9.5 -- Verify tests pass

```bash
go test ./internal/cli/time/entry/ -run "TestAddCmd" -count=1
go vet ./internal/cli/time/entry/
```

### Step 9.6 -- Commit

```bash
git add internal/cli/time/entry/add.go internal/cli/time/entry/add_test.go internal/cli/time/entry/print.go internal/cli/time/entry/show.go internal/cli/time/entry/test_helpers_test.go
git commit -m "feat(cli): implement 'tdx time entry add' command

Full flag parsing, target resolution, pre-write validation (locked days,
week report status), --dry-run, human and JSON output.

Extracts printEntry() helper shared by show.go, add.go, update.go.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: CLI `update` command

### Step 10.1 -- Write failing tests

Create `internal/cli/time/entry/update_test.go`:

```go
package entry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func updateTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":100,"ItemTitle":"Ticket",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":5,"AppName":"App","Component":9,
				"TicketID":100,"ProjectID":0,"ProjectName":"",
				"PlanID":0,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"original",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodPut && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":100,"ItemTitle":"Ticket",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":5,"AppName":"App","Component":9,
				"TicketID":100,"ProjectID":0,"ProjectName":"",
				"PlanID":0,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"updated desc",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/locked"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":0}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestUpdateCmdDescription(t *testing.T) {
	srv := updateTestServer(t)
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"update", "999", "-d", "updated desc"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v\nOutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "updated entry 999") {
		t.Errorf("expected 'updated entry 999', got:\n%s", out.String())
	}
}

func TestUpdateCmdNothingToUpdate(t *testing.T) {
	cmd := NewCmd()
	cmd.SetArgs([]string{"update", "999"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nothing to update")
	}
	if !strings.Contains(err.Error(), "nothing to update") {
		t.Errorf("expected 'nothing to update', got: %v", err)
	}
}

func TestUpdateCmdDryRun(t *testing.T) {
	putCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":100,"ItemTitle":"Ticket",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":5,"AppName":"App","Component":9,
				"TicketID":100,"ProjectID":0,"ProjectName":"",
				"PlanID":0,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"original",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/locked"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":0}`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"update", "999", "-d", "new desc", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if putCalled {
		t.Error("expected no PUT call in dry-run mode")
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("expected 'dry run' in output, got:\n%s", out.String())
	}
}
```

### Step 10.2 -- Verify test failure

```bash
go test ./internal/cli/time/entry/ -run "TestUpdateCmd" -count=1
```

Expected: failures because `newUpdateCmd` is a stub.

### Step 10.3 -- Implement

Replace the stub `internal/cli/time/entry/update.go`:

```go
package entry

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	authsvc "github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

func newUpdateCmd() *cobra.Command {
	var (
		dateFlag    string
		hoursFlag   float64
		minutesFlag int
		typeFlag    string
		descFlag    string
		dryRun      bool
		jsonFlag    bool
		profileFlag string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing time entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid entry ID %q: %w", args[0], err)
			}
			return runUpdate(cmd, id, updateFlags{
				date:     dateFlag,
				hours:    hoursFlag,
				minutes:  minutesFlag,
				typeName: typeFlag,
				desc:     descFlag,
				dryRun:   dryRun,
				json:     jsonFlag,
				profile:  profileFlag,
			})
		},
	}

	cmd.Flags().StringVar(&dateFlag, "date", "", "new date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&hoursFlag, "hours", 0, "new duration in hours")
	cmd.Flags().IntVar(&minutesFlag, "minutes", 0, "new duration in minutes")
	cmd.Flags().StringVar(&typeFlag, "type", "", "new time type name")
	cmd.Flags().StringVarP(&descFlag, "description", "d", "", "new description")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without updating")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")

	return cmd
}

type updateFlags struct {
	date     string
	hours    float64
	minutes  int
	typeName string
	desc     string
	dryRun   bool
	json     bool
	profile  string
}

func runUpdate(cmd *cobra.Command, id int, f updateFlags) error {
	// Build EntryUpdate from flags.
	var update domain.EntryUpdate

	if f.date != "" {
		d, err := time.Parse("2006-01-02", f.date)
		if err != nil {
			return fmt.Errorf("invalid --date %q: expected YYYY-MM-DD", f.date)
		}
		update.Date = &d
	}
	if f.hours != 0 {
		mins := int(math.Round(f.hours * 60))
		update.Minutes = &mins
	}
	if f.minutes != 0 {
		update.Minutes = &f.minutes
	}
	if cmd.Flags().Changed("description") {
		update.Description = &f.desc
	}

	// Type resolution happens after we check emptiness, since we need the service.
	hasType := f.typeName != ""

	if update.IsEmpty() && !hasType {
		return fmt.Errorf("nothing to update: set at least one of --date, --hours, --minutes, --type, --description")
	}

	// --- Resolve profile ---
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	// Resolve type if specified.
	if hasType {
		types, err := tsvc.ListTimeTypes(cmd.Context(), profileName)
		if err != nil {
			return fmt.Errorf("lookup time types: %w", err)
		}
		match, ok := domain.FindTimeTypeByName(types, f.typeName)
		if !ok {
			return fmt.Errorf("no time type named %q", f.typeName)
		}
		update.TimeTypeID = &match.ID
	}

	// Fetch current entry for pre-write validation and dry-run display.
	current, err := tsvc.GetEntry(cmd.Context(), profileName, id)
	if err != nil {
		return fmt.Errorf("fetch entry %d: %w", id, err)
	}

	// Determine the date to validate (new date if changing, else current).
	checkDate := current.Date
	if update.Date != nil {
		checkDate = *update.Date
	}

	// --- Pre-write validation ---
	locked, err := tsvc.GetLockedDays(cmd.Context(), profileName, checkDate, checkDate)
	if err != nil {
		return fmt.Errorf("check locked days: %w", err)
	}
	for _, ld := range locked {
		if ld.Date.Equal(checkDate) {
			return fmt.Errorf("%w: %s", domain.ErrDayLocked, checkDate.Format("2006-01-02"))
		}
	}

	weekReport, err := tsvc.GetWeekReport(cmd.Context(), profileName, checkDate)
	if err != nil {
		return fmt.Errorf("check week report: %w", err)
	}
	if weekReport.Status != domain.ReportStatusOpen {
		return fmt.Errorf("%w: week of %s is %s", domain.ErrWeekSubmitted, checkDate.Format("2006-01-02"), weekReport.Status)
	}

	w := cmd.OutOrStdout()

	// --- Dry run ---
	if f.dryRun {
		fmt.Fprintln(w, "dry run -- no changes made")
		fmt.Fprintf(w, "entry:  %d\n", id)
		if update.Date != nil {
			fmt.Fprintf(w, "date:   %s -> %s\n", current.Date.Format("2006-01-02"), update.Date.Format("2006-01-02"))
		}
		if update.Minutes != nil {
			fmt.Fprintf(w, "minutes: %d -> %d\n", current.Minutes, *update.Minutes)
		}
		if update.TimeTypeID != nil {
			fmt.Fprintf(w, "type:   %s -> %s\n", current.TimeType.Name, f.typeName)
		}
		if update.Description != nil {
			fmt.Fprintf(w, "description: %q -> %q\n", current.Description, *update.Description)
		}
		return nil
	}

	// --- Update ---
	entry, err := tsvc.UpdateEntry(cmd.Context(), profileName, id, update)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}

	// --- Output ---
	format := render.ResolveFormat(render.Flags{JSON: f.json})
	if format == render.FormatJSON {
		return render.JSON(w, struct {
			Schema string           `json:"schema"`
			Entry  domain.TimeEntry `json:"entry"`
		}{
			Schema: "tdx.v1.entryUpdate",
			Entry:  entry,
		})
	}

	fmt.Fprintf(w, "updated entry %d\n", entry.ID)
	printEntry(w, entry)
	return nil
}
```

### Step 10.4 -- Verify tests pass

```bash
go test ./internal/cli/time/entry/ -run "TestUpdateCmd" -count=1
go vet ./internal/cli/time/entry/
```

### Step 10.5 -- Commit

```bash
git add internal/cli/time/entry/update.go internal/cli/time/entry/update_test.go
git commit -m "feat(cli): implement 'tdx time entry update' command

Positional entry ID, optional flags for date/hours/minutes/type/desc.
Pre-write validation, --dry-run with old->new diff, human and JSON output.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: CLI `delete` command

### Step 11.1 -- Write failing tests

Create `internal/cli/time/entry/delete_test.go`:

```go
package entry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func deleteTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func TestDeleteCmdSingleSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodDelete && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"delete", "999"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v\nOutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "deleted entry 999") {
		t.Errorf("expected 'deleted entry 999', got:\n%s", out.String())
	}
}

func TestDeleteCmdSingleNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNotFound)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"delete", "9999"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestDeleteCmdMultiSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time/delete":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1},{"Index":1,"ID":2},{"Index":2,"ID":3}],"Failed":[]}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"delete", "1", "2", "3"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v\nOutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "deleted 3 entries") {
		t.Errorf("expected 'deleted 3 entries', got:\n%s", out.String())
	}
}

func TestDeleteCmdMultiPartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time/delete":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1}],"Failed":[{"Index":1,"TimeEntryID":2,"ErrorMessage":"Could not find a time entry with an ID of 2","ErrorCode":10,"ErrorCodeName":"InvalidTimeEntryID"}]}`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"delete", "1", "2"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	// Partial failure should return an error (which the caller can map to exit 2).
	if err == nil {
		t.Fatal("expected error for partial failure")
	}
	if !strings.Contains(err.Error(), "partial") {
		t.Errorf("expected error containing 'partial', got: %v", err)
	}
}

func TestDeleteCmdDryRunSingle(t *testing.T) {
	deleteCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":100,"ItemTitle":"Ticket",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":5,"AppName":"App","Component":9,
				"TicketID":100,"ProjectID":0,"ProjectName":"",
				"PlanID":0,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"original",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		case r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time/delete":
			deleteCalled = true
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestProfile(t, dir, srv.URL)
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"delete", "999", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if deleteCalled {
		t.Error("expected no DELETE call in dry-run mode")
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("expected 'dry run' in output, got:\n%s", out.String())
	}
}
```

### Step 11.2 -- Verify test failure

```bash
go test ./internal/cli/time/entry/ -run "TestDeleteCmd" -count=1
```

Expected: failures because `newDeleteCmd` is a stub.

### Step 11.3 -- Implement

Replace the stub `internal/cli/time/entry/delete.go`:

```go
package entry

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	authsvc "github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// ErrPartialDelete is returned when a batch delete has partial failures.
// The caller should map this to exit code 2.
type ErrPartialDelete struct {
	Succeeded int
	Failed    int
	Message   string
}

func (e *ErrPartialDelete) Error() string {
	return e.Message
}

func newDeleteCmd() *cobra.Command {
	var (
		dryRun      bool
		jsonFlag    bool
		profileFlag string
	)

	cmd := &cobra.Command{
		Use:   "delete <id> [<id>...]",
		Short: "Delete one or more time entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]int, len(args))
			for i, arg := range args {
				id, err := strconv.Atoi(arg)
				if err != nil {
					return fmt.Errorf("invalid entry ID %q: %w", arg, err)
				}
				ids[i] = id
			}
			return runDelete(cmd, ids, deleteFlags{
				dryRun:  dryRun,
				json:    jsonFlag,
				profile: profileFlag,
			})
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without deleting")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")

	return cmd
}

type deleteFlags struct {
	dryRun  bool
	json    bool
	profile string
}

func runDelete(cmd *cobra.Command, ids []int, f deleteFlags) error {
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	// --- Dry run ---
	if f.dryRun {
		fmt.Fprintln(w, "dry run -- no entries deleted")
		// Fetch and display each entry.
		for _, id := range ids {
			entry, err := tsvc.GetEntry(cmd.Context(), profileName, id)
			if err != nil {
				fmt.Fprintf(w, "entry %d: %v\n", id, err)
				continue
			}
			fmt.Fprintf(w, "\nwould delete:\n")
			printEntry(w, entry)
		}
		return nil
	}

	// --- Single delete ---
	if len(ids) == 1 {
		if err := tsvc.DeleteEntry(cmd.Context(), profileName, ids[0]); err != nil {
			return fmt.Errorf("delete entry %d: %w", ids[0], err)
		}

		format := render.ResolveFormat(render.Flags{JSON: f.json})
		if format == render.FormatJSON {
			return render.JSON(w, struct {
				Schema string `json:"schema"`
				ID     int    `json:"id"`
			}{
				Schema: "tdx.v1.entryDelete",
				ID:     ids[0],
			})
		}

		fmt.Fprintf(w, "deleted entry %d\n", ids[0])
		return nil
	}

	// --- Batch delete ---
	result, err := tsvc.DeleteEntries(cmd.Context(), profileName, ids)
	if err != nil {
		return fmt.Errorf("delete entries: %w", err)
	}

	format := render.ResolveFormat(render.Flags{JSON: f.json})
	if format == render.FormatJSON {
		return render.JSON(w, struct {
			Schema    string `json:"schema"`
			Succeeded []int  `json:"succeeded"`
			Failed    []struct {
				ID      int    `json:"id"`
				Message string `json:"message"`
			} `json:"failed"`
		}{
			Schema:    "tdx.v1.entryDeleteBatch",
			Succeeded: result.Succeeded,
		})
	}

	if result.FullSuccess() {
		fmt.Fprintf(w, "deleted %d entries\n", len(result.Succeeded))
		return nil
	}

	// Partial or total failure.
	if len(result.Succeeded) > 0 {
		fmt.Fprintf(w, "deleted %d entries\n", len(result.Succeeded))
	}
	fmt.Fprintf(w, "failed to delete %d entries:\n", len(result.Failed))
	for _, f := range result.Failed {
		fmt.Fprintf(w, "  %d: %s\n", f.ID, f.Message)
	}

	if result.PartialSuccess() {
		return &ErrPartialDelete{
			Succeeded: len(result.Succeeded),
			Failed:    len(result.Failed),
			Message:   fmt.Sprintf("partial delete: %d succeeded, %d failed", len(result.Succeeded), len(result.Failed)),
		}
	}
	return fmt.Errorf("all %d deletes failed", len(result.Failed))
}
```

### Step 11.4 -- Verify tests pass

```bash
go test ./internal/cli/time/entry/ -run "TestDeleteCmd" -count=1
go vet ./internal/cli/time/entry/
```

### Step 11.5 -- Commit

```bash
git add internal/cli/time/entry/delete.go internal/cli/time/entry/delete_test.go
git commit -m "feat(cli): implement 'tdx time entry delete' command

Single delete via DELETE /api/time/{id}, batch delete via
POST /api/time/delete with auto-split at 50. Partial failure
returns ErrPartialDelete for exit code 2. --dry-run fetches
and displays entries without deleting.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: Walkthrough script extension

### Step 12.1 -- Read current walkthrough script

Read `scripts/walkthrough.sh` to understand the current structure and find the
right insertion point (after read-only steps, before failure cases).

### Step 12.2 -- Add write operation steps

Add the following to `scripts/walkthrough.sh`, after the existing read-only
steps and before any failure-case steps:

```bash
# ---------------------------------------------------------------------------
# Phase 3: Write operations (create → read → update → read → delete → verify)
# ---------------------------------------------------------------------------

TDX_WALKTHROUGH_PROJECT="${TDX_WALKTHROUGH_PROJECT:-54}"
TDX_WALKTHROUGH_PLAN="${TDX_WALKTHROUGH_PLAN:-2091}"
TDX_WALKTHROUGH_TASK="${TDX_WALKTHROUGH_TASK:-2612}"

CREATED_ENTRY_ID=""

# Cleanup trap: delete the test entry if it was created.
cleanup_entry() {
  if [ -n "$CREATED_ENTRY_ID" ]; then
    echo "Cleaning up: deleting test entry $CREATED_ENTRY_ID"
    "$BIN" time entry delete "$CREATED_ENTRY_ID" --profile default 2>/dev/null || true
  fi
}
trap cleanup_entry EXIT

# Step: add a time entry
ADD_OUTPUT=$("$BIN" time entry add \
  --date "$WALKTHROUGH_WEEK" \
  --minutes 15 \
  --type "Development" \
  --project "$TDX_WALKTHROUGH_PROJECT" \
  --plan "$TDX_WALKTHROUGH_PLAN" \
  --task "$TDX_WALKTHROUGH_TASK" \
  -d "walkthrough test entry" \
  --profile default 2>&1)
ADD_EXIT=$?

if [ $ADD_EXIT -ne 0 ]; then
  echo "FAIL: entry add (exit $ADD_EXIT)"
  echo "$ADD_OUTPUT"
  exit 1
fi

# Extract the entry ID from "created entry <id>" output.
CREATED_ENTRY_ID=$(echo "$ADD_OUTPUT" | grep -o 'created entry [0-9]*' | grep -o '[0-9]*')
if [ -z "$CREATED_ENTRY_ID" ]; then
  echo "FAIL: could not extract entry ID from add output"
  echo "$ADD_OUTPUT"
  exit 1
fi
echo "PASS: entry add (created entry $CREATED_ENTRY_ID)"

# Step: verify entry exists via show
step "entry show created" \
  "$BIN time entry show $CREATED_ENTRY_ID --profile default" \
  "walkthrough test entry"

# Step: update the entry description
step "entry update description" \
  "$BIN time entry update $CREATED_ENTRY_ID -d 'updated by walkthrough' --profile default" \
  "updated entry $CREATED_ENTRY_ID"

# Step: verify update via show
step "entry show after update" \
  "$BIN time entry show $CREATED_ENTRY_ID --profile default" \
  "updated by walkthrough"

# Step: delete the entry
step "entry delete" \
  "$BIN time entry delete $CREATED_ENTRY_ID --profile default" \
  "deleted entry $CREATED_ENTRY_ID"

# Clear the cleanup trap since we successfully deleted.
CREATED_ENTRY_ID=""

# Step: verify deletion (should fail with not found)
step "entry show after delete (expect not found)" \
  "$BIN time entry show $CREATED_ENTRY_ID_SAVED --profile default" \
  "not found" \
  1
```

Note: The exact insertion point and variable handling may need minor adjustment
based on the actual script structure. Read the script first in step 12.1 to
confirm the insertion point. The key requirement is:
1. Write steps go after read-only steps, before failure cases.
2. A cleanup trap deletes the test entry on script exit.
3. The entry ID is captured from the `add` output and reused in subsequent steps.

### Step 12.3 -- Verify script syntax

```bash
bash -n scripts/walkthrough.sh
```

### Step 12.4 -- Commit

```bash
git add scripts/walkthrough.sh
git commit -m "test(walkthrough): add Phase 3 write operation steps

Create -> show -> update -> show -> delete -> verify-gone lifecycle.
Cleanup trap ensures test entry is deleted even on failure.
Uses TDX_WALKTHROUGH_PROJECT/PLAN/TASK env vars (defaults to UFL tenant).

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 13: Final manual smoke test

This is a verification task, not a code task. Run these commands:

### Step 13.1 -- Run all tests

```bash
go test ./... -count=1
```

All tests must pass. Fix any failures and commit fixes.

### Step 13.2 -- Run vet and build

```bash
go vet ./...
go build -o tdx .
```

### Step 13.3 -- Verify no new dependencies

```bash
git diff main -- go.mod go.sum
```

Expected: no changes (or only the module line if the branch diverges). No new
`require` lines.

### Step 13.4 -- Run walkthrough against live tenant

```bash
export TDX_WALKTHROUGH_TOKEN="<your-token>"
export TDX_WALKTHROUGH_URL="https://ufl.teamdynamix.com"
bash scripts/walkthrough.sh
```

All steps must pass. Fix any failures, commit fixes, and re-run until clean.

### Step 13.5 -- Push and open PR

```bash
git push -u origin phase-3-write-ops

gh pr create \
  --base phase-2-read-ops \
  --title "Phase 3: Write operations (add/update/delete)" \
  --body "$(cat <<'EOF'
## Summary
- Add `tdx time entry add` with full target resolution, pre-write validation, --dry-run
- Add `tdx time entry update` with partial field updates, old->new diff in --dry-run
- Add `tdx time entry delete` with single + batch (auto-split at 50), partial-success exit 2
- Fix Target.Validate() to only require AppID for ticket kinds
- New domain types: BatchResult, EntryInput, EntryUpdate
- Wire encode/decode round-trip for all 8 target kinds
- Walkthrough script extended with full CRUD lifecycle

## Test plan
- [ ] `go test ./... -count=1` -- all green
- [ ] `go vet ./...` -- clean
- [ ] `go build` -- no errors
- [ ] `git diff main -- go.mod go.sum` -- no new deps
- [ ] `scripts/walkthrough.sh` against live UFL tenant -- all steps pass

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```
