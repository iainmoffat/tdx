# `tdx project` Phase 2 — Feed + Comment

**Date:** 2026-05-12
**Goal:** Read feed activity and post comments on projects and project tasks. Closes the daily-flow loop for `tdx project` by adding the mutating commands that Phase 1 deliberately deferred. Mirrors `tdx ticket feed` / `tdx ticket comment` / `tdx ticket task feed` and the existing task-feed POST pattern.

> **Post-implementation correction (2026-05-12):** Live verification revealed that TD's project feed POST uses a different field name than the ticket/task feed POST. `POST /api/projects/{id}/feed` requires `{"Body": "...", "Notify": [...], "IsPrivate": ...}` — NOT `{"Comments": ...}` as the ticket and task feed endpoints use. Posting with `"Comments"` returns a stub response (`ID: -1`, body null) and silently no-ops; posting with `"Body"` returns a real entry ID and the comment is persisted. The implementation splits the wire body into two distinct structs: `wireProjectFeedAdd { Body, Notify, IsPrivate }` for project-level POST and `wireTaskFeedAdd { Comments, Notify, IsPrivate }` for task-level POST. Task-level feed POST continues to use `Comments` (matching ticket/task feed). Both verified live.

## Motivation

Phase 1 made projects/plans/tasks visible but read-only. Daily flows want two more things:

1. **See what changed** — recent feed entries on a project or task (status changes, comments, attachments).
2. **Add a comment** — a lightweight way to communicate progress / blockers without leaving the terminal.

The wire shape for project feed / task feed is essentially identical to ticket feed (same universal POST body `{Comments, Notify, IsPrivate}`; same enum-3 system events; same `CreatedUid`/`CreatedFullName` fields). No new infrastructure needed beyond the project equivalents of types and service methods.

## Decisions

Settled during planning on 2026-05-12:

1. **4 new commands + 4 new MCP tools** in two pairs (project, project-task). No batched "update + comment" surface like `tdx ticket task update` — projects don't need it (Phase 1 stays read-only for tasks).
2. **Full ticket parity** for flags: `--message`, `--notify UID` (repeatable, resolves `me`/email/UID), `--private`, `--yes` required.
3. **New domain type** `domain.ProjectFeedEntry`. Small duplication of `TicketFeedEntry` (same precedent as `ProjectTask` vs `TicketTask`). Keeps domain models clean per area.
4. **POST returns the new entry ID.** Surfaced on the human side and in the JSON envelope.
5. **JSON envelopes (additive):** `tdx.v1.projectFeed`, `tdx.v1.projectTaskFeed`.

## Decisions deferred / out of scope

- **Reply to a feed entry / like a feed entry.** TD exposes this via `/api/feed/{id}`. Not asked for yet; defer.
- **Rich HTML body** (`IsRichHtml: true`). All comments are plain text in Phase 2.
- **Bulk fetch of replies for a project feed.** The list endpoint returns top-level entries only; replies live under `/api/feed/{id}` and require a per-entry fetch. Phase 2 surfaces `RepliesCount` in human output and JSON, but does not fetch replies.

## Behavior

### Commands

```bash
# Read feed
tdx project feed <project-id> [--limit N] [--json] [--profile P]
tdx project task feed <project-id> <task-id> --plan <plan-id> [--limit N] [--json] [--profile P]

# Post comment (mutating; --yes required)
tdx project comment <project-id> \
  --message "..." [--notify me|UID|email]... [--private] \
  --yes [--profile P]

tdx project task comment <project-id> <task-id> --plan <plan-id> \
  --message "..." [--notify me|UID|email]... [--private] \
  --yes [--profile P]
```

### Feed rendering (human)

```
ID        DATE              BY                 TYPE     BODY
1782210   2026-05-07 18:35  Pat Manager        system   Changed Portfolio(s) from "" to "Sample Portfolio".
1782180   2026-05-06 14:12  Sample User        comment  Backup config review complete — all checks passed.
```

`TYPE` is `comment` for `UpdateType==1`, else `system`. Body is truncated at 80 chars in human output; full body shown via `--json`.

### JSON envelope

```json
{
  "schema": "tdx.v1.projectFeed",
  "projectID": 9001,
  "entries": [
    {
      "id": 1782210,
      "createdByUID": "aaaa...",
      "createdByName": "Pat Manager",
      "createdDate": "2026-05-07T18:35:38.877Z",
      "updateType": "system",
      "body": "Changed Portfolio(s) from ...",
      "isPrivate": false,
      "likesCount": 0,
      "repliesCount": 0
    }
  ]
}
```

Task feed envelope schema is `tdx.v1.projectTaskFeed` and adds `planID` + `taskID` to the top-level object.

### Comment success line

```
posted comment on project 259 (feed entry 1782250)
posted comment on task #4938 / project 259 / plan 1292 (feed entry 1782260)
```

## MCP tools

