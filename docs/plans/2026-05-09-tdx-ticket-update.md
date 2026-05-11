# tdx Ticket Update Implementation Plan (v0.16.3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `tdx ticket update <id>` for generic ticket-field updates (title, description, type, account, requestor, responsibility-group, priority-id) plus an optional accompanying comment. Thin PATCH wrapper. One new MCP tool. Tool count 62 → 63. Ship as v0.16.3.

**Architecture:** Bottom-up — extend `peoplesvcAPI` interface with `ResolveAccountByName` → CLI helper for resolving raw flag args to typed-pointer field set → CLI helper for building `[]ticketsvc.PatchOp` from the field set → cobra command wiring → MCP tool → docs. Reuses everything from v0.16.0/0.16.1: existing `PatchTicket` service method, existing name resolvers (`ResolveTypeByName`, `ResolveGroupByName`, `ResolveAccountByName`, `resolvePrincipal`).

**Tech Stack:** Go 1.26.2; cobra; existing `tdx.Client`; `httptest` for service tests (none needed this round — service layer unchanged).

**Spec:** [`docs/specs/2026-05-09-tdx-ticket-update.md`](../specs/2026-05-09-tdx-ticket-update.md)

---

## File Structure

After this plan completes:

```
internal/
├── cli/
│   └── ticket/
│       ├── helpers.go             # MODIFY: peoplesvcAPI gains ResolveAccountByName
│       ├── helpers_test.go        # MODIFY: stubPeoplesvc gains ResolveAccountByName
│       ├── update.go              # NEW: cobra cmd + runner + helpers (~200 lines)
│       ├── update_test.go         # NEW: ~12 tests
│       └── ticket.go              # MODIFY: register newUpdateCmd
└── mcp/
    ├── tools_ticket_mutating.go   # MODIFY: add update_ticket tool + handler
    ├── tools_ticket_test.go       # MODIFY: smoke test
    └── server_test.go             # MODIFY: tool count 62 → 63
docs/
├── guide/
│   ├── ticket.md                  # MODIFY: ## tdx ticket update section
│   └── mcp.md                     # MODIFY: tool table + counts
└── guide.md                       # MODIFY: ASCII tree
README.md                          # MODIFY: ASCII tree (byte-identical)
```

## Established Patterns

Read these BEFORE starting:
- `internal/cli/ticket/status.go` and `assign.go` — closest siblings (both are PATCH-based mutators with --yes + optional --comment)
- `internal/cli/ticket/helpers.go` — `peoplesvcAPI` interface
- `internal/svc/peoplesvc/accounts.go` — `ResolveAccountByName` returns `peoplesvc.Account` (NOT `domain.Account`)
- Memory note `feedback_no_coauthor.md` — no Co-Authored-By trailer in commits

## Branch + Versioning

- Branch: `ticket-update` (Task 0)
- Version: v0.16.3 (no source change; tagged after merge)

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
git checkout -b ticket-update
```

---

## Task 1: Extend `peoplesvcAPI` with `ResolveAccountByName`

**Files:**
- Modify: `internal/cli/ticket/helpers.go`
- Modify: `internal/cli/ticket/helpers_test.go`

`ResolveAccountByName` exists on `peoplesvc.Service` (returns `peoplesvc.Account`). We widen the CLI's `peoplesvcAPI` interface to include it so `tdx ticket update --account` can use it.

- [ ] **Step 1: Extend the interface**

In `internal/cli/ticket/helpers.go`, find `peoplesvcAPI` and add a new method. Note: it returns `peoplesvc.Account` (the service's type), so we need to import `peoplesvc`.

```go
import (
	// ... existing imports ...
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

type peoplesvcAPI interface {
	LookupPeople(ctx context.Context, profile string, q string, limit int) ([]domain.User, error)
	SearchUsers(ctx context.Context, profile string, filter domain.UserFilter) ([]domain.User, error)
	ResolveAccountByName(ctx context.Context, profile, name string) (peoplesvc.Account, error) // NEW
}
```

(If `peoplesvc` is already imported, just add the method line.)

- [ ] **Step 2: Extend the test stub**

In `internal/cli/ticket/helpers_test.go`, find `stubPeoplesvc`. Add fields + method:

```go
type stubPeoplesvc struct {
	users        []domain.User
	err          error
	searchUsers  []domain.User
	searchErr    error
	lastFilter   domain.UserFilter
	resolvedAccount peoplesvc.Account // NEW
	accountErr      error             // NEW
}

// NEW:
func (s *stubPeoplesvc) ResolveAccountByName(_ context.Context, _, _ string) (peoplesvc.Account, error) {
	return s.resolvedAccount, s.accountErr
}
```

(Add `peoplesvc` import to the test file if not already there.)

- [ ] **Step 3: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
golangci-lint run ./internal/cli/ticket/...
```

Expected: clean. `peoplesvc.Service` already implements `ResolveAccountByName`, so the production wiring is already correct.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ticket/helpers.go internal/cli/ticket/helpers_test.go
git commit -m "feat(cli/ticket): widen peoplesvcAPI with ResolveAccountByName"
```

**No `Co-Authored-By:` trailer.**

---

## Task 2: CLI — `update.go` core types + helpers

**Files:**
- Create: `internal/cli/ticket/update.go` (helpers only this task; cobra wiring next)
- Create: `internal/cli/ticket/update_test.go`

This task creates the pure-helper foundation. No cobra command yet — that comes in Task 3.

- [ ] **Step 1: Define field-set types and the patch builder**

Create `internal/cli/ticket/update.go`:

```go
package ticket

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

// rawUpdateFlags captures the raw cobra flag values plus a boolean for
// each field indicating whether the flag was explicitly set
// (cmd.Flags().Changed(name)). The "set" booleans are needed because Title
// and Description accept empty strings as valid values; we can't use empty
// to mean "not provided".
type rawUpdateFlags struct {
	title          string
	titleSet       bool
	description    string
	descriptionSet bool
	typeArg        string // numeric or name; empty = not set
	accountArg     string
	requestorArg   string
	groupArg       string
	priorityID     int // 0 = not set
	prioritySet    bool
	comment        string
}

// ticketUpdateFields holds the resolved field values to PATCH. Pointers
// distinguish "set to value X" (including empty strings) from "don't touch".
type ticketUpdateFields struct {
	title           *string
	description     *string
	typeID          *int
	accountID       *int
	requestorUID    *string
	groupID         *int
	priorityID      *int
}

// hasAny reports whether at least one field is set.
func (f ticketUpdateFields) hasAny() bool {
	return f.title != nil || f.description != nil || f.typeID != nil ||
		f.accountID != nil || f.requestorUID != nil || f.groupID != nil ||
		f.priorityID != nil
}

// buildTicketPatchOps emits a JSON-Patch op per non-nil field. Order is
// stable for testability: Title, Description, TypeID, AccountID,
// RequestorUid, ResponsibleGroupID, PriorityID.
func buildTicketPatchOps(f ticketUpdateFields) []ticketsvc.PatchOp {
	var ops []ticketsvc.PatchOp
	if f.title != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/Title", Value: *f.title})
	}
	if f.description != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/Description", Value: *f.description})
	}
	if f.typeID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/TypeID", Value: *f.typeID})
	}
	if f.accountID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/AccountID", Value: *f.accountID})
	}
	if f.requestorUID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/RequestorUid", Value: *f.requestorUID})
	}
	if f.groupID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/ResponsibleGroupID", Value: *f.groupID})
	}
	if f.priorityID != nil {
		ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/PriorityID", Value: *f.priorityID})
	}
	return ops
}

