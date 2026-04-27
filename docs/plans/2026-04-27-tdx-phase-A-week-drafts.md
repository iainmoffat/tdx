# Phase A — Week Drafts MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> Each task follows strict TDD: failing test → verify failure → implement → verify pass → commit. Never amend commits — always create new ones. Branch: `phase-A-week-drafts` (off `main`).
>
> Do NOT run `go mod tidy` — Phase A adds zero new dependencies.
>
> No `Co-Authored-By` trailer on commit messages (causes unwanted GitHub contributor listing).

**Design spec:** `docs/specs/2026-04-27-tdx-week-drafts-design.md`

**Goal:** Ship the Week Drafts MVP — first-class, locally-stored, dated week documents that can be pulled from TeamDynamix, edited in the grid editor, validated, diffed, previewed, and pushed back, with safety guarantees built on the existing reconcile/apply engine extended with an `ActionDelete` kind. Includes a one-shot templates-per-profile storage migration.

**Architecture**

```
CLI layer (internal/cli/time/week/)
  |-- pull, list, show --draft, status, diff, preview, push, delete
  |-- set, note, history (SHOULD-tier)
  |-- shared draft helpers (internal/cli/time/week/draft.go)
  v
Service layer (internal/svc/draftsvc/)
  |-- Store (per-profile YAML + snapshots)
  |-- Service.Pull (WeekReport -> WeekDraft + watermark)
  |-- Reconcile (draft-aware, includes ActionDelete)
  |-- Apply (Create/Update/Delete with hash protection)
  |-- Snapshot helpers (auto-snapshot before destructive ops)
  |-- Migrate (templates-per-profile one-shot migration)
  v
Domain (internal/domain/)
  |-- WeekDraft, DraftRow, DraftCell, CellState
  |-- PullWatermark, DraftProvenance, DraftSyncState
  |-- ActionDelete added to ActionKind enum
```

Editor: extend the existing `internal/tui/editor` package with a draft model that reuses the grid renderer; add status bar, cell-state annotations, and a pre-save confirm dialog for cleared pulled cells.

MCP: 7 new tools in `internal/mcp/tools_drafts.go` mirroring the CLI MVP.

**Tech Stack:** Go 1.24, cobra, gopkg.in/yaml.v3, charmbracelet/bubbletea + lipgloss, modelcontextprotocol/go-sdk. No new deps.

---

## Task 1: Per-profile paths layout

Refactor `internal/config/paths.go` so the templates and weeks dirs live under `~/.config/tdx/profiles/<profile>/`. Keep the legacy `templates/` path readable until the migration runs (Task 2).

**Files:**
- Modify: `internal/config/paths.go`
- Modify: `internal/config/paths_test.go`

- [ ] **Step 1.1 — Write failing tests for the new path layout**

```go
// internal/config/paths_test.go (replacing or extending existing tests)
func TestProfilePaths(t *testing.T) {
    home := t.TempDir()
    t.Setenv("TDX_CONFIG_HOME", home)
    p := MustPaths()

    if got, want := p.ProfileTemplatesDir("work"), filepath.Join(home, "profiles", "work", "templates"); got != want {
        t.Errorf("ProfileTemplatesDir(work) = %q, want %q", got, want)
    }
    if got, want := p.ProfileWeeksDir("work"), filepath.Join(home, "profiles", "work", "weeks"); got != want {
        t.Errorf("ProfileWeeksDir(work) = %q, want %q", got, want)
    }
    if got, want := p.LegacyTemplatesDir, filepath.Join(home, "templates"); got != want {
        t.Errorf("LegacyTemplatesDir = %q, want %q", got, want)
    }
}
```

- [ ] **Step 1.2 — Run test, verify failure**

```bash
go test ./internal/config/ -run TestProfilePaths -v
```
Expected: FAIL — `ProfileTemplatesDir undefined` (or similar).

- [ ] **Step 1.3 — Implement the new methods**

Add to `internal/config/paths.go`:

```go
// ProfileTemplatesDir returns the per-profile templates directory.
func (p Paths) ProfileTemplatesDir(profile string) string {
    return filepath.Join(p.Root, "profiles", profile, "templates")
}

// ProfileWeeksDir returns the per-profile weeks (drafts) directory.
func (p Paths) ProfileWeeksDir(profile string) string {
    return filepath.Join(p.Root, "profiles", profile, "weeks")
}

// LegacyTemplatesDir is the pre-migration global templates directory.
// Read until the per-profile migration completes; thereafter only used
// to detect the .migrated marker.
var legacyTemplatesSubpath = "templates"
```

Add a `Root` field to `Paths` if not already present, and populate `LegacyTemplatesDir` in `MustPaths()`.

- [ ] **Step 1.4 — Run test, verify pass**

```bash
go test ./internal/config/ -v
```
Expected: PASS.

- [ ] **Step 1.5 — Commit**

```bash
git add internal/config/
git commit -m "feat(config): add per-profile paths for templates and weeks"
```

---

## Task 2: Templates-per-profile migration

One-shot migration on first run after upgrade. Detect legacy `~/.config/tdx/templates/`, prompt (auto-yes if only one profile exists), move files into the active profile's templates dir, leave `.migrated` marker.

**Files:**
- Create: `internal/svc/draftsvc/migrate.go`
- Create: `internal/svc/draftsvc/migrate_test.go`
- Modify: `internal/cli/root.go` (call migration at startup before resolving any service)

- [ ] **Step 2.1 — Write failing test (auto-yes single profile case)**

```go
// internal/svc/draftsvc/migrate_test.go
package draftsvc

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/iainmoffat/tdx/internal/config"
)

func TestMigrate_SingleProfile_AutoYes(t *testing.T) {
    home := t.TempDir()
    legacy := filepath.Join(home, "templates")
    if err := os.MkdirAll(legacy, 0o700); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(legacy, "canonical.yaml"), []byte("name: canonical\n"), 0o600); err != nil {
        t.Fatal(err)
    }

    paths := config.Paths{Root: home, LegacyTemplatesDir: legacy}
    profiles := []string{"work"}

    result, err := Migrate(paths, profiles, "work", &silentPrompter{})
    if err != nil {
        t.Fatalf("Migrate: %v", err)
    }
    if !result.Migrated {
        t.Errorf("Migrated = false, want true")
    }

    // File should now live under profiles/work/templates/
    target := paths.ProfileTemplatesDir("work")
    if _, err := os.Stat(filepath.Join(target, "canonical.yaml")); err != nil {
        t.Errorf("expected canonical.yaml in %s: %v", target, err)
    }

    // .migrated marker exists
    if _, err := os.Stat(filepath.Join(legacy, ".migrated")); err != nil {
        t.Errorf(".migrated marker missing: %v", err)
    }

    // Re-running is a no-op
    result2, err := Migrate(paths, profiles, "work", &silentPrompter{})
    if err != nil {
        t.Fatalf("re-Migrate: %v", err)
    }
    if result2.Migrated {
        t.Errorf("second Migrate Migrated = true, want false (already done)")
    }
}

type silentPrompter struct{}

func (silentPrompter) Confirm(question string) (bool, error) { return true, nil }
```

- [ ] **Step 2.2 — Run test, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run TestMigrate -v
```
Expected: FAIL — `Migrate undefined`.

- [ ] **Step 2.3 — Implement Migrate**

```go
// internal/svc/draftsvc/migrate.go
package draftsvc

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "github.com/iainmoffat/tdx/internal/config"
)

// Prompter abstracts user confirmation so tests can supply silent prompters.
type Prompter interface {
    Confirm(question string) (bool, error)
}

// MigrateResult reports what happened during the templates-per-profile migration.
type MigrateResult struct {
    Migrated      bool
    FilesMoved    int
    TargetProfile string
}

// Migrate moves templates from the legacy ~/.config/tdx/templates/ directory
// into the active profile's per-profile templates directory. It is a no-op if
// the legacy directory has a .migrated marker, or if it does not exist.
//
// When more than one profile is configured, the prompter is asked which profile
// should own the templates. With a single profile, migration runs automatically
// and silently.
func Migrate(paths config.Paths, profiles []string, activeProfile string, prompter Prompter) (MigrateResult, error) {
    legacy := paths.LegacyTemplatesDir
    if legacy == "" {
        return MigrateResult{}, nil
    }
    if _, err := os.Stat(filepath.Join(legacy, ".migrated")); err == nil {
        return MigrateResult{}, nil
    }
    if _, err := os.Stat(legacy); os.IsNotExist(err) {
        return MigrateResult{}, nil
    } else if err != nil {
        return MigrateResult{}, err
    }

    target := activeProfile
    if len(profiles) > 1 {
        ok, err := prompter.Confirm(fmt.Sprintf(
            "Move legacy templates into profile %q? (run for each profile manually if not.) [y/N]",
            activeProfile))
        if err != nil {
            return MigrateResult{}, err
        }
        if !ok {
            return MigrateResult{}, nil
        }
    }

    targetDir := paths.ProfileTemplatesDir(target)
    if err := os.MkdirAll(targetDir, 0o700); err != nil {
        return MigrateResult{}, err
    }

    entries, err := os.ReadDir(legacy)
    if err != nil {
        return MigrateResult{}, err
    }

    moved := 0
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
            continue
        }
        src := filepath.Join(legacy, e.Name())
        dst := filepath.Join(targetDir, e.Name())
        if err := moveFile(src, dst); err != nil {
            return MigrateResult{}, err
        }
        moved++
    }

    if err := os.WriteFile(filepath.Join(legacy, ".migrated"), []byte("ok\n"), 0o600); err != nil {
        return MigrateResult{}, err
    }
    return MigrateResult{Migrated: true, FilesMoved: moved, TargetProfile: target}, nil
}

func moveFile(src, dst string) error {
    if err := os.Rename(src, dst); err == nil {
        return nil
    }
    // Fallback for cross-device renames (rare in user home, but safe).
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()
    out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
    if err != nil {
        return err
    }
    if _, err := io.Copy(out, in); err != nil {
        out.Close()
        return err
    }
    if err := out.Close(); err != nil {
        return err
    }
    return os.Remove(src)
}
```

- [ ] **Step 2.4 — Run test, verify pass**

```bash
go test ./internal/svc/draftsvc/ -v
```
Expected: PASS.

- [ ] **Step 2.5 — Wire migration into CLI startup**

Modify `internal/cli/root.go` (or a `PersistentPreRunE` on the root cobra command) to call `draftsvc.Migrate` once on first invocation, using a CLI prompter that reads from stdin. Skip silently when stdin is not a TTY.

```go
// in internal/cli/root.go (or a new internal/cli/migrate.go)
func runMigration(paths config.Paths, profiles []string, active string) error {
    prompter := newCLIPrompter() // reads stdin; returns auto-no when not TTY
    _, err := draftsvc.Migrate(paths, profiles, active, prompter)
    return err
}
```

- [ ] **Step 2.6 — Add multi-profile prompt test**

```go
func TestMigrate_MultiProfile_PromptsAndRespects(t *testing.T) {
    home := t.TempDir()
    legacy := filepath.Join(home, "templates")
    if err := os.MkdirAll(legacy, 0o700); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(legacy, "x.yaml"), []byte("name: x\n"), 0o600); err != nil {
        t.Fatal(err)
    }

    paths := config.Paths{Root: home, LegacyTemplatesDir: legacy}
    profiles := []string{"work", "personal"}

    // User declines.
    decline := &fakePrompter{answer: false}
    result, err := Migrate(paths, profiles, "work", decline)
    if err != nil { t.Fatal(err) }
    if result.Migrated { t.Errorf("Migrated = true, want false (declined)") }

    // Marker NOT written; legacy file still present.
    if _, err := os.Stat(filepath.Join(legacy, ".migrated")); err == nil {
        t.Errorf(".migrated written despite decline")
    }
    if _, err := os.Stat(filepath.Join(legacy, "x.yaml")); err != nil {
        t.Errorf("legacy file removed despite decline: %v", err)
    }
}

type fakePrompter struct{ answer bool }
func (f fakePrompter) Confirm(string) (bool, error) { return f.answer, nil }
```

- [ ] **Step 2.7 — Run all tests, verify pass**

```bash
go test ./... -v
```
Expected: PASS.

- [ ] **Step 2.8 — Commit**

```bash
git add internal/svc/draftsvc/ internal/cli/
git commit -m "feat(draftsvc): templates-per-profile migration with one-shot prompt"
```

---

## Task 3: Domain — WeekDraft, DraftRow, DraftCell core types

Add the core types to `internal/domain/`. No state computation yet (Task 4). Marshalable to YAML with the same schema in §12.3 of the spec.

**Files:**
- Create: `internal/domain/draft.go`
- Create: `internal/domain/draft_test.go`

- [ ] **Step 3.1 — Write failing tests for WeekDraft/DraftRow/DraftCell shapes**

```go
// internal/domain/draft_test.go
package domain

import (
    "testing"
    "time"

    "gopkg.in/yaml.v3"
)

func TestWeekDraft_YAMLRoundTrip(t *testing.T) {
    in := WeekDraft{
        SchemaVersion: 1,
        Profile:       "work",
        WeekStart:     time.Date(2026, 5, 4, 0, 0, 0, 0, EasternTZ),
        Name:          "default",
        Notes:         "Friday short week.",
        Provenance: DraftProvenance{
            Kind:              ProvenancePulled,
            PulledAt:          time.Date(2026, 4, 27, 13, 12, 14, 0, time.UTC),
            RemoteFingerprint: "8a7fc2e1",
            RemoteStatus:      ReportOpen,
        },
        Rows: []DraftRow{{
            ID:       "row-01",
            Target:   Target{Kind: TargetTicket, AppID: 42, ItemID: 123, DisplayName: "Big Project"},
            TimeType: TimeType{ID: 7, Name: "Work"},
            Billable: true,
            Cells: []DraftCell{
                {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
                {Day: time.Tuesday, Hours: 8.0, SourceEntryID: 98732},
            },
        }},
    }

    data, err := yaml.Marshal(in)
    if err != nil { t.Fatalf("marshal: %v", err) }

    var out WeekDraft
    if err := yaml.Unmarshal(data, &out); err != nil { t.Fatalf("unmarshal: %v", err) }

    if out.Name != in.Name { t.Errorf("Name lost in round-trip") }
    if !out.WeekStart.Equal(in.WeekStart) { t.Errorf("WeekStart lost: %v vs %v", out.WeekStart, in.WeekStart) }
    if len(out.Rows) != 1 || len(out.Rows[0].Cells) != 2 {
        t.Errorf("Rows/Cells lost: %+v", out.Rows)
    }
    if out.Rows[0].Cells[0].SourceEntryID != 98731 {
        t.Errorf("SourceEntryID lost")
    }
}

func TestWeekDraft_Validate(t *testing.T) {
    valid := WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default",
        WeekStart: time.Date(2026, 5, 4, 0, 0, 0, 0, EasternTZ),
        Rows: []DraftRow{{ID: "row-01", Target: Target{Kind: TargetProject, ItemID: 1}}},
    }
    if err := valid.Validate(); err != nil { t.Errorf("valid draft errored: %v", err) }

    cases := []struct{ name string; d WeekDraft }{
        {"missing profile", WeekDraft{SchemaVersion: 1, Name: "x", WeekStart: valid.WeekStart}},
        {"missing name", WeekDraft{SchemaVersion: 1, Profile: "work", WeekStart: valid.WeekStart}},
        {"zero weekStart", WeekDraft{SchemaVersion: 1, Profile: "work", Name: "x"}},
        {"weekStart not Sunday", WeekDraft{SchemaVersion: 1, Profile: "work", Name: "x",
            WeekStart: time.Date(2026, 5, 5, 0, 0, 0, 0, EasternTZ)}},
        {"duplicate row IDs", WeekDraft{SchemaVersion: 1, Profile: "work", Name: "x",
            WeekStart: valid.WeekStart,
            Rows: []DraftRow{{ID: "row-01"}, {ID: "row-01"}}}},
    }
    for _, c := range cases {
        if err := c.d.Validate(); err == nil {
            t.Errorf("%s: expected error", c.name)
        }
    }
}
```

- [ ] **Step 3.2 — Run tests, verify failure**

```bash
go test ./internal/domain/ -run TestWeekDraft -v
```
Expected: FAIL — types undefined.

- [ ] **Step 3.3 — Implement the types**

```go
// internal/domain/draft.go
package domain

import (
    "fmt"
    "time"
)

// ProvenanceKind enumerates how a draft came into existence.
type ProvenanceKind string

const (
    ProvenanceBlank        ProvenanceKind = "blank"
    ProvenancePulled       ProvenanceKind = "pulled"
    ProvenanceFromTemplate ProvenanceKind = "from-template"
    ProvenanceFromDraft    ProvenanceKind = "from-draft"
)

// DraftProvenance records how a draft was created.
type DraftProvenance struct {
    Kind              ProvenanceKind `yaml:"kind" json:"kind"`
    PulledAt          time.Time      `yaml:"pulledAt,omitempty" json:"pulledAt,omitempty"`
    RemoteFingerprint string         `yaml:"remoteFingerprint,omitempty" json:"remoteFingerprint,omitempty"`
    RemoteStatus      ReportStatus   `yaml:"remoteStatus,omitempty" json:"remoteStatus,omitempty"`
    FromTemplate      string         `yaml:"fromTemplate,omitempty" json:"fromTemplate,omitempty"`
    FromDraft         string         `yaml:"fromDraft,omitempty" json:"fromDraft,omitempty"`
    ShiftedByDays     int            `yaml:"shiftedByDays,omitempty" json:"shiftedByDays,omitempty"`
}

// WeekDraft is the canonical local artifact for a single editable week.
type WeekDraft struct {
    SchemaVersion int             `yaml:"schemaVersion" json:"schemaVersion"`
    Profile       string          `yaml:"profile" json:"profile"`
    WeekStart     time.Time       `yaml:"weekStart" json:"weekStart"`
    Name          string          `yaml:"name" json:"name"`
    Notes         string          `yaml:"notes,omitempty" json:"notes,omitempty"`
    Tags          []string        `yaml:"tags,omitempty" json:"tags,omitempty"`
    Provenance    DraftProvenance `yaml:"provenance" json:"provenance"`
    CreatedAt     time.Time       `yaml:"createdAt" json:"createdAt"`
    ModifiedAt    time.Time       `yaml:"modifiedAt" json:"modifiedAt"`
    PushedAt      *time.Time      `yaml:"pushedAt,omitempty" json:"pushedAt,omitempty"`
    Rows          []DraftRow      `yaml:"rows" json:"rows"`
}

