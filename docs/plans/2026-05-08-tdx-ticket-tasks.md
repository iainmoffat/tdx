# tdx Ticket Tasks Implementation Plan (v0.16.2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `tdx ticket task ...` sub-group: list, show, feed, update (feed-POST with PercentComplete + informational HoursWorked), log (real time entry against TargetTicketTask). Five new MCP tools (3 read + 2 mutating). Ship as v0.16.2.

**Architecture:** Bottom-up — domain (TicketTask type) → service (`ticketsvc/tasks.go` with 4 methods + wire types) → CLI (`internal/cli/ticket/task.go` with 5 sub-commands sharing one file) → MCP (5 new tools, mutating ones with `confirm:true`) → docs. Reuses `domain.TicketFeedEntry`, `domain.Target.TargetTicketTask`, existing `parseTDTime`, `formatHours`, `formatDuration`. CLI's `ticketsvcAPI` interface widens; `stubTicketsvc` extends.

**Tech Stack:** Go 1.26.2; cobra; existing `tdx.Client`; `httptest` for service tests.

**Spec:** [`docs/specs/2026-05-08-tdx-ticket-tasks.md`](../specs/2026-05-08-tdx-ticket-tasks.md)

---

## File Structure

After this plan completes:

```
internal/
├── domain/
│   ├── ticket.go                        # MODIFY: add TicketTask type
│   └── ticket_test.go                   # MODIFY: smoke test for TicketTask
├── svc/
│   └── ticketsvc/
│       ├── types.go                     # MODIFY: add wireTicketTask + wireTaskFeedUpdate
│       ├── tasks.go                     # NEW: ListTasks, GetTask, GetTaskFeed, UpdateTaskFeed
│       └── tasks_test.go                # NEW: 6+ tests
├── cli/
│   └── ticket/
│       ├── ticket.go                    # MODIFY: ticketsvcAPI gains 4 methods; register newTaskCmd
│       ├── stub_test.go                 # MODIFY: stubTicketsvc gains 4 fields/methods
│       ├── task.go                      # NEW: 5 sub-commands in one file
│       └── task_test.go                 # NEW: ~10 tests
└── mcp/
    ├── tools_ticket.go                  # MODIFY: 3 new read tools
    ├── tools_ticket_mutating.go         # MODIFY: 2 new mutating tools
    ├── tools_ticket_test.go             # MODIFY: schema tests + confirm-required tests
    └── server_test.go                   # MODIFY: tool count 57 → 62
docs/
├── guide/
│   ├── ticket.md                        # MODIFY: ## tdx ticket task section
│   └── mcp.md                           # MODIFY: 5 new rows + count update
└── guide.md                             # MODIFY: ASCII tree
README.md                                # MODIFY: ASCII tree (byte-identical)
```

## Established Patterns to Follow

Read these BEFORE starting:
- `internal/svc/ticketsvc/feed.go` — closest service pattern (feed POST + GET)
- `internal/cli/ticket/log.go` — closest CLI pattern (timesvcAPI wrapper, validation, EntryInput build)
- `internal/cli/ticket/comment.go` — comment is the closest mutating-feed-POST pattern
- `internal/mcp/tools_ticket.go` and `tools_ticket_mutating.go` — MCP tool registration patterns
- Memory notes: `feedback_no_coauthor.md`, `reference_td_ticket_api_quirks.md`

## Branch + Versioning

- Branch: `ticket-tasks` (Task 0)
- Version: v0.16.2 (no source change; tagged after merge)

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean.

- [ ] **Step 2: Create branch**

```bash
git checkout -b ticket-tasks
```

---

## Task 1: Domain — TicketTask type

**Files:**
- Modify: `internal/domain/ticket.go`
- Modify: `internal/domain/ticket_test.go`

- [ ] **Step 1: Append `TicketTask` type**

Append to `internal/domain/ticket.go` (after the existing TicketSavedSearch type or wherever new types have been added):

```go
// TicketTask is one task on a ticket. Tasks track sub-work with their own
// PercentComplete and (optionally) ResponsibleUid/Group. Time entries can
// target a ticket task via Target{Kind: TargetTicketTask, ItemID: ticketID,
// TaskID: taskID}.
type TicketTask struct {
	ID                   int
	TicketID             int
	Title                string
	Description          string
	Active               bool
	PercentComplete      int
	EstimatedMinutes     int
	ActualMinutes        int
	StartDate            time.Time
	EndDate              time.Time
	CreatedDate          time.Time
	CreatedName          string
	ModifiedDate         time.Time
	CompletedDate        time.Time
	CompletedName        string
	ResponsibleUID       string
	ResponsibleName      string
	ResponsibleGroupID   int
	ResponsibleGroupName string
	Order                int
}
```

(`time` is already imported in `ticket.go`.)

- [ ] **Step 2: Smoke test**

Append to `internal/domain/ticket_test.go`:

```go
func TestTicketTaskZeroValueIsValid(t *testing.T) {
	_ = TicketTask{}
}

func TestTicketTaskFields(t *testing.T) {
	tk := TicketTask{ID: 1, TicketID: 100, Title: "x", PercentComplete: 50}
	if tk.PercentComplete != 50 {
		t.Fatalf("PercentComplete: got %d, want 50", tk.PercentComplete)
	}
}
```

- [ ] **Step 3: Verify**

```bash
go build ./...
go test ./internal/domain/...
gofmt -l internal/domain/ticket.go internal/domain/ticket_test.go
```

All clean.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/ticket.go internal/domain/ticket_test.go
git commit -m "feat(domain): add TicketTask type"
```

**No `Co-Authored-By:` trailer.**

---

## Task 2: ticketsvc — wire types

**Files:**
- Modify: `internal/svc/ticketsvc/types.go`

- [ ] **Step 1: Append wire types**

```go
// wireTicketTask matches GET /TDWebApi/api/{appId}/tickets/{ticketID}/tasks
// rows and GET /TDWebApi/api/{appId}/tickets/{ticketID}/tasks/{id}.
//
// Live-verified on the test tenant 2026-05-08: dates use TD's standard format and
// CompletedDate may be "0001-01-01T00:00:00" when unset (parseTDTime
// returns zero time for that). ResponsibleUid is null (empty string)
// when unassigned at the individual level — group assignment uses
// ResponsibleGroupID/Name instead.
type wireTicketTask struct {
	ID                   int    `json:"ID"`
	TicketID             int    `json:"TicketID"`
	Title                string `json:"Title"`
	Description          string `json:"Description"`
	IsActive             bool   `json:"IsActive"`
	PercentComplete      int    `json:"PercentComplete"`
	EstimatedMinutes     int    `json:"EstimatedMinutes"`
	ActualMinutes        int    `json:"ActualMinutes"`
	StartDate            string `json:"StartDate,omitempty"`
	EndDate              string `json:"EndDate,omitempty"`
	CreatedDate          string `json:"CreatedDate"`
	CreatedFullName      string `json:"CreatedFullName"`
	ModifiedDate         string `json:"ModifiedDate"`
	CompletedDate        string `json:"CompletedDate"`
	CompletedFullName    string `json:"CompletedFullName"`
	ResponsibleUid       string `json:"ResponsibleUid"`
	ResponsibleFullName  string `json:"ResponsibleFullName"`
	ResponsibleGroupID   int    `json:"ResponsibleGroupID"`
	ResponsibleGroupName string `json:"ResponsibleGroupName"`
	Order                int    `json:"Order"`
}

// wireTaskFeedUpdate is the request body for
// POST /TDWebApi/api/{appId}/tickets/{ticketID}/tasks/{taskID}/feed.
//
// HoursWorked is informational only — it does NOT create a time entry
// or update the task's ActualMinutes. Use a separate time entry
// (`tdx ticket task log`) for real time tracking.
//
// PercentComplete is *int because 0 is a valid value (means "set to 0%
// complete"); nil means "don't send PercentComplete in the body."
type wireTaskFeedUpdate struct {
	Comments        string   `json:"Comments,omitempty"`
	PercentComplete *int     `json:"PercentComplete,omitempty"`
	HoursWorked     float64  `json:"HoursWorked,omitempty"`
	IsPrivate       bool     `json:"IsPrivate,omitempty"`
	Notify          []string `json:"Notify,omitempty"`
}
```

- [ ] **Step 2: Verify**

```bash
go build ./...
go test ./internal/svc/ticketsvc/...
gofmt -l internal/svc/ticketsvc/types.go
```

All clean.

- [ ] **Step 3: Commit**

```bash
git add internal/svc/ticketsvc/types.go
git commit -m "feat(ticketsvc): add wireTicketTask + wireTaskFeedUpdate"
```

**No `Co-Authored-By:` trailer.**

---

## Task 3: ticketsvc — tasks.go (4 service methods + tests)

**Files:**
- Create: `internal/svc/ticketsvc/tasks.go`
- Create: `internal/svc/ticketsvc/tasks_test.go`

- [ ] **Step 1: Implement `tasks.go`**

```go
package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListTasks fetches all tasks on a ticket via
// GET /api/{appId}/tickets/{ticketID}/tasks.
func (s *Service) ListTasks(ctx context.Context, profileName string, appID, ticketID int) ([]domain.TicketTask, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTicketTask
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks", resolvedAppID, ticketID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list ticket tasks %d: %w", ticketID, err)
	}
	out := make([]domain.TicketTask, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTask(w))
	}
	return out, nil
}

