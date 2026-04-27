# Phase B.1 — Week Drafts Alternates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> Each task follows strict TDD: failing test → verify failure → implement → verify pass → commit. Never amend commits — always create new ones. Branch: `phase-B1-week-drafts` (off `main`).
>
> Do NOT run `go mod tidy` — Phase B.1 adds zero new dependencies.
>
> No `Co-Authored-By` trailer on commit messages.

**Design spec:** `docs/specs/2026-04-27-tdx-phase-B1-design.md`
**Builds on:** Phase A (v0.4.0)

**Goal:** Make alternate-named drafts a reality, ship the acquisition verbs (`new`, `copy`, `rename`, `reset`), and surface the snapshot-management UX (`snapshot --keep`, `restore`, `prune`). Add soft-archive via `archived: true`. Zero new domain logic — pure CLI/UX surface on top of Phase A's foundation.

**Architecture**

```
CLI layer (internal/cli/time/week/)
  |-- new, copy, rename, reset, archive, unarchive
  |-- snapshot, restore, prune  (wraps Phase A's existing SnapshotStore)
  |-- list (extend), history (extend)
  v
Service layer (internal/svc/draftsvc/)
  |-- Copy, Rename, Reset, SetArchived  (new methods on Service)
  |-- Store: AlreadyExists check, archived-filter in List
  v
Domain (internal/domain/)
  |-- WeekDraft.Archived bool        <-- the only domain delta
```

MCP: 8 new mutating tools + 1 new read-only tool + 1 filter extension on `list_week_drafts`. Tool count: 27 → 36.

**Tech Stack:** Go 1.24, cobra, gopkg.in/yaml.v3, modelcontextprotocol/go-sdk. No new deps.

---

## Task 1: Add `Archived` field to WeekDraft

**Files:**
- Modify: `internal/domain/draft.go`
- Modify: `internal/domain/draft_test.go`

- [ ] **Step 1.1 — Failing test for Archived round-trip + zero-value omission**

Append to `internal/domain/draft_test.go`:

```go
func TestWeekDraft_ArchivedField_RoundTrip(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, EasternTZ)
    in := WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Archived: true,
    }
    data, err := yaml.Marshal(in)
    if err != nil { t.Fatal(err) }
    if !strings.Contains(string(data), "archived: true") {
        t.Errorf("yaml should contain 'archived: true', got: %s", data)
    }

    var out WeekDraft
    if err := yaml.Unmarshal(data, &out); err != nil { t.Fatal(err) }
    if !out.Archived { t.Errorf("Archived = false after round-trip, want true") }
}

func TestWeekDraft_ArchivedField_OmittedWhenFalse(t *testing.T) {
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, EasternTZ)
    in := WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Archived: false,
    }
    data, err := yaml.Marshal(in)
    if err != nil { t.Fatal(err) }
    if strings.Contains(string(data), "archived:") {
        t.Errorf("zero Archived should be omitted by omitempty, got: %s", data)
    }
}
```

Add `"strings"` import to the test file if not present.

- [ ] **Step 1.2 — Run, verify failure**

```bash
go test ./internal/domain/ -run TestWeekDraft_Archived -v
```
Expected: FAIL — `Archived` field undefined.

- [ ] **Step 1.3 — Add the field**

Edit `internal/domain/draft.go`. Locate the `WeekDraft` struct and add the field at the end (after `Rows`):

```go
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
    // Archived hides the draft from default `list` output. Soft-archive: no
    // file motion, fully reversible via `unarchive`.
    Archived bool `yaml:"archived,omitempty" json:"archived,omitempty"`
}
```

- [ ] **Step 1.4 — Run, verify pass**

```bash
go test ./internal/domain/ -v
```
Expected: PASS (all 70+ domain tests).

- [ ] **Step 1.5 — Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): add Archived field to WeekDraft"
```

---

## Task 2: Verify `pull --name` plumbing works end-to-end

**Files:**
- Create: `internal/svc/draftsvc/alternates_test.go`

This is a smoke test for what Phase A *already supports* — alternate-name pulls. The plumbing is in place; we want a regression-proof test before B.1 layers more on top.

- [ ] **Step 2.1 — Failing test (no service code change, just covering an existing path)**

Create `internal/svc/draftsvc/alternates_test.go`:

```go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestStore_AlternateNamesIsolated(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := NewStore(paths)
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    a := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week, Notes: "primary"}
    b := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "pristine", WeekStart: week, Notes: "alt"}
    if err := s.Save(a); err != nil { t.Fatal(err) }
    if err := s.Save(b); err != nil { t.Fatal(err) }

    if !s.Exists("work", week, "default") { t.Errorf("default missing") }
    if !s.Exists("work", week, "pristine") { t.Errorf("pristine missing") }

    loadedA, err := s.Load("work", week, "default")
    if err != nil { t.Fatal(err) }
    loadedB, err := s.Load("work", week, "pristine")
    if err != nil { t.Fatal(err) }
    if loadedA.Notes == loadedB.Notes {
        t.Errorf("alternates not isolated: both have notes %q", loadedA.Notes)
    }

    list, err := s.List("work")
    if err != nil { t.Fatal(err) }
    if len(list) != 2 { t.Errorf("List returned %d, want 2", len(list)) }
}
```

- [ ] **Step 2.2 — Run, verify pass (existing code already supports this)**

```bash
go test ./internal/svc/draftsvc/ -run TestStore_AlternateNames -v
```
Expected: PASS — Phase A's Store already supports alternate names.

- [ ] **Step 2.3 — Commit**

```bash
git add internal/svc/draftsvc/alternates_test.go
git commit -m "test(draftsvc): regression test for alternate-named drafts"
```

---

## Task 3: `Store.Exists` checks for collision before save (refuse-overwrite helper)

We need a way to refuse creating a draft that already exists at `(profile, weekStart, name)`. `Save` today overwrites silently. Add a guarded variant.

**Files:**
- Modify: `internal/svc/draftsvc/store.go`
- Modify: `internal/svc/draftsvc/store_test.go`

- [ ] **Step 3.1 — Failing test**

Append to `internal/svc/draftsvc/store_test.go`:

```go
func TestStore_SaveNew_RefusesCollision(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := NewStore(paths)
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
    if err := s.SaveNew(d); err != nil { t.Fatal(err) }
    if err := s.SaveNew(d); err == nil {
        t.Errorf("SaveNew should refuse to overwrite existing draft")
    }
}
```

- [ ] **Step 3.2 — Run, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run TestStore_SaveNew -v
```
Expected: FAIL — `SaveNew` undefined.

- [ ] **Step 3.3 — Implement**

Append to `internal/svc/draftsvc/store.go`:

```go
// SaveNew is like Save but refuses when a draft already exists at the same
// (profile, weekStart, name). Used by `new`, `copy`, and `rename` flows
// that explicitly do not want to clobber.
func (s *Store) SaveNew(d domain.WeekDraft) error {
    if s.Exists(d.Profile, d.WeekStart, d.Name) {
        return fmt.Errorf("draft already exists: %s/%s/%s",
            d.Profile, d.WeekStart.In(domain.EasternTZ).Format("2006-01-02"), d.Name)
    }
    return s.Save(d)
}
```

- [ ] **Step 3.4 — Run, verify pass + commit**

```bash
go test ./internal/svc/draftsvc/ -v
git add internal/svc/draftsvc/
git commit -m "feat(draftsvc): SaveNew refuses to overwrite existing draft"
```

---

## Task 4: `Service.NewBlank` — create an empty dated draft

**Files:**
- Create: `internal/svc/draftsvc/new.go`
- Create: `internal/svc/draftsvc/new_test.go`

- [ ] **Step 4.1 — Failing test**

```go
// internal/svc/draftsvc/new_test.go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestService_NewBlank(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    d, err := s.NewBlank("work", week, "default")
    if err != nil { t.Fatal(err) }
    if d.Profile != "work" || d.Name != "default" || !d.WeekStart.Equal(week) {
        t.Errorf("identity wrong: %+v", d)
    }
    if d.Provenance.Kind != domain.ProvenanceBlank {
        t.Errorf("Provenance.Kind = %s, want blank", d.Provenance.Kind)
    }
    if len(d.Rows) != 0 {
        t.Errorf("blank draft has %d rows, want 0", len(d.Rows))
    }

    // Refuses to overwrite.
    if _, err := s.NewBlank("work", week, "default"); err == nil {
        t.Errorf("NewBlank should refuse on collision")
    }
}
```

- [ ] **Step 4.2 — Run, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run TestService_NewBlank -v
```
Expected: FAIL — `NewBlank` undefined.

- [ ] **Step 4.3 — Implement**

```go
// internal/svc/draftsvc/new.go
package draftsvc

import (
    "fmt"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// NewBlank creates an empty dated draft. Refuses if (profile, weekStart, name)
// already exists.
func (s *Service) NewBlank(profile string, weekStart time.Time, name string) (domain.WeekDraft, error) {
    if name == "" {
        name = "default"
    }
    now := time.Now().UTC()
    d := domain.WeekDraft{
        SchemaVersion: 1,
        Profile:       profile,
        WeekStart:     weekStart,
        Name:          name,
        Provenance:    domain.DraftProvenance{Kind: domain.ProvenanceBlank},
        CreatedAt:     now,
        ModifiedAt:    now,
    }
    if err := s.store.SaveNew(d); err != nil {
        return domain.WeekDraft{}, fmt.Errorf("new blank: %w", err)
    }
    return d, nil
}
```

- [ ] **Step 4.4 — Run, verify pass + commit**

```bash
go test ./internal/svc/draftsvc/ -v
git add internal/svc/draftsvc/
git commit -m "feat(draftsvc): NewBlank creates empty dated draft"
```

---

## Task 5: `Service.NewFromTemplate` — seed draft rows from a template

**Files:**
- Modify: `internal/svc/draftsvc/new.go`
- Modify: `internal/svc/draftsvc/new_test.go`

- [ ] **Step 5.1 — Failing test**

Append to `internal/svc/draftsvc/new_test.go`:

```go
func TestService_NewFromTemplate(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    tmpl := domain.Template{
        SchemaVersion: 1, Name: "canonical",
        Rows: []domain.TemplateRow{
            {ID: "row-01",
             Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 1},
             TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
             Hours: domain.WeekHours{Mon: 8, Tue: 8}},
        },
    }

    d, err := s.NewFromTemplate("work", week, "default", tmpl)
    if err != nil { t.Fatal(err) }
    if d.Provenance.Kind != domain.ProvenanceFromTemplate {
        t.Errorf("Provenance.Kind = %s, want from-template", d.Provenance.Kind)
    }
    if d.Provenance.FromTemplate != "canonical" {
        t.Errorf("Provenance.FromTemplate = %q, want canonical", d.Provenance.FromTemplate)
    }
    if len(d.Rows) != 1 {
        t.Fatalf("rows = %d, want 1", len(d.Rows))
    }
    if len(d.Rows[0].Cells) != 2 {
        t.Errorf("cells = %d, want 2 (Mon+Tue)", len(d.Rows[0].Cells))
    }
}
```

- [ ] **Step 5.2 — Run, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run TestService_NewFromTemplate -v
```

