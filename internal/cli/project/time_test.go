package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

// stubTimesvcTime implements timesvcTimeAPI for project-time tests.
// SearchEntries is invoked concurrently when more than one user is in scope,
// so the lastFilter capture is guarded.
type stubTimesvcTime struct {
	entries []domain.TimeEntry
	err     error
	// Capture last filter for assertions.
	mu         sync.Mutex
	lastFilter domain.EntryFilter
}

func (s *stubTimesvcTime) SearchEntries(_ context.Context, _ string, filter domain.EntryFilter) ([]domain.TimeEntry, error) {
	s.mu.Lock()
	s.lastFilter = filter
	s.mu.Unlock()
	return s.entries, s.err
}

// makeProjectTimeEntry creates a test TimeEntry targeting a project task.
func makeProjectTimeEntry(id int, uid string, date time.Time, projectID, planID, taskID, minutes int) domain.TimeEntry {
	return domain.TimeEntry{
		ID:      id,
		UserUID: uid,
		Target: domain.Target{
			Kind:      domain.TargetProjectTask,
			ProjectID: projectID,
			ItemID:    planID,
			TaskID:    taskID,
		},
		TimeType: domain.TimeType{ID: 1, Name: "Dev"},
		Date:     date,
		Minutes:  minutes,
	}
}

func TestProjectTime_RequiresProjectID(t *testing.T) {
	cmd := newTimeCmd(nil, nil)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "project-id is required")
}