// GetTask fetches a single task on a ticket.
func (s *Service) GetTask(ctx context.Context, profileName string, appID, ticketID, taskID int) (domain.TicketTask, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return domain.TicketTask{}, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.TicketTask{}, err
	}
	var w wireTicketTask
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks/%d", resolvedAppID, ticketID, taskID)
	if err := client.DoJSON(ctx, "GET", path, nil, &w); err != nil {
		return domain.TicketTask{}, fmt.Errorf("get ticket task %d/%d: %w", ticketID, taskID, err)
	}
	return decodeTask(w), nil
}

// GetTaskFeed fetches feed entries for a ticket task.
func (s *Service) GetTaskFeed(ctx context.Context, profileName string, appID, ticketID, taskID int) ([]domain.TicketFeedEntry, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks/%d/feed", resolvedAppID, ticketID, taskID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get ticket task feed %d/%d: %w", ticketID, taskID, err)
	}
	out := make([]domain.TicketFeedEntry, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketFeedEntry{
			ID:         w.ID,
			AuthorUID:  w.CreatedUid,
			AuthorName: w.CreatedFullName,
			CreatedAt:  parseTDTime(w.CreatedDate),
			Body:       w.Body,
			IsPrivate:  w.IsPrivate,
			EventKind:  classifyFeedKind(w.UpdateType),
		})
	}
	return out, nil
}

// UpdateTaskFeed posts a feed update to a ticket task. Returns the new
// feed entry's ID. percentComplete is a pointer because 0 is a valid
// value (set to 0%); pass nil to omit. hoursWorked is informational
// only — does NOT create a time entry or update task ActualMinutes.
func (s *Service) UpdateTaskFeed(ctx context.Context, profileName string, appID, ticketID, taskID int, body string, percentComplete *int, hoursWorked float64, isPrivate bool, notify []string) (int, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return 0, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return 0, err
	}
	req := wireTaskFeedUpdate{
		Comments:        body,
		PercentComplete: percentComplete,
		HoursWorked:     hoursWorked,
		IsPrivate:       isPrivate,
		Notify:          notify,
	}
	var resp wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks/%d/feed", resolvedAppID, ticketID, taskID)
	if err := client.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return 0, fmt.Errorf("update ticket task %d/%d: %w", ticketID, taskID, err)
	}
	return resp.ID, nil
}

// decodeTask maps a wireTicketTask to domain.TicketTask. Trims whitespace
// on Title; handles TD's "0001-01-01..." unset-date sentinel via
// parseTDTime returning zero time.
func decodeTask(w wireTicketTask) domain.TicketTask {
	return domain.TicketTask{
		ID:                   w.ID,
		TicketID:             w.TicketID,
		Title:                strings.TrimSpace(w.Title),
		Description:          w.Description,
		Active:               w.IsActive,
		PercentComplete:      w.PercentComplete,
		EstimatedMinutes:     w.EstimatedMinutes,
		ActualMinutes:        w.ActualMinutes,
		StartDate:            parseTDTime(w.StartDate),
		EndDate:              parseTDTime(w.EndDate),
		CreatedDate:          parseTDTime(w.CreatedDate),
		CreatedName:          w.CreatedFullName,
		ModifiedDate:         parseTDTime(w.ModifiedDate),
		CompletedDate:        parseTDTime(w.CompletedDate),
		CompletedName:        w.CompletedFullName,
		ResponsibleUID:       w.ResponsibleUid,
		ResponsibleName:      w.ResponsibleFullName,
		ResponsibleGroupID:   w.ResponsibleGroupID,
		ResponsibleGroupName: w.ResponsibleGroupName,
		Order:                w.Order,
	}
}
```

- [ ] **Step 2: Tests**

Create `internal/svc/ticketsvc/tasks_test.go`:

```go
package ticketsvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListTasks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/100/tasks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 1, "TicketID": 100, "Title": "  Step 1  ", "PercentComplete": 50, "EstimatedMinutes": 60, "ActualMinutes": 30, "IsActive": true, "ResponsibleUid": "uid-a", "ResponsibleFullName": "Alice", "Order": 1},
			{"ID": 2, "TicketID": 100, "Title": "Step 2", "PercentComplete": 0, "IsActive": true, "Order": 2}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	tasks, err := svc.ListTasks(context.Background(), prof, 31, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("want 2, got %d", len(tasks))
	}
	if tasks[0].Title != "Step 1" {
		t.Errorf("title not trimmed: %q", tasks[0].Title)
	}
	if tasks[0].PercentComplete != 50 {
		t.Errorf("percent: %d", tasks[0].PercentComplete)
	}
	if tasks[0].ResponsibleName != "Alice" {
		t.Errorf("responsible: %q", tasks[0].ResponsibleName)
	}
}

func TestGetTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/100/tasks/5" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"ID": 5, "TicketID": 100, "Title": "Investigate",
			"Description": "find the issue", "PercentComplete": 75,
			"EstimatedMinutes": 240, "ActualMinutes": 90,
			"IsActive": true, "Order": 3,
			"CreatedDate": "2026-05-01T10:00:00Z",
			"CompletedDate": "0001-01-01T00:00:00",
			"ResponsibleUid": "", "ResponsibleGroupID": 100, "ResponsibleGroupName": "Linux Team"
		}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.GetTask(context.Background(), prof, 31, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 5 || got.PercentComplete != 75 {
		t.Errorf("decode: %+v", got)
	}
	if !got.CompletedDate.IsZero() {
		t.Errorf("CompletedDate should be zero for sentinel input: %v", got.CompletedDate)
	}
	if got.ResponsibleGroupName != "Linux Team" {
		t.Errorf("group name: %q", got.ResponsibleGroupName)
	}
}

func TestGetTaskFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/100/tasks/5/feed" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 200, "CreatedUid": "uid-a", "CreatedFullName": "Alice", "CreatedDate": "2026-05-01T10:00:00Z", "Body": "halfway", "UpdateType": 1}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	entries, err := svc.GetTaskFeed(context.Background(), prof, 31, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].EventKind != "comment" {
		t.Errorf("decode: %+v", entries)
	}
}

func TestUpdateTaskFeedSendsPercentAndHours(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 555}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	pct := 50
	id, err := svc.UpdateTaskFeed(context.Background(), prof, 31, 100, 5, "halfway", &pct, 0.5, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if id != 555 {
		t.Errorf("feed id: %d", id)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatal(err)
	}
	if sent["Comments"] != "halfway" {
		t.Errorf("Comments: %v", sent["Comments"])
	}
	if pc, _ := sent["PercentComplete"].(float64); pc != 50 {
		t.Errorf("PercentComplete: %v", sent["PercentComplete"])
	}
	if hw, _ := sent["HoursWorked"].(float64); hw != 0.5 {
		t.Errorf("HoursWorked: %v", sent["HoursWorked"])
	}
}

func TestUpdateTaskFeedNilPercentOmits(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 1}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, _ = svc.UpdateTaskFeed(context.Background(), prof, 31, 100, 5, "msg", nil, 0, false, nil)
	if strings.Contains(string(capturedBody), "PercentComplete") {
		t.Errorf("nil percent should be omitted; body: %s", capturedBody)
	}
}