- [ ] **Step 5.3 — Implement**

Append to `internal/svc/draftsvc/new.go`:

```go
// NewFromTemplate creates a draft seeded from a template's rows.
// Cells are placed on weekdays where the template's WeekHours has non-zero hours.
func (s *Service) NewFromTemplate(profile string, weekStart time.Time, name string, tmpl domain.Template) (domain.WeekDraft, error) {
    if name == "" {
        name = "default"
    }
    rows := make([]domain.DraftRow, 0, len(tmpl.Rows))
    for i, tr := range tmpl.Rows {
        cells := make([]domain.DraftCell, 0, 7)
        for d := time.Sunday; d <= time.Saturday; d++ {
            h := tr.Hours.ForDay(d)
            if h == 0 {
                continue
            }
            cells = append(cells, domain.DraftCell{Day: d, Hours: h})
        }
        id := tr.ID
        if id == "" {
            id = fmt.Sprintf("row-%02d", i+1)
        }
        rows = append(rows, domain.DraftRow{
            ID:            id,
            Label:         tr.Label,
            Target:        tr.Target,
            TimeType:      tr.TimeType,
            Description:   tr.Description,
            Billable:      tr.Billable,
            ResolverHints: tr.ResolverHints,
            Cells:         cells,
        })
    }
    now := time.Now().UTC()
    d := domain.WeekDraft{
        SchemaVersion: 1,
        Profile:       profile,
        WeekStart:     weekStart,
        Name:          name,
        Provenance: domain.DraftProvenance{
            Kind:         domain.ProvenanceFromTemplate,
            FromTemplate: tmpl.Name,
        },
        CreatedAt:  now,
        ModifiedAt: now,
        Rows:       rows,
    }
    if err := s.store.SaveNew(d); err != nil {
        return domain.WeekDraft{}, fmt.Errorf("new from template: %w", err)
    }
    return d, nil
}
```

- [ ] **Step 5.4 — Run, verify pass + commit**

```bash
go test ./internal/svc/draftsvc/ -v
git add internal/svc/draftsvc/
git commit -m "feat(draftsvc): NewFromTemplate seeds draft from template rows"
```

---

## Task 6: `Service.NewFromDraft` — clone an existing draft (with optional shift)

**Files:**
- Modify: `internal/svc/draftsvc/new.go`
- Modify: `internal/svc/draftsvc/new_test.go`

- [ ] **Step 6.1 — Failing test**

```go
func TestService_NewFromDraft(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    srcWeek := time.Date(2026, 4, 26, 0, 0, 0, 0, domain.EasternTZ)
    dstWeek := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    src := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: srcWeek,
        Provenance: domain.DraftProvenance{Kind: domain.ProvenanceBlank},
        Rows: []domain.DraftRow{{
            ID: "row-01",
            Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4}},
        }},
    }
    if err := s.store.Save(src); err != nil { t.Fatal(err) }

    d, err := s.NewFromDraft("work", dstWeek, "default", "work", srcWeek, "default")
    if err != nil { t.Fatal(err) }
    if !d.WeekStart.Equal(dstWeek) {
        t.Errorf("dst weekStart = %v, want %v", d.WeekStart, dstWeek)
    }
    if d.Provenance.Kind != domain.ProvenanceFromDraft {
        t.Errorf("Provenance.Kind = %s, want from-draft", d.Provenance.Kind)
    }
    if d.Provenance.ShiftedByDays != 7 {
        t.Errorf("ShiftedByDays = %d, want 7", d.Provenance.ShiftedByDays)
    }
    if len(d.Rows) != 1 || len(d.Rows[0].Cells) != 1 {
        t.Errorf("rows/cells lost in clone")
    }
    // SourceEntryIDs and provenance NOT preserved (this is a fresh draft, not a snapshot).
    if d.Rows[0].Cells[0].SourceEntryID != 0 {
        t.Errorf("clone should drop sourceEntryIDs (was %d)", d.Rows[0].Cells[0].SourceEntryID)
    }
}
```

- [ ] **Step 6.2 — Run, verify failure**

- [ ] **Step 6.3 — Implement**

Append to `internal/svc/draftsvc/new.go`:

```go
// NewFromDraft clones an existing draft into a new (profile, weekStart, name).
// SourceEntryIDs are intentionally cleared — the cloned draft is fresh, not a
// snapshot of remote state. Provenance records the shift in days from src.
func (s *Service) NewFromDraft(profile string, weekStart time.Time, name string,
    srcProfile string, srcWeekStart time.Time, srcName string) (domain.WeekDraft, error) {
    if name == "" {
        name = "default"
    }
    src, err := s.store.Load(srcProfile, srcWeekStart, srcName)
    if err != nil {
        return domain.WeekDraft{}, fmt.Errorf("load source: %w", err)
    }

    rows := make([]domain.DraftRow, len(src.Rows))
    for i, r := range src.Rows {
        cells := make([]domain.DraftCell, len(r.Cells))
        for j, c := range r.Cells {
            cells[j] = domain.DraftCell{
                Day:     c.Day,
                Hours:   c.Hours,
                PerCell: c.PerCell,
                // SourceEntryID intentionally omitted.
            }
        }
        rows[i] = domain.DraftRow{
            ID: r.ID, Label: r.Label, Target: r.Target, TimeType: r.TimeType,
            Description: r.Description, Billable: r.Billable, ResolverHints: r.ResolverHints,
            Cells: cells,
        }
    }

    shiftDays := int(weekStart.Sub(srcWeekStart).Hours() / 24)
    fromRef := fmt.Sprintf("%s/%s",
        srcWeekStart.In(domain.EasternTZ).Format("2006-01-02"), srcName)

    now := time.Now().UTC()
    d := domain.WeekDraft{
        SchemaVersion: 1,
        Profile:       profile,
        WeekStart:     weekStart,
        Name:          name,
        Provenance: domain.DraftProvenance{
            Kind:          domain.ProvenanceFromDraft,
            FromDraft:     fromRef,
            ShiftedByDays: shiftDays,
        },
        CreatedAt:  now,
        ModifiedAt: now,
        Rows:       rows,
    }
    if err := s.store.SaveNew(d); err != nil {
        return domain.WeekDraft{}, fmt.Errorf("new from draft: %w", err)
    }
    return d, nil
}
```

- [ ] **Step 6.4 — Run, verify pass + commit**

```bash
go test ./internal/svc/draftsvc/ -v
git add internal/svc/draftsvc/
git commit -m "feat(draftsvc): NewFromDraft clones a draft with shift bookkeeping"
```

---

## Task 7: `tdx time week new` CLI command

**Files:**
- Create: `internal/cli/time/week/new.go`
- Create: `internal/cli/time/week/new_test.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 7.1 — Failing test**

```go
// internal/cli/time/week/new_test.go
package week

import (
    "bytes"
    "encoding/json"
    "testing"

    "github.com/iainmoffat/tdx/internal/domain"
)

func TestNewResultJSON_Schema(t *testing.T) {
    var buf bytes.Buffer
    err := writeNewResultJSON(&buf, domain.WeekDraft{Name: "default"})
    if err != nil { t.Fatal(err) }
    var resp map[string]any
    if err := json.Unmarshal(buf.Bytes(), &resp); err != nil { t.Fatal(err) }
    if resp["schema"] != "tdx.v1.weekDraftCreateResult" {
        t.Errorf("schema = %v", resp["schema"])
    }
}
```

- [ ] **Step 7.2 — Run, verify failure**

- [ ] **Step 7.3 — Implement**

```go
// internal/cli/time/week/new.go
package week

import (
    "encoding/json"
    "fmt"
    "io"
    "strconv"
    "strings"
    "time"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
    "github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

type newFlags struct {
    profile      string
    name         string
    fromTemplate string
    fromDraft    string
    shift        string
    json         bool
}

type weekDraftCreateResult struct {
    Schema string           `json:"schema"`
    Draft  domain.WeekDraft `json:"draft"`
}

func newNewCmd() *cobra.Command {
    var f newFlags
    cmd := &cobra.Command{
        Use:   "new <date>",
        Short: "Create a blank, template-seeded, or draft-cloned week draft",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runNew(cmd, f, args[0])
        },
    }
    cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
    cmd.Flags().StringVar(&f.name, "name", "", "draft name (default: default)")
    cmd.Flags().StringVar(&f.fromTemplate, "from-template", "", "seed rows from this template")
    cmd.Flags().StringVar(&f.fromDraft, "from-draft", "", "clone this draft (date or date/name)")
    cmd.Flags().StringVar(&f.shift, "shift", "", "with --from-draft, shift the source by this duration (e.g. 7d, -7d)")
    cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
    return cmd
}