// DraftRow is one row of a draft (target + type + billable + cells).
type DraftRow struct {
    ID            string        `yaml:"id" json:"id"`
    Label         string        `yaml:"label,omitempty" json:"label,omitempty"`
    Target        Target        `yaml:"target" json:"target"`
    TimeType      TimeType      `yaml:"timeType" json:"timeType"`
    Description   string        `yaml:"description,omitempty" json:"description,omitempty"`
    Billable      bool          `yaml:"billable" json:"billable"`
    ResolverHints ResolverHints `yaml:"resolverHints,omitempty" json:"resolverHints,omitempty"`
    Cells         []DraftCell   `yaml:"cells" json:"cells"`
}

// DraftCell is the atomic unit of a draft: one row × one weekday.
type DraftCell struct {
    Day           time.Weekday `yaml:"day" json:"day"`
    Hours         float64      `yaml:"hours" json:"hours"`
    SourceEntryID int          `yaml:"sourceEntryID,omitempty" json:"sourceEntryID,omitempty"`
    PerCell       *PerCell     `yaml:"perCell,omitempty" json:"perCell,omitempty"`
}

// PerCell holds per-cell metadata overrides (Phase C escape hatch). Empty
// pointer fields mean "use the row default."
type PerCell struct {
    Description *string `yaml:"description,omitempty" json:"description,omitempty"`
    TimeTypeID  *int    `yaml:"timeTypeID,omitempty" json:"timeTypeID,omitempty"`
    Billable    *bool   `yaml:"billable,omitempty" json:"billable,omitempty"`
}

// Validate checks structural integrity of the draft.
func (d WeekDraft) Validate() error {
    if d.Profile == "" {
        return fmt.Errorf("draft profile is required")
    }
    if d.Name == "" {
        return fmt.Errorf("draft name is required")
    }
    if d.WeekStart.IsZero() {
        return fmt.Errorf("draft weekStart is required")
    }
    if d.WeekStart.In(EasternTZ).Weekday() != time.Sunday {
        return fmt.Errorf("draft weekStart must be a Sunday in EasternTZ (got %s)",
            d.WeekStart.Weekday())
    }
    seen := make(map[string]struct{}, len(d.Rows))
    for i, row := range d.Rows {
        if row.ID == "" {
            return fmt.Errorf("draft row[%d] has empty ID", i)
        }
        if _, dup := seen[row.ID]; dup {
            return fmt.Errorf("draft has duplicate row ID %q", row.ID)
        }
        seen[row.ID] = struct{}{}
    }
    return nil
}
```

- [ ] **Step 3.4 — Run tests, verify pass**

```bash
go test ./internal/domain/ -v
```
Expected: PASS.

- [ ] **Step 3.5 — Commit**

```bash
git add internal/domain/draft.go internal/domain/draft_test.go
git commit -m "feat(domain): add WeekDraft, DraftRow, DraftCell, DraftProvenance types"
```

---

## Task 4: CellState computation and DraftSyncState

Add `CellState` enum and a function that computes cell state from a draft cell's history (pulled vs current). Add `DraftSyncState` and a function to compute it from a draft + remote fingerprint.

**Files:**
- Modify: `internal/domain/draft.go`
- Modify: `internal/domain/draft_test.go`

- [ ] **Step 4.1 — Write failing tests for CellState/DraftSyncState**

```go
// Append to internal/domain/draft_test.go

func TestComputeCellState(t *testing.T) {
    pulled := DraftCell{Day: time.Monday, Hours: 8.0, SourceEntryID: 98731}

    cases := []struct {
        name    string
        pulled  DraftCell
        current DraftCell
        want    CellState
    }{
        {"untouched", pulled, pulled, CellUntouched},
        {"edited (hours)", pulled, DraftCell{Day: time.Monday, Hours: 6.0, SourceEntryID: 98731}, CellEdited},
        {"edited (cleared = delete-on-push)", pulled,
            DraftCell{Day: time.Monday, Hours: 0, SourceEntryID: 98731}, CellEdited},
        {"added (no source)", DraftCell{}, DraftCell{Day: time.Monday, Hours: 4.0}, CellAdded},
    }
    for _, c := range cases {
        got := ComputeCellState(c.pulled, c.current)
        if got != c.want {
            t.Errorf("%s: got %s, want %s", c.name, got, c.want)
        }
    }
}

func TestComputeSyncState(t *testing.T) {
    weekStart := time.Date(2026, 5, 4, 0, 0, 0, 0, EasternTZ)
    pulledFingerprint := "abc123"

    base := WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: weekStart,
        Provenance: DraftProvenance{Kind: ProvenancePulled, RemoteFingerprint: pulledFingerprint},
    }

    pulledCells := map[string]DraftCell{
        "row-01:monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
    }

    // Clean: cells match what was pulled, fingerprint matches.
    cleanDraft := base
    cleanDraft.Rows = []DraftRow{{ID: "row-01", Cells: []DraftCell{
        {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
    }}}

    if state := ComputeSyncState(cleanDraft, pulledCells, pulledFingerprint); state.Sync != SyncClean {
        t.Errorf("clean: got %s, want clean", state.Sync)
    }

    // Dirty: cell hours edited.
    dirty := cleanDraft
    dirty.Rows[0].Cells[0].Hours = 6.0
    if state := ComputeSyncState(dirty, pulledCells, pulledFingerprint); state.Sync != SyncDirty {
        t.Errorf("dirty: got %s, want dirty", state.Sync)
    }
    if state := ComputeSyncState(dirty, pulledCells, pulledFingerprint); state.Stale {
        t.Errorf("dirty: got Stale=true, want false (fingerprint matches)")
    }

    // Stale: fingerprint differs.
    if state := ComputeSyncState(cleanDraft, pulledCells, "different"); !state.Stale {
        t.Errorf("stale: got Stale=false, want true")
    }
}
```

- [ ] **Step 4.2 — Run tests, verify failure**

```bash
go test ./internal/domain/ -run "TestCompute" -v
```
Expected: FAIL — `ComputeCellState`, `ComputeSyncState`, `CellState`, `DraftSyncState` undefined.

- [ ] **Step 4.3 — Implement**

```go
// Append to internal/domain/draft.go

// CellState classifies a draft cell relative to its pulled origin.
type CellState string

const (
    CellUntouched CellState = "untouched"
    CellEdited    CellState = "edited"
    CellAdded     CellState = "added"
    CellConflict  CellState = "conflict"
    CellInvalid   CellState = "invalid"
)

// SyncState classifies a draft as a whole.
type SyncState string

const (
    SyncClean      SyncState = "clean"
    SyncDirty      SyncState = "dirty"
    SyncConflicted SyncState = "conflicted"
)

// DraftSyncState bundles the sync verdict + drift flag + cell counts.
type DraftSyncState struct {
    Sync       SyncState
    Stale      bool
    Untouched  int
    Edited     int
    Added      int
    Conflict   int
    TotalHours float64
}

// ComputeCellState classifies the current cell against the cell at pull time.
// A zero-valued pulledAtPullTime means "no entry was pulled here" (added cell).
func ComputeCellState(pulledAtPullTime, current DraftCell) CellState {
    if pulledAtPullTime.SourceEntryID == 0 && current.SourceEntryID == 0 {
        if current.Hours > 0 { return CellAdded }
        return CellUntouched
    }
    if pulledAtPullTime.Hours == current.Hours {
        return CellUntouched
    }
    return CellEdited
}

// ComputeSyncState walks the draft's cells, comparing each to its pulled
// counterpart (keyed by "rowID:weekday"), and returns the aggregate state.
//
// pulledFingerprint should be the remote fingerprint observed at the most
// recent successful pull or push; if it differs from the draft's stored
// remoteFingerprint, the Stale flag is set.
func ComputeSyncState(draft WeekDraft, pulledByKey map[string]DraftCell, currentRemoteFingerprint string) DraftSyncState {
    s := DraftSyncState{Sync: SyncClean}
    for _, row := range draft.Rows {
        for _, cell := range row.Cells {
            key := fmt.Sprintf("%s:%s", row.ID, cell.Day)
            pulled := pulledByKey[key]
            switch ComputeCellState(pulled, cell) {
            case CellUntouched: s.Untouched++
            case CellEdited:
                s.Edited++
                if s.Sync == SyncClean { s.Sync = SyncDirty }
            case CellAdded:
                s.Added++
                if s.Sync == SyncClean { s.Sync = SyncDirty }
            case CellConflict:
                s.Conflict++
                s.Sync = SyncConflicted
            }
            s.TotalHours += cell.Hours
        }
    }
    if currentRemoteFingerprint != "" && draft.Provenance.RemoteFingerprint != "" {
        s.Stale = currentRemoteFingerprint != draft.Provenance.RemoteFingerprint
    }
    return s
}
```

- [ ] **Step 4.4 — Run tests, verify pass**

```bash
go test ./internal/domain/ -v
```
Expected: PASS.

- [ ] **Step 4.5 — Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): add CellState and DraftSyncState computation"
```

---

## Task 5: ActionDelete + extended ReconcileDiff

Extend `internal/domain/reconcile.go` with `ActionDelete` and update related helpers (`String`, `CountByKind`).

**Files:**
- Modify: `internal/domain/reconcile.go`
- Modify: `internal/domain/reconcile_test.go` (or create if absent)

- [ ] **Step 5.1 — Write failing test**

```go
// internal/domain/reconcile_test.go (append)
func TestActionKind_StringIncludesDelete(t *testing.T) {
    if got, want := ActionDelete.String(), "delete"; got != want {
        t.Errorf("ActionDelete.String() = %q, want %q", got, want)
    }
}

func TestReconcileDiff_CountByKindIncludesDelete(t *testing.T) {
    diff := ReconcileDiff{Actions: []Action{
        {Kind: ActionCreate}, {Kind: ActionUpdate}, {Kind: ActionDelete},
        {Kind: ActionDelete}, {Kind: ActionSkip},
    }}
    creates, updates, deletes, skips := diff.CountByKindV2()
    if creates != 1 || updates != 1 || deletes != 2 || skips != 1 {
        t.Errorf("counts wrong: %d/%d/%d/%d", creates, updates, deletes, skips)
    }
}
```

- [ ] **Step 5.2 — Run test, verify failure**

```bash
go test ./internal/domain/ -run "TestActionKind\|TestReconcileDiff" -v
```
Expected: FAIL — `ActionDelete`/`CountByKindV2` undefined.

- [ ] **Step 5.3 — Implement**

```go
// Edit internal/domain/reconcile.go
const (
    ActionCreate ActionKind = 0
    ActionUpdate ActionKind = 1
    ActionSkip   ActionKind = 2
    ActionDelete ActionKind = 3 // NEW
)

// String returns the canonical string representation of the action kind.
func (k ActionKind) String() string {
    switch k {
    case ActionCreate: return "create"
    case ActionUpdate: return "update"
    case ActionSkip:   return "skip"
    case ActionDelete: return "delete"
    }
    return fmt.Sprintf("ActionKind(%d)", int(k))
}

// CountByKindV2 tallies actions including deletes. The original CountByKind
// is kept for backwards-compatible call sites in tmplsvc; new draft-aware
// code uses V2.
func (d ReconcileDiff) CountByKindV2() (creates, updates, deletes, skips int) {
    for _, a := range d.Actions {
        switch a.Kind {
        case ActionCreate: creates++
        case ActionUpdate: updates++
        case ActionDelete: deletes++
        case ActionSkip:   skips++
        }
    }
    return
}
```

Also add a `DeleteEntryID` field to the `Action` struct for delete actions (mirrors `ExistingID` for updates):

```go
type Action struct {
    Kind          ActionKind
    RowID         string
    Date          time.Time
    Entry         EntryInput  // for ActionCreate
    ExistingID    int         // for ActionUpdate
    Patch         EntryUpdate // for ActionUpdate
    DeleteEntryID int         // for ActionDelete  (NEW)
    SkipReason    string      // for ActionSkip
}
```

- [ ] **Step 5.4 — Run all existing tests, verify nothing regresses**

```bash
go test ./... -v
```
Expected: PASS. The original `CountByKind` still works for tmplsvc.

- [ ] **Step 5.5 — Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): add ActionDelete to ReconcileDiff"
```

---

## Task 6: draftsvc.Store — per-profile draft persistence

YAML store for drafts under `<profile>/weeks/<weekStart>/<name>.yaml`. Mirror `tmplsvc.Store`'s API surface.

**Files:**
- Create: `internal/svc/draftsvc/store.go`
- Create: `internal/svc/draftsvc/store_test.go`

- [ ] **Step 6.1 — Write failing tests**

```go
// internal/svc/draftsvc/store_test.go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestStore_SaveLoad(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := NewStore(paths)

    draft := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default",
        WeekStart: time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ), // Sunday
        Rows: []domain.DraftRow{{ID: "row-01"}},
    }
    if err := s.Save(draft); err != nil { t.Fatalf("Save: %v", err) }

    loaded, err := s.Load("work", draft.WeekStart, "default")
    if err != nil { t.Fatalf("Load: %v", err) }
    if loaded.Name != "default" { t.Errorf("name lost") }

    if !s.Exists("work", draft.WeekStart, "default") {
        t.Errorf("Exists = false after Save")
    }
}

func TestStore_List(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := NewStore(paths)

    week1 := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    week2 := time.Date(2026, 5, 10, 0, 0, 0, 0, domain.EasternTZ)
    for _, d := range []domain.WeekDraft{
        {SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week1},
        {SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week2},
    } {
        if err := s.Save(d); err != nil { t.Fatalf("Save: %v", err) }
    }

    drafts, err := s.List("work")
    if err != nil { t.Fatalf("List: %v", err) }
    if len(drafts) != 2 {
        t.Errorf("List returned %d drafts, want 2", len(drafts))
    }
}

func TestStore_Delete(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := NewStore(paths)
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
    if err := s.Save(d); err != nil { t.Fatal(err) }
    if err := s.Delete("work", week, "default"); err != nil { t.Fatalf("Delete: %v", err) }
    if s.Exists("work", week, "default") {
        t.Errorf("Exists = true after Delete")
    }
}

func TestStore_LoadMissing(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := NewStore(paths)
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    if _, err := s.Load("work", week, "default"); err == nil {
        t.Errorf("expected error loading non-existent draft")
    }
}
```

- [ ] **Step 6.2 — Run tests, verify failure**

```bash
go test ./internal/svc/draftsvc/ -v
```
Expected: FAIL — `Store undefined`.

- [ ] **Step 6.3 — Implement Store**

```go
// internal/svc/draftsvc/store.go
package draftsvc

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

    "gopkg.in/yaml.v3"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

// Store persists week drafts as YAML files under
// <root>/profiles/<profile>/weeks/<weekStart>/<name>.yaml.
type Store struct {
    paths config.Paths
}

func NewStore(paths config.Paths) *Store { return &Store{paths: paths} }

func (s *Store) draftPath(profile string, weekStart time.Time, name string) string {
    dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
    return filepath.Join(s.paths.ProfileWeeksDir(profile), dateDir, name+".yaml")
}

// Save writes the draft to disk. Creates parent directories as needed.
func (s *Store) Save(d domain.WeekDraft) error {
    if err := d.Validate(); err != nil {
        return fmt.Errorf("validate draft: %w", err)
    }
    p := s.draftPath(d.Profile, d.WeekStart, d.Name)
    if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
        return fmt.Errorf("create dir: %w", err)
    }
    data, err := yaml.Marshal(d)
    if err != nil { return fmt.Errorf("marshal: %w", err) }
    if err := os.WriteFile(p, data, 0o600); err != nil {
        return fmt.Errorf("write %s: %w", p, err)
    }
    return nil
}

// Load reads the draft. Returns a "not found" error if the file is absent.
func (s *Store) Load(profile string, weekStart time.Time, name string) (domain.WeekDraft, error) {
    p := s.draftPath(profile, weekStart, name)
    data, err := os.ReadFile(p)
    if err != nil {
        if os.IsNotExist(err) {
            return domain.WeekDraft{}, fmt.Errorf("draft not found: %s/%s/%s",
                profile, weekStart.In(domain.EasternTZ).Format("2006-01-02"), name)
        }
        return domain.WeekDraft{}, err
    }
    var d domain.WeekDraft
    if err := yaml.Unmarshal(data, &d); err != nil {
        return domain.WeekDraft{}, fmt.Errorf("unmarshal %s: %w", p, err)
    }
    return d, nil
}

// Exists reports whether a draft exists at (profile, weekStart, name).
func (s *Store) Exists(profile string, weekStart time.Time, name string) bool {
    _, err := os.Stat(s.draftPath(profile, weekStart, name))
    return err == nil
}

// Delete removes the draft file. Snapshots beside it are NOT removed by Save/Load/Delete;
// the snapshot store has its own lifecycle.
func (s *Store) Delete(profile string, weekStart time.Time, name string) error {
    p := s.draftPath(profile, weekStart, name)
    if err := os.Remove(p); err != nil {
        if os.IsNotExist(err) {
            return fmt.Errorf("draft not found: %s/%s/%s",
                profile, weekStart.In(domain.EasternTZ).Format("2006-01-02"), name)
        }
        return err
    }
    return nil
}

// List returns all drafts for the given profile, ordered by (weekStart desc, name asc).
func (s *Store) List(profile string) ([]domain.WeekDraft, error) {
    root := s.paths.ProfileWeeksDir(profile)
    entries, err := os.ReadDir(root)
    if err != nil {
        if os.IsNotExist(err) { return nil, nil }
        return nil, err
    }

    var drafts []domain.WeekDraft
    for _, dateEntry := range entries {
        if !dateEntry.IsDir() { continue }
        weekStart, err := time.ParseInLocation("2006-01-02", dateEntry.Name(), domain.EasternTZ)
        if err != nil { continue }
        files, err := os.ReadDir(filepath.Join(root, dateEntry.Name()))
        if err != nil { return nil, err }
        for _, f := range files {
            if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") { continue }
            name := strings.TrimSuffix(f.Name(), ".yaml")
            d, err := s.Load(profile, weekStart, name)
            if err != nil { return nil, err }
            drafts = append(drafts, d)
        }
    }
    sort.SliceStable(drafts, func(i, j int) bool {
        if !drafts[i].WeekStart.Equal(drafts[j].WeekStart) {
            return drafts[i].WeekStart.After(drafts[j].WeekStart)
        }
        return drafts[i].Name < drafts[j].Name
    })
    return drafts, nil
}
```

- [ ] **Step 6.4 — Run tests, verify pass**

```bash
go test ./internal/svc/draftsvc/ -v
```
Expected: PASS.

- [ ] **Step 6.5 — Commit**

```bash
git add internal/svc/draftsvc/store.go internal/svc/draftsvc/store_test.go
git commit -m "feat(draftsvc): per-profile draft store with YAML persistence"
```

---

## Task 7: Snapshot store with bounded retention

Snapshots live in `<weekDir>/<draftName>.snapshots/NNNN-<op>-<ts>.yaml`. Default retention: keep last 10 unpinned. Op tags: `pre-pull`, `pre-push`, `pre-refresh`, `pre-restore`, `pre-delete`, `manual`.

**Files:**
- Create: `internal/svc/draftsvc/snapshot.go`
- Create: `internal/svc/draftsvc/snapshot_test.go`

- [ ] **Step 7.1 — Write failing tests**

```go
// internal/svc/draftsvc/snapshot_test.go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestSnapshotStore_TakeAndList(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    ss := NewSnapshotStore(paths, 10)
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}

    s1, err := ss.Take(d, OpPrePull, "")
    if err != nil { t.Fatalf("Take: %v", err) }
    s2, err := ss.Take(d, OpPrePush, "")
    if err != nil { t.Fatalf("Take: %v", err) }
    if s1.Sequence == s2.Sequence {
        t.Errorf("sequences not incrementing")
    }
    list, err := ss.List("work", week, "default")
    if err != nil { t.Fatalf("List: %v", err) }
    if len(list) != 2 { t.Errorf("List returned %d, want 2", len(list)) }
}