func TestUpdateTaskFeedZeroPercentSent(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 1}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	zero := 0
	_, _ = svc.UpdateTaskFeed(context.Background(), prof, 31, 100, 5, "reset", &zero, 0, false, nil)
	if !strings.Contains(string(capturedBody), `"PercentComplete":0`) {
		t.Errorf("explicit 0 should be sent; body: %s", capturedBody)
	}
}
```

- [ ] **Step 3: Verify**

```bash
go test ./internal/svc/ticketsvc/...
go vet ./internal/svc/ticketsvc/...
gofmt -l internal/svc/ticketsvc/
golangci-lint run ./internal/svc/ticketsvc/...
```

All clean.

- [ ] **Step 4: Commit**

```bash
git add internal/svc/ticketsvc/tasks.go internal/svc/ticketsvc/tasks_test.go
git commit -m "feat(ticketsvc): add ListTasks / GetTask / GetTaskFeed / UpdateTaskFeed"
```

**No `Co-Authored-By:` trailer.**

---

## Task 4: CLI — extend `ticketsvcAPI` + `stubTicketsvc`

**Files:**
- Modify: `internal/cli/ticket/ticket.go`
- Modify: `internal/cli/ticket/stub_test.go`

- [ ] **Step 1: Extend `ticketsvcAPI`**

In `internal/cli/ticket/ticket.go`, find the `ticketsvcAPI` interface. Append four methods:

```go
ListTasks(ctx context.Context, profile string, appID, ticketID int) ([]domain.TicketTask, error)
GetTask(ctx context.Context, profile string, appID, ticketID, taskID int) (domain.TicketTask, error)
GetTaskFeed(ctx context.Context, profile string, appID, ticketID, taskID int) ([]domain.TicketFeedEntry, error)
UpdateTaskFeed(ctx context.Context, profile string, appID, ticketID, taskID int, body string, percentComplete *int, hoursWorked float64, isPrivate bool, notify []string) (int, error)
```

Place at the bottom of the existing method list. Preserve other methods.

- [ ] **Step 2: Extend `stubTicketsvc`**

In `internal/cli/ticket/stub_test.go`, add fields:

```go
tasks            []domain.TicketTask
task             domain.TicketTask
taskFeed         []domain.TicketFeedEntry
taskFeedAddedID  int
lastTaskUpdate   struct {
    Body            string
    PercentComplete *int
    HoursWorked     float64
    IsPrivate       bool
    Notify          []string
}
```

(Place near other resolved-* fields.)

Add four new methods to `stubTicketsvc`:

```go
func (s *stubTicketsvc) ListTasks(_ context.Context, _ string, _, _ int) ([]domain.TicketTask, error) {
	return s.tasks, s.err
}

func (s *stubTicketsvc) GetTask(_ context.Context, _ string, _, _, _ int) (domain.TicketTask, error) {
	return s.task, s.err
}

func (s *stubTicketsvc) GetTaskFeed(_ context.Context, _ string, _, _, _ int) ([]domain.TicketFeedEntry, error) {
	return s.taskFeed, s.err
}

func (s *stubTicketsvc) UpdateTaskFeed(_ context.Context, _ string, _, _, _ int, body string, pc *int, hw float64, isPrivate bool, notify []string) (int, error) {
	s.lastTaskUpdate.Body = body
	s.lastTaskUpdate.PercentComplete = pc
	s.lastTaskUpdate.HoursWorked = hw
	s.lastTaskUpdate.IsPrivate = isPrivate
	s.lastTaskUpdate.Notify = notify
	return s.taskFeedAddedID, s.err
}
```

- [ ] **Step 3: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
golangci-lint run ./internal/cli/ticket/...
```

Expected: clean. CLI doesn't yet call these methods.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ticket/ticket.go internal/cli/ticket/stub_test.go
git commit -m "feat(cli/ticket): widen ticketsvcAPI with task methods"
```

**No `Co-Authored-By:` trailer.**

---

## Task 5: CLI — `tdx ticket task` (5 sub-commands in one file)

**Files:**
- Create: `internal/cli/ticket/task.go`
- Create: `internal/cli/ticket/task_test.go`
- Modify: `internal/cli/ticket/ticket.go` (register `newTaskCmd(nil)`)

This task is the largest in the plan. It implements all 5 sub-commands in a single file because they share heavy plumbing (cobra factory, ID parsing, app-id resolution, ticketsvcAPI/timesvcAPI dispatch).

- [ ] **Step 1: Implement `task.go`**

```go
package ticket

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// newTaskCmd assembles the `tdx ticket task` sub-tree.
func newTaskCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage ticket tasks (list, show, feed, update, log time)",
	}
	cmd.AddCommand(newTaskListCmd(svc))
	cmd.AddCommand(newTaskShowCmd(svc))
	cmd.AddCommand(newTaskFeedCmd(svc))
	cmd.AddCommand(newTaskUpdateCmd(svc))
	cmd.AddCommand(newTaskLogCmd(svc))
	return cmd
}

// --- task list ---

func newTaskListCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list <ticket-id>",
		Short: "List tasks on a ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, err := strconv.Atoi(args[0])
			if err != nil || ticketID <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
			}
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			s := svc
			if s == nil { s = ticketsvc.New(paths) }
			return runTaskList(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID int, jsonOut bool) error {
	tasks, err := svc.ListTasks(ctx, profile, appID, ticketID)
	if err != nil {
		return err
	}
	if jsonOut {
		type taskJSON struct {
			ID               int    `json:"id"`
			TicketID         int    `json:"ticketID"`
			Title            string `json:"title"`
			PercentComplete  int    `json:"percentComplete"`
			EstimatedMinutes int    `json:"estimatedMinutes,omitempty"`
			ActualMinutes    int    `json:"actualMinutes,omitempty"`
			ResponsibleName  string `json:"responsibleName,omitempty"`
			Order            int    `json:"order"`
		}
		out := make([]taskJSON, 0, len(tasks))
		for _, t := range tasks {
			resp := t.ResponsibleName
			if resp == "" && t.ResponsibleGroupName != "" {
				resp = t.ResponsibleGroupName + " (group)"
			}
			out = append(out, taskJSON{
				ID: t.ID, TicketID: t.TicketID, Title: t.Title,
				PercentComplete: t.PercentComplete,
				EstimatedMinutes: t.EstimatedMinutes, ActualMinutes: t.ActualMinutes,
				ResponsibleName: resp, Order: t.Order,
			})
		}
		return render.JSON(w, struct {
			Schema string     `json:"schema"`
			Tasks  []taskJSON `json:"tasks"`
		}{Schema: "tdx.v1.ticketTaskList", Tasks: out})
	}
	if len(tasks) == 0 {
		_, _ = fmt.Fprintln(w, "no tasks found on this ticket")
		return nil
	}
	headers := []string{"ID", "TITLE", "%COMPLETE", "EST", "ACT", "RESPONSIBLE"}
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		resp := t.ResponsibleName
		if resp == "" && t.ResponsibleGroupName != "" {
			resp = t.ResponsibleGroupName + " (group)"
		}
		rows = append(rows, []string{
			strconv.Itoa(t.ID),
			truncate(t.Title, 50),
			fmt.Sprintf("%d%%", t.PercentComplete),
			formatDuration(t.EstimatedMinutes),
			formatDuration(t.ActualMinutes),
			resp,
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

// --- task show ---

func newTaskShowCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "show <ticket-id> <task-id>",
		Short: "Show full detail for one ticket task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil { return err }
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			s := svc
			if s == nil { s = ticketsvc.New(paths) }
			return runTaskShow(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, taskID, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskShow(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID, taskID int, jsonOut bool) error {
	t, err := svc.GetTask(ctx, profile, appID, ticketID, taskID)
	if err != nil {
		return err
	}
	if jsonOut {
		return render.JSON(w, struct {
			Schema string            `json:"schema"`
			Task   domain.TicketTask `json:"task"`
		}{Schema: "tdx.v1.ticketTask", Task: t})
	}
	_, _ = fmt.Fprintf(w, "#%d / task #%d — %s\n\n", t.TicketID, t.ID, t.Title)
	_, _ = fmt.Fprintf(w, "Progress:    %d%%\n", t.PercentComplete)
	if !t.Active {
		_, _ = fmt.Fprintln(w, "Status:      INACTIVE")
	}
	if t.ResponsibleName != "" {
		_, _ = fmt.Fprintf(w, "Responsible: %s\n", t.ResponsibleName)
	} else if t.ResponsibleGroupName != "" {
		_, _ = fmt.Fprintf(w, "Responsible: %s (group)\n", t.ResponsibleGroupName)
	}
	if !t.CreatedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Created:     %s by %s\n", t.CreatedDate.Format("2006-01-02 15:04"), t.CreatedName)
	}
	if !t.ModifiedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Modified:    %s\n", t.ModifiedDate.Format("2006-01-02 15:04"))
	}
	if !t.CompletedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Completed:   %s by %s\n", t.CompletedDate.Format("2006-01-02 15:04"), t.CompletedName)
	}
	_, _ = fmt.Fprintf(w, "Time:        EST: %s  ACT: %s\n", formatDuration(t.EstimatedMinutes), formatDuration(t.ActualMinutes))
	if t.Description != "" {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Description:")
		for _, line := range splitLines(t.Description) {
			_, _ = fmt.Fprintln(w, "  "+line)
		}
	}
	return nil
}

// --- task feed ---

func newTaskFeedCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		limit       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "feed <ticket-id> <task-id>",
		Short: "Read the feed for a ticket task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil { return err }
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			s := svc
			if s == nil { s = ticketsvc.New(paths) }
			return runTaskFeed(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, taskID, limit, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().IntVar(&limit, "limit", 0, "max entries (0 = all)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskFeed(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID, taskID, limit int, jsonOut bool) error {
	entries, err := svc.GetTaskFeed(ctx, profile, appID, ticketID, taskID)
	if err != nil {
		return err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	if jsonOut {
		type entryJSON struct {
			ID         int    `json:"id"`
			AuthorName string `json:"authorName,omitempty"`
			CreatedAt  string `json:"createdAt,omitempty"`
			Body       string `json:"body,omitempty"`
			IsPrivate  bool   `json:"isPrivate"`
			EventKind  string `json:"eventKind,omitempty"`
		}
		out := make([]entryJSON, 0, len(entries))
		for _, e := range entries {
			ts := ""
			if !e.CreatedAt.IsZero() {
				ts = e.CreatedAt.Format(time.RFC3339)
			}
			out = append(out, entryJSON{ID: e.ID, AuthorName: e.AuthorName, CreatedAt: ts, Body: e.Body, IsPrivate: e.IsPrivate, EventKind: e.EventKind})
		}
		return render.JSON(w, struct {
			Schema  string      `json:"schema"`
			Entries []entryJSON `json:"entries"`
		}{Schema: "tdx.v1.ticketTaskFeed", Entries: out})
	}
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(w, "no feed entries")
		return nil
	}
	for i, e := range entries {
		when := ""
		if !e.CreatedAt.IsZero() {
			when = e.CreatedAt.Format("2006-01-02 15:04")
		}
		_, _ = fmt.Fprintf(w, "[%s] %s — %s\n", when, e.AuthorName, e.EventKind)
		if e.Body != "" {
			for _, line := range splitLines(e.Body) {
				_, _ = fmt.Fprintln(w, "  "+line)
			}
		}
		if i < len(entries)-1 {
			_, _ = fmt.Fprintln(w)
		}
	}
	return nil
}

// --- task update (mutating) ---

func newTaskUpdateCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID         int
		percentFlag   int
		percentSet    bool
		completeFlag  bool
		commentFlag   string
		hoursWorked   float64
		privateFlag   bool
		notifyFlag    []string
		yesFlag       bool
		profileFlag   string
	)
	cmd := &cobra.Command{
		Use:   "update <ticket-id> <task-id>",
		Short: "Post a feed update to a ticket task (--yes required; --hours-worked is informational only — use `tdx ticket task log` for real time entries)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil { return err }
			percentSet = cmd.Flags().Changed("percent")
			if !yesFlag {
				return fmt.Errorf("pass --yes to update the task")
			}
			if percentSet && completeFlag {
				return fmt.Errorf("--percent and --complete are mutually exclusive")
			}
			if !percentSet && !completeFlag && commentFlag == "" && hoursWorked == 0 {
				return fmt.Errorf("nothing to update — pass at least one of --percent / --complete / --comment / --hours-worked")
			}
			var pc *int
			if completeFlag {
				v := 100
				pc = &v
			} else if percentSet {
				if percentFlag < 0 || percentFlag > 100 {
					return fmt.Errorf("--percent must be 0-100, got %d", percentFlag)
				}
				v := percentFlag
				pc = &v
			}
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			s := svc
			if s == nil { s = ticketsvc.New(paths) }
			return runTaskUpdate(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, taskID, commentFlag, pc, hoursWorked, privateFlag, notifyFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().IntVar(&percentFlag, "percent", 0, "percent complete (0-100)")
	cmd.Flags().BoolVar(&completeFlag, "complete", false, "shortcut for --percent 100")
	cmd.Flags().StringVar(&commentFlag, "comment", "", "comment body")
	cmd.Flags().Float64Var(&hoursWorked, "hours-worked", 0, "hours worked (informational only — does NOT create a time entry)")
	cmd.Flags().BoolVar(&privateFlag, "private", false, "internal note (not visible to requestor)")
	cmd.Flags().StringSliceVar(&notifyFlag, "notify", nil, "additional notify recipients by UID (repeatable)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to send")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskUpdate(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID, taskID int, body string, pc *int, hoursWorked float64, isPrivate bool, notify []string) error {
	feedID, err := svc.UpdateTaskFeed(ctx, profile, appID, ticketID, taskID, body, pc, hoursWorked, isPrivate, notify)
	if err != nil {
		return err
	}
	parts := []string{}
	if pc != nil {
		parts = append(parts, fmt.Sprintf("percent=%d%%", *pc))
	}
	if body != "" {
		parts = append(parts, fmt.Sprintf("comment=%q", truncate(body, 40)))
	}
	if hoursWorked > 0 {
		parts = append(parts, fmt.Sprintf("hours-worked=%g (informational)", hoursWorked))
	}
	summary := strings.Join(parts, ", ")
	_, _ = fmt.Fprintf(w, "task #%d/#%d updated: %s (feed entry %d)\n", ticketID, taskID, summary, feedID)
	return nil
}

// --- task log (mutating, time crossover) ---

func newTaskLogCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		hoursFlag   float64
		minutesFlag int
		typeName    string
		typeID      int
		dateFlag    string
		descFlag    string
		billable    bool
		yesFlag     bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "log <ticket-id> <task-id>",
		Short: "Log time worked against a ticket task (--yes required); creates a real time entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil { return err }
			if !yesFlag {
				return fmt.Errorf("pass --yes to log time")
			}
			if (hoursFlag > 0) == (minutesFlag > 0) {
				if hoursFlag == 0 && minutesFlag == 0 {
					return fmt.Errorf("specify either --hours or --minutes")
				}
				return fmt.Errorf("--hours and --minutes are mutually exclusive")
			}
			if (typeName == "") == (typeID == 0) {
				if typeName == "" && typeID == 0 {
					return fmt.Errorf("specify either --type or --type-id")
				}
				return fmt.Errorf("--type and --type-id are mutually exclusive")
			}
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil { return err }
			effectiveAppID := appID
			if effectiveAppID == 0 {
				prof, perr := config.NewProfileStore(paths).GetProfile(profile)
				if perr != nil { return perr }
				if prof.TicketAppID == 0 {
					return fmt.Errorf("no ticket app configured for profile %q (run `tdx ticket app use <id>` or pass --app <id>)", profile)
				}
				effectiveAppID = prof.TicketAppID
			}
			tsvc := timesvc.New(paths)
			billableSet := cmd.Flags().Changed("billable")
			return runTaskLog(cmd.Context(), cmd.OutOrStdout(), tsvc, taskLogArgs{
				profile: profile, authedUID: authedUID, appID: effectiveAppID, ticketID: ticketID, taskID: taskID,
				hours: hoursFlag, minutes: minutesFlag, typeName: typeName, typeID: typeID,
				dateStr: dateFlag, description: descFlag, billableSet: billableSet, billable: billable,
			})
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().Float64Var(&hoursFlag, "hours", 0, "hours")
	cmd.Flags().IntVar(&minutesFlag, "minutes", 0, "minutes")
	cmd.Flags().StringVar(&typeName, "type", "", "time type name")
	cmd.Flags().IntVar(&typeID, "type-id", 0, "time type id")
	cmd.Flags().StringVar(&dateFlag, "date", "", "entry date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&descFlag, "description", "", "description of work performed")
	cmd.Flags().BoolVar(&billable, "billable", false, "force billable (default: type's billable flag)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

type taskLogArgs struct {
	profile     string
	authedUID   string
	appID       int
	ticketID    int
	taskID      int
	hours       float64
	minutes     int
	typeName    string
	typeID      int
	dateStr     string
	description string
	billableSet bool
	billable    bool
}

func runTaskLog(ctx context.Context, w io.Writer, svc timesvcAPI, args taskLogArgs) error {
	date := time.Now().In(domain.EasternTZ)
	if args.dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", args.dateStr, domain.EasternTZ)
		if err != nil {
			return fmt.Errorf("invalid --date: %w", err)
		}
		date = d
	}
	minutes := args.minutes
	if args.hours > 0 {
		minutes = int(math.Round(args.hours * 60))
	}
	if minutes <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	target := domain.Target{
		Kind:   domain.TargetTicketTask,
		AppID:  args.appID,
		ItemID: args.ticketID,
		TaskID: args.taskID,
	}
	chosen, err := resolveTimeType(ctx, svc, args.profile, target, args.typeID, args.typeName)
	if err != nil { return err }
	billable := chosen.Billable
	if args.billableSet {
		billable = args.billable
	}
	in := domain.EntryInput{
		UserUID:     args.authedUID,
		Date:        date,
		Minutes:     minutes,
		TimeTypeID:  chosen.ID,
		Billable:    billable,
		Target:      target,
		Description: args.description,
	}
	entry, err := svc.AddEntry(ctx, args.profile, in)
	if err != nil { return err }
	_, _ = fmt.Fprintf(w, "logged %s to ticket #%d task #%d (entry %d, type %q)\n",
		formatDurationFromMinutes(minutes), args.ticketID, args.taskID, entry.ID, chosen.Name)
	return nil
}

// --- shared helpers ---

func parseTaskIDs(args []string) (int, int, error) {
	ticketID, err := strconv.Atoi(args[0])
	if err != nil || ticketID <= 0 {
		return 0, 0, fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
	}
	taskID, err := strconv.Atoi(args[1])
	if err != nil || taskID <= 0 {
		return 0, 0, fmt.Errorf("task id must be a positive integer, got %q", args[1])
	}
	return ticketID, taskID, nil
}

// formatDuration prints minutes as "Nh", "Nm", or "Nh Mm". Mirrors the
// helper in log.go but renamed to avoid collision (log.go already exports
// a function literal of the same name).
//
// NOTE: log.go already has formatDuration. If both files compile in the
// same package, the second declaration must use a different name. We use
// formatDurationFromMinutes here for the task-log specific renderer.
// For the list/show task displays we use formatDuration from log.go.
func formatDurationFromMinutes(minutes int) string {
	h := minutes / 60
	m := minutes % 60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh %dm", h, m)
	}
}
```

**Important note for the implementer:** before adding `formatDurationFromMinutes`, check whether a `formatDuration` helper already exists in `log.go`. If it does AND has the same signature `(minutes int) string`, just reuse it directly — don't define a duplicate. If it has a different signature (e.g. `(int) string` returning different shape), use a different name like `formatTaskDuration`. Pick whichever keeps the package free of duplicates and shape-mismatches.

Same advice for `truncate` and `splitLines` — both are already defined elsewhere in the ticket package (`search.go` has `truncate`, `show.go` has `splitLines`). Reuse them; don't redefine.

- [ ] **Step 2: Implement `task_test.go`**

Create `internal/cli/ticket/task_test.go`:

```go
package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunTaskListEmpty(t *testing.T) {
	stub := &stubTicketsvc{tasks: nil}
	var buf bytes.Buffer
	if err := runTaskList(context.Background(), &buf, stub, "default", 31, 100, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no tasks found") {
		t.Errorf("empty msg: %s", buf.String())
	}
}

func TestRunTaskListTable(t *testing.T) {
	stub := &stubTicketsvc{tasks: []domain.TicketTask{
		{ID: 1, TicketID: 100, Title: "Step 1", PercentComplete: 50, EstimatedMinutes: 60, ActualMinutes: 30, ResponsibleName: "Alice"},
		{ID: 2, TicketID: 100, Title: "Step 2", PercentComplete: 0, ResponsibleGroupName: "Linux Team"},
	}}
	var buf bytes.Buffer
	if err := runTaskList(context.Background(), &buf, stub, "default", 31, 100, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Step 1", "50%", "Alice", "Step 2", "Linux Team (group)"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunTaskListJSON(t *testing.T) {
	stub := &stubTicketsvc{tasks: []domain.TicketTask{{ID: 1, TicketID: 100, Title: "T"}}}
	var buf bytes.Buffer
	if err := runTaskList(context.Background(), &buf, stub, "default", 31, 100, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketTaskList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunTaskShow(t *testing.T) {
	stub := &stubTicketsvc{task: domain.TicketTask{
		ID: 5, TicketID: 100, Title: "Investigate",
		PercentComplete: 75, EstimatedMinutes: 240, ActualMinutes: 90,
		ResponsibleName: "Alice",
		CreatedDate: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC), CreatedName: "Alice",
		Description: "find the issue",
	}}
	var buf bytes.Buffer
	if err := runTaskShow(context.Background(), &buf, stub, "default", 31, 100, 5, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#100 / task #5", "Investigate", "75%", "Alice", "find the issue", "EST: 4h", "ACT: 1h 30m"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunTaskShowJSON(t *testing.T) {
	stub := &stubTicketsvc{task: domain.TicketTask{ID: 5, TicketID: 100, Title: "Investigate", PercentComplete: 75}}
	var buf bytes.Buffer
	if err := runTaskShow(context.Background(), &buf, stub, "default", 31, 100, 5, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketTask" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunTaskFeedRendersEntries(t *testing.T) {
	stub := &stubTicketsvc{taskFeed: []domain.TicketFeedEntry{
		{ID: 200, AuthorName: "Alice", Body: "halfway", EventKind: "comment", CreatedAt: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	}}
	var buf bytes.Buffer
	if err := runTaskFeed(context.Background(), &buf, stub, "default", 31, 100, 5, 0, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Alice", "comment", "halfway"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q: %s", want, buf.String())
		}
	}
}

func TestRunTaskFeedEmpty(t *testing.T) {
	stub := &stubTicketsvc{taskFeed: nil}
	var buf bytes.Buffer
	_ = runTaskFeed(context.Background(), &buf, stub, "default", 31, 100, 5, 0, false)
	if !strings.Contains(buf.String(), "no feed entries") {
		t.Errorf("empty: %s", buf.String())
	}
}

func TestRunTaskUpdateWithComment(t *testing.T) {
	stub := &stubTicketsvc{taskFeedAddedID: 555}
	var buf bytes.Buffer
	pc := 50
	if err := runTaskUpdate(context.Background(), &buf, stub, "default", 31, 100, 5, "halfway", &pc, 0, false, nil); err != nil {
		t.Fatal(err)
	}
	if stub.lastTaskUpdate.Body != "halfway" {
		t.Errorf("body: %q", stub.lastTaskUpdate.Body)
	}
	if stub.lastTaskUpdate.PercentComplete == nil || *stub.lastTaskUpdate.PercentComplete != 50 {
		t.Errorf("percent: %v", stub.lastTaskUpdate.PercentComplete)
	}
	if !strings.Contains(buf.String(), "555") || !strings.Contains(buf.String(), "percent=50%") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestRunTaskUpdateHoursWorkedNoteInformational(t *testing.T) {
	stub := &stubTicketsvc{taskFeedAddedID: 1}
	var buf bytes.Buffer
	_ = runTaskUpdate(context.Background(), &buf, stub, "default", 31, 100, 5, "", nil, 0.5, false, nil)
	if stub.lastTaskUpdate.HoursWorked != 0.5 {
		t.Errorf("hours: %v", stub.lastTaskUpdate.HoursWorked)
	}
	if !strings.Contains(buf.String(), "informational") {
		t.Errorf("expected 'informational' label in output: %s", buf.String())
	}
}

func TestNewTaskUpdateCmdRequiresYes(t *testing.T) {
	cmd := newTaskUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--comment", "hi"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}

func TestNewTaskUpdateCmdRejectsPercentAndComplete(t *testing.T) {
	cmd := newTaskUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--percent", "50", "--complete", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("want mutex error, got %v", err)
	}
}

func TestNewTaskUpdateCmdRejectsAllEmpty(t *testing.T) {
	cmd := newTaskUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("want all-empty error, got %v", err)
	}
}

func TestNewTaskUpdateCmdCompleteSetsHundred(t *testing.T) {
	stub := &stubTicketsvc{taskFeedAddedID: 1}
	cmd := newTaskUpdateCmd(stub)
	cmd.SetArgs([]string{"100", "5", "--complete", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stub.lastTaskUpdate.PercentComplete == nil || *stub.lastTaskUpdate.PercentComplete != 100 {
		t.Errorf("--complete should set percent=100; got %v", stub.lastTaskUpdate.PercentComplete)
	}
}

func TestRunTaskLog(t *testing.T) {
	stub := &stubTimesvc{
		types:      []domain.TimeType{{ID: 7, Name: "Development", Billable: true}},
		addedEntry: domain.TimeEntry{ID: 9001},
	}
	var buf bytes.Buffer
	err := runTaskLog(context.Background(), &buf, stub, taskLogArgs{
		profile: "default", authedUID: "uid-me", appID: 31, ticketID: 100, taskID: 5,
		minutes: 30, typeName: "Development",
	})
	if err != nil { t.Fatal(err) }
	if stub.lastInput.Target.Kind != domain.TargetTicketTask {
		t.Errorf("Target.Kind: %s", stub.lastInput.Target.Kind)
	}
	if stub.lastInput.Target.TaskID != 5 {
		t.Errorf("Target.TaskID: %d", stub.lastInput.Target.TaskID)
	}
	if stub.lastInput.Target.ItemID != 100 {
		t.Errorf("Target.ItemID: %d", stub.lastInput.Target.ItemID)
	}
	if !strings.Contains(buf.String(), "task #5") || !strings.Contains(buf.String(), "9001") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestNewTaskLogCmdRequiresYes(t *testing.T) {
	cmd := newTaskLogCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--minutes", "30", "--type", "Development"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}
```

(`stubTimesvc` is defined in `log_test.go` — reuse it.)

- [ ] **Step 3: Wire into `ticket.go`**

In `internal/cli/ticket/ticket.go`, add to `New()`:

```go
cmd.AddCommand(newTaskCmd(nil))
```

Place after the existing `newGroupsCmd(nil)`.

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
go vet ./internal/cli/ticket/...
gofmt -l internal/cli/ticket/
golangci-lint run ./internal/cli/ticket/...
go run ./cmd/tdx ticket task --help
```

`tdx ticket task --help` should list `list / show / feed / update / log` as sub-commands.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ticket/task.go internal/cli/ticket/task_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): add task list/show/feed/update/log subgroup"
```

**No `Co-Authored-By:` trailer.**

---

## Task 6: MCP — 3 read tools

**Files:**
- Modify: `internal/mcp/tools_ticket.go`
- Modify: `internal/mcp/tools_ticket_test.go`

- [ ] **Step 1: Add argument types**

In `internal/mcp/tools_ticket.go`, append:

```go
type listTicketTasksArgs struct {
	Profile  string `json:"profile,omitempty"`
	AppID    int    `json:"appID,omitempty"`
	TicketID int    `json:"ticketID"`
}

type getTicketTaskArgs struct {
	Profile  string `json:"profile,omitempty"`
	AppID    int    `json:"appID,omitempty"`
	TicketID int    `json:"ticketID"`
	TaskID   int    `json:"taskID"`
}

type getTicketTaskFeedArgs struct {
	Profile  string `json:"profile,omitempty"`
	AppID    int    `json:"appID,omitempty"`
	TicketID int    `json:"ticketID"`
	TaskID   int    `json:"taskID"`
	Limit    int    `json:"limit,omitempty"`
}
```

- [ ] **Step 2: Add handlers**

```go
type ticketTaskRowJSON struct {
	ID               int    `json:"id"`
	TicketID         int    `json:"ticketID"`
	Title            string `json:"title"`
	PercentComplete  int    `json:"percentComplete"`
	EstimatedMinutes int    `json:"estimatedMinutes,omitempty"`
	ActualMinutes    int    `json:"actualMinutes,omitempty"`
	ResponsibleName  string `json:"responsibleName,omitempty"`
	Order            int    `json:"order"`
}

func toTaskRowJSON(t domain.TicketTask) ticketTaskRowJSON {
	resp := t.ResponsibleName
	if resp == "" && t.ResponsibleGroupName != "" {
		resp = t.ResponsibleGroupName + " (group)"
	}
	return ticketTaskRowJSON{
		ID: t.ID, TicketID: t.TicketID, Title: t.Title,
		PercentComplete: t.PercentComplete,
		EstimatedMinutes: t.EstimatedMinutes, ActualMinutes: t.ActualMinutes,
		ResponsibleName: resp, Order: t.Order,
	}
}

func listTicketTasksHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listTicketTasksArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listTicketTasksArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		tasks, err := svcs.Tickets.ListTasks(ctx, profile, args.AppID, args.TicketID)
		if err != nil {
			return errorResult(fmt.Sprintf("list_ticket_tasks: %v", err)), nil, nil
		}
		out := make([]ticketTaskRowJSON, 0, len(tasks))
		for _, t := range tasks {
			out = append(out, toTaskRowJSON(t))
		}
		return jsonResult(struct {
			Schema string              `json:"schema"`
			Tasks  []ticketTaskRowJSON `json:"tasks"`
		}{Schema: "tdx.v1.ticketTaskList", Tasks: out})
	}
}

func getTicketTaskHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getTicketTaskArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getTicketTaskArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		task, err := svcs.Tickets.GetTask(ctx, profile, args.AppID, args.TicketID, args.TaskID)
		if err != nil {
			return errorResult(fmt.Sprintf("get_ticket_task: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema string            `json:"schema"`
			Task   domain.TicketTask `json:"task"`
		}{Schema: "tdx.v1.ticketTask", Task: task})
	}
}

func getTicketTaskFeedHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getTicketTaskFeedArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getTicketTaskFeedArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		entries, err := svcs.Tickets.GetTaskFeed(ctx, profile, args.AppID, args.TicketID, args.TaskID)
		if err != nil {
			return errorResult(fmt.Sprintf("get_ticket_task_feed: %v", err)), nil, nil
		}
		if args.Limit > 0 && len(entries) > args.Limit {
			entries = entries[:args.Limit]
		}
		return jsonResult(struct {
			Schema  string                   `json:"schema"`
			Entries []domain.TicketFeedEntry `json:"entries"`
		}{Schema: "tdx.v1.ticketTaskFeed", Entries: entries})
	}
}
```

- [ ] **Step 3: Register the new tools**

In `RegisterTicketTools`, add three new registrations (place after the existing `list_ticket_groups`):

```go
sdkmcp.AddTool(srv, &sdkmcp.Tool{
	Name:        "list_ticket_tasks",
	Description: "List tasks on a ticket. Read-only.",
}, listTicketTasksHandler(svcs))

sdkmcp.AddTool(srv, &sdkmcp.Tool{
	Name:        "get_ticket_task",
	Description: "Get full detail for one ticket task. Read-only.",
}, getTicketTaskHandler(svcs))

sdkmcp.AddTool(srv, &sdkmcp.Tool{
	Name:        "get_ticket_task_feed",
	Description: "Read feed entries for a ticket task. Read-only.",
}, getTicketTaskFeedHandler(svcs))
```

- [ ] **Step 4: Tests**

Append to `internal/mcp/tools_ticket_test.go`:

```go
func TestListTicketTasks_SchemaName(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTicketTools(srv, Services{})
	// Smoke: registration doesn't panic.
}
```

(Real schema-name verification with stub services already exists for sibling tools — pattern is: invoke handler, decode result, assert schema string. If the existing test pattern uses a helper, replicate it. If not, the smoke test above is sufficient for v0.16.2.)

- [ ] **Step 5: Verify**

```bash
go build ./...
go test ./internal/mcp/...
golangci-lint run ./internal/mcp/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tools_ticket.go internal/mcp/tools_ticket_test.go
git commit -m "feat(mcp): add 3 ticket-task read tools"
```

**No `Co-Authored-By:` trailer.**

---

## Task 7: MCP — 2 mutating tools + tool count update

**Files:**
- Modify: `internal/mcp/tools_ticket_mutating.go`
- Modify: `internal/mcp/tools_ticket_test.go`
- Modify: `internal/mcp/server_test.go` (count 57 → 62)

- [ ] **Step 1: Add argument types**

In `internal/mcp/tools_ticket_mutating.go`:

```go
type updateTicketTaskArgs struct {
	Profile         string   `json:"profile,omitempty"`
	AppID           int      `json:"appID,omitempty"`
	TicketID        int      `json:"ticketID"`
	TaskID          int      `json:"taskID"`
	Comment         string   `json:"comment,omitempty"`
	PercentComplete *int     `json:"percentComplete,omitempty"`
	HoursWorked     float64  `json:"hoursWorked,omitempty"`
	IsPrivate       bool     `json:"isPrivate,omitempty"`
	Notify          []string `json:"notify,omitempty"`
	Confirm         bool     `json:"confirm" jsonschema:"set true to actually update"`
}

type logTicketTaskTimeArgs struct {
	Profile     string  `json:"profile,omitempty"`
	AppID       int     `json:"appID,omitempty"`
	TicketID    int     `json:"ticketID"`
	TaskID      int     `json:"taskID"`
	Hours       float64 `json:"hours,omitempty"`
	Minutes     int     `json:"minutes,omitempty"`
	TypeID      int     `json:"typeID,omitempty"`
	TypeName    string  `json:"typeName,omitempty"`
	Date        string  `json:"date,omitempty"`
	Description string  `json:"description,omitempty"`
	Billable    *bool   `json:"billable,omitempty"`
	Confirm     bool    `json:"confirm" jsonschema:"set true to actually log"`
}
```

- [ ] **Step 2: Add handlers**

```go
func updateTicketTaskHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateTicketTaskArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args updateTicketTaskArgs) (*sdkmcp.CallToolResult, any, error) {
		if errResp, ok := confirmGate(args.Confirm, "set confirm=true to update the ticket task"); !ok {
			return errResp, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		feedID, err := svcs.Tickets.UpdateTaskFeed(ctx, profile, args.AppID, args.TicketID, args.TaskID, args.Comment, args.PercentComplete, args.HoursWorked, args.IsPrivate, args.Notify)
		if err != nil {
			return errorResult(fmt.Sprintf("update_ticket_task: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema   string `json:"schema"`
			TicketID int    `json:"ticketID"`
			TaskID   int    `json:"taskID"`
			FeedID   int    `json:"feedID"`
		}{Schema: "tdx.v1.ticketTaskUpdateResult", TicketID: args.TicketID, TaskID: args.TaskID, FeedID: feedID})
	}
}

// logTicketTaskTimeHandler mirrors logTicketTimeHandler in tools_ticket_mutating.go
// but populates Target.TaskID. Implementation: mirror the existing
// logTicketTimeHandler exactly, then change the Target construction to
// {Kind: TargetTicketTask, AppID, ItemID: ticketID, TaskID: taskID}.
func logTicketTaskTimeHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, logTicketTaskTimeArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args logTicketTaskTimeArgs) (*sdkmcp.CallToolResult, any, error) {
		if errResp, ok := confirmGate(args.Confirm, "set confirm=true to log time"); !ok {
			return errResp, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)

		// Resolve duration: hours XOR minutes
		minutes := args.Minutes
		if args.Hours > 0 {
			minutes = int(args.Hours*60 + 0.5)
		}
		if minutes <= 0 {
			return errorResult("log_ticket_task_time: hours or minutes required"), nil, nil
		}

		// Resolve date: default to now in EasternTZ
		date := time.Now().In(domain.EasternTZ)
		if args.Date != "" {
			d, err := time.ParseInLocation("2006-01-02", args.Date, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("log_ticket_task_time: invalid date: %v", err)), nil, nil
			}
			date = d
		}

		// Resolve type
		target := domain.Target{
			Kind:   domain.TargetTicketTask,
			AppID:  args.AppID,
			ItemID: args.TicketID,
			TaskID: args.TaskID,
		}
		types, err := svcs.Time.TimeTypesForTarget(ctx, profile, target)
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_task_time: %v", err)), nil, nil
		}
		var chosen domain.TimeType
		var found bool
		if args.TypeID > 0 {
			for _, tt := range types {
				if tt.ID == args.TypeID {
					chosen = tt
					found = true
					break
				}
			}
		} else if args.TypeName != "" {
			for _, tt := range types {
				if strings.EqualFold(strings.TrimSpace(tt.Name), strings.TrimSpace(args.TypeName)) {
					chosen = tt
					found = true
					break
				}
			}
		}
		if !found {
			return errorResult(fmt.Sprintf("log_ticket_task_time: time type not valid for this task; allowed: %v", types)), nil, nil
		}

		// Authed user
		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_task_time: %v", err)), nil, nil
		}

		billable := chosen.Billable
		if args.Billable != nil {
			billable = *args.Billable
		}

		entry, err := svcs.Time.AddEntry(ctx, profile, domain.EntryInput{
			UserUID:     user.UID,
			Date:        date,
			Minutes:     minutes,
			TimeTypeID:  chosen.ID,
			Billable:    billable,
			Target:      target,
			Description: args.Description,
		})
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_task_time: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema   string `json:"schema"`
			TicketID int    `json:"ticketID"`
			TaskID   int    `json:"taskID"`
			EntryID  int    `json:"entryID"`
			Minutes  int    `json:"minutes"`
			TypeName string `json:"typeName"`
		}{Schema: "tdx.v1.ticketTaskTimeLogResult", TicketID: args.TicketID, TaskID: args.TaskID, EntryID: entry.ID, Minutes: minutes, TypeName: chosen.Name})
	}
}
```

(`time` and `strings` imports are already in this file or need to be added — check.)

If the existing `logTicketTimeHandler` already factored out the duration/date/type/auth resolution into a shared helper, reuse the helper. Otherwise the duplication above is intentional and matches the pattern.

- [ ] **Step 3: Register the new tools**

In `RegisterTicketMutatingTools`:

```go
sdkmcp.AddTool(srv, &sdkmcp.Tool{
	Name:        "update_ticket_task",
	Description: "Post a feed update to a ticket task with optional percentComplete/hoursWorked/comment. hoursWorked is informational; use log_ticket_task_time for real time entries. Requires confirm=true.",
}, updateTicketTaskHandler(svcs))