func runNew(cmd *cobra.Command, f newFlags, dateRef string) error {
    weekStart, name, err := ParseDraftRef(dateRef)
    if err != nil { return err }
    if f.name != "" { name = f.name }

    if f.fromTemplate != "" && f.fromDraft != "" {
        return fmt.Errorf("--from-template and --from-draft are mutually exclusive")
    }
    if f.shift != "" && f.fromDraft == "" {
        return fmt.Errorf("--shift requires --from-draft")
    }

    paths, err := config.ResolvePaths()
    if err != nil { return err }
    auth := authsvc.New(paths)
    tsvc := timesvc.New(paths)
    drafts := draftsvc.NewService(paths, tsvc)

    profileName, err := auth.ResolveProfile(f.profile)
    if err != nil { return err }

    var draft domain.WeekDraft
    switch {
    case f.fromTemplate != "":
        tmplStore := tmplsvc.NewStore(paths)
        tmpl, err := tmplStore.Load(profileName, f.fromTemplate)
        if err != nil { return err }
        draft, err = drafts.NewFromTemplate(profileName, weekStart, name, tmpl)
        if err != nil { return err }
    case f.fromDraft != "":
        srcWeek, srcName, err := ParseDraftRef(f.fromDraft)
        if err != nil { return err }
        if f.shift != "" {
            d, err := parseShift(f.shift)
            if err != nil { return err }
            srcWeek = weekStart.Add(-d) // shift means "this draft is the source shifted by N to weekStart"
        }
        draft, err = drafts.NewFromDraft(profileName, weekStart, name, profileName, srcWeek, srcName)
        if err != nil { return err }
    default:
        draft, err = drafts.NewBlank(profileName, weekStart, name)
        if err != nil { return err }
    }

    w := cmd.OutOrStdout()
    if f.json {
        return writeNewResultJSON(w, draft)
    }
    _, _ = fmt.Fprintf(w, "Created draft %s/%s.\n",
        weekStart.Format("2006-01-02"), draft.Name)
    return nil
}

func writeNewResultJSON(w io.Writer, d domain.WeekDraft) error {
    return json.NewEncoder(w).Encode(weekDraftCreateResult{
        Schema: "tdx.v1.weekDraftCreateResult", Draft: d,
    })
}

// parseShift accepts strings like "7d", "-7d", "14d".
func parseShift(s string) (time.Duration, error) {
    s = strings.TrimSpace(s)
    if !strings.HasSuffix(s, "d") {
        return 0, fmt.Errorf("--shift must end in 'd' (e.g. 7d, -7d), got %q", s)
    }
    n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
    if err != nil { return 0, fmt.Errorf("invalid --shift %q: %w", s, err) }
    return time.Duration(n) * 24 * time.Hour, nil
}
```

- [ ] **Step 7.4 — Register in `week.go`**

In `internal/cli/time/week/week.go`, add `cmd.AddCommand(newNewCmd())` after the existing `cmd.AddCommand` calls.

- [ ] **Step 7.5 — Run, verify pass + build + commit**

```bash
go test ./... -v
go vet ./...
go build ./cmd/tdx
git add internal/cli/time/week/
git commit -m "feat(cli): tdx time week new (blank, --from-template, --from-draft, --shift)"
```

---

## Task 8: `Service.Copy` — clone src draft to a new dst ref

**Files:**
- Create: `internal/svc/draftsvc/copy.go`
- Create: `internal/svc/draftsvc/copy_test.go`

- [ ] **Step 8.1 — Failing test**

```go
// internal/svc/draftsvc/copy_test.go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestService_Copy_SameDateAlternate(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    src := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Notes: "primary",
        Rows: []domain.DraftRow{{ID: "row-01", Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4}}}},
    }
    if err := s.store.Save(src); err != nil { t.Fatal(err) }
    if err := s.store.SavePulledSnapshot(src); err != nil { t.Fatal(err) }

    dst, err := s.Copy("work", week, "default", "work", week, "pristine")
    if err != nil { t.Fatal(err) }
    if dst.Name != "pristine" || !dst.WeekStart.Equal(week) {
        t.Errorf("dst identity wrong: %+v", dst)
    }
    if dst.Notes != "primary" {
        t.Errorf("dst.Notes = %q, want %q", dst.Notes, "primary")
    }
    // Same-date copy should ALSO carry the pulled-snapshot sibling.
    if !s.store.Exists("work", week, "pristine") { t.Errorf("dst not saved") }
    if _, err := s.store.LoadPulledSnapshot("work", week, "pristine"); err != nil {
        t.Errorf("dst pulled snapshot missing: %v", err)
    }
}

func TestService_Copy_CrossWeek_DropsPulledSnapshot(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    srcWeek := time.Date(2026, 4, 26, 0, 0, 0, 0, domain.EasternTZ)
    dstWeek := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    src := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: srcWeek,
        Rows: []domain.DraftRow{{ID: "row-01", Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4, SourceEntryID: 100}}}},
    }
    if err := s.store.Save(src); err != nil { t.Fatal(err) }
    if err := s.store.SavePulledSnapshot(src); err != nil { t.Fatal(err) }

    dst, err := s.Copy("work", srcWeek, "default", "work", dstWeek, "default")
    if err != nil { t.Fatal(err) }
    if !dst.WeekStart.Equal(dstWeek) { t.Errorf("dst week wrong") }
    // Cross-week copy drops the pulled snapshot (different week, watermark meaningless).
    if _, err := s.store.LoadPulledSnapshot("work", dstWeek, "default"); err == nil {
        t.Errorf("dst should NOT have a pulled snapshot for cross-week copy")
    }
    // SourceEntryIDs cleared (cells point at src week's entries, irrelevant here).
    if dst.Rows[0].Cells[0].SourceEntryID != 0 {
        t.Errorf("cross-week copy should clear sourceEntryIDs, got %d",
            dst.Rows[0].Cells[0].SourceEntryID)
    }
}

func TestService_Copy_RefusesCollision(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    src := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
    dst := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "pristine", WeekStart: week}
    if err := s.store.Save(src); err != nil { t.Fatal(err) }
    if err := s.store.Save(dst); err != nil { t.Fatal(err) }

    if _, err := s.Copy("work", week, "default", "work", week, "pristine"); err == nil {
        t.Errorf("Copy should refuse on dst collision")
    }
}
```

- [ ] **Step 8.2 — Run, verify failure**

- [ ] **Step 8.3 — Implement**

```go
// internal/svc/draftsvc/copy.go
package draftsvc

