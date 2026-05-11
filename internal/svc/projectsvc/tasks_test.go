package projectsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListTasks_DecodesTasksWithResources(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/projects/259/plans/1292/tasks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{
				"ID": 4938, "Title": "Configure backup replication",
				"ProjectID": 259, "PlanID": 1292,
				"Status": "InProcess", "StatusID": 2,
				"PercentComplete": 50.0, "EstimatedHours": 8.0, "ActualHours": 4.0,
				"StartDateUtc": "2026-05-01T00:00:00", "EndDateUtc": "2026-05-15T00:00:00",
				"Resources": [
					{"ResourceUID": "AAAA-BBBB-CCCC", "ResourceFullName": "Alice", "PercentAssignedWhole": 100}
				]
			},
			{
				"ID": 4939, "Title": "Parent task", "ProjectID": 259, "PlanID": 1292,
				"IsParent": true, "IndentLevel": 0, "OutlineNumber": 1, "Wbs": "1"
			}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	tasks, err := svc.ListTasks(context.Background(), prof, 259, 1292)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	t0 := tasks[0]
	require.Equal(t, 4938, t0.ID)
	require.Equal(t, 259, t0.ProjectID)
	require.Equal(t, 1292, t0.PlanID)
	require.Equal(t, "InProcess", t0.Status)
	require.Equal(t, 50.0, t0.PercentComplete)
	require.Len(t, t0.Resources, 1)
	require.Equal(t, "AAAA-BBBB-CCCC", t0.Resources[0].UID)
	require.Equal(t, "Alice", t0.Resources[0].FullName)
	require.False(t, t0.StartDate.IsZero())
	require.True(t, tasks[1].IsParent)
}

func TestListTasks_FallsBackToCallerIDs(t *testing.T) {
	// When wire task has ProjectID/PlanID=0, use caller-supplied IDs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 100, "Title": "Task A"}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	tasks, err := svc.ListTasks(context.Background(), prof, 259, 1292)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, 259, tasks[0].ProjectID)
	require.Equal(t, 1292, tasks[0].PlanID)
}

func TestGetTask_FetchesSingleTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/tasks/4938") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"ID": 4938, "Title": "Configure backup replication",
			"ProjectID": 259, "PlanID": 1292,
			"Description": "Set up cross-region replication"
		}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	task, err := svc.GetTask(context.Background(), prof, 259, 1292, 4938)
	require.NoError(t, err)
	require.Equal(t, 4938, task.ID)
	require.Equal(t, "Configure backup replication", task.Title)
	require.Equal(t, "Set up cross-region replication", task.Description)
}