// resolveUpdateFields resolves rawUpdateFlags to ticketUpdateFields,
// using the appropriate name → ID resolver per field. Pure-ish (depends
// on stub-able interfaces only).
func resolveUpdateFields(ctx context.Context, svc ticketsvcAPI, people peoplesvcAPI, profile, authedUID string, appID int, raw rawUpdateFlags) (ticketUpdateFields, error) {
	var out ticketUpdateFields

	if raw.titleSet {
		v := raw.title
		out.title = &v
	}
	if raw.descriptionSet {
		v := raw.description
		out.description = &v
	}
	if raw.typeArg != "" {
		id, name := parseStatusArg(raw.typeArg) // reuse: numeric-or-name parser
		if id > 0 {
			v := id
			out.typeID = &v
		} else {
			tt, err := svc.ResolveTypeByName(ctx, profile, appID, name)
			if err != nil {
				return out, fmt.Errorf("--type %q: %w", raw.typeArg, err)
			}
			v := tt.ID
			out.typeID = &v
		}
	}
	if raw.accountArg != "" {
		id, name := parseStatusArg(raw.accountArg)
		if id > 0 {
			v := id
			out.accountID = &v
		} else {
			acct, err := people.ResolveAccountByName(ctx, profile, name)
			if err != nil {
				return out, fmt.Errorf("--account %q: %w", raw.accountArg, err)
			}
			v := acct.ID
			out.accountID = &v
		}
	}
	if raw.requestorArg != "" {
		uid, err := resolvePrincipal(ctx, people, profile, authedUID, raw.requestorArg)
		if err != nil {
			return out, fmt.Errorf("--requestor %q: %w", raw.requestorArg, err)
		}
		out.requestorUID = &uid
	}
	if raw.groupArg != "" {
		id, name := parseStatusArg(raw.groupArg)
		if id > 0 {
			v := id
			out.groupID = &v
		} else {
			g, err := svc.ResolveGroupByName(ctx, profile, name)
			if err != nil {
				return out, fmt.Errorf("--responsibility-group %q: %w", raw.groupArg, err)
			}
			v := g.ID
			out.groupID = &v
		}
	}
	if raw.prioritySet {
		v := raw.priorityID
		out.priorityID = &v
	}

	return out, nil
}

// changedFieldsSummary builds the human-readable summary of which fields
// changed. Used by runTicketUpdate. For long values like description, we
// just print "<changed>" rather than echoing the full text.
func changedFieldsSummary(f ticketUpdateFields, ticketAfterPatch domain.Ticket) string {
	parts := []string{}
	if f.title != nil {
		parts = append(parts, fmt.Sprintf("title=%q", truncate(*f.title, 60)))
	}
	if f.description != nil {
		parts = append(parts, "description=<changed>")
	}
	if f.typeID != nil {
		name := ticketAfterPatch.TypeName
		if name == "" {
			parts = append(parts, fmt.Sprintf("type-id=%d", *f.typeID))
		} else {
			parts = append(parts, fmt.Sprintf("type=%s", name))
		}
	}
	if f.accountID != nil {
		name := ticketAfterPatch.AccountName
		if name == "" {
			parts = append(parts, fmt.Sprintf("account-id=%d", *f.accountID))
		} else {
			parts = append(parts, fmt.Sprintf("account=%s", name))
		}
	}
	if f.requestorUID != nil {
		name := ticketAfterPatch.RequestorName
		if name == "" {
			parts = append(parts, "requestor=<changed>")
		} else {
			parts = append(parts, fmt.Sprintf("requestor=%s", name))
		}
	}
	if f.groupID != nil {
		parts = append(parts, fmt.Sprintf("responsibility-group-id=%d", *f.groupID))
	}
	if f.priorityID != nil {
		name := ticketAfterPatch.PriorityName
		if name == "" {
			parts = append(parts, fmt.Sprintf("priority-id=%d", *f.priorityID))
		} else {
			parts = append(parts, fmt.Sprintf("priority=%s", name))
		}
	}
	return strings.Join(parts, ", ")
}
```

(`truncate` and `parseStatusArg` are existing helpers in the package — no need to redefine.)

- [ ] **Step 2: Tests**

Create `internal/cli/ticket/update_test.go`:

```go
package ticket

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