func TestSnapshotStore_RetentionPrunes(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    ss := NewSnapshotStore(paths, 3) // small for test
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}

    for i := 0; i < 5; i++ {
        if _, err := ss.Take(d, OpManual, ""); err != nil { t.Fatal(err) }
    }
    list, err := ss.List("work", week, "default")
    if err != nil { t.Fatal(err) }
    if len(list) != 3 {
        t.Errorf("after 5 takes with retention=3 got %d", len(list))
    }
}

func TestSnapshotStore_PinnedSurvivesPrune(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    ss := NewSnapshotStore(paths, 2)
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}

    s1, _ := ss.Take(d, OpManual, "")
    if err := ss.Pin("work", week, "default", s1.Sequence, "important"); err != nil { t.Fatal(err) }
    for i := 0; i < 5; i++ { ss.Take(d, OpManual, "") }

    list, _ := ss.List("work", week, "default")
    found := false
    for _, s := range list { if s.Sequence == s1.Sequence && s.Pinned { found = true } }
    if !found { t.Errorf("pinned snapshot pruned") }
}
```

- [ ] **Step 7.2 — Run, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run TestSnapshot -v
```
Expected: FAIL — `SnapshotStore` undefined.

- [ ] **Step 7.3 — Implement**

```go
// internal/svc/draftsvc/snapshot.go
package draftsvc

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strconv"
    "strings"
    "time"

    "gopkg.in/yaml.v3"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

type OpTag string

const (
    OpPrePull    OpTag = "pre-pull"
    OpPrePush    OpTag = "pre-push"
    OpPreRefresh OpTag = "pre-refresh"
    OpPreRestore OpTag = "pre-restore"
    OpPreDelete  OpTag = "pre-delete"
    OpManual     OpTag = "manual"
)

type SnapshotInfo struct {
    Sequence int
    Op       OpTag
    Taken    time.Time
    Pinned   bool
    Note     string
    Path     string
}

// SnapshotStore manages per-draft snapshot files in <draft>.snapshots/.
type SnapshotStore struct {
    paths     config.Paths
    retention int
}

func NewSnapshotStore(paths config.Paths, retention int) *SnapshotStore {
    if retention <= 0 { retention = 10 }
    return &SnapshotStore{paths: paths, retention: retention}
}

func (ss *SnapshotStore) dir(profile string, weekStart time.Time, name string) string {
    dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
    return filepath.Join(ss.paths.ProfileWeeksDir(profile), dateDir, name+".snapshots")
}

var snapNameRE = regexp.MustCompile(`^(\d{4})-([\w-]+)-(\d{8}T\d{6}Z)(?:-([^.]+))?\.yaml$`)

// Take writes a snapshot of d, returns its info.
func (ss *SnapshotStore) Take(d domain.WeekDraft, op OpTag, note string) (SnapshotInfo, error) {
    dir := ss.dir(d.Profile, d.WeekStart, d.Name)
    if err := os.MkdirAll(dir, 0o700); err != nil { return SnapshotInfo{}, err }

    seq, err := ss.nextSequence(dir)
    if err != nil { return SnapshotInfo{}, err }

    ts := time.Now().UTC().Format("20060102T150405Z")
    suffix := ""
    if note != "" {
        // Sanitize note for filename usage.
        safe := strings.Map(func(r rune) rune {
            if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' { return r }
            if r >= 'A' && r <= 'Z' { return r + 32 }
            return '-'
        }, note)
        suffix = "-" + safe
    }
    filename := fmt.Sprintf("%04d-%s-%s%s.yaml", seq, op, ts, suffix)
    p := filepath.Join(dir, filename)

    data, err := yaml.Marshal(d)
    if err != nil { return SnapshotInfo{}, err }
    if err := os.WriteFile(p, data, 0o600); err != nil { return SnapshotInfo{}, err }

    if err := ss.prune(d.Profile, d.WeekStart, d.Name); err != nil {
        return SnapshotInfo{}, err
    }
    return SnapshotInfo{Sequence: seq, Op: op, Taken: time.Now().UTC(), Note: note, Path: p}, nil
}

// List returns snapshots ordered by sequence ascending.
func (ss *SnapshotStore) List(profile string, weekStart time.Time, name string) ([]SnapshotInfo, error) {
    dir := ss.dir(profile, weekStart, name)
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) { return nil, nil }
        return nil, err
    }
    var out []SnapshotInfo
    for _, e := range entries {
        if e.IsDir() { continue }
        if e.Name() == ".pinned" { continue }
        m := snapNameRE.FindStringSubmatch(e.Name())
        if m == nil { continue }
        seq, _ := strconv.Atoi(m[1])
        ts, _ := time.Parse("20060102T150405Z", m[3])
        info := SnapshotInfo{
            Sequence: seq, Op: OpTag(m[2]), Taken: ts,
            Note: m[4], Path: filepath.Join(dir, e.Name()),
        }
        out = append(out, info)
    }
    pinned, _ := ss.loadPinned(dir)
    for i := range out { if pinned[out[i].Sequence] { out[i].Pinned = true } }
    sort.SliceStable(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
    return out, nil
}

// Pin marks a snapshot as exempt from retention pruning.
func (ss *SnapshotStore) Pin(profile string, weekStart time.Time, name string, seq int, note string) error {
    dir := ss.dir(profile, weekStart, name)
    pinned, _ := ss.loadPinned(dir)
    pinned[seq] = true
    return ss.savePinned(dir, pinned)
}

func (ss *SnapshotStore) Load(profile string, weekStart time.Time, name string, seq int) (domain.WeekDraft, error) {
    list, err := ss.List(profile, weekStart, name)
    if err != nil { return domain.WeekDraft{}, err }
    for _, s := range list {
        if s.Sequence == seq {
            data, err := os.ReadFile(s.Path)
            if err != nil { return domain.WeekDraft{}, err }
            var d domain.WeekDraft
            if err := yaml.Unmarshal(data, &d); err != nil { return domain.WeekDraft{}, err }
            return d, nil
        }
    }
    return domain.WeekDraft{}, fmt.Errorf("snapshot %d not found", seq)
}

func (ss *SnapshotStore) nextSequence(dir string) (int, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) { return 1, nil }
        return 0, err
    }
    max := 0
    for _, e := range entries {
        m := snapNameRE.FindStringSubmatch(e.Name())
        if m == nil { continue }
        if seq, _ := strconv.Atoi(m[1]); seq > max { max = seq }
    }
    return max + 1, nil
}

func (ss *SnapshotStore) prune(profile string, weekStart time.Time, name string) error {
    list, err := ss.List(profile, weekStart, name)
    if err != nil { return err }
    var unpinned []SnapshotInfo
    for _, s := range list { if !s.Pinned { unpinned = append(unpinned, s) } }
    if len(unpinned) <= ss.retention { return nil }
    // Sort ascending so the oldest are pruned first.
    sort.SliceStable(unpinned, func(i, j int) bool { return unpinned[i].Sequence < unpinned[j].Sequence })
    excess := len(unpinned) - ss.retention
    for i := 0; i < excess; i++ {
        if err := os.Remove(unpinned[i].Path); err != nil { return err }
    }
    return nil
}

func (ss *SnapshotStore) loadPinned(dir string) (map[int]bool, error) {
    p := filepath.Join(dir, ".pinned")
    data, err := os.ReadFile(p)
    if err != nil {
        if os.IsNotExist(err) { return map[int]bool{}, nil }
        return nil, err
    }
    out := map[int]bool{}
    for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
        if line == "" { continue }
        if seq, err := strconv.Atoi(strings.TrimSpace(line)); err == nil { out[seq] = true }
    }
    return out, nil
}

func (ss *SnapshotStore) savePinned(dir string, pinned map[int]bool) error {
    if err := os.MkdirAll(dir, 0o700); err != nil { return err }
    var lines []string
    for seq := range pinned { lines = append(lines, strconv.Itoa(seq)) }
    sort.Strings(lines)
    return os.WriteFile(filepath.Join(dir, ".pinned"), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
```

- [ ] **Step 7.4 — Run, verify pass**

```bash
go test ./internal/svc/draftsvc/ -v
```
Expected: PASS.

- [ ] **Step 7.5 — Commit**

```bash
git add internal/svc/draftsvc/snapshot.go internal/svc/draftsvc/snapshot_test.go
git commit -m "feat(draftsvc): snapshot store with bounded retention and pinning"
```

---

## Task 8: draftsvc.Service.Pull — WeekReport → WeekDraft + watermark

Convert a live `WeekReport` to a `WeekDraft` with one row per `(target, type, billable)` and one cell per source entry. Compute the remote fingerprint (SHA-256 over a canonical encoding of the report's entries).

**Files:**
- Create: `internal/svc/draftsvc/pull.go`
- Create: `internal/svc/draftsvc/pull_test.go`
- Create: `internal/svc/draftsvc/service.go` (the Service struct + constructors)

- [ ] **Step 8.1 — Write failing test**

```go
// internal/svc/draftsvc/pull_test.go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

func TestBuildDraftFromReport_GroupsByTargetTypeBillable(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    report := domain.WeekReport{
        WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
        UserUID: "user-1",
        Status:  domain.ReportOpen,
        Entries: []domain.TimeEntry{
            {ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 480,
                Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
                TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
                Description: "morning"},
            {ID: 101, Date: week.AddDate(0, 0, 2), Minutes: 480,
                Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
                TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
                Description: "afternoon"},
            {ID: 102, Date: week.AddDate(0, 0, 5), Minutes: 240,
                Target: domain.Target{Kind: domain.TargetProject, ItemID: 456},
                TimeType: domain.TimeType{ID: 9, Name: "Planning"}, Billable: false},
        },
    }

    draft := buildDraftFromReport("work", "default", report)

    if got := len(draft.Rows); got != 2 {
        t.Fatalf("rows = %d, want 2", got)
    }
    var ticketRow *domain.DraftRow
    for i := range draft.Rows {
        if draft.Rows[i].Target.Kind == domain.TargetTicket { ticketRow = &draft.Rows[i] }
    }
    if ticketRow == nil { t.Fatal("ticket row missing") }
    if got := len(ticketRow.Cells); got != 2 {
        t.Errorf("ticket row cells = %d, want 2 (Mon+Tue)", got)
    }
    seenIDs := map[int]bool{}
    for _, c := range ticketRow.Cells {
        if c.Hours != 8.0 { t.Errorf("hours = %v, want 8.0", c.Hours) }
        seenIDs[c.SourceEntryID] = true
    }
    if !seenIDs[100] || !seenIDs[101] {
        t.Errorf("source IDs not preserved: %v", seenIDs)
    }
}

func TestComputeRemoteFingerprint_Stable(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    a := domain.WeekReport{
        WeekRef: domain.WeekRef{StartDate: week},
        Entries: []domain.TimeEntry{
            {ID: 1, Date: week.AddDate(0, 0, 1), Minutes: 60, ModifiedAt: time.Time{}},
            {ID: 2, Date: week.AddDate(0, 0, 2), Minutes: 30, ModifiedAt: time.Time{}},
        },
    }
    b := domain.WeekReport{
        WeekRef: domain.WeekRef{StartDate: week},
        Entries: []domain.TimeEntry{
            // different order
            {ID: 2, Date: week.AddDate(0, 0, 2), Minutes: 30, ModifiedAt: time.Now()},
            {ID: 1, Date: week.AddDate(0, 0, 1), Minutes: 60, ModifiedAt: time.Now()},
        },
    }
    if computeRemoteFingerprint(a) != computeRemoteFingerprint(b) {
        t.Errorf("fingerprint not stable across order/modifiedAt")
    }
}
```

- [ ] **Step 8.2 — Run, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run "TestBuildDraft\|TestComputeRemote" -v
```
Expected: FAIL.

- [ ] **Step 8.3 — Implement**

```go
// internal/svc/draftsvc/pull.go
package draftsvc

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "sort"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

type rowGroupKey struct {
    kind     domain.TargetKind
    appID    int
    itemID   int
    taskID   int
    typeID   int
    billable bool
}

func buildDraftFromReport(profile, name string, report domain.WeekReport) domain.WeekDraft {
    weekStart := report.WeekRef.StartDate
    groups := map[rowGroupKey]*domain.DraftRow{}
    var order []rowGroupKey

    for _, e := range report.Entries {
        k := rowGroupKey{
            kind: e.Target.Kind, appID: e.Target.AppID, itemID: e.Target.ItemID,
            taskID: e.Target.TaskID, typeID: e.TimeType.ID, billable: e.Billable,
        }
        row, ok := groups[k]
        if !ok {
            row = &domain.DraftRow{
                Target: e.Target, TimeType: e.TimeType, Billable: e.Billable,
                Description: e.Description,
                Label:       displayLabel(e.Target),
                ResolverHints: domain.ResolverHints{
                    TargetDisplayName: e.Target.DisplayName, TimeTypeName: e.TimeType.Name,
                },
            }
            groups[k] = row
            order = append(order, k)
        }
        // Compute weekday using calendar-day arithmetic (DST-safe).
        ey, em, ed := e.Date.Date()
        ry, rm, rd := weekStart.Date()
        entryDay := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
        refDay := time.Date(ry, rm, rd, 0, 0, 0, 0, time.UTC)
        dayIdx := int(entryDay.Sub(refDay).Hours() / 24)
        if dayIdx < 0 || dayIdx >= 7 { continue }
        wd := weekStart.AddDate(0, 0, dayIdx).Weekday()

        row.Cells = append(row.Cells, domain.DraftCell{
            Day: wd, Hours: float64(e.Minutes) / 60.0, SourceEntryID: e.ID,
        })
    }

    rows := make([]domain.DraftRow, 0, len(order))
    for _, k := range order { rows = append(rows, *groups[k]) }
    // Sort rows by total hours descending (stable).
    sort.SliceStable(rows, func(i, j int) bool {
        return totalHours(rows[i]) > totalHours(rows[j])
    })
    for i := range rows {
        rows[i].ID = fmt.Sprintf("row-%02d", i+1)
        // Sort cells in weekday order for determinism.
        sort.SliceStable(rows[i].Cells, func(a, b int) bool {
            return rows[i].Cells[a].Day < rows[i].Cells[b].Day
        })
    }

    now := time.Now().UTC()
    return domain.WeekDraft{
        SchemaVersion: 1,
        Profile:       profile,
        WeekStart:     weekStart,
        Name:          name,
        Provenance: domain.DraftProvenance{
            Kind:              domain.ProvenancePulled,
            PulledAt:          now,
            RemoteFingerprint: computeRemoteFingerprint(report),
            RemoteStatus:      report.Status,
        },
        CreatedAt:  now,
        ModifiedAt: now,
        Rows:       rows,
    }
}

func totalHours(r domain.DraftRow) float64 {
    var sum float64
    for _, c := range r.Cells { sum += c.Hours }
    return sum
}

func displayLabel(t domain.Target) string {
    if t.DisplayName != "" { return t.DisplayName }
    return t.DisplayRef
}

// computeRemoteFingerprint is order-stable and ignores fields TD touches
// automatically (CreatedAt, ModifiedAt). Two reports with the same canonical
// entry set produce the same fingerprint regardless of entry order.
func computeRemoteFingerprint(r domain.WeekReport) string {
    type fpEntry struct {
        ID         int    `json:"id"`
        Date       string `json:"date"`
        Minutes    int    `json:"minutes"`
        TimeTypeID int    `json:"timeTypeID"`
        Billable   bool   `json:"billable"`
        TargetKind string `json:"targetKind"`
        TargetAppID int   `json:"targetAppID"`
        TargetItemID int  `json:"targetItemID"`
        TargetTaskID int  `json:"targetTaskID"`
        Description string `json:"description"`
    }
    out := make([]fpEntry, 0, len(r.Entries))
    for _, e := range r.Entries {
        out = append(out, fpEntry{
            ID: e.ID, Date: e.Date.Format("2006-01-02"), Minutes: e.Minutes,
            TimeTypeID: e.TimeType.ID, Billable: e.Billable,
            TargetKind: string(e.Target.Kind), TargetAppID: e.Target.AppID,
            TargetItemID: e.Target.ItemID, TargetTaskID: e.Target.TaskID,
            Description: e.Description,
        })
    }
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].ID != out[j].ID { return out[i].ID < out[j].ID }
        return out[i].Date < out[j].Date
    })
    canonical := struct {
        WeekStart string    `json:"weekStart"`
        Status    string    `json:"status"`
        Entries   []fpEntry `json:"entries"`
    }{
        WeekStart: r.WeekRef.StartDate.Format("2006-01-02"),
        Status:    string(r.Status),
        Entries:   out,
    }
    data, _ := json.Marshal(canonical)
    h := sha256.Sum256(data)
    return fmt.Sprintf("%x", h)
}
```

- [ ] **Step 8.4 — Add Service struct**

```go
// internal/svc/draftsvc/service.go
package draftsvc

