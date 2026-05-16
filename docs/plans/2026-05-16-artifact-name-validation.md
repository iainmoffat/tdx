# Artifact Name Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject path-traversal in user-controlled artifact names (templates, drafts, profiles) before any filesystem operation. Single shared `domain.ValidateArtifactName` helper applied at every name-entry boundary — both at storage stores (security guarantee) and at CLI/MCP input parsers (UX layer).

**Architecture:** New `domain.ValidateArtifactName(name string) error` with regex `^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}$` plus Windows-reserved-name rejection. Wrap a new `domain.ErrInvalidArtifactName` sentinel. Existing `Template.Validate`, `WeekDraft.Validate`, and `Profile.Validate` call the helper for the `Name` field. Every store method (`tmplsvc.Store`, `draftsvc.Store`, `draftsvc.Service.Rename`, `config.ProfileStore`) that takes a `name string` validates first. CLI parsers (`ParseDraftRef`, `authsvc.ResolveProfile`, each template command's `RunE`) validate before any I/O so CI tests without a config still hit the right error path (Phase 1 lesson applied).

**Tech Stack:** Go 1.26.2; cobra; testify/require. No new dependencies.

**Spec:** [`docs/specs/2026-05-16-artifact-name-validation.md`](../specs/2026-05-16-artifact-name-validation.md)

---

## File Structure

After this plan completes:

```
internal/
├── domain/
│   ├── artifact.go                          # CREATE: ValidateArtifactName helper
│   ├── artifact_test.go                     # CREATE: table-driven unit tests
│   ├── errors.go                            # MODIFY: ErrInvalidArtifactName sentinel
│   ├── profile.go                           # MODIFY: Profile.Validate calls ValidateArtifactName
│   ├── profile_test.go                      # MODIFY: existing slash test broaden, add `..` case
│   ├── template.go                          # MODIFY: Template.Validate calls ValidateArtifactName
│   ├── template_test.go                     # MODIFY: add `..` case to Template.Validate test
│   ├── draft.go                             # MODIFY: WeekDraft.Validate calls ValidateArtifactName
│   └── draft_test.go                        # MODIFY: add `..` case to WeekDraft.Validate test
├── svc/
│   ├── tmplsvc/
│   │   ├── store.go                         # MODIFY: Load/Delete/Exists validate; List skips invalid
│   │   └── store_test.go                    # MODIFY: 3 new reject tests + 1 enumeration test
│   └── draftsvc/
│       ├── store.go                         # MODIFY: 6 read/exists/snapshot methods validate; List skips
│       ├── store_test.go                    # MODIFY: ~4 new tests
│       ├── rename.go                        # MODIFY: validate both names
│       └── rename_test.go                   # MODIFY: 2 new reject tests
├── config/
│   ├── profiles.go                          # MODIFY: 4 lookup/mutate methods validate
│   └── profiles_test.go                     # MODIFY: 4 new reject tests
└── cli/
    ├── time/
    │   ├── week/
    │   │   └── draft.go                     # MODIFY: ParseDraftRef validates name
    │   └── template/
    │       ├── apply.go                     # MODIFY: validate args[0] / args.Name at top of RunE
    │       ├── clone.go                     # MODIFY: ditto
    │       ├── compare.go                   # MODIFY: ditto
    │       ├── delete.go                    # MODIFY: ditto
    │       ├── derive.go                    # MODIFY: ditto
    │       ├── edit.go                      # MODIFY: ditto
    │       ├── list.go                      # (no name input — no change)
    │       └── show.go                      # MODIFY: ditto
    └── auth/
        └── profile.go                       # MODIFY: validate name in `use`/`remove`/`add` commands

internal/mcp/
├── tools_drafts.go                          # MODIFY: parseDraftRefMCP validates name
├── tools_tmpl.go                            # MODIFY: 4 handlers validate args.Name
└── tools_apply.go                           # MODIFY: 3 handlers validate args.Name

internal/svc/authsvc/
└── service.go                               # MODIFY: ResolveProfile validates result

docs/
└── manual-tests/
    └── 2026-05-16-artifact-name-validation-walkthrough.md  # CREATE
```

## Branch + Versioning

- Branch: `artifact-name-validation` (Task 0)
- Version: **v0.21.0** — minor; pure validation addition; no breaking changes to existing well-behaved names (verified against on-disk artifacts on 2026-05-16).

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. (Main is at `0854e86` — the spec commit.)

- [ ] **Step 2: Create branch**

```bash
git checkout -b artifact-name-validation
```

Expected: `Switched to a new branch 'artifact-name-validation'`.

---

## Task 1: Domain sentinel + `ValidateArtifactName` helper

**Files:**
- Modify: `internal/domain/errors.go`
- Create: `internal/domain/artifact.go`
- Create: `internal/domain/artifact_test.go`

- [ ] **Step 1: Add sentinel error**

In `internal/domain/errors.go`, inside the existing `var (...)` block (just above the closing paren), add:

```go
	// ErrInvalidArtifactName indicates a template/draft/profile name failed
	// validation. Wrap with the specific reason for the user.
	ErrInvalidArtifactName = errors.New("invalid_artifact_name")
```

- [ ] **Step 2: Write failing unit tests**

Create `internal/domain/artifact_test.go`:

```go
package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateArtifactName_Accepts(t *testing.T) {
	cases := []string{
		"default",
		"my-week",
		"my-week2",
		"My_Week.draft",
		"a",
		"COM10",   // not COM1-9
		"LPT10",   // not LPT1-9
		"CONsole", // not exact CON
		strings.Repeat("a", 64),
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, ValidateArtifactName(name))
		})
	}
}

func TestValidateArtifactName_Rejects(t *testing.T) {
	cases := []struct {
		name   string
		reason string
	}{
		{"", "required"},
		{"..", "may not start with"},
		{".", "may not start with"},
		{".hidden", "may not start with"},
		{"-flag", "may not start with"},
		{"../../credentials", "invalid character"},
		{"/etc/passwd", "invalid character"},
		{"foo/bar", "invalid character"},
		{"foo\\bar", "invalid character"},
		{"foo bar", "invalid character"},
		{"foo\tbar", "invalid character"},
		{"naïve", "invalid character"},
		{"foo\x00bar", "invalid character"},
		{strings.Repeat("a", 65), "exceeds 64 characters"},
		{"CON", "reserved name"},
		{"con", "reserved name"},
		{"COM1", "reserved name"},
		{"LPT9", "reserved name"},
		{"NUL", "reserved name"},
		{"PRN", "reserved name"},
		{"AUX", "reserved name"},
		{"CON.txt", "reserved name"},
		{"nul.foo", "reserved name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateArtifactName(c.name)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInvalidArtifactName), "must wrap ErrInvalidArtifactName, got: %v", err)
			require.Contains(t, err.Error(), c.reason, "expected error to contain %q, got: %v", c.reason, err)
		})
	}
}
```

- [ ] **Step 3: Run tests — must FAIL**

```bash
go test ./internal/domain/... -run TestValidateArtifactName -v
```

Expected: FAIL — `ValidateArtifactName` undefined.

- [ ] **Step 4: Implement `artifact.go`**

Create `internal/domain/artifact.go`:

```go
package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// artifactNamePattern is the authoritative allowlist for template/draft/profile
// names. First character must be ASCII letter, digit, or underscore. Total
// length 1–64. Subsequent characters allow `.` and `-` additionally.
var artifactNamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}$`)

