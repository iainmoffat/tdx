# `tdx` Phase 1 — Auth & Environment/Profile Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go project foundation, profile/credentials storage, TD HTTP client skeleton, and the `tdx auth` + `tdx config` command trees, ending with a working `tdx auth login` using the paste-token flow against `https://ufl.teamdynamix.com/`.

**Architecture:** Cobra CLI → thin command layer → services (`authsvc`) → config/client primitives. All business logic lives in functions that take explicit dependencies (paths, readers, writers), so commands stay thin and everything is unit-testable. The TD HTTP client is a generic typed wrapper with token plumbing and `Retry-After` handling; no time-specific endpoints in this phase.

**Tech stack:** Go 1.22+, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `github.com/stretchr/testify/require`, `golang.org/x/term` (token input without echo), standard library (`net/http`, `net/http/httptest`).

**Spec reference:** `docs/superpowers/specs/2026-04-10-tdx-framework-design.md` — this plan implements Phase 1 from §11 and its associated pieces of §2.1, §3, §5, §8.

**Out of scope (deferred to Phase 1B):**
- Loopback HTTP callback flow
- Device-code polling flow
- Any ergonomic browser-SSO path beyond "paste the token"

## File structure

```
tdx/
├── cmd/
│   └── tdx/
│       └── main.go                     # Task 1
├── internal/
│   ├── cli/
│   │   ├── root.go                     # Task 1
│   │   ├── version.go                  # Task 1
│   │   ├── auth/
│   │   │   ├── auth.go                 # Task 11 (parent cmd)
│   │   │   ├── profile.go              # Task 11
│   │   │   ├── status.go               # Task 12
│   │   │   ├── logout.go               # Task 13
│   │   │   └── login.go                # Task 14
│   │   └── config/
│   │       └── config.go               # Task 10
│   ├── config/
│   │   ├── paths.go                    # Task 3
│   │   ├── profiles.go                 # Task 4
│   │   └── credentials.go              # Task 5
│   ├── domain/
│   │   ├── profile.go                  # Task 2
│   │   ├── session.go                  # Task 2
│   │   └── errors.go                   # Task 2
│   ├── svc/
│   │   └── authsvc/
│   │       └── service.go              # Task 8
│   ├── tdx/
│   │   ├── client.go                   # Task 6
│   │   └── errors.go                   # Task 6
│   └── render/
│       └── render.go                   # Task 9
├── docs/
│   └── superpowers/
│       ├── specs/
│       └── plans/
│           └── 2026-04-10-tdx-phase-1-auth-foundation.md   # this file
├── go.mod                              # Task 1
└── go.sum                              # Task 1
```

**Principles carried from spec §5.8:**
- `domain/` has no imports outside stdlib.
- `tdx/` depends only on `domain/`.
- `config/` depends only on `domain/`.
- `svc/authsvc/` depends on `domain/`, `config/`, `tdx/`.
- `cli/*` depends on `svc/`, `domain/`, `render/`. Commands are thin; logic lives in `svc/`.

---

## Task 1: Project scaffolding + root command + version command

**Files:**
- Create: `go.mod`
- Create: `cmd/tdx/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/version.go`
- Test: `internal/cli/version_test.go`

- [ ] **Step 1: Initialize Go module and add the dependencies this task uses**

Run:
```bash
cd /Users/ipm/code/tdx
go mod init github.com/iainmoffat/tdx
go get github.com/spf13/cobra@latest
go get github.com/stretchr/testify@latest
```

Expected: `go.mod` and `go.sum` exist; `go.mod` lists cobra and testify as direct dependencies. **Do not** `go get` `yaml.v3` or `golang.org/x/term` yet — they are added by Tasks 4 and 14 when first imported. **Do not** run `go mod tidy` during Phase 1; each task runs targeted `go get` calls when it introduces a new import.

- [ ] **Step 2: Write a failing test for the `version` command**

Create `internal/cli/version_test.go`:

```go
package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCommand_PrintsVersionString(t *testing.T) {
	var out bytes.Buffer
	cmd := newVersionCmd("0.1.0-dev")
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "tdx 0.1.0-dev")
}
```

- [ ] **Step 3: Run the test and confirm it fails**

Run:
```bash
go test ./internal/cli/...
```

Expected: compile error, `newVersionCmd` undefined.

- [ ] **Step 4: Implement the version command**

Create `internal/cli/version.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the tdx version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "tdx %s\n", version)
			return err
		},
	}
}
```

- [ ] **Step 5: Create the root command that wires version in**

Create `internal/cli/root.go`:

```go
package cli

import "github.com/spf13/cobra"

// NewRootCmd returns the top-level tdx command.
// version is injected at build time by cmd/tdx/main.go.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "tdx",
		Short:         "Manage TeamDynamix time entries from the terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd(version))
	return root
}
```

- [ ] **Step 6: Create the entrypoint binary**

Create `cmd/tdx/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/iainmoffat/tdx/internal/cli"
)

// version is overridden at build time with -ldflags "-X main.version=..."
var version = "0.1.0-dev"

func main() {
	root := cli.NewRootCmd(version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tdx:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Run the test and the binary**

Run:
```bash
go test ./internal/cli/...
go build ./cmd/tdx
./tdx version
```

Expected: test passes; `./tdx version` prints `tdx 0.1.0-dev`.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum cmd/tdx/main.go internal/cli/root.go internal/cli/version.go internal/cli/version_test.go
git commit -m "feat: scaffold tdx project with root and version commands"
```

---

## Task 2: Domain types (Profile, Session, errors)

**Files:**
- Create: `internal/domain/profile.go`
- Create: `internal/domain/session.go`
- Create: `internal/domain/errors.go`
- Test: `internal/domain/profile_test.go`

- [ ] **Step 1: Write failing tests for Profile validation**

Create `internal/domain/profile_test.go`:

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfile_Validate_AcceptsValidProfile(t *testing.T) {
	p := Profile{
		Name:          "default",
		TenantBaseURL: "https://ufl.teamdynamix.com/",
	}
	require.NoError(t, p.Validate())
}