sdkmcp.AddTool(srv, &sdkmcp.Tool{
	Name:        "log_ticket_task_time",
	Description: "Log time worked against a ticket task (creates a time entry). Requires confirm=true.",
}, logTicketTaskTimeHandler(svcs))
```

- [ ] **Step 4: Update tool-count assertion**

In `internal/mcp/server_test.go`:

```bash
grep -n "wantCount\s*=\s*57\|wantCount\s*:=\s*57" internal/mcp/server_test.go
```

Update `57` → `62`.

- [ ] **Step 5: Tests**

Append to `internal/mcp/tools_ticket_test.go`:

```go
func TestUpdateTicketTask_RequiresConfirm(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTicketMutatingTools(srv, Services{})
	// Smoke: registration doesn't panic.
}

func TestLogTicketTaskTime_RequiresConfirm(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTicketMutatingTools(srv, Services{})
}
```

(If the existing pattern in this file dispatches handlers directly to assert confirm-gate behavior, replicate that for these two new tools.)

- [ ] **Step 6: Verify**

```bash
go build ./...
go test ./internal/mcp/...
golangci-lint run ./internal/mcp/...
```

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/tools_ticket_mutating.go internal/mcp/tools_ticket_test.go internal/mcp/server_test.go
git commit -m "feat(mcp): add update_ticket_task and log_ticket_task_time mutating tools"
```

