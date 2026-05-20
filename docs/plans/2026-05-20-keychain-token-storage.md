# Keychain Token Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move bearer tokens from plaintext YAML into the OS keychain by default. Keep YAML as an explicit opt-in. Auto-migrate existing tokens on first read.

**Architecture:** Internal `tokenBackend` interface in `internal/config/` with three implementations: `keychainBackend` (wraps `zalando/go-keyring`), `yamlBackend` (extracted from today's `credentials.go`), and `memoryBackend` (test-only). `CredentialsStore` becomes a thin wrapper that selects a backend at construction time via `TDX_TOKEN_BACKEND` env var (default `auto` = try keychain, fall back to YAML with stderr warning). On `Get` miss under keychain, attempt a one-time migration from YAML.

**Tech Stack:** Go 1.26.2; `github.com/zalando/go-keyring v0.2.6`; testify/require.

**Spec:** [`docs/specs/2026-05-20-keychain-token-storage.md`](../specs/2026-05-20-keychain-token-storage.md)

---

## File Structure

After this plan completes:

```
internal/
└── config/
    ├── credentials.go                          # SHRINK: thin wrapper, ~60 lines
    ├── credentials_backend.go                  # CREATE: tokenBackend iface + selectBackend
    ├── credentials_keychain.go                 # CREATE: zalando/go-keyring wrapper
    ├── credentials_yaml.go                     # CREATE: yamlBackend (lifted from credentials.go)
    ├── credentials_test.go                     # MODIFY: existing tests + TestMain
    ├── credentials_backend_test.go             # CREATE: selectBackend table tests
    ├── credentials_memory_test.go              # CREATE: memoryBackend (test-only)
    ├── credentials_migration_test.go           # CREATE: auto-migration tests
    └── credentials_keychain_test.go            # CREATE: live keychain smoke test (self-skip)

internal/mcp/
└── tools_auth_test.go                          # MODIFY: add TestMain or use writablePaths
internal/cli/auth/
├── helpers_test.go                             # MODIFY: TestMain
└── (login_test.go, logout_test.go, profile_test.go all use helpers_test.go's setup)
internal/cli/time/entry/
└── list_test.go                                # MODIFY: seedProfile sets env var
(other packages that touch CredentialsStore — audited in Task 5)

docs/
├── guide/
│   └── auth.md                                 # MODIFY: new "Credential storage" section
├── manual-tests/
│   └── 2026-05-20-keychain-token-storage-walkthrough.md   # CREATE
README.md                                       # MODIFY: line 168 update

go.mod / go.sum                                 # MODIFY: add zalando/go-keyring
```

## Branch + Versioning

- Branch: `keychain-token-storage` (Task 0)
- Version: **v0.24.0** — minor; new dep, new default backend, user-visible stderr notice on upgrade.

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. Main is at `3fa1174` (the spec commit).

- [ ] **Step 2: Create branch**

```bash
git checkout -b keychain-token-storage
```

Expected: `Switched to a new branch 'keychain-token-storage'`.

---

## Task 1: Add `zalando/go-keyring` dependency

**Files:** `go.mod`, `go.sum`

This is the explicit deps-add moment (one exception to the user's "don't run `go mod tidy` unless adding deps" preference).

- [ ] **Step 1: Add the dep**

```bash
go get github.com/zalando/go-keyring@v0.2.6
go mod tidy
```

Expected: `go.mod` gains `github.com/zalando/go-keyring v0.2.6` in the `require` block as a direct dep; `go.sum` gains entries for it and its transitive deps (`godbus/dbus/v5`, `alessio/shellescape`).

- [ ] **Step 2: Verify nothing broke**

```bash
go build ./...
go test ./... -count=1
```

Both pass. (No code uses keyring yet, so this just verifies the dep resolves.)

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add github.com/zalando/go-keyring v0.2.6"
```

No `Co-Authored-By` trailer.

---

## Task 2: Define `tokenBackend` interface + extract `yamlBackend`

This task is **refactor-only**: lift the existing YAML logic from `credentials.go` into a `yamlBackend` type, then have `CredentialsStore` call it. All 7 existing credentials tests pass unchanged.

**Files:**
- Create: `internal/config/credentials_backend.go`
- Create: `internal/config/credentials_yaml.go`
- Modify: `internal/config/credentials.go`

- [ ] **Step 1: Create the backend interface**

Create `internal/config/credentials_backend.go`:

```go
package config

// tokenBackend is the per-process strategy for storing bearer tokens.
// Implementations: yamlBackend (file), keychainBackend (OS), memoryBackend
// (test-only).
type tokenBackend interface {
	// Get returns the token for the named profile, or ErrNoCredentials.
	Get(profile string) (string, error)

	// Set writes or replaces the token for the named profile.
	Set(profile, token string) error

	// Clear removes the token for the named profile. Missing is not an error.
	Clear(profile string) error

	// Name returns a stable identifier ("yaml", "keychain", "memory") for
	// diagnostics and migration logic.
	Name() string
}
```

- [ ] **Step 2: Lift YAML logic into `yamlBackend`**

Create `internal/config/credentials_yaml.go`:

```go
package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/iainmoffat/tdx/internal/domain"
	"gopkg.in/yaml.v3"
)

// credentialsFile is the on-disk shape of credentials.yaml.
type credentialsFile struct {
	Tokens map[string]string `yaml:"tokens"`
}

// yamlBackend persists tokens in ~/.config/tdx/credentials.yaml with 0600 perms.
// This is the fallback backend used when the OS keychain is unavailable or
// explicitly disabled via TDX_TOKEN_BACKEND=yaml.
type yamlBackend struct {
	paths Paths
}

func newYAMLBackend(paths Paths) *yamlBackend {
	return &yamlBackend{paths: paths}
}

func (b *yamlBackend) Name() string { return "yaml" }

func (b *yamlBackend) Get(profile string) (string, error) {
	cf, err := b.load()
	if err != nil {
		return "", err
	}
	token, ok := cf.Tokens[profile]
	if !ok || token == "" {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	return token, nil
}

func (b *yamlBackend) Set(profile, token string) error {
	cf, err := b.load()
	if err != nil {
		return err
	}
	if cf.Tokens == nil {
		cf.Tokens = make(map[string]string)
	}
	cf.Tokens[profile] = token
	return b.save(cf)
}

func (b *yamlBackend) Clear(profile string) error {
	cf, err := b.load()
	if err != nil {
		return err
	}
	if _, ok := cf.Tokens[profile]; !ok {
		return nil
	}
	delete(cf.Tokens, profile)
	if len(cf.Tokens) == 0 {
		// Empty file — remove it entirely. Best-effort.
		_ = os.Remove(b.paths.CredentialsFile)
		return nil
	}
	return b.save(cf)
}

func (b *yamlBackend) load() (credentialsFile, error) {
	data, err := os.ReadFile(b.paths.CredentialsFile)
	if errors.Is(err, os.ErrNotExist) {
		return credentialsFile{Tokens: map[string]string{}}, nil
	}
	if err != nil {
		return credentialsFile{}, fmt.Errorf("read credentials: %w", err)
	}
	if err := enforcePerms(b.paths.CredentialsFile); err != nil {
		return credentialsFile{}, err
	}
	var cf credentialsFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return credentialsFile{}, fmt.Errorf("parse credentials: %w", err)
	}
	if cf.Tokens == nil {
		cf.Tokens = map[string]string{}
	}
	return cf, nil
}

func (b *yamlBackend) save(cf credentialsFile) error {
	if err := b.paths.EnsureRoot(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cf)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	tmp := b.paths.CredentialsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Rename(tmp, b.paths.CredentialsFile); err != nil {
		return fmt.Errorf("finalize credentials: %w", err)
	}
	return nil
}

// enforcePerms narrows the credentials file permissions if they are too open.
// Windows perms are not meaningfully enforceable here, so it is a no-op there.
func enforcePerms(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o600 {
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("tighten credentials perms: %w", err)
		}
	}
	return nil
}
```

This is the existing logic, verbatim, with three changes:
- Receiver `s *CredentialsStore` → `b *yamlBackend`.
- Field `s.paths` → `b.paths` (same accessor pattern).
- New `Name()` method and an extra empty-file cleanup in `Clear` (the lifted code now removes the file when it becomes empty, matching Phase 7 spec's "if YAML is now empty, remove the file").

- [ ] **Step 3: Shrink `credentials.go` to a thin wrapper**

Replace the entire contents of `internal/config/credentials.go` with:

```go
package config