import (
    "context"
    "fmt"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// Service is the draft-aware service layer.
type Service struct {
    paths     config.Paths
    store     *Store
    snapshots *SnapshotStore
    tsvc      *timesvc.Service
}

func NewService(paths config.Paths, tsvc *timesvc.Service) *Service {
    return &Service{
        paths:     paths,
        store:     NewStore(paths),
        snapshots: NewSnapshotStore(paths, 10),
        tsvc:      tsvc,
    }
}

func (s *Service) Store() *Store               { return s.store }
func (s *Service) Snapshots() *SnapshotStore   { return s.snapshots }

// Pull fetches the live week and saves it as a draft. Refuses to overwrite
// a dirty draft unless force=true (auto-snapshots first when forcing).
func (s *Service) Pull(ctx context.Context, profile string, weekStart time.Time, name string, force bool) (domain.WeekDraft, error) {
    if name == "" { name = "default" }
    if existing, err := s.store.Load(profile, weekStart, name); err == nil {
        // Existing draft. Check dirty state.
        pulledByKey := pulledCellsByKey(existing)
        sync := domain.ComputeSyncState(existing, pulledByKey, "")
        if sync.Sync == domain.SyncDirty && !force {
            return domain.WeekDraft{}, fmt.Errorf(
                "dirty draft exists for %s/%s/%s; pass force=true (auto-snapshots) or use refresh",
                profile, weekStart.Format("2006-01-02"), name)
        }
        if sync.Sync == domain.SyncDirty && force {
            if _, err := s.snapshots.Take(existing, OpPrePull, ""); err != nil {
                return domain.WeekDraft{}, fmt.Errorf("auto-snapshot before force pull: %w", err)
            }
        }
    }

    report, err := s.tsvc.GetWeekReport(ctx, profile, weekStart)
    if err != nil { return domain.WeekDraft{}, fmt.Errorf("fetch week: %w", err) }

    draft := buildDraftFromReport(profile, name, report)
    if err := s.store.Save(draft); err != nil { return domain.WeekDraft{}, err }
    return draft, nil
}

// pulledCellsByKey snapshots the draft's current cells as the "pulled at pull time"
// view. Used by ComputeSyncState. For a freshly-pulled draft this is identical
// to draft.Rows; for an edited draft this represents the last-known clean state
// reconstructed from cells with non-zero SourceEntryID.
//
// In Phase A, since we only support pull-and-edit (no merge), this is just
// the current cells: sync state is dirty when *any* cell has been touched
// relative to its hours-at-pull-time, which we track by saving a sibling
// "pulled.snapshot" alongside the draft on every successful pull.
func pulledCellsByKey(d domain.WeekDraft) map[string]domain.DraftCell {
    out := map[string]domain.DraftCell{}
    for _, row := range d.Rows {
        for _, cell := range row.Cells {
            if cell.SourceEntryID == 0 { continue }
            key := fmt.Sprintf("%s:%s", row.ID, cell.Day)
            out[key] = cell
        }
    }
    return out
}
```

> **Note for the implementer:** the simple `pulledCellsByKey` above does not actually track the at-pull-time hours separately from the current hours — it only knows which cells originated from a pull (by `SourceEntryID`). For accurate dirty detection in Phase A, save a "pulled snapshot" sibling file (e.g., `<draftname>.pulled.yaml`) on every successful pull, and load it during `ComputeSyncState`. The test in Step 4.1 already assumes the pulled-by-key map is supplied separately — so this concern lives in the calling layer (CLI status command and MCP tools), not in the Service.Pull implementation.

- [ ] **Step 8.5 — Add a pulled-snapshot sibling**

In `Pull()`, after `s.store.Save(draft)`, also save a sibling pulled snapshot to track at-pull-time state:

```go
// Save pulled snapshot for accurate dirty-detection later.
if err := s.savePulledSnapshot(draft); err != nil {
    return domain.WeekDraft{}, fmt.Errorf("save pulled snapshot: %w", err)
}
```

Add helper to `internal/svc/draftsvc/store.go`:

```go
func (s *Store) pulledSnapshotPath(profile string, weekStart time.Time, name string) string {
    return s.draftPath(profile, weekStart, name+".pulled")
}
func (s *Store) SavePulledSnapshot(d domain.WeekDraft) error {
    p := s.pulledSnapshotPath(d.Profile, d.WeekStart, d.Name)
    if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil { return err }
    data, _ := yaml.Marshal(d)
    return os.WriteFile(p, data, 0o600)
}
func (s *Store) LoadPulledSnapshot(profile string, weekStart time.Time, name string) (domain.WeekDraft, error) {
    p := s.pulledSnapshotPath(profile, weekStart, name)
    data, err := os.ReadFile(p)
    if err != nil { return domain.WeekDraft{}, err }
    var d domain.WeekDraft
    if err := yaml.Unmarshal(data, &d); err != nil { return domain.WeekDraft{}, err }
    return d, nil
}
```

And in `Service`:

```go
func (s *Service) savePulledSnapshot(d domain.WeekDraft) error {
    return s.store.SavePulledSnapshot(d)
}

// PulledCellsByKey returns the at-pull-time cells map for sync-state computation.
// Returns an empty map if no pulled snapshot exists (nascent or imported drafts).
func (s *Service) PulledCellsByKey(profile string, weekStart time.Time, name string) (map[string]domain.DraftCell, error) {
    snap, err := s.store.LoadPulledSnapshot(profile, weekStart, name)
    if err != nil {
        if os.IsNotExist(err) { return map[string]domain.DraftCell{}, nil }
        return nil, err
    }
    return pulledCellsByKey(snap), nil
}
```

- [ ] **Step 8.6 — Run all tests, verify pass**

```bash
go test ./... -v
```
Expected: PASS.

- [ ] **Step 8.7 — Commit**

```bash
git add internal/svc/draftsvc/
git commit -m "feat(draftsvc): Pull and Service scaffolding with pulled-snapshot for dirty detection"
```

---

## Task 9: draftsvc.Reconcile + draft diff hash

Compute `ReconcileDiff` for a draft against current remote, classifying actions by cell state. Cleared pulled cell (`hours=0` + `SourceEntryID > 0`) → `ActionDelete`. Edited pulled cell (hours changed) → `ActionUpdate`. Added cell (no source) → `ActionCreate` (or skip per `--mode`). Untouched → `ActionSkip(noChange)`.

Hash includes the pull watermark fingerprint.

**Files:**
- Create: `internal/svc/draftsvc/reconcile.go`
- Create: `internal/svc/draftsvc/reconcile_test.go`

- [ ] **Step 9.1 — Write failing tests**

```go
// internal/svc/draftsvc/reconcile_test.go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// (full table-driven test — see implementation; covers create/update/delete/skip/blocked/hash-stability)
func TestReconcile_DeleteOnClearedPulledCell(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    pulledRow := domain.DraftRow{
        ID: "row-01",
        Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
        TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
        Cells: []domain.DraftCell{
            {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
            {Day: time.Tuesday, Hours: 0, SourceEntryID: 98732}, // cleared
        },
    }
    draft := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Rows: []domain.DraftRow{pulledRow},
        Provenance: domain.DraftProvenance{
            Kind: domain.ProvenancePulled, RemoteFingerprint: "fp1",
        },
    }
    pulled := map[string]domain.DraftCell{
        "row-01:Monday":  {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
        "row-01:Tuesday": {Day: time.Tuesday, Hours: 8.0, SourceEntryID: 98732},
    }
    report := domain.WeekReport{
        WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0,0,6)},
        Status: domain.ReportOpen,
        Entries: []domain.TimeEntry{
            {ID: 98731, Date: week.AddDate(0,0,1), Minutes: 480,
                Target: pulledRow.Target, TimeType: pulledRow.TimeType, Billable: true},
            {ID: 98732, Date: week.AddDate(0,0,2), Minutes: 480,
                Target: pulledRow.Target, TimeType: pulledRow.TimeType, Billable: true},
        },
    }

    diff, err := reconcileDraft(draft, pulled, report, []domain.LockedDay{}, "fp1", "user-1")
    if err != nil { t.Fatalf("reconcile: %v", err) }

    var hasDelete bool
    for _, a := range diff.Actions {
        if a.Kind == domain.ActionDelete && a.DeleteEntryID == 98732 { hasDelete = true }
    }
    if !hasDelete { t.Errorf("expected ActionDelete for entry 98732, got %+v", diff.Actions) }

    // Untouched Monday cell → skip with noChange reason.
    var hasMondaySkip bool
    for _, a := range diff.Actions {
        if a.Kind == domain.ActionSkip && a.Date.Weekday() == time.Monday {
            hasMondaySkip = true
        }
    }
    if !hasMondaySkip { t.Errorf("expected skip for untouched Monday") }
}

func TestReconcile_HashStableAndIncludesWatermark(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    row := domain.DraftRow{
        ID: "row-01",
        Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
        TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
        Cells: []domain.DraftCell{{Day: time.Monday, Hours: 6.0, SourceEntryID: 98731}},
    }
    draft := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Rows: []domain.DraftRow{row},
        Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fpA"},
    }
    pulled := map[string]domain.DraftCell{
        "row-01:Monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
    }
    report := domain.WeekReport{
        WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0,0,6)},
        Status: domain.ReportOpen,
        Entries: []domain.TimeEntry{
            {ID: 98731, Date: week.AddDate(0,0,1), Minutes: 480,
                Target: row.Target, TimeType: row.TimeType, Billable: true},
        },
    }

    diff1, err := reconcileDraft(draft, pulled, report, nil, "fpA", "user-1")
    if err != nil { t.Fatal(err) }
    diff2, err := reconcileDraft(draft, pulled, report, nil, "fpA", "user-1")
    if err != nil { t.Fatal(err) }
    if diff1.DiffHash != diff2.DiffHash {
        t.Errorf("hash unstable across runs: %s vs %s", diff1.DiffHash, diff2.DiffHash)
    }
    diff3, err := reconcileDraft(draft, pulled, report, nil, "fpDIFFERENT", "user-1")
    if err != nil { t.Fatal(err) }
    if diff1.DiffHash == diff3.DiffHash {
        t.Errorf("hash did not change with watermark; expected different hashes")
    }
}

func TestReconcile_LockedDayBlocks(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    monday := week.AddDate(0, 0, 1)
    row := domain.DraftRow{
        ID: "row-01",
        Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
        TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
        Cells: []domain.DraftCell{{Day: time.Monday, Hours: 6.0, SourceEntryID: 98731}},
    }
    draft := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Rows: []domain.DraftRow{row},
        Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fp"},
    }
    pulled := map[string]domain.DraftCell{
        "row-01:Monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
    }
    report := domain.WeekReport{
        WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0,0,6)},
        Status: domain.ReportOpen,
        Entries: []domain.TimeEntry{
            {ID: 98731, Date: monday, Minutes: 480, Target: row.Target,
                TimeType: row.TimeType, Billable: true},
        },
    }
    locked := []domain.LockedDay{{Date: monday}}

    diff, err := reconcileDraft(draft, pulled, report, locked, "fp", "user-1")
    if err != nil { t.Fatal(err) }
    if len(diff.Actions) != 0 {
        t.Errorf("expected 0 actions for locked day, got %d", len(diff.Actions))
    }
    if len(diff.Blockers) != 1 || diff.Blockers[0].Kind != domain.BlockerLocked {
        t.Errorf("expected 1 BlockerLocked, got %+v", diff.Blockers)
    }
}

func TestReconcile_SubmittedWeekRefuses(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    row := domain.DraftRow{
        ID: "row-01",
        Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
        TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
        Cells: []domain.DraftCell{
            {Day: time.Monday, Hours: 6.0, SourceEntryID: 98731},
            {Day: time.Tuesday, Hours: 4.0},
        },
    }
    draft := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Rows: []domain.DraftRow{row},
        Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fp"},
    }
    pulled := map[string]domain.DraftCell{
        "row-01:Monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
    }
    report := domain.WeekReport{
        WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0,0,6)},
        Status: domain.ReportSubmitted,
        Entries: []domain.TimeEntry{
            {ID: 98731, Date: week.AddDate(0,0,1), Minutes: 480, Target: row.Target,
                TimeType: row.TimeType, Billable: true},
        },
    }

    diff, err := reconcileDraft(draft, pulled, report, nil, "fp", "user-1")
    if err != nil { t.Fatal(err) }
    if len(diff.Actions) != 0 {
        t.Errorf("submitted week: expected 0 actions, got %d", len(diff.Actions))
    }
    if len(diff.Blockers) != 2 {
        t.Errorf("submitted week: expected 2 blockers (one per non-untouched cell), got %d", len(diff.Blockers))
    }
    for _, b := range diff.Blockers {
        if b.Kind != domain.BlockerSubmitted {
            t.Errorf("expected BlockerSubmitted, got %s", b.Kind)
        }
    }
}
```

- [ ] **Step 9.2 — Run, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run TestReconcile -v
```
Expected: FAIL — `reconcileDraft` undefined.

- [ ] **Step 9.3 — Implement reconcile**

```go
// internal/svc/draftsvc/reconcile.go
package draftsvc

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "math"
    "sort"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// reconcileDraft is the core engine: given a draft + at-pull-time cells +
// current remote report + locked days + watermark fingerprint, produce a
// ReconcileDiff describing what push will do.
func reconcileDraft(
    draft domain.WeekDraft,
    pulledByKey map[string]domain.DraftCell,
    report domain.WeekReport,
    locked []domain.LockedDay,
    pullWatermark string,
    userUID string,
) (domain.ReconcileDiff, error) {
    // Build locked-day set.
    lockedSet := map[string]bool{}
    for _, ld := range locked {
        lockedSet[ld.Date.Format("2006-01-02")] = true
    }

    // Build remote-by-id index for fast lookup.
    remoteByID := map[int]domain.TimeEntry{}
    for _, e := range report.Entries { remoteByID[e.ID] = e }

    // Determine week-level blocker.
    weekBlocked, weekBlockerKind, weekReason := false, domain.BlockerSubmitted, ""
    switch report.Status {
    case domain.ReportSubmitted:
        weekBlocked, weekReason = true, "week report is submitted for approval"
    case domain.ReportApproved:
        weekBlocked, weekBlockerKind, weekReason = true, domain.BlockerApproved, "week report is approved"
    }

    var actions []domain.Action
    var blockers []domain.Blocker

    for _, row := range draft.Rows {
        for _, cell := range row.Cells {
            date := domain.WeekRefContaining(draft.WeekStart).StartDate.
                AddDate(0, 0, int(cell.Day))
            dateStr := date.Format("2006-01-02")
            key := fmt.Sprintf("%s:%s", row.ID, cell.Day)
            pulled := pulledByKey[key]

            // Compute the cell's state.
            state := domain.ComputeCellState(pulled, cell)

            // Untouched cells produce no actions (or noChange skips for visibility).
            if state == domain.CellUntouched {
                if cell.Hours > 0 && cell.SourceEntryID != 0 {
                    actions = append(actions, domain.Action{
                        Kind: domain.ActionSkip, RowID: row.ID, Date: date,
                        SkipReason: "noChange", ExistingID: cell.SourceEntryID,
                    })
                }
                continue
            }

            // Week-level / locked blockers apply BEFORE other actions.
            if weekBlocked {
                blockers = append(blockers, domain.Blocker{
                    Kind: weekBlockerKind, RowID: row.ID, Date: date, Reason: weekReason,
                })
                continue
            }
            if lockedSet[dateStr] {
                blockers = append(blockers, domain.Blocker{
                    Kind: domain.BlockerLocked, RowID: row.ID, Date: date,
                    Reason: fmt.Sprintf("%s is locked", dateStr),
                })
                continue
            }

            // Cleared pulled cell → delete.
            if cell.Hours == 0 && cell.SourceEntryID != 0 {
                actions = append(actions, domain.Action{
                    Kind: domain.ActionDelete, RowID: row.ID, Date: date,
                    DeleteEntryID: cell.SourceEntryID,
                })
                continue
            }
            // Skip cells that simply have no value AND no remote source.
            if cell.Hours == 0 { continue }

            // Round-to-minute check.
            rawMin := cell.Hours * 60
            mins := int(math.Round(rawMin))
            if math.Abs(rawMin-float64(mins)) > 0.001 {
                return domain.ReconcileDiff{}, fmt.Errorf(
                    "row %s on %s: %.4fh produces non-integer minutes",
                    row.ID, dateStr, cell.Hours)
            }

            entryInput := domain.EntryInput{
                UserUID: userUID, Date: date, Minutes: mins,
                TimeTypeID: row.TimeType.ID, Billable: row.Billable,
                Target: row.Target, Description: row.Description,
            }

            // If the cell has a source ID and exists remotely, emit Update with patch.
            if cell.SourceEntryID != 0 {
                if existing, ok := remoteByID[cell.SourceEntryID]; ok {
                    patch := buildDraftPatch(existing, entryInput)
                    if patch.IsEmpty() {
                        actions = append(actions, domain.Action{
                            Kind: domain.ActionSkip, RowID: row.ID, Date: date,
                            ExistingID: cell.SourceEntryID, SkipReason: "noChange",
                        })
                    } else {
                        actions = append(actions, domain.Action{
                            Kind: domain.ActionUpdate, RowID: row.ID, Date: date,
                            ExistingID: cell.SourceEntryID, Patch: patch,
                        })
                    }
                    continue
                }
                // Source ID set but the entry no longer exists on remote.
                // Treat as a stale reference — emit a Create.
            }

            // Added cell → Create.
            actions = append(actions, domain.Action{
                Kind: domain.ActionCreate, RowID: row.ID, Date: date, Entry: entryInput,
            })
        }
    }

    // Sort for deterministic hashing.
    sort.SliceStable(actions, func(i, j int) bool {
        if actions[i].RowID != actions[j].RowID {
            return actions[i].RowID < actions[j].RowID
        }
        return actions[i].Date.Before(actions[j].Date)
    })
    sort.SliceStable(blockers, func(i, j int) bool {
        if blockers[i].RowID != blockers[j].RowID {
            return blockers[i].RowID < blockers[j].RowID
        }
        return blockers[i].Date.Before(blockers[j].Date)
    })

    hash := computeDraftDiffHash(actions, blockers, draft.Name, draft.WeekStart, pullWatermark)
    return domain.ReconcileDiff{Actions: actions, Blockers: blockers, DiffHash: hash}, nil
}

func buildDraftPatch(existing domain.TimeEntry, desired domain.EntryInput) domain.EntryUpdate {
    var p domain.EntryUpdate
    if existing.Minutes != desired.Minutes { m := desired.Minutes; p.Minutes = &m }
    if existing.TimeType.ID != desired.TimeTypeID { id := desired.TimeTypeID; p.TimeTypeID = &id }
    if existing.Billable != desired.Billable { b := desired.Billable; p.Billable = &b }
    if existing.Description != desired.Description { d := desired.Description; p.Description = &d }
    return p
}

func computeDraftDiffHash(actions []domain.Action, blockers []domain.Blocker, name string, weekStart time.Time, watermark string) string {
    type ha struct {
        Kind, RowID, Date, SkipReason string
        Minutes, TimeTypeID, ExistingID, DeleteEntryID int
        Billable *bool
        Description string
    }
    type hb struct{ Kind, RowID, Date string }
    type input struct {
        Actions []ha; Blockers []hb
        Name, WeekStart, Watermark string
    }
    in := input{Name: name, WeekStart: weekStart.Format("2006-01-02"), Watermark: watermark}
    for _, a := range actions {
        x := ha{Kind: a.Kind.String(), RowID: a.RowID, Date: a.Date.Format("2006-01-02"),
            SkipReason: a.SkipReason, ExistingID: a.ExistingID, DeleteEntryID: a.DeleteEntryID}
        switch a.Kind {
        case domain.ActionCreate:
            x.Minutes, x.TimeTypeID = a.Entry.Minutes, a.Entry.TimeTypeID
            b := a.Entry.Billable; x.Billable = &b
            x.Description = a.Entry.Description
        case domain.ActionUpdate:
            if a.Patch.Minutes != nil { x.Minutes = *a.Patch.Minutes }
            if a.Patch.TimeTypeID != nil { x.TimeTypeID = *a.Patch.TimeTypeID }
            x.Billable = a.Patch.Billable
            if a.Patch.Description != nil { x.Description = *a.Patch.Description }
        }
        in.Actions = append(in.Actions, x)
    }
    for _, bl := range blockers {
        in.Blockers = append(in.Blockers, hb{Kind: bl.Kind.String(), RowID: bl.RowID, Date: bl.Date.Format("2006-01-02")})
    }
    data, _ := json.Marshal(in)
    h := sha256.Sum256(data)
    return fmt.Sprintf("%x", h)
}
```