// windowsReservedNames lists case-insensitive base names that Windows refuses
// to open even with an extension (CON.yaml is treated as CON). Match is on the
// substring before the first `.` so CON.txt also rejects; COM10 doesn't.
var windowsReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {},
	"COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {},
	"LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

// ValidateArtifactName returns nil if name is a safe filesystem component for
// use as a template, draft, or profile name. See
// docs/specs/2026-05-16-artifact-name-validation.md for the rule and threat
// model. On reject, returns an error wrapping ErrInvalidArtifactName with a
// specific reason.
func ValidateArtifactName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidArtifactName)
	}
	if len(name) > 64 {
		return fmt.Errorf("%w: name exceeds 64 characters (got %d)", ErrInvalidArtifactName, len(name))
	}
	switch name[0] {
	case '.':
		return fmt.Errorf("%w: name may not start with %q", ErrInvalidArtifactName, ".")
	case '-':
		return fmt.Errorf("%w: name may not start with %q", ErrInvalidArtifactName, "-")
	}
	if !artifactNamePattern.MatchString(name) {
		for i, r := range name {
			if r > 127 || (r != '.' && r != '_' && r != '-' &&
				!(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')) {
				return fmt.Errorf("%w: name contains invalid character %q at position %d",
					ErrInvalidArtifactName, r, i)
			}
		}
		// Defensive: regex failed but no offending rune found (shouldn't
		// happen given the pattern; treat as generic reject).
		return fmt.Errorf("%w: name failed validation", ErrInvalidArtifactName)
	}
	// Reserved-name check: match the substring before the first `.`.
	head := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		head = name[:i]
	}
	if _, reserved := windowsReservedNames[strings.ToUpper(head)]; reserved {
		return fmt.Errorf("%w: %q is a reserved name", ErrInvalidArtifactName, name)
	}
	return nil
}
```

- [ ] **Step 5: Run tests — must PASS**

```bash
go test ./internal/domain/... -v
```

Expected: PASS for all `TestValidateArtifactName_*` plus all existing tests.

`go vet ./...` clean; `gofmt -l ./internal/...` empty.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/errors.go internal/domain/artifact.go internal/domain/artifact_test.go
git commit -m "feat(domain): add ValidateArtifactName helper and ErrInvalidArtifactName sentinel"
```

No `Co-Authored-By` trailer.

---

## Task 2: Wire `ValidateArtifactName` into the three domain `Validate` methods

**Files:**
- Modify: `internal/domain/profile.go`
- Modify: `internal/domain/template.go`
- Modify: `internal/domain/draft.go`
- Modify: `internal/domain/profile_test.go`
- Modify: `internal/domain/template_test.go`
- Modify: `internal/domain/draft_test.go`

- [ ] **Step 1: Write failing tests for `Profile.Validate`**

In `internal/domain/profile_test.go`, append:

```go
func TestProfile_Validate_RejectsDotDot(t *testing.T) {
	p := Profile{Name: "..", TenantBaseURL: "https://example.com"}
	err := p.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArtifactName)
}

func TestProfile_Validate_RejectsDotPrefix(t *testing.T) {
	p := Profile{Name: ".hidden", TenantBaseURL: "https://example.com"}
	err := p.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArtifactName)
}
```

The existing `TestProfile_Validate_RejectsNameWithSlash` test currently asserts `ErrInvalidProfile`. The new validator returns `ErrInvalidArtifactName`. Update that test:

```go
func TestProfile_Validate_RejectsNameWithSlash(t *testing.T) {
	p := Profile{Name: "evil/name", TenantBaseURL: "https://example.com"}
	err := p.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArtifactName)
}
```

- [ ] **Step 2: Write failing tests for `Template.Validate`**

In `internal/domain/template_test.go`, append a subtest inside the existing `TestTemplate_Validate` or as a new function:

```go
func TestTemplate_Validate_RejectsInvalidName(t *testing.T) {
	tmpl := Template{
		Name: "../../foo",
		Rows: []TemplateRow{{ID: "r1"}},
	}
	err := tmpl.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArtifactName)
}
```

- [ ] **Step 3: Write failing tests for `WeekDraft.Validate`**

In `internal/domain/draft_test.go`, append:

```go
func TestWeekDraft_Validate_RejectsInvalidName(t *testing.T) {
	d := WeekDraft{
		Profile:   "default",
		Name:      "../../foo",
		WeekStart: time.Date(2026, 4, 12, 0, 0, 0, 0, EasternTZ),
	}
	err := d.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArtifactName)
}
```

If `"time"` isn't imported in `draft_test.go` yet, add it.

- [ ] **Step 4: Run tests — must FAIL**

```bash
go test ./internal/domain/... -v
```

Expected: the 4 new test functions fail; existing slash test fails because old error wasn't sentinel-wrapped.

- [ ] **Step 5: Update `Profile.Validate`**

In `internal/domain/profile.go`, replace the existing block:

```go
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidProfile)
	}
	if strings.ContainsAny(p.Name, "/\\ \t") {
		return fmt.Errorf("%w: name may not contain slashes or whitespace", ErrInvalidProfile)
	}
```

With:

```go
	if err := ValidateArtifactName(p.Name); err != nil {
		return err
	}
```

The `strings` import may now be unused in this file — remove it if so.

- [ ] **Step 6: Update `Template.Validate`**

In `internal/domain/template.go`, find the existing function (around line 120):

```go
func (t Template) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if len(t.Rows) == 0 {
		return fmt.Errorf("template %q must have at least one row", t.Name)
	}
	...
}
```

Replace the first guard with the helper call:

```go
func (t Template) Validate() error {
	if err := ValidateArtifactName(t.Name); err != nil {
		return fmt.Errorf("template: %w", err)
	}
	if len(t.Rows) == 0 {
		return fmt.Errorf("template %q must have at least one row", t.Name)
	}
	...
}
```

Note the `fmt.Errorf("template: %w", err)` wrap keeps the sentinel detectable via `errors.Is` and preserves the existing "template" context in the message.

- [ ] **Step 7: Update `WeekDraft.Validate`**

In `internal/domain/draft.go`, find the existing function (around line 94):

```go
func (d WeekDraft) Validate() error {
	if d.Profile == "" {
		return fmt.Errorf("draft profile is required")
	}
	if d.Name == "" {
		return fmt.Errorf("draft name is required")
	}
	...
}
```

Insert the name validation:

```go
func (d WeekDraft) Validate() error {
	if d.Profile == "" {
		return fmt.Errorf("draft profile is required")
	}
	if err := ValidateArtifactName(d.Name); err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	...
}
```

Note: this removes the old empty-name check because `ValidateArtifactName("")` already returns the "name is required" error.

- [ ] **Step 8: Run tests — must PASS**

```bash
go test ./internal/domain/... -v
```

Expected: all PASS including the 4 new tests and the updated slash test.

`go vet ./...` clean; `gofmt -l ./internal/...` empty.

- [ ] **Step 9: Commit**

```bash
git add internal/domain/profile.go internal/domain/template.go internal/domain/draft.go \
        internal/domain/profile_test.go internal/domain/template_test.go internal/domain/draft_test.go
git commit -m "feat(domain): Profile/Template/WeekDraft.Validate call ValidateArtifactName"
```

---

## Task 3: Store-level guards in `tmplsvc.Store`

**Files:**
- Modify: `internal/svc/tmplsvc/store.go`
- Modify: `internal/svc/tmplsvc/store_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/svc/tmplsvc/store_test.go`:

```go
func TestTmplStore_Load_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	_, err := store.Load("default", "../../credentials")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestTmplStore_Delete_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	err := store.Delete("default", "../../credentials")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestTmplStore_Exists_FalseForInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	// Exists returns bool; invalid names must read as "not exists" rather
	// than panic or attempt the filesystem operation.
	require.False(t, store.Exists("default", "../../credentials"))
}
```

Verify which imports tmplsvc/store_test.go uses; add `"github.com/iainmoffat/tdx/internal/domain"` and `"github.com/iainmoffat/tdx/internal/config"` if not already present.

- [ ] **Step 2: Run tests — must FAIL**

```bash
go test ./internal/svc/tmplsvc/... -run TestTmplStore -v
```

- [ ] **Step 3: Add guards in `tmplsvc/store.go`**

In `internal/svc/tmplsvc/store.go`, add a validation call at the top of each method that takes `name`.

`Load` (~line 57):

```go
func (s *Store) Load(profile, name string) (domain.Template, error) {
	if err := domain.ValidateArtifactName(name); err != nil {
		return domain.Template{}, err
	}
	// ... rest unchanged
}
```

`Exists` (~line 82):

```go
func (s *Store) Exists(profile, name string) bool {
	if err := domain.ValidateArtifactName(name); err != nil {
		return false
	}
	// ... rest unchanged
}
```

`Delete` (~line 96):

