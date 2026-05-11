# tdx Ticket MVP (Phase D.1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `tdx ticket ...` namespace covering daily ticket workflows (search, show, feed, light mutations, saved-search execution) plus the `tdx ticket log` time crossover. Ship as v0.16.0.

**Architecture:** Bottom-up — domain types → service layer (`ticketsvc`) → CLI layer (`internal/cli/ticket`) → MCP tools — following the same shape as the existing `peoplesvc` / `internal/cli/people` / `tools_people.go` triple. Each new ticket command exposes a pure runner function (`runTicketX`) with a cobra wrapper on top; tests target the runner directly. Per-profile `ticketAppID` config provides the `appId` default; `--app <id>` overrides per-call.

**Tech Stack:** Go 1.26.2; cobra; existing `tdxhttp` client; `httptest` for service tests; `golangci-lint` for static checks.

**Spec:** [`docs/specs/2026-05-08-tdx-ticket-mvp.md`](../specs/2026-05-08-tdx-ticket-mvp.md)

---

## File Structure

After this plan completes:

```
internal/
├── domain/
│   ├── ticket.go                  # NEW — Ticket, TicketStatus, TicketType, TicketApp,
│   │                              #       TicketFeedEntry, TicketSavedSearch, TicketSearchFilter
│   └── ticket_test.go             # NEW — small constructor/decode helpers if any
├── svc/
│   └── ticketsvc/                 # NEW package
│       ├── service.go             # constructor, clientFor (mirrors peoplesvc/service.go)
│       ├── types.go               # wire structs matching TD JSON
│       ├── apps.go                # ListApps
│       ├── metadata.go            # ListStatuses / ListTypes / ResolveStatusByName / ResolveTypeByName
│       ├── tickets.go             # GetTicket / SearchTickets / PatchTicket
│       ├── feed.go                # GetFeed / AddFeed
│       ├── saved_searches.go      # ListSavedSearches / RunSavedSearch
│       └── *_test.go              # one per file, httptest fixtures
├── cli/
│   └── ticket/                    # NEW package
│       ├── ticket.go              # top-level cobra cmd + sub-cmd registration
│       ├── helpers.go             # resolveAppID, resolvePrincipal, resolveStatus, partialResultBanner
│       ├── helpers_test.go
│       ├── app.go                 # app list / use / show
│       ├── app_test.go
│       ├── search.go              # search + search saved (list / run)
│       ├── search_test.go
│       ├── show.go                # show (incl. this-week time crossover)
│       ├── show_test.go
│       ├── feed.go                # feed
│       ├── feed_test.go
│       ├── comment.go             # comment (mutating)
│       ├── comment_test.go
│       ├── status.go              # status (mutating)
│       ├── status_test.go
│       ├── assign.go              # assign (mutating)
│       ├── assign_test.go
│       ├── log.go                 # log (mutating, time crossover)
│       ├── log_test.go
│       ├── types.go               # types list
│       ├── types_test.go
│       ├── statuses.go            # statuses list
│       └── statuses_test.go
├── mcp/
│   ├── tools_ticket.go            # NEW — register 12 tools
│   └── tools_ticket_test.go       # NEW
├── domain/profile.go              # MODIFY — add TicketAppID int json/yaml field
└── cli/root.go                    # MODIFY — register `ticket` subcommand
docs/
├── specs/2026-05-08-tdx-ticket-mvp.md   # already exists
├── plans/2026-05-08-tdx-ticket-mvp.md   # this file
├── guide.md                       # MODIFY — ASCII tree adds ticket branch; reference list adds ticket link
├── guide/
│   ├── ticket.md                  # NEW
│   └── mcp.md                     # MODIFY — add Phase D ticket tool tables (read + mutating)
└── (manual-tests/ untouched)
README.md                          # MODIFY — ASCII tree adds ticket branch (byte-identical to guide.md)
```

## Established Patterns to Follow

Read these BEFORE starting any task. They're the model for every layer:

- **Service layer model:** `/Users/ipm/code/tdx/internal/svc/peoplesvc/users.go`, `service.go`, `types.go`. Note: thin wire structs; decoder funcs (`decodeUser`); `client.DoJSON(ctx, METHOD, path, body, &out)` for transport; `clientFor(profileName)` resolves an authenticated `tdxhttp.Client`.
- **CLI runner pattern:** `/Users/ipm/code/tdx/internal/cli/people/search.go` and `pools.go`. Note: factory `newSearchCmd(svc peoplesvcAPI) *cobra.Command`; runner `runPeopleSearch(ctx, w, svc, ...)`; tests call the runner directly.
- **Service interface in CLI package:** see `peoplesvcAPI` in `internal/cli/people/people.go`. Each CLI package defines its own minimal service interface (smallest set of methods used) so tests can stub easily.
- **MCP tool registration:** `/Users/ipm/code/tdx/internal/mcp/tools_people.go`. Tools are functions that read a JSON arg blob and return JSON output. Mutating tools require `args["confirm"] == true` or return an error.
- **JSON envelopes:** `/Users/ipm/code/tdx/internal/render/json.go` (`render.JSON`). Schema name lives in the envelope's `schema` field.

## Live-Verification Strategy

Per memory note `feedback_probe_wire_formats_early.md`: TD docs lie. The historical pattern in this repo:
1. Write services against documented wire types.
2. Run `go test ./...` to confirm fixture-based tests pass.
3. **Live-verify against Sample during Task 19** (before tag). Capture real responses; if any wire field is wrong, amend in a follow-up commit on the same branch.

Do **not** block tasks 1-18 on live access. Build against documented types; fix in Task 19 if needed.

## Branch + Versioning

- Branch: `ticket-mvp` (created in Task 0)
- Version bump: v0.16.0 (minor — additive feature, no breaking changes)
- Goreleaser pipeline: existing GitHub Actions handles `v*` tags

---

## Task 0: Create branch

**Files:** none (git operation)

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: `On branch main`, `nothing to commit, working tree clean`. The spec commit `793c8c5` is already on main.

- [ ] **Step 2: Create feature branch**

```bash
git checkout -b ticket-mvp
```

Expected: `Switched to a new branch 'ticket-mvp'`.

---

## Task 1: Domain types (`internal/domain/ticket.go`)

**Files:**
- Create: `internal/domain/ticket.go`
- Create: `internal/domain/ticket_test.go`

- [ ] **Step 1: Define types**

Write `internal/domain/ticket.go`:

```go
package domain

import "time"

// TicketApp represents a TD ticketing application. A tenant has multiple
// ticket apps (service desk, project management, departmental). Every
// ticket-API call is scoped to one app via {appId} in the path.
type TicketApp struct {
	ID          int
	Name        string
	Description string
	Active      bool
	AppType     string // e.g. "TDNext", "TDTickets" — informational
}

// TicketStatus is one entry in an app's status workflow.
type TicketStatus struct {
	ID        int
	Name      string
	IsClosed  bool
	IsDefault bool
	Order     float64
}

// TicketType categorizes tickets within an app.
type TicketType struct {
	ID          int
	Name        string
	Description string
	Active      bool
}

// Ticket is one row from POST /tickets/search (partial; IsFull=false) or
// GET /tickets/{id} (full; IsFull=true).
type Ticket struct {
	ID               int
	AppID            int
	Title            string
	Description      string
	StatusID         int
	StatusName       string
	TypeID           int
	TypeName         string
	PriorityID       int
	PriorityName     string
	AccountID        int
	AccountName      string
	ResponsibleUID   string
	ResponsibleName  string
	RequestorUID     string
	RequestorName    string
	CreatedDate      time.Time
	ModifiedDate     time.Time
	EstimatedMinutes int
	ActualMinutes    int
	Tags             []string
	IsFull           bool
}

// TicketFeedEntry is a single feed row (comment, status change, etc).
type TicketFeedEntry struct {
	ID         int
	AuthorUID  string
	AuthorName string
	CreatedAt  time.Time
	Body       string
	IsPrivate  bool
	EventKind  string // "comment" | "statusChange" | "assignment" | "task" | etc.
}

// TicketSearchFilter drives POST /tickets/search.
type TicketSearchFilter struct {
	AppID         int
	StatusIDs     []int
	AssigneeUIDs  []string
	RequestorUIDs []string
	AccountIDs    []int
	Text          string
	IncludeClosed bool
	Limit         int
}

// TicketSavedSearch is one row from GET /tickets/searches.
type TicketSavedSearch struct {
	ID          int
	Name        string
	OwnerUID    string
	OwnerName   string
	Description string
}
```

- [ ] **Step 2: Write trivial constructor tests**

Write `internal/domain/ticket_test.go`:

```go
package domain

import "testing"

func TestTicketIsFullDefault(t *testing.T) {
	tk := Ticket{ID: 1}
	if tk.IsFull {
		t.Fatal("Ticket.IsFull must default to false")
	}
}

func TestTicketSearchFilterZeroValueIsValid(t *testing.T) {
	// No fields are required at construction time — service layer applies defaults.
	_ = TicketSearchFilter{}
}
```

- [ ] **Step 3: Verify build + test**

```bash
go build ./... && go test ./internal/domain/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/ticket.go internal/domain/ticket_test.go
git commit -m "feat(domain): add ticket domain types"
```

---

## Task 2: Profile config extension (`TicketAppID`)

**Files:**
- Modify: `internal/domain/profile.go` (or wherever the profile struct lives — find it first)
- Modify: corresponding `*_test.go`

- [ ] **Step 1: Find the profile struct**

```bash
grep -rn "type Profile struct" internal/
```

Note the file path returned. Most likely `internal/domain/profile.go` or `internal/config/...`.

- [ ] **Step 2: Add `TicketAppID int` field**

In the Profile struct, add:

```go
// TicketAppID is the default ticket-app ID for this profile.
// Set via `tdx ticket app use <id>`. Optional — when zero, all
// tdx ticket commands require --app <id>.
TicketAppID int `yaml:"ticketAppID,omitempty" json:"ticketAppID,omitempty"`
```

Place after the existing URL field, before any unexported fields.

- [ ] **Step 3: Write a config round-trip test**

If there's an existing `profile_test.go`, add:

```go
func TestProfileTicketAppIDRoundTrip(t *testing.T) {
	p := Profile{Name: "test", URL: "https://x.example.com/", TicketAppID: 31}
	b, err := yaml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got Profile
	if err := yaml.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.TicketAppID != 31 {
		t.Fatalf("ticketAppID round-trip: got %d, want 31", got.TicketAppID)
	}
}

func TestProfileTicketAppIDOmittedWhenZero(t *testing.T) {
	p := Profile{Name: "test", URL: "https://x.example.com/"}
	b, err := yaml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "ticketAppID") {
		t.Fatalf("ticketAppID must be omitted when zero; got: %s", b)
	}
}
```

(If `yaml` import differs or marshal helper is in a different package, follow the existing test style in that file.)

- [ ] **Step 4: Verify build + test**

```bash
go test ./internal/domain/... && go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/profile.go internal/domain/profile_test.go
git commit -m "feat(config): add ticketAppID per-profile config field"
```

(Adjust file paths in the `git add` if Profile lives elsewhere.)

---

## Task 3: ticketsvc skeleton + apps.go

**Files:**
- Create: `internal/svc/ticketsvc/service.go`
- Create: `internal/svc/ticketsvc/types.go`
- Create: `internal/svc/ticketsvc/apps.go`
- Create: `internal/svc/ticketsvc/apps_test.go`

Read `/Users/ipm/code/tdx/internal/svc/peoplesvc/service.go` first — copy the constructor/`clientFor` pattern verbatim, swapping `peoplesvc` for `ticketsvc`.

- [ ] **Step 1: service.go (constructor + clientFor)**

```go
package ticketsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/tdxhttp"
)

type Service struct {
	paths config.Paths
}

func New(paths config.Paths) *Service {
	return &Service{paths: paths}
}

func (s *Service) clientFor(profileName string) (*tdxhttp.Client, error) {
	auth := authsvc.New(s.paths)
	return auth.ClientForProfile(profileName)
}

// resolveAppID returns the appID to use: if explicit > 0, use it;
// otherwise fall back to profile.TicketAppID; if both zero, error.
func (s *Service) resolveAppID(profileName string, explicit int) (int, error) {
	if explicit > 0 {
		return explicit, nil
	}
	auth := authsvc.New(s.paths)
	prof, err := auth.GetProfile(profileName)
	if err != nil {
		return 0, err
	}
	if prof.TicketAppID == 0 {
		return 0, fmt.Errorf("no ticket app configured for profile %q (run `tdx ticket app list` then `tdx ticket app use <id>`, or pass --app <id>)", profileName)
	}
	return prof.TicketAppID, nil
}
```

(If `authsvc.GetProfile` doesn't exist, `grep -rn "func.*GetProfile\|ResolveProfile\|ProfileByName" internal/svc/authsvc/` to find the existing accessor.)

- [ ] **Step 2: types.go (initial wire types — apps only)**

```go
package ticketsvc

// wireApp matches one row in the response from
// GET /TDWebApi/api/applications. The endpoint returns all platform
// applications; ListApps filters to ticketing apps via Type contains "Ticket".
type wireApp struct {
	ID          int    `json:"AppID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Active      bool   `json:"Active"`
	Type        string `json:"Type"`
	AppClass    string `json:"AppClass"`
}
```

- [ ] **Step 3: apps.go**

```go
package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListApps fetches all platform applications and filters to ticketing apps.
// TD's tenant-level /api/applications endpoint returns every app type
// (tickets, projects, KB, assets, etc.); we filter to AppClass containing
// "Ticket" or Type "TDNext" with ticket capability.
func (s *Service) ListApps(ctx context.Context, profileName string) ([]domain.TicketApp, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireApp
	if err := client.DoJSON(ctx, "GET", "/TDWebApi/api/applications", nil, &wire); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	out := make([]domain.TicketApp, 0, len(wire))
	for _, w := range wire {
		if !isTicketApp(w) {
			continue
		}
		out = append(out, domain.TicketApp{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			Active:      w.Active,
			AppType:     w.Type,
		})
	}
	return out, nil
}

func isTicketApp(w wireApp) bool {
	// Heuristic: TDNext + AppClass containing "Tickets" is the canonical
	// shape; broaden if live verification reveals other discriminators.
	return strings.Contains(strings.ToLower(w.AppClass), "ticket")
}
```

- [ ] **Step 4: apps_test.go (httptest fixture)**

```go
package ticketsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	// import config + auth scaffolding consistent with peoplesvc tests
)

func TestListAppsFiltersToTicketApps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/applications" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"AppID": 31, "Name": "Service Desk", "Description": "Help desk", "Active": true, "Type": "TDNext", "AppClass": "TDTickets"},
			{"AppID": 50, "Name": "Knowledge", "Description": "KB", "Active": true, "Type": "TDNext", "AppClass": "TDKnowledgeBase"},
			{"AppID": 71, "Name": "Project Tickets", "Description": "PM", "Active": true, "Type": "TDNext", "AppClass": "TDTicketsProjects"}
		]`))
	}))
	defer srv.Close()

	svc, prof := newTestService(t, srv.URL)  // helper similar to peoplesvc tests
	apps, err := svc.ListApps(context.Background(), prof)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("want 2 ticket apps, got %d: %+v", len(apps), apps)
	}
	if apps[0].ID != 31 || apps[1].ID != 71 {
		t.Fatalf("filtered IDs wrong: %+v", apps)
	}
}
```

(Helper `newTestService` mirrors the equivalent in `peoplesvc/users_test.go` — find it via `grep -rn "func newTestService\|func newServiceForTest" internal/svc/peoplesvc/`. Replicate the same scaffold in `ticketsvc/service_test.go`.)

- [ ] **Step 5: Verify**

```bash
go test ./internal/svc/ticketsvc/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/svc/ticketsvc/
git commit -m "feat(ticketsvc): add ListApps with /api/applications filtering"
```

---

## Task 4: ticketsvc metadata (statuses + types + name resolvers)

**Files:**
- Modify: `internal/svc/ticketsvc/types.go`
- Create: `internal/svc/ticketsvc/metadata.go`
- Create: `internal/svc/ticketsvc/metadata_test.go`

- [ ] **Step 1: Add wire types**

Append to `types.go`:

```go
type wireTicketStatus struct {
	ID        int     `json:"ID"`
	Name      string  `json:"Name"`
	IsActive  bool    `json:"IsActive"`
	Order     float64 `json:"Order"`
	IsDefault bool    `json:"IsDefault"`
	StatusClass int   `json:"StatusClass"` // 6 = Closed (TD convention; verify live)
}

type wireTicketType struct {
	ID          int    `json:"ID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	IsActive    bool   `json:"IsActive"`
}
```

- [ ] **Step 2: Implement metadata.go**

```go
package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListStatuses returns all active statuses for the given app.
func (s *Service) ListStatuses(ctx context.Context, profileName string, appID int) ([]domain.TicketStatus, error) {
	id, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTicketStatus
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/statuses", id)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list ticket statuses: %w", err)
	}
	out := make([]domain.TicketStatus, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketStatus{
			ID:        w.ID,
			Name:      strings.TrimSpace(w.Name),
			IsClosed:  w.StatusClass == 6, // TD's Closed class; verify live
			IsDefault: w.IsDefault,
			Order:     w.Order,
		})
	}
	return out, nil
}

// ListTypes returns active ticket types for the given app.
func (s *Service) ListTypes(ctx context.Context, profileName string, appID int) ([]domain.TicketType, error) {
	id, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTicketType
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/types?isActive=true", id)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list ticket types: %w", err)
	}
	out := make([]domain.TicketType, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketType{
			ID:          w.ID,
			Name:        strings.TrimSpace(w.Name),
			Description: w.Description,
			Active:      w.IsActive,
		})
	}
	return out, nil
}