- [ ] **Step 9.4 — Wire reconcile into Service**

Add to `internal/svc/draftsvc/service.go`:

```go
// Reconcile loads current remote state and produces a ReconcileDiff for the named draft.
func (s *Service) Reconcile(ctx context.Context, profile string, weekStart time.Time, name string) (domain.WeekDraft, domain.ReconcileDiff, error) {
    if name == "" { name = "default" }
    draft, err := s.store.Load(profile, weekStart, name)
    if err != nil { return domain.WeekDraft{}, domain.ReconcileDiff{}, err }

    pulled, err := s.PulledCellsByKey(profile, weekStart, name)
    if err != nil { return domain.WeekDraft{}, domain.ReconcileDiff{}, err }

    report, err := s.tsvc.GetWeekReport(ctx, profile, weekStart)
    if err != nil { return domain.WeekDraft{}, domain.ReconcileDiff{}, err }
    locked, err := s.tsvc.GetLockedDays(ctx, profile, weekStart, weekStart.AddDate(0,0,6))
    if err != nil { return domain.WeekDraft{}, domain.ReconcileDiff{}, err }

    diff, err := reconcileDraft(draft, pulled, report, locked, computeRemoteFingerprint(report), draft.Provenance.RemoteStatus.String())
    if err != nil { return draft, domain.ReconcileDiff{}, err }
    return draft, diff, nil
}
```

(Adjust `userUID` argument: thread it through from a `whoami` call — or fetch via `authsvc`. Simplest pattern: make `Reconcile` take `userUID` as a parameter and have CLI/MCP layers resolve it. Update tests and callers accordingly.)

- [ ] **Step 9.5 — Run tests, verify pass**

```bash
go test ./internal/svc/draftsvc/ -v
```
Expected: PASS.

- [ ] **Step 9.6 — Commit**

```bash
git add internal/svc/draftsvc/reconcile.go internal/svc/draftsvc/reconcile_test.go internal/svc/draftsvc/service.go
git commit -m "feat(draftsvc): draft-aware reconcile with ActionDelete and watermark hash"
```

---

## Task 10: draftsvc.Apply — push with hash protection

Re-runs reconcile, verifies `expectedDiffHash` match, executes Create/Update/Delete via timesvc, requires `allowDeletes=true` if any delete actions.

**Files:**
- Create: `internal/svc/draftsvc/apply.go`
- Create: `internal/svc/draftsvc/apply_test.go`

- [ ] **Step 10.1 — Failing test**

The cleanest way to test Apply without a real TD tenant is to inject a `timesvc`-shaped interface (mock) into the Service. Refactor `Service` to depend on a small interface (`timeWriter`) that the real `timesvc.Service` already satisfies. Then write the test using a recording mock.

```go
// internal/svc/draftsvc/apply_test.go
package draftsvc

import (
    "context"
    "fmt"
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

type mockTimeWriter struct {
    creates []domain.EntryInput
    updates []struct{ ID int; Patch domain.EntryUpdate }
    deletes []int
    failOn  string // "create" | "update" | "delete" — simulate failure
}

func (m *mockTimeWriter) AddEntry(_ context.Context, _ string, e domain.EntryInput) (int, error) {
    if m.failOn == "create" { return 0, fmt.Errorf("simulated create failure") }
    m.creates = append(m.creates, e)
    return 1000 + len(m.creates), nil
}
func (m *mockTimeWriter) UpdateEntry(_ context.Context, _ string, id int, p domain.EntryUpdate) (domain.TimeEntry, error) {
    if m.failOn == "update" { return domain.TimeEntry{}, fmt.Errorf("simulated update failure") }
    m.updates = append(m.updates, struct{ ID int; Patch domain.EntryUpdate }{id, p})
    return domain.TimeEntry{ID: id}, nil
}
func (m *mockTimeWriter) DeleteEntry(_ context.Context, _ string, id int) error {
    if m.failOn == "delete" { return fmt.Errorf("simulated delete failure") }
    m.deletes = append(m.deletes, id)
    return nil
}
// Stub the read methods used by Reconcile so we can test Apply end-to-end.
func (m *mockTimeWriter) GetWeekReport(context.Context, string, time.Time) (domain.WeekReport, error) {
    return domain.WeekReport{ /* populated by tests via embedding */ }, nil
}
func (m *mockTimeWriter) GetLockedDays(context.Context, string, time.Time, time.Time) ([]domain.LockedDay, error) {
    return nil, nil
}

func TestApply_AllowDeletesGate(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    paths := config.Paths{Root: t.TempDir()}

    // Construct draft with one cleared pulled cell (delete-on-push).
    draft := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Provenance: domain.DraftProvenance{
            Kind: domain.ProvenancePulled, RemoteFingerprint: "fp1",
            RemoteStatus: domain.ReportOpen,
        },
        Rows: []domain.DraftRow{{
            ID: "row-01",
            Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
            TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
            Cells: []domain.DraftCell{{Day: time.Monday, Hours: 0, SourceEntryID: 98731}},
        }},
    }
    store := NewStore(paths)
    if err := store.Save(draft); err != nil { t.Fatal(err) }
    pulled := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Rows: []domain.DraftRow{{
            ID: "row-01",
            Cells: []domain.DraftCell{{Day: time.Monday, Hours: 8.0, SourceEntryID: 98731}},
        }},
    }
    if err := store.SavePulledSnapshot(pulled); err != nil { t.Fatal(err) }

    mw := &mockTimeWriter{}
    s := newServiceWithTimeWriter(paths, mw) // helper introduced in Step 10.3

    // First, do a preview to capture the diff hash.
    _, diff, err := s.Reconcile(context.Background(), "work", week, "default")
    if err != nil { t.Fatal(err) }

    // allowDeletes=false → must refuse.
    _, err = s.Apply(context.Background(), "work", week, "default", diff.DiffHash, false, "user-1")
    if err == nil {
        t.Fatal("expected error when allowDeletes=false with delete actions")
    }
    if len(mw.deletes) != 0 {
        t.Errorf("delete attempted despite refusal: %v", mw.deletes)
    }

    // allowDeletes=true → succeeds.
    res, err := s.Apply(context.Background(), "work", week, "default", diff.DiffHash, true, "user-1")
    if err != nil { t.Fatalf("Apply: %v", err) }
    if res.Deleted != 1 { t.Errorf("Deleted = %d, want 1", res.Deleted) }
    if len(mw.deletes) != 1 || mw.deletes[0] != 98731 {
        t.Errorf("expected delete of 98731, got %v", mw.deletes)
    }
}

func TestApply_HashMismatchRefuses(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    paths := config.Paths{Root: t.TempDir()}

    draft := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fp1"},
        Rows: []domain.DraftRow{{
            ID: "row-01", Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4.0}},
        }},
    }
    store := NewStore(paths)
    if err := store.Save(draft); err != nil { t.Fatal(err) }

    mw := &mockTimeWriter{}
    s := newServiceWithTimeWriter(paths, mw)

    _, err := s.Apply(context.Background(), "work", week, "default", "wrong-hash", false, "user-1")
    if err == nil {
        t.Fatal("expected hash mismatch error")
    }
    if len(mw.creates)+len(mw.updates)+len(mw.deletes) > 0 {
        t.Error("writes attempted despite hash mismatch")
    }
}
```

- [ ] **Step 10.2 — Run, verify failure**
- [ ] **Step 10.3 — Implement Apply**

```go
// internal/svc/draftsvc/apply.go
package draftsvc

import (
    "context"
    "fmt"

    "github.com/iainmoffat/tdx/internal/domain"
)

type ApplyResult struct {
    Created, Updated, Deleted, Skipped int
    Failed []ApplyFailure
}

type ApplyFailure struct {
    Kind, RowID, Date, Message string
    EntryID int
}

// Apply executes the actions in the diff after re-verifying the hash.
// allowDeletes must be true if any ActionDelete actions are present.
func (s *Service) Apply(ctx context.Context, profile string, weekStart time.Time, name string, expectedHash string, allowDeletes bool, userUID string) (ApplyResult, error) {
    draft, diff, err := s.Reconcile(ctx, profile, weekStart, name)
    if err != nil { return ApplyResult{}, err }

    if diff.DiffHash != expectedHash {
        return ApplyResult{}, fmt.Errorf("week changed since preview (hash mismatch)")
    }

    hasDeletes := false
    for _, a := range diff.Actions { if a.Kind == domain.ActionDelete { hasDeletes = true; break } }
    if hasDeletes && !allowDeletes {
        return ApplyResult{}, fmt.Errorf("draft contains delete actions; pass --allow-deletes to confirm")
    }

    // Auto-snapshot before any writes.
    if _, err := s.snapshots.Take(draft, OpPrePush, ""); err != nil {
        return ApplyResult{}, fmt.Errorf("auto-snapshot pre-push: %w", err)
    }

    var result ApplyResult
    for _, a := range diff.Actions {
        switch a.Kind {
        case domain.ActionCreate:
            if _, err := s.tsvc.AddEntry(ctx, profile, a.Entry); err != nil {
                result.Failed = append(result.Failed, ApplyFailure{
                    Kind: "create", RowID: a.RowID, Date: a.Date.Format("2006-01-02"), Message: err.Error()})
            } else { result.Created++ }
        case domain.ActionUpdate:
            if _, err := s.tsvc.UpdateEntry(ctx, profile, a.ExistingID, a.Patch); err != nil {
                result.Failed = append(result.Failed, ApplyFailure{
                    Kind: "update", RowID: a.RowID, Date: a.Date.Format("2006-01-02"),
                    EntryID: a.ExistingID, Message: err.Error()})
            } else { result.Updated++ }
        case domain.ActionDelete:
            if err := s.tsvc.DeleteEntry(ctx, profile, a.DeleteEntryID); err != nil {
                result.Failed = append(result.Failed, ApplyFailure{
                    Kind: "delete", RowID: a.RowID, Date: a.Date.Format("2006-01-02"),
                    EntryID: a.DeleteEntryID, Message: err.Error()})
            } else { result.Deleted++ }
        case domain.ActionSkip:
            result.Skipped++
        }
    }

    // Update pushedAt + refresh pulled snapshot from the now-current remote.
    now := time.Now().UTC()
    draft.PushedAt = &now
    if err := s.store.Save(draft); err != nil { return result, err }
    return result, nil
}
```

- [ ] **Step 10.4 — Run tests, verify pass**
- [ ] **Step 10.5 — Commit**

```bash
git add internal/svc/draftsvc/apply.go internal/svc/draftsvc/apply_test.go
git commit -m "feat(draftsvc): Apply with hash protection and --allow-deletes gate"
```

---

## Task 11: CLI scaffolding — `tdx time week` subcommand wiring + draft helpers

Add the new `pull/list/show --draft/status/edit/diff/preview/push/delete` commands as cobra subcommands of the existing `tdx time week`. Add a shared `draft.go` for parsing `<date>[/<name>]` tokens.

**Files:**
- Modify: `internal/cli/time/week/week.go` (register subcommands)
- Create: `internal/cli/time/week/draft.go` (shared helpers)
- Create: `internal/cli/time/week/draft_test.go`

- [ ] **Step 11.1 — Tests for token parsing**

```go
// internal/cli/time/week/draft_test.go
package week

import (
    "testing"
    "time"
)

func TestParseDraftRef(t *testing.T) {
    cases := []struct{
        in        string
        wantDate  string
        wantName  string
        wantErr   bool
    }{
        {"2026-05-04", "2026-05-04", "default", false},
        {"2026-05-04/pristine", "2026-05-04", "pristine", false},
        {"bogus", "", "", true},
        {"2026-05-04/", "", "", true},
    }
    for _, c := range cases {
        date, name, err := ParseDraftRef(c.in)
        if (err != nil) != c.wantErr {
            t.Errorf("%s: err=%v, wantErr=%v", c.in, err, c.wantErr)
            continue
        }
        if c.wantErr { continue }
        if date.Format("2006-01-02") != c.wantDate {
            t.Errorf("%s: date=%s, want %s", c.in, date.Format("2006-01-02"), c.wantDate)
        }
        if name != c.wantName {
            t.Errorf("%s: name=%q, want %q", c.in, name, c.wantName)
        }
    }
    _ = time.Now
}
```

- [ ] **Step 11.2 — Implement helper**

```go
// internal/cli/time/week/draft.go
package week

import (
    "fmt"
    "strings"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// ParseDraftRef parses a "<YYYY-MM-DD>[/<name>]" token. Defaults name to "default".
// Returns the weekStart (Sunday containing that date) and the name.
func ParseDraftRef(s string) (time.Time, string, error) {
    if s == "" { return time.Time{}, "", fmt.Errorf("draft reference required") }
    var dateStr, name string
    if i := strings.IndexByte(s, '/'); i >= 0 {
        dateStr, name = s[:i], s[i+1:]
        if name == "" {
            return time.Time{}, "", fmt.Errorf("empty name after slash in %q", s)
        }
    } else {
        dateStr, name = s, "default"
    }
    d, err := time.ParseInLocation("2006-01-02", dateStr, domain.EasternTZ)
    if err != nil { return time.Time{}, "", fmt.Errorf("invalid date %q: %w", dateStr, err) }
    return domain.WeekRefContaining(d).StartDate, name, nil
}
```

- [ ] **Step 11.3 — Run tests, verify pass**
- [ ] **Step 11.4 — Commit**

```bash
git add internal/cli/time/week/draft.go internal/cli/time/week/draft_test.go
git commit -m "feat(cli/week): ParseDraftRef helper for <date>[/<name>] tokens"
```

---

## Task 12: `tdx time week pull <date>`

**Files:**
- Create: `internal/cli/time/week/pull.go`
- Create: `internal/cli/time/week/pull_test.go`
- Modify: `internal/cli/time/week/week.go` (register)