```go
func (s *Store) Delete(profile, name string) error {
	if err := domain.ValidateArtifactName(name); err != nil {
		return err
	}
	// ... rest unchanged
}
```

`Save` is already covered by `Template.Validate()` (Task 2) — leave it.

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/svc/tmplsvc/... -v
```

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/svc/tmplsvc/store.go internal/svc/tmplsvc/store_test.go
git commit -m "feat(tmplsvc): validate template name in Load/Delete/Exists"
```

---

## Task 4: Store-level guards in `draftsvc.Store` and `Service.Rename`

**Files:**
- Modify: `internal/svc/draftsvc/store.go`
- Modify: `internal/svc/draftsvc/rename.go`
- Modify: `internal/svc/draftsvc/store_test.go`
- Modify: `internal/svc/draftsvc/rename_test.go`

- [ ] **Step 1: Write failing tests for store**

Append to `internal/svc/draftsvc/store_test.go`:

```go
func TestDraftStore_Load_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	_, err := store.Load("default", weekStart, "../../credentials")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestDraftStore_Delete_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	err := store.Delete("default", weekStart, "../../credentials")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestDraftStore_Exists_FalseForInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	require.False(t, store.Exists("default", weekStart, "../../credentials"))
}

func TestDraftStore_SaveNew_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	d := domain.WeekDraft{
		Profile:   "default",
		Name:      "../../foo",
		WeekStart: time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ),
	}
	err := store.SaveNew(d)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
```

Verify imports include `"time"`, `"github.com/iainmoffat/tdx/internal/config"`, `"github.com/iainmoffat/tdx/internal/domain"` — add if missing.

- [ ] **Step 2: Write failing tests for rename**

Append to `internal/svc/draftsvc/rename_test.go`:

```go
func TestRename_RejectsInvalidOldName(t *testing.T) {
	dir := t.TempDir()
	tsvc := timesvc.New(config.Paths{Root: dir})
	svc := NewService(config.Paths{Root: dir}, tsvc)
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	err := svc.Rename("default", weekStart, "../../old", "newname")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestRename_RejectsInvalidNewName(t *testing.T) {
	dir := t.TempDir()
	tsvc := timesvc.New(config.Paths{Root: dir})
	svc := NewService(config.Paths{Root: dir}, tsvc)
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	err := svc.Rename("default", weekStart, "default", "../../new")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
```

Verify the file has needed imports (`time`, `config`, `domain`, `timesvc`); if `rename_test.go` doesn't exist yet, create it with package `draftsvc` and the imports above.

- [ ] **Step 3: Run tests — must FAIL**

```bash
go test ./internal/svc/draftsvc/... -run 'TestDraftStore|TestRename' -v
```

- [ ] **Step 4: Add guards to `draftsvc/store.go`**

In `internal/svc/draftsvc/store.go`, add validation calls at the top of each method that takes `name`.

`Load` (~line 51), `Exists` (~line 69), `Delete` (~line 75), `LoadPulledSnapshot` (~line 106):

```go
	if err := domain.ValidateArtifactName(name); err != nil {
		return domain.WeekDraft{}, err   // for Load/LoadPulledSnapshot
		// OR `return false` for Exists, `return err` for Delete
	}
```

For `Save`, `SaveNew`, and `SavePulledSnapshot`: the name comes from `d.Name`. `Save` already calls `d.Validate()` which (after Task 2) catches it. `SaveNew` calls `Save`. `SavePulledSnapshot` does NOT call `Validate` — add an explicit guard:

```go
func (s *Store) SavePulledSnapshot(d domain.WeekDraft) error {
	if err := domain.ValidateArtifactName(d.Name); err != nil {
		return err
	}
	// ... rest unchanged
}
```

- [ ] **Step 5: Add guards to `draftsvc/rename.go`**

In `internal/svc/draftsvc/rename.go`, at the top of `Rename`:

```go
func (s *Service) Rename(profile string, weekStart time.Time, oldName, newName string) error {
	if err := domain.ValidateArtifactName(oldName); err != nil {
		return fmt.Errorf("rename old: %w", err)
	}
	if err := domain.ValidateArtifactName(newName); err != nil {
		return fmt.Errorf("rename new: %w", err)
	}
	// ... rest unchanged
}
```

Ensure `domain` and `fmt` are imported.

- [ ] **Step 6: Run tests — must PASS**

```bash
go test ./internal/svc/draftsvc/... -v
```

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 7: Commit**

```bash
git add internal/svc/draftsvc/store.go internal/svc/draftsvc/rename.go \
        internal/svc/draftsvc/store_test.go internal/svc/draftsvc/rename_test.go
git commit -m "feat(draftsvc): validate draft name in store and rename methods"
```

---

## Task 5: Store-level guards in `ProfileStore`

**Files:**
- Modify: `internal/config/profiles.go`
- Modify: `internal/config/profiles_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/config/profiles_test.go`:

```go
func TestProfileStore_GetProfile_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewProfileStore(Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})
	_, err := store.GetProfile("..")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestProfileStore_RemoveProfile_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewProfileStore(Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})
	err := store.RemoveProfile("..")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestProfileStore_SetDefault_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewProfileStore(Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})
	err := store.SetDefault("..")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestProfileStore_UpdateProfile_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewProfileStore(Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})
	p := domain.Profile{Name: "..", TenantBaseURL: "https://example.com"}
	err := store.UpdateProfile(p)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
```

Verify imports include `"path/filepath"`, `"github.com/iainmoffat/tdx/internal/domain"`.

