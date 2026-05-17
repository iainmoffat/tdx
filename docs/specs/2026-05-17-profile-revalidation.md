# Security Hardening — Profile Revalidation on Load

**Date:** 2026-05-17
**Phase:** 4 of 7 (security hardening rollup; see [Phase 1 spec](2026-05-15-report-fanout-caps.md) for the full roadmap)
**Goal:** Reject tampered profile configs at every read boundary so a malicious `config.yaml` cannot redirect bearer-token API calls to a non-HTTPS or attacker-controlled endpoint.

## Motivation

A security audit on 2026-05-15 identified profile-trust-on-load as a Medium-severity finding (`#3`). Today:

- `Profile.Validate()` runs on `AddProfile` and `UpdateProfile` — HTTPS-only (with a loopback HTTP escape), host non-empty, name allowlist (added in Phase 2).
- But `ProfileStore.Load()` and `ProfileStore.GetProfile()` decode YAML and return the profile **without revalidating**. A user (or attacker with write access) can hand-edit `~/.config/tdx/config.yaml` to flip `tenantBaseURL` from `https://yourorg.teamdynamix.com/` to `http://attacker.example/` and every subsequent `tdx ...` command will send the bearer token in cleartext to that endpoint.
- `tdx.NewClient(baseURL, token)` accepts ANY scheme — `http://`, `file://`, `gopher://` — as long as scheme and host are non-empty. A direct caller bypassing the profile store has no second-line defense.

## Threat model

- **Malicious local actor with FS write access to `~/.config/tdx/config.yaml`** — can rewrite `tenantBaseURL`. Defended by revalidation at `Load` and `GetProfile`.
- **Compromised dependency that calls `tdx.NewClient` directly** — bypasses the profile store. Defended by HTTPS enforcement in `NewClient`.
- **Out of scope:** attacker with full filesystem access (can also read the credentials file directly).

## Policy: three layered defenses

1. **`GetProfile` fails closed.** After looking up by name, call `p.Validate()`. On failure return a wrapped error. Every service that resolves a profile before `tdx.NewClient` (auth/time/people/project/ticket) goes through this gate.
2. **`Load` skips and warns.** Iterate decoded profiles, drop invalid entries, print `warning: skipping invalid profile %q: %v` to stderr for each. Matches the Phase 2 `List` graceful-skip pattern for templates and drafts. `auth profile list` still shows the valid set; user sees warnings for tampered entries.
3. **`tdx.NewClient` enforces HTTPS.** Reject any base URL whose scheme is not `https`, with the same loopback-HTTP escape `Profile.Validate` already allows. Defense in depth — protects against future code paths that construct a client without going through the profile store.

## `GetProfile` revalidates and fails closed

`internal/config/profiles.go:135` (the existing `GetProfile`). After finding the matching profile by name, call `Validate` before returning:

```go
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

Note: in practice, with the Section "`Load` skips and warns" change below, `Load` has already filtered out invalid profiles, so the inner `Validate()` check in `GetProfile` rarely fires from disk — it's a backstop in case `Load` (or a test) returns an invalid profile in `cfg.Profiles`. The check is intentionally kept here for defense in depth.

Error chain: `p.Validate()` returns `ErrInvalidProfile` (the existing sentinel from `internal/domain/errors.go`). `errors.Is(err, domain.ErrInvalidProfile)` continues to work.

## `Load` skips and warns

`internal/config/profiles.go:29`. After YAML unmarshal:

```go
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

In-place filter via `cfg.Profiles[:0]`. Order preserved.

If `cfg.DefaultProfile` names a profile that was skipped, downstream `ResolveProfile` calls `GetProfile(defaultName)` which returns `ErrProfileNotFound` — coherent end-to-end.

`auth profile list` calls `Load` directly and renders only the valid profiles; stderr warnings appear above the table.

## `tdx.NewClient` enforces HTTPS

`internal/tdx/client.go:27`. Adopt the same scheme policy `Profile.Validate` uses — `https`, OR `http` for loopback hosts only:

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

func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
```

`internal/domain/profile.go` already has an unexported `isLoopbackHost` with the same body. We duplicate it here rather than promoting to a shared exported helper because `internal/tdx/` deliberately doesn't import `internal/domain/` — it's a low-level HTTP client. Two 6-line copies; reviewer comments on each side reference the other for parity.

## Tests

### `GetProfile` revalidation (`internal/config/profiles_test.go`)

```go
func TestProfileStore_GetProfile_RejectsTamperedHTTPS(t *testing.T) {
	dir := t.TempDir()
	store := NewProfileStore(Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})

	// Write a config.yaml with an http:// profile by hand (bypasses AddProfile validation).
	yamlBody := "defaultProfile: tampered\nprofiles:\n  - name: tampered\n    tenantBaseURL: http://attacker.example/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlBody), 0o600))

	_, err := store.GetProfile("tampered")
	// Load skips the invalid profile, so GetProfile sees nothing and returns ErrProfileNotFound.
	// This is the right end-state: the bearer token never leaves.
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrProfileNotFound)
}
```

### `Load` skip-and-warn (`internal/config/profiles_test.go`)

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

### `tdx.NewClient` HTTPS

In `internal/tdx/client_test.go` (or whatever client-level test file exists; create if needed):

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
}
```

### Walkthrough doc

`docs/manual-tests/2026-05-17-profile-revalidation-walkthrough.md`:

- Hand-edit `~/.config/tdx/config.yaml` to flip an existing profile's `tenantBaseURL` from `https://` to `http://` → next command warns to stderr (from `Load`) and fails with profile-not-found (since the entry was skipped).
- Hand-add a profile with `tenantBaseURL: file:///etc/passwd` → same outcome.
- Restoring the file to a valid scheme brings the profile back.

## Branch / version

- Branch: `profile-revalidation`
- Version: **v0.23.0** — minor; behavior tightening. Reads from a clean config.yaml are unaffected. A tampered config that previously would have leaked a bearer token now refuses.

## Open questions

None. All design decisions settled in the 2026-05-17 brainstorming session.
