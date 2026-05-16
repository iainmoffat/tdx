# Security Hardening — Artifact Name Validation

**Date:** 2026-05-16
**Phase:** 2 of 7 (security hardening rollup; see [Phase 1 spec](2026-05-15-report-fanout-caps.md) for the full roadmap)
**Goal:** Reject path-traversal in user-controlled artifact names (templates, drafts, profiles) before any filesystem operation. Single shared helper applied at every name-entry boundary (CLI flags, MCP inputs, service-store methods).

## Motivation

A security audit on 2026-05-15 identified path-traversal as the highest-severity finding (`#1: High`). User-controlled template and draft names are concatenated directly into filesystem paths via `filepath.Join(dir, name+".yaml")`. Names containing `../` can escape the intended config subtree and read, overwrite, or delete arbitrary `*.yaml` paths reachable by the current user.

The MCP surface makes this exploitable not just by typos but by LLM-driven inputs. A user asking the model to manage time entries could be coerced into a tool call with `name: "../../credentials"` and overwrite or read sensitive state.

Profile names share the same risk via `paths.ProfileTemplatesDir(profile)` and `paths.ProfileWeeksDir(profile)` — existing `Profile.Validate()` rejects slashes and whitespace but not `..`.

## Threat model

- **Local user attacks themselves via accident.** Typo like `tdx time template show ./foo` produces nothing dangerous today, but `tdx time week edit 2026-04-12/../../something` could clobber a sibling profile's draft.
- **LLM-driven coercion via MCP.** A prompt-injected or confused LLM emits `args.Name: "../../credentials"` to a template tool. With no validation, the tool reads or writes `credentials.yaml` (containing the bearer token).
- **Out of scope:** other local users on the same box, kernel-level attacks, container escape.

## Scope