- [ ] **Step 2: Run tests — must FAIL**

```bash
go test ./internal/config/... -run TestProfileStore -v
```

- [ ] **Step 3: Add guards to `profiles.go`**

In `internal/config/profiles.go`, add validation at the top of each lookup/mutate method that takes a name string.

`GetProfile` (~line 133):

```go
func (s *ProfileStore) GetProfile(name string) (domain.Profile, error) {
	if err := domain.ValidateArtifactName(name); err != nil {
		return domain.Profile{}, err
	}
	// ... rest unchanged
}
```

`RemoveProfile` (~line 106):

```go
func (s *ProfileStore) RemoveProfile(name string) error {
	if err := domain.ValidateArtifactName(name); err != nil {
		return err
	}
	// ... rest unchanged
}
```

`SetDefault` (~line 147):

```go
func (s *ProfileStore) SetDefault(name string) error {
	if err := domain.ValidateArtifactName(name); err != nil {
		return err
	}
	// ... rest unchanged
}
```

`UpdateProfile` and `AddProfile` are already covered because they call `p.Validate()` (which now uses ValidateArtifactName after Task 2). Leave them.

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/config/... -v
```

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/config/profiles.go internal/config/profiles_test.go
git commit -m "feat(config): validate profile name in ProfileStore lookups and mutations"
```

---

## Task 6: List enumeration — graceful skip with stderr warning

**Files:**
- Modify: `internal/svc/tmplsvc/store.go` (`List` method)
- Modify: `internal/svc/draftsvc/store.go` (`List` method)
- Modify: `internal/svc/tmplsvc/store_test.go`
- Modify: `internal/svc/draftsvc/store_test.go`