// CredentialsStore persists bearer tokens per profile. It selects a backend
// at construction time based on TDX_TOKEN_BACKEND (auto / keychain / yaml).
// See credentials_backend.go for the tokenBackend interface and
// credentials_{yaml,keychain,memory}.go for implementations.
type CredentialsStore struct {
	backend tokenBackend
}

// NewCredentialsStore constructs a store with the YAML backend.
//
// TODO(Task 3): switch to selectBackend(paths) so the env var controls choice.
// TODO(Task 4): wire keychain backend.
// TODO(Task 5): wire auto-migration on Get miss.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	return &CredentialsStore{backend: newYAMLBackend(paths)}
}

// GetToken returns the token for the named profile, or ErrNoCredentials.
func (s *CredentialsStore) GetToken(profile string) (string, error) {
	return s.backend.Get(profile)
}

// SetToken writes or replaces the token for the named profile.
func (s *CredentialsStore) SetToken(profile, token string) error {
	return s.backend.Set(profile, token)
}

// ClearToken removes the token for the named profile. Missing is not an error.
func (s *CredentialsStore) ClearToken(profile string) error {
	return s.backend.Clear(profile)
}
```

(The two `TODO(Task N)` comments will be removed in their respective tasks. They're temporarily explicit so a reviewer of this task isn't confused about whether the wrapper is intentionally minimal.)

- [ ] **Step 4: Verify existing tests still pass**

```bash
go test ./internal/config/... -v
```

All 7 existing credentials tests + all profile tests pass. `gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/config/credentials.go internal/config/credentials_backend.go internal/config/credentials_yaml.go
git commit -m "refactor(config): extract yamlBackend behind tokenBackend interface"
```

No `Co-Authored-By` trailer.

---

## Task 3: `selectBackend` env-var parsing + `memoryBackend` + config-package `TestMain`

After this task, `CredentialsStore` reads `TDX_TOKEN_BACKEND` (values: empty/`auto`, `yaml`, anything else = error). Keychain itself is added in Task 4 — for now, `auto` is treated as `yaml` (no keychain probe yet).

**Files:**
- Modify: `internal/config/credentials.go` (drop the Task 2 `TODO(Task 3)` line; use `selectBackend`)
- Modify: `internal/config/credentials_backend.go` (add `selectBackend`)
- Create: `internal/config/credentials_memory_test.go`
- Create: `internal/config/credentials_backend_test.go`
- Modify: `internal/config/credentials_test.go` (add `TestMain` that defaults env var to `yaml` for whole package)

- [ ] **Step 1: Add `selectBackend` to `credentials_backend.go`**

Append to `internal/config/credentials_backend.go`:

```go
import (
	"fmt"
	"os"
)

// backendName values for TDX_TOKEN_BACKEND.
const (
	backendAuto     = "auto"
	backendKeychain = "keychain"
	backendYAML     = "yaml"
)