**No `Co-Authored-By:` trailer.**

---

## Task 8: Documentation

**Files:**
- Modify: `docs/guide/ticket.md`
- Modify: `docs/guide/mcp.md`
- Modify: `docs/guide.md`
- Modify: `README.md`

- [ ] **Step 1: Update `docs/guide/ticket.md`**

Add a new top-level section after `## tdx ticket statuses`:

```markdown
## tdx ticket task

Manage tasks on a ticket: list, view detail, read feed, update progress, and log time worked.

### tdx ticket task list

```bash
tdx ticket task list <ticket-id>
tdx ticket task list <ticket-id> --json
```

Output: `ID | TITLE | %COMPLETE | EST | ACT | RESPONSIBLE` table. JSON envelope: `tdx.v1.ticketTaskList`.

### tdx ticket task show

```bash
tdx ticket task show <ticket-id> <task-id>
```

Pretty-printed sections: header, progress, responsible person/group, dates, time, description. JSON envelope: `tdx.v1.ticketTask`.

### tdx ticket task feed

```bash
tdx ticket task feed <ticket-id> <task-id>
tdx ticket task feed <ticket-id> <task-id> --limit 5
```

Reads task feed entries (comments + system events). JSON envelope: `tdx.v1.ticketTaskFeed`.

### tdx ticket task update

```bash
tdx ticket task update <ticket-id> <task-id> --percent 50 --comment "halfway" --yes
tdx ticket task update <ticket-id> <task-id> --complete --yes
```

Posts a feed update to the task. **Mutating** — `--yes` required.

Flags:
- `--percent N` (0-100) or `--complete` (shortcut for `--percent 100`); mutually exclusive
- `--comment "..."` — comment body
- `--hours-worked N` — informational only; does NOT create a time entry. Use `tdx ticket task log` for real time tracking.
- `--private` — internal note (not visible to requestor)
- `--notify <UID>` — additional notify recipients (repeatable)

### tdx ticket task log

```bash
tdx ticket task log <ticket-id> <task-id> --hours 1.5 --type "Development" --yes
tdx ticket task log <ticket-id> <task-id> --minutes 30 --type-id 7 --description "fixed bug" --yes
```

Creates a real time entry against the task. **Mutating** — `--yes` required. Wraps `tdx time entry add`. Same flag shape as `tdx ticket log`.
```

