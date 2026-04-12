# tdx

A CLI and MCP server for managing [TeamDynamix](https://www.teamdynamix.com/)
time entries from the terminal. Derive reusable weekly templates, apply them
with safe previews, and expose everything to AI agents via MCP.

## Install

**Homebrew** (macOS / Linux):

```bash
brew install iainmoffat/tdx/tdx
```

**Go install**:

```bash
go install github.com/ipm/tdx/cmd/tdx@latest
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
| `tdx time template edit <name>` | Edit template YAML in $EDITOR | |
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

The MCP server exposes 20 tools (12 read-only, 8 mutating). All mutating
tools require `confirm: true`. Template applies require an `expectedDiffHash`
from a prior preview call for race protection.

## JSON Output

All commands support `--json` for machine-readable output with stable
`tdx.v1.*` schema envelopes. JSON is auto-detected when stdout is not a TTY:

```bash
tdx time entry list | jq '.entries[].id'
```

## Shell Completions

```bash
# bash
echo 'eval "$(tdx completion bash)"' >> ~/.bashrc

# zsh
tdx completion zsh > "${fpath[1]}/_tdx"

# fish
tdx completion fish | source
```

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