// selectBackend chooses a token backend based on the TDX_TOKEN_BACKEND
// environment variable. Returns an error only when the env var holds an
// unrecognized value (defensive — caller should treat as fatal).
//
// Task 3: only "yaml" and "auto" are supported here. "auto" falls back to
// yamlBackend silently because keychain support is added in Task 4.
func selectBackend(paths Paths) (tokenBackend, error) {
	switch os.Getenv("TDX_TOKEN_BACKEND") {
	case "", backendAuto:
		// Task 4 will probe keychain here; for now, yaml is the only option.
		return newYAMLBackend(paths), nil
	case backendYAML:
		return newYAMLBackend(paths), nil
	case backendKeychain:
		// Task 4 will return keychainBackend or fail closed; for now, error
		// so a user who sets keychain= now gets a clear "not yet wired"
		// signal rather than silent YAML fallthrough.
		return nil, fmt.Errorf("TDX_TOKEN_BACKEND=keychain not yet supported (will land in Task 4)")
	default:
		return nil, fmt.Errorf("invalid TDX_TOKEN_BACKEND %q (want auto, keychain, or yaml)", os.Getenv("TDX_TOKEN_BACKEND"))
	}
}
```

Update the package imports — `credentials_backend.go` had no imports before; add the new `fmt` and `os` block at the top.

- [ ] **Step 2: Use `selectBackend` from `NewCredentialsStore`**

In `internal/config/credentials.go`, replace `NewCredentialsStore`:

```go
// NewCredentialsStore constructs a store with the backend selected by
// TDX_TOKEN_BACKEND. Default (auto / unset) currently falls back to YAML
// — Task 4 wires the keychain backend. Returns an error if the env var
// holds an invalid value.
func NewCredentialsStore(paths Paths) (*CredentialsStore, error) {
	backend, err := selectBackend(paths)
	if err != nil {
		return nil, err
	}
	return &CredentialsStore{backend: backend}, nil
}
```

**Breaking API change:** the return signature gains an error. Every caller of `NewCredentialsStore` must be updated. Audit:

```bash
grep -rn "config.NewCredentialsStore\|NewCredentialsStore(" /Users/ipm/code/tdx --include='*.go' | grep -v '_test.go'
```

Each non-test call site (in `internal/svc/authsvc/service.go`, `peoplesvc/service.go`, `projectsvc/service.go`, `timesvc/service.go`, `ticketsvc/service.go`) needs the error-return update. Pattern: where today they do

```go
credentials: config.NewCredentialsStore(paths),
```

inside a struct literal, that becomes (after the constructor returns an error):

```go
// outside the struct literal
creds, err := config.NewCredentialsStore(paths)
if err != nil {
    return nil, fmt.Errorf("init credentials: %w", err)
}
return &Service{
    ...
    credentials: creds,
}
```

For services whose `New(...)` doesn't currently return an error (most of them), the constructor signature also has to gain an error return. Trace each one and update its callers. This propagates to a handful of `service.New(...)` call sites in CLI and MCP code.

That's a substantial ripple. **Alternative:** keep `NewCredentialsStore(paths) *CredentialsStore` returning a single value, and surface the env-var-invalid case via a `Get`/`Set`/`Clear` error at the first call. The store stores the env-var error and returns it on every method call. Less idiomatic but no ripple.

**This plan picks the no-ripple alternative.** Replace the new `NewCredentialsStore` body with:

```go
// NewCredentialsStore constructs a store with the backend selected by
// TDX_TOKEN_BACKEND. Default (auto / unset) currently falls back to YAML
// — Task 4 wires the keychain backend. If TDX_TOKEN_BACKEND holds an
// invalid value, the store stores the error and surfaces it on the
// first Get/Set/Clear call. This keeps the constructor signature
// backward-compatible.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	backend, err := selectBackend(paths)
	return &CredentialsStore{backend: backend, initErr: err}
}
```

Add an `initErr error` field to the struct, and update each method to return it first:

```go
type CredentialsStore struct {
	backend tokenBackend
	initErr error
}

func (s *CredentialsStore) GetToken(profile string) (string, error) {
	if s.initErr != nil {
		return "", s.initErr
	}
	return s.backend.Get(profile)
}

func (s *CredentialsStore) SetToken(profile, token string) error {
	if s.initErr != nil {
		return s.initErr
	}
	return s.backend.Set(profile, token)
}

func (s *CredentialsStore) ClearToken(profile string) error {
	if s.initErr != nil {
		return s.initErr
	}
	return s.backend.Clear(profile)
}
```

No call-site changes needed in services. Tests can assert the error via the existing `errors.Is(err, ...)` pattern.

- [ ] **Step 3: Add `memoryBackend` (test-only)**

Create `internal/config/credentials_memory_test.go`:

```go
package config

import (
	"fmt"
	"sync"

	"github.com/iainmoffat/tdx/internal/domain"
)

// memoryBackend is an in-process token backend used only by tests. Lives in
// a _test.go file so it's not compiled into production binaries.
type memoryBackend struct {
	mu     sync.Mutex
	tokens map[string]string

	// failNextSet, if true, causes the next Set call to return failSetErr.
	// Test machinery uses this to exercise migration partial-failure paths.
	failNextSet bool
	failSetErr  error
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{tokens: map[string]string{}}
}

func (b *memoryBackend) Name() string { return "memory" }

