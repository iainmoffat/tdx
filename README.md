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

# Weekly time-status report for your direct reports
tdx time report status --manager me --week 2026-04-12

# Or for a TD resource pool (matches the TD UI's filter)
tdx time report status --resource-pool "ICT - DBP - Linux Platform Services LPS" --week 2026-04-12

# Direct reports who haven't logged a full week
tdx time report status --manager me --week 2026-04-19 --incomplete

# Multiple managers (union of direct reports)
tdx time report status --manager me --manager other-uid --week 2026-04-19

# Multiple resource pools
tdx time report status --resource-pool "Pool A" --resource-pool "Pool B" --week 2026-04-19
```

## Commands

```text
tdx
├── auth
│   ├── login / logout / status
│   └── profile          → list / add / remove / use
├── people
│   ├── search / show
│   ├── accounts         → list
│   └── pools            → list
├── time
│   ├── entry            → list / show / add / update / delete
│   ├── template         → derive / list / show / edit / clone / delete / apply / compare
│   ├── type             → list / for
│   ├── week             → show / locked / pull / list / status / edit / diff / preview / push / delete / set / note / history / new / copy / rename / reset / refresh / rebase / resolve / archive / unarchive / snapshot / restore / prune
│   └── report           → status
├── mcp                  → serve
├── config               → path / init / show
├── completion           → bash / zsh / fish
└── version
```

Full reference: [User Guide](docs/guide.md). The guide is split per top-level command:

- [tdx auth](docs/guide/auth.md)
- [tdx time](docs/guide/time.md)
- [tdx people](docs/guide/people.md)
- [tdx mcp](docs/guide/mcp.md)

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

The MCP server exposes 44 tools (22 read-only, 22 mutating). All mutating
tools require `confirm: true`. Template applies require an `expectedDiffHash`
from a prior preview call for race protection. See [tdx mcp](docs/guide/mcp.md)
for the full tool catalog and safety model.

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

Schema names introduced in Phase B.2a: `tdx.v1.weekDraftRefreshResult`.

Schema names introduced in Phase C — Reports: `tdx.v1.timeStatusReport`.

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