func TestBuildTicketPatchOpsAllFields(t *testing.T) {
	title := "T"
	desc := "D"
	tid := 5
	aid := 10
	uid := "uid-x"
	gid := 100
	pid := 3
	ops := buildTicketPatchOps(ticketUpdateFields{
		title: &title, description: &desc,
		typeID: &tid, accountID: &aid,
		requestorUID: &uid, groupID: &gid, priorityID: &pid,
	})
	if len(ops) != 7 {
		t.Fatalf("want 7 ops, got %d: %+v", len(ops), ops)
	}
	wantPaths := []string{"/Title", "/Description", "/TypeID", "/AccountID", "/RequestorUid", "/ResponsibleGroupID", "/PriorityID"}
	for i, op := range ops {
		if op.Op != "replace" {
			t.Errorf("ops[%d].Op = %q, want replace", i, op.Op)
		}
		if op.Path != wantPaths[i] {
			t.Errorf("ops[%d].Path = %q, want %q", i, op.Path, wantPaths[i])
		}
	}
}

func TestBuildTicketPatchOpsNoFields(t *testing.T) {
	ops := buildTicketPatchOps(ticketUpdateFields{})
	if len(ops) != 0 {
		t.Fatalf("want 0 ops, got %d", len(ops))
	}
}

func TestBuildTicketPatchOpsEmptyDescriptionStillEmits(t *testing.T) {
	empty := ""
	ops := buildTicketPatchOps(ticketUpdateFields{description: &empty})
	if len(ops) != 1 {
		t.Fatalf("want 1 op, got %d", len(ops))
	}
	if ops[0].Path != "/Description" {
		t.Errorf("path: %q", ops[0].Path)
	}
	if v, _ := ops[0].Value.(string); v != "" {
		t.Errorf("value should be empty string, got %v", ops[0].Value)
	}
}

func TestTicketUpdateFieldsHasAny(t *testing.T) {
	if (ticketUpdateFields{}).hasAny() {
		t.Error("empty fields should not hasAny()")
	}
	title := "T"
	if !(ticketUpdateFields{title: &title}).hasAny() {
		t.Error("title-only should hasAny()")
	}
}

func TestResolveUpdateFieldsTitleAndDescription(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		title: "Hello", titleSet: true, description: "", descriptionSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.title == nil || *got.title != "Hello" {
		t.Errorf("title: %v", got.title)
	}
	if got.description == nil || *got.description != "" {
		t.Errorf("empty description should be set, got %v", got.description)
	}
}

func TestResolveUpdateFieldsTypeByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedType: domain.TicketType{ID: 7, Name: "Incident"}}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		typeArg: "Incident",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.typeID == nil || *got.typeID != 7 {
		t.Errorf("typeID: %v", got.typeID)
	}
}

func TestResolveUpdateFieldsTypeByNumericArg(t *testing.T) {
	stub := &stubTicketsvc{} // no resolvedType — verifies we don't call the resolver
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		typeArg: "7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.typeID == nil || *got.typeID != 7 {
		t.Errorf("typeID: %v", got.typeID)
	}
}

func TestResolveUpdateFieldsAccountByName(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{resolvedAccount: peoplesvc.Account{ID: 1566, Name: "Test Acct"}}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		accountArg: "Test Acct",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.accountID == nil || *got.accountID != 1566 {
		t.Errorf("accountID: %v", got.accountID)
	}
}

func TestResolveUpdateFieldsRequestorMe(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		requestorArg: "me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.requestorUID == nil || *got.requestorUID != "uid-me" {
		t.Errorf("requestorUID: %v", got.requestorUID)
	}
}

func TestResolveUpdateFieldsGroupByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedGroup: domain.TicketGroup{ID: 100, Name: "Linux Team"}}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		groupArg: "Linux Team",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.groupID == nil || *got.groupID != 100 {
		t.Errorf("groupID: %v", got.groupID)
	}
}

func TestResolveUpdateFieldsPriority(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		priorityID: 3, prioritySet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.priorityID == nil || *got.priorityID != 3 {
		t.Errorf("priorityID: %v", got.priorityID)
	}
}

func TestResolveUpdateFieldsTypeResolverErrorPropagates(t *testing.T) {
	stub := &stubTicketsvc{err: errors.New("boom")}
	people := &stubPeoplesvc{}
	_, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		typeArg: "Nonsense",
	})
	if err == nil || !strings.Contains(err.Error(), "--type") {
		t.Fatalf("want propagated --type error, got %v", err)
	}
}

func TestChangedFieldsSummaryUsesNamesFromUpdatedTicket(t *testing.T) {
	tid := 7
	pid := 3
	got := changedFieldsSummary(ticketUpdateFields{typeID: &tid, priorityID: &pid}, domain.Ticket{
		TypeName: "Incident", PriorityName: "High",
	})
	if !strings.Contains(got, "type=Incident") {
		t.Errorf("missing type=Incident: %s", got)
	}
	if !strings.Contains(got, "priority=High") {
		t.Errorf("missing priority=High: %s", got)
	}
}

