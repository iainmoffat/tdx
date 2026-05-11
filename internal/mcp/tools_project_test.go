package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// TestRegisterProjectTools_NoPanic verifies that registering all project tools
// does not panic with an empty Services{}.
func TestRegisterProjectTools_NoPanic(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterProjectTools(srv, Services{})
	RegisterProjectMutatingTools(srv, Services{})
}

// TestListMyProjects_SchemaName verifies the projectPlanList schema envelope.
func TestListMyProjects_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/projects/list":
			_, _ = w.Write([]byte(`[
				{"ID":10,"ProjectID":259,"ProjectName":"Portal Rewrite","Title":"Phase 1","PlanType":1,"TaskCount":5,"MyTaskCount":2}
			]`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpHarness(t, ts.URL)
	res, _, err := listMyProjectsHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listMyProjectsArgs{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.projectPlanList", got["schema"])
	plans, ok := got["plans"].([]any)
	require.True(t, ok)
	require.Len(t, plans, 1)
	first, ok := plans[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Phase 1", first["title"])
	require.Equal(t, float64(259), first["projectID"])
}

// TestSearchProjects_SchemaName verifies the projectList schema envelope.
func TestSearchProjects_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/projects/search":
			_, _ = w.Write([]byte(`[
				{"ID":259,"Name":"Portal Rewrite","StatusName":"In Progress","IsActive":true}
			]`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpHarness(t, ts.URL)
	res, _, err := searchProjectsHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, searchProjectsArgs{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.projectList", got["schema"])
	projects, ok := got["projects"].([]any)
	require.True(t, ok)
	require.Len(t, projects, 1)
	first, ok := projects[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Portal Rewrite", first["name"])
}

// TestGetProject_SchemaName verifies the project schema envelope.
func TestGetProject_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/projects/259":
			_, _ = w.Write([]byte(`{"ID":259,"Name":"Portal Rewrite","StatusName":"In Progress"}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpHarness(t, ts.URL)
	res, _, err := getProjectHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, getProjectArgs{ID: 259})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.project", got["schema"])
	project, ok := got["project"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Portal Rewrite", project["name"])
	require.Equal(t, float64(259), project["id"])
}

// TestGetProject_MissingID verifies that id=0 returns an error result.
func TestGetProject_MissingID(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost:0")
	res, _, err := getProjectHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, getProjectArgs{ID: 0})
	require.NoError(t, err)
	require.True(t, res.IsError, "id=0 should return error result")
}

// TestListProjectPlans_SchemaName verifies the projectPlanList schema for a
// single project's plans.
func TestListProjectPlans_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/projects/259/plans/search":
			_, _ = w.Write([]byte(`[
				{"ID":10,"ProjectID":259,"ProjectName":"Portal Rewrite","Title":"Phase 1","PlanType":1},
				{"ID":11,"ProjectID":259,"ProjectName":"Portal Rewrite","Title":"Phase 2","PlanType":2}
			]`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpHarness(t, ts.URL)
	res, _, err := listProjectPlansHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listProjectPlansArgs{ProjectID: 259})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.projectPlanList", got["schema"])
	plans, ok := got["plans"].([]any)
	require.True(t, ok)
	require.Len(t, plans, 2)
}

// TestListProjectPlans_MissingProjectID verifies that projectID=0 returns an
// error result.
func TestListProjectPlans_MissingProjectID(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost:0")
	res, _, err := listProjectPlansHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listProjectPlansArgs{ProjectID: 0})
	require.NoError(t, err)
	require.True(t, res.IsError)
}

// TestListProjectTasks_SchemaName verifies the projectTaskList schema for
// single-plan mode.
func TestListProjectTasks_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/projects/259/plans/10/tasks":
			_, _ = w.Write([]byte(`[
				{"ID":1292,"Title":"Implement login","ProjectID":259,"PlanID":10,"Status":"In Progress","EstimatedHours":8}
			]`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpHarness(t, ts.URL)
	res, _, err := listProjectTasksHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listProjectTasksArgs{
		ProjectID: 259,
		PlanID:    10,
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.projectTaskList", got["schema"])
	tasks, ok := got["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, tasks, 1)
	first, ok := tasks[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Implement login", first["title"])
	require.Equal(t, float64(1292), first["taskID"])
}

// TestListProjectTasks_MineXORPlanID verifies that mine=true with projectID
// or planID returns an error result.
func TestListProjectTasks_MineXORPlanID(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost:0")
	res, _, err := listProjectTasksHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listProjectTasksArgs{
		Mine:      true,
		ProjectID: 259,
	})
	require.NoError(t, err)
	require.True(t, res.IsError, "mine=true with projectID should be an error")
}

// TestListProjectTasks_NeitherMineNorPlan verifies that omitting both mine
// and projectID+planID returns an error result.
func TestListProjectTasks_NeitherMineNorPlan(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost:0")
	res, _, err := listProjectTasksHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listProjectTasksArgs{})
	require.NoError(t, err)
	require.True(t, res.IsError, "missing both mine and projectID+planID should be an error")
}

// TestGetProjectTask_SchemaName verifies the projectTask schema envelope.
func TestGetProjectTask_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/projects/259/plans/10/tasks/1292":
			_, _ = w.Write([]byte(`{"ID":1292,"Title":"Implement login","ProjectID":259,"PlanID":10,"Status":"In Progress","EstimatedHours":8,"Resources":[{"ResourceUID":"ABC-123","ResourceFullName":"Jane Dev"}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpHarness(t, ts.URL)
	res, _, err := getProjectTaskHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, getProjectTaskArgs{
		ProjectID: 259,
		PlanID:    10,
		TaskID:    1292,
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.projectTask", got["schema"])
	task, ok := got["task"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Implement login", task["title"])
	resources, ok := task["resources"].([]any)
	require.True(t, ok)
	require.Len(t, resources, 1)
}

// TestGetProjectTask_MissingIDs verifies that missing required IDs return
// error results.
func TestGetProjectTask_MissingIDs(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost:0")
	res, _, err := getProjectTaskHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, getProjectTaskArgs{
		ProjectID: 259, // missing planID and taskID
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
}

// TestLogProjectTaskTime_RequiresConfirm verifies the confirm gate.
func TestLogProjectTaskTime_RequiresConfirm(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost:0")
	res, _, err := logProjectTaskTimeHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, logProjectTaskTimeArgs{
		ProjectID: 259,
		PlanID:    10,
		TaskID:    1292,
		Hours:     2.0,
		TypeID:    5,
		Confirm:   false,
	})
	require.NoError(t, err)
	require.True(t, res.IsError, "should be error result when confirm=false")
}

// TestLogProjectTaskTime_MissingIDs verifies that missing projectID/planID/taskID
// returns an error result (after confirm gate passes).
func TestLogProjectTaskTime_MissingIDs(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost:0")
	res, _, err := logProjectTaskTimeHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, logProjectTaskTimeArgs{
		ProjectID: 259,
		// missing PlanID and TaskID
		Hours:   1.0,
		TypeID:  5,
		Confirm: true,
	})
	require.NoError(t, err)
	require.True(t, res.IsError, "missing planID and taskID should return error")
}
