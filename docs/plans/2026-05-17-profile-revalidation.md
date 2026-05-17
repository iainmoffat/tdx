# Profile Revalidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject tampered profile configs at every read boundary. `ProfileStore.Load` filters out invalid profiles with a stderr warning; `ProfileStore.GetProfile` revalidates as a defense-in-depth backstop; `tdx.NewClient` enforces HTTPS (with the same loopback HTTP escape `Profile.Validate` already allows).

**Architecture:** Three minimal, independent changes — one in `internal/config/profiles.go` for `Load`, one in the same file for `GetProfile`, and one in `internal/tdx/client.go` for `NewClient`. All three reuse existing `Profile.Validate` policy: scheme must be `https` (or `http` to a loopback host). No new packages, no new sentinels.

**Tech Stack:** Go 1.26.2; testify/require. No new dependencies.

**Spec:** [`docs/specs/2026-05-17-profile-revalidation.md`](../specs/2026-05-17-profile-revalidation.md)

---

## File Structure

After this plan completes:

```
internal/
├── config/
│   ├── profiles.go            # MODIFY: Load filters invalid + warns; GetProfile inner Validate
│   └── profiles_test.go       # MODIFY: 2 new tests (Load skip; GetProfile tampered)
└── tdx/
    ├── client.go              # MODIFY: NewClient HTTPS check + isLoopbackHost helper
    └── client_test.go         # MODIFY: 4 new HTTPS-scheme tests

docs/
└── manual-tests/
    └── 2026-05-17-profile-revalidation-walkthrough.md  # CREATE
```

## Branch + Versioning

- Branch: `profile-revalidation` (Task 0)
- Version: **v0.23.0** — minor; behavior tightening. Clean config.yaml unaffected. Tampered config refuses.

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. Main is at `a8b153d` (the spec commit).

- [ ] **Step 2: Create branch**

```bash
git checkout -b profile-revalidation
```

Expected: `Switched to a new branch 'profile-revalidation'`.

---

## Task 1: `Load` skips invalid profiles and warns

**Files:**
- Modify: `internal/config/profiles.go:29-42` (`Load`)
- Modify: `internal/config/profiles_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/config/profiles_test.go`:

```go
func TestProfileStore_Load_SkipsInvalidProfile(t *testing.T) {
	dir := t.TempDir()
	store := NewProfileStore(Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})

	yamlBody := `defaultProfile: good
profiles:
  - name: good
    tenantBaseURL: https://example.com/
  - name: bad
    tenantBaseURL: http://attacker.example/
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlBody), 0o600))

	// Capture stderr to assert the warning.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	cfg, err := store.Load()
	_ = w.Close()
	os.Stderr = oldStderr
	stderr, _ := io.ReadAll(r)

	require.NoError(t, err)
	require.Len(t, cfg.Profiles, 1)
	require.Equal(t, "good", cfg.Profiles[0].Name)
	require.Contains(t, string(stderr), `warning: skipping invalid profile "bad"`)
}
```

Verify the test file's imports include `"io"`, `"os"`, `"path/filepath"`. Add any that are missing.

- [ ] **Step 2: Run test — must FAIL**

```bash
go test ./internal/config/... -run TestProfileStore_Load_SkipsInvalidProfile -v
```

Expected: `cfg.Profiles` has length 2 because `Load` doesn't filter today.

- [ ] **Step 3: Update `Load`**

In `internal/config/profiles.go`, replace the existing `Load` (lines 29-42):

```go
// Load returns the current profile config, or an empty config if none exists.
// Profiles that fail Validate are dropped with a stderr warning so a tampered
// config.yaml cannot redirect later API calls to an attacker-controlled URL.
func (s *ProfileStore) Load() (ProfileConfig, error) {
	data, err := os.ReadFile(s.paths.ConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return ProfileConfig{}, nil
	}
	if err != nil {
		return ProfileConfig{}, fmt.Errorf("read config: %w", err)
	}
	var cfg ProfileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProfileConfig{}, fmt.Errorf("parse config: %w", err)
	}
	valid := cfg.Profiles[:0]
	for _, p := range cfg.Profiles {
		if err := p.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping invalid profile %q: %v\n", p.Name, err)
			continue
		}
		valid = append(valid, p)
	}
	cfg.Profiles = valid
	return cfg, nil
}
```

No new imports — `os`, `errors`, `fmt`, `yaml` are already imported.

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/config/... -v
```