func TestProfile_Validate_RejectsEmptyName(t *testing.T) {
	p := Profile{
		Name:          "",
		TenantBaseURL: "https://ufl.teamdynamix.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsMissingURL(t *testing.T) {
	p := Profile{Name: "default"}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsNonHTTPSURL(t *testing.T) {
	p := Profile{
		Name:          "default",
		TenantBaseURL: "http://ufl.teamdynamix.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsNameWithSlash(t *testing.T) {
	p := Profile{
		Name:          "bad/name",
		TenantBaseURL: "https://ufl.teamdynamix.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/domain/...`
Expected: compile error, `Profile` and `ErrInvalidProfile` undefined.

- [ ] **Step 3: Implement errors**

Create `internal/domain/errors.go`:

```go
package domain

import "errors"

var (
	// ErrInvalidProfile indicates a profile failed structural validation.
	ErrInvalidProfile = errors.New("invalid profile")

	// ErrProfileNotFound indicates a lookup by name failed.
	ErrProfileNotFound = errors.New("profile not found")

	// ErrProfileExists indicates a name collision.
	ErrProfileExists = errors.New("profile already exists")

	// ErrNoCredentials indicates no stored token for a profile.
	ErrNoCredentials = errors.New("no credentials for profile")

	// ErrInvalidToken indicates a token failed server-side validation.
	ErrInvalidToken = errors.New("invalid token")
)
```

- [ ] **Step 4: Implement Profile**

Create `internal/domain/profile.go`:

```go
package domain

import (
	"fmt"
	"net/url"
	"strings"
)

// Profile names a TeamDynamix environment and its base URL.
type Profile struct {
	Name          string `yaml:"name"`
	TenantBaseURL string `yaml:"tenantBaseURL"`
}

// Validate returns nil if the profile is structurally sound.
func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidProfile)
	}
	if strings.ContainsAny(p.Name, "/\\ \t") {
		return fmt.Errorf("%w: name may not contain slashes or whitespace", ErrInvalidProfile)
	}
	if p.TenantBaseURL == "" {
		return fmt.Errorf("%w: tenantBaseURL is required", ErrInvalidProfile)
	}
	u, err := url.Parse(p.TenantBaseURL)
	if err != nil {
		return fmt.Errorf("%w: tenantBaseURL: %v", ErrInvalidProfile, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: tenantBaseURL must use https", ErrInvalidProfile)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: tenantBaseURL must include a host", ErrInvalidProfile)
	}
	return nil
}
```

- [ ] **Step 5: Implement Session**

Create `internal/domain/session.go`:

```go
package domain

// Session is an authenticated pairing of a Profile and its bearer token.
// Phase 1 does not track expiry or identity; those arrive in later phases.
type Session struct {
	Profile Profile
	Token   string
}

// HasToken returns true if the session carries a non-empty token.
func (s Session) HasToken() bool {
	return s.Token != ""
}
```

- [ ] **Step 6: Run tests and confirm they pass**

Run: `go test ./internal/domain/...`
Expected: all five profile tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): add Profile, Session, and sentinel errors"
```

---

## Task 3: Config paths helper (XDG resolution)

**Files:**
- Create: `internal/config/paths.go`
- Test: `internal/config/paths_test.go`

- [ ] **Step 1: Write failing tests for path resolution**

Create `internal/config/paths_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaths_RespectsTdxConfigHomeEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	p, err := ResolvePaths()
	require.NoError(t, err)
	require.Equal(t, dir, p.Root)
	require.Equal(t, filepath.Join(dir, "config.yaml"), p.ConfigFile)
	require.Equal(t, filepath.Join(dir, "credentials.yaml"), p.CredentialsFile)
	require.Equal(t, filepath.Join(dir, "templates"), p.TemplatesDir)
}

func TestPaths_FallsBackToXdgConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", dir)

	p, err := ResolvePaths()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "tdx"), p.Root)
}

func TestPaths_FallsBackToHomeDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)

	p, err := ResolvePaths()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".config", "tdx"), p.Root)
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/config/...`
Expected: compile error, `ResolvePaths` and `Paths` undefined.

- [ ] **Step 3: Implement paths resolution**

Create `internal/config/paths.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds resolved filesystem locations used by tdx.
type Paths struct {
	Root            string
	ConfigFile      string
	CredentialsFile string
	TemplatesDir    string
}

// ResolvePaths determines the tdx config root and derived file locations.
// Precedence: TDX_CONFIG_HOME > XDG_CONFIG_HOME/tdx > $HOME/.config/tdx.
func ResolvePaths() (Paths, error) {
	root, err := resolveRoot()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Root:            root,
		ConfigFile:      filepath.Join(root, "config.yaml"),
		CredentialsFile: filepath.Join(root, "credentials.yaml"),
		TemplatesDir:    filepath.Join(root, "templates"),
	}, nil
}

func resolveRoot() (string, error) {
	if v := os.Getenv("TDX_CONFIG_HOME"); v != "" {
		return v, nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "tdx"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tdx"), nil
}

// EnsureRoot creates the config root directory if it does not exist.
func (p Paths) EnsureRoot() error {
	if err := os.MkdirAll(p.Root, 0o700); err != nil {
		return fmt.Errorf("create config root: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests and confirm they pass**

Run: `go test ./internal/config/...`
Expected: all three tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/paths.go internal/config/paths_test.go
git commit -m "feat(config): add XDG-aware paths resolver"
```

---

## Task 4: Profile file IO (`config.yaml`)

**Files:**
- Create: `internal/config/profiles.go`
- Test: `internal/config/profiles_test.go`

- [ ] **Step 0: Add the YAML dependency**

This is the first task that imports `gopkg.in/yaml.v3`. Add it now so `go.sum` has the entry before the first build.

Run:
```bash
cd /Users/ipm/code/tdx
go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 1: Write failing tests for profile store round-trip**

Create `internal/config/profiles_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func writablePaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
}

func TestProfileStore_LoadEmptyReturnsNothing(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	cfg, err := s.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.Profiles)
	require.Equal(t, "", cfg.DefaultProfile)
}

func TestProfileStore_RoundTrip(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	in := ProfileConfig{
		DefaultProfile: "ufl",
		Profiles: []domain.Profile{
			{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"},
			{Name: "sandbox", TenantBaseURL: "https://sandbox.teamdynamix.com/"},
		},
	}
	require.NoError(t, s.Save(in))

	out, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestProfileStore_AddProfile(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	err := s.AddProfile(domain.Profile{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"})
	require.NoError(t, err)

	cfg, err := s.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Profiles, 1)
	require.Equal(t, "ufl", cfg.DefaultProfile, "first profile becomes default")
}

func TestProfileStore_AddDuplicateRejected(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	prof := domain.Profile{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"}
	require.NoError(t, s.AddProfile(prof))
	err := s.AddProfile(prof)
	require.ErrorIs(t, err, domain.ErrProfileExists)
}

func TestProfileStore_RemoveProfile(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	require.NoError(t, s.AddProfile(domain.Profile{Name: "a", TenantBaseURL: "https://a.teamdynamix.com/"}))
	require.NoError(t, s.AddProfile(domain.Profile{Name: "b", TenantBaseURL: "https://b.teamdynamix.com/"}))
	require.NoError(t, s.SetDefault("a"))

	require.NoError(t, s.RemoveProfile("a"))

	cfg, err := s.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Profiles, 1)
	require.Equal(t, "b", cfg.Profiles[0].Name)
	require.Equal(t, "b", cfg.DefaultProfile, "default rolls over to remaining profile")
}

func TestProfileStore_RemoveMissing(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	err := s.RemoveProfile("nope")
	require.ErrorIs(t, err, domain.ErrProfileNotFound)
}

func TestProfileStore_GetProfile(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	prof := domain.Profile{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"}
	require.NoError(t, s.AddProfile(prof))

	got, err := s.GetProfile("ufl")
	require.NoError(t, err)
	require.Equal(t, prof, got)
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/config/...`
Expected: compile errors, `NewProfileStore`, `ProfileConfig` undefined.

- [ ] **Step 3: Implement ProfileStore**

Create `internal/config/profiles.go`:

```go
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/iainmoffat/tdx/internal/domain"
	"gopkg.in/yaml.v3"
)

// ProfileConfig is the on-disk shape of config.yaml.
type ProfileConfig struct {
	DefaultProfile string           `yaml:"defaultProfile"`
	Profiles       []domain.Profile `yaml:"profiles"`
}

// ProfileStore reads and writes the profile configuration file.
type ProfileStore struct {
	paths Paths
}

// NewProfileStore constructs a store rooted at the given paths.
func NewProfileStore(paths Paths) *ProfileStore {
	return &ProfileStore{paths: paths}
}

// Load returns the current profile config, or an empty config if none exists.
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
	return cfg, nil
}

// Save writes the config atomically (write temp, rename).
func (s *ProfileStore) Save(cfg ProfileConfig) error {
	if err := s.paths.EnsureRoot(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := s.paths.ConfigFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.paths.ConfigFile); err != nil {
		return fmt.Errorf("finalize config: %w", err)
	}
	return nil
}

// AddProfile inserts a new profile; rejects name collisions.
// The first profile added becomes the default.
func (s *ProfileStore) AddProfile(p domain.Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	for _, existing := range cfg.Profiles {
		if existing.Name == p.Name {
			return fmt.Errorf("%w: %s", domain.ErrProfileExists, p.Name)
		}
	}
	cfg.Profiles = append(cfg.Profiles, p)
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = p.Name
	}
	return s.Save(cfg)
}

// RemoveProfile deletes a profile by name.
// If the removed profile was the default, another remaining profile becomes default.
func (s *ProfileStore) RemoveProfile(name string) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	idx := -1
	for i, p := range cfg.Profiles {
		if p.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("%w: %s", domain.ErrProfileNotFound, name)
	}
	cfg.Profiles = append(cfg.Profiles[:idx], cfg.Profiles[idx+1:]...)
	if cfg.DefaultProfile == name {
		if len(cfg.Profiles) > 0 {
			cfg.DefaultProfile = cfg.Profiles[0].Name
		} else {
			cfg.DefaultProfile = ""
		}
	}
	return s.Save(cfg)
}

// GetProfile returns a profile by name, or ErrProfileNotFound.
func (s *ProfileStore) GetProfile(name string) (domain.Profile, error) {
	cfg, err := s.Load()
	if err != nil {
		return domain.Profile{}, err
	}
	for _, p := range cfg.Profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return domain.Profile{}, fmt.Errorf("%w: %s", domain.ErrProfileNotFound, name)
}

// SetDefault sets the default profile, verifying it exists.
func (s *ProfileStore) SetDefault(name string) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	found := false
	for _, p := range cfg.Profiles {
		if p.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: %s", domain.ErrProfileNotFound, name)
	}
	cfg.DefaultProfile = name
	return s.Save(cfg)
}
```

- [ ] **Step 4: Run tests and confirm they pass**

Run: `go test ./internal/config/...`
Expected: all profile store tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/profiles.go internal/config/profiles_test.go
git commit -m "feat(config): add profile store with YAML round-trip"
```

---

## Task 5: Credentials file IO (`credentials.yaml`, `0600`)

**Files:**
- Create: `internal/config/credentials.go`
- Test: `internal/config/credentials_test.go`

- [ ] **Step 1: Write failing tests for credentials store**

Create `internal/config/credentials_test.go`:

```go
package config

import (
	"os"
	"runtime"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCredentialsStore_MissingFileReturnsNoCredentials(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	_, err := s.GetToken("ufl")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestCredentialsStore_SetAndGetToken(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "abc123"))

	token, err := s.GetToken("ufl")
	require.NoError(t, err)
	require.Equal(t, "abc123", token)
}

func TestCredentialsStore_FileIsZeroSixHundred(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix perms not meaningful on Windows")
	}
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "abc123"))

	info, err := os.Stat(p.CredentialsFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCredentialsStore_OverwriteRekeys(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "first"))
	require.NoError(t, s.SetToken("ufl", "second"))

	token, err := s.GetToken("ufl")
	require.NoError(t, err)
	require.Equal(t, "second", token)
}

func TestCredentialsStore_MultipleProfiles(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "token-ufl"))
	require.NoError(t, s.SetToken("sandbox", "token-sandbox"))

	tu, err := s.GetToken("ufl")
	require.NoError(t, err)
	require.Equal(t, "token-ufl", tu)

	ts, err := s.GetToken("sandbox")
	require.NoError(t, err)
	require.Equal(t, "token-sandbox", ts)
}