- [ ] **Step 12.1 — Test** (CLI test using cobra command + a mock service or recorded HTTP fixtures; mirror Phase 3's pattern in `internal/cli/time/entry/add_test.go`).

- [ ] **Step 12.2 — Implement `pull.go`**

```go
package week

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/output"
    "github.com/iainmoffat/tdx/internal/cli/runtime"
)

func newPullCmd() *cobra.Command {
    var name string
    var force bool
    cmd := &cobra.Command{
        Use:   "pull <date>",
        Short: "Pull a live week into a local draft",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()

            weekStart, defaultName, err := ParseDraftRef(args[0])
            if err != nil { return err }
            if name == "" { name = defaultName }

            draft, err := rt.Drafts.Pull(ctx, rt.Profile, weekStart, name, force)
            if err != nil { return err }

            return output.RenderDraftPullResult(cmd.OutOrStdout(), draft, rt.JSON)
        },
    }
    cmd.Flags().StringVar(&name, "name", "", "draft name (default: default)")
    cmd.Flags().BoolVar(&force, "force", false, "overwrite a dirty draft (auto-snapshots first)")
    return cmd
}
```

> Build out `internal/cli/runtime` and `internal/cli/output` helpers if they don't already exist; or use the inline pattern from `internal/cli/time/entry/add.go`. Mirror the existing project conventions exactly — do NOT introduce a new style.

- [ ] **Step 12.3 — Register in `week.go`**, run tests, commit:

```bash
git add internal/cli/time/week/
git commit -m "feat(cli): tdx time week pull"
```

---

## Task 13: `tdx time week list`

**Files:**
- Create: `internal/cli/time/week/list.go`
- Create: `internal/cli/time/week/list_test.go`

JSON schema: `tdx.v1.weekDraftList`.

- [ ] **Step 13.1 — Failing test (cobra command behavior)**

```go
// internal/cli/time/week/list_test.go
package week

import (
    "bytes"
    "encoding/json"
    "strings"
    "testing"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
)

func TestList_EmptyJSON(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    drafts := draftsvc.NewStore(paths) // no draft saved
    _ = drafts

    cmd := newListCmdForTest(paths, "work", true /* json */)
    var out bytes.Buffer
    cmd.SetOut(&out)
    if err := cmd.Execute(); err != nil { t.Fatal(err) }

    var resp struct {
        Schema string `json:"schema"`
        Drafts []any  `json:"drafts"`
    }
    if err := json.Unmarshal(out.Bytes(), &resp); err != nil { t.Fatal(err) }
    if resp.Schema != "tdx.v1.weekDraftList" {
        t.Errorf("schema = %q", resp.Schema)
    }
    if len(resp.Drafts) != 0 { t.Errorf("expected empty drafts list") }
}

func TestList_FiltersDirty(t *testing.T) {
    // Save two drafts: one clean, one dirty. Verify --dirty surfaces only the dirty one.
    // (Implementer: build the store with two saved drafts + pulled snapshots that
    // produce clean vs dirty sync states; then run with --dirty and assert.)
    if testing.Short() { t.Skip() }
    _ = strings.Contains
}
```

- [ ] **Step 13.2 — Run, verify failure**

```bash
go test ./internal/cli/time/week/ -run TestList -v
```

- [ ] **Step 13.3 — Implement**

```go
// internal/cli/time/week/list.go
package week

import (
    "encoding/json"
    "fmt"
    "io"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/domain"
)

type weekDraftListItem struct {
    WeekStart   string                  `json:"weekStart"`
    Name        string                  `json:"name"`
    Profile     string                  `json:"profile"`
    SyncState   string                  `json:"syncState"`
    SyncDetail  domain.DraftSyncState   `json:"syncDetail"`
    TotalHours  float64                 `json:"totalHours"`
    PulledAt    string                  `json:"pulledAt,omitempty"`
}

type weekDraftListResp struct {
    Schema string              `json:"schema"`
    Drafts []weekDraftListItem `json:"drafts"`
}

func newListCmd() *cobra.Command {
    var dirty, conflicted, jsonOut, noRemote bool
    var dateFilter string

    cmd := &cobra.Command{
        Use:   "list",
        Short: "List local week drafts",
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()

            drafts, err := rt.Drafts.Store().List(rt.Profile)
            if err != nil { return err }

            items := make([]weekDraftListItem, 0, len(drafts))
            for _, d := range drafts {
                if dateFilter != "" && d.WeekStart.Format("2006-01-02") != dateFilter { continue }
                pulled, _ := rt.Drafts.PulledCellsByKey(rt.Profile, d.WeekStart, d.Name)
                fingerprint := ""
                if !noRemote {
                    fingerprint = rt.Drafts.ProbeRemoteFingerprint(ctx, rt.Profile, d.WeekStart) // helper added below
                }
                state := domain.ComputeSyncState(d, pulled, fingerprint)
                if dirty && state.Sync != domain.SyncDirty { continue }
                if conflicted && state.Sync != domain.SyncConflicted { continue }
                items = append(items, weekDraftListItem{
                    WeekStart: d.WeekStart.Format("2006-01-02"), Name: d.Name, Profile: d.Profile,
                    SyncState: string(state.Sync), SyncDetail: state, TotalHours: state.TotalHours,
                    PulledAt: formatTimeOrEmpty(d.Provenance.PulledAt),
                })
            }

            return renderList(cmd.OutOrStdout(), items, jsonOut)
        },
    }
    cmd.Flags().BoolVar(&dirty, "dirty", false, "show only dirty drafts")
    cmd.Flags().BoolVar(&conflicted, "conflicted", false, "show only conflicted drafts")
    cmd.Flags().StringVar(&dateFilter, "date", "", "filter by week-start date (YYYY-MM-DD)")
    cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
    cmd.Flags().BoolVar(&noRemote, "no-remote-check", false, "skip remote fingerprint probe (faster)")
    return cmd
}

func renderList(w io.Writer, items []weekDraftListItem, jsonOut bool) error {
    if jsonOut {
        return json.NewEncoder(w).Encode(weekDraftListResp{
            Schema: "tdx.v1.weekDraftList", Drafts: items,
        })
    }
    if len(items) == 0 {
        fmt.Fprintln(w, "No drafts found.")
        return nil
    }
    fmt.Fprintf(w, "%-12s  %-10s  %-12s  %5s  %s\n",
        "WEEK", "NAME", "STATE", "HOURS", "PULLED")
    for _, it := range items {
        fmt.Fprintf(w, "%-12s  %-10s  %-12s  %5.1f  %s\n",
            it.WeekStart, it.Name, it.SyncState, it.TotalHours, it.PulledAt)
    }
    return nil
}
```

> Add `Drafts.ProbeRemoteFingerprint(ctx, profile, weekStart) string` to `draftsvc.Service` — it calls `tsvc.GetWeekReport`, computes the fingerprint, and swallows errors (returns "" on failure). Tests mock this through the `runtime` injection point.

- [ ] **Step 13.4 — Run, verify pass**

```bash
go test ./internal/cli/time/week/ -v
```

- [ ] **Step 13.5 — Commit**

```bash
git add internal/cli/time/week/list.go internal/cli/time/week/list_test.go internal/svc/draftsvc/
git commit -m "feat(cli): tdx time week list"
```

---

## Task 14: `tdx time week show <date> --draft [name]`

Extends existing `tdx time week show` to render a draft instead of the live week. Adds a banner to the no-`--draft` path when a `default` draft exists.

**Files:**
- Modify: `internal/cli/time/week/show.go` (existing — add `--draft` flag)
- Modify: `internal/cli/time/week/show_test.go`

JSON schema: `tdx.v1.weekDraft`.

- [ ] **Step 14.1 — Failing test**

```go
func TestShow_DraftMode(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    d := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Rows: []domain.DraftRow{{
            ID: "row-01",
            Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123, DisplayName: "P"},
            TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
            Cells: []domain.DraftCell{{Day: time.Monday, Hours: 8.0}},
        }},
    }
    if err := draftsvc.NewStore(paths).Save(d); err != nil { t.Fatal(err) }

    cmd := newShowCmdForTest(paths, "work")
    cmd.SetArgs([]string{"2026-05-04", "--draft"})
    var out bytes.Buffer; cmd.SetOut(&out)
    if err := cmd.Execute(); err != nil { t.Fatal(err) }
    if !strings.Contains(out.String(), "Work") {
        t.Errorf("draft grid did not render: %s", out.String())
    }
}

func TestShow_BannerWhenDraftExists(t *testing.T) {
    // Save a default draft; run `tdx time week show <date>` (no --draft);
    // verify banner mentions "Local draft exists for this week".
}
```

- [ ] **Step 14.2 — Implement**

In `internal/cli/time/week/show.go`, add to the existing cobra command:

```go
var draftFlag string  // used as `--draft [name]`; "" = not provided, "<empty>" = --draft alone, "<name>" = --draft <name>
cmd.Flags().StringVar(&draftFlag, "draft", "", "render the local draft instead of the live week (optional name; default \"default\")")

// Inside RunE, after parsing the date:
if cmd.Flags().Changed("draft") {
    name := draftFlag
    if name == "" { name = "default" }
    draft, err := rt.Drafts.Store().Load(rt.Profile, weekStart, name)
    if err != nil { return err }
    return renderDraftGrid(cmd.OutOrStdout(), draft, annotated, jsonOut)
}

// Existing live-week rendering path:
report, err := rt.Time.GetWeekReport(ctx, rt.Profile, weekStart)
if err != nil { return err }
// Add the banner:
if rt.Drafts.Store().Exists(rt.Profile, weekStart, "default") {
    pulled, _ := rt.Drafts.PulledCellsByKey(rt.Profile, weekStart, "default")
    if d, err := rt.Drafts.Store().Load(rt.Profile, weekStart, "default"); err == nil {
        state := domain.ComputeSyncState(d, pulled, "")
        fmt.Fprintf(cmd.OutOrStdout(),
            "Local draft exists for this week (%s, %d cells changed). View with `tdx time week show %s --draft`.\n\n",
            state.Sync, state.Edited+state.Added+state.Conflict,
            weekStart.Format("2006-01-02"))
    }
}
return renderLiveWeekGrid(cmd.OutOrStdout(), report, jsonOut)
```

`renderDraftGrid` reuses the existing grid renderer in `internal/render/grid.go` — pass cell-state-decorating options so edited cells get `*`, added cells get `+`.

- [ ] **Step 14.3 — Run, verify pass + commit**

```bash
go test ./internal/cli/time/week/ -v
git add internal/cli/time/week/show.go internal/cli/time/week/show_test.go internal/render/
git commit -m "feat(cli): tdx time week show --draft + draft-detected banner"
```

---

## Task 15: `tdx time week status <date>[/<name>]`

**Files:**
- Create: `internal/cli/time/week/status.go`
- Create: `internal/cli/time/week/status_test.go`

JSON schema: `tdx.v1.weekDraftStatus`.

Recommended-action logic:
- `conflicted` → "edit to resolve conflicts (refresh available in Phase B)"
- `dirty` and `stale` → "remote drifted; pull --force (auto-snapshots) before pushing"
- `dirty` only → "tdx time week preview <date>, then push --yes"
- `clean` and `stale` → "tdx time week pull <date> (will adopt remote changes)"
- `clean` only → "no action recommended"

- [ ] **Step 15.1 — Failing tests** (table-driven, cover each recommendation branch)

```go
func TestRecommendedAction(t *testing.T) {
    cases := []struct {
        sync  domain.SyncState
        stale bool
        wantContains string
    }{
        {domain.SyncConflicted, false, "edit to resolve"},
        {domain.SyncDirty, true, "remote drifted"},
        {domain.SyncDirty, false, "preview"},
        {domain.SyncClean, true, "adopt remote"},
        {domain.SyncClean, false, "no action"},
    }
    for _, c := range cases {
        got := recommendedAction(c.sync, c.stale)
        if !strings.Contains(got, c.wantContains) {
            t.Errorf("sync=%s stale=%v: got %q, want contains %q", c.sync, c.stale, got, c.wantContains)
        }
    }
}
```

- [ ] **Step 15.2 — Implement**

```go
// internal/cli/time/week/status.go
package week

import (
    "encoding/json"
    "fmt"
    "io"
    "time"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/domain"
)

type weekDraftStatusResp struct {
    Schema             string                `json:"schema"`
    Profile            string                `json:"profile"`
    WeekStart          string                `json:"weekStart"`
    Name               string                `json:"name"`
    SyncState          string                `json:"syncState"`
    SyncDetail         domain.DraftSyncState `json:"syncDetail"`
    TotalHours         float64               `json:"totalHours"`
    PulledAt           string                `json:"pulledAt,omitempty"`
    PushedAt           string                `json:"pushedAt,omitempty"`
    RecommendedAction  string                `json:"recommendedAction"`
}

func newStatusCmd() *cobra.Command {
    var jsonOut, noRemote bool
    cmd := &cobra.Command{
        Use:   "status <date>[/<name>]",
        Short: "Show one-line draft status",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()

            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }

            d, err := rt.Drafts.Store().Load(rt.Profile, weekStart, name)
            if err != nil { return err }
            pulled, _ := rt.Drafts.PulledCellsByKey(rt.Profile, weekStart, name)
            fingerprint := ""
            if !noRemote {
                fingerprint = rt.Drafts.ProbeRemoteFingerprint(ctx, rt.Profile, weekStart)
            }
            state := domain.ComputeSyncState(d, pulled, fingerprint)
            action := recommendedAction(state.Sync, state.Stale)

            return renderStatus(cmd.OutOrStdout(), d, state, action, jsonOut)
        },
    }
    cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
    cmd.Flags().BoolVar(&noRemote, "no-remote-check", false, "skip remote fingerprint probe")
    return cmd
}

func recommendedAction(sync domain.SyncState, stale bool) string {
    switch {
    case sync == domain.SyncConflicted:
        return "edit to resolve conflicts (refresh available in Phase B)"
    case sync == domain.SyncDirty && stale:
        return "remote drifted since pull; tdx time week pull --force (auto-snapshots) before pushing"
    case sync == domain.SyncDirty:
        return "tdx time week preview <date>, then push --yes"
    case sync == domain.SyncClean && stale:
        return "tdx time week pull <date> (will adopt remote changes)"
    default:
        return "no action recommended"
    }
}

func renderStatus(w io.Writer, d domain.WeekDraft, state domain.DraftSyncState, action string, jsonOut bool) error {
    if jsonOut {
        return json.NewEncoder(w).Encode(weekDraftStatusResp{
            Schema: "tdx.v1.weekDraftStatus", Profile: d.Profile,
            WeekStart: d.WeekStart.Format("2006-01-02"), Name: d.Name,
            SyncState: string(state.Sync), SyncDetail: state, TotalHours: state.TotalHours,
            PulledAt: formatTimeOrEmpty(d.Provenance.PulledAt),
            PushedAt: formatPtrTimeOrEmpty(d.PushedAt),
            RecommendedAction: action,
        })
    }
    fmt.Fprintf(w, "%s / %s\n", d.WeekStart.Format("2006-01-02"), d.Name)
    fmt.Fprintf(w, "  Profile:     %s\n", d.Profile)
    fmt.Fprintf(w, "  Pulled:      %s\n", formatTimeWithAge(d.Provenance.PulledAt))
    fmt.Fprintf(w, "  Pushed:      %s\n", formatPtrTimeWithAge(d.PushedAt))
    syncLabel := string(state.Sync)
    if state.Stale { syncLabel += " (and STALE)" }
    fmt.Fprintf(w, "  Sync state:  %s\n", syncLabel)
    fmt.Fprintf(w, "  Cells:       %d untouched · %d edited · %d added · %d conflict\n",
        state.Untouched, state.Edited, state.Added, state.Conflict)
    fmt.Fprintf(w, "  Total hours: %.1fh\n\n", state.TotalHours)
    fmt.Fprintf(w, "  Action recommended:\n    %s\n", action)
    return nil
}

func formatTimeOrEmpty(t time.Time) string {
    if t.IsZero() { return "" }
    return t.UTC().Format(time.RFC3339)
}
func formatPtrTimeOrEmpty(t *time.Time) string {
    if t == nil { return "" }
    return formatTimeOrEmpty(*t)
}
func formatTimeWithAge(t time.Time) string {
    if t.IsZero() { return "never" }
    return fmt.Sprintf("%s (%s ago)", t.UTC().Format("2006-01-02 15:04:05"),
        time.Since(t).Round(time.Minute))
}
func formatPtrTimeWithAge(t *time.Time) string {
    if t == nil { return "never" }
    return formatTimeWithAge(*t)
}
```

- [ ] **Step 15.3 — Run, verify pass + commit**

```bash
go test ./internal/cli/time/week/ -v
git add internal/cli/time/week/status.go internal/cli/time/week/status_test.go
git commit -m "feat(cli): tdx time week status with recommended-action heuristic"
```

---

## Task 16: `tdx time week diff <date>[/<name>]`

**Files:**
- Create: `internal/cli/time/week/diff.go`
- Create: `internal/cli/time/week/diff_test.go`

MVP scope: `--against remote` only (Phase D adds template/snapshot/draft). JSON schema: `tdx.v1.weekDraftDiff`.

- [ ] **Step 16.1 — Failing test** (cover the diff line shapes)

```go
func TestDiff_Shape(t *testing.T) {
    // Setup: a draft with an edited cell + an added cell + a deleted cell;
    // a mock GetWeekReport returning the original entries.
    // Verify diff JSON contains 3 entries with kinds "update", "add", "delete".
}
```

- [ ] **Step 16.2 — Implement**

```go
// internal/cli/time/week/diff.go
package week

import (
    "encoding/json"
    "fmt"
    "io"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/domain"
)

type weekDraftDiffEntry struct {
    Row      string  `json:"row"`
    Day      string  `json:"day"`
    Kind     string  `json:"kind"`
    Before   float64 `json:"before"`
    After    float64 `json:"after"`
    SourceID int     `json:"sourceID,omitempty"`
}
type weekDraftDiffResp struct {
    Schema  string               `json:"schema"`
    Entries []weekDraftDiffEntry `json:"entries"`
    Summary struct {
        Adds    int `json:"adds"`
        Updates int `json:"updates"`
        Deletes int `json:"deletes"`
        Matches int `json:"matches"`
    } `json:"summary"`
}

func newDiffCmd() *cobra.Command {
    var jsonOut, listOut, gridOut bool
    var against string
    cmd := &cobra.Command{
        Use:   "diff <date>[/<name>]",
        Short: "Diff a draft against current remote",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()
            if against != "" && against != "remote" {
                return fmt.Errorf("--against %q not supported in MVP (Phase D adds template/snapshot/draft)", against)
            }
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }
            d, diff, err := rt.Drafts.Reconcile(ctx, rt.Profile, weekStart, name)
            if err != nil { return err }
            return renderDiff(cmd.OutOrStdout(), d, diff, jsonOut, listOut, gridOut)
        },
    }
    cmd.Flags().StringVar(&against, "against", "remote", "remote (MVP) | template <name> | snapshot N | draft <ref> (Phase D)")
    cmd.Flags().BoolVar(&listOut, "list", true, "list view (default)")
    cmd.Flags().BoolVar(&gridOut, "grid", false, "grid view")
    cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
    return cmd
}

func renderDiff(w io.Writer, d domain.WeekDraft, diff domain.ReconcileDiff, jsonOut, listOut, gridOut bool) error {
    entries := make([]weekDraftDiffEntry, 0, len(diff.Actions))
    var adds, updates, deletes, matches int
    for _, a := range diff.Actions {
        e := weekDraftDiffEntry{Row: a.RowID, Day: a.Date.Weekday().String()}
        switch a.Kind {
        case domain.ActionCreate:
            e.Kind, e.After = "add", float64(a.Entry.Minutes)/60.0
            adds++
        case domain.ActionUpdate:
            e.Kind, e.SourceID = "update", a.ExistingID
            if a.Patch.Minutes != nil { e.After = float64(*a.Patch.Minutes)/60.0 }
            updates++
        case domain.ActionDelete:
            e.Kind, e.SourceID = "delete", a.DeleteEntryID
            deletes++
        case domain.ActionSkip:
            e.Kind = "match"
            matches++
        }
        entries = append(entries, e)
    }
    if jsonOut {
        resp := weekDraftDiffResp{Schema: "tdx.v1.weekDraftDiff", Entries: entries}
        resp.Summary.Adds, resp.Summary.Updates = adds, updates
        resp.Summary.Deletes, resp.Summary.Matches = deletes, matches
        return json.NewEncoder(w).Encode(resp)
    }
    if gridOut {
        // Grid view delegates to the same renderer used by `preview`.
        return renderPreviewGrid(w, d, diff)
    }
    // Default list view.
    for _, e := range entries {
        if e.Kind == "match" { continue }
        fmt.Fprintf(w, "  %-8s  %-3s  %-7s  %.1f -> %.1f", e.Row, e.Day[:3], e.Kind, e.Before, e.After)
        if e.SourceID > 0 { fmt.Fprintf(w, "  (entry #%d)", e.SourceID) }
        fmt.Fprintln(w)
    }
    fmt.Fprintf(w, "\nSummary: %d adds · %d updates · %d deletes · %d matches\n", adds, updates, deletes, matches)
    return nil
}
```

- [ ] **Step 16.3 — Commit**

```bash
git add internal/cli/time/week/diff.go internal/cli/time/week/diff_test.go
git commit -m "feat(cli): tdx time week diff --against remote"
```

---

## Task 17: `tdx time week preview <date>[/<name>]`

**Files:**
- Create: `internal/cli/time/week/preview.go`
- Create: `internal/cli/time/week/preview_test.go`

JSON schema: `tdx.v1.weekDraftPreview`.

- [ ] **Step 17.1 — Failing test**

```go
func TestPreview_OutputContainsDiffHash(t *testing.T) {
    // Setup: draft + mocked report. Run preview --json. Verify response has `diffHash`
    // and `expectedDiffHash` is non-empty.
}
```

- [ ] **Step 17.2 — Implement**

```go
// internal/cli/time/week/preview.go
package week

import (
    "encoding/json"
    "fmt"
    "io"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/domain"
)

type weekDraftPreviewResp struct {
    Schema           string             `json:"schema"`
    Actions          []domain.Action    `json:"actions"`
    Blockers         []domain.Blocker   `json:"blockers"`
    Creates          int                `json:"creates"`
    Updates          int                `json:"updates"`
    Deletes          int                `json:"deletes"`
    Skips            int                `json:"skips"`
    BlockedCount     int                `json:"blockedCount"`
    ExpectedDiffHash string             `json:"expectedDiffHash"`
}

func newPreviewCmd() *cobra.Command {
    var jsonOut bool
    var days, mode string
    cmd := &cobra.Command{
        Use:   "preview <date>[/<name>]",
        Short: "Preview what tdx time week push will do",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }
            _ = days; _ = mode // wire days/mode through to Reconcile when supported in MVP

            d, diff, err := rt.Drafts.Reconcile(ctx, rt.Profile, weekStart, name)
            if err != nil { return err }
            creates, updates, deletes, skips := diff.CountByKindV2()
            return renderPreview(cmd.OutOrStdout(), d, diff, creates, updates, deletes, skips, jsonOut)
        },
    }
    cmd.Flags().StringVar(&days, "days", "", "weekday filter, e.g. mon-fri")
    cmd.Flags().StringVar(&mode, "mode", "add", "add | replace-matching | replace-mine")
    cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
    return cmd
}

func renderPreview(w io.Writer, d domain.WeekDraft, diff domain.ReconcileDiff, creates, updates, deletes, skips int, jsonOut bool) error {
    if jsonOut {
        return json.NewEncoder(w).Encode(weekDraftPreviewResp{
            Schema: "tdx.v1.weekDraftPreview", Actions: diff.Actions, Blockers: diff.Blockers,
            Creates: creates, Updates: updates, Deletes: deletes, Skips: skips,
            BlockedCount: len(diff.Blockers), ExpectedDiffHash: diff.DiffHash,
        })
    }
    if err := renderPreviewGrid(w, d, diff); err != nil { return err }
    fmt.Fprintf(w, "\n  Summary: %d creates · %d updates · %d deletes · %d skips · %d blocked\n",
        creates, updates, deletes, skips, len(diff.Blockers))
    fmt.Fprintf(w, "  Diff hash: %s\n", diff.DiffHash[:16]+"…")
    return nil
}

// renderPreviewGrid uses the existing grid renderer with action symbols.
func renderPreviewGrid(w io.Writer, d domain.WeekDraft, diff domain.ReconcileDiff) error {
    // Implementation: build a per-cell action map keyed by (rowID, weekday); pass
    // it to internal/render/grid.go's renderer with a symbols map:
    //   ActionCreate -> "+"  ActionUpdate -> "~"  ActionSkip -> "="
    //   ActionDelete -> "−"  blocker -> "x"
    // The renderer prints the grid with these suffixes appended to each cell.
    return renderDraftActionGrid(w, d, diff)
}
```

> Implementer: extract `renderDraftActionGrid(w, d, diff)` into `internal/render/draft_grid.go`, parameterizing today's grid renderer over a "cell decorator" function.

- [ ] **Step 17.3 — Commit**

```bash
git add internal/cli/time/week/preview.go internal/cli/time/week/preview_test.go internal/render/
git commit -m "feat(cli): tdx time week preview with annotated grid"
```

---

## Task 18: `tdx time week push <date>[/<name>] --yes`

**Files:**
- Create: `internal/cli/time/week/push.go`
- Create: `internal/cli/time/week/push_test.go`

JSON schema: `tdx.v1.weekDraftPushResult`.

- [ ] **Step 18.1 — Failing tests**

```go
func TestPush_RefusesWithoutYes(t *testing.T) {
    // Run push without --yes. Expect cobra to print preview and exit non-zero.
}

func TestPush_RefusesDeletesWithoutAllowDeletes(t *testing.T) {
    // Setup draft with cleared pulled cell. Run with --yes but not --allow-deletes.
    // Expect error referencing --allow-deletes.
}

func TestPush_HashMismatch_RecoverableMessage(t *testing.T) {
    // Save draft → call preview → mutate watermark fingerprint behind the scenes
    // (or change the underlying remote mock) → call push with the original hash.
    // Expect hash mismatch error containing "preview" / "refresh" guidance.
}
```

- [ ] **Step 18.2 — Implement**

```go
// internal/cli/time/week/push.go
package week

import (
    "encoding/json"
    "fmt"
    "io"
    "strings"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/domain"
)

type weekDraftPushResp struct {
    Schema  string                    `json:"schema"`
    Created int                       `json:"created"`
    Updated int                       `json:"updated"`
    Deleted int                       `json:"deleted"`
    Skipped int                       `json:"skipped"`
    Failed  []map[string]any          `json:"failed,omitempty"`
}

func newPushCmd() *cobra.Command {
    var yes, allowDeletes, jsonOut bool
    var expectedHash, days, mode string
    cmd := &cobra.Command{
        Use:   "push <date>[/<name>]",
        Short: "Push a draft to TD",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }

            // Always preview first.
            d, diff, err := rt.Drafts.Reconcile(ctx, rt.Profile, weekStart, name)
            if err != nil { return err }
            creates, updates, deletes, skips := diff.CountByKindV2()

            if !yes {
                // Behave identically to preview when --yes is not set.
                return renderPreview(cmd.OutOrStdout(), d, diff, creates, updates, deletes, skips, jsonOut)
            }
            if deletes > 0 && !allowDeletes {
                return fmt.Errorf("draft contains %d delete actions; pass --allow-deletes to confirm", deletes)
            }

            hash := diff.DiffHash
            if expectedHash != "" { hash = expectedHash }

            user, err := rt.Auth.WhoAmI(ctx, rt.Profile)
            if err != nil { return err }

            res, err := rt.Drafts.Apply(ctx, rt.Profile, weekStart, name, hash, allowDeletes, user.UID)
            if err != nil {
                if strings.Contains(err.Error(), "hash mismatch") {
                    return fmt.Errorf(
                        "push aborted: remote week changed since preview\n  Re-run: tdx time week preview %s\n  Or:    tdx time week pull --force %s (auto-snapshots)",
                        weekStart.Format("2006-01-02"), weekStart.Format("2006-01-02"))
                }
                return err
            }
            return renderPushResult(cmd.OutOrStdout(), res, jsonOut)
        },
    }
    cmd.Flags().BoolVar(&yes, "yes", false, "execute the push (otherwise behaves as preview)")
    cmd.Flags().BoolVar(&allowDeletes, "allow-deletes", false, "required if any delete actions in preview")
    cmd.Flags().StringVar(&expectedHash, "expected-diff-hash", "", "use a specific diff hash (advanced; default re-computes from preview)")
    cmd.Flags().StringVar(&days, "days", "", "weekday filter")
    cmd.Flags().StringVar(&mode, "mode", "add", "add | replace-matching | replace-mine")
    cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
    return cmd
}

func renderPushResult(w io.Writer, res draftsvc.ApplyResult, jsonOut bool) error {
    if jsonOut {
        failed := make([]map[string]any, 0, len(res.Failed))
        for _, f := range res.Failed {
            failed = append(failed, map[string]any{
                "kind": f.Kind, "row": f.RowID, "date": f.Date,
                "entryID": f.EntryID, "message": f.Message,
            })
        }
        return json.NewEncoder(w).Encode(weekDraftPushResp{
            Schema: "tdx.v1.weekDraftPushResult",
            Created: res.Created, Updated: res.Updated, Deleted: res.Deleted, Skipped: res.Skipped,
            Failed: failed,
        })
    }
    fmt.Fprintf(w, "Push complete: %d created · %d updated · %d deleted · %d skipped\n",
        res.Created, res.Updated, res.Deleted, res.Skipped)
    if len(res.Failed) > 0 {
        fmt.Fprintf(w, "\nFailures (%d):\n", len(res.Failed))
        for _, f := range res.Failed {
            fmt.Fprintf(w, "  %s %s/%s: %s (entry #%d)\n", f.Kind, f.RowID, f.Date, f.Message, f.EntryID)
        }
    }
    return nil
}
```

- [ ] **Step 18.3 — Commit**

```bash
git add internal/cli/time/week/push.go internal/cli/time/week/push_test.go
git commit -m "feat(cli): tdx time week push with --allow-deletes safety gate"
```

---

## Task 19: `tdx time week delete <date>[/<name>] --yes`

**Files:**
- Create: `internal/cli/time/week/delete.go`
- Create: `internal/cli/time/week/delete_test.go`

- [ ] **Step 19.1 — Failing tests**

```go
func TestDelete_AutoSnapshotsBeforeRemove(t *testing.T) {
    // Save a draft. Call delete --yes. Verify snapshot file exists with op="pre-delete".
    // Verify draft file is removed.
}

func TestDelete_RequiresYes(t *testing.T) {
    // Run without --yes. Expect cobra error mentioning --yes.
}

func TestDelete_KeepSnapshotsRetainsDir(t *testing.T) {
    // Run with --keep-snapshots. Verify snapshots/ dir + files remain.
}
```

- [ ] **Step 19.2 — Implement**

```go
// internal/cli/time/week/delete.go
package week

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
)

func newDeleteCmd() *cobra.Command {
    var yes, keepSnapshots bool
    cmd := &cobra.Command{
        Use:   "delete <date>[/<name>]",
        Short: "Delete a local draft",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }
            if !yes {
                return fmt.Errorf("pass --yes to delete the draft (auto-snapshots first)")
            }
            d, err := rt.Drafts.Store().Load(rt.Profile, weekStart, name)
            if err != nil { return err }
            if _, err := rt.Drafts.Snapshots().Take(d, draftsvc.OpPreDelete, ""); err != nil {
                return fmt.Errorf("auto-snapshot pre-delete: %w", err)
            }
            if err := rt.Drafts.Store().Delete(rt.Profile, weekStart, name); err != nil {
                return err
            }
            // Remove pulled-snapshot sibling.
            os.Remove(filepath.Join(rt.Paths.ProfileWeeksDir(rt.Profile),
                weekStart.Format("2006-01-02"), name+".pulled.yaml"))
            if !keepSnapshots {
                // For MVP: keep snapshots dir intact even when --keep-snapshots not set;
                // explicit purge requires Phase B's `prune --all` command.
                _ = keepSnapshots
            }
            fmt.Fprintf(cmd.OutOrStdout(), "Deleted draft %s/%s.\n",
                weekStart.Format("2006-01-02"), name)
            return nil
        },
    }
    cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
    cmd.Flags().BoolVar(&keepSnapshots, "keep-snapshots", true, "keep snapshot history (default true)")
    return cmd
}
```

- [ ] **Step 19.3 — Commit**

```bash
git add internal/cli/time/week/delete.go internal/cli/time/week/delete_test.go
git commit -m "feat(cli): tdx time week delete with auto-snapshot"
```

---

## Task 20: `tdx time week set <date>[/<name>] <row>:<day>=<hours>` (SHOULD-tier)

**Files:**
- Create: `internal/cli/time/week/set.go`
- Create: `internal/cli/time/week/set_test.go`

Repeatable positional cell-write tokens, parsed via the same syntax as today's `--override` flag (existing parser at `internal/mcp/tools_apply.go:parseOverrides` is the reference; extract it into `internal/domain/cellref.go` if useful for reuse).

- [ ] **Step 20.1 — Failing tests**

```go
func TestSet_UpdatesMultipleCells(t *testing.T) {
    // Save draft with one row, no cells.
    // Run: tdx time week set 2026-05-04 row-01:mon=8 row-01:fri=4
    // Verify draft now has Mon=8, Fri=4 cells, marked added.
}

func TestSet_ParsesValidationErrors(t *testing.T) {
    // Run with invalid syntax "row-01:invalid=8". Expect helpful error.
}
```

- [ ] **Step 20.2 — Implement**

```go
// internal/cli/time/week/set.go
package week

import (
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/domain"
)

var dayNames = map[string]time.Weekday{
    "sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
    "wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

type cellWrite struct {
    RowID string
    Day   time.Weekday
    Hours float64
}

func parseCellWrite(s string) (cellWrite, error) {
    colon := strings.Index(s, ":")
    if colon < 0 { return cellWrite{}, fmt.Errorf("expected row:day=hours, got %q", s) }
    eq := strings.Index(s, "=")
    if eq < 0 || eq < colon { return cellWrite{}, fmt.Errorf("expected row:day=hours, got %q", s) }
    rowID, dayStr, hoursStr := s[:colon], s[colon+1:eq], s[eq+1:]
    day, ok := dayNames[strings.ToLower(strings.TrimSpace(dayStr))]
    if !ok { return cellWrite{}, fmt.Errorf("unknown day %q", dayStr) }
    h, err := strconv.ParseFloat(strings.TrimSpace(hoursStr), 64)
    if err != nil { return cellWrite{}, fmt.Errorf("invalid hours %q: %w", hoursStr, err) }
    return cellWrite{RowID: strings.TrimSpace(rowID), Day: day, Hours: h}, nil
}

func newSetCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "set <date>[/<name>] <row>:<day>=<hours> ...",
        Short: "Non-interactive cell write (repeatable)",
        Args:  cobra.MinimumNArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }

            d, err := rt.Drafts.Store().Load(rt.Profile, weekStart, name)
            if err != nil { return err }

            for _, tok := range args[1:] {
                w, err := parseCellWrite(tok)
                if err != nil { return err }
                applyCellWrite(&d, w)
            }
            d.ModifiedAt = time.Now().UTC()
            return rt.Drafts.Store().Save(d)
        },
    }
    return cmd
}

// applyCellWrite mutates d to set the named row's day cell to hours.
// Adds a new cell if absent. Adds a row only if explicitly tracked elsewhere
// (Phase B will add a `tdx time week add-row` command for that).
func applyCellWrite(d *domain.WeekDraft, w cellWrite) {
    for ri := range d.Rows {
        if d.Rows[ri].ID != w.RowID { continue }
        for ci := range d.Rows[ri].Cells {
            if d.Rows[ri].Cells[ci].Day == w.Day {
                d.Rows[ri].Cells[ci].Hours = w.Hours
                return
            }
        }
        d.Rows[ri].Cells = append(d.Rows[ri].Cells, domain.DraftCell{
            Day: w.Day, Hours: w.Hours,
        })
        return
    }
}
```

- [ ] **Step 20.3 — Commit**

```bash
git add internal/cli/time/week/set.go internal/cli/time/week/set_test.go
git commit -m "feat(cli): tdx time week set (non-interactive cell write)"
```

---

## Task 21: `tdx time week note <date>[/<name>]` (SHOULD-tier)

**Files:**
- Create: `internal/cli/time/week/note.go`
- Create: `internal/cli/time/week/note_test.go`

- [ ] **Step 21.1 — Failing tests**

```go
func TestNote_Append(t *testing.T) {
    // Save draft with notes "first.\n". Run: tdx time week note ... --append "second."
    // Verify final notes contains both lines.
}

func TestNote_Clear(t *testing.T) {
    // Save draft with notes. Run with --clear. Verify notes is "".
}
```

- [ ] **Step 21.2 — Implement**

```go
// internal/cli/time/week/note.go
package week

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
)

func newNoteCmd() *cobra.Command {
    var appendText string
    var clear bool
    cmd := &cobra.Command{
        Use:   "note <date>[/<name>]",
        Short: "Edit free-form notes on a draft",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }
            d, err := rt.Drafts.Store().Load(rt.Profile, weekStart, name)
            if err != nil { return err }

            switch {
            case clear:
                d.Notes = ""
            case appendText != "":
                if d.Notes != "" && !strings.HasSuffix(d.Notes, "\n") { d.Notes += "\n" }
                d.Notes += appendText + "\n"
            default:
                edited, err := openEditor(d.Notes)
                if err != nil { return err }
                d.Notes = edited
            }
            d.ModifiedAt = time.Now().UTC()
            return rt.Drafts.Store().Save(d)
        },
    }
    cmd.Flags().StringVar(&appendText, "append", "", "append text without invoking $EDITOR")
    cmd.Flags().BoolVar(&clear, "clear", false, "clear notes")
    return cmd
}

func openEditor(initial string) (string, error) {
    editor := os.Getenv("EDITOR")
    if editor == "" { editor = "vi" }
    f, err := os.CreateTemp("", "tdx-note-*.txt")
    if err != nil { return "", err }
    if _, err := f.WriteString(initial); err != nil { f.Close(); os.Remove(f.Name()); return "", err }
    f.Close()
    defer os.Remove(f.Name())
    cmd := exec.Command(editor, f.Name())
    cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
    if err := cmd.Run(); err != nil { return "", fmt.Errorf("editor: %w", err) }
    data, err := os.ReadFile(f.Name())
    if err != nil { return "", err }
    _ = filepath.Clean // unused
    return string(data), nil
}
```

- [ ] **Step 21.3 — Commit**

```bash
git add internal/cli/time/week/note.go internal/cli/time/week/note_test.go
git commit -m "feat(cli): tdx time week note"
```

---

## Task 22: `tdx time week history <date>[/<name>]` (SHOULD-tier, read-only)

**Files:**
- Create: `internal/cli/time/week/history.go`
- Create: `internal/cli/time/week/history_test.go`

JSON schema: `tdx.v1.weekDraftSnapshotList`.

- [ ] **Step 22.1 — Failing test**

```go
func TestHistory_ListsSnapshots(t *testing.T) {
    // Save draft, take 3 snapshots. Run history. Verify all 3 lines + sequence numbers.
}
```

- [ ] **Step 22.2 — Implement**

```go
// internal/cli/time/week/history.go
package week

import (
    "encoding/json"
    "fmt"
    "io"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/cli/runtime"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
)

type weekDraftSnapshotListResp struct {
    Schema    string                  `json:"schema"`
    Snapshots []draftsvc.SnapshotInfo `json:"snapshots"`
}

func newHistoryCmd() *cobra.Command {
    var jsonOut bool
    var limit int
    cmd := &cobra.Command{
        Use:   "history <date>[/<name>]",
        Short: "List snapshots of a draft",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            rt, err := runtime.New(cmd)
            if err != nil { return err }
            defer rt.Close()
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }
            list, err := rt.Drafts.Snapshots().List(rt.Profile, weekStart, name)
            if err != nil { return err }
            if limit > 0 && len(list) > limit { list = list[len(list)-limit:] }
            return renderHistory(cmd.OutOrStdout(), list, jsonOut)
        },
    }
    cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
    cmd.Flags().IntVar(&limit, "limit", 0, "show at most N most recent snapshots (0 = all)")
    return cmd
}

func renderHistory(w io.Writer, list []draftsvc.SnapshotInfo, jsonOut bool) error {
    if jsonOut {
        return json.NewEncoder(w).Encode(weekDraftSnapshotListResp{
            Schema: "tdx.v1.weekDraftSnapshotList", Snapshots: list,
        })
    }
    if len(list) == 0 { fmt.Fprintln(w, "No snapshots."); return nil }
    fmt.Fprintf(w, "%-4s  %-12s  %-20s  %-6s  %s\n", "SEQ", "OP", "TAKEN", "PINNED", "NOTE")
    for _, s := range list {
        pin := ""
        if s.Pinned { pin = "yes" }
        fmt.Fprintf(w, "%-4d  %-12s  %-20s  %-6s  %s\n",
            s.Sequence, s.Op, s.Taken.UTC().Format("2006-01-02 15:04:05"), pin, s.Note)
    }
    return nil
}
```

- [ ] **Step 22.3 — Commit**

```bash
git add internal/cli/time/week/history.go internal/cli/time/week/history_test.go
git commit -m "feat(cli): tdx time week history (read-only)"
```

---

## Task 23: Editor extension for draft mode

Extend `internal/tui/editor` to support a draft model alongside the existing template model. The grid widget itself is reused; new behavior:

- **Status bar** at top: `<date>/<name> · <syncState> · <N cells edited> · last pull <Xh ago>`.
- **Cell annotations** in the grid: yellow background or `*` suffix for edited, green or `+` for added, red border for invalid. (`untouched` plain.)
- **Pre-save confirm**: when `Ctrl-S` is pressed and any cleared pulled cells exist (`hours=0 && SourceEntryID > 0`), show a modal: "N cleared cells will be marked for deletion at push time: [entry IDs]. Proceed? [Y/n]". Default Y; Esc cancels save.
- **Validation warnings**: rows with invalid type/target combos shown in gutter with a warning glyph (does not block save).

**Files:**
- Modify: `internal/tui/editor/editor.go` (model layer — accept either `domain.Template` or `domain.WeekDraft` via interface)
- Modify: `internal/tui/editor/view.go` (status bar + annotations + pre-save modal)
- Create: `internal/tui/editor/draft.go` (draft-specific model methods)
- Create: `internal/tui/editor/draft_test.go`
- Modify: `internal/cli/time/week/edit.go` (entry point that hands a draft to the editor and handles save-back)

- [ ] **Step 23.1 — Test the model layer**

Write tests for the draft adapter: navigation between cells, edit operations (set hours, clear), Ctrl-S behavior with and without cleared pulled cells, Esc behavior with unsaved changes.

- [ ] **Step 23.2 — Implement the model adapter**

Use an interface like:

```go
// internal/tui/editor/model.go (or similar)
type GridModel interface {
    Rows() int
    Cols() int
    CellHours(row, col int) float64
    SetCellHours(row, col int, hours float64)
    CellState(row, col int) string // "untouched"|"edited"|"added"|"invalid"
    StatusLine() string
    HasClearedPulledCells() (count int, entryIDs []int)
    Save() error
    IsDirty() bool
}
```

Implement adapters for both `Template` and `WeekDraft`. The `WeekDraft` adapter wraps a draft + the at-pull-time map and exposes the cell state computation.

- [ ] **Step 23.3 — Wire `tdx time week edit`**

```go
// internal/cli/time/week/edit.go — outline
func newEditCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "edit <date>[/<name>]",
        Short: "Edit a week draft in the grid editor",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // 1. Load draft + pulled-snapshot.
            // 2. Build draft adapter.
            // 3. Run editor TUI (existing bubbletea program, parameterized by GridModel).
            // 4. On Save: persist draft via Drafts.Store().Save().
            // 5. On cancel: discard.
            return nil
        },
    }
    return cmd
}
```

- [ ] **Step 23.4 — Tests + commit**

```bash
go test ./internal/tui/editor/... -v
git add internal/tui/editor/ internal/cli/time/week/edit.go
git commit -m "feat(editor): draft mode with status bar, cell-state annotations, pre-save delete confirm"
```

---

## Task 24: MCP — read tools (`list_week_drafts`, `get_week_draft`, `preview_push_week_draft`, `diff_week_draft`)

Mirror existing `tools_apply.go` / `tools_tmpl.go` patterns. Read tools have no `confirm` field. JSON outputs use the same envelopes as the CLI.

**Files:**
- Create: `internal/mcp/tools_drafts.go`
- Create: `internal/mcp/tools_drafts_test.go`
- Modify: `internal/mcp/server.go` (register tools)

- [ ] **Step 24.1 — Tests for read tools**

(Mirror `internal/mcp/tools_week_test.go` patterns: in-memory service stub, request → response check.)

- [ ] **Step 24.2 — Implement**

```go
// internal/mcp/tools_drafts.go (read-only handlers shown; mutating in Task 25)
package mcp

import (
    "context"
    "fmt"
    "time"

    sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

    "github.com/iainmoffat/tdx/internal/domain"
)

type listDraftsArgs struct {
    Profile      string `json:"profile,omitempty"`
    Dirty        bool   `json:"dirty,omitempty"`
    Conflicted   bool   `json:"conflicted,omitempty"`
    WeekStart    string `json:"weekStart,omitempty"`
}

type getDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart" jsonschema:"YYYY-MM-DD any day in target week"`
    Name      string `json:"name,omitempty"`
}

type previewDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart"`
    Name      string `json:"name,omitempty"`
    Mode      string `json:"mode,omitempty"`
    Days      string `json:"days,omitempty"`
}

type diffDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart"`
    Name      string `json:"name,omitempty"`
}

func RegisterDraftTools(srv *sdkmcp.Server, svcs Services) {
    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name:        "list_week_drafts",
        Description: "List all local week drafts with sync state. Read-only.",
    }, listDraftsHandler(svcs))

    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name:        "get_week_draft",
        Description: "Load a single draft. Returns full content plus sync state and remote fingerprint. Read-only.",
    }, getDraftHandler(svcs))

    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name:        "preview_push_week_draft",
        Description: "Preview what push_week_draft will do. Returns actions, blockers, and diffHash. Always call this before push_week_draft.",
    }, previewDraftHandler(svcs))

    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name:        "diff_week_draft",
        Description: "Diff a draft vs current remote week. Cell-level. Read-only.",
    }, diffDraftHandler(svcs))

    // Mutating handlers registered by Task 25:
    RegisterDraftMutatingTools(srv, svcs)
}

// (handler functions follow today's pattern — see tools_apply.go for reference)
```

- [ ] **Step 24.3 — Register in `server.go`**

```go
// internal/mcp/server.go
RegisterDraftTools(srv, svcs)
```

- [ ] **Step 24.4 — Run tests + commit**

```bash
go test ./internal/mcp/... -v
git add internal/mcp/
git commit -m "feat(mcp): read tools for week drafts (list/get/preview/diff)"
```

---

## Task 25: MCP — mutating tools (`pull`, `update`, `delete`, `push`)

All require `confirm: true`. `push` also requires `expectedDiffHash` and `allowDeletes`.

**Files:**
- Modify: `internal/mcp/tools_drafts.go`
- Modify: `internal/mcp/tools_drafts_test.go`

- [ ] **Step 25.1 — Tests for `confirm:false` rejection on each mutating tool**

```go
func TestPullDraft_ConfirmGate(t *testing.T) {
    // Build args without Confirm=true; expect error result.
}
```

(Mirror the pattern in `internal/mcp/tools_entry_test.go` for confirm-gate testing.)

- [ ] **Step 25.2 — Implement handlers**

Tool args:

```go
type pullDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart"`
    Name      string `json:"name,omitempty"`
    Force     bool   `json:"force,omitempty"`
    Confirm   bool   `json:"confirm" jsonschema:"must be true to execute"`
}