This task makes the `List` methods robust against any legacy file on disk with a name that the new validator rejects. We log a warning to `os.Stderr` and skip the file rather than returning an error. (Not expected on this user's box, but defensive.)

- [ ] **Step 1: Write failing test for tmplsvc.List**

Append to `internal/svc/tmplsvc/store_test.go`:

```go
func TestTmplStore_List_SkipsInvalidNamesGracefully(t *testing.T) {
	dir := t.TempDir()
	paths := config.Paths{Root: dir}
	store := NewStore(paths)

	// Seed: one valid template via Save, then one invalid file written directly.
	require.NoError(t, store.Save("default", domain.Template{
		Name: "valid", Rows: []domain.TemplateRow{{ID: "r1"}},
	}))
	invalidDir := paths.ProfileTemplatesDir("default")
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "..invalid.yaml"), []byte("name: ..invalid\nrows: []\n"), 0o600))

	templates, err := store.List("default")
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, "valid", templates[0].Name)
}
```

Imports: add `"os"`, `"path/filepath"` if not already present.

- [ ] **Step 2: Write failing test for draftsvc.List**

Append to `internal/svc/draftsvc/store_test.go`:

```go
func TestDraftStore_List_SkipsInvalidNamesGracefully(t *testing.T) {
	dir := t.TempDir()
	paths := config.Paths{Root: dir}
	store := NewStore(paths)

	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	require.NoError(t, store.Save(domain.WeekDraft{
		Profile: "default", Name: "valid", WeekStart: weekStart,
	}))

	// Drop a stray file with an invalid name into the same week dir.
	dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
	bogus := filepath.Join(paths.ProfileWeeksDir("default"), dateDir, "..bogus.yaml")
	require.NoError(t, os.WriteFile(bogus, []byte("profile: default\nname: ..bogus\nweekStart: 2026-04-12T00:00:00-04:00\n"), 0o600))

	drafts, err := store.List("default")
	require.NoError(t, err)
	require.Len(t, drafts, 1)
	require.Equal(t, "valid", drafts[0].Name)
}
```

Imports: `"os"`, `"path/filepath"`, `"time"`.

- [ ] **Step 3: Run tests — must FAIL**

Current `List` errors out (or panics) on the invalid file because `Load` (now) rejects the name.

```bash
go test ./internal/svc/tmplsvc/... ./internal/svc/draftsvc/... -run 'TestTmplStore_List|TestDraftStore_List' -v
```

- [ ] **Step 4: Update `tmplsvc.List`**

In `internal/svc/tmplsvc/store.go`'s `List` (~line 115), inside both the per-profile and legacy loops, wrap the `Load` call so that an `ErrInvalidArtifactName` from a rejected on-disk name skips rather than errors:

Replace each occurrence of:

```go
			t, err := s.Load(profile, name)
			if err != nil {
				return nil, fmt.Errorf("load template %q: %w", name, err)
			}
```

With:

```go
			t, err := s.Load(profile, name)
			if err != nil {
				if errors.Is(err, domain.ErrInvalidArtifactName) {
					fmt.Fprintf(os.Stderr, "warning: skipping template with invalid name %q\n", name)
					continue
				}
				return nil, fmt.Errorf("load template %q: %w", name, err)
			}
```

Add `"errors"` and `"os"` to the imports.

- [ ] **Step 5: Update `draftsvc.List`**

In `internal/svc/draftsvc/store.go`'s `List` (~line 131), apply the same pattern:

```go
			d, err := s.Load(profile, weekStart, name)
			if err != nil {
				if errors.Is(err, domain.ErrInvalidArtifactName) {
					fmt.Fprintf(os.Stderr, "warning: skipping draft with invalid name %q\n", name)
					continue
				}
				return nil, err
			}
```

Add `"errors"` and `"os"` if not already imported.

- [ ] **Step 6: Run tests — must PASS**

```bash
go test ./internal/svc/tmplsvc/... ./internal/svc/draftsvc/... -v
```

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 7: Commit**

```bash
git add internal/svc/tmplsvc/store.go internal/svc/tmplsvc/store_test.go \
        internal/svc/draftsvc/store.go internal/svc/draftsvc/store_test.go
git commit -m "feat(svc): List skips files with invalid names and warns to stderr"
```

---

## Task 7: Boundary check — `ParseDraftRef` (CLI) and `parseDraftRefMCP`

**Files:**
- Modify: `internal/cli/time/week/draft.go`
- Modify: `internal/cli/time/week/draft_test.go`
- Modify: `internal/mcp/tools_drafts.go`

- [ ] **Step 1: Write failing test for ParseDraftRef**

Append to `internal/cli/time/week/draft_test.go`:

```go
func TestParseDraftRef_RejectsInvalidName(t *testing.T) {
	_, _, err := ParseDraftRef("2026-04-12/../../foo")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
```

Add `"github.com/iainmoffat/tdx/internal/domain"` to the imports if not present.

- [ ] **Step 2: Run test — must FAIL**

```bash
go test ./internal/cli/time/week/... -run TestParseDraftRef_RejectsInvalidName -v
```

- [ ] **Step 3: Add validation to ParseDraftRef**

In `internal/cli/time/week/draft.go`, modify `ParseDraftRef` to validate the name after the slash-split:

```go
func ParseDraftRef(s string) (time.Time, string, error) {
	if s == "" {
		return time.Time{}, "", fmt.Errorf("draft reference required")
	}
	var dateStr, name string
	if i := strings.IndexByte(s, '/'); i >= 0 {
		dateStr, name = s[:i], s[i+1:]
		if name == "" {
			return time.Time{}, "", fmt.Errorf("empty name after slash in %q", s)
		}
	} else {
		dateStr, name = s, "default"
	}
	if err := domain.ValidateArtifactName(name); err != nil {
		return time.Time{}, "", err
	}
	d, err := time.ParseInLocation("2006-01-02", dateStr, domain.EasternTZ)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid date %q: %w", dateStr, err)
	}
	return domain.WeekRefContaining(d).StartDate, name, nil
}
```

- [ ] **Step 4: Update `parseDraftRefMCP`**

In `internal/mcp/tools_drafts.go` find `parseDraftRefMCP` (around line 821 — the duplicate of `ParseDraftRef`). Add the same `domain.ValidateArtifactName` call after the slash-split and before date parsing. Mirror the CLI exactly:

```go
	if err := domain.ValidateArtifactName(name); err != nil {
		return time.Time{}, "", err
	}
```

Confirm `domain` is imported in this file (it likely already is).

- [ ] **Step 5: Run tests — must PASS**

```bash
go test ./internal/cli/time/week/... ./internal/mcp/... -v
```

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/time/week/draft.go internal/cli/time/week/draft_test.go \
        internal/mcp/tools_drafts.go
git commit -m "feat(cli/mcp): validate draft name in ParseDraftRef and parseDraftRefMCP"
```

---

## Task 8: Boundary check — `authsvc.ResolveProfile`

**Files:**
- Modify: `internal/svc/authsvc/service.go`
- Modify: `internal/svc/authsvc/service_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestResolveProfile_RejectsInvalidExplicitName(t *testing.T) {
	dir := t.TempDir()
	svc := New(config.Paths{Root: dir, ConfigFile: filepath.Join(dir, "config.yaml")})
	_, err := svc.ResolveProfile("..")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
```

Imports as needed.

- [ ] **Step 2: Run test — must FAIL**

```bash
go test ./internal/svc/authsvc/... -run TestResolveProfile_RejectsInvalidExplicitName -v
```

- [ ] **Step 3: Update `ResolveProfile`**

In `internal/svc/authsvc/service.go`, modify the existing function (around line 150):

```go
func (s *Service) ResolveProfile(explicit string) (string, error) {
	if explicit != "" {
		if err := domain.ValidateArtifactName(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}
	cfg, err := s.profiles.Load()
	if err != nil {
		return "", err
	}
	if cfg.DefaultProfile == "" {
		return "", fmt.Errorf("%w: no default profile configured", domain.ErrProfileNotFound)
	}
	if err := domain.ValidateArtifactName(cfg.DefaultProfile); err != nil {
		return "", fmt.Errorf("default profile invalid: %w", err)
	}
	return cfg.DefaultProfile, nil
}
```

Confirm `domain` is imported.

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/svc/authsvc/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/svc/authsvc/service.go internal/svc/authsvc/service_test.go
git commit -m "feat(authsvc): validate profile name from --profile flag and stored default"
```

---

## Task 9: Boundary check — CLI template commands

**Files:**
- Modify: `internal/cli/time/template/apply.go`
- Modify: `internal/cli/time/template/clone.go`
- Modify: `internal/cli/time/template/compare.go`
- Modify: `internal/cli/time/template/delete.go`
- Modify: `internal/cli/time/template/derive.go`
- Modify: `internal/cli/time/template/edit.go`
- Modify: `internal/cli/time/template/show.go`

Each command takes a template name as `args[0]` (or via `--name`). Add a validation call at the top of `RunE`, BEFORE `config.ResolvePaths`. This matches the Phase 1 pattern (flag validation before config).

- [ ] **Step 1: Read each file to confirm name source**

For each file in the list, identify whether the name comes from `args[0]` or a flag variable.

```bash
grep -nE "args\[0\]|var .*Name|StringVar.*name" /Users/ipm/code/tdx/internal/cli/time/template/{apply,clone,compare,delete,derive,edit,show}.go
```

- [ ] **Step 2: Write a failing integration test**

In `internal/cli/time/template/show_test.go`, append:

```go
func TestShow_RejectsInvalidName(t *testing.T) {
	cmd := newShowCmd()
	cmd.SetArgs([]string{"../../foo"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
```

Add imports as needed (`"io"`, `domain`, `require`).

This single test gives confidence the boundary fires before config.ResolvePaths.

- [ ] **Step 3: Run test — must FAIL**

```bash
go test ./internal/cli/time/template/... -run TestShow_RejectsInvalidName -v
```

The test fails today either with `profile not found` (config gotcha) or via the store-level guard from Task 3 — depending on environment.

- [ ] **Step 4: Add name validation at the top of each command's RunE**

For each of the 7 files listed above, at the very top of `RunE` (BEFORE `config.ResolvePaths`), insert a validation block. Example for `show.go`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	if err := domain.ValidateArtifactName(args[0]); err != nil {
		return err
	}
	// ... existing config.ResolvePaths and rest
}
```

Verified arg shapes (from `grep "Args: cobra.ExactArgs"`):
- `apply.go`, `compare.go`, `delete.go`, `derive.go`, `edit.go`, `show.go` — `ExactArgs(1)`, name is `args[0]`
- `clone.go` — `ExactArgs(2)`, validate BOTH `args[0]` (src) and `args[1]` (dst)

Confirm `"github.com/iainmoffat/tdx/internal/domain"` is imported in each file.

- [ ] **Step 5: Run tests — must PASS**

```bash
go test ./internal/cli/time/template/... -v
```

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/time/template/apply.go internal/cli/time/template/clone.go \
        internal/cli/time/template/compare.go internal/cli/time/template/delete.go \
        internal/cli/time/template/derive.go internal/cli/time/template/edit.go \
        internal/cli/time/template/show.go internal/cli/time/template/show_test.go
git commit -m "feat(cli): validate template name at top of every template command"
```

---

## Task 10: Boundary check — CLI auth/profile commands

**Files:**
- Modify: `internal/cli/auth/profile.go`
- Modify: `internal/cli/auth/profile_test.go`

The `tdx auth profile add/use/remove <name>` commands take a profile name. `use` and `remove` need explicit validation at RunE. `add` already goes through `Profile.Validate()` via `ProfileStore.AddProfile`, so it's already covered — but a boundary check before config.ResolvePaths is still useful for the CI-test-without-config pattern.

Subcommands present in `internal/cli/auth/profile.go`:
- `newProfileListCmd` — no name, skip
- `newProfileAddCmd` — `add <name>`, RunE around line 59
- `newProfileRemoveCmd` — `remove <name>`, RunE around line 82
- `newProfileUseCmd` — `use <name>`, RunE around line 98

- [ ] **Step 1: Write failing test**

In `internal/cli/auth/profile_test.go`, append:

```go
func TestProfileUse_RejectsInvalidName(t *testing.T) {
	cmd := newProfileCmd()
	cmd.SetArgs([]string{"use", ".."})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
```

Add imports: `"io"`, `"github.com/iainmoffat/tdx/internal/domain"`, `"github.com/stretchr/testify/require"`.

- [ ] **Step 2: Run test — must FAIL**

```bash
go test ./internal/cli/auth/... -run TestProfileUse_RejectsInvalidName -v
```

- [ ] **Step 3: Add validation to `use`, `remove`, `add` RunE**

For each of the three subcommands' `RunE`, insert at the very top (before `newProfileStore()` is called):

```go
		if err := domain.ValidateArtifactName(args[0]); err != nil {
			return err
		}
```

Confirm `"github.com/iainmoffat/tdx/internal/domain"` is imported.

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/cli/auth/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/cli/auth/profile.go internal/cli/auth/profile_test.go
git commit -m "feat(cli/auth): validate profile name in profile subcommands"
```

---

## Task 11: Boundary check — MCP template handlers

**Files:**
- Modify: `internal/mcp/tools_tmpl.go`
- Modify: `internal/mcp/tools_apply.go`

Each handler that takes `args.Name` validates at the top, before any service call. Match the existing `errorResult(string)` pattern used for MCP error responses.

- [ ] **Step 1: Identify the handlers**

```bash
grep -n "args.Name" /Users/ipm/code/tdx/internal/mcp/tools_tmpl.go /Users/ipm/code/tdx/internal/mcp/tools_apply.go
```

This lists every line that references `args.Name`. Each unique handler that takes the name needs the guard.

- [ ] **Step 2: Add validation block**

At the top of each handler that uses `args.Name` (immediately after the `args` struct is unmarshaled, before any service call), add:

```go
		if err := domain.ValidateArtifactName(args.Name); err != nil {
			return errorResult(err.Error()), nil, nil
		}
```

The string is `invalid_artifact_name: <specific reason>` — LLM-parseable via the prefix.

- [ ] **Step 3: Run MCP tests**

```bash
go test ./internal/mcp/... -v
```

All existing tests must continue to pass. No new tests required — the helper is already unit-tested in Task 1, and MCP handlers are difficult to drive directly (same caveat as Phase 1, Task 6).

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools_tmpl.go internal/mcp/tools_apply.go
git commit -m "feat(mcp): validate template name in tmpl and apply tool handlers"
```

---

## Task 12: Full test + lint sweep

**Files:** none

- [ ] **Step 1: Run the full suite**

```bash
go test ./... -race && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green. No gofmt output. No vet warnings. No lint warnings.

- [ ] **Step 2: If failures appear, fix in place and commit per-issue**

Common gotchas:
- A removed import (e.g., `strings` from profile.go after Task 2) — add or remove as the compiler dictates
- Tests in other packages that constructed profiles/templates/drafts with names like `.. or ..` or whitespace — update to use valid names
- Pre-existing tests that depended on the old `ErrInvalidProfile` slash error — update to assert `ErrInvalidArtifactName`

- [ ] **Step 3: Confirm green**

```bash
go test ./... -race
```

Expected: PASS.

---

## Task 13: Manual walkthrough doc

**Files:**
- Create: `docs/manual-tests/2026-05-16-artifact-name-validation-walkthrough.md`

- [ ] **Step 1: Write walkthrough**

```markdown
# Artifact Name Validation Walkthrough (v0.21.0)

Spec: [`docs/specs/2026-05-16-artifact-name-validation.md`](../specs/2026-05-16-artifact-name-validation.md)

## Step 1: CLI template — traversal in name

    tdx time template show ../../credentials

Expected: Exit 1; stderr `invalid_artifact_name: name contains invalid character '/' at position 2`.

## Step 2: CLI draft — traversal via ref

    tdx time week pull 2026-04-12/../../foo

Expected: Exit 1; stderr `invalid_artifact_name: name contains invalid character '/' at position 2`.

## Step 3: CLI auth — traversal in profile use

    tdx auth profile use ..

Expected: Exit 1; stderr `invalid_artifact_name: name may not start with "."`.

## Step 4: Profile add with reserved name

    tdx auth profile add CON --tenant https://example.com

Expected: Exit 1; stderr `invalid_artifact_name: "CON" is a reserved name`.

## Step 5: MCP — invalid template name

Via Claude or any MCP client, call `get_template` (or `apply_template`, `delete_template`) with `name: "../../credentials"`. The tool result should be an error containing `invalid_artifact_name: name contains invalid character '/' at position 2`. The LLM should be able to retry with a valid name.

## Step 6: MCP — invalid draft ref

Call `get_week_draft` with `ref: "2026-04-12/../../foo"`. Tool result error contains `invalid_artifact_name`.

## Step 7: Existing artifacts unaffected

    tdx time template show my-week
    tdx time week show 2026-04-12

Expected: Exit 0; normal output. No regression on existing well-behaved names.

## Step 8: Long name rejected

    tdx time template show $(head -c 100 /dev/urandom | base64 | tr -d '/+=' | head -c 100)

Expected: Exit 1; stderr `invalid_artifact_name: name exceeds 64 characters`.
```

- [ ] **Step 2: Commit**

```bash
git add docs/manual-tests/2026-05-16-artifact-name-validation-walkthrough.md
git commit -m "docs: walkthrough for artifact name validation (v0.21.0)"
```

---

## Task 14: PR

**Files:** none

- [ ] **Step 1: Push branch**

```bash
git push -u origin artifact-name-validation
```

- [ ] **Step 2: Create PR**

```bash
gh pr create --title "Artifact name validation (security hardening phase 2)" --body-file <(cat <<'EOF'
## Summary

Phase 2 of the security hardening rollup. Addresses audit finding #1 (High: path-traversal in template / draft / profile names).

- New `domain.ValidateArtifactName` helper enforcing `^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}$` plus Windows-reserved-name rejection.
- New `domain.ErrInvalidArtifactName` sentinel.
- Two-layer enforcement: store-level (security guarantee) and CLI/MCP boundary (UX + early refusal pre-config).
- Existing on-disk names verified compatible — no migration.

## Test plan

- [x] `go test ./... -race` green
- [x] `go vet ./...`, `gofmt -l .`, `golangci-lint run ./...` all clean
- [ ] Live manual walkthrough at `docs/manual-tests/2026-05-16-artifact-name-validation-walkthrough.md`

Closes: security audit finding #1.

Spec: docs/specs/2026-05-16-artifact-name-validation.md
EOF
)
```

If the `--body-file <(...)` form trips a HEREDOC quote issue (as it did during Phase 1), write the body to `/tmp/pr-body.md` first and use `--body-file /tmp/pr-body.md`.

---

## Self-Review Notes

- [ ] Spec coverage:
  - Helper + sentinel — Task 1.
  - Domain Validate updates (Profile/Template/WeekDraft) — Task 2.
  - Store-level guards: tmplsvc Task 3; draftsvc Task 4; ProfileStore Task 5.
  - List enumeration graceful skip — Task 6.
  - Boundary at ParseDraftRef + parseDraftRefMCP — Task 7.
  - Boundary at authsvc.ResolveProfile — Task 8.
  - Boundary at CLI template commands — Task 9.
  - Boundary at CLI auth/profile commands — Task 10.
  - Boundary at MCP template handlers — Task 11.
  - Tests (unit + store-integration + CLI-integration + walkthrough) — across Tasks 1, 3, 4, 5, 6, 7, 8, 9, 10, 13.
- [ ] No placeholders.
- [ ] Type consistency: `ValidateArtifactName`, `ErrInvalidArtifactName`, and the regex are referenced consistently across tasks.
- [ ] Each task is self-contained — tasks can be reviewed and committed independently.