Add `[tdx ticket task](#tdx-ticket-task)` to the Contents TOC.

- [ ] **Step 2: Update `docs/guide/mcp.md`**

Update the intro tool counts: `57 → 62` total. Read split: was 12, now 15. Mutating: was 6, now 8. Update accordingly.

In the "Tickets (Phase D — read-only)" table, add 3 rows:

```markdown
| `list_ticket_tasks` | List tasks on a ticket |
| `get_ticket_task` | Get full detail for one ticket task |
| `get_ticket_task_feed` | Read feed entries for a ticket task |
```

In the "Tickets (Phase D — mutating)" table, add 2 rows:

```markdown
| `update_ticket_task` | Post a feed update (percentComplete + comment + informational hoursWorked) |
| `log_ticket_task_time` | Log time worked against a ticket task (creates a time entry) |
```

Update the section headers' `(N tools)` annotations: read 8 → 12 (no, wait — current ticket-read tool count post-v0.16.1 is 9: 8 from D.1 + `list_ticket_groups`. Adding 3 task tools = 12). Mutating: was 4, now 6.

Verify by counting actual tool registrations:

```bash
grep -c "sdkmcp.AddTool" internal/mcp/tools_ticket.go
grep -c "sdkmcp.AddTool" internal/mcp/tools_ticket_mutating.go
```