// ResolveStatusByName finds a status by case-insensitive exact match.
// Returns an error listing candidates if zero or >1 match.
func (s *Service) ResolveStatusByName(ctx context.Context, profileName string, appID int, name string) (domain.TicketStatus, error) {
	statuses, err := s.ListStatuses(ctx, profileName, appID)
	if err != nil {
		return domain.TicketStatus{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketStatus
	for _, st := range statuses {
		if strings.ToLower(st.Name) == target {
			matches = append(matches, st)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketStatus{}, fmt.Errorf("no ticket status matches %q (use `tdx ticket statuses list` to see options)", name)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%d (%s)", m.ID, m.Name))
		}
		return domain.TicketStatus{}, fmt.Errorf("multiple statuses match %q: %s — pass --status-id <int> instead", name, strings.Join(names, ", "))
	}
}

// ResolveTypeByName finds a ticket type by case-insensitive exact match.
func (s *Service) ResolveTypeByName(ctx context.Context, profileName string, appID int, name string) (domain.TicketType, error) {
	types, err := s.ListTypes(ctx, profileName, appID)
	if err != nil {
		return domain.TicketType{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketType
	for _, tt := range types {
		if strings.ToLower(tt.Name) == target {
			matches = append(matches, tt)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketType{}, fmt.Errorf("no ticket type matches %q", name)
	case 1:
		return matches[0], nil
	default:
		return domain.TicketType{}, fmt.Errorf("multiple ticket types match %q (use --type-id instead)", name)
	}
}
```

- [ ] **Step 3: Tests**

Write `metadata_test.go` with httptest fixtures: verify `ListStatuses` filters trims pool-name whitespace, `ResolveStatusByName` handles 0/1/many cases. Mirror style of `peoplesvc/pools_test.go`.

Key cases (write fixture+test for each):
- `TestListStatuses` returns 4 rows with one Closed (StatusClass=6).
- `TestResolveStatusByNameSingleMatch` resolves "In Progress" to the unique row.
- `TestResolveStatusByNameNoMatch` errors with "no ticket status matches".
- `TestResolveStatusByNameMultipleMatches` errors with all candidate IDs.
- `TestListTypes` returns active types.
- `TestResolveTypeByNameSingleMatch` works.

- [ ] **Step 4: Verify**

```bash
go test ./internal/svc/ticketsvc/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/svc/ticketsvc/metadata.go internal/svc/ticketsvc/metadata_test.go internal/svc/ticketsvc/types.go
git commit -m "feat(ticketsvc): add ListStatuses/ListTypes + name resolvers"
```

---

## Task 5: ticketsvc tickets.go (Get, Search, Patch)

**Files:**
- Modify: `internal/svc/ticketsvc/types.go`
- Create: `internal/svc/ticketsvc/tickets.go`
- Create: `internal/svc/ticketsvc/tickets_test.go`

- [ ] **Step 1: Wire types**

Append to `types.go`:

```go
type wireTicket struct {
	ID                int      `json:"ID"`
	AppID             int      `json:"AppID"`
	Title             string   `json:"Title"`
	Description       string   `json:"Description"`
	StatusID          int      `json:"StatusID"`
	StatusName        string   `json:"StatusName"`
	TypeID            int      `json:"TypeID"`
	TypeName          string   `json:"TypeName"`
	PriorityID        int      `json:"PriorityID"`
	PriorityName      string   `json:"PriorityName"`
	AccountID         int      `json:"AccountID"`
	AccountName       string   `json:"AccountName"`
	ResponsibleUid    string   `json:"ResponsibleUid"`
	ResponsibleFullName string `json:"ResponsibleFullName"`
	RequestorUid      string   `json:"RequestorUid"`
	RequestorName     string   `json:"RequestorName"`
	CreatedDate       string   `json:"CreatedDate"`
	ModifiedDate      string   `json:"ModifiedDate"`
	EstimatedMinutes  int      `json:"EstimatedMinutes"`
	ActualMinutes     int      `json:"ActualMinutes"`
	Tags              []string `json:"Tags"`
}

type wireTicketSearch struct {
	StatusIDs       []int    `json:"StatusIDs,omitempty"`
	ResponsibilityUids []string `json:"ResponsibilityUids,omitempty"`
	RequestorUids   []string `json:"RequestorUids,omitempty"`
	AccountIDs      []int    `json:"AccountIDs,omitempty"`
	SearchText      string   `json:"SearchText,omitempty"`
	IsOpen          *bool    `json:"IsOpen,omitempty"`
	MaxResults      int      `json:"MaxResults,omitempty"`
}

// PatchOp is exported (vs. unexported wireXxx) because the CLI layer
// (Task 15) constructs ops directly. Other wire types stay unexported.
type PatchOp struct {
	Op    string      `json:"op"`    // "replace", "add", etc.
	Path  string      `json:"path"`  // "/StatusID", "/ResponsibleUid", etc.
	Value interface{} `json:"value"`
}
```

- [ ] **Step 2: tickets.go**

```go
package ticketsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetTicket fetches a single ticket by ID. Returns IsFull=true on the result.
func (s *Service) GetTicket(ctx context.Context, profileName string, appID, id int) (domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return domain.Ticket{}, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.Ticket{}, err
	}
	var w wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d", resolvedAppID, id)
	if err := client.DoJSON(ctx, "GET", path, nil, &w); err != nil {
		return domain.Ticket{}, fmt.Errorf("get ticket %d: %w", id, err)
	}
	return decodeTicket(w, true), nil
}

// SearchTickets calls POST /tickets/search. Returns partial records (IsFull=false).
func (s *Service) SearchTickets(ctx context.Context, profileName string, filter domain.TicketSearchFilter) ([]domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, filter.AppID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	req := wireTicketSearch{
		StatusIDs:          filter.StatusIDs,
		ResponsibilityUids: filter.AssigneeUIDs,
		RequestorUids:      filter.RequestorUIDs,
		AccountIDs:         filter.AccountIDs,
		SearchText:         filter.Text,
		MaxResults:         filter.Limit,
	}
	if !filter.IncludeClosed {
		t := true
		req.IsOpen = &t
	}
	if req.MaxResults == 0 {
		req.MaxResults = 50
	}
	var wire []wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/search", resolvedAppID)
	if err := client.DoJSON(ctx, "POST", path, req, &wire); err != nil {
		return nil, fmt.Errorf("search tickets: %w", err)
	}
	out := make([]domain.Ticket, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTicket(w, false))
	}
	return out, nil
}

// PatchTicket applies one or more JSON-Patch operations to a ticket.
// Returns the updated ticket (IsFull=true).
func (s *Service) PatchTicket(ctx context.Context, profileName string, appID, id int, ops []PatchOp) (domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return domain.Ticket{}, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.Ticket{}, err
	}
	var w wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d", resolvedAppID, id)
	if err := client.DoJSON(ctx, "PATCH", path, ops, &w); err != nil {
		return domain.Ticket{}, fmt.Errorf("patch ticket %d: %w", id, err)
	}
	return decodeTicket(w, true), nil
}

func decodeTicket(w wireTicket, full bool) domain.Ticket {
	return domain.Ticket{
		ID:               w.ID,
		AppID:            w.AppID,
		Title:            w.Title,
		Description:      w.Description,
		StatusID:         w.StatusID,
		StatusName:       w.StatusName,
		TypeID:           w.TypeID,
		TypeName:         w.TypeName,
		PriorityID:       w.PriorityID,
		PriorityName:     w.PriorityName,
		AccountID:        w.AccountID,
		AccountName:      w.AccountName,
		ResponsibleUID:   w.ResponsibleUid,
		ResponsibleName:  w.ResponsibleFullName,
		RequestorUID:     w.RequestorUid,
		RequestorName:    w.RequestorName,
		CreatedDate:      parseTDTime(w.CreatedDate),
		ModifiedDate:     parseTDTime(w.ModifiedDate),
		EstimatedMinutes: w.EstimatedMinutes,
		ActualMinutes:    w.ActualMinutes,
		Tags:             w.Tags,
		IsFull:           full,
	}
}

// parseTDTime parses TD's date format. TD historically uses both
// `2006-01-02T15:04:05Z` and `2006-01-02T15:04:05.000-04:00`. Tolerate both.
func parseTDTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
```

(Watch the `parseTDTime` — TD's actual format must be verified live. Per `feedback_probe_wire_formats_early.md`, fixtures captured live in Task 19 are authoritative.)

- [ ] **Step 3: Tests**

Tests in `tickets_test.go`:
- `TestGetTicketFull` — happy path; verify `IsFull` true.
- `TestSearchTicketsDefaultsLimit` — `MaxResults=0` → request body has `MaxResults: 50`.
- `TestSearchTicketsIncludeClosed` — `IncludeClosed: false` sets `IsOpen: true` in the body; `IncludeClosed: true` omits `IsOpen`.
- `TestSearchTicketsMapsAssignees` — `AssigneeUIDs` → `ResponsibilityUids` in wire.
- `TestPatchTicketSendsOps` — verify request body contains JSON-Patch ops.
- `TestParseTDTimeMultipleFormats` — both RFC3339 with TZ and the `Z` form.

- [ ] **Step 4: Verify**

```bash
go test ./internal/svc/ticketsvc/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/svc/ticketsvc/tickets.go internal/svc/ticketsvc/tickets_test.go internal/svc/ticketsvc/types.go
git commit -m "feat(ticketsvc): add GetTicket / SearchTickets / PatchTicket"
```

---

## Task 6: ticketsvc feed.go

**Files:**
- Modify: `internal/svc/ticketsvc/types.go`
- Create: `internal/svc/ticketsvc/feed.go`
- Create: `internal/svc/ticketsvc/feed_test.go`

- [ ] **Step 1: Wire types**

```go
type wireFeedEntry struct {
	ID            int    `json:"ID"`
	CreatedUid    string `json:"CreatedUid"`
	CreatedFullName string `json:"CreatedFullName"`
	CreatedDate   string `json:"CreatedDate"`
	Body          string `json:"Body"`
	IsPrivate     bool   `json:"IsPrivate"`
	UpdateType    int    `json:"UpdateType"` // TD enum; verify live
}

type wireFeedAdd struct {
	Comments  string   `json:"Comments"`
	Notify    []string `json:"Notify,omitempty"`
	IsPrivate bool     `json:"IsPrivate"`
}
```

- [ ] **Step 2: feed.go**

```go
package ticketsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetFeed fetches feed entries for a ticket, newest first per TD default.
func (s *Service) GetFeed(ctx context.Context, profileName string, appID, ticketID int) ([]domain.TicketFeedEntry, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/feed", resolvedAppID, ticketID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get ticket feed %d: %w", ticketID, err)
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

// AddFeed posts a comment to a ticket. Returns the created entry's ID.
func (s *Service) AddFeed(ctx context.Context, profileName string, appID, ticketID int, body string, isPrivate bool, notify []string) (int, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return 0, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return 0, err
	}
	req := wireFeedAdd{
		Comments:  body,
		Notify:    notify,
		IsPrivate: isPrivate,
	}
	var resp wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/feed", resolvedAppID, ticketID)
	if err := client.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return 0, fmt.Errorf("add ticket feed %d: %w", ticketID, err)
	}
	return resp.ID, nil
}