func (b *memoryBackend) Get(profile string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.tokens[profile]
	if !ok || t == "" {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	return t, nil
}

func (b *memoryBackend) Set(profile, token string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failNextSet {
		b.failNextSet = false
		return b.failSetErr
	}
	b.tokens[profile] = token
	return nil
}

func (b *memoryBackend) Clear(profile string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.tokens, profile)
	return nil
}
```

- [ ] **Step 4: Add `selectBackend` tests**

Create `internal/config/credentials_backend_test.go`:

```go
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectBackend_DefaultIsYAML(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_AutoIsYAML(t *testing.T) {
	// Until Task 4, auto falls back to yaml.
	t.Setenv("TDX_TOKEN_BACKEND", "auto")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_YAMLForced(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "yaml")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_KeychainErrorsUntilTask4(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "keychain")
	_, err := selectBackend(writablePaths(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "keychain")
}

func TestSelectBackend_InvalidValue(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "bogus")
	_, err := selectBackend(writablePaths(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid TDX_TOKEN_BACKEND")
}
```

- [ ] **Step 5: Add `TestMain` to default `TDX_TOKEN_BACKEND=yaml` for the whole config package**

Insert at the top of `internal/config/credentials_test.go` (before the first existing test, after imports):

```go
// TestMain defaults TDX_TOKEN_BACKEND=yaml for the entire config package so
// tests never touch the dev's real keychain. Individual tests that need to
// exercise a different backend override the env var via t.Setenv.
func TestMain(m *testing.M) {
	if os.Getenv("TDX_TOKEN_BACKEND") == "" {
		os.Setenv("TDX_TOKEN_BACKEND", "yaml")
	}
	os.Exit(m.Run())
}
```

`os` is already imported by this file. (Why only set if empty: the new selectBackend tests use `t.Setenv` to flip the value; we don't want to fight them.)

- [ ] **Step 6: Run all config tests**

```bash
go test ./internal/config/... -v
```

All existing tests + 5 new `selectBackend` tests pass.

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 7: Commit**

```bash
git add internal/config/credentials.go internal/config/credentials_backend.go \
        internal/config/credentials_memory_test.go internal/config/credentials_backend_test.go \
        internal/config/credentials_test.go
git commit -m "feat(config): selectBackend env-var parsing + memoryBackend + TestMain"
```

---

## Task 4: `keychainBackend` + probe + wire into `selectBackend`

After this task, `TDX_TOKEN_BACKEND=keychain` uses a real keychain (fails closed if unavailable); `auto` probes keychain and falls back to YAML with a stderr notice. Keychain itself is exercised in tests via the `memoryBackend` substituting for it.

**Files:**
- Create: `internal/config/credentials_keychain.go`
- Modify: `internal/config/credentials_backend.go` (probe + wire keychain)
- Modify: `internal/config/credentials_backend_test.go` (update `TestSelectBackend_KeychainErrorsUntilTask4` since this task makes it work — replace with appropriate-shape tests)

- [ ] **Step 1: Create `keychainBackend`**

Create `internal/config/credentials_keychain.go`:

```go
package config

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"

	"github.com/iainmoffat/tdx/internal/domain"
)

// keychainServiceName is the service identifier under which tdx tokens are
// stored. On macOS this appears in Keychain Access.app as the "Where" field;
// on Linux it's the secret service name; on Windows it's the Credential
// Manager target name (combined with the profile as the account).
const keychainServiceName = "tdx"

// keychainBackend wraps zalando/go-keyring. Each profile's token is stored
// under service=tdx, account=<profile-name>.
type keychainBackend struct{}

func newKeychainBackend() *keychainBackend { return &keychainBackend{} }

func (b *keychainBackend) Name() string { return "keychain" }

func (b *keychainBackend) Get(profile string) (string, error) {
	t, err := keyring.Get(keychainServiceName, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	if err != nil {
		return "", fmt.Errorf("keychain get: %w", err)
	}
	return t, nil
}

func (b *keychainBackend) Set(profile, token string) error {
	if err := keyring.Set(keychainServiceName, profile, token); err != nil {
		return fmt.Errorf("keychain set: %w", err)
	}
	return nil
}

func (b *keychainBackend) Clear(profile string) error {
	err := keyring.Delete(keychainServiceName, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keychain clear: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Add probe + wire into `selectBackend`**

Replace `selectBackend` in `internal/config/credentials_backend.go`:

```go
const keychainProbeAccount = "__probe__"

// probeKeychain returns nil if the OS keychain is usable in this process.
// Tries a Get on a sentinel account we never Set; both nil and ErrNotFound
// indicate the backend is reachable.
//
// Variable, not function, so tests can swap it out.
var probeKeychain = func() error {
	_, err := keyring.Get(keychainServiceName, keychainProbeAccount)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// selectBackend chooses a token backend based on the TDX_TOKEN_BACKEND
// environment variable.
//
//   - "" / "auto": try keychain; if probe fails, fall back to yaml with a
//     stderr notice so the user knows to set TDX_TOKEN_BACKEND=yaml to silence.
//   - "keychain": keychain only; fail closed if probe fails.
//   - "yaml": yaml only; no keychain probe.
//   - anything else: error.
func selectBackend(paths Paths) (tokenBackend, error) {
	switch os.Getenv("TDX_TOKEN_BACKEND") {
	case "", backendAuto:
		if err := probeKeychain(); err != nil {
			fmt.Fprintf(os.Stderr,
				"notice: keychain unavailable, using credentials.yaml (set TDX_TOKEN_BACKEND=yaml to silence)\n")
			return newYAMLBackend(paths), nil
		}
		return newKeychainBackend(), nil
	case backendKeychain:
		if err := probeKeychain(); err != nil {
			return nil, fmt.Errorf("TDX_TOKEN_BACKEND=keychain but keychain is unavailable: %w", err)
		}
		return newKeychainBackend(), nil
	case backendYAML:
		return newYAMLBackend(paths), nil
	default:
		return nil, fmt.Errorf("invalid TDX_TOKEN_BACKEND %q (want auto, keychain, or yaml)", os.Getenv("TDX_TOKEN_BACKEND"))
	}
}
```

Add `"errors"` and `"github.com/zalando/go-keyring"` to the file's imports.

- [ ] **Step 3: Update the Task 3 selectBackend tests**

The Task 3 `TestSelectBackend_KeychainErrorsUntilTask4` is now wrong-shaped — replace with:

```go
func TestSelectBackend_KeychainStrictFailsClosedWhenProbeFails(t *testing.T) {
	// Swap out the probe to simulate keychain unavailable.
	originalProbe := probeKeychain
	probeKeychain = func() error { return errors.New("simulated keychain down") }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "keychain")
	_, err := selectBackend(writablePaths(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "keychain is unavailable")
}

func TestSelectBackend_KeychainStrictSucceedsWhenProbeOK(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return nil }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "keychain")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "keychain", b.Name())
}

func TestSelectBackend_AutoFallsBackToYAMLWhenProbeFails(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return errors.New("simulated keychain down") }
	t.Cleanup(func() { probeKeychain = originalProbe })

	// Capture stderr to assert the notice.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	t.Setenv("TDX_TOKEN_BACKEND", "auto")
	b, err := selectBackend(writablePaths(t))

	_ = w.Close()
	os.Stderr = oldStderr
	stderr, _ := io.ReadAll(r)

	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
	require.Contains(t, string(stderr), "notice: keychain unavailable")
}

func TestSelectBackend_AutoUsesKeychainWhenProbeOK(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return nil }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "auto")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "keychain", b.Name())
}
```

Add `"errors"` and `"io"` to the test file's imports.

Delete the now-obsolete `TestSelectBackend_KeychainErrorsUntilTask4` from Task 3 (it's been superseded).

- [ ] **Step 4: Run config tests**

```bash
go test ./internal/config/... -v
```

All pass. The keychain tests use the swapped `probeKeychain` and don't touch the real OS keychain.

- [ ] **Step 5: Commit**

```bash
git add internal/config/credentials_keychain.go internal/config/credentials_backend.go \
        internal/config/credentials_backend_test.go
git commit -m "feat(config): wire keychainBackend with auto-fallback + strict modes"
```

---

## Task 5: Auto-migration on `Get` miss

When the active backend is keychain and the requested profile isn't there but YAML has a token, migrate.

**Files:**
- Modify: `internal/config/credentials.go` (Get logic; Clear paranoia)
- Create: `internal/config/credentials_migration_test.go`

- [ ] **Step 1: Write failing migration tests**

Create `internal/config/credentials_migration_test.go`:

```go
package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// newStoreWithBackends is a test-only constructor that wires a specific
// backend AND keeps yamlBackend available as the migration source. Mirrors
// what the production NewCredentialsStore does at runtime.
func newStoreWithBackends(t *testing.T, backend tokenBackend, paths Paths) *CredentialsStore {
	t.Helper()
	return &CredentialsStore{
		backend: backend,
		yaml:    newYAMLBackend(paths),
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = oldStderr
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestMigration_HappyPath(t *testing.T) {
	paths := writablePaths(t)

	// Seed YAML directly via yamlBackend.
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	// Construct store with memory backend + yaml fallback.
	mem := newMemoryBackend()
	store := newStoreWithBackends(t, mem, paths)

	var token string
	stderr := captureStderr(t, func() {
		var err error
		token, err = store.GetToken("default")
		require.NoError(t, err)
	})

	// Returned token is the migrated value.
	require.Equal(t, "yaml-token", token)

	// Now in keychain (mem) and absent from YAML.
	memToken, _ := mem.Get("default")
	require.Equal(t, "yaml-token", memToken)

	yamlToken, err := yb.Get("default")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
	_ = yamlToken

	// Stderr notice fired.
	require.Contains(t, stderr, `migrated token for profile "default"`)

	// YAML file removed (was the last entry).
	_, err = os.Stat(filepath.Join(paths.Root, "credentials.yaml"))
	require.True(t, os.IsNotExist(err))
}

func TestMigration_IdempotentSecondCall(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	mem := newMemoryBackend()
	store := newStoreWithBackends(t, mem, paths)

	// First Get migrates.
	_, err := store.GetToken("default")
	require.NoError(t, err)

	// Second Get should NOT print the notice again.
	stderr := captureStderr(t, func() {
		token, err := store.GetToken("default")
		require.NoError(t, err)
		require.Equal(t, "yaml-token", token)
	})
	require.NotContains(t, stderr, "migrated token")
}

func TestMigration_KeychainSetFailsLeavesYAMLIntact(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	mem := newMemoryBackend()
	mem.failNextSet = true
	mem.failSetErr = errors.New("simulated keychain Set failure")

	store := newStoreWithBackends(t, mem, paths)

	_, err := store.GetToken("default")
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated keychain Set failure")

	// YAML token untouched — bearer token not lost.
	stillThere, gerr := yb.Get("default")
	require.NoError(t, gerr)
	require.Equal(t, "yaml-token", stillThere)
}

func TestMigration_NoYAMLTokenReturnsErrNoCredentials(t *testing.T) {
	paths := writablePaths(t)
	mem := newMemoryBackend()
	store := newStoreWithBackends(t, mem, paths)

	_, err := store.GetToken("default")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestMigration_YAMLBackendDoesNotTriggerMigration(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	store := newStoreWithBackends(t, yb, paths)

	stderr := captureStderr(t, func() {
		token, err := store.GetToken("default")
		require.NoError(t, err)
		require.Equal(t, "yaml-token", token)
	})
	// No migration notice when backend IS yaml — nothing to migrate.
	require.NotContains(t, stderr, "migrated token")
}

func TestClear_RemovesFromBothBackends(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	mem := newMemoryBackend()
	require.NoError(t, mem.Set("default", "memory-token"))

	store := newStoreWithBackends(t, mem, paths)
	require.NoError(t, store.ClearToken("default"))

	_, memErr := mem.Get("default")
	require.ErrorIs(t, memErr, domain.ErrNoCredentials)

	_, yamlErr := yb.Get("default")
	require.ErrorIs(t, yamlErr, domain.ErrNoCredentials)
}
```

- [ ] **Step 2: Run tests — must FAIL**

```bash
go test ./internal/config/... -run TestMigration -v
go test ./internal/config/... -run TestClear_RemovesFromBoth -v
```

All new tests fail because `CredentialsStore` doesn't have a `yaml` field, doesn't migrate, and `ClearToken` only clears the active backend.

- [ ] **Step 3: Update `CredentialsStore`**

In `internal/config/credentials.go`, replace the entire file (drop the Task 2-era TODOs):

```go
package config

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/iainmoffat/tdx/internal/domain"
)

// CredentialsStore persists bearer tokens per profile. It selects a backend
// at construction time based on TDX_TOKEN_BACKEND (auto / keychain / yaml).
// On Get miss under a non-YAML backend, the store attempts a one-time
// migration from credentials.yaml — see credentials_migration_test.go.
type CredentialsStore struct {
	backend tokenBackend
	yaml    *yamlBackend // always set, used for auto-migration

	initErr error

	// migrationNoticed records profiles already announced as migrated this
	// process so the stderr notice prints at most once per profile per run.
	mu                sync.Mutex
	migrationNoticed  map[string]bool
}

// NewCredentialsStore constructs a store. If TDX_TOKEN_BACKEND holds an
// invalid value the store stores the error and surfaces it on the first
// Get/Set/Clear call.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	backend, err := selectBackend(paths)
	return &CredentialsStore{
		backend:          backend,
		yaml:             newYAMLBackend(paths),
		initErr:          err,
		migrationNoticed: map[string]bool{},
	}
}

// GetToken returns the token for the named profile, or ErrNoCredentials.
// Under a non-YAML backend, a YAML token found here is auto-migrated.
func (s *CredentialsStore) GetToken(profile string) (string, error) {
	if s.initErr != nil {
		return "", s.initErr
	}
	token, err := s.backend.Get(profile)
	if err == nil {
		return token, nil
	}
	if !errors.Is(err, domain.ErrNoCredentials) {
		return "", err
	}
	// Backend miss. If the active backend isn't yaml AND yaml has a token,
	// migrate.
	if s.backend.Name() == "yaml" {
		return "", err
	}
	yamlToken, yerr := s.yaml.Get(profile)
	if yerr != nil {
		// No YAML token either — propagate the original miss.
		return "", err
	}
	return s.migrate(profile, yamlToken)
}

// migrate moves a token from yaml to the active backend with rollback-safe
// semantics. See the spec's "Migration semantics" section for invariants.
func (s *CredentialsStore) migrate(profile, yamlToken string) (string, error) {
	// 1. Write to active backend.
	if err := s.backend.Set(profile, yamlToken); err != nil {
		// Leave YAML untouched; bubble the error.
		return "", fmt.Errorf("migrate %q to %s: %w", profile, s.backend.Name(), err)
	}
	// 2. Read back; if it differs, defensive abort.
	readBack, err := s.backend.Get(profile)
	if err != nil || readBack != yamlToken {
		return "", fmt.Errorf("migrate %q to %s: round-trip mismatch", profile, s.backend.Name())
	}
	// 3. Clear from YAML.
	if cerr := s.yaml.Clear(profile); cerr != nil {
		// Token now in BOTH places. Warn but return the token so the
		// command can proceed.
		fmt.Fprintf(os.Stderr,
			"warning: token migrated to %s but YAML cleanup failed: %v (re-run will retry)\n",
			s.backend.Name(), cerr)
		return yamlToken, nil
	}
	// 4. Print one-time notice per profile per process.
	s.mu.Lock()
	already := s.migrationNoticed[profile]
	s.migrationNoticed[profile] = true
	s.mu.Unlock()
	if !already {
		fmt.Fprintf(os.Stderr,
			"notice: migrated token for profile %q from credentials.yaml to OS keychain\n",
			profile)
	}
	return yamlToken, nil
}

// SetToken writes or replaces the token via the active backend.
func (s *CredentialsStore) SetToken(profile, token string) error {
	if s.initErr != nil {
		return s.initErr
	}
	return s.backend.Set(profile, token)
}

// ClearToken removes the token from the active backend AND from yaml. Missing
// entries on either side are ignored so logout always produces a clean state.
func (s *CredentialsStore) ClearToken(profile string) error {
	if s.initErr != nil {
		return s.initErr
	}
	// Clear active backend first.
	bErr := s.backend.Clear(profile)
	// Always also clear YAML — even when it IS the active backend, this is a
	// safe no-op (yamlBackend.Clear handles missing entries).
	yErr := s.yaml.Clear(profile)
	if bErr != nil && !errors.Is(bErr, domain.ErrNoCredentials) {
		return bErr
	}
	if yErr != nil && !errors.Is(yErr, domain.ErrNoCredentials) {
		return yErr
	}
	return nil
}
```

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/config/... -v
```

All 6 migration tests + 1 clear test + all existing tests pass.

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/config/credentials.go internal/config/credentials_migration_test.go
git commit -m "feat(config): auto-migrate yaml token to active backend on Get miss"
```

---

## Task 6: Audit and update test helpers in other packages

Tests in `internal/mcp/`, `internal/cli/auth/`, `internal/cli/time/entry/`, and any other package that constructs `CredentialsStore` must set `TDX_TOKEN_BACKEND=yaml` so they never touch the dev's real keychain.

**Files:** see audit step

- [ ] **Step 1: Find every test that constructs `CredentialsStore`**

```bash
grep -rn "config.NewCredentialsStore\|NewCredentialsStore(" /Users/ipm/code/tdx --include='*_test.go'
```

Expected files (verify; may have shifted):
- `internal/config/credentials_test.go` — already covered by Task 3 TestMain.
- `internal/mcp/tools_auth_test.go`
- `internal/mcp/tools_ticket_test.go`
- `internal/cli/auth/helpers_test.go`
- `internal/cli/auth/login_test.go`
- `internal/cli/auth/logout_test.go`
- `internal/cli/auth/profile_test.go` (uses helpers_test.go's setup)
- `internal/cli/time/entry/list_test.go` (via `seedProfile`)
- (any other hit from the grep)

- [ ] **Step 2: For each test package in the audit list, add a `TestMain`**

Use the same pattern as `internal/config/credentials_test.go`. Pick the most stable file in each package (alphabetically first or the one likely to remain) and prepend:

```go
import (
	"os"
	"testing"
)

// TestMain defaults TDX_TOKEN_BACKEND=yaml for this package so tests never
// touch the dev's real OS keychain.
func TestMain(m *testing.M) {
	if os.Getenv("TDX_TOKEN_BACKEND") == "" {
		os.Setenv("TDX_TOKEN_BACKEND", "yaml")
	}
	os.Exit(m.Run())
}
```

Specifically:
- `internal/mcp/tools_auth_test.go` — add TestMain here (it appears once across the package).
- `internal/cli/auth/helpers_test.go` — add TestMain here.
- `internal/cli/time/entry/list_test.go` — add TestMain here.

If a package already has a `TestMain`, integrate the env-var line into it instead of adding a second.

- [ ] **Step 3: Run the full test suite**

```bash
go test ./... -count=1
```

All tests pass. No package should be touching the host's keychain.

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools_auth_test.go internal/cli/auth/helpers_test.go internal/cli/time/entry/list_test.go
git commit -m "test: TDX_TOKEN_BACKEND=yaml in test packages that touch credentials"
```

(Adjust the file list to match the actual audit output.)

---

## Task 7: Live keychain smoke test

A single test that exercises the real OS keychain end-to-end. Self-skips on machines without keychain (CI Linux without D-Bus).

**Files:**
- Create: `internal/config/credentials_keychain_test.go`

- [ ] **Step 1: Write the test**

Create `internal/config/credentials_keychain_test.go`:

```go
package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// TestKeychainBackend_LiveRoundTrip exercises the real OS keychain end-to-end
// when one is available. Self-skips otherwise (e.g. CI Linux without D-Bus,
// or any environment where the probe fails). Uses a unique random account
// name to avoid colliding with the user's real tdx tokens; cleans up via
// t.Cleanup.
//
// To force-skip this test even on machines with a keychain, set
// TDX_TOKEN_BACKEND=yaml — the test honors that opt-out so a user running
// `go test ./...` doesn't see a Keychain Access permission prompt on macOS.
func TestKeychainBackend_LiveRoundTrip(t *testing.T) {
	if os.Getenv("TDX_TOKEN_BACKEND") == "yaml" {
		t.Skip("TDX_TOKEN_BACKEND=yaml; skipping live keychain test")
	}
	if err := probeKeychain(); err != nil {
		t.Skipf("keychain not available: %v", err)
	}

	// Random account name so we don't collide with real tokens.
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		t.Fatal(err)
	}
	account := "__tdx_test_" + hex.EncodeToString(buf[:]) + "__"

	b := newKeychainBackend()
	t.Cleanup(func() { _ = b.Clear(account) })

	// Initial Get → ErrNoCredentials
	_, err := b.Get(account)
	require.ErrorIs(t, err, domain.ErrNoCredentials)

	// Set
	require.NoError(t, b.Set(account, "live-test-token"))

	// Get back the same value
	got, err := b.Get(account)
	require.NoError(t, err)
	require.Equal(t, "live-test-token", got)

	// Clear
	require.NoError(t, b.Clear(account))

	// Now Get → ErrNoCredentials again
	_, err = b.Get(account)
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}
```

- [ ] **Step 2: Run config tests**

```bash
go test ./internal/config/... -v
```

If the Task 3 TestMain is still defaulting `TDX_TOKEN_BACKEND=yaml`, this test self-skips. To exercise it locally on macOS, run:

```bash
TDX_TOKEN_BACKEND=auto go test ./internal/config/... -run TestKeychainBackend_LiveRoundTrip -v
```

Expected: PASS (macOS may prompt for keychain access the first time; allow it).

- [ ] **Step 3: Commit**

```bash
git add internal/config/credentials_keychain_test.go
git commit -m "test(config): live keychain round-trip with self-skip"
```

---

## Task 8: Docs

**Files:**
- Modify: `docs/guide/auth.md` — add "Credential storage" section
- Modify: `README.md` — line 168
- Create: `docs/manual-tests/2026-05-20-keychain-token-storage-walkthrough.md`

- [ ] **Step 1: Update `README.md` line 168**

In `README.md`, find the line:

```
| `credentials.yaml` | Authentication tokens (per profile) |
```

Replace with:

```
| `credentials.yaml` | Fallback authentication tokens (per profile); default is the OS keychain |
```

- [ ] **Step 2: Append a "Credential storage" section to `docs/guide/auth.md`**

Find a sensible insertion point (after the existing "tdx auth login" section, before any "Multiple profiles" section). Append:

```markdown
## Credential storage

By default, `tdx` stores your bearer token in the OS keychain:

- **macOS** — Keychain Access.app, under the "tdx" service.
- **Linux** — Secret Service (GNOME Keyring, KWallet) via D-Bus.
- **Windows** — Credential Manager, target name `tdx:<profile>`.

This replaces the older plaintext `~/.config/tdx/credentials.yaml`. If you upgrade from a previous tdx version that wrote tokens to YAML, the next `tdx` command auto-migrates each token into the keychain and prints a one-time notice to stderr. The YAML file is removed when it becomes empty.

### Choosing the backend

Set `TDX_TOKEN_BACKEND` in your shell:

- `auto` (default, or unset) — try keychain; fall back to YAML if the keychain isn't available (headless servers, etc.). A stderr notice fires on every fallback so you know to opt in explicitly.
- `keychain` — keychain only. The command fails if the keychain isn't available, rather than silently writing plaintext.
- `yaml` — YAML only, no keychain. Use this on headless servers, in CI, or when you've deliberately decided plaintext-with-0600 is the right trade-off for your environment.

### Logout / clear

`tdx auth logout` clears the token from both the active backend and the YAML fallback, so there's no stale entry left behind from a previous migration.

### Token scope and rotation

Bearer tokens issued by TeamDynamix are JWTs with a 24-hour lifetime and no refresh mechanism. When `tdx auth status` reports the token as expired, re-run `tdx auth login` (or `tdx auth login --sso`) to mint a fresh one.
```

- [ ] **Step 3: Create the walkthrough doc**

Create `docs/manual-tests/2026-05-20-keychain-token-storage-walkthrough.md`:

```markdown
# OS Keychain Token Storage Walkthrough (v0.24.0)

Spec: [`docs/specs/2026-05-20-keychain-token-storage.md`](../specs/2026-05-20-keychain-token-storage.md)

## Setup

Save your current credentials.yaml (if any):

    cp ~/.config/tdx/credentials.yaml ~/.config/tdx/credentials.yaml.bak 2>/dev/null || true

## Step 1: Auto-migration from existing YAML (macOS)

If you have an existing token in YAML, run any tdx command:

    tdx auth status

Expected:
- stderr: `notice: migrated token for profile "default" from credentials.yaml to OS keychain`
- `~/.config/tdx/credentials.yaml` is removed (was the only profile) OR no longer contains that profile.
- macOS Keychain Access.app shows a new entry under "tdx" service. Verify with:
  ```
  security find-generic-password -s tdx -a default -w
  ```
  (prints the token)

## Step 2: Re-run is silent

    tdx auth status

Expected: normal output; no migration notice this time.

## Step 3: Fresh login lands directly in keychain

    tdx auth logout
    security find-generic-password -s tdx -a default -w 2>&1 | grep -q "could not be found" && echo "logged out (keychain cleared)"
    tdx auth login   # paste a fresh JWT
    security find-generic-password -s tdx -a default -w   # token present

Expected: each command does what its echo says.

## Step 4: Force yaml backend

    TDX_TOKEN_BACKEND=yaml tdx auth status

Expected: normal output. Then:

    ls -la ~/.config/tdx/credentials.yaml

Expected: file exists (yaml backend reads from it for THIS process only — keychain still has the canonical token).

## Step 5: Force keychain strict mode

    TDX_TOKEN_BACKEND=keychain tdx auth status

Expected on macOS: normal output (keychain available, token there).

To simulate keychain unavailable (Linux servers without D-Bus): unset D-Bus and force keychain mode:

    DBUS_SESSION_BUS_ADDRESS=disabled TDX_TOKEN_BACKEND=keychain tdx auth status

Expected: `error: TDX_TOKEN_BACKEND=keychain but keychain is unavailable: ...` and exit 1.

## Step 6: Auto-fallback stderr notice (Linux server simulation)

    DBUS_SESSION_BUS_ADDRESS=disabled tdx auth status

Expected: stderr `notice: keychain unavailable, using credentials.yaml (set TDX_TOKEN_BACKEND=yaml to silence)` followed by the normal status output (which uses the YAML fallback).

## Cleanup

    cp ~/.config/tdx/credentials.yaml.bak ~/.config/tdx/credentials.yaml 2>/dev/null || true
```

- [ ] **Step 4: Commit**

```bash
git add README.md docs/guide/auth.md docs/manual-tests/2026-05-20-keychain-token-storage-walkthrough.md
git commit -m "docs: describe keychain backend, env var, and migration behavior"
```

---

## Task 9: Full test + lint sweep

**Files:** none

- [ ] **Step 1: Run the full suite**

```bash
go test ./... -race && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green. No gofmt output. No vet warnings. No lint warnings.

- [ ] **Step 2: If failures appear, fix in place and commit per-issue**

Common gotchas:
- A test in some package still constructs `CredentialsStore` without a `TestMain` opt-out — add the `TestMain` to that package's tests.
- A linter staticcheck on `!= keychain.ErrNotFound` style — use `errors.Is`.
- gofmt nits from formatted-with-trailing-spaces — `gofmt -w` the offending file.

- [ ] **Step 3: Confirm green**

```bash
go test ./... -race -count=1
```

PASS.

---

## Task 10: Push branch + create PR

**Files:** none

- [ ] **Step 1: Push**

```bash
git push -u origin keychain-token-storage
```

- [ ] **Step 2: Create PR**

Write body to `/tmp/pr-body-phase7.md`:

```markdown
## Summary

Phase 7 of the security hardening rollup — the **last one**. Addresses audit finding #5 (Low/Medium: bearer tokens stored as plaintext YAML).

- New `tokenBackend` interface in `internal/config/` with three implementations: `keychainBackend` (zalando/go-keyring), `yamlBackend` (lifted from existing code), `memoryBackend` (test-only).
- `TDX_TOKEN_BACKEND=auto|keychain|yaml` env var controls selection; default `auto` tries keychain first and falls back to YAML with a one-line stderr notice.
- Auto-migration: when keychain is active and the requested profile isn't there, but `credentials.yaml` has it, the token moves to keychain and the YAML entry is cleared. Rollback-safe partial-failure handling — if keychain Set fails or the round-trip mismatches, YAML stays intact.
- `tdx auth logout` (via `Clear`) wipes both backends so there's no stale YAML entry after migration.
- Test isolation: `TestMain` in each affected package defaults the env var to `yaml` so tests never touch the dev's real keychain. Live keychain round-trip test self-skips on systems without one.
- New dep: `github.com/zalando/go-keyring v0.2.6`.

## Test plan

- [x] `go test ./... -race` green
- [x] `go vet ./...`, `gofmt -l .`, `golangci-lint run ./...` clean
- [x] Live keychain test passes on macOS (`TDX_TOKEN_BACKEND=auto go test ./internal/config/... -run LiveRoundTrip`)
- [ ] Live manual walkthrough on macOS at `docs/manual-tests/2026-05-20-keychain-token-storage-walkthrough.md`
- [ ] Smoke test on a Linux box with D-Bus (GNOME Keyring) — Step 1 migration
- [ ] Smoke test forced `TDX_TOKEN_BACKEND=yaml` (CI simulation)

Closes: security audit finding #5. Phase 7 of 7.

Spec: `docs/specs/2026-05-20-keychain-token-storage.md`
```

Then:

```bash
gh pr create --title "OS keychain token storage (security hardening phase 7)" --body-file /tmp/pr-body-phase7.md
rm /tmp/pr-body-phase7.md
```

---

## Self-Review Notes

- [ ] Spec coverage:
  - tokenBackend interface — Task 2.
  - yamlBackend extracted — Task 2.
  - selectBackend env-var parsing — Tasks 3, 4.
  - memoryBackend (test-only) — Task 3.
  - keychainBackend + probe — Task 4.
  - CredentialsStore wrapper + auto-migration — Task 5.
  - Clear paranoia (both backends) — Task 5.
  - Test isolation (TestMain in every affected package) — Tasks 3, 6.
  - Live keychain smoke test — Task 7.
  - Docs (auth.md, README, walkthrough) — Task 8.
  - Sweep + PR — Tasks 9, 10.
- [ ] No placeholders. Every step shows concrete code or commands.
- [ ] Type consistency: `tokenBackend`, `newYAMLBackend`, `newKeychainBackend`, `newMemoryBackend`, `keychainServiceName`, `keychainProbeAccount`, `probeKeychain` all spelled consistently across tasks.
- [ ] Each task commits independently.