func TestCredentialsStore_ClearToken(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "abc"))
	require.NoError(t, s.ClearToken("ufl"))

	_, err := s.GetToken("ufl")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestCredentialsStore_ClearMissingIsNoop(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.ClearToken("never-existed"))
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/config/...`
Expected: compile error, `NewCredentialsStore` undefined.

- [ ] **Step 3: Implement CredentialsStore**

Create `internal/config/credentials.go`:

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
// Tokens are keyed by profile name.
type credentialsFile struct {
	Tokens map[string]string `yaml:"tokens"`
}

// CredentialsStore persists bearer tokens per profile with 0600 perms.
type CredentialsStore struct {
	paths Paths
}

// NewCredentialsStore constructs a store rooted at the given paths.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	return &CredentialsStore{paths: paths}
}

// GetToken returns the token for the named profile, or ErrNoCredentials.
func (s *CredentialsStore) GetToken(profile string) (string, error) {
	cf, err := s.load()
	if err != nil {
		return "", err
	}
	token, ok := cf.Tokens[profile]
	if !ok || token == "" {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	return token, nil
}

// SetToken writes or replaces the token for the named profile.
func (s *CredentialsStore) SetToken(profile, token string) error {
	cf, err := s.load()
	if err != nil {
		return err
	}
	if cf.Tokens == nil {
		cf.Tokens = make(map[string]string)
	}
	cf.Tokens[profile] = token
	return s.save(cf)
}

// ClearToken removes the token for the named profile. Missing is not an error.
func (s *CredentialsStore) ClearToken(profile string) error {
	cf, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := cf.Tokens[profile]; !ok {
		return nil
	}
	delete(cf.Tokens, profile)
	return s.save(cf)
}

func (s *CredentialsStore) load() (credentialsFile, error) {
	data, err := os.ReadFile(s.paths.CredentialsFile)
	if errors.Is(err, os.ErrNotExist) {
		return credentialsFile{Tokens: map[string]string{}}, nil
	}
	if err != nil {
		return credentialsFile{}, fmt.Errorf("read credentials: %w", err)
	}
	if err := enforcePerms(s.paths.CredentialsFile); err != nil {
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

func (s *CredentialsStore) save(cf credentialsFile) error {
	if err := s.paths.EnsureRoot(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cf)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	tmp := s.paths.CredentialsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Rename(tmp, s.paths.CredentialsFile); err != nil {
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

- [ ] **Step 4: Run tests and confirm they pass**

Run: `go test ./internal/config/...`
Expected: all credentials tests pass on macOS/Linux; perm test is skipped on Windows.

- [ ] **Step 5: Commit**

```bash
git add internal/config/credentials.go internal/config/credentials_test.go
git commit -m "feat(config): add credentials store with 0600 enforcement"
```

---

## Task 6: TD HTTP client skeleton

**Files:**
- Create: `internal/tdx/client.go`
- Create: `internal/tdx/errors.go`
- Test: `internal/tdx/client_test.go`

- [ ] **Step 1: Write failing tests for the client's request plumbing**

Create `internal/tdx/client_test.go`:

```go
package tdx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient_AttachesBearerToken(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "token-xyz")
	require.NoError(t, err)

	_, err = c.Do(context.Background(), http.MethodGet, "/api/ping", nil)
	require.NoError(t, err)
	require.Equal(t, "Bearer token-xyz", seenAuth)
}

func TestClient_ReturnsBodyOn2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	body, err := c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	require.NoError(t, err)
	require.Contains(t, string(body), `"ok":true`)
}

func TestClient_401ReturnsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"Message":"bad token"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	_, err = c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	require.ErrorIs(t, err, ErrUnauthorized)
}

func TestClient_4xxReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad input`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	_, err = c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.Status)
	require.Contains(t, apiErr.Message, "bad input")
}

func TestClient_RetryAfterOn429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)
	c.maxRetries = 2
	c.retryAfterCap = time.Millisecond

	_, err = c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

func TestClient_RejectsInvalidBaseURL(t *testing.T) {
	_, err := NewClient("not a url", "t")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/tdx/...`
Expected: compile errors, `NewClient`, `APIError`, `ErrUnauthorized` undefined.

- [ ] **Step 3: Implement errors**

Create `internal/tdx/errors.go`:

```go
package tdx

import (
	"errors"
	"fmt"
)

// ErrUnauthorized is returned when TD responds with 401.
var ErrUnauthorized = errors.New("tdx: unauthorized")

// APIError wraps non-2xx responses with structured detail.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("tdx api: %d: %s", e.Status, e.Message)
}
```

- [ ] **Step 4: Implement the HTTP client**

Create `internal/tdx/client.go`:

```go
package tdx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a thin typed wrapper around net/http for the TeamDynamix Web API.
type Client struct {
	base          *url.URL
	token         string
	http          *http.Client
	maxRetries    int
	retryAfterCap time.Duration
	userAgent     string
}

// NewClient validates the base URL and returns a ready client.
func NewClient(baseURL, token string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("base url must be absolute: %q", baseURL)
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

// Do performs an authenticated request and returns the response body on 2xx.
// On 429 it honours Retry-After up to retryAfterCap, retrying up to maxRetries.
// On 401 it returns ErrUnauthorized. On other non-2xx it returns an *APIError.
// body may be nil.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.doOnce(ctx, method, path, bodyBytes)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			wait := parseRetryAfter(resp.Header.Get("Retry-After"), c.retryAfterCap)
			_ = resp.Body.Close()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			lastErr = &APIError{Status: resp.StatusCode, Message: "rate limited"}
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, strings.TrimSpace(string(respBody)))
		}
		return nil, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("request failed after retries")
	}
	return nil, lastErr
}

func (c *Client) doOnce(ctx context.Context, method, path string, bodyBytes []byte) (*http.Response, error) {
	full := c.base.ResolveReference(&url.URL{Path: strings.TrimLeft(path, "/")})
	// Preserve the base path if present.
	if c.base.Path != "" && !strings.HasPrefix(path, "/") {
		full = c.base.ResolveReference(&url.URL{Path: path})
	}
	var reader io.Reader
	if bodyBytes != nil {
		reader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, full.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func parseRetryAfter(h string, cap time.Duration) time.Duration {
	if h == "" {
		return 1 * time.Second
	}
	if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
		d := time.Duration(n) * time.Second
		if d > cap {
			return cap
		}
		return d
	}
	return 1 * time.Second
}
```

- [ ] **Step 5: Run tests and confirm they pass**

Run: `go test ./internal/tdx/...`
Expected: all six client tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tdx/client.go internal/tdx/errors.go internal/tdx/client_test.go
git commit -m "feat(tdx): add HTTP client skeleton with auth and retry-after"
```

---

## Task 7: Token validation endpoint

**Note:** Phase 1 validates a pasted token by making any authenticated read call. We intentionally do not pick a TD-specific "whoami" endpoint yet — that is deferred to Phase 2 once the exact endpoint is verified. For Phase 1, the validation call is `GET /api/time/types` (confirmed to exist per the TD Time API reference) and we only care about the HTTP status, not the body.

**Files:**
- Modify: `internal/tdx/client.go` — add `Ping(ctx)` method
- Modify: `internal/tdx/client_test.go` — add test for Ping

- [ ] **Step 1: Write failing test for Ping**

Append to `internal/tdx/client_test.go`:

```go
func TestClient_PingCallsTimeTypes(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/api/time/types", seenPath)
}

func TestClient_PingOnUnauthorizedReturnsErrInvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	err = c.Ping(context.Background())
	require.ErrorIs(t, err, ErrUnauthorized)
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/tdx/...`
Expected: compile error, `Ping` undefined.

- [ ] **Step 3: Implement Ping**

Append to `internal/tdx/client.go`:

```go
// Ping makes a cheap authenticated call to verify the token is valid.
// It calls GET /api/time/types and discards the body; only the status matters.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Do(ctx, http.MethodGet, "/api/time/types", nil)
	return err
}
```

- [ ] **Step 4: Run tests and confirm they pass**

Run: `go test ./internal/tdx/...`
Expected: all client tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tdx/client.go internal/tdx/client_test.go
git commit -m "feat(tdx): add Ping for token validation via /api/time/types"
```

---

## Task 8: authsvc (session composition and operations)

**Files:**
- Create: `internal/svc/authsvc/service.go`
- Test: `internal/svc/authsvc/service_test.go`

- [ ] **Step 1: Write failing tests for authsvc**

Create `internal/svc/authsvc/service_test.go`:

```go
package authsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

type harness struct {
	paths  config.Paths
	srv    *httptest.Server
	tenant string
	svc    *Service
}

func newHarness(t *testing.T, handler http.HandlerFunc) *harness {
	t.Helper()
	dir := t.TempDir()
	paths := config.Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	svc := New(paths)
	return &harness{paths: paths, srv: srv, tenant: srv.URL, svc: svc}
}

func TestService_LoginWritesTokenAndProfile(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	sess, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "good-token",
	})
	require.NoError(t, err)
	require.Equal(t, "ufl-test", sess.Profile.Name)
	require.Equal(t, "good-token", sess.Token)

	// Verify the token was actually persisted.
	creds := config.NewCredentialsStore(h.paths)
	stored, err := creds.GetToken("ufl-test")
	require.NoError(t, err)
	require.Equal(t, "good-token", stored)
}

func TestService_LoginRejectsBadToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "nope",
	})
	require.ErrorIs(t, err, domain.ErrInvalidToken)

	// Nothing should have been written.
	creds := config.NewCredentialsStore(h.paths)
	_, err = creds.GetToken("ufl-test")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestService_StatusReportsNotAuthenticatedWhenNoToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {})

	profiles := config.NewProfileStore(h.paths)
	require.NoError(t, profiles.AddProfile(domain.Profile{
		Name:          "ufl-test",
		TenantBaseURL: h.tenant,
	}))

	status, err := h.svc.Status(context.Background(), "ufl-test")
	require.NoError(t, err)
	require.False(t, status.Authenticated)
	require.Equal(t, "ufl-test", status.Profile.Name)
}

func TestService_StatusVerifiesValidToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "good",
	})
	require.NoError(t, err)

	status, err := h.svc.Status(context.Background(), "ufl-test")
	require.NoError(t, err)
	require.True(t, status.Authenticated)
	require.True(t, status.TokenValid)
}

func TestService_StatusFlagsExpiredToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	profiles := config.NewProfileStore(h.paths)
	require.NoError(t, profiles.AddProfile(domain.Profile{
		Name:          "ufl-test",
		TenantBaseURL: h.tenant,
	}))
	creds := config.NewCredentialsStore(h.paths)
	require.NoError(t, creds.SetToken("ufl-test", "stale"))

	status, err := h.svc.Status(context.Background(), "ufl-test")
	require.NoError(t, err)
	require.True(t, status.Authenticated, "token file exists")
	require.False(t, status.TokenValid, "but server rejects it")
}

func TestService_LogoutClearsCredentialsOnly(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "t",
	})
	require.NoError(t, err)

	require.NoError(t, h.svc.Logout("ufl-test"))

	// Profile still exists.
	profiles := config.NewProfileStore(h.paths)
	_, err = profiles.GetProfile("ufl-test")
	require.NoError(t, err)

	// Credentials are gone.
	creds := config.NewCredentialsStore(h.paths)
	_, err = creds.GetToken("ufl-test")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/svc/authsvc/...`
Expected: compile errors, `Service`, `LoginInput`, `New`, etc. undefined.

- [ ] **Step 3: Implement the service**

Create `internal/svc/authsvc/service.go`:

```go
package authsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/tdx"
)

// Service orchestrates the auth-related operations.
// It composes the profile store, credentials store, and TD client.
type Service struct {
	paths       config.Paths
	profiles    *config.ProfileStore
	credentials *config.CredentialsStore
}

// New constructs an auth service rooted at the given paths.
func New(paths config.Paths) *Service {
	return &Service{
		paths:       paths,
		profiles:    config.NewProfileStore(paths),
		credentials: config.NewCredentialsStore(paths),
	}
}

// LoginInput describes a paste-token login attempt.
type LoginInput struct {
	ProfileName   string
	TenantBaseURL string
	Token         string
}

// Login validates the token against TD and persists it on success.
// It upserts the profile: creating it if new, updating tenant URL if existing.
func (s *Service) Login(ctx context.Context, in LoginInput) (domain.Session, error) {
	profile := domain.Profile{
		Name:          in.ProfileName,
		TenantBaseURL: in.TenantBaseURL,
	}
	if err := profile.Validate(); err != nil {
		return domain.Session{}, err
	}
	if in.Token == "" {
		return domain.Session{}, fmt.Errorf("%w: empty token", domain.ErrInvalidToken)
	}

	client, err := tdx.NewClient(profile.TenantBaseURL, in.Token)
	if err != nil {
		return domain.Session{}, err
	}
	if err := client.Ping(ctx); err != nil {
		if errors.Is(err, tdx.ErrUnauthorized) {
			return domain.Session{}, fmt.Errorf("%w: server rejected token", domain.ErrInvalidToken)
		}
		return domain.Session{}, fmt.Errorf("validate token: %w", err)
	}

	// Upsert profile.
	if existing, err := s.profiles.GetProfile(profile.Name); err == nil {
		if existing.TenantBaseURL != profile.TenantBaseURL {
			if err := s.profiles.RemoveProfile(profile.Name); err != nil {
				return domain.Session{}, err
			}
			if err := s.profiles.AddProfile(profile); err != nil {
				return domain.Session{}, err
			}
		}
	} else if errors.Is(err, domain.ErrProfileNotFound) {
		if err := s.profiles.AddProfile(profile); err != nil {
			return domain.Session{}, err
		}
	} else {
		return domain.Session{}, err
	}

	if err := s.credentials.SetToken(profile.Name, in.Token); err != nil {
		return domain.Session{}, err
	}

	return domain.Session{Profile: profile, Token: in.Token}, nil
}

// Logout clears the token for a profile. The profile itself is preserved.
// Missing credentials are not an error.
func (s *Service) Logout(profileName string) error {
	return s.credentials.ClearToken(profileName)
}

// Status describes the current state of an auth profile.
type Status struct {
	Profile       domain.Profile
	Authenticated bool // a token is stored
	TokenValid    bool // the stored token was accepted by the server (only set if Authenticated)
	ValidationErr string
}

// Status reports the state of a profile's credentials.
// If no token is stored, Authenticated is false and the server is not contacted.
// If a token is stored, the service makes a cheap Ping to verify it.
func (s *Service) Status(ctx context.Context, profileName string) (Status, error) {
	profile, err := s.profiles.GetProfile(profileName)
	if err != nil {
		return Status{}, err
	}
	status := Status{Profile: profile}

	token, err := s.credentials.GetToken(profileName)
	if errors.Is(err, domain.ErrNoCredentials) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.Authenticated = true

	client, err := tdx.NewClient(profile.TenantBaseURL, token)
	if err != nil {
		status.ValidationErr = err.Error()
		return status, nil
	}
	if err := client.Ping(ctx); err != nil {
		status.ValidationErr = err.Error()
		return status, nil
	}
	status.TokenValid = true
	return status, nil
}

// ResolveProfile picks a profile name from an explicit flag or the configured default.
// Used by commands that accept --profile.
func (s *Service) ResolveProfile(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	cfg, err := s.profiles.Load()
	if err != nil {
		return "", err
	}
	if cfg.DefaultProfile == "" {
		return "", fmt.Errorf("%w: no default profile configured", domain.ErrProfileNotFound)
	}
	return cfg.DefaultProfile, nil
}

// Profiles returns the underlying profile store (used by CLI commands that CRUD profiles).
func (s *Service) Profiles() *config.ProfileStore {
	return s.profiles
}

// Paths returns the resolved paths (used by the config command).
func (s *Service) Paths() config.Paths {
	return s.paths
}
```

- [ ] **Step 4: Run tests and confirm they pass**

Run: `go test ./internal/svc/authsvc/...`
Expected: all six authsvc tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/svc/authsvc/
git commit -m "feat(authsvc): add Login, Logout, Status, ResolveProfile"
```

---

## Task 9: Output helpers (human + JSON)

**Files:**
- Create: `internal/render/render.go`
- Test: `internal/render/render_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/render/render_test.go`:

```go
package render

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormat_ExplicitJSONWins(t *testing.T) {
	f := ResolveFormat(Flags{JSON: true}, os.Stdout)
	require.Equal(t, FormatJSON, f)
}

func TestFormat_ExplicitHumanWins(t *testing.T) {
	f := ResolveFormat(Flags{Human: true}, os.Stdout)
	require.Equal(t, FormatHuman, f)
}

func TestFormat_EnvOverride(t *testing.T) {
	t.Setenv("TDX_FORMAT", "json")
	f := ResolveFormat(Flags{}, os.Stdout)
	require.Equal(t, FormatJSON, f)
}

func TestJSON_EncodesPrettily(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, JSON(&buf, map[string]any{"hello": "world"}))
	require.Contains(t, buf.String(), "\"hello\": \"world\"")
}