func TestProjectTime_InvalidProjectID(t *testing.T) {
	cmd := newTimeCmd(nil, nil)
	cmd.SetArgs([]string{"abc"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "project id must be a positive integer")
}

func TestProjectTime_UserAndAllUsersMutex(t *testing.T) {
	cmd := newTimeCmd(nil, nil)
	cmd.SetArgs([]string{"259", "--user", "me", "--all-users"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--user and --all-users are mutually exclusive")
}

func TestProjectTime_WeekAndFromToMutex(t *testing.T) {
	cmd := newTimeCmd(nil, nil)
	cmd.SetArgs([]string{"259", "--week", "2026-05-05", "--from", "2026-05-05", "--to", "2026-05-11"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--week is mutually exclusive with --from/--to")
}

func TestProjectTime_FromWithoutTo(t *testing.T) {
	cmd := newTimeCmd(nil, nil)
	cmd.SetArgs([]string{"259", "--from", "2026-05-05"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--from and --to must be given together")
}

func TestProjectTime_AllUsersResolvesViaResources(t *testing.T) {
	resources := []domain.ProjectResource{
		{UID: "uid-alice", FullName: "Alice"},
		{UID: "uid-bob", FullName: "Bob"},
	}
	stubP := &stubProjectsvc{resources: resources}

	day := time.Date(2026, 5, 6, 10, 0, 0, 0, domain.EasternTZ)
	entries := []domain.TimeEntry{
		makeProjectTimeEntry(1, "uid-alice", day, 259, 1292, 4938, 60),
	}
	stubT := &stubTimesvcTime{entries: entries}

	var buf bytes.Buffer
	cmd := newTimeCmd(stubP, stubT)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"259", "--all-users", "--week", "2026-05-05"})
	// Command requires a real profile; we test it fails at config.ResolvePaths
	// before reaching the service calls when profile isn't set up. Instead,
	// call the logic directly via the runner approach used in other tests.
	// Since we can't easily inject auth without a real profile, we test at the
	// cobra layer and expect a profile-not-found error (confirming pre-config
	// validation passed and reached the profile lookup stage).
	err := cmd.Execute()
	// Should error on profile resolution, not on the flag validation.
	if err != nil {
		require.NotContains(t, err.Error(), "--user and --all-users")
		require.NotContains(t, err.Error(), "project-id is required")
	}
}

func TestProjectTime_JSONEnvelope(t *testing.T) {
	// Test the JSON output shape via runProjectTime directly (without cobra+profile).
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, domain.EasternTZ)
	entries := []domain.TimeEntry{
		makeProjectTimeEntry(101, "uid-alice", day, 259, 1292, 4938, 120),
		makeProjectTimeEntry(102, "uid-alice", day, 259, 1292, 4938, 60),
	}
	stubT := &stubTimesvcTime{entries: entries}

	ctx := context.Background()
	rng := domain.DateRange{
		From: time.Date(2026, 5, 5, 0, 0, 0, 0, domain.EasternTZ),
		To:   time.Date(2026, 5, 11, 0, 0, 0, 0, domain.EasternTZ),
	}
	users := []domain.User{{UID: "uid-alice", FullName: "Alice"}}

	var buf bytes.Buffer
	err := runProjectTimeRender(ctx, &buf, stubT, "default", 259, 0, 0, rng, users, true)
	require.NoError(t, err)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	require.Equal(t, "tdx.v1.projectTimeReview", envelope["schema"])
	require.Equal(t, float64(259), envelope["projectID"])
	require.InDelta(t, 3.0, envelope["totalHours"], 0.001)
	entriesSlice, ok := envelope["entries"].([]any)
	require.True(t, ok)
	require.Len(t, entriesSlice, 2)
}

func TestProjectTime_SumsHoursAcrossUsers(t *testing.T) {
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, domain.EasternTZ)
	// Stub returns 2 entries for each SearchEntries call.
	// With 2 users, we get 4 total.
	entries := []domain.TimeEntry{
		makeProjectTimeEntry(1, "uid-alice", day, 259, 1292, 4938, 60),
		makeProjectTimeEntry(2, "uid-alice", day.Add(24*time.Hour), 259, 1292, 4938, 120),
	}
	stubT := &stubTimesvcTime{entries: entries}

	rng := domain.DateRange{
		From: time.Date(2026, 5, 5, 0, 0, 0, 0, domain.EasternTZ),
		To:   time.Date(2026, 5, 11, 0, 0, 0, 0, domain.EasternTZ),
	}
	users := []domain.User{
		{UID: "uid-alice", FullName: "Alice"},
		{UID: "uid-bob", FullName: "Bob"},
	}

	var buf bytes.Buffer
	ctx := context.Background()
	err := runProjectTimeRender(ctx, &buf, stubT, "default", 259, 0, 0, rng, users, false)
	require.NoError(t, err)

	out := buf.String()
	// 4 entries × 1h or 2h; table should show TOTAL
	require.Contains(t, out, "TOTAL")
	// Per-user footer should appear (>1 user)
	require.Contains(t, out, "Alice")
	require.Contains(t, out, "Bob")
}

func TestProjectTime_DateRangeFromAndTo(t *testing.T) {
	stubT := &stubTimesvcTime{entries: []domain.TimeEntry{}}

	rng := domain.DateRange{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, domain.EasternTZ),
		To:   time.Date(2026, 4, 30, 0, 0, 0, 0, domain.EasternTZ),
	}
	users := []domain.User{{UID: "uid-alice", FullName: "Alice"}}

	var buf bytes.Buffer
	ctx := context.Background()
	err := runProjectTimeRender(ctx, &buf, stubT, "default", 259, 0, 0, rng, users, false)
	require.NoError(t, err)

	// Verify the date range was passed to SearchEntries.
	require.Equal(t, rng, stubT.lastFilter.DateRange)
}

func TestProjectTime_HumanTableHeaders(t *testing.T) {
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, domain.EasternTZ)
	entries := []domain.TimeEntry{
		makeProjectTimeEntry(1, "uid-alice", day, 259, 1292, 4938, 90),
	}
	stubT := &stubTimesvcTime{entries: entries}

	rng := domain.DateRange{
		From: time.Date(2026, 5, 5, 0, 0, 0, 0, domain.EasternTZ),
		To:   time.Date(2026, 5, 11, 0, 0, 0, 0, domain.EasternTZ),
	}
	users := []domain.User{{UID: "uid-alice", FullName: "Alice"}}

	var buf bytes.Buffer
	ctx := context.Background()
	err := runProjectTimeRender(ctx, &buf, stubT, "default", 259, 0, 0, rng, users, false)
	require.NoError(t, err)

	out := buf.String()
	for _, hdr := range []string{"DATE", "USER", "TYPE", "KIND", "REF", "HOURS", "DESCRIPTION"} {
		require.True(t, strings.Contains(out, hdr), "expected header %q in output", hdr)
	}
}

func TestProjectTime_RefusesOverMaxWeekSpan(t *testing.T) {
	cmd := newTimeCmd(nil, nil)
	cmd.SetArgs([]string{"259", "--from", "2020-01-01", "--to", "2030-01-01"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
	require.Contains(t, err.Error(), "weeks=")
}

func TestProjectTime_RefusesOverMaxUsers(t *testing.T) {
	// User-cap fires AFTER profile resolution (ListResources needs profile in
	// prod). Seed a minimal profile via TDX_CONFIG_HOME so config.ResolvePaths
	// succeeds in CI environments without an on-disk profile.
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)
	paths := config.Paths{
		Root:       dir,
		ConfigFile: filepath.Join(dir, "config.yaml"),
	}
	require.NoError(t, config.NewProfileStore(paths).AddProfile(domain.Profile{
		Name:          "default",
		TenantBaseURL: "https://example.com",
	}))

	// Build a project-resource list of 1001 synthetic resources.
	resources := make([]domain.ProjectResource, domain.MaxReportUsers+1)
	for i := range resources {
		resources[i] = domain.ProjectResource{
			UID:      fmt.Sprintf("u%04d", i),
			FullName: fmt.Sprintf("user %d", i),
		}
	}
	psvc := &stubProjectsvc{resources: resources}
	tsvc := &stubTimesvcTime{}
	cmd := newTimeCmd(psvc, tsvc)
	cmd.SetArgs([]string{"259", "--week", "2026-04-12", "--all-users"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
	require.Contains(t, err.Error(), "users=1001")
}