All existing tests pass (they construct profiles via `AddProfile` which validates first, so they're never invalid) plus the new `TestProfileStore_Load_SkipsInvalidProfile`.

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/config/profiles.go internal/config/profiles_test.go
git commit -m "feat(config): Load skips invalid profiles with stderr warning"
```

No `Co-Authored-By` trailer.

---

## Task 2: `GetProfile` inner `Validate` (defense in depth)

**Files:**
- Modify: `internal/config/profiles.go:135-150` (`GetProfile`)
- Modify: `internal/config/profiles_test.go`

After Task 1, `Load` already filters out invalid profiles, so a hand-tampered config.yaml never produces an invalid entry visible to `GetProfile`. The inner `Validate` is a backstop: if a future caller bypasses `Load` (e.g. constructs `cfg.Profiles` directly) or if `Load`'s filter has a bug, `GetProfile` still refuses.

- [ ] **Step 1: Write failing test**

Append to `internal/config/profiles_test.go`:

```go
func TestProfileStore_GetProfile_RejectsTamperedHTTPS(t *testing.T) {
	dir := t.TempDir()
	store := NewProfileStore(Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})

	yamlBody := "defaultProfile: tampered\nprofiles:\n  - name: tampered\n    tenantBaseURL: http://attacker.example/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlBody), 0o600))

	// Suppress the warning Load will print so test output stays quiet.
	oldStderr := os.Stderr
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stderr = devnull
	_, err := store.GetProfile("tampered")
	os.Stderr = oldStderr
	_ = devnull.Close()

	// Load skips the invalid profile, so GetProfile sees nothing and returns ErrProfileNotFound.
	// The bearer token never leaves — end-state matches the spec's "fail closed" intent.
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrProfileNotFound)
}
```

- [ ] **Step 2: Run test — must FAIL or PASS**

```bash
go test ./internal/config/... -run TestProfileStore_GetProfile_RejectsTamperedHTTPS -v
```

After Task 1's `Load` change, this test may PASS even without modifying `GetProfile` — because `Load` already filtered the tampered profile, so `GetProfile("tampered")` returns `ErrProfileNotFound` from the not-found branch. That's the spec-described outcome: the defense-in-depth backstop is for a future-bug scenario, not the steady-state user flow. The test still locks in the correct end-state behavior.

If the test passes already at this step, proceed to Step 3 anyway — we add the `Validate` backstop now so a future change to `Load` cannot regress this invariant.

- [ ] **Step 3: Update `GetProfile`**

In `internal/config/profiles.go`, replace the existing `GetProfile` (lines 135-150):

```go
// GetProfile returns a profile by name, or ErrProfileNotFound. The returned
// profile is also re-validated (defense in depth — Load already filters, but
// this guards against future code paths that bypass Load).
func (s *ProfileStore) GetProfile(name string) (domain.Profile, error) {
	if err := domain.ValidateArtifactName(name); err != nil {
		return domain.Profile{}, err
	}
	cfg, err := s.Load()
	if err != nil {
		return domain.Profile{}, err
	}
	for _, p := range cfg.Profiles {
		if p.Name == name {
			if err := p.Validate(); err != nil {
				return domain.Profile{}, fmt.Errorf("stored profile %q is invalid: %w", name, err)
			}
			return p, nil
		}
	}
	return domain.Profile{}, fmt.Errorf("%w: %s", domain.ErrProfileNotFound, name)
}
```

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/config/... -v
```

All tests pass. `gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/config/profiles.go internal/config/profiles_test.go
git commit -m "feat(config): GetProfile inner Validate as defense-in-depth backstop"
```