func TestHuman_WritesRawLines(t *testing.T) {
	var buf bytes.Buffer
	Humanf(&buf, "profile: %s", "default")
	require.Equal(t, "profile: default\n", buf.String())
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/render/...`
Expected: compile error, `ResolveFormat`, `JSON`, `Humanf` undefined.

- [ ] **Step 3: Implement render helpers**

Create `internal/render/render.go`:

```go
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Format selects human vs machine output.
type Format int

const (
	FormatHuman Format = iota
	FormatJSON
)

// Flags carries the global format flags parsed from CLI.
type Flags struct {
	JSON  bool
	Human bool
}

// ResolveFormat picks the output format based on explicit flags, env, and TTY.
// Precedence: --json > --human > TDX_FORMAT > TTY detection (TTY->Human, pipe->JSON).
func ResolveFormat(f Flags, out *os.File) Format {
	if f.JSON {
		return FormatJSON
	}
	if f.Human {
		return FormatHuman
	}
	if env := os.Getenv("TDX_FORMAT"); env != "" {
		if env == "json" {
			return FormatJSON
		}
		return FormatHuman
	}
	if out == nil || !isTerminal(out) {
		return FormatJSON
	}
	return FormatHuman
}

// JSON writes v as pretty JSON followed by a newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Humanf writes a formatted line to w with a trailing newline.
func Humanf(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...)
	fmt.Fprintln(w)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
```

- [ ] **Step 4: Run tests and confirm they pass**

Run: `go test ./internal/render/...`
Expected: all five tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/render/
git commit -m "feat(render): add JSON/human format resolver and writers"
```

---

## Task 10: `tdx config` commands (path/show/init)

**Files:**
- Create: `internal/cli/config/config.go`
- Modify: `internal/cli/root.go` — wire in the `config` subcommand
- Test: `internal/cli/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cli/config/config_test.go`:

```go
package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathCmd_PrintsResolvedPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"path"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), dir)
	require.Contains(t, out.String(), "config.yaml")
	require.Contains(t, out.String(), "credentials.yaml")
	require.Contains(t, out.String(), "templates")
}

func TestInitCmd_CreatesConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "tdx")
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init"})
	require.NoError(t, cmd.Execute())

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestShowCmd_ReportsEmptyWhenNoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "no profiles configured")
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/cli/config/...`
Expected: compile error, `NewCmd` undefined.

- [ ] **Step 3: Implement the `config` command tree**

Create `internal/cli/config/config.go`:

```go
package config

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/spf13/cobra"
)

// NewCmd returns the `tdx config` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and initialise tdx configuration",
	}
	cmd.AddCommand(newPathCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newInitCmd())
	return cmd
}

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print resolved tdx config paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "root:         %s\n", p.Root)
			fmt.Fprintf(w, "config:       %s\n", p.ConfigFile)
			fmt.Fprintf(w, "credentials:  %s\n", p.CredentialsFile)
			fmt.Fprintf(w, "templates:    %s\n", p.TemplatesDir)
			return nil
		},
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the tdx config directory if it does not exist",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			if err := p.EnsureRoot(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "initialised %s\n", p.Root)
			return nil
		},
	}
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current profile configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			store := config.NewProfileStore(p)
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if len(cfg.Profiles) == 0 {
				fmt.Fprintln(w, "no profiles configured")
				fmt.Fprintln(w, "run 'tdx auth login' to create one")
				return nil
			}
			fmt.Fprintf(w, "default profile: %s\n", cfg.DefaultProfile)
			fmt.Fprintln(w, "profiles:")
			for _, prof := range cfg.Profiles {
				fmt.Fprintf(w, "  - %s  %s\n", prof.Name, prof.TenantBaseURL)
			}
			return nil
		},
	}
}
```

- [ ] **Step 4: Wire into root**

Modify `internal/cli/root.go`:

```go
package cli

import (
	"github.com/iainmoffat/tdx/internal/cli/config"
	"github.com/spf13/cobra"
)

// NewRootCmd returns the top-level tdx command.
// version is injected at build time by cmd/tdx/main.go.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "tdx",
		Short:         "Manage TeamDynamix time entries from the terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd(version))
	root.AddCommand(config.NewCmd())
	return root
}
```

- [ ] **Step 5: Run tests and build**

Run:
```bash
go test ./...
go build ./cmd/tdx
./tdx config path
./tdx config init
./tdx config show
```

Expected: tests pass; the binary runs each command and prints meaningful output.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/config/ internal/cli/root.go
git commit -m "feat(cli): add tdx config path/show/init"
```

---

## Task 11: `tdx auth profile` commands (list/add/remove/use)

**Files:**
- Create: `internal/cli/auth/auth.go`
- Create: `internal/cli/auth/profile.go`
- Modify: `internal/cli/root.go` — wire in the `auth` subcommand
- Test: `internal/cli/auth/profile_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cli/auth/profile_test.go`:

```go
package auth

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfileAdd_AddsAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"profile", "add", "default", "--url", "https://ufl.teamdynamix.com/"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "added profile \"default\"")
}

func TestProfileList_ShowsAddedProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", "https://ufl.teamdynamix.com/"})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "list"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "default")
	require.Contains(t, out.String(), "ufl.teamdynamix.com")
	require.Contains(t, out.String(), "*") // default marker
}

func TestProfileRemove_RemovesNamedProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	for _, name := range []string{"a", "b"} {
		cmd := NewCmd()
		cmd.SetArgs([]string{"profile", "add", name, "--url", "https://" + name + ".teamdynamix.com/"})
		require.NoError(t, cmd.Execute())
	}

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "remove", "a"})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "list"})
	require.NoError(t, cmd.Execute())
	require.NotContains(t, out.String(), "a.teamdynamix.com")
	require.Contains(t, out.String(), "b.teamdynamix.com")
}

func TestProfileUse_SwitchesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	for _, name := range []string{"first", "second"} {
		cmd := NewCmd()
		cmd.SetArgs([]string{"profile", "add", name, "--url", "https://" + name + ".teamdynamix.com/"})
		require.NoError(t, cmd.Execute())
	}

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "use", "second"})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "list"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "* second")
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/cli/auth/...`
Expected: compile error, `NewCmd` undefined.

- [ ] **Step 3: Implement the `auth` parent command**

Create `internal/cli/auth/auth.go`:

```go
package auth

import "github.com/spf13/cobra"

// NewCmd returns the `tdx auth` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage TeamDynamix authentication",
	}
	cmd.AddCommand(newProfileCmd())
	// login, logout, status are added in later tasks.
	return cmd
}
```

- [ ] **Step 4: Implement the profile commands**

Create `internal/cli/auth/profile.go`:

```go
package auth

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage tdx auth profiles",
	}
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileAddCmd())
	cmd.AddCommand(newProfileRemoveCmd())
	cmd.AddCommand(newProfileUseCmd())
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if len(cfg.Profiles) == 0 {
				fmt.Fprintln(w, "no profiles configured")
				return nil
			}
			for _, p := range cfg.Profiles {
				marker := "  "
				if p.Name == cfg.DefaultProfile {
					marker = "* "
				}
				fmt.Fprintf(w, "%s%s  %s\n", marker, p.Name, p.TenantBaseURL)
			}
			return nil
		},
	}
}

func newProfileAddCmd() *cobra.Command {
	var url string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			p := domain.Profile{Name: args[0], TenantBaseURL: url}
			if err := store.AddProfile(p); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added profile %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "tenant base URL (e.g. https://ufl.teamdynamix.com/)")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func newProfileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			if err := store.RemoveProfile(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed profile %q\n", args[0])
			return nil
		},
	}
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			if err := store.SetDefault(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "default profile set to %q\n", args[0])
			return nil
		},
	}
}

// newProfileStore resolves config paths fresh each call so tests can flip
// TDX_CONFIG_HOME between invocations.
func newProfileStore() (*config.ProfileStore, error) {
	p, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}
	return config.NewProfileStore(p), nil
}
```

- [ ] **Step 5: Wire into root**

Modify `internal/cli/root.go`:

```go
package cli

import (
	"github.com/iainmoffat/tdx/internal/cli/auth"
	"github.com/iainmoffat/tdx/internal/cli/config"
	"github.com/spf13/cobra"
)

// NewRootCmd returns the top-level tdx command.
// version is injected at build time by cmd/tdx/main.go.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "tdx",
		Short:         "Manage TeamDynamix time entries from the terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd(version))
	root.AddCommand(config.NewCmd())
	root.AddCommand(auth.NewCmd())
	return root
}
```

- [ ] **Step 6: Run tests and build**

Run:
```bash
go test ./...
go build ./cmd/tdx
./tdx auth profile list
./tdx auth profile add default --url https://ufl.teamdynamix.com/
./tdx auth profile list
```

Expected: tests pass; commands work end-to-end.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/auth/ internal/cli/root.go
git commit -m "feat(cli): add tdx auth profile list/add/remove/use"
```

---

## Task 12: `tdx auth status`

**Files:**
- Create: `internal/cli/auth/status.go`
- Modify: `internal/cli/auth/auth.go` — add status to the tree
- Test: `internal/cli/auth/status_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cli/auth/status_test.go`:

```go
package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatus_NoProfileConfigured(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, out.String()+err.Error(), "no profile")
}

func TestStatus_ProfileWithoutToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "not authenticated")
}

func TestStatus_ProfileWithValidToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	// Seed profile and credentials.
	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	setTokenForTest(t, "default", "good-token")

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "authenticated")
	require.Contains(t, out.String(), "token: valid")
}

func TestStatus_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	setTokenForTest(t, "default", "good-token")

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--json"})
	require.NoError(t, cmd.Execute())

	s := out.String()
	require.Contains(t, s, `"profile": "default"`)
	require.Contains(t, s, `"authenticated": true`)
	require.Contains(t, s, `"tokenValid": true`)
}
```

Create `internal/cli/auth/helpers_test.go`:

```go
package auth

import (
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/stretchr/testify/require"
)

// setTokenForTest writes a token directly to the credentials store.
func setTokenForTest(t *testing.T, profile, token string) {
	t.Helper()
	p, err := config.ResolvePaths()
	require.NoError(t, err)
	store := config.NewCredentialsStore(p)
	require.NoError(t, store.SetToken(profile, token))
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/cli/auth/...`
Expected: compile error, `status` command not registered.

- [ ] **Step 3: Implement status command**

Create `internal/cli/auth/status.go`:

```go
package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/spf13/cobra"
)

// statusJSON is the stable JSON shape emitted by `tdx auth status --json`.
// Part of the tdx.v1 schema per spec §9.
type statusJSON struct {
	Profile       string `json:"profile"`
	Tenant        string `json:"tenant"`
	Authenticated bool   `json:"authenticated"`
	TokenValid    bool   `json:"tokenValid"`
	Error         string `json:"error,omitempty"`
}

func newStatusCmd() *cobra.Command {
	var profileFlag string
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current auth state",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			svc := authsvc.New(paths)

			profileName, err := svc.ResolveProfile(profileFlag)
			if err != nil {
				return fmt.Errorf("no profile configured — run 'tdx auth login' or 'tdx auth profile add'")
			}

			status, err := svc.Status(context.Background(), profileName)
			if err != nil {
				return err
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag}, os.Stdout)
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), statusJSON{
					Profile:       status.Profile.Name,
					Tenant:        status.Profile.TenantBaseURL,
					Authenticated: status.Authenticated,
					TokenValid:    status.TokenValid,
					Error:         status.ValidationErr,
				})
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "profile:  %s\n", status.Profile.Name)
			fmt.Fprintf(w, "tenant:   %s\n", status.Profile.TenantBaseURL)
			if !status.Authenticated {
				fmt.Fprintln(w, "state:    not authenticated")
				fmt.Fprintln(w, "          run 'tdx auth login' to sign in")
				return nil
			}
			fmt.Fprintln(w, "state:    authenticated")
			if status.TokenValid {
				fmt.Fprintln(w, "token:    valid")
			} else {
				fmt.Fprintf(w, "token:    invalid (%s)\n", status.ValidationErr)
				fmt.Fprintln(w, "          run 'tdx auth login' to refresh")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to the configured default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit status as JSON")
	return cmd
}
```

- [ ] **Step 4: Register status command**

Modify `internal/cli/auth/auth.go`:

```go
package auth

import "github.com/spf13/cobra"

// NewCmd returns the `tdx auth` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage TeamDynamix authentication",
	}
	cmd.AddCommand(newProfileCmd())
	cmd.AddCommand(newStatusCmd())
	// login and logout are added in later tasks.
	return cmd
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/cli/auth/...`
Expected: all three status tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/auth/status.go internal/cli/auth/auth.go internal/cli/auth/status_test.go internal/cli/auth/helpers_test.go
git commit -m "feat(cli): add tdx auth status"
```

---

## Task 13: `tdx auth logout`

**Files:**
- Create: `internal/cli/auth/logout.go`
- Modify: `internal/cli/auth/auth.go` — add logout to tree
- Test: `internal/cli/auth/logout_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cli/auth/logout_test.go`:

```go
package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestLogout_ClearsTokenButKeepsProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	setTokenForTest(t, "default", "abc")

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"logout"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "logged out")

	p, err := config.ResolvePaths()
	require.NoError(t, err)
	creds := config.NewCredentialsStore(p)
	_, err = creds.GetToken("default")
	require.ErrorIs(t, err, domain.ErrNoCredentials)

	store := config.NewProfileStore(p)
	_, err = store.GetProfile("default")
	require.NoError(t, err, "profile should remain")
}

func TestLogout_NoCredentialsIsStillSuccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	cmd = NewCmd()
	cmd.SetArgs([]string{"logout"})
	require.NoError(t, cmd.Execute())
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/cli/auth/...`
Expected: compile error, `logout` not registered.

- [ ] **Step 3: Implement logout command**

Create `internal/cli/auth/logout.go`:

```go
package auth

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/spf13/cobra"
)

func newLogoutCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear the stored token for a profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			svc := authsvc.New(paths)

			profileName, err := svc.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			if err := svc.Logout(profileName); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged out of profile %q\n", profileName)
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to the configured default)")
	return cmd
}
```

- [ ] **Step 4: Register logout command**

Modify `internal/cli/auth/auth.go`:

```go
package auth

import "github.com/spf13/cobra"

// NewCmd returns the `tdx auth` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage TeamDynamix authentication",
	}
	cmd.AddCommand(newProfileCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newLogoutCmd())
	// login is added in the next task.
	return cmd
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/cli/auth/...`
Expected: both logout tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/auth/logout.go internal/cli/auth/auth.go internal/cli/auth/logout_test.go
git commit -m "feat(cli): add tdx auth logout"
```

---

## Task 14: `tdx auth login` (paste-token flow) + manual integration doc

**Files:**
- Create: `internal/cli/auth/login.go`
- Modify: `internal/cli/auth/auth.go` — add login to tree
- Test: `internal/cli/auth/login_test.go`
- Create: `docs/manual-tests/phase-1-auth-walkthrough.md`

- [ ] **Step 0: Add the golang.org/x/term dependency**

This is the first task that imports `golang.org/x/term`. Add it now so `go.sum` has the entry before the first build.

Run:
```bash
cd /Users/ipm/code/tdx
go get golang.org/x/term@latest
```

- [ ] **Step 1: Write failing tests**

Create `internal/cli/auth/login_test.go`:

```go
package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/stretchr/testify/require"
)

// loginRunner lets tests inject a fake token reader instead of prompting on tty.
type loginRunner struct {
	input string
}

func (l loginRunner) ReadToken(prompt string) (string, error) {
	return l.input, nil
}

func TestLogin_PasteTokenSuccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer good", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	cmd := NewCmdWithTokenReader(loginRunner{input: "good"})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"login", "--profile", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "signed in as profile \"default\"")

	p, err := config.ResolvePaths()
	require.NoError(t, err)
	creds := config.NewCredentialsStore(p)
	token, err := creds.GetToken("default")
	require.NoError(t, err)
	require.Equal(t, "good", token)
}

func TestLogin_EmptyTokenRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmdWithTokenReader(loginRunner{input: "   "})
	cmd.SetArgs([]string{"login", "--profile", "default", "--url", "https://ufl.teamdynamix.com/"})
	err := cmd.Execute()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "empty token") || strings.Contains(err.Error(), "invalid token"))
}

func TestLogin_ServerRejectsToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cmd := NewCmdWithTokenReader(loginRunner{input: "bad"})
	cmd.SetArgs([]string{"login", "--profile", "default", "--url", srv.URL})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token")

	p, err := config.ResolvePaths()
	require.NoError(t, err)
	creds := config.NewCredentialsStore(p)
	_, err = creds.GetToken("default")
	require.Error(t, err, "no token should be written on failure")
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/cli/auth/...`
Expected: compile errors, `NewCmdWithTokenReader` undefined.

- [ ] **Step 3: Implement login command**

Create `internal/cli/auth/login.go`:

```go
package auth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// TokenReader abstracts how the CLI collects a token from the user.
// Production reads from a TTY without echo; tests inject a fake.
type TokenReader interface {
	ReadToken(prompt string) (string, error)
}

type ttyReader struct{}

func (ttyReader) ReadToken(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func newLoginCmd(reader TokenReader) *cobra.Command {
	var profileFlag, urlFlag string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to TeamDynamix via paste-token flow",
		Long: `Paste-token login.

