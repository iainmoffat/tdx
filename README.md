# tdx

A CLI and MCP server for managing [TeamDynamix](https://www.teamdynamix.com/)
time entries from the terminal. Derive reusable weekly templates, apply them
with safe previews, and expose everything to AI agents via MCP.

For detailed documentation, see the [User Guide](docs/guide.md).

## Install

**Homebrew** (macOS / Linux):

```bash
brew install iainmoffat/tdx/tdx
```

**Go install**:

```bash
go install github.com/iainmoffat/tdx/cmd/tdx@latest
```

**GitHub Releases**: download the binary for your platform from
[Releases](https://github.com/iainmoffat/tdx/releases).

## Quick Start

```bash
# Log in to your TeamDynamix tenant
tdx auth login --url https://yourorg.teamdynamix.com/

# List this week's time entries
tdx time entry list

# Derive a template from a week with known data
tdx time template derive my-week --from-week 2026-04-07

# See the template as a grid
tdx time template show my-week

# Apply it to next week (preview first, then confirm)
tdx time template apply my-week --week 2026-04-14 --yes
```

## Commands

### Auth

| Command | Description |
|---|---|
| `tdx auth login --url <tenant>` | Authenticate with a TD tenant |
| `tdx auth login --sso` | Re-authenticate via SSO |
| `tdx auth status` | Show current user and session |
| `tdx auth logout` | Clear stored credentials |
| `tdx auth profile list` | List configured profiles |
| `tdx auth profile add <name> --url <tenant>` | Add a profile |
| `tdx auth profile remove <name>` | Remove a profile |
| `tdx auth profile use <name>` | Set the default profile |

### Time Entries

| Command | Description | Key Flags |
|---|---|---|
| `tdx time entry list` | List entries (default: this week) | `--week`, `--from`/`--to`, `--ticket`, `--type`, `--json` |
| `tdx time entry show <id>` | Show a single entry | `--json` |
| `tdx time entry add` | Create an entry | `--date`, `--hours`/`--minutes`, `--type`, target flags, `--dry-run` |
| `tdx time entry update <id>` | Update an entry | `--date`, `--hours`, `--type`, `-d`, `--dry-run` |
| `tdx time entry delete <id> [<id>...]` | Delete entries | `--dry-run` |

### Time Week

| Command | Description |
|---|---|
| `tdx time week show [date]` | Show week as a grid (default: this week) |
| `tdx time week locked` | Show locked days |

### Time Week Drafts

Local week drafts let you pull a live week down, edit it as a YAML
artifact, validate, diff, preview, and push back with safety guarantees.

| Command | Description | Key Flags |
|---|---|---|
| `tdx time week pull <date>` | Pull live week into a local draft | `--name`, `--force`, `--json` |
| `tdx time week list` | List local drafts with sync state | `--dirty`, `--conflicted`, `--date`, `--archived`, `--no-remote-check`, `--json` |
| `tdx time week show <date> --draft [name]` | Show a draft as a grid | (flag added to existing `show`) |
| `tdx time week status <date>[/<name>]` | One-screen draft status | `--json`, `--no-remote-check` |
| `tdx time week edit <date>[/<name>]` | Edit a draft as YAML in $EDITOR | (vi fallback) |
| `tdx time week diff <date>[/<name>]` | Diff a draft vs current remote | `--against remote`, `--json` |
| `tdx time week preview <date>[/<name>]` | Preview what `push` will do | `--json` |
| `tdx time week push <date>[/<name>] --yes` | Push a draft to TD | `--allow-deletes`, `--expected-diff-hash`, `--json` |
| `tdx time week delete <date>[/<name>] --yes` | Delete a draft (auto-snapshots first) | `--keep-snapshots` |
| `tdx time week set <date>[/<name>] <row>:<day>=<h>` | Non-interactive cell write | (repeatable) |
| `tdx time week note <date>[/<name>]` | Edit free-form notes | `--append`, `--clear` |
| `tdx time week history <date>[/<name>]` | List snapshots | `--json`, `--limit N` |
| `tdx time week new <date>` | Create blank/template-seeded/cloned draft | `--from-template`, `--from-draft`, `--shift`, `--name` |
| `tdx time week copy <src> <dst>` | Clone a draft to a new ref | (positional) |
| `tdx time week rename <date>[/<old>] <new>` | Rename a draft (preserves snapshots) | (positional) |
| `tdx time week reset <date>[/<name>] --yes` | Discard local edits + re-pull (auto-snapshots) | `--yes` |
| `tdx time week archive <date>[/<name>]` | Hide draft from default `list` | (none) |
| `tdx time week unarchive <date>[/<name>]` | Show previously archived draft | (none) |
| `tdx time week snapshot <date>[/<name>]` | Take a manual snapshot | `--keep`, `--note` |
| `tdx time week restore <date>[/<name>] --snapshot N --yes` | Restore from snapshot | `--snapshot`, `--yes` |
| `tdx time week prune <date>[/<name>] --yes` | Drop unpinned snapshots | `--older-than`, `--yes` |

### Time Types

| Command | Description |
|---|---|
| `tdx time type list` | List all time types |
| `tdx time type for <kind> <id>` | Show valid types for a work item |

### Templates

| Command | Description | Key Flags |
|---|---|---|
| `tdx time template derive <name>` | Create template from a live week | `--from-week` |
| `tdx time template list` | List saved templates | `--json` |
| `tdx time template show <name>` | Show template as a grid | `--json` |
| `tdx time template edit <name>` | Edit template hours in a grid editor | `--web` |
| `tdx time template clone <name> <new>` | Copy a template | |
| `tdx time template delete <name>` | Delete a template | |
| `tdx time template apply <name>` | Apply template to a week | `--week`, `--mode`, `--days`, `--dry-run`, `--yes` |
| `tdx time template compare <name>` | Compare template vs live week | `--week`, `--mode` |

### MCP

| Command | Description |
|---|---|
| `tdx mcp serve` | Start the MCP server over stdio |

### Other

| Command | Description |
|---|---|
| `tdx version` | Print version |
| `tdx config show` | Show configuration |
| `tdx completion bash\|zsh\|fish` | Generate shell completions |

## MCP Integration

Add tdx as an MCP server in your AI tool's configuration:

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

The MCP server exposes 37 tools (17 read-only, 20 mutating). All mutating
tools require `confirm: true`. Template applies require an `expectedDiffHash`
from a prior preview call for race protection.

**Week drafts (Phase A — read-only, 4 tools):**

| Tool | Description |
|------|-------------|
| `list_week_drafts` | List local drafts with sync state |
| `get_week_draft` | Load a single draft + sync state |
| `preview_push_week_draft` | Preview push and capture diffHash |
| `diff_week_draft` | Cell-level diff vs remote |

**Week drafts (Phase A — mutating, 4 tools, all require `confirm: true`):**

| Tool | Description |
|------|-------------|
| `pull_week_draft` | Pull live week into a local draft |
| `update_week_draft` | Apply per-cell edits (hours=0 on pulled cell = delete-on-push) |
| `delete_week_draft` | Delete a draft (auto-snapshots) |
| `push_week_draft` | Push to TD; requires `expectedDiffHash` and `allowDeletes` for any deletes |

**Week drafts (Phase B.1 — read-only, 1 tool):**

| Tool | Description |
|------|-------------|
| `list_week_draft_snapshots` | List snapshots for a draft |

**Week drafts (Phase B.1 — mutating, 8 tools, all require `confirm: true`):**

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

## JSON Output

All commands support `--json` for machine-readable output with stable
`tdx.v1.*` schema envelopes. JSON is auto-detected when stdout is not a TTY:

```bash
tdx time entry list | jq '.entries[].id'
```

Schema names introduced in Phase A: `tdx.v1.weekDraft`, `tdx.v1.weekDraftList`,
`tdx.v1.weekDraftStatus`, `tdx.v1.weekDraftDiff`, `tdx.v1.weekDraftPreview`,
`tdx.v1.weekDraftPullResult`, `tdx.v1.weekDraftPushResult`,
`tdx.v1.weekDraftSnapshotList`.

Schema names introduced in Phase B.1: `tdx.v1.weekDraftCreateResult`,
`tdx.v1.weekDraftCopyResult`, `tdx.v1.weekDraftRenameResult`,
`tdx.v1.weekDraftArchiveResult`, `tdx.v1.weekDraftSnapshot`,
`tdx.v1.weekDraftSnapshotPruneResult`.

## Shell Completions

```bash
# bash
echo 'eval "$(tdx completion bash)"' >> ~/.bashrc

# zsh
tdx completion zsh > "${fpath[1]}/_tdx"

# fish
tdx completion fish | source
```

## Configuration

tdx stores configuration in `~/.config/tdx/`:

| Path | Contents |
|------|----------|
| `config.yaml` | Profiles and default profile |
| `credentials.yaml` | Authentication tokens (per profile) |
| `templates/` | Legacy templates (migrated to per-profile on upgrade) |
| `profiles/<profile>/templates/` | Per-profile templates |
| `profiles/<profile>/weeks/<YYYY-MM-DD>/<name>.yaml` | Local week drafts |
| `profiles/<profile>/weeks/<YYYY-MM-DD>/<name>.snapshots/` | Per-draft auto-snapshots |

## Development

```bash
make build      # build binary
make test       # run tests
make lint       # run linters (requires golangci-lint)
make coverage   # test coverage report
make all        # fmt + vet + lint + test + build
```

## License

MIT