---

## Task 3: `tdx.NewClient` enforces HTTPS

**Files:**
- Modify: `internal/tdx/client.go:27-43` (`NewClient`)
- Modify: `internal/tdx/client_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/tdx/client_test.go`:

```go
func TestNewClient_AcceptsHTTPS(t *testing.T) {
	_, err := NewClient("https://example.com", "tok")
	require.NoError(t, err)
}

func TestNewClient_AcceptsLoopbackHTTP(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1:8080", "http://localhost:8080", "http://[::1]:8080"} {
		t.Run(u, func(t *testing.T) {
			_, err := NewClient(u, "tok")
			require.NoError(t, err)
		})
	}
}

func TestNewClient_RejectsHTTPNonLoopback(t *testing.T) {
	_, err := NewClient("http://attacker.example", "tok")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must use https")
}

func TestNewClient_RejectsExoticScheme(t *testing.T) {
	_, err := NewClient("file:///etc/passwd", "tok")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must use https")
}
```

- [ ] **Step 2: Run tests — must FAIL**

```bash
go test ./internal/tdx/... -run TestNewClient -v
```

`AcceptsHTTPS` and `AcceptsLoopbackHTTP` pass today (NewClient accepts anything with scheme+host). `RejectsHTTPNonLoopback` and `RejectsExoticScheme` fail.

- [ ] **Step 3: Update `NewClient`**

In `internal/tdx/client.go`, replace the existing `NewClient` (lines 26-43) with:

```go
// NewClient validates the base URL and returns a ready client.
//
// HTTPS is required. The only exception is http:// to a loopback host
// (localhost / 127.0.0.1 / ::1) so httptest servers continue to work.
// This mirrors the Profile.Validate scheme policy.
func NewClient(baseURL, token string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("base url must be absolute: %q", baseURL)
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname())) {
		return nil, fmt.Errorf("base url must use https: %q", baseURL)
	}
	return &Client{
		base:          u,
		token:         token,
		http:          &http.Client{Timeout: 30 * time.Second},
		maxRetries:    3,
		retryAfterCap: 30 * time.Second,
		userAgent:     "tdx/0.1",
	}, nil
}

// isLoopbackHost reports whether host is a loopback literal. Mirrors the
// helper in internal/domain/profile.go — duplicated here so this package
// doesn't import internal/domain. Update both if you add an entry.
func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
```

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/tdx/... -v
```

All tests pass — including the existing `TestClient_RejectsInvalidBaseURL` (which uses `"not a url"`, no scheme, caught by the existing absolute-URL check).

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Add a parity comment in `internal/domain/profile.go`**

In `internal/domain/profile.go`, find the existing `isLoopbackHost` function and prepend a one-line WHY comment referencing the parallel helper. The function body is unchanged:

```go
// isLoopbackHost reports whether host is a loopback literal. Mirrored in
// internal/tdx/client.go — update both if you add an entry.
func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/tdx/client.go internal/tdx/client_test.go internal/domain/profile.go
git commit -m "feat(tdx): NewClient enforces HTTPS (loopback HTTP escape, mirrors Profile.Validate)"
```

---

## Task 4: Full test + lint sweep

**Files:** none

- [ ] **Step 1: Run the full suite**

```bash
go test ./... -race && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green. No gofmt output. No vet warnings. No lint warnings.

- [ ] **Step 2: If failures appear, fix in place and commit per-issue**

Common gotchas:
- A service test that calls `tdx.NewClient` directly with a non-HTTPS, non-loopback URL — should be updated to use `httptest.NewServer` (which yields loopback HTTP) or `https://`. Likely none exist; all existing service tests already use `httptest.NewServer`.
- A test that asserts `Load` returns N profiles when the seed file has tampered ones — should be updated; none expected.

---

## Task 5: Manual walkthrough doc

**Files:**
- Create: `docs/manual-tests/2026-05-17-profile-revalidation-walkthrough.md`

- [ ] **Step 1: Write walkthrough**