import (
    "fmt"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// Copy clones src into a new draft at dst. If src and dst share the same
// (profile, weekStart), the pulled-snapshot sibling is also copied (the
// watermark remains meaningful). Cross-date copies clear sourceEntryIDs and
// skip the pulled-snapshot copy.
func (s *Service) Copy(srcProfile string, srcWeekStart time.Time, srcName string,
    dstProfile string, dstWeekStart time.Time, dstName string) (domain.WeekDraft, error) {
    if dstName == "" {
        dstName = "default"
    }
    src, err := s.store.Load(srcProfile, srcWeekStart, srcName)
    if err != nil {
        return domain.WeekDraft{}, fmt.Errorf("copy: load source: %w", err)
    }

    sameWeek := srcWeekStart.Equal(dstWeekStart) && srcProfile == dstProfile

    rows := make([]domain.DraftRow, len(src.Rows))
    for i, r := range src.Rows {
        cells := make([]domain.DraftCell, len(r.Cells))
        for j, c := range r.Cells {
            nc := c
            if !sameWeek {
                nc.SourceEntryID = 0
            }
            cells[j] = nc
        }
        nr := r
        nr.Cells = cells
        rows[i] = nr
    }

    shiftDays := int(dstWeekStart.Sub(srcWeekStart).Hours() / 24)
    fromRef := fmt.Sprintf("%s/%s",
        srcWeekStart.In(domain.EasternTZ).Format("2006-01-02"), srcName)

    now := time.Now().UTC()
    dst := domain.WeekDraft{
        SchemaVersion: 1,
        Profile:       dstProfile,
        WeekStart:     dstWeekStart,
        Name:          dstName,
        Notes:         src.Notes,
        Tags:          src.Tags,
        Provenance: domain.DraftProvenance{
            Kind:          domain.ProvenanceFromDraft,
            FromDraft:     fromRef,
            ShiftedByDays: shiftDays,
        },
        CreatedAt:  now,
        ModifiedAt: now,
        Rows:       rows,
    }
    if err := s.store.SaveNew(dst); err != nil {
        return domain.WeekDraft{}, fmt.Errorf("copy: %w", err)
    }
    if sameWeek {
        if pulled, err := s.store.LoadPulledSnapshot(srcProfile, srcWeekStart, srcName); err == nil {
            pulled.Name = dstName
            if err := s.store.SavePulledSnapshot(pulled); err != nil {
                return domain.WeekDraft{}, fmt.Errorf("copy pulled snapshot: %w", err)
            }
        }
    }
    return dst, nil
}
```

- [ ] **Step 8.4 — Run, verify pass + commit**

```bash
go test ./internal/svc/draftsvc/ -v
git add internal/svc/draftsvc/
git commit -m "feat(draftsvc): Copy with same-week pulled-snapshot preservation"
```

---

## Task 9: `tdx time week copy <src> <dst>` CLI

**Files:**
- Create: `internal/cli/time/week/copy.go`
- Create: `internal/cli/time/week/copy_test.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 9.1 — Failing test**

```go
// internal/cli/time/week/copy_test.go
package week

import (
    "bytes"
    "encoding/json"
    "testing"

    "github.com/iainmoffat/tdx/internal/domain"
)

func TestCopyResultJSON_Schema(t *testing.T) {
    var buf bytes.Buffer
    if err := writeCopyResultJSON(&buf, domain.WeekDraft{Name: "pristine"}); err != nil { t.Fatal(err) }
    var resp map[string]any
    if err := json.Unmarshal(buf.Bytes(), &resp); err != nil { t.Fatal(err) }
    if resp["schema"] != "tdx.v1.weekDraftCopyResult" {
        t.Errorf("schema = %v", resp["schema"])
    }
}
```

- [ ] **Step 9.2 — Run, verify failure**

- [ ] **Step 9.3 — Implement**

```go
// internal/cli/time/week/copy.go
package week

import (
    "encoding/json"
    "fmt"
    "io"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type copyFlags struct {
    profile string
    json    bool
}

type weekDraftCopyResult struct {
    Schema string           `json:"schema"`
    Draft  domain.WeekDraft `json:"draft"`
}

func newCopyCmd() *cobra.Command {
    var f copyFlags
    cmd := &cobra.Command{
        Use:   "copy <src> <dst>",
        Short: "Clone a draft into a new (date, name) ref",
        Args:  cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runCopy(cmd, f, args[0], args[1])
        },
    }
    cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
    cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
    return cmd
}

func runCopy(cmd *cobra.Command, f copyFlags, srcRef, dstRef string) error {
    srcWeek, srcName, err := ParseDraftRef(srcRef)
    if err != nil { return fmt.Errorf("src: %w", err) }
    dstWeek, dstName, err := ParseDraftRef(dstRef)
    if err != nil { return fmt.Errorf("dst: %w", err) }

    paths, err := config.ResolvePaths()
    if err != nil { return err }
    auth := authsvc.New(paths)
    tsvc := timesvc.New(paths)
    drafts := draftsvc.NewService(paths, tsvc)

    profileName, err := auth.ResolveProfile(f.profile)
    if err != nil { return err }

    dst, err := drafts.Copy(profileName, srcWeek, srcName, profileName, dstWeek, dstName)
    if err != nil { return err }

    w := cmd.OutOrStdout()
    if f.json {
        return writeCopyResultJSON(w, dst)
    }
    _, _ = fmt.Fprintf(w, "Copied draft %s/%s -> %s/%s.\n",
        srcWeek.Format("2006-01-02"), srcName, dstWeek.Format("2006-01-02"), dstName)
    return nil
}

func writeCopyResultJSON(w io.Writer, d domain.WeekDraft) error {
    return json.NewEncoder(w).Encode(weekDraftCopyResult{
        Schema: "tdx.v1.weekDraftCopyResult", Draft: d,
    })
}
```

- [ ] **Step 9.4 — Register in `week.go`**, run tests, commit:

```bash
go test ./... -v
go build ./cmd/tdx
git add internal/cli/time/week/
git commit -m "feat(cli): tdx time week copy <src> <dst>"
```

---

## Task 10: `Service.Rename` with auto-snapshot pre-flight

**Files:**
- Create: `internal/svc/draftsvc/rename.go`
- Create: `internal/svc/draftsvc/rename_test.go`

- [ ] **Step 10.1 — Failing tests**

```go
// internal/svc/draftsvc/rename_test.go
package draftsvc

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestService_Rename_Success(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "old", WeekStart: week}
    if err := s.store.Save(d); err != nil { t.Fatal(err) }
    if err := s.store.SavePulledSnapshot(d); err != nil { t.Fatal(err) }
    if _, err := s.snapshots.Take(d, OpManual, ""); err != nil { t.Fatal(err) }

    if err := s.Rename("work", week, "old", "new"); err != nil { t.Fatal(err) }

    if !s.store.Exists("work", week, "new") { t.Errorf("new draft missing after rename") }
    if s.store.Exists("work", week, "old") { t.Errorf("old draft still present after rename") }

    // Pulled-snapshot followed.
    if _, err := s.store.LoadPulledSnapshot("work", week, "new"); err != nil {
        t.Errorf("pulled snapshot missing for new: %v", err)
    }
    // Snapshots dir followed.
    list, err := s.snapshots.List("work", week, "new")
    if err != nil { t.Fatal(err) }
    // There should be at least 1 manual + 1 pre-rename.
    if len(list) < 2 {
        t.Errorf("snapshots list = %d, want >= 2", len(list))
    }
    var hasPreRename bool
    for _, sn := range list {
        if sn.Op == OpPreRename { hasPreRename = true }
    }
    if !hasPreRename {
        t.Errorf("no pre-rename snapshot found")
    }

    // Loaded draft has new name in YAML.
    loaded, err := s.store.Load("work", week, "new")
    if err != nil { t.Fatal(err) }
    if loaded.Name != "new" {
        t.Errorf("YAML name = %q, want new", loaded.Name)
    }
}

func TestService_Rename_RefusesCollision(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    a := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "a", WeekStart: week}
    b := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "b", WeekStart: week}
    if err := s.store.Save(a); err != nil { t.Fatal(err) }
    if err := s.store.Save(b); err != nil { t.Fatal(err) }

    if err := s.Rename("work", week, "a", "b"); err == nil {
        t.Errorf("Rename should refuse when target exists")
    }
    if !s.store.Exists("work", week, "a") { t.Errorf("source disappeared on failed rename") }
    if !s.store.Exists("work", week, "b") { t.Errorf("destination disappeared on failed rename") }

    // No leftover .pulled.yaml files for orphaned name.
    pulledPath := filepath.Join(paths.ProfileWeeksDir("work"),
        week.In(domain.EasternTZ).Format("2006-01-02"), "a.pulled.yaml")
    if _, err := os.Stat(pulledPath); err == nil {
        // Allowed: a's pulled may not exist anyway. Just don't crash.
        _ = err
    }
}
```

- [ ] **Step 10.2 — Run, verify failure**

```bash
go test ./internal/svc/draftsvc/ -run TestService_Rename -v
```
Expected: FAIL — `Rename` and `OpPreRename` undefined.

- [ ] **Step 10.3 — Implement**

```go
// internal/svc/draftsvc/rename.go
package draftsvc

import (
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// Rename moves a draft from oldName to newName at the same (profile, weekStart).
// Auto-snapshots before any file motion. Refuses on collision. Renames the YAML,
// the pulled-snapshot sibling, and the snapshots directory.
func (s *Service) Rename(profile string, weekStart time.Time, oldName, newName string) error {
    if oldName == newName {
        return fmt.Errorf("rename: oldName == newName")
    }
    if !s.store.Exists(profile, weekStart, oldName) {
        return fmt.Errorf("rename: draft %q does not exist", oldName)
    }
    if s.store.Exists(profile, weekStart, newName) {
        return fmt.Errorf("rename: target %q already exists", newName)
    }

    src, err := s.store.Load(profile, weekStart, oldName)
    if err != nil { return err }

    if _, err := s.snapshots.Take(src, OpPreRename, "rename:"+oldName+"->"+newName); err != nil {
        return fmt.Errorf("rename: pre-flight snapshot: %w", err)
    }

    dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
    weeksDir := s.paths.ProfileWeeksDir(profile)
    oldYAML := filepath.Join(weeksDir, dateDir, oldName+".yaml")
    newYAML := filepath.Join(weeksDir, dateDir, newName+".yaml")
    oldPulled := filepath.Join(weeksDir, dateDir, oldName+".pulled.yaml")
    newPulled := filepath.Join(weeksDir, dateDir, newName+".pulled.yaml")
    oldSnaps := filepath.Join(weeksDir, dateDir, oldName+".snapshots")
    newSnaps := filepath.Join(weeksDir, dateDir, newName+".snapshots")

    src.Name = newName
    if err := s.store.Save(src); err != nil {
        return fmt.Errorf("rename: write new YAML: %w", err)
    }
    if err := os.Remove(oldYAML); err != nil {
        return fmt.Errorf("rename: remove old YAML: %w", err)
    }

    if _, err := os.Stat(oldPulled); err == nil {
        if err := os.Rename(oldPulled, newPulled); err != nil {
            return fmt.Errorf("rename: pulled sibling: %w", err)
        }
    } else if !os.IsNotExist(err) {
        return fmt.Errorf("rename: stat pulled sibling: %w", err)
    }

    if _, err := os.Stat(oldSnaps); err == nil {
        if err := os.Rename(oldSnaps, newSnaps); err != nil {
            return fmt.Errorf("rename: snapshots dir: %w", err)
        }
    } else if !os.IsNotExist(err) {
        return fmt.Errorf("rename: stat snapshots dir: %w", err)
    }

    return nil
}
```

Add `OpPreRename OpTag = "pre-rename"` to the constants block in `internal/svc/draftsvc/snapshot.go`.

- [ ] **Step 10.4 — Run, verify pass + commit**

```bash
go test ./internal/svc/draftsvc/ -v
git add internal/svc/draftsvc/
git commit -m "feat(draftsvc): Rename with pre-rename auto-snapshot"
```

---

## Task 11: `tdx time week rename` CLI

**Files:**
- Create: `internal/cli/time/week/rename.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 11.1 — Implement**

```go
// internal/cli/time/week/rename.go
package week

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type renameFlags struct {
    profile string
}

func newRenameCmd() *cobra.Command {
    var f renameFlags
    cmd := &cobra.Command{
        Use:   "rename <date>[/<oldName>] <newName>",
        Short: "Rename a draft (auto-snapshots first)",
        Args:  cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runRename(cmd, f, args[0], args[1])
        },
    }
    cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
    return cmd
}

func runRename(cmd *cobra.Command, f renameFlags, srcRef, newName string) error {
    weekStart, oldName, err := ParseDraftRef(srcRef)
    if err != nil { return err }
    if newName == "" {
        return fmt.Errorf("newName cannot be empty")
    }

    paths, err := config.ResolvePaths()
    if err != nil { return err }
    auth := authsvc.New(paths)
    tsvc := timesvc.New(paths)
    drafts := draftsvc.NewService(paths, tsvc)

    profileName, err := auth.ResolveProfile(f.profile)
    if err != nil { return err }

    if err := drafts.Rename(profileName, weekStart, oldName, newName); err != nil {
        return err
    }
    _, _ = fmt.Fprintf(cmd.OutOrStdout(), "Renamed draft %s/%s -> %s/%s.\n",
        weekStart.Format("2006-01-02"), oldName, weekStart.Format("2006-01-02"), newName)
    return nil
}
```

Register in `week.go`. Run tests. Commit:

```bash
go test ./... -v
go build ./cmd/tdx
git add internal/cli/time/week/
git commit -m "feat(cli): tdx time week rename"
```

---

## Task 12: `Service.Reset` and `tdx time week reset`

**Files:**
- Create: `internal/svc/draftsvc/reset.go`
- Modify: `internal/svc/draftsvc/snapshot.go` (add `OpPreReset`)
- Create: `internal/cli/time/week/reset.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 12.1 — Failing test**