func TestChangedFieldsSummaryFallsBackToIDWhenNameMissing(t *testing.T) {
	tid := 7
	got := changedFieldsSummary(ticketUpdateFields{typeID: &tid}, domain.Ticket{}) // no TypeName
	if !strings.Contains(got, "type-id=7") {
		t.Errorf("expected fallback to type-id=7: %s", got)
	}
}
```

(`stubTicketsvc.resolvedType` doesn't exist yet — the existing stub only has `resolvedStatus` and `resolvedGroup`. Add a `resolvedType domain.TicketType` field to `stubTicketsvc` AND implement `ResolveTypeByName` on it.)

- [ ] **Step 3: Extend stub if needed**

Check `internal/cli/ticket/stub_test.go`:

```bash
grep -n "resolvedType\|ResolveTypeByName" internal/cli/ticket/stub_test.go
```

If `ResolveTypeByName` is NOT yet stubbed (it's on the `ticketsvcAPI` interface but the stub may not implement it because no caller has needed it before), add:

```go
type stubTicketsvc struct {
	// ... existing fields ...
	resolvedType domain.TicketType  // NEW (if not present)
}

// NEW (if not present):
func (s *stubTicketsvc) ResolveTypeByName(_ context.Context, _ string, _ int, _ string) (domain.TicketType, error) {
	return s.resolvedType, s.err
}
```

If `ticketsvcAPI` interface currently has `ResolveTypeByName` but `stubTicketsvc` doesn't implement it, the build will already be broken (interface unsatisfied). Add the implementation.

If `ResolveTypeByName` is NOT on the interface yet, add it AND the stub method:

```go
// in ticket.go
ResolveTypeByName(ctx context.Context, profile string, appID int, name string) (domain.TicketType, error)
```

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
golangci-lint run ./internal/cli/ticket/...
```

All clean.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ticket/update.go internal/cli/ticket/update_test.go internal/cli/ticket/stub_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): add update helpers (buildPatchOps, resolveFields, summary)"
```

(Adjust the file list — only include files actually modified.)

**No `Co-Authored-By:` trailer.**

---

## Task 3: CLI — `update.go` cobra command + runner

**Files:**
- Modify: `internal/cli/ticket/update.go` (append cobra wiring + runner)
- Modify: `internal/cli/ticket/update_test.go` (append runner tests)
- Modify: `internal/cli/ticket/ticket.go` (register `newUpdateCmd(nil)`)

- [ ] **Step 1: Add the runner**

Append to `internal/cli/ticket/update.go`:

```go
import (
	"io"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

func newUpdateCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		titleFlag   string
		descFlag    string
		typeArg     string
		accountArg  string
		requestArg  string
		groupArg    string
		priorityID  int
		commentFlag string
		yesFlag     bool
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update editable ticket fields (title/description/type/account/requestor/group/priority); --yes required",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
			}
			if !yesFlag {
				return fmt.Errorf("pass --yes to update the ticket")
			}
			raw := rawUpdateFlags{
				title:          titleFlag,
				titleSet:       cmd.Flags().Changed("title"),
				description:    descFlag,
				descriptionSet: cmd.Flags().Changed("description"),
				typeArg:        typeArg,
				accountArg:     accountArg,
				requestorArg:   requestArg,
				groupArg:       groupArg,
				priorityID:     priorityID,
				prioritySet:    cmd.Flags().Changed("priority-id"),
				comment:        commentFlag,
			}
			paths, err := config.ResolvePaths()
			if err != nil { return err }
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil { return err }
			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil { return err }
			s := svc
			if s == nil { s = ticketsvc.New(paths) }
			people := peoplesvc.New(paths)
			return runTicketUpdate(cmd.Context(), cmd.OutOrStdout(), s, people, profile, authedUID, appID, id, raw, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().StringVar(&titleFlag, "title", "", "set ticket title")
	cmd.Flags().StringVar(&descFlag, "description", "", "set ticket description (replaces existing)")
	cmd.Flags().StringVar(&typeArg, "type", "", "set ticket type by name or id")
	cmd.Flags().StringVar(&accountArg, "account", "", "set account by name or id")
	cmd.Flags().StringVar(&requestArg, "requestor", "", "set requestor by uid|email|me")
	cmd.Flags().StringVar(&groupArg, "responsibility-group", "", "set responsibility group by name or id")
	cmd.Flags().IntVar(&priorityID, "priority-id", 0, "set priority by id (numeric only this round)")
	cmd.Flags().StringVar(&commentFlag, "comment", "", "optional accompanying feed comment (posted after PATCH succeeds)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to mutate")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// ticketsvcUpdateAPI is the subset of ticketsvcAPI used by runTicketUpdate.
// (Not strictly required — the runner can just take ticketsvcAPI directly.
// We declare a local alias only if it improves test ergonomics; otherwise drop.)

func runTicketUpdate(ctx context.Context, w io.Writer, svc ticketsvcAPI, people peoplesvcAPI, profile, authedUID string, appID, id int, raw rawUpdateFlags, jsonOut bool) error {
	fields, err := resolveUpdateFields(ctx, svc, people, profile, authedUID, appID, raw)
	if err != nil {
		return err
	}
	if !fields.hasAny() && raw.comment == "" {
		return fmt.Errorf("nothing to update — pass at least one of --title / --description / --type / --account / --requestor / --responsibility-group / --priority-id / --comment")
	}
	var updated domain.Ticket
	if fields.hasAny() {
		ops := buildTicketPatchOps(fields)
		updated, err = svc.PatchTicket(ctx, profile, appID, id, ops)
		if err != nil {
			return err
		}
	} else {
		// Comment-only: don't PATCH; fetch the ticket so we can render JSON if asked.
		updated, err = svc.GetTicket(ctx, profile, appID, id)
		if err != nil {
			return err
		}
	}

	commentNote := ""
	if raw.comment != "" {
		feedID, ferr := svc.AddFeed(ctx, profile, appID, id, raw.comment, false, nil)
		if ferr != nil {
			commentNote = fmt.Sprintf(" (warning: comment failed: %v)", ferr)
		} else {
			commentNote = fmt.Sprintf(" (comment posted: feed entry %d)", feedID)
		}
	}

	if jsonOut {
		return render.JSON(w, struct {
			Schema string        `json:"schema"`
			Ticket domain.Ticket `json:"ticket"`
		}{Schema: "tdx.v1.ticket", Ticket: updated})
	}

	summary := changedFieldsSummary(fields, updated)
	if summary == "" {
		_, _ = fmt.Fprintf(w, "ticket #%d: no field changes%s\n", id, commentNote)
	} else {
		_, _ = fmt.Fprintf(w, "ticket #%d updated: %s%s\n", id, summary, commentNote)
	}
	return nil
}
```

Note: missing imports (`strconv`, `ticketsvc`) need adding at the top of the file. Reuse existing imports if present.

- [ ] **Step 2: Wire into `New()` in `ticket.go`**

```bash
grep -n "newAssignCmd\|newLogCmd" internal/cli/ticket/ticket.go
```

Find the line `cmd.AddCommand(newAssignCmd(nil))`. Add directly after it:

```go
cmd.AddCommand(newUpdateCmd(nil))
```

(Place after `assign` and before `log` to match the spec's stated ordering.)

- [ ] **Step 3: Append runner tests**

In `internal/cli/ticket/update_test.go`:

```go
func TestRunTicketUpdateSuccess(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{ID: 100, Title: "After", TypeName: "Incident"}}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		title: "After", titleSet: true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ticket #100 updated") {
		t.Errorf("output: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `title="After"`) {
		t.Errorf("expected title in summary: %s", buf.String())
	}
	if len(stub.lastPatchOps) != 1 || stub.lastPatchOps[0].Path != "/Title" {
		t.Errorf("patch ops: %+v", stub.lastPatchOps)
	}
}

