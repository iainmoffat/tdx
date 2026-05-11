package project

import (
	"bytes"
	"context"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// stubTimesvc implements timesvcLogAPI for log tests.
type stubTimesvc struct {
	types  []domain.TimeType
	entry  domain.TimeEntry
	err    error
	lastIn domain.EntryInput
}

func (s *stubTimesvc) TimeTypesForTarget(_ context.Context, _ string, _ domain.Target) ([]domain.TimeType, error) {
	return s.types, s.err
}
func (s *stubTimesvc) AddEntry(_ context.Context, _ string, in domain.EntryInput) (domain.TimeEntry, error) {
	s.lastIn = in
	return s.entry, s.err
}

func TestLog_RequiresYes(t *testing.T) {
	cmd := newLogCmd(nil)
	cmd.SetArgs([]string{"259", "4938", "--plan", "1292", "--hours", "0.5", "--type", "Work", "--profile", "default"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--yes")
}

func TestLog_RequiresPlan(t *testing.T) {
	cmd := newLogCmd(nil)
	cmd.SetArgs([]string{"259", "4938", "--hours", "0.5", "--type", "Work", "--yes"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--plan")
}

func TestLog_HoursMinutesMutex(t *testing.T) {
	svc := &stubTimesvc{types: []domain.TimeType{{ID: 1, Name: "Work"}}}
	var buf bytes.Buffer
	err := runProjectLog(context.Background(), &buf, svc, "default", "uid-me",
		259, 1292, 4938, 0.5, 30, "Work", 0, "", "", false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestLog_TypeMutex(t *testing.T) {
	svc := &stubTimesvc{types: []domain.TimeType{{ID: 1, Name: "Work"}}}
	var buf bytes.Buffer
	err := runProjectLog(context.Background(), &buf, svc, "default", "uid-me",
		259, 1292, 4938, 0.5, 0, "Work", 1, "", "", false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestLog_BuildsTargetProjectTask(t *testing.T) {
	svc := &stubTimesvc{
		types: []domain.TimeType{{ID: 1, Name: "Work", Billable: false}},
		entry: domain.TimeEntry{ID: 999},
	}
	var buf bytes.Buffer
	err := runProjectLog(context.Background(), &buf, svc, "default", "uid-me",
		259, 1292, 4938, 0.25, 0, "Work", 0, "", "", false, false)
	require.NoError(t, err)

	tgt := svc.lastIn.Target
	require.Equal(t, domain.TargetProjectTask, tgt.Kind)
	require.Equal(t, 4938, tgt.ItemID)
	require.Equal(t, 4938, tgt.TaskID)
	require.Equal(t, 1292, tgt.PlanID)
	require.Equal(t, 259, tgt.ProjectID)
}

func TestLog_HappyPath(t *testing.T) {
	svc := &stubTimesvc{
		types: []domain.TimeType{{ID: 1, Name: "Work", Billable: false}},
		entry: domain.TimeEntry{ID: 999},
	}
	var buf bytes.Buffer
	err := runProjectLog(context.Background(), &buf, svc, "default", "uid-me",
		259, 1292, 4938, 0.5, 0, "Work", 0, "", "test entry", false, false)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "999")
	require.Contains(t, out, "4938")
	require.Equal(t, "test entry", svc.lastIn.Description)
	require.Equal(t, "uid-me", svc.lastIn.UserUID)
}

func TestLog_TypeIDResolution(t *testing.T) {
	svc := &stubTimesvc{
		types: []domain.TimeType{{ID: 7, Name: "Development", Billable: true}},
		entry: domain.TimeEntry{ID: 100},
	}
	var buf bytes.Buffer
	err := runProjectLog(context.Background(), &buf, svc, "default", "uid-me",
		259, 1292, 4938, 0.5, 0, "", 7, "", "", false, false)
	require.NoError(t, err)
	require.Equal(t, 7, svc.lastIn.TimeTypeID)
}

func TestLog_BillableFlag(t *testing.T) {
	svc := &stubTimesvc{
		types: []domain.TimeType{{ID: 1, Name: "Work", Billable: false}},
		entry: domain.TimeEntry{ID: 1},
	}
	var buf bytes.Buffer
	err := runProjectLog(context.Background(), &buf, svc, "default", "uid-me",
		259, 1292, 4938, 0.5, 0, "Work", 0, "", "", true, true)
	require.NoError(t, err)
	require.True(t, svc.lastIn.Billable)
}
