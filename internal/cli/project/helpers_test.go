package project

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestFormatDate(t *testing.T) {
	require.Equal(t, "2026-05-01", formatDate(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)))
	require.Equal(t, "", formatDate(time.Time{}))
}

func TestTruncate(t *testing.T) {
	require.Equal(t, "hello", truncate("hello", 10))
	require.Equal(t, "hello", truncate("hello", 5))  // exactly at limit: no truncation
	require.Equal(t, "hell…", truncate("hello!", 5)) // over limit: truncate
	require.Equal(t, "hi", truncate("hi", 2))
}

func TestPrintProjectList_Table(t *testing.T) {
	var buf bytes.Buffer
	projects := []domain.Project{
		{ID: 259, Name: "Disaster Recovery", StatusName: "Executing", ManagerName: "Charlotte", PercentComplete: 96.0},
	}
	err := printProjectList(&buf, projects, false, "tdx.v1.projectList")
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "259")
	require.Contains(t, out, "Disaster Recovery")
	require.Contains(t, out, "Charlotte")
}

func TestPrintProjectList_JSON(t *testing.T) {
	var buf bytes.Buffer
	projects := []domain.Project{
		{ID: 259, Name: "DR"},
	}
	err := printProjectList(&buf, projects, true, "tdx.v1.projectList")
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, `tdx.v1.projectList`)
	require.Contains(t, out, `259`)
}

func TestPrintPlanList_Table(t *testing.T) {
	var buf bytes.Buffer
	plans := []domain.ProjectPlan{
		{ID: 1292, ProjectID: 259, ProjectName: "DR", Title: "FY2026 Plan", Type: domain.PlanWaterfall, MyTaskCount: 3},
	}
	err := printPlanList(&buf, plans, false)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "1292")
	require.Contains(t, out, "waterfall")
	require.Contains(t, out, "3")
}

func TestPrintTaskList_Table(t *testing.T) {
	var buf bytes.Buffer
	tasks := []domain.ProjectTask{
		{
			ProjectID: 259, PlanID: 1292, ID: 4938,
			Title: "Configure backups", Status: "InProcess",
			Resources: []domain.ProjectTaskResource{{UID: "uid-a", FullName: "Alice"}},
		},
	}
	err := printTaskList(&buf, tasks, false)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "4938")
	require.Contains(t, out, "Configure backups")
	require.Contains(t, out, "Alice")
}

func TestPrintPlanList_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := printPlanList(&buf, nil, false)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no plans found")
}

func TestAssigneeNames(t *testing.T) {
	res := []domain.ProjectTaskResource{
		{UID: "a", FullName: "Alice"},
		{UID: "b", FullName: "Bob"},
	}
	got := assigneeNames(res)
	require.True(t, strings.Contains(got, "Alice"))
	require.True(t, strings.Contains(got, "Bob"))
}