func TestRunTicketUpdateWithComment(t *testing.T) {
	stub := &stubTicketsvc{
		patched:     domain.Ticket{ID: 100, Title: "T"},
		feedAddedID: 999,
	}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		title: "T", titleSet: true, comment: "fyi",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if stub.lastFeedBody != "fyi" {
		t.Errorf("comment not posted: %q", stub.lastFeedBody)
	}
	if !strings.Contains(buf.String(), "feed entry 999") {
		t.Errorf("expected feed-entry mention: %s", buf.String())
	}
}

func TestRunTicketUpdateNothingToUpdate(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	err := runTicketUpdate(context.Background(), io.Discard, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{}, false)
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("want nothing-to-update error, got %v", err)
	}
}

func TestRunTicketUpdateCommentOnlyDoesNotPatch(t *testing.T) {
	// --comment alone, no field flags — should call GetTicket and AddFeed but NOT PatchTicket.
	stub := &stubTicketsvc{
		ticket:      domain.Ticket{ID: 100, Title: "T"}, // returned by GetTicket
		feedAddedID: 555,
	}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		comment: "just a note",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.lastPatchOps) != 0 {
		t.Errorf("PatchTicket should NOT have been called; got ops %+v", stub.lastPatchOps)
	}
	if stub.lastFeedBody != "just a note" {
		t.Errorf("comment not posted: %q", stub.lastFeedBody)
	}
}

func TestRunTicketUpdateJSON(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{ID: 100, Title: "T"}}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		title: "T", titleSet: true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticket" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestNewUpdateCmdRequiresYes(t *testing.T) {
	cmd := newUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "--title", "x"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}
```

Required test imports: `bytes`, `encoding/json`, `io`. Check existing imports in the file and add what's missing.

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./internal/cli/ticket/...
golangci-lint run ./internal/cli/ticket/...
go run ./cmd/tdx ticket update --help
```

