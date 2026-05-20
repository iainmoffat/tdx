# Security Hardening — OS Keychain Token Storage

**Date:** 2026-05-20
**Phase:** 7 of 7 (final phase of the security hardening rollup; see [Phase 1 spec](2026-05-15-report-fanout-caps.md) for the full roadmap)
**Goal:** Move bearer tokens from plaintext YAML into the OS keychain by default. Keep YAML as an explicit fallback for headless / CI use. Auto-migrate existing tokens on first read.

## Motivation

A security audit on 2026-05-15 identified plaintext-YAML token storage as a Low/Medium-severity finding (`#5`). Today `~/.config/tdx/credentials.yaml` holds bearer tokens as plain text, gated only by 0600 permissions. Two real problems:

- **Windows perms are a no-op.** The existing `enforcePerms` function returns early on Windows because the POSIX mode model doesn't map. Anyone running tdx on Windows has tokens readable by every user account with FS access.
- **Even on Unix, plaintext at rest is below the modern bar.** macOS, Linux (via Secret Service / GNOME Keyring / KWallet), and Windows (Credential Manager) all expose encrypted credential stores with native auth dialogs and per-app access controls. We've been opting out.

This phase moves to keychain-by-default while keeping YAML available as an explicit opt-in for headless and CI environments where no keychain is installed.

## Threat model

- **Malicious local read.** Another process running as the user, an attacker with FS access via an unrelated bug, a stolen backup tarball — all currently read the token in cleartext. Defended by keychain encryption.
- **Cross-user leak on shared boxes (e.g. dev VMs).** A 0600 file on the same filesystem is still readable as root or via privilege escalation. Defended by keychain per-user separation.
- **Out of scope:** root-equivalent attackers (they can read keychain too via OS APIs); kernel-level attacks; physical theft of unlocked machine.

## Policy: three backends, one default

- **Default = keychain.** macOS Keychain, Linux Secret Service (GNOME Keyring / KWallet), Windows Credential Manager.
- **Auto-fallback to YAML** when keychain is unavailable. Each fallback prints `notice: keychain unavailable, using credentials.yaml (set TDX_TOKEN_BACKEND=yaml to silence)` to stderr.
- **Explicit override via `TDX_TOKEN_BACKEND` env var.** Values:
  - unset / `auto` (default): try keychain, fall back to YAML with stderr warning.
  - `keychain`: keychain only, fail closed if unavailable.
  - `yaml`: YAML only, no keychain probe. For tests, CI, and headless servers.
- **Auto-migrate existing YAML tokens on first read** under `auto` or `keychain` backend. One-time stderr notice per profile.
- **Service identifier:** `tdx` (the binary name). **Account:** the profile name (e.g. `default`).

## Library

`github.com/zalando/go-keyring` v0.2.6. Selected because:

- Single-purpose, ~300 lines, single backend per OS.
- Last release 2024; used by `gh` CLI and other small tools.
- No CGo. Cross-compile unchanged.
- Two transitive deps on Linux: `godbus/dbus/v5`, `alessio/shellescape`.

Alternative considered: `github.com/99designs/keyring` (multi-backend, file-encrypted fallback). Rejected — more surface than tdx needs.

This is the explicit deps-add moment, the one exception to the documented "don't run `go mod tidy` unless adding deps" preference. Tooling: `go get github.com/zalando/go-keyring@v0.2.6 && go mod tidy`.

## Architecture

New internal type system in `internal/config/`:

```go
// tokenBackend is the per-process strategy for storing bearer tokens.
type tokenBackend interface {
    Get(profile string) (string, error)       // ErrNoCredentials on miss
    Set(profile, token string) error
    Clear(profile string) error
    Name() string                              // "keychain", "yaml", "memory" — diagnostics
}
```

Three implementations:

| Type | File | Behavior |
|---|---|---|
| `keychainBackend` | `internal/config/credentials_keychain.go` (new) | Wraps `zalando/go-keyring`. Service=`tdx`, Account=profile. |
| `yamlBackend` | `internal/config/credentials_yaml.go` (new — lifted from today's `credentials.go`) | Same YAML+0600 logic as today; no behavior change. |
| `memoryBackend` | `internal/config/credentials_memory_test.go` (new, test-only) | In-process `map[string]string` for tests that need a deterministic non-YAML backend. |

`CredentialsStore` becomes a thin wrapper:

```go
type CredentialsStore struct {
    backend tokenBackend
    yaml    *yamlBackend  // always set, used for auto-migration even when backend != yaml
}

func NewCredentialsStore(paths Paths) *CredentialsStore {
    return &CredentialsStore{
        backend: selectBackend(paths),
        yaml:    newYAMLBackend(paths),
    }
}
```

`selectBackend(paths) tokenBackend` reads `TDX_TOKEN_BACKEND`:

| Value | Probe | Result on probe OK | Result on probe fail |
|---|---|---|---|
| `""` / `auto` | yes | keychainBackend | yamlBackend + stderr notice |
| `keychain` | yes | keychainBackend | error from `NewCredentialsStore` (fail closed) |
| `yaml` | no | yamlBackend (silent) | n/a |
| anything else | n/a | n/a | error: invalid value |

`selectBackend` runs once per process; the resulting backend is cached on the struct.

### `Get(profile)` flow

1. `backend.Get(profile)` — if found, return.
2. If miss AND `backend.Name() != "yaml"` AND `yaml.Get(profile)` returns a non-empty token: enter the **auto-migrate path** (next section).
3. Otherwise return `ErrNoCredentials`.

### `Set(profile, token)`

Writes to `backend` only. The YAML file is never written by `Set` after this phase (except by `yamlBackend` when that's the active backend).

### `Clear(profile)`

Paranoid: clears both keychain and YAML. Missing-entry errors on either side are ignored. This guarantees `tdx auth logout` produces a clean state regardless of where the token actually lives.

## Migration semantics

Auto-migration is the most subtle part. Invariants:

**When migration fires:** `backend.Name() == "keychain"` AND `backend.Get(profile)` returned `ErrNoCredentials` AND `yaml.Get(profile)` returned a non-empty token.

**Steps (in order, with explicit failure handling):**

1. **Write to keychain.** `backend.Set(profile, yamlToken)`. On error: leave YAML untouched; surface the error.
2. **Read back from keychain.** If round-trip token differs from input: leave YAML untouched; return error. (Defensive — keychain quirks are real.)
3. **Clear from YAML.** `yaml.Clear(profile)`. On error: token now in BOTH places. Stderr-warn but return the token; next `tdx` command will re-migrate harmlessly.
4. **If YAML is empty, remove the file.** Best-effort; an `os.Remove` failure here doesn't block the command.
5. **One-time stderr notice per profile per process:** `notice: migrated token for profile %q from credentials.yaml to OS keychain`.

**Idempotency:** After successful migration the next command finds the token in keychain at step 1 of `Get`; YAML is empty or absent; nothing fires.

**No `tdx auth migrate` command.** Auto-migration covers every real case.

## Behavior matrix

### Backend selection outcomes

| `TDX_TOKEN_BACKEND` | Keychain probe | Result | Stderr |
|---|---|---|---|
| unset / `auto` | OK | keychainBackend | (silent) |
| unset / `auto` | fails | yamlBackend | `notice: keychain unavailable, using credentials.yaml (set TDX_TOKEN_BACKEND=yaml to silence)` |
| `keychain` | OK | keychainBackend | (silent) |
| `keychain` | fails | error from `NewCredentialsStore`; command fails | `error: TDX_TOKEN_BACKEND=keychain but keychain is unavailable: %v` |
| `yaml` | (not run) | yamlBackend | (silent) |
| anything else | (not run) | error | `error: invalid TDX_TOKEN_BACKEND %q (want auto, keychain, or yaml)` |

### Keychain probe

A `Get` on a sentinel account (`__probe__`) we never `Set`. Three outcomes:

- `nil` (somehow set in keychain): treat as OK.
- `keyring.ErrNotFound`: treat as OK — the backend works, this key just isn't there.
- Any other error: treat as unavailable.

### Concurrency

`NewCredentialsStore` is called per-command (no daemon); selection happens once per process. No locking needed.

## Tests

| File | Purpose |
|---|---|
| `internal/config/credentials_test.go` (modify) | Reset every existing test to `t.Setenv("TDX_TOKEN_BACKEND", "yaml")` so file-based behavior is unchanged. No assertions change. |
| `internal/config/credentials_backend_test.go` (new) | `selectBackend` table tests: each env-var value × probe outcome. Uses an injectable probe function so we can simulate keychain-OK vs keychain-fails. |
| `internal/config/credentials_migration_test.go` (new) | Seed YAML with a token; force `keychain` backend with an in-memory implementation; call `GetToken`; assert: keychain holds token, YAML is empty/absent, stderr captured contains migration notice. Partial-failure: simulate keychain `Set` returning an error; assert YAML untouched, original token returned. |
| `internal/config/credentials_keychain_test.go` (new) | Live keychain round-trip — Set / Get / Clear on a unique `__test_<UUID>__` account. Guards with `t.Skip` based on the probe so CI without D-Bus self-skips. Uses `defer Clear` to clean up the entry. |

**Test isolation guarantee:** every test that doesn't explicitly exercise keychain sets `TDX_TOKEN_BACKEND=yaml` via `t.Setenv`. The dev's real keychain is never written to except by the explicit live-keychain smoke test (which uses a unique account ID and cleans up).

## Docs

- `docs/guide/auth.md` — new "Credential storage" section: three backends, env var, migration behavior, what shows up in macOS Keychain Access / Linux secret services / Windows Credential Manager.
- `docs/manual-tests/2026-05-20-keychain-token-storage-walkthrough.md` (new) — five-step manual walkthrough covering: fresh install (auto, keychain), upgrade (migration notice), forced `yaml` mode, forced `keychain` failing on a system without one, `tdx auth logout` clears both.
- `README.md:168` — update credentials.yaml row to: `| credentials.yaml | Fallback authentication tokens (per profile); default is the OS keychain |`.

## Branch / version

- Branch: `keychain-token-storage`
- Version: **v0.24.0** — minor. New dependency, new default backend, user-visible stderr notice on upgrade. Backward-compatible — no breaking CLI API change; `TDX_TOKEN_BACKEND=yaml` reproduces today's behavior exactly.

## Open questions

None. All design decisions settled in the 2026-05-20 brainstorming session.
