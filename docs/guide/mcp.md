# tdx mcp

tdx includes an MCP (Model Context Protocol) server that exposes all functionality to AI agents like Claude.

## Contents

- [tdx mcp serve](#tdx-mcp-serve)
- [Available tools](#available-tools)
- [Safety model](#safety-model)

---

## tdx mcp serve

### Start the server

```bash
tdx mcp serve
```

The server runs over stdio and speaks the MCP protocol. It's designed to be
launched by AI tools, not run manually.

### Configure in your AI tool

Add tdx to your AI tool's MCP configuration:

**Claude Code** (`~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "tdx": {
      "command": "tdx",
      "args": ["mcp", "serve"]
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "tdx": {
      "command": "tdx",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Available tools

The MCP server exposes 57 tools (30 read-only, 27 mutating). All mutating
tools require `confirm: true`. Template applies require an `expectedDiffHash`
from a prior preview call for race protection.

#### Week drafts (Phase A — read-only, 4 tools)

| Tool | Description |
|------|-------------|
| `list_week_drafts` | List local drafts with sync state |
| `get_week_draft` | Load a single draft + sync state |
| `preview_push_week_draft` | Preview push and capture diffHash |
| `diff_week_draft` | Cell-level diff vs remote |

#### Week drafts (Phase A — mutating, 4 tools)

All require `confirm: true`.

| Tool | Description |
|------|-------------|
| `pull_week_draft` | Pull live week into a local draft |
| `update_week_draft` | Apply per-cell edits (hours=0 on pulled cell = delete-on-push) |
| `delete_week_draft` | Delete a draft (auto-snapshots) |
| `push_week_draft` | Push to TD; requires `expectedDiffHash` and `allowDeletes` for any deletes |

#### Week drafts (Phase B.1 — read-only, 1 tool)

| Tool | Description |
|------|-------------|
| `list_week_draft_snapshots` | List snapshots for a draft |

#### Week drafts (Phase B.1 — mutating, 8 tools)

All require `confirm: true`.

| Tool | Description |
|------|-------------|
| `create_week_draft` | Create a draft: blank, template:<n>, or draft:<ref> seed |
| `copy_week_draft` | Clone a draft (src ref) to a new ref (dst ref) |
| `rename_week_draft` | Rename a draft (preserves snapshot history) |
| `reset_week_draft` | Discard local edits and re-pull from remote (auto-snapshots first) |
| `archive_week_draft` | Hide a draft from default list output |
| `unarchive_week_draft` | Show a previously archived draft in list output |
| `snapshot_week_draft` | Take a manual snapshot; optional `--keep` to pin |
| `restore_week_draft_snapshot` | Restore a draft from a snapshot by sequence number |
| `prune_week_draft_snapshots` | Drop unpinned snapshots (by age or to retention cap) |

#### Week drafts (Phase B.2a — mutating, 2 tools)

Requires `confirm: true`.

| Tool | Description |
|------|-------------|
| `refresh_week_draft` | Three-way merge a draft against current remote; supports `strategy` `abort`/`ours`/`theirs`/`surface` |
| `resolve_week_draft` | Pick winners for conflicts surfaced by `refresh strategy=surface` (cell-by-cell or bulk) |

#### Time Reports (Phase C — read-only, 1 tool)

| Tool | Description |
|------|-------------|
| `get_time_status_report` | Generate a per-user, per-week time-status report (selectors: `userUIDs`, `managers`, `accounts`, `resourcePools`, `all`) |

#### People (read-only, 4 tools)

| Tool | Description |
|------|-------------|
| `search_people` | Find people by partial name/email/ID; defaults to staff only (`includeClients: true` to add portal users) |
| `get_person` | Fetch full record for a single user by UID |
| `list_accounts` | List TD accounts/departments, sorted by name |
| `list_resource_pools` | List TD resource pools, sorted by name |

#### Tickets (Phase D — read-only, 9 tools)

| Tool | Description |
|------|-------------|
| `list_ticket_apps` | List ticket apps in the tenant |
| `list_ticket_types` | List ticket types in an app |
| `list_ticket_statuses` | List ticket statuses in an app |
| `list_saved_searches` | List saved searches visible to the user |
| `search_tickets` | Search tickets with filters (returns partial records) |
| `run_saved_search` | Execute a saved search by ID (rate-limited 60/min/IP) |
| `get_ticket` | Get full detail for one ticket |
| `get_ticket_feed` | Read feed entries for a ticket |
| `list_ticket_groups` | List tenant responsibility groups (teams) |

#### Tickets (Phase D — mutating, 4 tools, all require `confirm: true`)

| Tool | Description |
|------|-------------|
| `add_ticket_comment` | Post a feed comment to a ticket |
| `update_ticket_status` | Change a ticket's status (by name or id) |
| `update_ticket_assignee` | Reassign a ticket by responsible UID |
| `log_ticket_time` | Log time worked against a ticket (creates a time entry) |

## Safety model

All mutating tools require `confirm: true` in the request. This ensures the
AI agent explicitly confirms each write operation with the user.

Template applies have an additional safety layer: the agent must first call
`preview_apply_time_template` to get a `diffHash`, then pass that hash to
`apply_time_template_to_week`. If the week changed between preview and
apply, the hash won't match and the apply is rejected.