`tdx ticket update --help` should list every flag.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ticket/update.go internal/cli/ticket/update_test.go internal/cli/ticket/ticket.go
git commit -m "feat(cli/ticket): add update command (mutating, --yes required)"
```

**No `Co-Authored-By:` trailer.**

---

## Task 4: MCP — `update_ticket` tool

**Files:**
- Modify: `internal/mcp/tools_ticket_mutating.go`
- Modify: `internal/mcp/tools_ticket_test.go`
- Modify: `internal/mcp/server_test.go` (tool count 62 → 63)

- [ ] **Step 1: Add argument type**

Append to `internal/mcp/tools_ticket_mutating.go`:

```go
type updateTicketArgs struct {
	Profile               string `json:"profile,omitempty"`
	AppID                 int    `json:"appID,omitempty"`
	ID                    int    `json:"id"`
	Title                 string `json:"title,omitempty"`
	TitleSet              bool   `json:"titleSet,omitempty" jsonschema:"set true to send title even if empty (otherwise empty title means 'don't change')"`
	Description           string `json:"description,omitempty"`
	DescriptionSet        bool   `json:"descriptionSet,omitempty" jsonschema:"set true to send description even if empty"`
	TypeID                int    `json:"typeID,omitempty"`
	TypeName              string `json:"typeName,omitempty"`
	AccountID             int    `json:"accountID,omitempty"`
	AccountName           string `json:"accountName,omitempty"`
	RequestorUID          string `json:"requestorUID,omitempty"`
	ResponsibilityGroupID int    `json:"responsibilityGroupID,omitempty"`
	PriorityID            int    `json:"priorityID,omitempty"`
	Comment               string `json:"comment,omitempty"`
	Confirm               bool   `json:"confirm" jsonschema:"set true to actually update"`
}
```

The `TitleSet`/`DescriptionSet` flags are needed to disambiguate empty-string-as-value from omitted: in JSON, `"title": ""` and not-providing-title both deserialize to `Title=""`. The opt-in `*Set` booleans let agents intentionally clear those fields.

- [ ] **Step 2: Add handler**

```go
func updateTicketHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateTicketArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args updateTicketArgs) (*sdkmcp.CallToolResult, any, error) {
		if errResp, ok := confirmGate(args.Confirm, "set confirm=true to update the ticket"); !ok {
			return errResp, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)

		// Build the field set inline. Numeric ID fields are checked > 0;
		// name fields trigger resolver calls.
		var fields struct {
			title           *string
			description     *string
			typeID          *int
			accountID       *int
			requestorUID    *string
			groupID         *int
			priorityID      *int
		}

		if args.TitleSet || args.Title != "" {
			v := args.Title
			fields.title = &v
		}
		if args.DescriptionSet || args.Description != "" {
			v := args.Description
			fields.description = &v
		}
		if args.TypeID > 0 {
			v := args.TypeID
			fields.typeID = &v
		} else if args.TypeName != "" {
			tt, err := svcs.Tickets.ResolveTypeByName(ctx, profile, args.AppID, args.TypeName)
			if err != nil {
				return errorResult(fmt.Sprintf("update_ticket: typeName: %v", err)), nil, nil
			}
			v := tt.ID
			fields.typeID = &v
		}
		if args.AccountID > 0 {
			v := args.AccountID
			fields.accountID = &v
		} else if args.AccountName != "" {
			acct, err := svcs.People.ResolveAccountByName(ctx, profile, args.AccountName)
			if err != nil {
				return errorResult(fmt.Sprintf("update_ticket: accountName: %v", err)), nil, nil
			}
			v := acct.ID
			fields.accountID = &v
		}
		if args.RequestorUID != "" {
			v := args.RequestorUID
			fields.requestorUID = &v
		}
		if args.ResponsibilityGroupID > 0 {
			v := args.ResponsibilityGroupID
			fields.groupID = &v
		}
		if args.PriorityID > 0 {
			v := args.PriorityID
			fields.priorityID = &v
		}

		hasAny := fields.title != nil || fields.description != nil || fields.typeID != nil ||
			fields.accountID != nil || fields.requestorUID != nil || fields.groupID != nil || fields.priorityID != nil

		if !hasAny && args.Comment == "" {
			return errorResult("update_ticket: nothing to update"), nil, nil
		}

		var updated domain.Ticket
		var err error
		if hasAny {
			ops := []ticketsvc.PatchOp{}
			if fields.title != nil {
				ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/Title", Value: *fields.title})
			}
			if fields.description != nil {
				ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/Description", Value: *fields.description})
			}
			if fields.typeID != nil {
				ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/TypeID", Value: *fields.typeID})
			}
			if fields.accountID != nil {
				ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/AccountID", Value: *fields.accountID})
			}
			if fields.requestorUID != nil {
				ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/RequestorUid", Value: *fields.requestorUID})
			}
			if fields.groupID != nil {
				ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/ResponsibleGroupID", Value: *fields.groupID})
			}
			if fields.priorityID != nil {
				ops = append(ops, ticketsvc.PatchOp{Op: "replace", Path: "/PriorityID", Value: *fields.priorityID})
			}
			updated, err = svcs.Tickets.PatchTicket(ctx, profile, args.AppID, args.ID, ops)
			if err != nil {
				return errorResult(fmt.Sprintf("update_ticket: %v", err)), nil, nil
			}
		} else {
			updated, err = svcs.Tickets.GetTicket(ctx, profile, args.AppID, args.ID)
			if err != nil {
				return errorResult(fmt.Sprintf("update_ticket: %v", err)), nil, nil
			}
		}

		if args.Comment != "" {
			if _, ferr := svcs.Tickets.AddFeed(ctx, profile, args.AppID, args.ID, args.Comment, false, nil); ferr != nil {
				return errorResult(fmt.Sprintf("update_ticket: patch succeeded but comment failed: %v", ferr)), nil, nil
			}
		}

		return jsonResult(struct {
			Schema string        `json:"schema"`
			Ticket domain.Ticket `json:"ticket"`
		}{Schema: "tdx.v1.ticket", Ticket: updated})
	}
}
```

(Imports needed: `context`, `fmt`, `domain`, `ticketsvc`, `sdkmcp`. Verify.)

The patch-op-builder loop here duplicates the CLI's `buildTicketPatchOps` because the MCP and CLI use slightly different field-set types. If you want to share, factor `buildTicketPatchOps` to take an exported type — but for v0.16.3 the duplication is acceptable (~30 lines, one place each).

- [ ] **Step 3: Register the tool**

In `RegisterTicketMutatingTools`, after the existing mutating tools:

```go
sdkmcp.AddTool(srv, &sdkmcp.Tool{
	Name:        "update_ticket",
	Description: "Update a ticket's editable fields (title/description/type/account/requestor/group/priority). Excludes status (use update_ticket_status) and assignee (use update_ticket_assignee). Optional comment posted after the patch. Requires confirm=true.",
}, updateTicketHandler(svcs))
```

- [ ] **Step 4: Update tool-count assertion**

In `internal/mcp/server_test.go`:

```bash
grep -n "wantCount\s*=\s*62\|wantCount\s*:=\s*62" internal/mcp/server_test.go
```

Update `62` → `63`.

- [ ] **Step 5: Smoke test**

Append to `internal/mcp/tools_ticket_test.go`:

```go
func TestRegisterUpdateTicketTool_NoPanic(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTicketMutatingTools(srv, Services{})
}
```

(Skip if a sibling `TestRegisterTicketMutatingTools_NoPanic` already covers this.)

- [ ] **Step 6: Verify**

```bash
go build ./...
go test ./internal/mcp/...
golangci-lint run ./internal/mcp/...
```

All clean. Tool count test should now expect 63.

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/tools_ticket_mutating.go internal/mcp/tools_ticket_test.go internal/mcp/server_test.go
git commit -m "feat(mcp): add update_ticket mutating tool"
```