// classifyFeedKind maps TD's UpdateType integer to a human label.
// Mapping per TD docs (verify live in Task 19):
// 0 unknown / 1 comment / 2 status / 3 attachment / 4 task / ...
func classifyFeedKind(t int) string {
	switch t {
	case 1:
		return "comment"
	case 2:
		return "statusChange"
	case 3:
		return "attachment"
	case 4:
		return "task"
	default:
		return "event"
	}
}
```

- [ ] **Step 3: Tests**

`feed_test.go`:
- `TestGetFeedDecodes` — fixture with 3 entries; assert author+body+kind.
- `TestAddFeedSendsCommentBody` — captures POST body; verifies `Comments` and `IsPrivate`.
- `TestClassifyFeedKind` — table-driven over the known mappings.

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/svc/ticketsvc/...
git add internal/svc/ticketsvc/feed.go internal/svc/ticketsvc/feed_test.go internal/svc/ticketsvc/types.go
git commit -m "feat(ticketsvc): add GetFeed / AddFeed"
```

---

## Task 7: ticketsvc saved_searches.go

**Files:**
- Modify: `internal/svc/ticketsvc/types.go`
- Create: `internal/svc/ticketsvc/saved_searches.go`
- Create: `internal/svc/ticketsvc/saved_searches_test.go`

- [ ] **Step 1: Wire types + service**

```go
// in types.go:
type wireSavedSearch struct {
	ID          int    `json:"ID"`
	Name        string `json:"Name"`
	OwnerUid    string `json:"OwnerUid"`
	OwnerFullName string `json:"OwnerFullName"`
	Description string `json:"Description"`
}

type wireSavedSearchOptions struct {
	MaxResults int `json:"MaxResults,omitempty"`
}
```

```go
// saved_searches.go:
package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

func (s *Service) ListSavedSearches(ctx context.Context, profileName string, appID int) ([]domain.TicketSavedSearch, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireSavedSearch
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/searches", resolvedAppID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list saved searches: %w", err)
	}
	out := make([]domain.TicketSavedSearch, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketSavedSearch{
			ID: w.ID, Name: strings.TrimSpace(w.Name),
			OwnerUID: w.OwnerUid, OwnerName: w.OwnerFullName,
			Description: w.Description,
		})
	}
	return out, nil
}

func (s *Service) RunSavedSearch(ctx context.Context, profileName string, appID, searchID, limit int) ([]domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	req := wireSavedSearchOptions{MaxResults: limit}
	var wire []wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/searches/%d/results", resolvedAppID, searchID)
	if err := client.DoJSON(ctx, "POST", path, req, &wire); err != nil {
		return nil, fmt.Errorf("run saved search %d: %w", searchID, err)
	}
	out := make([]domain.Ticket, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTicket(w, false))
	}
	return out, nil
}

// ResolveSavedSearchByName finds a saved search by case-insensitive exact match.
func (s *Service) ResolveSavedSearchByName(ctx context.Context, profileName string, appID int, name string) (domain.TicketSavedSearch, error) {
	all, err := s.ListSavedSearches(ctx, profileName, appID)
	if err != nil {
		return domain.TicketSavedSearch{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketSavedSearch
	for _, ss := range all {
		if strings.ToLower(ss.Name) == target {
			matches = append(matches, ss)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketSavedSearch{}, fmt.Errorf("no saved search matches %q", name)
	case 1:
		return matches[0], nil
	default:
		return domain.TicketSavedSearch{}, fmt.Errorf("multiple saved searches match %q (use --search-id <int> instead)", name)
	}
}
```

- [ ] **Step 2: Tests** mirror metadata tests: list, single resolve, ambiguous resolve, no-match.

- [ ] **Step 3: Verify + commit**

```bash
go test ./internal/svc/ticketsvc/...
git add internal/svc/ticketsvc/saved_searches.go internal/svc/ticketsvc/saved_searches_test.go internal/svc/ticketsvc/types.go
git commit -m "feat(ticketsvc): add ListSavedSearches / RunSavedSearch + name resolver"
```

---

## Task 8: CLI ticket subcommand wiring + helpers

**Files:**
- Create: `internal/cli/ticket/ticket.go`
- Create: `internal/cli/ticket/helpers.go`
- Create: `internal/cli/ticket/helpers_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: ticket.go (top-level cmd + service interface)**

```go
package ticket

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ticketsvcAPI is the minimal interface CLI commands depend on.
// Defining it here (not in the service package) lets tests stub easily.
type ticketsvcAPI interface {
	ListApps(ctx context.Context, profile string) ([]domain.TicketApp, error)
	ListStatuses(ctx context.Context, profile string, appID int) ([]domain.TicketStatus, error)
	ListTypes(ctx context.Context, profile string, appID int) ([]domain.TicketType, error)
	ResolveStatusByName(ctx context.Context, profile string, appID int, name string) (domain.TicketStatus, error)
	GetTicket(ctx context.Context, profile string, appID, id int) (domain.Ticket, error)
	SearchTickets(ctx context.Context, profile string, filter domain.TicketSearchFilter) ([]domain.Ticket, error)
	PatchTicket(ctx context.Context, profile string, appID, id int, ops []ticketsvc.PatchOp) (domain.Ticket, error)
	GetFeed(ctx context.Context, profile string, appID, ticketID int) ([]domain.TicketFeedEntry, error)
	AddFeed(ctx context.Context, profile string, appID, ticketID int, body string, isPrivate bool, notify []string) (int, error)
	ListSavedSearches(ctx context.Context, profile string, appID int) ([]domain.TicketSavedSearch, error)
	RunSavedSearch(ctx context.Context, profile string, appID, searchID, limit int) ([]domain.Ticket, error)
	ResolveSavedSearchByName(ctx context.Context, profile string, appID int, name string) (domain.TicketSavedSearch, error)
}

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
	return cmd
}
```

(Each `newXCmd(svc ticketsvcAPI)` is defined in its own file — Tasks 9-16.)

CLI imports `ticketsvc.PatchOp` directly (it's exported in Task 5). The interface signature shown above reflects that.

- [ ] **Step 2: helpers.go**

```go
package ticket

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

// resolvePrincipal maps a CLI argument ("me" / UID / email / partial name)
// to a UID. "me" requires the authenticated UID — caller must pass it in.
func resolvePrincipal(ctx context.Context, peoplesvc peoplesvcAPI, profile, authedUID, arg string) (string, error) {
	if arg == "me" {
		if authedUID == "" {
			return "", fmt.Errorf("`me` requires an authenticated session — run `tdx auth status` to verify")
		}
		return authedUID, nil
	}
	// Looks like a UID? (36-char with dashes).
	if len(arg) >= 32 && strings.Count(arg, "-") >= 4 {
		return arg, nil
	}
	// Otherwise treat as an email or name; substring-search staff.
	users, err := peoplesvc.LookupPeople(ctx, profile, arg, 5)
	if err != nil {
		return "", err
	}
	if len(users) == 0 {
		return "", fmt.Errorf("no user matches %q", arg)
	}
	if len(users) > 1 {
		names := make([]string, 0, len(users))
		for _, u := range users {
			names = append(names, fmt.Sprintf("%s (%s)", u.FullName, u.Email))
		}
		return "", fmt.Errorf("multiple users match %q: %s — pass UID directly", arg, strings.Join(names, ", "))
	}
	return users[0].UID, nil
}

// peoplesvcAPI is the subset of peoplesvc used by ticket helpers.
type peoplesvcAPI interface {
	LookupPeople(ctx context.Context, profile string, q string, limit int) ([]domain.User, error)
}

// parseStatusArg returns (statusID, statusName-or-empty). If arg is purely
// numeric, treat as ID; else, name (caller resolves via ResolveStatusByName).
func parseStatusArg(arg string) (int, string) {
	if id, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil {
		return id, ""
	}
	return 0, arg
}

// partialResultBanner returns a footer line warning about partial records.
// Used by `tdx ticket search` and `tdx ticket search saved`.
func partialResultBanner(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("(%d row(s) — partial; use `tdx ticket show <id>` for full detail)", n)
}
```

- [ ] **Step 3: helpers_test.go**

```go
func TestResolvePrincipalMe(t *testing.T) {
	got, err := resolvePrincipal(context.Background(), nil, "", "uid-of-me", "me")
	if err != nil { t.Fatal(err) }
	if got != "uid-of-me" { t.Fatalf("want uid-of-me, got %s", got) }
}

func TestResolvePrincipalRawUID(t *testing.T) {
	uid := "12345678-1234-1234-1234-123456789012"
	got, err := resolvePrincipal(context.Background(), nil, "", "", uid)
	if err != nil { t.Fatal(err) }
	if got != uid { t.Fatalf("want %s, got %s", uid, got) }
}

func TestResolvePrincipalEmailLookupSingleMatch(t *testing.T) {
	stub := stubPeoplesvc{lookup: []domain.User{{UID: "alice-uid", FullName: "Alice", Email: "alice@uf.edu"}}}
	got, err := resolvePrincipal(context.Background(), &stub, "default", "", "alice@uf.edu")
	if err != nil { t.Fatal(err) }
	if got != "alice-uid" { t.Fatalf("want alice-uid, got %s", got) }
}

func TestResolvePrincipalAmbiguous(t *testing.T) {
	stub := stubPeoplesvc{lookup: []domain.User{
		{UID: "a", FullName: "Alice"}, {UID: "b", FullName: "Bob"},
	}}
	_, err := resolvePrincipal(context.Background(), &stub, "default", "", "a")
	if err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("want ambiguous error, got %v", err)
	}
}