Log in to TeamDynamix in your browser, navigate to your user profile's
API token view, copy the token, then run this command and paste the
token when prompted. The token is validated against the tenant's
/api/time/types endpoint before being saved to ~/.config/tdx/credentials.yaml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			svc := authsvc.New(paths)

			// Resolve profile name. If none given and none configured, require --profile.
			profileName := profileFlag
			if profileName == "" {
				profileName, _ = svc.ResolveProfile("")
			}
			if profileName == "" {
				profileName = "default"
			}

			// Resolve tenant URL. Priority: --url > existing profile > prompt.
			tenantURL := urlFlag
			if tenantURL == "" {
				if existing, err := svc.Profiles().GetProfile(profileName); err == nil {
					tenantURL = existing.TenantBaseURL
				}
			}
			if tenantURL == "" {
				tenantURL = "https://ufl.teamdynamix.com/"
				fmt.Fprintf(cmd.ErrOrStderr(), "no --url given; defaulting to %s\n", tenantURL)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Signing in to %s as profile %q.\n", tenantURL, profileName)
			fmt.Fprintln(cmd.ErrOrStderr(), "Open TeamDynamix in your browser, copy your API token, and paste it here.")

			raw, err := reader.ReadToken("Token: ")
			if err != nil {
				return err
			}
			token := strings.TrimSpace(raw)

			sess, err := svc.Login(context.Background(), authsvc.LoginInput{
				ProfileName:   profileName,
				TenantBaseURL: tenantURL,
				Token:         token,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "signed in as profile %q (%s)\n", sess.Profile.Name, sess.Profile.TenantBaseURL)
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name to sign in as (default: existing default or 'default')")
	cmd.Flags().StringVar(&urlFlag, "url", "", "tenant base URL (default: existing profile or https://ufl.teamdynamix.com/)")
	return cmd
}
```

- [ ] **Step 4: Add the test-injection constructor and wire login into the auth tree**

Modify `internal/cli/auth/auth.go`:

```go
package auth

import "github.com/spf13/cobra"

// NewCmd returns the `tdx auth` command tree with the production TTY token reader.
func NewCmd() *cobra.Command {
	return NewCmdWithTokenReader(ttyReader{})
}

// NewCmdWithTokenReader lets tests inject a fake token reader.
func NewCmdWithTokenReader(reader TokenReader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage TeamDynamix authentication",
	}
	cmd.AddCommand(newProfileCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newLoginCmd(reader))
	return cmd
}
```

- [ ] **Step 5: Run tests and build the binary**

Run:
```bash
go test ./...
go build ./cmd/tdx
```

Expected: all tests pass; binary builds.

- [ ] **Step 6: Commit the login implementation**

```bash
git add internal/cli/auth/login.go internal/cli/auth/auth.go internal/cli/auth/login_test.go
git commit -m "feat(cli): add tdx auth login with paste-token flow"
```

- [ ] **Step 7: Write the manual integration walkthrough**

Create `docs/manual-tests/phase-1-auth-walkthrough.md`:

```markdown
# Phase 1 — Manual Auth Walkthrough

This document exercises the Phase 1 auth flow against a real TeamDynamix tenant.

## Prerequisites

- A built `tdx` binary (`go build ./cmd/tdx`).
- Access to a TeamDynamix tenant (default: `https://ufl.teamdynamix.com/`).
- A valid API token for your TD user (obtain from the TD web UI — exact location
  depends on your tenant configuration, typically under Profile → API Tokens).

## Walkthrough

1. **Verify the binary runs.**
   ```
   ./tdx version
   ```
   Expected: `tdx 0.1.0-dev`.

2. **Check the config paths.**
   ```
   ./tdx config path
   ```
   Expected: four lines showing `root`, `config`, `credentials`, `templates` all
   under `~/.config/tdx/`.

3. **Initialise the config directory.**
   ```
   ./tdx config init
   ```
   Expected: `initialised /Users/you/.config/tdx`.

4. **Check that no profiles exist.**
   ```
   ./tdx config show
   ```
   Expected: `no profiles configured`.

5. **Sign in with a paste token.**
   ```
   ./tdx auth login --profile default --url https://ufl.teamdynamix.com/
   ```
   Expected: a prompt asking for the token. Paste the token (it will not
   echo) and press Enter. On success: `signed in as profile "default" (https://ufl.teamdynamix.com/)`.

6. **Confirm status reports valid.**
   ```
   ./tdx auth status
   ```
   Expected:
   ```
   profile:  default
   tenant:   https://ufl.teamdynamix.com/
   state:    authenticated
   token:    valid
   ```

7. **List profiles to confirm persistence.**
   ```
   ./tdx auth profile list
   ```
   Expected: `* default  https://ufl.teamdynamix.com/`.

8. **Verify credentials file permissions.**
   ```
   ls -l ~/.config/tdx/credentials.yaml
   ```
   Expected: `-rw-------` (mode 0600).

9. **Log out.**
   ```
   ./tdx auth logout
   ```
   Expected: `logged out of profile "default"`.

10. **Re-check status.**
    ```
    ./tdx auth status
    ```
    Expected:
    ```
    profile:  default
    tenant:   https://ufl.teamdynamix.com/
    state:    not authenticated
              run 'tdx auth login' to sign in
    ```

## Failure cases to try

- **Bad token.** Enter a random string at the login prompt. Expected: `invalid token: server rejected token`.
- **Wrong tenant URL.** Pass `--url https://wrong.teamdynamix.com/`. Expected: an HTTP error surfaced clearly.
- **Missing URL and no existing profile.** Run `./tdx auth login` on a fresh system. Expected: defaults to `https://ufl.teamdynamix.com/` with a `stderr` notice.

## Notes

- Phase 1 does not implement any ergonomic browser SSO flow. Phase 1B is the
  follow-up for that, once the UFL SSO callback mechanism is verified.
- Phase 1 does not fetch user identity. `tdx auth status` will gain a "signed
  in as …" line in Phase 2 once a whoami endpoint is confirmed.
```

- [ ] **Step 8: Commit the walkthrough**

```bash
git add docs/manual-tests/phase-1-auth-walkthrough.md
git commit -m "docs: add phase 1 manual auth walkthrough"
```

---

## Final verification

- [ ] **Run the full test suite.**

```bash
go test ./...
```
Expected: all tests pass across `internal/domain/`, `internal/config/`, `internal/tdx/`, `internal/svc/authsvc/`, `internal/render/`, `internal/cli/config/`, `internal/cli/auth/`, `internal/cli/`.

- [ ] **Build the binary.**

```bash
go build ./cmd/tdx
```

- [ ] **Execute the manual walkthrough** against a real UFL token (`docs/manual-tests/phase-1-auth-walkthrough.md`).

- [ ] **Confirm the phase-1 exit criteria (from spec §11 Phase 1):**
  - `tdx auth login` succeeds end-to-end on a UFL laptop. ✓ via paste-token.
  - `tdx auth status` prints identity + tenant + expiry. ✓ *partial* — prints profile/tenant/state; identity and expiry deferred to later phases per the plan.
  - A manual curl-equivalent works against TD using the stored token. ✓ verified transitively by the status check's `Ping`.

---

## Open items entering Phase 2

- **Identity display** — `tdx auth status` currently does not show the signed-in user's name/email. Phase 2 will pick a verified TD endpoint (likely `/api/people/...` or an auth whoami) and add it.
- **Token expiry display** — paste-token flow gives no TTL metadata. Phase 1B may add expiry tracking if the proper SSO flow provides it.
- **Loopback / device-code SSO** — Phase 1B, gated on a dedicated brainstorm verifying the UFL SSO wire protocol.
- **Global flags** — this plan only wires `--profile`. `--json`, `--no-color`, `--yes`, `--verbose`, `--quiet` will be added when their first consumer appears (likely in Phase 2).