**No `Co-Authored-By:` trailer.**

---

## Task 5: Documentation

**Files:**
- Modify: `docs/guide/ticket.md`
- Modify: `docs/guide/mcp.md`
- Modify: `docs/guide.md`
- Modify: `README.md`

- [ ] **Step 1: Update `docs/guide/ticket.md`**

Add a new section AFTER `## tdx ticket assign` and BEFORE `## tdx ticket log`:

```markdown
## tdx ticket update

Update editable ticket fields. **Mutating** — `--yes` required. Excludes status (use `tdx ticket status`) and assignee (use `tdx ticket assign`).

```bash
tdx ticket update <id> --title "New title" --yes
tdx ticket update <id> --description "..." --comment "rolled back" --yes
tdx ticket update <id> --type "Incident" --priority-id 3 --yes
tdx ticket update <id> --requestor alice@uf.edu --yes
tdx ticket update <id> --responsibility-group "Linux Team" --yes
```

Flags:

- `--title "<text>"` — replace ticket title
- `--description "<text>"` — replace ticket description (full replacement; `--description ""` clears it)
- `--type <name|id>` — set ticket type by name (case-insensitive exact match) or numeric id
- `--account <name|id>` — set account by name (resolved via `tdx people accounts list`) or numeric id
- `--requestor <uid|email|me>` — set requestor; `me` = the authenticated user
- `--responsibility-group <name|id>` — set responsibility group by name or id (use `tdx ticket groups list` to discover names)
- `--priority-id <int>` — set priority by numeric id (priority-name resolution not supported in v0.16.3)
- `--comment "<text>"` — optional accompanying feed comment posted after the PATCH succeeds; comment-only invocations (no field flags) are valid and just post the comment

At least one of the field flags or `--comment` must be set. The PATCH is sent as a single TD call; if any field is rejected, nothing applies. JSON output is the full updated ticket (`tdx.v1.ticket` envelope).
```

Update the Contents TOC at the top of `ticket.md` to include `[tdx ticket update](#tdx-ticket-update)` after the `assign` entry.

- [ ] **Step 2: Update `docs/guide/mcp.md`**

(a) Tool count: total `62 → 63`. Mutating count for tickets: was 6, now 7.

(b) Add a row to "Tickets (Phase D — mutating)" table:

```markdown
| `update_ticket` | Update editable ticket fields (title/description/type/account/requestor/group/priority + optional comment) |
```

Update the section header `(Phase D — mutating, 6 tools)` → `(Phase D — mutating, 7 tools)`.

- [ ] **Step 3: ASCII tree in `docs/guide.md`**

Find the `ticket` branch. The current `comment / status / assign / log` line becomes:

```text
│   ├── comment / status / assign / update / log
```

(Insert `update` between `assign` and `log`.)

- [ ] **Step 4: ASCII tree in `README.md`**

Apply the IDENTICAL change. Verify byte-identity:

```bash
diff <(awk '/^```text$/,/^```$/' docs/guide.md) <(awk '/^```text$/,/^```$/' README.md)
```

Expected: empty.

- [ ] **Step 5: Verify**

```bash
grep -n "## tdx ticket update" docs/guide/ticket.md
grep -n "update_ticket\b" docs/guide/mcp.md
grep -n "63" docs/guide/mcp.md | head
diff <(awk '/^```text$/,/^```$/' docs/guide.md) <(awk '/^```text$/,/^```$/' README.md)
```

Each should return non-empty / empty as appropriate.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/ticket.md docs/guide/mcp.md docs/guide.md README.md
git commit -m "docs: add tdx ticket update reference + tree updates + MCP tool table"
```

**No `Co-Authored-By:` trailer.**

---

## Task 6: Live verification + PR + release

**Files:** none modified — verification + git operations only.

- [ ] **Step 1: User must re-auth (token expired during design)**

```bash
tdx auth login --sso
tdx auth status
```

Expected: `state: authenticated`, `token: valid`. (This step requires the human running `tdx`; if you're a subagent, surface this as a blocker and ask the controller to re-auth before proceeding.)

- [ ] **Step 2: Pre-flight**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

All green required.

- [ ] **Step 3: Build local binary**

```bash
go build -o tdx ./cmd/tdx
```

- [ ] **Step 4: Live-verify each command path**

```bash
# Title change
./tdx ticket update 542034 --title "v0.16.3 verify" --yes
./tdx ticket show 542034 | head -2   # confirm Title shows new value