```go
// internal/svc/draftsvc/reset_test.go
package draftsvc

import (
    "context"
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestService_Reset_DiscardsLocalAndRePulls(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    target := domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 1}
    timeType := domain.TimeType{ID: 7, Name: "Work"}

    mw := &mockTimeWriter{
        weekRpt: domain.WeekReport{
            WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
            Status:  domain.ReportOpen,
            Entries: []domain.TimeEntry{
                {ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 480,
                    Target: target, TimeType: timeType, Billable: true},
            },
        },
    }
    s := newServiceWithTimeWriter(paths, mw)

    // Pre-reset draft has dirty hand-edits.
    edited := domain.WeekDraft{
        SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
        Notes: "local edits",
        Rows: []domain.DraftRow{{ID: "row-99", Cells: []domain.DraftCell{{Day: time.Friday, Hours: 99}}}},
    }
    if err := s.store.Save(edited); err != nil { t.Fatal(err) }

    if err := s.Reset(context.Background(), "work", week, "default"); err != nil {
        t.Fatal(err)
    }

    // Reset replaces the draft with a fresh pull.
    fresh, err := s.store.Load("work", week, "default")
    if err != nil { t.Fatal(err) }
    if fresh.Notes == "local edits" {
        t.Errorf("Reset did not discard notes")
    }
    if len(fresh.Rows) != 1 || fresh.Rows[0].Cells[0].SourceEntryID != 100 {
        t.Errorf("Reset did not produce fresh-pull rows: %+v", fresh.Rows)
    }

    // pre-reset snapshot saved.
    list, err := s.snapshots.List("work", week, "default")
    if err != nil { t.Fatal(err) }
    var hasPreReset bool
    for _, sn := range list { if sn.Op == OpPreReset { hasPreReset = true } }
    if !hasPreReset { t.Errorf("no pre-reset snapshot taken") }
}
```

- [ ] **Step 12.2 — Run, verify failure**

- [ ] **Step 12.3 — Implement**

Add to `snapshot.go` constants:
```go
OpPreReset OpTag = "pre-reset"
```

Create `internal/svc/draftsvc/reset.go`:

```go
package draftsvc

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// Reset discards local edits and re-pulls the week. Requires the existing
// draft to load successfully; takes a pre-reset snapshot before clobbering.
func (s *Service) Reset(ctx context.Context, profile string, weekStart time.Time, name string) error {
    if name == "" {
        name = "default"
    }
    existing, err := s.store.Load(profile, weekStart, name)
    if err != nil {
        return fmt.Errorf("reset: load existing: %w", err)
    }
    if _, err := s.snapshots.Take(existing, OpPreReset, ""); err != nil {
        return fmt.Errorf("reset: snapshot: %w", err)
    }

    if err := s.store.Delete(profile, weekStart, name); err != nil {
        return fmt.Errorf("reset: delete existing: %w", err)
    }
    dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
    pulled := filepath.Join(s.paths.ProfileWeeksDir(profile), dateDir, name+".pulled.yaml")
    _ = os.Remove(pulled)

    if _, err := s.Pull(ctx, profile, weekStart, name, false); err != nil {
        return fmt.Errorf("reset: re-pull: %w", err)
    }
    return nil
}
```

Create `internal/cli/time/week/reset.go`:

```go
package week

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type resetFlags struct {
    profile string
    yes     bool
}

func newResetCmd() *cobra.Command {
    var f resetFlags
    cmd := &cobra.Command{
        Use:   "reset <date>[/<name>]",
        Short: "Discard local edits and re-pull (auto-snapshots first)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if !f.yes {
                return fmt.Errorf("pass --yes to discard local edits")
            }
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }

            paths, err := config.ResolvePaths()
            if err != nil { return err }
            auth := authsvc.New(paths)
            tsvc := timesvc.New(paths)
            drafts := draftsvc.NewService(paths, tsvc)

            profileName, err := auth.ResolveProfile(f.profile)
            if err != nil { return err }

            if err := drafts.Reset(cmd.Context(), profileName, weekStart, name); err != nil {
                return err
            }
            _, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reset draft %s/%s.\n",
                weekStart.Format("2006-01-02"), name)
            return nil
        },
    }
    cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
    cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm discarding local edits")
    return cmd
}
```

Register in `week.go`. Run tests. Commit:

```bash
go test ./... -v
go build ./cmd/tdx
git add internal/svc/draftsvc/ internal/cli/time/week/
git commit -m "feat(cli, draftsvc): Reset (discard local + re-pull, auto-snapshot)"
```

---

## Task 13: `Service.SetArchived` and `tdx time week archive` / `unarchive`

**Files:**
- Create: `internal/svc/draftsvc/archive.go`
- Create: `internal/svc/draftsvc/archive_test.go`
- Create: `internal/cli/time/week/archive.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 13.1 — Failing test**

```go
// internal/svc/draftsvc/archive_test.go
package draftsvc

import (
    "testing"
    "time"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/domain"
)

func TestService_SetArchived(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
    if err := s.store.Save(d); err != nil { t.Fatal(err) }

    if err := s.SetArchived("work", week, "default", true); err != nil { t.Fatal(err) }
    loaded, _ := s.store.Load("work", week, "default")
    if !loaded.Archived { t.Errorf("Archived = false after SetArchived(true)") }

    if err := s.SetArchived("work", week, "default", false); err != nil { t.Fatal(err) }
    loaded, _ = s.store.Load("work", week, "default")
    if loaded.Archived { t.Errorf("Archived = true after SetArchived(false)") }
}
```

- [ ] **Step 13.2 — Implement**

```go
// internal/svc/draftsvc/archive.go
package draftsvc

import (
    "fmt"
    "time"

    "github.com/iainmoffat/tdx/internal/domain"
)

// SetArchived flips the Archived flag on the draft and saves it. Idempotent.
func (s *Service) SetArchived(profile string, weekStart time.Time, name string, archived bool) error {
    if name == "" {
        name = "default"
    }
    d, err := s.store.Load(profile, weekStart, name)
    if err != nil {
        return fmt.Errorf("set archived: %w", err)
    }
    if d.Archived == archived {
        return nil
    }
    d.Archived = archived
    d.ModifiedAt = time.Now().UTC()
    return s.store.Save(d)
}

var _ = domain.WeekDraft{}
```

Create `internal/cli/time/week/archive.go`:

```go
package week

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

func newArchiveCmd() *cobra.Command {
    var profile string
    cmd := &cobra.Command{
        Use:   "archive <date>[/<name>]",
        Short: "Hide a draft from default `list` output",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runArchive(cmd, profile, args[0], true)
        },
    }
    cmd.Flags().StringVar(&profile, "profile", "", "profile name")
    return cmd
}

func newUnarchiveCmd() *cobra.Command {
    var profile string
    cmd := &cobra.Command{
        Use:   "unarchive <date>[/<name>]",
        Short: "Show a previously archived draft in default `list` output",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runArchive(cmd, profile, args[0], false)
        },
    }
    cmd.Flags().StringVar(&profile, "profile", "", "profile name")
    return cmd
}

func runArchive(cmd *cobra.Command, profileFlag, ref string, archive bool) error {
    weekStart, name, err := ParseDraftRef(ref)
    if err != nil { return err }

    paths, err := config.ResolvePaths()
    if err != nil { return err }
    auth := authsvc.New(paths)
    tsvc := timesvc.New(paths)
    drafts := draftsvc.NewService(paths, tsvc)

    profileName, err := auth.ResolveProfile(profileFlag)
    if err != nil { return err }

    if err := drafts.SetArchived(profileName, weekStart, name, archive); err != nil {
        return err
    }
    verb := "Archived"
    if !archive { verb = "Unarchived" }
    _, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s draft %s/%s.\n",
        verb, weekStart.Format("2006-01-02"), name)
    return nil
}
```

Register both in `week.go`. Test, commit:

```bash
go test ./... -v
git add internal/svc/draftsvc/ internal/cli/time/week/
git commit -m "feat(cli, draftsvc): archive / unarchive (soft via Archived flag)"
```

---

## Task 14: `tdx time week snapshot` (manual snapshot with --keep + --note)

**Files:**
- Create: `internal/cli/time/week/snapshot.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 14.1 — Implement**

```go
// internal/cli/time/week/snapshot.go
package week

import (
    "encoding/json"
    "fmt"
    "io"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type snapshotFlags struct {
    profile string
    keep    bool
    note    string
    json    bool
}

type weekDraftSnapshotResp struct {
    Schema string                  `json:"schema"`
    Snapshot draftsvc.SnapshotInfo `json:"snapshot"`
}

func newSnapshotCmd() *cobra.Command {
    var f snapshotFlags
    cmd := &cobra.Command{
        Use:   "snapshot <date>[/<name>]",
        Short: "Take a manual snapshot of a draft",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runSnapshot(cmd, f, args[0])
        },
    }
    cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
    cmd.Flags().BoolVar(&f.keep, "keep", false, "pin the snapshot (exempt from auto-prune)")
    cmd.Flags().StringVar(&f.note, "note", "", "note to attach to the snapshot")
    cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
    return cmd
}

func runSnapshot(cmd *cobra.Command, f snapshotFlags, ref string) error {
    weekStart, name, err := ParseDraftRef(ref)
    if err != nil { return err }

    paths, err := config.ResolvePaths()
    if err != nil { return err }
    auth := authsvc.New(paths)
    tsvc := timesvc.New(paths)
    drafts := draftsvc.NewService(paths, tsvc)

    profileName, err := auth.ResolveProfile(f.profile)
    if err != nil { return err }

    d, err := drafts.Store().Load(profileName, weekStart, name)
    if err != nil { return err }

    info, err := drafts.Snapshots().Take(d, draftsvc.OpManual, f.note)
    if err != nil { return err }
    if f.keep {
        if err := drafts.Snapshots().Pin(profileName, weekStart, name, info.Sequence, f.note); err != nil {
            return err
        }
        info.Pinned = true
    }

    w := cmd.OutOrStdout()
    if f.json {
        return writeSnapshotJSON(w, info)
    }
    pin := ""
    if info.Pinned { pin = " (pinned)" }
    _, _ = fmt.Fprintf(w, "Snapshot %d taken for draft %s/%s%s.\n",
        info.Sequence, weekStart.Format("2006-01-02"), name, pin)
    return nil
}

func writeSnapshotJSON(w io.Writer, info draftsvc.SnapshotInfo) error {
    return json.NewEncoder(w).Encode(weekDraftSnapshotResp{
        Schema: "tdx.v1.weekDraftSnapshot", Snapshot: info,
    })
}
```