| Tool | Type | Inputs |
|---|---|---|
| `get_project_feed` | read | `profile?`, `projectID`, `limit?` |
| `add_project_comment` | mutating, `confirm:true` | `profile?`, `projectID`, `message`, `notifyUIDs?`, `isPrivate?`, `confirm` |
| `get_project_task_feed` | read | `profile?`, `projectID`, `planID`, `taskID`, `limit?` |
| `add_project_task_comment` | mutating, `confirm:true` | `profile?`, `projectID`, `planID`, `taskID`, `message`, `notifyUIDs?`, `isPrivate?`, `confirm` |

Tool count: 70 → 74.

## Wire format

All confirmed by live probe on 2026-05-12.

- `GET /api/projects/{id}/feed` — returns array of feed entries. Fields used: `ID`, `Body`, `CreatedUid`, `CreatedFullName`, `CreatedDate`, `LastUpdatedDate`, `UpdateType`, `IsPrivate`, `LikesCount`, `RepliesCount`. Extra fields ignored. `UID`/`FullName` are null on system events.
- `POST /api/projects/{id}/feed` — body `{Comments: "...", Notify: ["uid", ...], IsPrivate: bool}`. Returns the created entry.
- `GET /api/projects/{p}/plans/{pl}/tasks/{t}/feed` — same row shape; verified empty on a task with zero comments.
- `POST /api/projects/{p}/plans/{pl}/tasks/{t}/feed` — same body shape as project feed POST.

## Domain types

`internal/domain/project_feed.go` (new):

```go
type ProjectFeedEntry struct {
    ID              int       `json:"id"`
    Body            string    `json:"body"`
    CreatedByUID    string    `json:"createdByUID,omitempty"`
    CreatedByName   string    `json:"createdByName,omitempty"`
    CreatedDate     time.Time `json:"createdDate,omitempty"`
    LastUpdatedDate time.Time `json:"lastUpdatedDate,omitempty"`
    UpdateType      int       `json:"updateTypeID,omitempty"` // 1=comment, 3=system
    IsPrivate       bool      `json:"isPrivate,omitempty"`
    LikesCount      int       `json:"likesCount,omitempty"`
    RepliesCount    int       `json:"repliesCount,omitempty"`
}

// UpdateTypeLabel returns "comment" for 1, "system" otherwise.
func (e ProjectFeedEntry) UpdateTypeLabel() string {
    if e.UpdateType == 1 {
        return "comment"
    }
    return "system"
}
```

## Service layer

`internal/svc/projectsvc/feed.go` (new):

```go
func (s *Service) GetFeed(ctx context.Context, profile string, projectID int) ([]domain.ProjectFeedEntry, error)
func (s *Service) AddFeed(ctx context.Context, profile string, projectID int, message string, isPrivate bool, notify []string) (int, error)
func (s *Service) GetTaskFeed(ctx context.Context, profile string, projectID, planID, taskID int) ([]domain.ProjectFeedEntry, error)
func (s *Service) AddTaskFeed(ctx context.Context, profile string, projectID, planID, taskID int, message string, isPrivate bool, notify []string) (int, error)
```

Wire types in `types.go`:

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

## CLI layer

Four new files under `internal/cli/project/`:
- `feed.go` / `feed_test.go` — `tdx project feed <id>`
- `comment.go` / `comment_test.go` — `tdx project comment <id>`
- Extends `task.go` — `tdx project task feed` and `tdx project task comment` (these live next to the existing `task list` / `task show`)

`projectsvcAPI` interface in `project.go` grows the 4 new methods.
`peoplesvcAPI` already covers principal resolution for `--notify`.

## Testing

Mirrors ticket feed/comment tests:
- Stub-based unit tests for each command (rendering, JSON, mutex/required-flag validation, success line)
- httptest-based service-layer tests for each of the 4 methods (decode + body shape)
- One integration-flavored test that the human renderer truncates the Body field at 80 chars
- All existing tests still pass

## Live verification

1. `tdx project feed 9001` returns recent entries; matches what the web UI shows.
2. `tdx project feed 9001 --json | jq '.entries[0]'` — full envelope shape.
3. `tdx project comment 9001 --message "test from tdx (delete me)" --yes` creates an entry; printed ID appears in the next `tdx project feed 9001`.
4. `tdx project task feed 9001 9002 --plan 9101` returns task feed (or empty).
5. `tdx project task comment 9001 9002 --plan 9101 --message "test from tdx (delete me)" --yes` creates a task-feed entry.

Rollback: comments cannot be deleted via the public API on the test tenant (verified during the v0.16.x ticket comment work). The test comments will remain visible on the test project — keep the test message obvious so it's clear it was a CLI smoke test.

## Risks / mitigations

- **`Notify` UID resolution** — same risk as `tdx ticket comment`. Reuse `resolvePrincipal` from `internal/cli/project/helpers.go`.
- **POST returns unexpected shape** — fixture-based service test pins the response shape; live probe will catch divergence.
- **Truncating Body in human view** — make sure newlines in the body don't break table alignment. Replace `\n` with `↵` (single rune) in the rendered cell.

## Definition of done

1. All 4 CLI commands work in human and `--json` mode.
2. All 4 MCP tools register and respond correctly with mocked services.
3. Live verification steps 1-5 above pass on the test tenant.
4. `docs/guide/project.md` updated with sections for each new command.
5. `go test ./... && go vet && gofmt -l . && golangci-lint run ./...` all green.