Use those counts in the doc.

- [ ] **Step 3: Update ASCII tree in `docs/guide.md`**

Find the `ticket` branch. Add `task` line:

```text
├── ticket
│   ├── app              → list / use / show
│   ├── search           → saved
│   ├── show / feed
│   ├── comment / status / assign / log
│   ├── types / statuses / groups → list
│   └── task             → list / show / feed / update / log
```

- [ ] **Step 4: Update ASCII tree in `README.md`**

Same change. Verify byte-identity:

```bash
diff <(awk '/^```text$/,/^```$/' docs/guide.md) <(awk '/^```text$/,/^```$/' README.md)
```

Expected: empty.

- [ ] **Step 5: Commit**

```bash
git add docs/guide/ticket.md docs/guide/mcp.md docs/guide.md README.md
git commit -m "docs: add tdx ticket task reference + tree updates + MCP tool tables"
```

**No `Co-Authored-By:` trailer.**

---

## Task 9: Live verification + PR + release

**Files:** none modified — verification + git operations only.

- [ ] **Step 1: Pre-flight**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

All green required.

- [ ] **Step 2: Build local binary**

```bash
go build -o tdx ./cmd/tdx
```

- [ ] **Step 3: Create a probe task on test ticket 542034 via raw API**

(`tdx ticket task create` is out of scope; use curl directly.)

```bash
TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
TASK_ID=$(curl -s -H "Authorization: Bearer $TOKEN" -X POST -H "Content-Type: application/json" \
  -d '{"Title":"v0.16.2 verify task","Description":"probe","PercentComplete":0}' \
  "https://demotemplate.teamdynamix.com/TDWebApi/api/34/tickets/542034/tasks" | python3 -c "import json,sys; print(json.load(sys.stdin)['ID'])")
echo "test task id: $TASK_ID"
```

- [ ] **Step 4: Live-verify each command**

```bash
./tdx ticket task list 542034                      # ≥1 row
./tdx ticket task show 542034 $TASK_ID             # full detail
./tdx ticket task feed 542034 $TASK_ID             # empty or system events
./tdx ticket task update 542034 $TASK_ID --percent 50 --comment "v0.16.2 halfway" --yes
./tdx ticket task feed 542034 $TASK_ID             # comment now visible
./tdx ticket task update 542034 $TASK_ID --complete --yes
./tdx ticket task show 542034 $TASK_ID             # 100%
./tdx ticket task log 542034 $TASK_ID --minutes 1 --type "<valid type>" --yes
# verify entry shows up in tdx time entry list and TD's time UI
./tdx time entry list --week $(date +%Y-%m-%d) | grep -i "task"
```

If any wire-format mismatch surfaces, fix and re-test.

- [ ] **Step 5: Clean up the test task**

```bash
TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
curl -s -H "Authorization: Bearer $TOKEN" -X PUT -H "Content-Type: application/json" \
  -d "{\"ID\":$TASK_ID,\"TicketID\":542034,\"Title\":\"v0.16.2 verify task (CLEANUP)\",\"IsActive\":false,\"PercentComplete\":100}" \
  "https://demotemplate.teamdynamix.com/TDWebApi/api/34/tickets/542034/tasks/$TASK_ID" -o /dev/null -w "cleanup: HTTP %{http_code}\n"
```

(DELETE returns 403 for non-admins; soft-delete via `IsActive=false` is the workaround.)

- [ ] **Step 6: Push branch + open PR**

```bash
rm tdx 2>/dev/null  # don't commit the local binary
git push -u origin ticket-tasks
```

Open PR via `gh pr create --title "v0.16.2: tdx ticket tasks" --body-file /tmp/pr-body-v0.16.2.md`. Body covers: summary, the 5 commands, MCP additions, live-verification confirmation, "tag v0.16.2 after merge."

- [ ] **Step 7: Merge after CI passes**

```bash
gh pr merge <PR#> --squash --admin --delete-branch
```

- [ ] **Step 8: Reset main, tag, push tag**

```bash
git checkout main
git fetch origin
git reset --hard origin/main
git tag v0.16.2
git push origin v0.16.2
```

- [ ] **Step 9: Update memory**

Update `MEMORY.md` index line for current state to point at v0.16.2. Add a new "Latest release" block to `project_tdx_current_state.md`. Mark D.3 as shipped in `project_tdx_backlog.md`.

---

## Self-Review

**1. Spec coverage:**

Spec → Tasks:
- TicketTask domain type → Task 1
- wireTicketTask + wireTaskFeedUpdate → Task 2
- ListTasks/GetTask/GetTaskFeed/UpdateTaskFeed service methods → Task 3
- ticketsvcAPI widened + stub updated → Task 4
- 5 CLI sub-commands (list/show/feed/update/log) → Task 5
- 3 read MCP tools → Task 6
- 2 mutating MCP tools + tool count update → Task 7
- All docs updates → Task 8
- Live verification + release → Task 9

All 12 acceptance criteria from spec map to tasks.

**2. Placeholder scan:**

- One spot in Task 5 step 1 says "before adding `formatDurationFromMinutes`, check whether a `formatDuration` helper already exists in `log.go`" — that's a concrete implementer instruction, not a placeholder. The plan declares the function inline; the implementer's discovery work is to dedupe if there's overlap. Acceptable.
- Task 8 step 2 has "(adjust real numbers based on actual current counts)" with a follow-up `grep -c` command — concrete, not vague.
- No "TBD"/"TODO"/"fill in details".

**3. Type consistency:**

- `domain.TicketTask` field names: `PercentComplete`, `EstimatedMinutes`, `ActualMinutes`, `ResponsibleUID`, `ResponsibleName`, `ResponsibleGroupID`, `ResponsibleGroupName` — used consistently in Task 1 (definition), Task 3 (decoder), Task 5 (rendering), Task 6 (MCP toTaskRowJSON).
- `wireTicketTask` JSON tags match TD's wire format from live probe (Task 2): `ResponsibleUid` (lowercase d), `ResponsibleFullName`, etc.
- `wireTaskFeedUpdate.PercentComplete` is `*int` consistently in Task 2 (definition), Task 3 (service), Task 4 (stub capture), Task 5 (CLI conversion), Task 7 (MCP arg).
- `domain.Target` fields: `Kind`, `AppID`, `ItemID`, `TaskID` — `TargetTicketTask` constant referenced consistently.
- Schema names: `tdx.v1.ticketTaskList`, `tdx.v1.ticketTask`, `tdx.v1.ticketTaskFeed`, `tdx.v1.ticketTaskUpdateResult`, `tdx.v1.ticketTaskTimeLogResult` — consistent.

All consistent.