type updateDraftArgs struct {
    Profile             string  `json:"profile,omitempty"`
    WeekStart           string  `json:"weekStart"`
    Name                string  `json:"name,omitempty"`
    Edits               []Edit  `json:"edits"`
    ExpectedModifiedAt  string  `json:"expectedModifiedAt,omitempty"`
    Confirm             bool    `json:"confirm"`
}

type Edit struct {
    RowID       string  `json:"rowID"`
    Day         string  `json:"day"`              // "mon", "tue", ...
    Hours       float64 `json:"hours"`            // 0 + sourceEntryID set = delete-on-push
    Description string  `json:"description,omitempty"`
}

type deleteDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart"`
    Name      string `json:"name,omitempty"`
    Confirm   bool   `json:"confirm"`
}

type pushDraftArgs struct {
    Profile          string `json:"profile,omitempty"`
    WeekStart        string `json:"weekStart"`
    Name             string `json:"name,omitempty"`
    ExpectedDiffHash string `json:"expectedDiffHash" jsonschema:"hash from preview_push_week_draft"`
    AllowDeletes     bool   `json:"allowDeletes,omitempty" jsonschema:"required if any delete actions in preview"`
    Mode             string `json:"mode,omitempty"`
    Days             string `json:"days,omitempty"`
    Confirm          bool   `json:"confirm"`
}

func RegisterDraftMutatingTools(srv *sdkmcp.Server, svcs Services) {
    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name: "pull_week_draft",
        Description: "Pull a live TD week into a local draft. Requires confirm=true. Refuses to overwrite a dirty draft unless force=true (auto-snapshots first).",
    }, pullDraftHandler(svcs))

    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name: "update_week_draft",
        Description: `Apply per-cell edits to a draft. Requires confirm=true.

To delete a pulled entry: set hours: 0 on a cell with sourceEntryID. The cell becomes a delete-on-push.
To add a new cell: include a new {rowID, day, hours} entry; the row's existing target/type/billable apply.

For multi-turn edit sessions: cache modifiedAt from get_week_draft and pass it as expectedModifiedAt to detect concurrent edits.`,
    }, updateDraftHandler(svcs))

    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name: "delete_week_draft",
        Description: "Delete a local draft. Auto-snapshots first. Requires confirm=true.",
    }, deleteDraftHandler(svcs))

    sdkmcp.AddTool(srv, &sdkmcp.Tool{
        Name: "push_week_draft",
        Description: `Push a draft to TD. Requires confirm=true and expectedDiffHash from preview_push_week_draft.

Recipe:
  1. preview_push_week_draft → capture diffHash and check for delete actions.
  2. If any deletes: set allowDeletes=true and confirm with the user.
  3. push_week_draft → on hash mismatch, do NOT retry; call diff_week_draft and re-preview.`,
    }, pushDraftHandler(svcs))
}

// (handlers — implementation mirrors tools_apply.applyTemplateHandler closely)
```

- [ ] **Step 25.3 — Run tests + commit**

```bash
go test ./internal/mcp/... -v
git add internal/mcp/
git commit -m "feat(mcp): mutating tools for week drafts (pull/update/delete/push)"
```

---

## Task 26: README + guide.md updates

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`

- [ ] **Step 26.1 — README updates**

Add a new "Time Week Drafts" command table to `README.md` (after the existing "Time Week" table). Mirror the existing terse style:

```markdown
### Time Week Drafts

| Command | Description | Key Flags |
|---|---|---|
| `tdx time week pull <date>` | Pull live week into a local draft | `--name`, `--force` |
| `tdx time week list` | List local drafts | `--dirty`, `--conflicted`, `--json` |
| `tdx time week show <date> --draft [name]` | Show a draft as a grid | `--annotated`, `--json` |
| `tdx time week status <date>[/<name>]` | One-line draft status | `--json` |
| `tdx time week edit <date>[/<name>]` | Edit a draft in the grid editor | (TUI; `--web` in Phase C) |
| `tdx time week diff <date>[/<name>]` | Diff a draft vs remote | `--list`, `--grid`, `--json` |
| `tdx time week preview <date>[/<name>]` | Preview what `push` will do | `--days`, `--mode`, `--json` |
| `tdx time week push <date>[/<name>] --yes` | Push a draft to TD | `--allow-deletes`, `--days`, `--mode` |
| `tdx time week delete <date>[/<name>] --yes` | Delete a draft (auto-snapshots first) | `--keep-snapshots` |
| `tdx time week set <date>[/<name>] <row>:<day>=<h>` | Non-interactive cell write | repeatable |
| `tdx time week note <date>[/<name>]` | Edit free-form notes | `--append`, `--clear` |
| `tdx time week history <date>[/<name>]` | List snapshots | `--json` |
```

Update the MCP section: add the 7 new tools (`list_week_drafts`, `get_week_draft`, `preview_push_week_draft`, `diff_week_draft`, `pull_week_draft`, `update_week_draft`, `delete_week_draft`, `push_week_draft`) to the read-only / mutating tables. Update the JSON schema list with `tdx.v1.weekDraft*` entries.

Add a one-line note in the configuration section: "Templates and week drafts are stored per-profile under `~/.config/tdx/profiles/<profile>/`."

- [ ] **Step 26.2 — guide.md: new "Week drafts" section**

Add a new top-level section between "Templates" and "MCP Server" with:

1. **Concepts** — what a draft is, draft vs template wall, sync state (clean/dirty/stale/conflicted).
2. **Lifecycle diagram** — copy from spec §5.1.
3. **Editor cheatsheet** — same key list as the template editor section, plus the pre-save delete-confirm behavior.
4. **Push safety contract** — three-bullet summary: `--yes` required, hash protection, `--allow-deletes` for any delete actions.
5. **Worked examples** — three examples (mid-week correction, snapshot live before edits, partial-week push) using the §9 commands.
6. **Storage layout** — new subsection at the bottom describing per-profile paths and the templates migration.

- [ ] **Step 26.3 — Run docs lint / link check**

```bash
go run github.com/iainmoffat/tdx/cmd/tdx --help > /dev/null  # smoke test help text
```

- [ ] **Step 26.4 — Commit**

```bash
git add README.md docs/guide.md
git commit -m "docs: week drafts user-facing documentation"
```

---

## Task 27: Manual walkthrough doc

Create `docs/manual-tests/phase-A-week-drafts-walkthrough.md` mirroring the format of the existing `phase-1-auth-walkthrough.md` and `phase-2-read-ops-walkthrough.md`.

**Files:**
- Create: `docs/manual-tests/phase-A-week-drafts-walkthrough.md`

- [ ] **Step 27.1 — Write the walkthrough**

Outline:

1. **Setup** — `tdx auth status`, confirm authenticated to UFL tenant.
2. **Templates migration** — the user upgrades to the new tdx version; confirm migration prompt fires (or auto-yes if single profile); verify templates moved to per-profile dir.
3. **Pull a recent week** — `tdx time week pull 2026-04-26` against real data; verify draft file exists at `~/.config/tdx/profiles/<active>/weeks/2026-04-26/default.yaml`; verify pulled-snapshot sibling.
4. **Show / status** — verify grid output, status banner, sync state shows `clean`.
5. **Edit a cell** — `tdx time week edit 2026-04-26`; tweak Tuesday's hours; save; verify dirty state in `status`.
6. **Clear a cell to delete** — clear an entry in the editor; verify pre-save confirm names the entry ID.
7. **Diff and preview** — `tdx time week diff 2026-04-26`; `tdx time week preview 2026-04-26`; verify update + delete actions and a diff hash.
8. **Push without `--allow-deletes`** — verify refusal message.
9. **Push with `--allow-deletes`** — verify successful push with operation summary.
10. **Verify in TD web UI** — confirm the entries match.
11. **Auto-snapshot recovery test** — manually corrupt the draft (or delete + re-pull), confirm a `pre-pull` snapshot exists in the snapshots dir, and that you could restore from it.
12. **Delete the draft** — `tdx time week delete 2026-04-26 --yes`; verify draft file gone, snapshots retained.
13. **MCP smoke test** — run `tdx mcp serve` and exercise `list_week_drafts`, `get_week_draft`, `preview_push_week_draft` via `mcp-cli` or equivalent.

- [ ] **Step 27.2 — Commit**

```bash
git add docs/manual-tests/phase-A-week-drafts-walkthrough.md
git commit -m "docs(manual-tests): phase A week drafts walkthrough"
```

---

## Task 28: Final verification + version bump

- [ ] **Step 28.1 — Full test suite**

```bash
go test ./... -v
```
Expected: all packages PASS.

- [ ] **Step 28.2 — Lint clean**

```bash
golangci-lint run ./...
```
Expected: no findings.

- [ ] **Step 28.3 — Vet clean**

```bash
go vet ./...
```

- [ ] **Step 28.4 — Manual walkthrough on real tenant**

Execute every step of `docs/manual-tests/phase-A-week-drafts-walkthrough.md` against the live UFL tenant. Sign off in the doc.

- [ ] **Step 28.5 — Version bump**

Bump version in `internal/cli/version.go` (or wherever the version constant lives).

- [ ] **Step 28.6 — Final commit**

```bash
git add -u
git commit -m "release: tdx vX.Y.Z — Phase A week drafts MVP"
```

- [ ] **Step 28.7 — Open PR**

```bash
git push -u origin phase-A-week-drafts
gh pr create --title "Phase A — Week Drafts MVP" --body "$(cat <<'EOF'
## Summary

Implements the Week Drafts MVP: pull/list/show/status/edit/diff/preview/push/delete for
local week artifacts, with auto-snapshots, hash-protected deletes, 7 MCP tools, and a
templates-per-profile storage migration.

Spec: docs/specs/2026-04-27-tdx-week-drafts-design.md

## Test plan

- [ ] go test ./... passes
- [ ] golangci-lint run ./... clean
- [ ] Manual walkthrough at docs/manual-tests/phase-A-week-drafts-walkthrough.md completed against UFL tenant
EOF
)"
```

---

## Self-review notes

This plan covers spec §14.1 (MVP scope) + §15.A docs deliverables + §16.7 backlog, compressed from 37 backlog items to 28 tasks. Adjacencies folded:
- Backlog #1 → Task 1; #2 → Task 3+4; #3 → Task 6; #4 → Task 7; #5 → Task 2.
- Backlog #6+#7+#8+#9 → Task 9 (plus #10 auto-snapshots layered into Tasks 8/10).
- Backlog #11–#22 → Tasks 11–22 (1:1 with #11 absorbed into Task 11 scaffolding).
- Backlog #23–#29 → Tasks 24–25.
- Backlog #30–#34 → Task 26+27.
- Backlog #35–#37 → Task 28.

**Open questions inherited from spec §15.A (resolve during implementation):**
- Auto-snapshot retention default: codified as `10` in `NewSnapshotStore`. Make configurable in a follow-up if needed.
- Templates-per-profile prompt auto-yes when only one profile exists: implemented in Task 2.
- JSON schema name: chosen `tdx.v1.weekDraft*` (not `tdx.v1.timeWeekDraft*`) for brevity.

**Risks captured in spec §15.A — mitigations baked into plan:**
- Reconcile-engine extension: Task 5 + Task 9 have explicit failing-test-first coverage of `ActionDelete` × all blocker types.
- Templates migration UX: Task 2 splits auto-yes (single profile) from prompt (multi); declines leave the legacy dir intact.
- Editor's "cleared cell deletes" UX: Task 23 includes the pre-save confirm modal as a required step.