Create `docs/manual-tests/2026-05-17-profile-revalidation-walkthrough.md`:

```markdown
# Profile Revalidation Walkthrough (v0.23.0)

Spec: [`docs/specs/2026-05-17-profile-revalidation.md`](../specs/2026-05-17-profile-revalidation.md)

## Setup

Back up your real config first:

    cp ~/.config/tdx/config.yaml ~/.config/tdx/config.yaml.bak

## Step 1: Flip HTTPS to HTTP — Load skips and warns

Edit `~/.config/tdx/config.yaml` and change the active profile's `tenantBaseURL`
from `https://yourorg.teamdynamix.com/` (or whatever you have) to
`http://attacker.example/`.

Run:

    tdx auth status

Expected:
- stderr: `warning: skipping invalid profile "<name>": invalid profile: tenantBaseURL must use https`
- the command fails (likely "profile not found: <name>" because Load skipped it
  and ResolveProfile can't find the default).

## Step 2: file:// scheme — same outcome

Edit `tenantBaseURL` to `file:///etc/passwd`. Run `tdx auth status` again.

Expected: same warning shape; same failure.

## Step 3: Restore — everything works

    cp ~/.config/tdx/config.yaml.bak ~/.config/tdx/config.yaml

Run:

    tdx auth status

Expected: normal output. No stderr warnings.

## Step 4: tdx.NewClient guard — direct check

Not a CLI test, but worth noting: the same policy now lives in `tdx.NewClient`.
Any future code path that constructs a client with a non-HTTPS, non-loopback
URL fails fast at construction.
```

- [ ] **Step 2: Commit**

```bash
git add docs/manual-tests/2026-05-17-profile-revalidation-walkthrough.md
git commit -m "docs: walkthrough for profile revalidation (v0.23.0)"
```

---

## Task 6: Push branch and create PR

**Files:** none

- [ ] **Step 1: Push branch**

```bash
git push -u origin profile-revalidation
```

- [ ] **Step 2: Create PR**

Write the body to `/tmp/pr-body-phase4.md`:

```markdown
## Summary

Phase 4 of the security hardening rollup. Addresses audit finding #3 (Medium: stored profile config trusted without revalidation).

- `ProfileStore.Load` now filters out profiles that fail `Profile.Validate`, with a stderr warning per skipped entry. Matches the Phase 2 List graceful-skip pattern.
- `ProfileStore.GetProfile` adds an inner `Validate` backstop — defense in depth, since `Load` already filters.
- `tdx.NewClient` enforces HTTPS with the same loopback-HTTP escape `Profile.Validate` allows. Protects future code paths that construct a client without going through the profile store.

## Test plan

- [x] `go test ./... -race` green
- [x] `go vet ./...`, `gofmt -l .`, `golangci-lint run ./...` all clean
- [ ] Live manual walkthrough at `docs/manual-tests/2026-05-17-profile-revalidation-walkthrough.md`

Closes: security audit finding #3.

Spec: `docs/specs/2026-05-17-profile-revalidation.md`
```

Then:

```bash
gh pr create --title "Profile revalidation (security hardening phase 4)" --body-file /tmp/pr-body-phase4.md
rm /tmp/pr-body-phase4.md
```

---

## Self-Review Notes

- [ ] Spec coverage:
  - `Load` skip-and-warn — Task 1.
  - `GetProfile` inner Validate — Task 2.
  - `tdx.NewClient` HTTPS enforcement (with loopback escape) — Task 3.
  - Tests: 1 Load + 1 GetProfile + 4 NewClient = 6 new test functions, matching the spec's test list.
  - Parity comment between the two `isLoopbackHost` copies — Task 3 step 5.
  - Walkthrough doc — Task 5.
- [ ] No placeholders. Each step has concrete code or commands.
- [ ] Type consistency: `Profile.Validate`, `ValidateArtifactName`, `ErrProfileNotFound`, `isLoopbackHost` all referenced consistently across tasks.
- [ ] Each task is self-contained.
