# tdx Phase 6 — Polish, Docs, Packaging Design Spec

**Goal:** Make tdx ship-ready for public use: tenant-agnostic defaults, README with full command reference, CI pipeline, goreleaser for cross-platform binaries + Homebrew tap, shell completions, test coverage reporting, and a recorded demo.

**Spec basis:** Framework spec §11 (Phase 6), §9 (Output strategy).

**Milestone:** A newcomer installs and uses the tool in under five minutes.

---

## 1. Scope

### In scope

| Deliverable | Detail |
|---|---|
| Tenant abstraction | Remove UFL-specific defaults from walkthrough, audit SSO for generality, clean hardcoded references |
| README.md | Install, quick start, command reference, MCP setup, JSON schemas, development |
| Makefile | build, test, lint, vet, fmt, clean, coverage, all |
| Linting config | `.golangci.yml` with conservative defaults |
| CI workflow | `.github/workflows/ci.yml` — test, vet, lint, build on push + PR |
| Goreleaser | `.goreleaser.yaml` — cross-platform binaries, GitHub Releases, Homebrew tap |
| Release workflow | `.github/workflows/release.yml` — triggered on `v*` tags |
| Shell completions | `tdx completion bash/zsh/fish` via Cobra's built-in generators |
| Test coverage | `make coverage` target, reconciler edge case review |
| Asciinema demo | `scripts/demo.sh` script + `demo/` directory for recorded cast |

### Out of scope

| Item | Reason |
|---|---|
| npm wrapper | Low priority; `brew install` and `go install` cover the target audience |
| Man pages | Cobra can generate them but they're low-value for a modern CLI; README suffices |
| Docker image | Not needed for a single-binary CLI tool |
| Coverage threshold gate | Informational reporting only; no blocking threshold in CI |

---

## 2. Tenant Abstraction

### Current UFL-specific items

1. **`scripts/walkthrough.sh`** — defaults to `https://ufl.teamdynamix.com/`, project ID 54, plan ID 2091, task ID 2612
2. **SSO login** (`--sso` flag) — calls `/TDWebApi/api/auth/loginsso` which is a standard TD endpoint, not UFL-specific. The flag itself is generic.
3. **Code comments** — references to "UFL tenant" in spec docs and some source comments
4. **Walkthrough probing notes** — memory entries reference UFL-specific field discoveries

### Changes

1. **Walkthrough defaults:** Remove all default values. The walkthrough requires env vars to be set explicitly:
   ```bash
   TDX_WALKTHROUGH_TOKEN   # required — no default
   TDX_WALKTHROUGH_URL     # required — no default (was https://ufl.teamdynamix.com/)
   TDX_WALKTHROUGH_WEEK    # required — no default (was 2026-04-01)
   TDX_WALKTHROUGH_PROJECT # required for Phase 3+ steps
   TDX_WALKTHROUGH_PLAN    # required for Phase 3+ steps  
   TDX_WALKTHROUGH_TASK    # required for Phase 3+ steps
   ```
   The script errors clearly if any required var is missing.

2. **Comments cleanup:** Replace "UFL" references in source code comments with generic "TD tenant" language. Spec/plan docs are historical — leave them as-is.

3. **SSO login:** Already generic (`/TDWebApi/api/auth/loginsso` is standard TD). No changes needed.

---

## 3. README

Single `README.md` at the repo root. Sections:

### 3.1 Header

One-liner description, install options (brew, go install, GitHub releases).

### 3.2 Quick Start

Five commands showing the core workflow:
```bash
tdx auth login --url https://yourorg.teamdynamix.com/
tdx time entry list
tdx time template derive my-week --from-week 2026-04-07
tdx time template show my-week
tdx time template apply my-week --week 2026-04-14 --yes
```

### 3.3 Command Reference

Table grouped by domain:

| Group | Commands |
|---|---|
| Auth | `login`, `status`, `logout`, `profile list/add/remove/use` |
| Time Entry | `list`, `show`, `add`, `update`, `delete` |
| Time Week | `show`, `locked` |
| Time Type | `list`, `show`, `for` |
| Time Template | `derive`, `list`, `show`, `edit`, `clone`, `delete`, `apply`, `compare` |
| MCP | `serve` |
| Config | `show` |
| Completion | `bash`, `zsh`, `fish` |

Each row: command, short description, key flags.

### 3.4 MCP Integration

How to add tdx as an MCP server in Claude Code, Cursor, etc.:
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

Brief description of the safety model (confirm:true, expectedDiffHash).

### 3.5 JSON Output

Note that all commands support `--json` with stable `tdx.v1.*` schemas. Auto-detected when piped (`tdx time entry list | jq .`).

### 3.6 Development

```bash
make build    # build binary
make test     # run tests
make lint     # run linters
make coverage # coverage report
```

---

## 4. Makefile

```makefile
.PHONY: all build test vet fmt lint clean coverage

all: fmt vet lint test build

build:
	go build -o tdx ./cmd/tdx

test:
	go test ./... -count=1

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint:
	golangci-lint run ./...

clean:
	rm -f tdx coverage.out

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
```

---

## 5. Linting Config

`.golangci.yml`:

```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - gofmt

linters-settings:
  errcheck:
    check-type-assertions: true

issues:
  exclude-use-default: true
```

Conservative set — no exotic linters that produce false positives. `errcheck` with type assertion checking catches the most common Go bugs.

---

## 6. CI Workflow

`.github/workflows/ci.yml`:

```yaml
name: CI
on:
  push:
    branches: ['*']
  pull_request:

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v6
      - run: go test ./... -count=1 -race
      - run: go build ./cmd/tdx
```

Single job, sequential steps. Triggered on push to any branch and on PRs.

---

## 7. Goreleaser + Homebrew

### 7.1 `.goreleaser.yaml`

```yaml
version: 2
builds:
  - main: ./cmd/tdx
    binary: tdx
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64

archives:
  - format: tar.gz
    name_template: "tdx_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

brews:
  - repository:
      owner: iainmoffat
      name: homebrew-tdx
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: https://github.com/iainmoffat/tdx
    description: CLI and MCP server for managing TeamDynamix time entries
    install: |
      bin.install "tdx"

checksum:
  name_template: checksums.txt

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
```

### 7.2 `.github/workflows/release.yml`

```yaml
name: Release
on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

### 7.3 Prerequisites (manual, outside code)

1. Create `iainmoffat/homebrew-tdx` repo on GitHub (empty)
2. Create a PAT with `repo` scope for pushing to the tap
3. Add `HOMEBREW_TAP_TOKEN` as a secret on the `tdx` repo

---

## 8. Shell Completions

New `tdx completion` command using Cobra's built-in generators:

```
tdx completion bash   → shell completion script for bash
tdx completion zsh    → shell completion script for zsh
tdx completion fish   → shell completion script for fish
```

Implemented via `cobra.Command.GenBashCompletionV2()`, `GenZshCompletion()`, `GenFishCompletion()`. No custom completion logic — Cobra generates completions from the command tree automatically.

README documents installation:
```bash
# bash
echo 'eval "$(tdx completion bash)"' >> ~/.bashrc

# zsh
tdx completion zsh > "${fpath[1]}/_tdx"

# fish
tdx completion fish | source
```

---

## 9. Test Coverage

### 9.1 `make coverage` target

Runs `go test -coverprofile=coverage.out ./...` then prints per-function coverage via `go tool cover -func`. Informational only — no CI gate.

### 9.2 Reconciler edge cases

Review `tmplsvc/reconcile_test.go` for gaps. The Phase 4 code review identified:
- Rounding behavior (non-integer minutes with `Round: true` vs `false`)
- These were already addressed during Phase 4 implementation

Additional edge cases to test if not covered:
- Template with zero-hour rows (should be skipped, not create 0-minute entries)
- Override that sets hours to 0 (should skip the cell)
- Multiple entries matching the same key (first match wins)

### 9.3 Coverage report in CI

Optional: add a coverage step to CI that uploads `coverage.out` as an artifact. No threshold enforcement.

---

## 10. Asciinema Demo

### 10.1 Demo script

`scripts/demo.sh` — a scripted walkthrough designed to be run under `asciinema rec`:

```bash
#!/usr/bin/env bash
# Run: asciinema rec demo/demo.cast -c "bash scripts/demo.sh"
```

Shows:
1. `tdx version`
2. `tdx auth status` (assumes already logged in)
3. `tdx time entry list --week 2026-04-07`
4. `tdx time template derive my-week --from-week 2026-04-07`
5. `tdx time template show my-week`
6. `tdx time template apply my-week --week 2026-04-14 --dry-run`
7. Cleanup: `tdx time template delete my-week`

### 10.2 Cast file

`demo/demo.cast` — checked into the repo. The README links to it (either hosted on asciinema.org or rendered via an embedded player).

Note: The cast file is manually recorded against a real tenant. The `scripts/demo.sh` provides the commands; the recording captures actual output.

---

## 11. Build Order

| Step | What |
|---|---|
| 1 | Tenant abstraction — walkthrough defaults, comment cleanup |
| 2 | README.md |
| 3 | Makefile |
| 4 | `.golangci.yml` + lint fixes |
| 5 | `.github/workflows/ci.yml` |
| 6 | `.goreleaser.yaml` + `.github/workflows/release.yml` |
| 7 | Shell completions (`tdx completion`) |
| 8 | Test coverage target + reconciler edge cases |
| 9 | Asciinema demo script + placeholder |

---

## 12. Decision Log

| # | Decision | Rationale |
|---|---|---|
| D1 | Full Phase 6 scope | User chose comprehensive ship-readiness |
| D2 | GitHub Releases + Homebrew tap | Standard install experience for macOS/Linux; goreleaser automates the tap |
| D3 | golangci-lint with conservative defaults | Catches real bugs without false positive noise |
| D4 | Single CI job (not parallel) | Repo is small; sequential steps finish in ~30s |
| D5 | No coverage threshold gate | Coverage is informational; enforced thresholds create perverse incentives |
| D6 | Tenant abstraction first | Must happen before README so docs describe a generic tool |
| D7 | Walkthrough requires explicit env vars | No defaults — prevents accidentally running against wrong tenant |
| D8 | Asciinema demo manually recorded | Automated demos look robotic; manual recording captures natural pacing |
