# tdx User Guide

This guide covers all tdx features in depth. For a quick reference of
commands and flags, see the [README](../README.md).

---

## Table of Contents

- [Command tree](#command-tree)
- [Reference](#reference)
- [Concepts](#concepts)
- [Storage Layout](#storage-layout)
- [JSON Output](#json-output)
- [Shell Completions](#shell-completions)
- [Configuration](#configuration)

---

## Command tree

```text
tdx
├── auth
│   ├── login / logout / status
│   └── profile          → list / add / remove / use
├── people
│   ├── search / show
│   ├── accounts         → list
│   └── pools            → list
├── ticket
│   ├── app              → list / use / show
│   ├── search           → saved
│   ├── show / feed
│   ├── comment / status / assign / update / log
│   ├── types / statuses / groups → list
│   └── task             → list / show / feed / update / log
├── project
│   ├── list / search / show
│   ├── plan             → list
│   ├── task             → list (--mine) / show
│   └── log
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

## Reference

- [tdx auth](guide/auth.md) — authenticate and manage profiles
- [tdx time](guide/time.md) — time entries, week drafts, templates, reports
- [tdx people](guide/people.md) — find users, accounts, resource pools
- [tdx ticket](guide/ticket.md) — search, show, comment, status/assign, log time
- [tdx project](guide/project.md) — list/search projects, plans, tasks (incl. tasks assigned to you), log time
- [tdx mcp](guide/mcp.md) — MCP server for AI agents

---

## Concepts

tdx manages **time entries** in [TeamDynamix](https://www.teamdynamix.com/).
Each entry records hours worked on a specific date against a **target**
(ticket, project, or workspace) with a **time type** (e.g. "Development",
"Planning").

A **week** in TD runs Sunday through Saturday. All dates are interpreted in
Eastern time (America/New_York), matching how TD computes billing periods.

A **template** is a saved weekly pattern of time entries that can be applied
to any future week. Templates are the core productivity feature of tdx:
derive one from a representative week, then replay it whenever you need to.

---

## Storage Layout

Week drafts and templates live under per-profile directories:

```
~/.config/tdx/
├── config.yaml
├── credentials.yaml
└── profiles/
    └── <profile>/
        ├── templates/          # per-profile templates (Phase A migration)
        └── weeks/
            └── <YYYY-MM-DD>/
                ├── default.yaml             # the draft
                ├── default.pulled.yaml      # at-pull-time snapshot
                └── default.snapshots/       # auto-history
                    ├── 0001-pre-pull-...yaml
                    └── 0002-pre-push-...yaml
```

On first run after upgrading from a pre-Phase-A version, tdx detects any
templates in the legacy `~/.config/tdx/templates/` directory and offers to
migrate them into the active profile. Single-profile users see the
migration run silently; multi-profile users get a one-time prompt naming
the target profile.

---

## JSON Output

All commands support `--json` for machine-readable output:

```bash
tdx time entry list --json
tdx time entry list --json | jq '.entries[].id'
```

You can also set the `TDX_FORMAT` environment variable:

```bash
export TDX_FORMAT=json
tdx time entry list   # outputs JSON without --json flag
```

### Schema envelopes

JSON output uses stable `tdx.v1.*` schemas. Every response has a top-level
`"schema"` field:

```json
{
  "schema": "tdx.v1.entryList",
  "filter": { "from": "2026-04-05", "to": "2026-04-11" },
  "totalHours": 40.0,
  "totalMinutes": 2400,
  "entries": [...]
}
```

Schema names include: `tdx.v1.entryList`, `tdx.v1.entryAdd`,
`tdx.v1.weekReport`, `tdx.v1.timeTypes`, `tdx.v1.template`,
`tdx.v1.templateList`, `tdx.v1.templateDerive`,
`tdx.v1.templateApplyPreview`, `tdx.v1.timeTypesForTarget`.

---

## Shell Completions

### Bash

```bash
echo 'eval "$(tdx completion bash)"' >> ~/.bashrc
```

### Zsh

```bash
tdx completion zsh > "${fpath[1]}/_tdx"
```

### Fish

```bash
tdx completion fish | source
```

---

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

Override the config directory with `TDX_CONFIG_HOME`:

```bash
export TDX_CONFIG_HOME=/path/to/custom/config
```