In scope (audit finding #1):

- Template names — `internal/svc/tmplsvc/store.go`, MCP tools at `internal/mcp/tools_tmpl.go` and `internal/mcp/tools_apply.go`, CLI commands under `internal/cli/time/template/`
- Draft names — `internal/svc/draftsvc/store.go`, `internal/svc/draftsvc/rename.go`, MCP `parseDraftRefMCP` in `internal/mcp/tools_drafts.go`, CLI `ParseDraftRef` / `ResolveWeekRef` in `internal/cli/time/week/draft.go`
- Profile names — `internal/domain/profile.go` `Profile.Validate()` upgrade, all `ProfileStore` write methods

Out of scope (later phases per [Phase 1 roadmap](2026-05-15-report-fanout-caps.md#roadmap)):

- Web editor session protection (Phase 3)
- Profile revalidation on load (Phase 4)
- Docs token-handling snippet (Phase 5)
- CI/release hardening (Phase 6)
- OS keychain token storage (Phase 7)

## Policy

**Defense in depth — two layers, applied independently:**

1. **Boundary check** at CLI argument parsing and MCP input decoding. Gives a friendly error before any service call.
2. **Store-level check** in `tmplsvc.Store` and `draftsvc.Store` and `ProfileStore`. Backstop in case a new caller forgets the boundary check. The store-level check is the security guarantee; the boundary check is the UX layer.

A reject at either layer wraps the same `domain.ErrInvalidArtifactName` sentinel, so downstream callers see a consistent error shape regardless of where the check fired.

## The validation rule

A single function `domain.ValidateArtifactName(name string) error` returns nil if valid, else a wrapped `domain.ErrInvalidArtifactName` with a specific reason.

**Accepts:** `^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}$`. In words:

- Length: 1–64 characters total.
- First character: ASCII letter, digit, or underscore.
- Remaining characters: ASCII letter, digit, `.`, `_`, or `-`.

**Rejects** (each with a distinct, layered error message):

| Input shape | Reason | Message fragment |
|---|---|---|
| `""` | empty | `name is required` |
| length > 64 | too long | `name exceeds 64 characters` |
| Leading `.` (incl. `.`, `..`, `.hidden`) | hidden / traversal | `name may not start with '.'` |
| Leading `-` (e.g., `-foo`) | CLI-flag confusion | `name may not start with '-'` |
| Any char outside `[A-Za-z0-9._-]` (slashes, NUL, control, whitespace, unicode, etc.) | invalid char | `name contains invalid character %q at position %d` |
| Name matches a Windows-reserved word, case-insensitive, evaluated on the substring before the first `.` (so `CON`, `con`, and `CON.foo` all reject; `COM10` doesn't). Reserved list: `CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9` | reserved | `%q is a reserved name` |

The regex is the authoritative gate; the reserved-word check is layered on top so the error message can say *why* a CON-like name fails. Reserved-word check applies on all platforms — it costs nothing and keeps cross-platform behavior consistent.

**Existing on-disk names verified compatible.** A scan of `~/.config/tdx/profiles/default/{templates,weeks}/` on 2026-05-16 showed every existing name fits the strict allowlist: templates `my-week`, `my-week2`, `my-week3`, `my-week4`; drafts `default`; snapshot files like `0001-pre-push-20260429T173319Z`. No migration needed.

## Helper signature

```go
// internal/domain/artifact.go (new)
package domain

import "errors"

// ErrInvalidArtifactName indicates a template/draft/profile name failed
// validation. Wrap with the specific reason for the user.
var ErrInvalidArtifactName = errors.New("invalid_artifact_name")

// ValidateArtifactName returns nil if name is a safe filesystem component
// for use as a template, draft, or profile name. See spec for the rule.
func ValidateArtifactName(name string) error { ... }
```

Lives alongside other domain sentinels and validators (`Profile.Validate`, `ErrFanoutLimitExceeded`, `ErrPermission`). No new dependencies.

Helper is platform-agnostic — does not call `filepath.IsAbs` or any OS-specific function. The regex + reserved-list approach catches every traversal vector without branching by OS.

## Call sites

### A. Store-level (security guarantee)

Every public method that takes a name calls `ValidateArtifactName` first.

| Store | Methods that need the check | Notes |
|---|---|---|
| `internal/svc/tmplsvc/store.go` | `Save`, `Load`, `Delete`, `Exists` | `List` enumerates from disk; per-entry `Load` does the check |
| `internal/svc/draftsvc/store.go` | `Save`, `Load`, `Delete`, `Exists`, `SaveNew`, `SavePulledSnapshot`, `LoadPulledSnapshot` | All take `name`; `pulledSnapshotPath` derives `name+".pulled"` from validated name |
| `internal/svc/draftsvc/rename.go` | `Rename` | Validates both `oldName` and `newName` |
| `internal/config/profiles.go` | `AddProfile`, `UpdateProfile`, `RemoveProfile`, `GetProfile`, `SetDefault` | All paths through `Profile.Validate()` or a direct `ValidateArtifactName` call |

For `Save` on templates and drafts, the name lives on the struct (`tmpl.Name`, `d.Name`). `Template.Validate()` and `WeekDraft.Validate()` already exist — they add a `ValidateArtifactName(t.Name)` call.

For `Profile.Validate()`, replace the existing `strings.ContainsAny("/\\ \t")` check with a call to `ValidateArtifactName`. The new check is strictly stronger than the old one.

For `List` methods: they enumerate files on disk and call `Load(name)` per entry. `Load`'s validation will reject any file whose name doesn't validate. Log a warning to stderr and skip such entries rather than returning an error — preserves graceful degradation if any legacy file slips in. (Not expected on this user's box; defensive only.)

### B. Boundary-level (UX layer)

Both surfaces produce the same `ErrInvalidArtifactName`-wrapped error so downstream callers see a consistent shape.

**CLI:**
- Validate inside `ParseDraftRef` (`internal/cli/time/week/draft.go`) immediately after the slash-split.
- Validate in each template-name flag parser in `internal/cli/time/template/*.go`.
- For profile, validate the `--profile <name>` value in `authsvc.ResolveProfile` (or a wrapper that calls it).

**MCP:**
- Validate in each handler that takes `args.Name` (`tools_tmpl.go`, `tools_apply.go`).
- Validate inside `parseDraftRefMCP` (`tools_drafts.go`).

## Error shape

Follows the pattern set by `ErrFanoutLimitExceeded` from Phase 1.

**Sentinel:** `domain.ErrInvalidArtifactName = errors.New("invalid_artifact_name")`

**Wrapped at every reject site** with the specific reason:

```go
return fmt.Errorf("%w: name is required", domain.ErrInvalidArtifactName)
return fmt.Errorf("%w: name exceeds 64 characters (got %d)", domain.ErrInvalidArtifactName, len(name))
return fmt.Errorf("%w: name may not start with %q", domain.ErrInvalidArtifactName, ".")
return fmt.Errorf("%w: name may not start with %q", domain.ErrInvalidArtifactName, "-")
return fmt.Errorf("%w: name contains invalid character %q at position %d", domain.ErrInvalidArtifactName, r, i)
return fmt.Errorf("%w: %q is a reserved name", domain.ErrInvalidArtifactName, name)
```

**CLI rendering:**
```
$ tdx time template show ../../credentials
Error: invalid_artifact_name: name contains invalid character '/' at position 2
```

**MCP rendering:** `errorResult(err.Error())` — same string, parseable by an LLM via the `invalid_artifact_name:` prefix.

**Tests assert** via `errors.Is(err, domain.ErrInvalidArtifactName)` plus `Contains` on the specific reason fragment.

## Tests

### Unit tests on `ValidateArtifactName`

`internal/domain/artifact_test.go` (new). Table-driven `{name, wantOK, wantReason}`.

Accept cases (must return nil):

- `default`, `my-week`, `my-week2`, `My_Week.draft`
- `a` (1 char)
- 64-char alphanumeric string

Reject cases (must wrap sentinel, must contain stated fragment):

- `""` → `name is required`
- `..` → `may not start with`
- `.` → `may not start with`
- `.hidden` → `may not start with`
- `-flag` → `may not start with`
- `../../credentials` → `invalid character` (the `/` rejects before the leading-`.` rule)
- `/etc/passwd` → `invalid character`
- `foo/bar` → `invalid character`
- `foo\bar` → `invalid character`
- `foo bar` → `invalid character` (space)
- `foo\tbar` → `invalid character` (tab)
- `naïve` → `invalid character` (unicode)
- `foo\x00bar` → `invalid character` (NUL)
- 65-char string → `exceeds 64 characters`
- `CON`, `con`, `COM1`, `LPT9`, `NUL`, `PRN`, `AUX`, `CON.txt`, `nul.foo` → `reserved name`
- `COM10`, `LPT10`, `CONsole` (no leading-dot truncation) → must PASS (not reserved per the prefix rule)

### Integration tests at the store layer

For each store, add ONE test that drives the full reject path:

- `tmplsvc.Store.Load(profile, "../../credentials")` → wrapped sentinel.
- `draftsvc.Store.Save(d)` where `d.Name = "../../foo"` → wrapped sentinel via `WeekDraft.Validate()`.
- `config.ProfileStore.AddProfile(domain.Profile{Name: ".."})` → wrapped sentinel via `Profile.Validate()`.

### CLI integration

One test per surface in existing test files:

- `tdx time template show ../../foo` → exit 1; stderr contains `invalid_artifact_name`.
- `tdx time week pull 2026-04-12/../../foo` → exit 1.
- `tdx auth profile use ..` → exit 1.

### MCP integration

Same pre-config gotcha as Phase 1 — for any MCP handler test that goes through service code, either skip and rely on the helper unit tests OR seed `TDX_CONFIG_HOME`. Lean on unit tests; one smoke test per MCP tool that exercises a refusing input is sufficient. Match the existing testing posture in `tools_tmpl_test.go` (if present) — pragmatic, not exhaustive.

## Branch / version

- Branch: `artifact-name-validation`
- Version: **v0.21.0** — minor; pure validation addition. No breaking changes to existing well-behaved names. Refusing previously-exploitable inputs is a behavior tightening but breaks no legitimate use.

## Open questions

None. All design decisions settled in the 2026-05-16 brainstorming session.
