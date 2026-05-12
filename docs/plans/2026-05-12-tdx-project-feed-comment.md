# `tdx project` Phase 2 — Feed + Comment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Ship `tdx project feed`, `tdx project comment`, `tdx project task feed`, `tdx project task comment` + 4 MCP tools. Mirror `tdx ticket feed` / `comment` / `task feed` line-for-line.

**Spec:** `docs/specs/2026-05-12-tdx-project-feed-comment.md`

**Tech stack:** Go 1.26.2 + cobra + the established `tdx ticket` patterns. No new dependencies.

**Wire-format facts confirmed by live probe (2026-05-12):**

- `GET /api/projects/{id}/feed` returns array of entries with shape: `ID`, `Body`, `CreatedUid` (lowercase Uid in TD's JSON), `CreatedFullName`, `CreatedDate`, `LastUpdatedDate`, `UpdateType` (1=comment, 3=system), `IsPrivate`, `LikesCount`, `RepliesCount`. System entries have null `UID`/`FullName` but populated `CreatedFullName` ("API User" etc.).
- `POST /api/projects/{id}/feed` accepts body `{Comments, Notify, IsPrivate}`.
- Task feed paths: `/api/projects/{p}/plans/{pl}/tasks/{t}/feed` for both GET and POST. Same row shape.

**Existing pattern to mirror:** `internal/svc/ticketsvc/feed.go` + `internal/cli/ticket/feed.go` + `internal/cli/ticket/comment.go`. These are the closest analogs.

---

## File structure

```
internal/
├── domain/
│   ├── project_feed.go            # NEW: ProjectFeedEntry + UpdateTypeLabel()
│   └── project_feed_test.go       # NEW
├── svc/projectsvc/
│   ├── feed.go                    # NEW: GetFeed/AddFeed/GetTaskFeed/AddTaskFeed
│   ├── feed_test.go               # NEW
│   ├── types.go                   # MODIFY: add wireFeedEntry, wireFeedAdd
│   └── service.go                 # unchanged
├── cli/project/
│   ├── feed.go                    # NEW: tdx project feed
│   ├── feed_test.go               # NEW
│   ├── comment.go                 # NEW: tdx project comment
│   ├── comment_test.go            # NEW
│   ├── task.go                    # MODIFY: add `task feed` + `task comment` subcommands
│   ├── task_test.go               # MODIFY: tests for the new subcommands
│   └── project.go                 # MODIFY: projectsvcAPI gains 4 new methods; register new subcommands
├── mcp/
│   ├── tools_project.go           # MODIFY: add get_project_feed + get_project_task_feed
│   ├── tools_project_mutating.go  # MODIFY: add add_project_comment + add_project_task_comment
│   ├── tools_project_test.go      # MODIFY: tests for the 4 new tools
│   └── server.go                  # already wires RegisterProjectTools (no change)
docs/
├── specs/2026-05-12-tdx-project-feed-comment.md   # already committed
├── plans/2026-05-12-tdx-project-feed-comment.md   # this file
└── guide/project.md               # MODIFY: 4 new command sections + updated MCP table
```

---

## Tasks

### Task 1: Domain type + tests

**Files:** `internal/domain/project_feed.go`, `internal/domain/project_feed_test.go`

- [ ] **Step 1.1: Create `project_feed.go`**

```go
package domain

import "time"

// ProjectFeedEntry is one row from GET /api/projects/{id}/feed or
// /api/projects/{p}/plans/{pl}/tasks/{t}/feed. UpdateType: 1=comment, 3=system.
type ProjectFeedEntry struct {
    ID              int       `json:"id"`
    Body            string    `json:"body"`
    CreatedByUID    string    `json:"createdByUID,omitempty"`
    CreatedByName   string    `json:"createdByName,omitempty"`
    CreatedDate     time.Time `json:"createdDate,omitempty"`
    LastUpdatedDate time.Time `json:"lastUpdatedDate,omitempty"`
    UpdateType      int       `json:"updateTypeID,omitempty"`
    IsPrivate       bool      `json:"isPrivate,omitempty"`
    LikesCount      int       `json:"likesCount,omitempty"`
    RepliesCount    int       `json:"repliesCount,omitempty"`
}

func (e ProjectFeedEntry) UpdateTypeLabel() string {
    if e.UpdateType == 1 {
        return "comment"
    }
    return "system"
}
```

- [ ] **Step 1.2: Tests**
  - `TestProjectFeedEntry_UpdateTypeLabel` covers 1 → "comment", 3 → "system", 0 → "system" (default).
  - `TestProjectFeedEntry_JSONRoundTrip` ensures the struct serializes and deserializes losslessly.

- [ ] **Step 1.3: Verify + commit**

```
feat(domain): add ProjectFeedEntry type for project + task feed
```

---

### Task 2: Service layer

**Files:** `internal/svc/projectsvc/feed.go`, `feed_test.go`; modify `types.go`

- [ ] **Step 2.1: Wire types in `types.go`** — append:

```go
type wireFeedEntry struct {
    ID              int    `json:"ID"`
    Body            string `json:"Body"`
    CreatedUid      string `json:"CreatedUid,omitempty"`
    CreatedFullName string `json:"CreatedFullName,omitempty"`
    CreatedDate     string `json:"CreatedDate,omitempty"`
    LastUpdatedDate string `json:"LastUpdatedDate,omitempty"`
    UpdateType      int    `json:"UpdateType,omitempty"`
    IsPrivate       bool   `json:"IsPrivate,omitempty"`
    LikesCount      int    `json:"LikesCount,omitempty"`
    RepliesCount    int    `json:"RepliesCount,omitempty"`
}

type wireFeedAdd struct {
    Comments  string   `json:"Comments"`
    Notify    []string `json:"Notify,omitempty"`
    IsPrivate bool     `json:"IsPrivate"`
}
```

- [ ] **Step 2.2: Implement `feed.go`** — TDD against httptest. Methods:

```go
GetFeed(ctx, profile, projectID int) ([]domain.ProjectFeedEntry, error)
AddFeed(ctx, profile, projectID int, message string, isPrivate bool, notify []string) (int, error)
GetTaskFeed(ctx, profile, projectID, planID, taskID int) ([]domain.ProjectFeedEntry, error)
AddTaskFeed(ctx, profile, projectID, planID, taskID int, message string, isPrivate bool, notify []string) (int, error)
```

Both Add* methods return the integer entry ID from the response (`ID` field of the returned feed entry).

`decodeFeedEntry(w wireFeedEntry) domain.ProjectFeedEntry` parses dates via the existing `parseTD` helper, maps `CreatedUid`/`CreatedFullName` → `CreatedByUID`/`CreatedByName`.

- [ ] **Step 2.3: Tests in `feed_test.go`** — for each method:
  - `TestGetFeed_DecodesEntries` — httptest serves an array including one comment + one system event; verify both decode correctly with proper UpdateType.
  - `TestAddFeed_PostsExpectedBody` — capture request body; assert `Comments`/`Notify`/`IsPrivate` serialize as expected; return `{"ID": 9999}` and assert the method returns 9999.
  - Repeat for `GetTaskFeed` / `AddTaskFeed` against the longer path.

- [ ] **Step 2.4: Verify + commit**

```
feat(projectsvc): add GetFeed/AddFeed + task variants
```

---

### Task 3: CLI `tdx project feed` + tests

**Files:** `internal/cli/project/feed.go`, `feed_test.go`; modify `project.go` to register

- [ ] **Step 3.1: `project.go`** — extend `projectsvcAPI` interface with the 4 new methods.

- [ ] **Step 3.2: `feed.go`** — implement:

```
tdx project feed <project-id> [--limit N] [--json] [--profile P]
```

Default limit 50, max 500. Rendering: `printProjectFeed(w io.Writer, entries []domain.ProjectFeedEntry, jsonOut bool, schemaTag string, extras map[string]any)`. Columns: ID, DATE, BY, TYPE, BODY (truncated to 80 chars; replace embedded `\n` with `↵`).

JSON envelope: `tdx.v1.projectFeed` with `projectID` + `entries[]`.

- [ ] **Step 3.3: Tests** — stub-based:
  - `TestFeed_Renders` — 2 entries, asserts both IDs + bodies.
  - `TestFeed_JSON` — asserts schema + entries field.
  - `TestFeed_EmptyMessage` — empty result prints "no feed entries on project N".
  - `TestFeed_RespectsLimit`.

- [ ] **Step 3.4: Register in `project.go`** — add to `New()` subcommand list.

- [ ] **Step 3.5: Verify + commit**

```
feat(cli/project): tdx project feed
```

---

### Task 4: CLI `tdx project comment` + tests

**Files:** `internal/cli/project/comment.go`, `comment_test.go`

- [ ] **Step 4.1: Implement**

```
tdx project comment <project-id> --message "..." [--notify me|UID|email]... [--private] --yes [--profile P]
```

Validation order (BEFORE config.ResolvePaths, so unit tests without profile config still hit the right errors):
1. `--yes` required (no preview mode for comments; mirrors `tdx ticket comment`)
2. `--message` required and non-empty after trim
3. project-id positive integer

`--notify` resolved via `resolvePrincipal` (same helper used in search). Calls `svc.AddFeed`. Prints `posted comment on project N (feed entry M)`.

- [ ] **Step 4.2: Tests**
  - `TestComment_RequiresYes`
  - `TestComment_RequiresMessage`
  - `TestComment_BuildsExpectedCall` — verify `AddFeed` receives message, notify slice, isPrivate=false.
  - `TestComment_ResolvesNotifyMe` — stub people-svc returns authed UID for "me".
  - `TestComment_Private` — `--private` propagates.
  - `TestComment_HappyPath` — success line includes the entry ID.

- [ ] **Step 4.3: Register + commit**

```
feat(cli/project): tdx project comment
```

---

### Task 5: CLI `task feed` + `task comment` (extends task.go)

**Files:** modify `internal/cli/project/task.go` + `task_test.go`

- [ ] **Step 5.1: Add `newTaskFeedCmd(svc)` and `newTaskCommentCmd(svc)`** alongside the existing `task list` / `task show`.

```
tdx project task feed <project-id> <task-id> --plan <plan-id> [--limit N] [--json] [--profile P]
tdx project task comment <project-id> <task-id> --plan <plan-id> --message "..." [--notify ...]... [--private] --yes [--profile P]
```

Same validation precedence as project-level commands. `printProjectTaskFeed` mirrors `printProjectFeed` but JSON schema is `tdx.v1.projectTaskFeed` with `planID` + `taskID` in the envelope.

- [ ] **Step 5.2: Tests** — minimal: one happy-path render test + one happy-path comment test. Validation is covered by the project-level tests for the same patterns.

- [ ] **Step 5.3: Register** under `task` subcommand. Verify + commit.

```
feat(cli/project): tdx project task feed + task comment
```

---

### Task 6: MCP tools

**Files:** `internal/mcp/tools_project.go`, `tools_project_mutating.go`, `tools_project_test.go`

- [ ] **Step 6.1: Read tools** in `tools_project.go`:
  - `get_project_feed` — input `{profile?, projectID, limit?}`; output `{schema:"tdx.v1.projectFeed", projectID, entries:[...]}`.
  - `get_project_task_feed` — input `{profile?, projectID, planID, taskID, limit?}`; output `{schema:"tdx.v1.projectTaskFeed", projectID, planID, taskID, entries:[...]}`.

- [ ] **Step 6.2: Mutating tools** in `tools_project_mutating.go`:
  - `add_project_comment` — input `{profile?, projectID, message, notifyUIDs?, isPrivate?, confirm}`; mirrors `add_ticket_comment` for confirm-gate handling.
  - `add_project_task_comment` — input `{profile?, projectID, planID, taskID, message, notifyUIDs?, isPrivate?, confirm}`.

Both validate `message` non-empty and `confirm:true`; return success envelope `{ok:true, entryID:N}`.

- [ ] **Step 6.3: Tests** — extend `tools_project_test.go`:
  - schema envelope + happy path for each tool
  - `add_project_comment` errors when confirm missing
  - `add_project_comment` errors when message empty

- [ ] **Step 6.4: Update tool-count assertion** (`internal/mcp/server_test.go`) from 70 → 74.

- [ ] **Step 6.5: Commit**

```
feat(mcp): add 4 project feed/comment tools (70 → 74)
```

---

### Task 7: Live verification

- [ ] **Step 7.1: Build + run each new command against the live tenant**

```bash
go build -o /tmp/tdx-pcom ./cmd/tdx
/tmp/tdx-pcom project feed 259                       # known project from Phase 1
/tmp/tdx-pcom project feed 259 --json | jq '.entries | length'
/tmp/tdx-pcom project task feed 259 4938 --plan 1292
/tmp/tdx-pcom project comment 259 --message "tdx Phase 2 live test" --yes
/tmp/tdx-pcom project task comment 259 4938 --plan 1292 --message "tdx Phase 2 live test" --yes
```

- [ ] **Step 7.2: Confirm each posted comment** appears at the top of the next `feed` invocation. Capture the returned entry IDs.

- [ ] **Step 7.3: Document any wire-format surprises** in the spec's post-implementation section.

---

### Task 8: Docs

- [ ] **Step 8.1: `docs/guide/project.md`** — add four new sections:
  - `tdx project feed`
  - `tdx project comment`
  - `tdx project task feed`
  - `tdx project task comment`

  Update the MCP tools table (70 → 74).

- [ ] **Step 8.2: Update the index `docs/guide.md`** command tree to mention `feed`/`comment` under `project`.

- [ ] **Step 8.3: README ASCII tree** — same update.

- [ ] **Step 8.4: Commit**

```
docs: tdx project feed + comment guide entries
```

---

### Task 9: PR + tag

- [ ] **Step 9.1: `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...`** clean.

- [ ] **Step 9.2: Push + open PR** titled `v0.18.0: tdx project feed + comment (Phase 2)`.

- [ ] **Step 9.3: Squash-merge with `--admin` after CI green.**

- [ ] **Step 9.4: Tag `v0.18.0`** → Goreleaser publishes.

- [ ] **Step 9.5: Update memory** — bump current_state to v0.18.0; mark Phase 2 shipped in backlog.

---

## Coverage check vs spec

- ✅ `tdx project feed` → Task 3
- ✅ `tdx project comment` → Task 4
- ✅ `tdx project task feed` + `task comment` → Task 5
- ✅ 4 MCP tools → Task 6
- ✅ `domain.ProjectFeedEntry` + tests → Task 1
- ✅ Service-layer methods + tests → Task 2
- ✅ Live verify → Task 7
- ✅ Docs → Task 8
- ✅ Release → Task 9