Register in `week.go`. Test, commit:

```bash
go test ./... -v
git add internal/cli/time/week/
git commit -m "feat(cli): tdx time week snapshot --keep --note"
```

---

## Task 15: `tdx time week restore --snapshot N`

**Files:**
- Modify: `internal/svc/draftsvc/snapshot.go` (add `Restore` method or use existing Load + Save)
- Create: `internal/cli/time/week/restore.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 15.1 — Failing test**

```go
// Append to internal/svc/draftsvc/snapshot_test.go
func TestService_RestoreSnapshot(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

    base := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week, Notes: "v1"}
    if err := s.store.Save(base); err != nil { t.Fatal(err) }
    snap, err := s.snapshots.Take(base, OpManual, "")
    if err != nil { t.Fatal(err) }

    // Mutate.
    base.Notes = "v2"
    if err := s.store.Save(base); err != nil { t.Fatal(err) }

    // Restore to snap.
    if err := s.RestoreSnapshot("work", week, "default", snap.Sequence); err != nil { t.Fatal(err) }

    restored, _ := s.store.Load("work", week, "default")
    if restored.Notes != "v1" { t.Errorf("Notes = %q after restore, want v1", restored.Notes) }

    // pre-restore snapshot taken.
    list, _ := s.snapshots.List("work", week, "default")
    var hasPreRestore bool
    for _, sn := range list { if sn.Op == OpPreRestore { hasPreRestore = true } }
    if !hasPreRestore { t.Errorf("no pre-restore snapshot taken") }
}
```

- [ ] **Step 15.2 — Implement service method**

Append to `internal/svc/draftsvc/snapshot.go`:

```go
// RestoreSnapshot reloads a snapshot's contents back into the live draft.
// Auto-snapshots the current state as pre-restore first.
func (s *Service) RestoreSnapshot(profile string, weekStart time.Time, name string, seq int) error {
    cur, err := s.store.Load(profile, weekStart, name)
    if err != nil {
        return fmt.Errorf("restore: load current: %w", err)
    }
    if _, err := s.snapshots.Take(cur, OpPreRestore, ""); err != nil {
        return fmt.Errorf("restore: pre-snapshot: %w", err)
    }
    snap, err := s.snapshots.Load(profile, weekStart, name, seq)
    if err != nil {
        return fmt.Errorf("restore: load snapshot %d: %w", seq, err)
    }
    snap.ModifiedAt = time.Now().UTC()
    return s.store.Save(snap)
}
```

(Add `"fmt"` and `"time"` imports if not already present in `snapshot.go`.)

- [ ] **Step 15.3 — CLI**

Create `internal/cli/time/week/restore.go`:

```go
package week

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type restoreFlags struct {
    profile  string
    snapshot int
    yes      bool
}

func newRestoreCmd() *cobra.Command {
    var f restoreFlags
    cmd := &cobra.Command{
        Use:   "restore <date>[/<name>] --snapshot N --yes",
        Short: "Restore a draft from a snapshot (auto-snapshots current first)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if !f.yes {
                return fmt.Errorf("pass --yes to overwrite the current draft")
            }
            if f.snapshot <= 0 {
                return fmt.Errorf("--snapshot is required (use `tdx time week history` to find sequence numbers)")
            }
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }

            paths, err := config.ResolvePaths()
            if err != nil { return err }
            auth := authsvc.New(paths)
            tsvc := timesvc.New(paths)
            drafts := draftsvc.NewService(paths, tsvc)

            profileName, err := auth.ResolveProfile(f.profile)
            if err != nil { return err }

            if err := drafts.RestoreSnapshot(profileName, weekStart, name, f.snapshot); err != nil {
                return err
            }
            _, _ = fmt.Fprintf(cmd.OutOrStdout(), "Restored draft %s/%s from snapshot %d.\n",
                weekStart.Format("2006-01-02"), name, f.snapshot)
            return nil
        },
    }
    cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
    cmd.Flags().IntVar(&f.snapshot, "snapshot", 0, "snapshot sequence number to restore")
    cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm overwrite")
    return cmd
}
```

Register in `week.go`. Test, commit:

```bash
go test ./... -v
git add internal/svc/draftsvc/ internal/cli/time/week/
git commit -m "feat(cli, draftsvc): tdx time week restore --snapshot N"
```

---

## Task 16: `tdx time week prune` (drops unpinned snapshots beyond retention or older-than)

**Files:**
- Modify: `internal/svc/draftsvc/snapshot.go` (add `PruneOlderThan` method)
- Create: `internal/cli/time/week/prune.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 16.1 — Failing test**

```go
// Append to internal/svc/draftsvc/snapshot_test.go
func TestSnapshotStore_PruneOlderThan(t *testing.T) {
    paths := config.Paths{Root: t.TempDir()}
    ss := NewSnapshotStore(paths, 100)  // big retention so age is what matters
    week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
    d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}

    // Take 3 snapshots; we'll backdate the first two via filesystem touch.
    s1, _ := ss.Take(d, OpManual, "")
    s2, _ := ss.Take(d, OpManual, "")
    s3, _ := ss.Take(d, OpManual, "")

    // Backdate s1 and s2 to 60 days ago.
    old := time.Now().Add(-60 * 24 * time.Hour)
    if err := os.Chtimes(s1.Path, old, old); err != nil { t.Fatal(err) }
    if err := os.Chtimes(s2.Path, old, old); err != nil { t.Fatal(err) }

    n, err := ss.PruneOlderThan("work", week, "default", 30*24*time.Hour)
    if err != nil { t.Fatal(err) }
    if n != 2 { t.Errorf("pruned %d, want 2", n) }

    list, _ := ss.List("work", week, "default")
    if len(list) != 1 || list[0].Sequence != s3.Sequence {
        t.Errorf("survivors wrong: %+v", list)
    }
}
```

Add `"os"` import in the test file if not already present.

- [ ] **Step 16.2 — Implement service method**

Append to `internal/svc/draftsvc/snapshot.go`:

```go
// PruneOlderThan removes unpinned snapshots whose mtime is older than `maxAge`.
// Returns the number pruned.
func (ss *SnapshotStore) PruneOlderThan(profile string, weekStart time.Time, name string, maxAge time.Duration) (int, error) {
    list, err := ss.List(profile, weekStart, name)
    if err != nil { return 0, err }
    cutoff := time.Now().Add(-maxAge)
    pruned := 0
    for _, s := range list {
        if s.Pinned {
            continue
        }
        info, err := os.Stat(s.Path)
        if err != nil {
            return pruned, err
        }
        if info.ModTime().After(cutoff) {
            continue
        }
        if err := os.Remove(s.Path); err != nil {
            return pruned, err
        }
        pruned++
    }
    return pruned, nil
}
```

- [ ] **Step 16.3 — CLI**

Create `internal/cli/time/week/prune.go`:

```go
package week

import (
    "fmt"
    "time"

    "github.com/spf13/cobra"

    "github.com/iainmoffat/tdx/internal/config"
    "github.com/iainmoffat/tdx/internal/svc/authsvc"
    "github.com/iainmoffat/tdx/internal/svc/draftsvc"
    "github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type pruneFlags struct {
    profile    string
    olderThan  string
    yes        bool
}

func newPruneCmd() *cobra.Command {
    var f pruneFlags
    cmd := &cobra.Command{
        Use:   "prune <date>[/<name>]",
        Short: "Drop unpinned snapshots older than --older-than (default: prune to retention cap)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if !f.yes {
                return fmt.Errorf("pass --yes to actually delete snapshots")
            }
            weekStart, name, err := ParseDraftRef(args[0])
            if err != nil { return err }

            paths, err := config.ResolvePaths()
            if err != nil { return err }
            auth := authsvc.New(paths)
            tsvc := timesvc.New(paths)
            drafts := draftsvc.NewService(paths, tsvc)

            profileName, err := auth.ResolveProfile(f.profile)
            if err != nil { return err }

            if f.olderThan != "" {
                d, err := parseDuration(f.olderThan)
                if err != nil { return err }
                n, err := drafts.Snapshots().PruneOlderThan(profileName, weekStart, name, d)
                if err != nil { return err }
                _, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d snapshot(s) older than %s.\n", n, f.olderThan)
                return nil
            }

            // Default: prune to retention cap by re-taking and discarding.
            // Phase A's prune fires automatically on Take; here we re-prune by
            // calling the internal `prune` via a fresh Take + discard? Simpler:
            // expose a Prune helper.
            n, err := drafts.Snapshots().PruneToRetention(profileName, weekStart, name)
            if err != nil { return err }
            _, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d snapshot(s) beyond retention cap.\n", n)
            return nil
        },
    }
    cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
    cmd.Flags().StringVar(&f.olderThan, "older-than", "", "prune snapshots older than this (e.g. 30d, 7d)")
    cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm deletion")
    return cmd
}

// parseDuration accepts "Nd" suffixes (days). Reuse existing parseShift logic.
func parseDuration(s string) (time.Duration, error) {
    return parseShift(s)
}
```

Add a public `PruneToRetention` wrapper to `internal/svc/draftsvc/snapshot.go`:

```go
// PruneToRetention exposes the internal retention-based prune.
func (ss *SnapshotStore) PruneToRetention(profile string, weekStart time.Time, name string) (int, error) {
    list, err := ss.List(profile, weekStart, name)
    if err != nil { return 0, err }
    var unpinned []SnapshotInfo
    for _, s := range list { if !s.Pinned { unpinned = append(unpinned, s) } }
    if len(unpinned) <= ss.retention { return 0, nil }
    sort.SliceStable(unpinned, func(i, j int) bool { return unpinned[i].Sequence < unpinned[j].Sequence })
    excess := len(unpinned) - ss.retention
    for i := 0; i < excess; i++ {
        if err := os.Remove(unpinned[i].Path); err != nil { return i, err }
    }
    return excess, nil
}
```

Register in `week.go`. Test, commit:

```bash
go test ./... -v
git add internal/svc/draftsvc/ internal/cli/time/week/
git commit -m "feat(cli, draftsvc): tdx time week prune --older-than"
```

---

## Task 17: `list` UX — alternate-name grouping + `--archived` flag

**Files:**
- Modify: `internal/cli/time/week/list.go`
- Modify: `internal/cli/time/week/list_test.go`

- [ ] **Step 17.1 — Failing test**

Append to `internal/cli/time/week/list_test.go`:

```go
func TestListText_GroupsByDate(t *testing.T) {
    items := []weekDraftListItem{
        {WeekStart: "2026-05-04", Name: "default", SyncState: "dirty", TotalHours: 18},
        {WeekStart: "2026-05-04", Name: "pristine", SyncState: "clean", TotalHours: 20},
        {WeekStart: "2026-04-12", Name: "default", SyncState: "clean", TotalHours: 20},
    }
    var buf bytes.Buffer
    writeListText(&buf, items)
    out := buf.String()
    // First date row shows the date.
    if !strings.Contains(out, "2026-05-04") {
        t.Errorf("first date missing: %q", out)
    }
    // Second alternate row should have its date column blank.
    lines := strings.Split(out, "\n")
    var pristineLine string
    for _, l := range lines { if strings.Contains(l, "pristine") { pristineLine = l } }
    if strings.HasPrefix(pristineLine, "2026-05-04") {
        t.Errorf("pristine line should have blank date column, got %q", pristineLine)
    }
}

func TestList_FilterArchived(t *testing.T) {
    items := []weekDraftListItem{
        {WeekStart: "2026-04-12", Name: "default", SyncState: "clean", Archived: false},
        {WeekStart: "2026-04-05", Name: "default", SyncState: "clean", Archived: true},
    }
    visible := filterArchived(items, false)
    if len(visible) != 1 || visible[0].WeekStart != "2026-04-12" {
        t.Errorf("filterArchived(false) wrong: %+v", visible)
    }
    all := filterArchived(items, true)
    if len(all) != 2 { t.Errorf("filterArchived(true) wrong: %+v", all) }
}
```

- [ ] **Step 17.2 — Implement**

Modify `internal/cli/time/week/list.go`:

1. Add `Archived bool` field to `weekDraftListItem`:

```go
type weekDraftListItem struct {
    // existing fields...
    Archived bool `json:"archived,omitempty"`
}
```

2. Populate `Archived: d.Archived` in the loop.

3. Add `--archived` flag and a `filterArchived` helper:

```go
func filterArchived(items []weekDraftListItem, includeArchived bool) []weekDraftListItem {
    if includeArchived {
        return items
    }
    out := make([]weekDraftListItem, 0, len(items))
    for _, it := range items {
        if !it.Archived {
            out = append(out, it)
        }
    }
    return out
}
```

4. Modify `writeListText` to blank the date column for repeats:

```go
func writeListText(w io.Writer, items []weekDraftListItem) {
    if len(items) == 0 {
        _, _ = fmt.Fprintln(w, "No drafts found.")
        return
    }
    _, _ = fmt.Fprintf(w, "%-12s  %-10s  %-12s  %5s  %s\n",
        "WEEK", "NAME", "STATE", "HOURS", "PULLED")
    var prevDate string
    for _, it := range items {
        dateCol := it.WeekStart
        if dateCol == prevDate { dateCol = "" }
        _, _ = fmt.Fprintf(w, "%-12s  %-10s  %-12s  %5.1f  %s\n",
            dateCol, it.Name, it.SyncState, it.TotalHours, it.PulledAt)
        prevDate = it.WeekStart
    }
}
```

5. In `runList`, apply filter before rendering:

```go
items = filterArchived(items, f.archived)
```

Add the flag in `newListCmd`:

```go
cmd.Flags().BoolVar(&f.archived, "archived", false, "include archived drafts")
```

And `archived bool` to `listFlags`.

- [ ] **Step 17.3 — Run, verify pass + commit**

```bash
go test ./... -v
git add internal/cli/time/week/
git commit -m "feat(cli): tdx time week list --archived + alternate-name grouping"
```

---

## Task 18: `history` text output — PINNED column

**Files:**
- Modify: `internal/cli/time/week/history.go`
- Modify: `internal/cli/time/week/history_test.go` (the existing test already covers PINNED)

- [ ] **Step 18.1 — Verify (already covered in Phase A)**

The Phase A test `TestRenderHistory_TextRows` already asserts that `pinned == "yes"` appears in text output. Verify the output already uses a PINNED column. If yes, this task is a no-op verification step. If the column header is missing or misaligned, fix.

```bash
go test ./internal/cli/time/week/ -run TestRenderHistory -v
```
Expected: PASS.

- [ ] **Step 18.2 — Commit (if changes were needed)**

If no changes needed, skip the commit. If column adjustments were needed, commit:

```bash
git add internal/cli/time/week/history.go
git commit -m "fix(cli): align PINNED column in history text output"
```

---

## Task 19: MCP — `create_week_draft`

**Files:**
- Modify: `internal/mcp/tools_drafts.go`
- Modify: `internal/mcp/tools_drafts_test.go`
- Modify: `internal/mcp/server_test.go` (bump tool count)

- [ ] **Step 19.1 — Implement**

Append to `internal/mcp/tools_drafts.go`:

```go
type createDraftArgs struct {
    Profile      string `json:"profile,omitempty"`
    WeekStart    string `json:"weekStart"`
    Name         string `json:"name,omitempty"`
    From         string `json:"from,omitempty" jsonschema:"blank | template:<n> | draft:<date>[/<n>]"`
    ShiftDays    int    `json:"shiftDays,omitempty"`
    Confirm      bool   `json:"confirm"`
}

// register in RegisterDraftMutatingTools:
sdkmcp.AddTool(srv, &sdkmcp.Tool{
    Name: "create_week_draft",
    Description: `Create a new week draft. From values:
  blank             - empty draft
  template:<name>   - seed rows from a template
  draft:<date>      - clone from another draft (default name)
  draft:<date>/<n>  - clone from a specifically-named draft

Optional shiftDays adjusts the source's WeekStart when from=draft:<...>.
Requires confirm=true. Refuses to overwrite an existing draft at the same
(profile, weekStart, name).`,
}, createDraftHandler(svcs))
```

```go
func createDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, createDraftArgs) (*sdkmcp.CallToolResult, any, error) {
    return func(ctx context.Context, req *sdkmcp.CallToolRequest, args createDraftArgs) (*sdkmcp.CallToolResult, any, error) {
        if r, ok := confirmGate(args.Confirm, "Set confirm=true to create the draft."); !ok {
            return r, nil, nil
        }
        profile := resolveProfile(svcs, args.Profile)
        weekStart, err := parseWeekStart(args.WeekStart)
        if err != nil { return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil }
        name := args.Name
        if name == "" { name = "default" }

        var draft domain.WeekDraft
        switch {
        case args.From == "" || args.From == "blank":
            draft, err = svcs.Drafts.NewBlank(profile, weekStart, name)
        case strings.HasPrefix(args.From, "template:"):
            tname := strings.TrimPrefix(args.From, "template:")
            tmpl, terr := svcs.Template.Store().Load(profile, tname)
            if terr != nil { return errorResult(fmt.Sprintf("load template: %v", terr)), nil, nil }
            draft, err = svcs.Drafts.NewFromTemplate(profile, weekStart, name, tmpl)
        case strings.HasPrefix(args.From, "draft:"):
            ref := strings.TrimPrefix(args.From, "draft:")
            srcDate, srcName, perr := parseDraftRefMCP(ref)
            if perr != nil { return errorResult(fmt.Sprintf("from: %v", perr)), nil, nil }
            if args.ShiftDays != 0 {
                srcDate = srcDate.AddDate(0, 0, -args.ShiftDays)
            }
            draft, err = svcs.Drafts.NewFromDraft(profile, weekStart, name, profile, srcDate, srcName)
        default:
            return errorResult(fmt.Sprintf("unknown from value: %q", args.From)), nil, nil
        }
        if err != nil { return errorResult(fmt.Sprintf("create: %v", err)), nil, nil }

        return jsonResult(struct {
            Schema string           `json:"schema"`
            Draft  domain.WeekDraft `json:"draft"`
        }{Schema: "tdx.v1.weekDraftCreateResult", Draft: draft})
    }
}

// parseDraftRefMCP duplicates ParseDraftRef without the cli/week import.
func parseDraftRefMCP(s string) (time.Time, string, error) {
    var dateStr, name string
    if i := strings.IndexByte(s, '/'); i >= 0 {
        dateStr, name = s[:i], s[i+1:]
        if name == "" { return time.Time{}, "", fmt.Errorf("empty name after slash") }
    } else {
        dateStr, name = s, "default"
    }
    d, err := time.ParseInLocation("2006-01-02", dateStr, domain.EasternTZ)
    if err != nil { return time.Time{}, "", err }
    return domain.WeekRefContaining(d).StartDate, name, nil
}
```

Add `"strings"` import.

Bump `wantCount` in `server_test.go` from 27 to 28. Add a confirm-gate test in `tools_drafts_test.go`:

```go
func TestCreateDraft_ConfirmGate(t *testing.T) {
    h := mcpHarness(t)
    handler := createDraftHandler(h.svcs)
    res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, createDraftArgs{WeekStart: "2026-05-04", Confirm: false})
    if err != nil { t.Fatal(err) }
    if !res.IsError { t.Errorf("expected error result for confirm=false") }
}
```