func TestParseStatusArgNumeric(t *testing.T) {
	id, name := parseStatusArg("4")
	if id != 4 || name != "" { t.Fatal() }
}

func TestParseStatusArgName(t *testing.T) {
	id, name := parseStatusArg("In Progress")
	if id != 0 || name != "In Progress" { t.Fatal() }
}

type stubPeoplesvc struct{ lookup []domain.User }
func (s *stubPeoplesvc) LookupPeople(_ context.Context, _ string, _ string, _ int) ([]domain.User, error) {
	return s.lookup, nil
}
```

- [ ] **Step 4: Wire into root.go**

```go
// internal/cli/root.go — find where other subcommands are added, then:
import "github.com/iainmoffat/tdx/internal/cli/ticket"

// in the function that adds subcommands:
rootCmd.AddCommand(ticket.New())
```

- [ ] **Step 5: Verify**

```bash
go build ./... && go test ./internal/cli/ticket/...
./tdx ticket --help
```

Expected: `tdx ticket --help` shows the empty top-level command (sub-commands not yet defined will be missing). `go test` passes.

Wait — Step 1's `New()` references `newAppCmd`/`newSearchCmd`/etc that don't exist yet. At this stage, comment those out or use empty stubs that return `&cobra.Command{Use: "stub"}` so the build works. Subsequent tasks replace them with real implementations.

Adjust `ticket.go`:

```go
func New() *cobra.Command {
	return &cobra.Command{Use: "ticket", Short: "Manage TeamDynamix tickets"}
}
```

Sub-commands wire in as each file lands.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/ticket/ticket.go internal/cli/ticket/helpers.go internal/cli/ticket/helpers_test.go internal/cli/root.go
git commit -m "feat(cli/ticket): wire top-level command + helpers"
```

---

## Task 9: CLI app sub-group (list / use / show)

**Files:**
- Create: `internal/cli/ticket/app.go`
- Create: `internal/cli/ticket/app_test.go`
- Modify: `internal/cli/ticket/ticket.go` (uncomment/add `cmd.AddCommand(newAppCmd(nil))`)

- [ ] **Step 1: app.go**

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

func newAppCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Discover and select the default ticket app for this profile",
	}
	cmd.AddCommand(newAppListCmd(svc))
	cmd.AddCommand(newAppUseCmd())
	cmd.AddCommand(newAppShowCmd())
	return cmd
}