# Description + comment
./tdx ticket update 542034 --description "v0.16.3 description" --comment "fyi" --yes
./tdx ticket feed 542034 --limit 1   # confirm comment posted
./tdx ticket show 542034 | grep -A 3 "Description"   # confirm new description

# Type by name
./tdx ticket update 542034 --type "IT Tickets - Default" --yes

# Comment-only path
./tdx ticket update 542034 --comment "comment-only path" --yes
./tdx ticket feed 542034 --limit 2

# Requestor change (back to me — should be no-op since 542034 is yours, but verify the wire works)
./tdx ticket update 542034 --requestor me --yes

# Roll back to original title/description
./tdx ticket update 542034 --title "Test ticket for tdx cli testing" --description "Test ticket for tdx cli testing" --comment "rolled back" --yes
```

Expected behaviors:
- Each `--yes` invocation succeeds with the field-changed summary.
- `--yes`-less and all-empty invocations error fast.
- Comment-only invocation does NOT call PATCH; you can verify by capturing TD's API logs (or just trust the runner test).

If any wire-format issue surfaces (e.g. PATCH `/AccountID` returns 400, or the path name is wrong), fix `buildTicketPatchOps` and re-test.

- [ ] **Step 5: Push branch**

```bash
rm tdx 2>/dev/null
git push -u origin ticket-update
```

- [ ] **Step 6: Open PR**

Write the body to `/tmp/pr-body-v0.16.3.md`:

```markdown
## Summary

Phase D.2a — `tdx ticket update <id>` for generic field updates beyond the existing dedicated `status`/`assign`/`comment`/`log` commands. One new MCP tool (`update_ticket`). Tool count 62 → 63.

`tdx ticket create` is split out and deferred to v0.17.0 — Sample probing on 2026-05-09 confirmed typical (non-admin) users lack API create permission on the canonical IT Tickets app.

### What `update` covers

- `--title`, `--description` — replace those fields (empty string = clear)
- `--type <name|id>`, `--account <name|id>`, `--responsibility-group <name|id>` — name-or-id resolution via existing helpers
- `--requestor <uid|email|me>` — existing principal resolution
- `--priority-id <int>` — numeric only this round
- `--comment "..."` — optional accompanying feed comment after the PATCH (or comment-only — no field flags + just `--comment`)

Excludes status and assignee — those have dedicated commands; do not duplicate.

### Live-verified on the test tenant ticket 542034

- title / description changes ✓
- type by name ✓
- requestor by `me` ✓
- comment-only (no PATCH) ✓
- comment-after-PATCH ✓
- `--yes`-less and all-empty rejection ✓

Spec: `docs/specs/2026-05-09-tdx-ticket-update.md`
Plan: `docs/plans/2026-05-09-tdx-ticket-update.md`

## Test plan

- [x] `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` all green
- [x] `tdx ticket update --help` lists all flags
- [x] All field-flag combinations exercised on test ticket 542034
- [x] `--yes`-less and all-empty rejected fast
- [x] Tool count = 63

After merge, tag `v0.16.3` to trigger Goreleaser.
```

```bash
gh pr create --title "v0.16.3: tdx ticket update" --body-file /tmp/pr-body-v0.16.3.md
```

- [ ] **Step 7: Merge after CI passes**

```bash
gh pr merge <PR#> --squash --admin --delete-branch
```

- [ ] **Step 8: Reset main, tag, push tag**

```bash
git checkout main
git fetch origin
git reset --hard origin/main
git tag v0.16.3
git push origin v0.16.3
```

- [ ] **Step 9: Update memory**

`MEMORY.md` index → point at v0.16.3 + "tdx ticket update". `project_tdx_current_state.md` gets a new "Latest release" block. `project_tdx_backlog.md` notes "ticket create deferred to v0.17.0" (already noted; refresh if helpful).

---

## Self-Review

**1. Spec coverage:**
- Spec § Command surface (`tdx ticket update <id>` + 9 flags) → Tasks 2 + 3
- Spec § Validation (at least one flag, `--yes` required) → Task 3 (cobra-level + runner-level)
- Spec § Behavior nuances (atomic PATCH, comment-only path, empty-string handling) → Tasks 2-3 + tests
- Spec § MCP (`update_ticket` tool with 14 input fields, mutating, `confirm:true`) → Task 4
- Spec § Service layer (no new methods) → no task needed
- Spec § Domain layer (no changes) → no task needed
- Spec § Documentation (4 files) → Task 5
- Spec § Live verification → Task 6
- Spec § Acceptance criteria 1-11 → covered by Tasks 3, 4, 5, 6

All requirements have a task.

**2. Placeholder scan:**
- Some "Adjust the file list" / "Verify by grep" instructions are concrete probe directives, not vague placeholders.
- No "TBD"/"TODO".

**3. Type consistency:**
- `rawUpdateFlags` and `ticketUpdateFields` defined in Task 2, used in Task 3 + (parallel duplicates) in Task 4 MCP handler.
- `buildTicketPatchOps` defined in Task 2, used in Task 3 runner; MCP handler in Task 4 inlines a parallel version (acknowledged in the plan).
- `peoplesvcAPI.ResolveAccountByName` signature — Task 1 defines it consistent with `peoplesvc.Service.ResolveAccountByName` already in the codebase.
- `stubTicketsvc.resolvedType` and `ResolveTypeByName` — Task 2 step 3 instructs the implementer to add them if not already present.
- `tdx.v1.ticket` envelope schema name — consistent across Task 3 (CLI JSON) and Task 4 (MCP result).

All consistent.