- [ ] **Step 19.2 — Run, verify pass + commit**

```bash
go test ./internal/mcp/... -v
git add internal/mcp/
git commit -m "feat(mcp): create_week_draft (blank/template/draft)"
```

---

## Task 20: MCP — copy / rename / reset (3 mutating tools)

**Files:**
- Modify: `internal/mcp/tools_drafts.go`
- Modify: `internal/mcp/server_test.go` (bump count)

- [ ] **Step 20.1 — Implement**

Append three handlers + registrations to `internal/mcp/tools_drafts.go`:

```go
type copyDraftArgs struct {
    Profile string `json:"profile,omitempty"`
    Src     string `json:"src" jsonschema:"<date>[/<name>]"`
    Dst     string `json:"dst" jsonschema:"<date>[/<name>]"`
    Confirm bool   `json:"confirm"`
}

type renameDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart"`
    OldName   string `json:"oldName"`
    NewName   string `json:"newName"`
    Confirm   bool   `json:"confirm"`
}

type resetDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart"`
    Name      string `json:"name,omitempty"`
    Confirm   bool   `json:"confirm"`
}

// In RegisterDraftMutatingTools, add:
sdkmcp.AddTool(srv, &sdkmcp.Tool{
    Name: "copy_week_draft",
    Description: "Clone a draft from src to dst. Cells are dimensionless so cross-week copies work without rewrites. Requires confirm=true. Refuses if dst already exists.",
}, copyDraftHandler(svcs))

sdkmcp.AddTool(srv, &sdkmcp.Tool{
    Name: "rename_week_draft",
    Description: "Rename a draft (preserves snapshot history). Auto-snapshots before any file motion. Requires confirm=true.",
}, renameDraftHandler(svcs))

sdkmcp.AddTool(srv, &sdkmcp.Tool{
    Name: "reset_week_draft",
    Description: "Discard local edits and re-pull from TD. Auto-snapshots first. Requires confirm=true.",
}, resetDraftHandler(svcs))
```

Implementations follow the same shape as Task 19's createDraftHandler — call `svcs.Drafts.Copy`, `Rename`, or `Reset`. Each guards `confirmGate` first.

Bump `wantCount` from 28 to 31 in `server_test.go`.

- [ ] **Step 20.2 — Test + commit**

```bash
go test ./internal/mcp/... -v
git add internal/mcp/
git commit -m "feat(mcp): copy/rename/reset week draft tools"
```

---

## Task 21: MCP — archive / unarchive (2 mutating tools)

Same pattern as Task 20. Implement `archiveDraftArgs` (with `Profile`, `WeekStart`, `Name`, `Confirm`) and dispatch to `svcs.Drafts.SetArchived(... true|false)`. Bump `wantCount` to 33.

```bash
git add internal/mcp/
git commit -m "feat(mcp): archive/unarchive week draft tools"
```

---

## Task 22: MCP — snapshot / restore_snapshot / prune_snapshots + list_week_draft_snapshots

**Files:**
- Modify: `internal/mcp/tools_drafts.go`
- Modify: `internal/mcp/server_test.go`

- [ ] **Step 22.1 — Implement**

Three mutating tools (`snapshot_week_draft`, `restore_week_draft_snapshot`, `prune_week_draft_snapshots`) and one read tool (`list_week_draft_snapshots`).

Read tool (`list_week_draft_snapshots`) reads via `svcs.Drafts.Snapshots().List(profile, weekStart, name)` and returns `tdx.v1.weekDraftSnapshotList` (existing schema). No confirm needed.

Mutating tools:
- `snapshot_week_draft` (args: WeekStart, Name, Keep bool, Note string, Confirm) → `Snapshots().Take(... OpManual, note)` then optionally `Pin`.
- `restore_week_draft_snapshot` (args: WeekStart, Name, Sequence int, Confirm) → `Drafts.RestoreSnapshot(... seq)`.
- `prune_week_draft_snapshots` (args: WeekStart, Name, OlderThanDays int, Confirm) → `Snapshots().PruneOlderThan` if OlderThanDays > 0, else `PruneToRetention`.

Bump `wantCount` to 37. Add confirm-gate tests for each mutating tool.

- [ ] **Step 22.2 — Test + commit**

```bash
go test ./internal/mcp/... -v
git add internal/mcp/
git commit -m "feat(mcp): snapshot/restore/prune + list_week_draft_snapshots tools"
```

---

## Task 23: MCP — `archived` filter on `list_week_drafts`

**Files:**
- Modify: `internal/mcp/tools_drafts.go`

- [ ] **Step 23.1 — Implement**

Add `Archived bool` to `listDraftsArgs`. In `listDraftsHandler`, after building items, filter:

```go
if !args.Archived {
    out := make([]item, 0, len(items))
    for _, it := range items {
        if !it.Archived { out = append(out, it) }
    }
    items = out
}
```

Add `Archived bool` to the item struct.

- [ ] **Step 23.2 — Commit**

```bash
go test ./internal/mcp/... -v
git add internal/mcp/
git commit -m "feat(mcp): list_week_drafts adds archived filter"
```

---

## Task 24: README + guide.md updates

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`

- [ ] **Step 24.1 — README updates**

Extend the **Time Week Drafts** table with the new commands:

```markdown
| `tdx time week new <date>` | Create blank/template-seeded/cloned draft | `--from-template`, `--from-draft`, `--shift`, `--name` |
| `tdx time week copy <src> <dst>` | Clone a draft to a new ref | (positional) |
| `tdx time week rename <date>[/<old>] <new>` | Rename a draft (preserves snapshots) | (positional) |
| `tdx time week reset <date>[/<name>] --yes` | Discard local edits + re-pull (auto-snapshots) | `--yes` |
| `tdx time week archive <date>[/<name>]` | Hide draft from default `list` | (none) |
| `tdx time week unarchive <date>[/<name>]` | Show previously archived draft | (none) |
| `tdx time week snapshot <date>[/<name>]` | Take a manual snapshot | `--keep`, `--note` |
| `tdx time week restore <date>[/<name>] --snapshot N --yes` | Restore from snapshot | `--snapshot`, `--yes` |
| `tdx time week prune <date>[/<name>] --yes` | Drop unpinned snapshots | `--older-than`, `--yes` |
```

Update the `tdx time week list` row to mention `--archived`. Add the 9 new MCP tools to the MCP table. Bump tool count to ~37.

- [ ] **Step 24.2 — guide.md — three new subsections**

In `docs/guide.md`, inside the "Week drafts" section, add:

1. **"Multiple drafts per week"** — `--name`, `pull --name`, `new --from-template`, `new --from-draft --shift 7d`, `copy <src> <dst>`, `rename`, `list` grouping.
2. **"Snapshots & history"** — auto-snapshots from Phase A (recap: pre-pull, pre-push, pre-delete), manual `snapshot --keep --note`, `restore --snapshot N`, `prune --older-than 30d`.
3. **"Archive & unarchive"** — soft-archive semantics, `--archived` filter on `list`.

- [ ] **Step 24.3 — Commit**

```bash
git add README.md docs/guide.md
git commit -m "docs: Phase B.1 (alternates, acquisition, snapshots) user docs"
```

---

## Task 25: Manual walkthrough doc

**Files:**
- Create: `docs/manual-tests/phase-B1-week-drafts-walkthrough.md`

- [ ] **Step 25.1 — Write the walkthrough**

Mirror the Phase A walkthrough format. Sections (each with goal + commands + expected output / verify):

1. Setup (auth, choose week with entries).
2. Pull default + alternate (`pull <date> --name pristine`).
3. Verify list shows both alternates grouped under same date.
4. Edit default (use `set` for non-interactive); verify pristine untouched.
5. `new <next-week> --from-draft <this-week>/default --shift 7d`; verify rows cloned, sourceEntryIDs cleared.
6. `copy <date>/default <date>/experiment`; verify pulled-snapshot followed.
7. `rename <date>/experiment trial`; verify snapshot history followed (`history` shows pre-rename).
8. Manual `snapshot --keep --note "before risky edit"`; verify pinned in `history`.
9. Edit; `restore --snapshot N --yes`; verify content reverted.
10. `prune --older-than 30d --yes` on a draft with no old snapshots; expect "Pruned 0".
11. `archive <date>/trial`; verify hidden from `list`; `--archived` shows it.
12. `unarchive`; verify visible again.
13. `reset <date>/default --yes`; verify pre-reset snapshot taken.
14. MCP smoke test exercising the new tools (`create_week_draft`, `copy_week_draft`, `rename_week_draft`, `archive_week_draft`, `snapshot_week_draft`, `list_week_draft_snapshots`, etc).
15. Sign-off block (matching Phase A's format).

- [ ] **Step 25.2 — Commit**

```bash
git add docs/manual-tests/phase-B1-week-drafts-walkthrough.md
git commit -m "docs(manual-tests): Phase B.1 walkthrough"
```

---

## Task 26: Final verification + version bump + PR

- [ ] **Step 26.1 — Full local verification**

```bash
go test ./...
go vet ./...
golangci-lint run ./...
go build ./cmd/tdx
```
All four must succeed.

- [ ] **Step 26.2 — Manual walkthrough**

Run `docs/manual-tests/phase-B1-week-drafts-walkthrough.md` against the live UFL tenant. Sign off the bottom block.

- [ ] **Step 26.3 — Push branch + open PR**

```bash
git push -u origin phase-B1-week-drafts
gh pr create --title "Phase B.1 — Week Drafts: alternates, acquisition, snapshot polish" \
  --body-file <(echo "...PR body summarizing changes; reference the spec...")
```

- [ ] **Step 26.4 — After merge: tag v0.5.0**

```bash
# After PR merges:
git checkout main
git pull
git tag -a v0.5.0 -m "Phase B.1 — Week Drafts alternates, acquisition, snapshot polish"
git push origin v0.5.0
```

The `release.yml` workflow auto-builds binaries and updates the Homebrew tap.