func newAppListCmd(svc ticketsvcAPI) *cobra.Command {
	var jsonFlag bool
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ticket apps in the tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			s := svc
			if s == nil { s = ticketsvc.New(paths) }
			return runAppList(cmd.Context(), cmd.OutOrStdout(), s, profile, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runAppList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, jsonOut bool) error {
	apps, err := svc.ListApps(ctx, profile)
	if err != nil { return err }
	if jsonOut {
		type appJSON struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Active      bool   `json:"active"`
		}
		out := make([]appJSON, 0, len(apps))
		for _, a := range apps {
			out = append(out, appJSON{ID: a.ID, Name: a.Name, Description: a.Description, Active: a.Active})
		}
		return render.JSON(w, struct {
			Schema string    `json:"schema"`
			Apps   []appJSON `json:"apps"`
		}{Schema: "tdx.v1.ticketAppList", Apps: out})
	}
	if len(apps) == 0 {
		_, _ = fmt.Fprintln(w, "no ticket apps found")
		return nil
	}
	headers := []string{"ID", "NAME", "DESCRIPTION", "ACTIVE"}
	rows := make([][]string, 0, len(apps))
	for _, a := range apps {
		active := "yes"
		if !a.Active { active = "no" }
		rows = append(rows, []string{strconv.Itoa(a.ID), a.Name, a.Description, active})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

func newAppUseCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "use <id>",
		Short: "Set the default ticket app for the current profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("app id must be a positive integer, got %q", args[0])
			}
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			return runAppUse(cmd.Context(), cmd.OutOrStdout(), auth, profile, id)
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runAppUse(ctx context.Context, w io.Writer, auth *authsvc.Service, profile string, appID int) error {
	if err := auth.SetTicketAppID(profile, appID); err != nil {
		return fmt.Errorf("save ticketAppID: %w", err)
	}
	_, _ = fmt.Fprintf(w, "ticket app set: profile %q → app %d\n", profile, appID)
	return nil
}

func newAppShowCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the current default ticket app for this profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			return runAppShow(cmd.Context(), cmd.OutOrStdout(), auth, profile)
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runAppShow(ctx context.Context, w io.Writer, auth *authsvc.Service, profile string) error {
	prof, err := auth.GetProfile(profile)
	if err != nil { return err }
	if prof.TicketAppID == 0 {
		_, _ = fmt.Fprintf(w, "profile %q has no ticket app set (run `tdx ticket app list` then `tdx ticket app use <id>`)\n", profile)
		return nil
	}
	_, _ = fmt.Fprintf(w, "profile %q ticket app: %d\n", profile, prof.TicketAppID)
	return nil
}
```

(`SetTicketAppID` and `GetProfile` may need to be added to `authsvc`. If absent, add them in this task — search `internal/svc/authsvc/` for existing `SetX`/`GetProfile` patterns and follow them.)

- [ ] **Step 2: app_test.go**

Test the `runAppList` runner with a stub service:

```go
func TestRunAppList(t *testing.T) {
	stub := &stubTicketsvc{
		apps: []domain.TicketApp{
			{ID: 31, Name: "Service Desk", Active: true},
			{ID: 71, Name: "Project Tickets", Active: true},
		},
	}
	var buf bytes.Buffer
	if err := runAppList(context.Background(), &buf, stub, "default", false); err != nil { t.Fatal(err) }
	out := buf.String()
	if !strings.Contains(out, "Service Desk") || !strings.Contains(out, "31") { t.Fatalf("missing rows: %s", out) }
}

func TestRunAppListJSON(t *testing.T) {
	stub := &stubTicketsvc{apps: []domain.TicketApp{{ID: 31, Name: "Service Desk", Active: true}}}
	var buf bytes.Buffer
	if err := runAppList(context.Background(), &buf, stub, "default", true); err != nil { t.Fatal(err) }
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil { t.Fatal(err) }
	if got["schema"] != "tdx.v1.ticketAppList" { t.Fatalf("schema: %v", got["schema"]) }
}
```

Define `stubTicketsvc` once in `helpers_test.go` (or a shared `testhelpers_test.go`). It implements every method on `ticketsvcAPI`; tests configure only the methods they use.

- [ ] **Step 3: Wire into ticket.go**

```go
func New() *cobra.Command {
	cmd := &cobra.Command{Use: "ticket", Short: "Manage TeamDynamix tickets"}
	cmd.AddCommand(newAppCmd(nil))
	return cmd
}
```

- [ ] **Step 4: Verify**

```bash
go build ./... && go test ./internal/cli/ticket/...
./tdx ticket app --help
./tdx ticket app list --help
```

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ticket/app.go internal/cli/ticket/app_test.go internal/cli/ticket/ticket.go internal/svc/authsvc/
git commit -m "feat(cli/ticket): app list / use / show subgroup"
```

---

## Task 10: CLI types list + statuses list

**Files:**
- Create: `internal/cli/ticket/types.go` + `types_test.go`
- Create: `internal/cli/ticket/statuses.go` + `statuses_test.go`
- Modify: `internal/cli/ticket/ticket.go`

- [ ] **Step 1: types.go**

`types list` reads `--app <id>` flag (optional), calls `svc.ListTypes`, renders table or JSON envelope `tdx.v1.ticketTypeList`.

```go
func newTypesCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{Use: "types", Short: "List ticket types in the current app"}
	cmd.AddCommand(newTypesListCmd(svc))
	return cmd
}

func newTypesListCmd(svc ticketsvcAPI) *cobra.Command {
	var appID int
	var jsonFlag bool
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ticket types",
		RunE: func(cmd *cobra.Command, args []string) error {
			// resolve paths/auth/profile, then call runTypesList
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTypesList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID int, jsonOut bool) error {
	types, err := svc.ListTypes(ctx, profile, appID)
	if err != nil { return err }
	if jsonOut {
		// envelope tdx.v1.ticketTypeList
	}
	// table: ID | NAME | DESCRIPTION | ACTIVE
}
```

- [ ] **Step 2: statuses.go**

Mirror `types.go`. Envelope: `tdx.v1.ticketStatusList`. Columns: `ID | NAME | IS-CLOSED | IS-DEFAULT | ORDER`.

- [ ] **Step 3: Tests**

For each: stub returns N rows; assert table contains the names; assert JSON envelope has the right schema.

- [ ] **Step 4: Wire into ticket.go**

```go
cmd.AddCommand(newTypesCmd(nil))
cmd.AddCommand(newStatusesCmd(nil))
```

- [ ] **Step 5: Verify + commit**

```bash
go test ./internal/cli/ticket/...
git add internal/cli/ticket/types.go internal/cli/ticket/types_test.go internal/cli/ticket/statuses.go internal/cli/ticket/statuses_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): types list and statuses list metadata commands"
```

---

## Task 11: CLI search + search saved

**Files:**
- Create: `internal/cli/ticket/search.go` + `search_test.go`
- Modify: `internal/cli/ticket/ticket.go`

- [ ] **Step 1: search.go**

```go
func newSearchCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		statusFlags    []string
		assigneeFlags  []string
		requestorFlags []string
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
			// resolve paths, auth, profile, authedUID
			// build TicketSearchFilter
			// if no assignees set, default to authedUID
			// resolve --status names → IDs via svc.ResolveStatusByName
			// call runTicketSearch
		},
	}
	cmd.Flags().StringSliceVar(&statusFlags, "status", nil, "status name or id (repeatable)")
	cmd.Flags().StringSliceVar(&assigneeFlags, "assignee", nil, "assignee me|UID|email (repeatable)")
	cmd.Flags().StringSliceVar(&requestorFlags, "requestor", nil, "requestor me|UID|email (repeatable)")
	cmd.Flags().StringVar(&accountFlag, "account", "", "account name (exact match)")
	cmd.Flags().StringVar(&textFlag, "text", "", "free-text search")
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results (capped at 1000)")
	cmd.Flags().BoolVar(&includeClosed, "include-closed", false, "include closed tickets")
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	cmd.AddCommand(newSearchSavedCmd(svc))
	return cmd
}

func runTicketSearch(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, filter domain.TicketSearchFilter, jsonOut bool) error {
	tickets, err := svc.SearchTickets(ctx, profile, filter)
	if err != nil { return err }
	return printTicketList(w, tickets, jsonOut, "tdx.v1.ticketList")
}

// printTicketList renders rows or JSON. Used by search and saved-search.
func printTicketList(w io.Writer, tickets []domain.Ticket, jsonOut bool, schema string) error {
	if jsonOut {
		// envelope
	}
	if len(tickets) == 0 {
		_, _ = fmt.Fprintln(w, "no tickets matched")
		return nil
	}
	headers := []string{"ID", "TITLE", "STATUS", "TYPE", "ASSIGNEE", "REQUESTOR", "MODIFIED"}
	rows := [][]string{}
	for _, t := range tickets {
		rows = append(rows, []string{
			strconv.Itoa(t.ID),
			truncate(t.Title, 60),
			t.StatusName,
			t.TypeName,
			t.ResponsibleName,
			t.RequestorName,
			t.ModifiedDate.Format("2006-01-02"),
		})
	}
	render.Table(w, headers, rows, nil)
	if banner := partialResultBanner(len(tickets)); banner != "" {
		_, _ = fmt.Fprintln(w, banner)
	}
	return nil
}

func newSearchSavedCmd(svc ticketsvcAPI) *cobra.Command {
	var appID int
	var jsonFlag bool
	var limitFlag int
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "saved [NAME]",
		Short: "List saved searches; with NAME, run that saved search",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runSavedSearchList(cmd.Context(), cmd.OutOrStdout(), svc, profileFlag, appID, jsonFlag)
			}
			return runSavedSearchRun(cmd.Context(), cmd.OutOrStdout(), svc, profileFlag, appID, args[0], limitFlag, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results when running")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}
```

- [ ] **Step 2: Tests**

- `TestRunTicketSearchDefaultsToMe` — when filter has no assignees, runner injects authed UID. (Need to test the cobra-level default-injection logic; consider extracting to a helper that's testable without cobra.)
- `TestPrintTicketListEmpty` — outputs "no tickets matched".
- `TestPrintTicketListWithRows` — output contains expected ticket IDs/titles.
- `TestPrintTicketListJSON` — envelope schema correct.
- `TestRunSavedSearchListEmpty` — empty list message.
- `TestRunSavedSearchByName` — stub resolves name → run → prints rows.
- `TestRunSavedSearchByNameAmbiguous` — stub returns ambiguous error.

- [ ] **Step 3: Verify + commit**

```bash
go test ./internal/cli/ticket/...
git add internal/cli/ticket/search.go internal/cli/ticket/search_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): search + search saved (list/run)"
```

---

## Task 12: CLI show (with this-week time crossover)

**Files:**
- Create: `internal/cli/ticket/show.go` + `show_test.go`
- Modify: `internal/cli/ticket/ticket.go`

The crossover: `runTicketShow` accepts a draftsvc dependency. If a current week draft exists, it scans entries with `Target.Type=Ticket && Target.ID==id` and aggregates hours.

- [ ] **Step 1: Add the draftsvc subset interface**

In `ticket.go` or `helpers.go`:

```go
type draftsvcAPI interface {
	// LoadCurrentWeekDraft returns the draft for the user's current week,
	// or (nil, nil) if no draft exists.
	LoadCurrentWeekDraft(ctx context.Context, profile string) (*domain.WeekDraft, error)
}
```

(Find the actual name of the existing draftsvc method and the WeekDraft type — `grep -rn "func.*LoadDraft\|func.*GetDraft\|WeekDraft" internal/svc/draftsvc/`.)

- [ ] **Step 2: show.go**

```go
func newShowCmd(svc ticketsvcAPI) *cobra.Command {
	var appID int
	var jsonFlag bool
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for one ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("ticket id must be int, got %q", args[0])
			}
			// resolve services
			return runTicketShow(cmd.Context(), cmd.OutOrStdout(), svc, drafts, profile, appID, id, jsonFlag)
		},
	}
	// flags
	return cmd
}

func runTicketShow(ctx context.Context, w io.Writer, svc ticketsvcAPI, drafts draftsvcAPI, profile string, appID, id int, jsonOut bool) error {
	t, err := svc.GetTicket(ctx, profile, appID, id)
	if err != nil { return err }
	weekHours := 0
	weekEntries := 0
	if drafts != nil {
		if d, derr := drafts.LoadCurrentWeekDraft(ctx, profile); derr == nil && d != nil {
			weekHours, weekEntries = sumThisWeekForTicket(d, id)
		}
	}
	if jsonOut {
		// envelope tdx.v1.ticket; include thisWeek: {hours, entries}
	}
	// pretty-print sections
	return printTicketShow(w, t, weekHours, weekEntries)
}

// sumThisWeekForTicket walks the draft's cells looking for entries against
// this ticket. Hours are accumulated across the whole week.
func sumThisWeekForTicket(d *domain.WeekDraft, ticketID int) (hours float64, entries int) {
	// iterate d.Rows, filter by Target.Type=="Ticket" && Target.ID==ticketID,
	// sum cell.Hours, count non-zero cells.
}
```

(Find the actual draft row iteration pattern in existing code: `grep -rn "Target.Type.*Ticket\|TargetTypeTicket" internal/`.)

- [ ] **Step 3: Tests**

- `TestRunTicketShowFull` — stub returns ticket; output contains title, status, assignee.
- `TestRunTicketShowWithThisWeek` — stub draft has 1.5h on this ticket; output line includes "this week: 1.5h (1 entry)".
- `TestRunTicketShowNoDraft` — drafts=nil; "this week" line is absent or "0h".
- `TestRunTicketShowJSON` — schema is `tdx.v1.ticket`; `thisWeek.hours` is 1.5.

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/cli/ticket/...
git add internal/cli/ticket/show.go internal/cli/ticket/show_test.go internal/cli/ticket/ticket.go internal/cli/ticket/helpers.go
git commit -m "feat(cli/ticket): show command with this-week time crossover"
```

---

## Task 13: CLI feed (read)

**Files:**
- Create: `internal/cli/ticket/feed.go` + `feed_test.go`
- Modify: `internal/cli/ticket/ticket.go`

- [ ] **Step 1: feed.go**

```go
func newFeedCmd(svc ticketsvcAPI) *cobra.Command {
	var appID int
	var limit int
	var jsonFlag bool
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "feed <id>",
		Short: "Read the feed for a ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil { return err }
			// resolve, call runTicketFeed
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().IntVar(&limit, "limit", 0, "max entries (0 = all)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketFeed(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, id, limit int, jsonOut bool) error {
	entries, err := svc.GetFeed(ctx, profile, appID, id)
	if err != nil { return err }
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	if jsonOut {
		// envelope tdx.v1.ticketFeed
	}
	// chronological print: [date] author — kind\n  body\n
}
```

- [ ] **Step 2: Tests**

- `TestRunTicketFeedRendersAllEntries`
- `TestRunTicketFeedRespectsLimit`
- `TestRunTicketFeedEmptyOK`
- `TestRunTicketFeedJSON`

- [ ] **Step 3: Verify + commit**

```bash
go test ./internal/cli/ticket/...
git add internal/cli/ticket/feed.go internal/cli/ticket/feed_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): feed read command"
```

---

## Task 14: CLI comment (mutation)

**Files:**
- Create: `internal/cli/ticket/comment.go` + `comment_test.go`
- Modify: `internal/cli/ticket/ticket.go`

- [ ] **Step 1: comment.go**

```go
func newCommentCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		isPrivate   bool
		notify      []string
		yesFlag     bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "comment <id> <message>",
		Short: "Post a feed comment to a ticket",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil { return err }
			if !yesFlag {
				return fmt.Errorf("pass --yes to post the comment (preview-mode not yet supported)")
			}
			// resolve services
			return runTicketComment(cmd.Context(), cmd.OutOrStdout(), svc, profile, appID, id, args[1], isPrivate, notify)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().BoolVar(&isPrivate, "private", false, "internal note (not visible to requestor)")
	cmd.Flags().StringSliceVar(&notify, "notify", nil, "additional notify recipients (UID, repeatable)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to post")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketComment(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, id int, body string, isPrivate bool, notify []string) error {
	feedID, err := svc.AddFeed(ctx, profile, appID, id, body, isPrivate, notify)
	if err != nil { return err }
	visibility := "public"
	if isPrivate { visibility = "private" }
	_, _ = fmt.Fprintf(w, "comment posted to ticket #%d (feed entry %d, %s)\n", id, feedID, visibility)
	return nil
}
```

- [ ] **Step 2: Tests**

- `TestRunTicketCommentSuccess` — happy path; output mentions ticket id and feed entry id.
- `TestRunTicketCommentPrivate` — isPrivate=true; output says "private".
- `TestNewCommentCmdRequiresYes` — running cobra cmd without --yes errors.

- [ ] **Step 3: Verify + commit**

```bash
go test ./internal/cli/ticket/...
git add internal/cli/ticket/comment.go internal/cli/ticket/comment_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): comment command (mutating, --yes required)"
```

---

## Task 15: CLI status + assign

**Files:**
- Create: `internal/cli/ticket/status.go` + `status_test.go`
- Create: `internal/cli/ticket/assign.go` + `assign_test.go`
- Modify: `internal/cli/ticket/ticket.go`

Both follow the same shape: PATCH op + optional --comment follow-up.

- [ ] **Step 1: status.go**

```go
func newStatusCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		statusID    int
		yesFlag     bool
		commentFlag string
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "status <id> <name-or-id>",
		Short: "Change a ticket's status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil { return err }
			if !yesFlag {
				return fmt.Errorf("pass --yes to change status")
			}
			parsedID, name := parseStatusArg(args[1])
			if statusID > 0 { parsedID = statusID }
			// resolve services
			return runTicketStatus(cmd.Context(), cmd.OutOrStdout(), svc, profile, appID, id, parsedID, name, commentFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().IntVar(&statusID, "status-id", 0, "status id (overrides positional name)")
	cmd.Flags().StringVar(&commentFlag, "comment", "", "optional accompanying feed comment")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketStatus(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, id, statusID int, statusName, comment string) error {
	if statusID == 0 {
		st, err := svc.ResolveStatusByName(ctx, profile, appID, statusName)
		if err != nil { return err }
		statusID = st.ID
	}
	ops := []ticketsvc.PatchOp{{Op: "replace", Path: "/StatusID", Value: statusID}}
	updated, err := svc.PatchTicket(ctx, profile, appID, id, ops)
	if err != nil { return err }
	_, _ = fmt.Fprintf(w, "ticket #%d status → %s\n", id, updated.StatusName)
	if comment != "" {
		if _, err := svc.AddFeed(ctx, profile, appID, id, comment, false, nil); err != nil {
			_, _ = fmt.Fprintf(w, "warning: status changed but comment failed: %v\n", err)
		}
	}
	return nil
}
```

- [ ] **Step 2: assign.go** — same pattern, but resolves principal via `resolvePrincipal` and patches `/ResponsibleUid`.

- [ ] **Step 3: Tests**

For each:
- happy-path runner test (stub patch returns updated ticket; output shows new state)
- cobra cmd requires `--yes`
- status: name resolved via stub `ResolveStatusByName`
- assign: `me` resolves to authed UID

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/cli/ticket/...
git add internal/cli/ticket/status.go internal/cli/ticket/status_test.go internal/cli/ticket/assign.go internal/cli/ticket/assign_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): status and assign commands (mutating, --yes required)"
```

---

## Task 16: CLI log (time crossover)

**Files:**
- Create: `internal/cli/ticket/log.go` + `log_test.go`
- Modify: `internal/cli/ticket/ticket.go`

`tdx ticket log` is a thin wrapper over the existing `timesvc.AddEntry` code path. Read `internal/cli/time/entry/add.go` first to understand the existing flow.

- [ ] **Step 1: log.go**

```go
func newLogCmd(svc ticketsvcAPI) *cobra.Command {
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
		Use:   "log <id>",
		Short: "Log time worked against a ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil { return err }
			if !yesFlag { return fmt.Errorf("pass --yes to log time") }
			// validation: exactly one of --hours / --minutes
			// validation: exactly one of --type / --type-id
			// build domain.EntryInput with Target = {Type: Ticket, ID: id}
			// shell to timesvc.AddEntry (use the same code path tdx time entry add uses)
			return runTicketLog(cmd.Context(), cmd.OutOrStdout(), timeSvc, profile, appID, id, /* ... */)
		},
	}
	// flags
	return cmd
}

func runTicketLog(ctx context.Context, w io.Writer, timeSvc ticketLogTimesvcAPI, profile string, appID, ticketID int, hours float64, minutes int, typeName string, typeID int, dateStr, desc string, billable bool) error {
	// construct EntryInput; call timeSvc.AddEntry; print:
	//   logged 1h30m to ticket #12345 (entry id 98765, type "Development")
}

// ticketLogTimesvcAPI is the subset of timesvc.Service used here.
type ticketLogTimesvcAPI interface {
	AddEntry(ctx context.Context, profile string, in domain.EntryInput) (domain.Entry, error)
	TimeTypesForTarget(ctx context.Context, profile string, target domain.Target) ([]domain.TimeType, error)
}
```

(Find the actual `EntryInput` shape and `AddEntry`/`TimeTypesForTarget` signatures via `grep -rn "func.*AddEntry\|EntryInput struct" internal/`.)

- [ ] **Step 2: Tests**

- `TestRunTicketLogHours` — `--hours 1.5` produces an EntryInput with 90 minutes.
- `TestRunTicketLogMinutes` — `--minutes 90` produces same.
- `TestRunTicketLogResolvesTypeByName` — type name resolved via `TimeTypesForTarget`.
- `TestNewLogCmdRequiresYes` — without `--yes`, errors.
- `TestRunTicketLogHoursAndMinutesError` — both passed → error.

- [ ] **Step 3: Verify + commit**

```bash
go test ./internal/cli/ticket/...
git add internal/cli/ticket/log.go internal/cli/ticket/log_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): log command (time crossover)"
```

---

## Task 17: MCP tools

**Files:**
- Create: `internal/mcp/tools_ticket.go`
- Create: `internal/mcp/tools_ticket_test.go`
- Modify: `internal/mcp/server.go` — register `RegisterTicketTools(srv, svcs)`

Read `internal/mcp/tools_people.go` and `tools_people_test.go` first — they're the closest pattern (fewest dependencies).

- [ ] **Step 1: tools_ticket.go**

```go
package mcp

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// RegisterTicketTools registers all 12 ticket MCP tools on srv.
func RegisterTicketTools(srv *server, svcs *Services) {
	srv.AddTool("list_ticket_apps", "List ticket apps in the tenant", listTicketAppsHandler(svcs))
	srv.AddTool("list_ticket_types", "List ticket types in an app", listTicketTypesHandler(svcs))
	srv.AddTool("list_ticket_statuses", "List ticket statuses in an app", listTicketStatusesHandler(svcs))
	srv.AddTool("list_saved_searches", "List saved searches", listSavedSearchesHandler(svcs))
	srv.AddTool("search_tickets", "Search tickets with filters", searchTicketsHandler(svcs))
	srv.AddTool("run_saved_search", "Execute a saved search", runSavedSearchHandler(svcs))
	srv.AddTool("get_ticket", "Get full detail for one ticket", getTicketHandler(svcs))
	srv.AddTool("get_ticket_feed", "Read feed entries for a ticket", getTicketFeedHandler(svcs))
	// mutating
	srv.AddTool("add_ticket_comment", "Post a feed comment (requires confirm:true)", addTicketCommentHandler(svcs))
	srv.AddTool("update_ticket_status", "Change a ticket's status (requires confirm:true)", updateTicketStatusHandler(svcs))
	srv.AddTool("update_ticket_assignee", "Reassign a ticket (requires confirm:true)", updateTicketAssigneeHandler(svcs))
	srv.AddTool("log_ticket_time", "Log time against a ticket (requires confirm:true)", logTicketTimeHandler(svcs))
}
```

(The actual `srv.AddTool` signature and `Services` shape live in existing code. Match them.)

For each handler: parse args (id, appId optional, plus tool-specific fields); call the right `svcs.Tickets.X` method; return output as JSON with the matching `tdx.v1.*` schema. Mutating handlers check `args["confirm"] == true` first; without it return `errors.New("confirm: true required for mutating tool")`.

- [ ] **Step 2: Tests**

For each tool: one happy path + one missing-confirm test (mutating only). Mirror `tools_people_test.go` style.

- [ ] **Step 3: Wire into server.go**

```go
RegisterTicketTools(srv, svcs)
```

Also add `Tickets *ticketsvc.Service` to `Services` struct.

- [ ] **Step 4: Verify**

```bash
go test ./internal/mcp/...
./tdx mcp serve <<< 'method_test'  # smoke
```

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/
git commit -m "feat(mcp): add 12 ticket tools (8 read + 4 mutating)"
```

---

## Task 18: Documentation

**Files:**
- Create: `docs/guide/ticket.md`
- Modify: `docs/guide.md` (ASCII tree + reference list)
- Modify: `README.md` (ASCII tree)
- Modify: `docs/guide/mcp.md` (Phase D ticket tool tables)

- [ ] **Step 1: docs/guide/ticket.md**

Use the heading scheme established in the docs restructure (command-as-heading). Structure:

```markdown
# tdx ticket

Manage TeamDynamix tickets: search, view, comment, change status/assignee, run saved searches, and log time worked.

## Contents

- [tdx ticket app](#tdx-ticket-app)
- [tdx ticket search](#tdx-ticket-search)
- [tdx ticket show](#tdx-ticket-show)
- [tdx ticket feed](#tdx-ticket-feed)
- [tdx ticket comment](#tdx-ticket-comment)
- [tdx ticket status](#tdx-ticket-status)
- [tdx ticket assign](#tdx-ticket-assign)
- [tdx ticket log](#tdx-ticket-log)
- [tdx ticket types](#tdx-ticket-types)
- [tdx ticket statuses](#tdx-ticket-statuses)

---

## tdx ticket app
[overview: appId concept, why per-profile default]

### tdx ticket app list
### tdx ticket app use
### tdx ticket app show

## tdx ticket search
[default behavior, all flags, examples]

### tdx ticket search saved

#### tdx ticket search saved (no args — list saved searches)
#### tdx ticket search saved <name> (run a saved search)

## tdx ticket show
[full detail layout, this-week time crossover explained]

## tdx ticket feed
## tdx ticket comment
## tdx ticket status
## tdx ticket assign
## tdx ticket log
[time crossover explained: thin wrapper over tdx time entry add]

## tdx ticket types
### tdx ticket types list

## tdx ticket statuses
### tdx ticket statuses list
```

For each command, document: usage, flags (with defaults), examples, JSON envelope name.

- [ ] **Step 2: Update `docs/guide.md` ASCII tree**

Find the `## Command tree` section (around line 20). The ticket branch slots in alphabetically between `people` and `time`:

```text
├── ticket
│   ├── app              → list / use / show
│   ├── search           → saved
│   ├── show / feed
│   ├── comment / status / assign / log
│   └── types / statuses → list
```

Also add to the Reference list:
```markdown
- [tdx ticket](guide/ticket.md) — search, show, comment, change status/assignee, log time
```

- [ ] **Step 3: Update `README.md` ASCII tree**

Same change to the README's tree (must stay byte-identical to `docs/guide.md`'s tree per the docs restructure decision).

- [ ] **Step 4: Update `docs/guide/mcp.md`**

After the existing People (read-only) section, add:

```markdown
#### Tickets (Phase D — read-only, 8 tools)

| Tool | Description |
|------|-------------|
| `list_ticket_apps` | List ticket apps in the tenant |
| `list_ticket_types` | List ticket types in an app |
| `list_ticket_statuses` | List ticket statuses in an app |
| `list_saved_searches` | List saved searches in an app |
| `search_tickets` | Search tickets by status/assignee/requestor/text |
| `run_saved_search` | Execute a saved search by ID |
| `get_ticket` | Get full detail for one ticket |
| `get_ticket_feed` | Read feed entries for a ticket |

#### Tickets (Phase D — mutating, 4 tools, all require `confirm: true`)

| Tool | Description |
|------|-------------|
| `add_ticket_comment` | Post a feed comment to a ticket |
| `update_ticket_status` | Change a ticket's status |
| `update_ticket_assignee` | Reassign a ticket |
| `log_ticket_time` | Log time against a ticket (creates a time entry) |
```

Update the "44 tools" intro paragraph to "56 tools (30 read, 26 mutating)" or similar, recalculated.

- [ ] **Step 5: Verify ASCII tree byte-identical**

```bash
diff <(awk '/^```text$/,/^```$/' docs/guide.md) <(awk '/^```text$/,/^```$/' README.md)
```

Expected: empty.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/ticket.md docs/guide.md docs/guide/mcp.md README.md
git commit -m "docs: add tdx ticket reference + tree updates + MCP tool tables"
```

---

## Task 19: Live verification + version bump + PR + release

**Files:**
- Modify: version source (find via `grep -rn "0.15.1\|v0.15.1" internal/cli/`)

### Live verification

These steps require an authenticated session against Sample (or another tenant the user has). Before committing fixtures or code changes from live data, sanitize anything sensitive.

- [ ] **Step 1: Pre-flight**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

All green required.

- [ ] **Step 2: Live-probe each command**

For each, capture output (and capture wire-level response shapes if a wire-format mismatch is suspected):

```bash
./tdx ticket app list                                          # returns ≥1 app
./tdx ticket app use <id>                                      # persists; tdx auth status reflects?
./tdx ticket app show                                          # echoes the id
./tdx ticket types list                                        # ≥1 type
./tdx ticket statuses list                                     # ≥1 status
./tdx ticket search                                            # default = my open
./tdx ticket search --status "In Progress" --limit 5
./tdx ticket search saved                                      # ≥1 saved search
./tdx ticket search saved "<some saved-search name>" --limit 5
./tdx ticket show <real-id>                                    # full detail; check this-week time line if you have a draft
./tdx ticket feed <real-id>                                    # entries print
./tdx ticket comment <test-ticket-id> "test from tdx" --yes
./tdx ticket status <test-ticket-id> "<some status>" --yes
./tdx ticket assign <test-ticket-id> me --yes
./tdx ticket log <test-ticket-id> --minutes 1 --type "<some type>" --yes
```

Use a low-stakes test ticket for the four mutations. After verification, roll back: re-comment "test undone", restore status, restore assignee, delete the 1-minute time entry.

- [ ] **Step 3: Fix wire-format mismatches**

If any command fails because of a wire-format issue (TD field names different from what the spec assumed), fix in `internal/svc/ticketsvc/types.go` and the relevant decoder. Add a test fixture from the captured live response. Commit:

```bash
git commit -m "fix(ticketsvc): correct wire format for X (live Sample probe)"
```

### Version bump + PR

- [ ] **Step 4: Bump version**

```bash
grep -rn "v0.15.1\|0.15.1" internal/cli/version*.go
```

Update the version string to `v0.16.0`. Commit:

```bash
git add internal/cli/version.go  # or wherever
git commit -m "chore: bump version to v0.16.0"
```

- [ ] **Step 5: Push branch and open PR**

```bash
git push -u origin ticket-mvp
gh pr create --title "v0.16.0: tdx ticket MVP (Phase D.1)" --body "$(cat <<'EOF'
## Summary

Adds first-class TeamDynamix ticket support under `tdx ticket ...`. 11 commands across 4 sub-groups; 12 new MCP tools. Includes the `tdx ticket log` time-crossover (logs a time entry against a ticket with one command).

### Highlights
- `tdx ticket app list` / `use` / `show` — discover and persist a default ticket app per profile (new `ticketAppID` config field)
- `tdx ticket search` — defaults to "my open"; flags `--status` / `--assignee` / `--requestor` / `--account` / `--text` / `--include-closed`
- `tdx ticket search saved [NAME]` — list / run saved searches (rate-limited 60/min/IP)
- `tdx ticket show <id>` — full detail + this-week time logged from local week drafts (crossover #1)
- `tdx ticket feed <id>`, `comment <id>`, `status <id>`, `assign <id>` — read + 3 light mutations
- `tdx ticket log <id>` — log time against a ticket (crossover #2, wraps `tdx time entry add`)
- `tdx ticket types list` / `statuses list` — metadata discovery

### MCP
44 → 56 tools. 8 new read-only + 4 new mutating (all require `confirm: true`).

### Out of scope (deferred)
- `create`, generic `update`, tasks (`tdx ticket task ...`), workflow approvals, attachments/tags/contacts/assets

Spec: `docs/specs/2026-05-08-tdx-ticket-mvp.md`
Plan: `docs/plans/2026-05-08-tdx-ticket-mvp.md`

## Test plan

- [x] All 11 commands wired and discoverable via `tdx ticket --help`.
- [x] `tdx ticket search` (no flags) returns my open tickets after `tdx ticket app use`.
- [x] `tdx ticket show <id>` displays TD's ActualMinutes plus locally-computed this-week hours.
- [x] All 4 mutations require `--yes`; without it, fail-fast with a helpful error.
- [x] `tdx ticket log` creates a time entry visible in `tdx time entry list` and TD's UI.
- [x] 12 MCP tools registered; mutating ones require `confirm: true`.
- [x] `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` all green.
- [ ] Visual render check on GitHub PR view.
EOF
)"
```

- [ ] **Step 6: Wait for CI; merge with admin**

```bash
gh pr merge <PR#> --squash --admin --delete-branch
```

- [ ] **Step 7: Reset local main + tag**

```bash
git checkout main
git fetch origin
git reset --hard origin/main
git tag v0.16.0
git push origin v0.16.0
```

Goreleaser will produce the release artifacts. Confirm at the GitHub Releases page.

- [ ] **Step 8: Update memory**

Edit `/Users/ipm/.claude/projects/-Users-ipm-code-tdx/memory/MEMORY.md` index line for "tdx current state" to point at v0.16.0. Edit `project_tdx_current_state.md` and add a new "Latest release" block for v0.16.0.

---

## Self-Review (run by plan author after writing)

**1. Spec coverage:**
- 11 CLI commands → Tasks 8-16 (each command implemented)
- 12 MCP tools → Task 17
- Profile config (`ticketAppID`) → Task 2
- Domain types → Task 1
- Service layer (apps/metadata/tickets/feed/saved-searches) → Tasks 3-7
- Docs (`guide/ticket.md`, tree updates, mcp.md) → Task 18
- Live verification → Task 19
- Tests at every layer → embedded in each task
- Crossover #1 (`tdx ticket show` this-week time) → Task 12
- Crossover #2 (`tdx ticket log`) → Task 16
- Version bump v0.16.0 → Task 19

All spec requirements have a task. Acceptance criteria 1-10 in the spec all map to verification steps in Task 19.

**2. Placeholder scan:**
- Some Step blocks reference "find the existing X via grep" — these are concrete instructions to discover existing code, not placeholders.
- A handful of code stubs use `// resolve services` shorthand where the resolution boilerplate is the same as Task 9 step 1. Implementer is expected to copy the boilerplate; this is acceptable shorthand because the pattern is explicit elsewhere.
- No "TBD", "TODO", "implement later" detected.

**3. Type consistency:**
- `ticketsvcAPI` interface: defined in Task 8 step 1, used consistently throughout Tasks 9-16.
- `ticketsvc.PatchOp`: exported in Task 5, imported by Task 8's `ticketsvcAPI` interface and used in Task 15. Consistent.
- `domain.Ticket.IsFull` set true on `GetTicket`, false on `SearchTickets`/`RunSavedSearch`. Consistent.
- `ticketAppID` field name on `Profile` matches throughout (yaml/json tags `ticketAppID,omitempty`).
- `parseTDTime` defined in Task 5 (tickets.go), used in Task 6 (feed.go). Consistent.
- `decodeTicket(w, full bool)` second arg consistently used: `true` for `GetTicket`, `false` for search/saved-search.

All consistent after the inline `PatchOp` export fix above.
