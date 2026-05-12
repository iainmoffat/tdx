package project

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestTaskList_SinglePlan_RendersTable(t *testing.T) {
	stub := &stubProjectsvc{
		tasks: []domain.ProjectTask{
			{ProjectID: 259, PlanID: 1292, ID: 4938, Title: "Configure backups",
				Status: "InProcess", PercentComplete: 50,
				Resources: []domain.ProjectTaskResource{{UID: "uid-a", FullName: "Alice"}}},
		},
	}
	var buf bytes.Buffer
	err := runTaskListSinglePlan(context.Background(), &buf, stub, "default", 259, 1292, 50, false)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "4938")
	require.Contains(t, out, "Configure backups")
	require.Contains(t, out, "Alice")
	require.Equal(t, 259, stub.lastProjectID)
	require.Equal(t, 1292, stub.lastPlanID)
}

func TestTaskList_Mine_FansOutAcrossProjects(t *testing.T) {
	myUID := "aaaaaaaa-1234-5678-9abc-def012345678"
	// 3 plans: one with MyTaskCount=0 (should be skipped), two with tasks assigned to me.
	plans := []domain.ProjectPlan{
		{ID: 100, ProjectID: 10, ProjectName: "Proj A", Title: "Plan 1", MyTaskCount: 0},
		{ID: 101, ProjectID: 10, ProjectName: "Proj A", Title: "Plan 2", MyTaskCount: 2},
		{ID: 102, ProjectID: 11, ProjectName: "Proj B", Title: "Plan 3", MyTaskCount: 1},
	}

	callCounts := map[int]int{}
	tasksByPlan := map[int][]domain.ProjectTask{
		101: {
			{ProjectID: 10, PlanID: 101, ID: 200, Title: "Task A",
				Resources: []domain.ProjectTaskResource{{UID: myUID, FullName: "Me"}}},
			{ProjectID: 10, PlanID: 101, ID: 201, Title: "Task B",
				Resources: []domain.ProjectTaskResource{{UID: "uid-other", FullName: "Other"}}},
		},
		102: {
			{ProjectID: 11, PlanID: 102, ID: 202, Title: "Task C",
				Resources: []domain.ProjectTaskResource{{UID: myUID, FullName: "Me"}}},
		},
	}

	stubSvc := &callTrackingStub{
		plans:       plans,
		tasksByPlan: tasksByPlan,
		callCounts:  callCounts,
	}

	var buf bytes.Buffer
	err := runTaskListMine(context.Background(), &buf, stubSvc, "default", myUID, 50, false)
	require.NoError(t, err)

	// Plan 100 (MyTaskCount=0) should never have had ListTasks called.
	require.Equal(t, 0, callCounts[100], "plan with MyTaskCount=0 should not be fetched")
	// Plans 101 and 102 should have been fetched.
	require.Greater(t, callCounts[101]+callCounts[102], 0)

	out := buf.String()
	// My tasks should appear.
	require.Contains(t, out, "Task A")
	require.Contains(t, out, "Task C")
	// Other's task should not appear.
	require.NotContains(t, out, "Task B")
}

func TestTaskList_Mine_UpperCaseUIDMatch(t *testing.T) {
	myUID := "aaaaaaaa-1234-5678-9abc-def012345678"
	plans := []domain.ProjectPlan{
		{ID: 101, ProjectID: 10, MyTaskCount: 1},
	}
	tasksByPlan := map[int][]domain.ProjectTask{
		101: {
			{ProjectID: 10, PlanID: 101, ID: 200, Title: "My Task",
				// Resource UID is UPPERCASE — common in TD API responses.
				Resources: []domain.ProjectTaskResource{
					{UID: "AAAAAAAA-1234-5678-9ABC-DEF012345678", FullName: "Me"},
				}},
		},
	}
	stub := &callTrackingStub{plans: plans, tasksByPlan: tasksByPlan, callCounts: map[int]int{}}
	var buf bytes.Buffer
	err := runTaskListMine(context.Background(), &buf, stub, "default", myUID, 50, false)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "My Task")
}

func TestTaskList_Mine_RespectsLimit(t *testing.T) {
	myUID := "uid-me"
	plans := []domain.ProjectPlan{
		{ID: 101, ProjectID: 10, MyTaskCount: 5},
	}
	tasks := make([]domain.ProjectTask, 10)
	for i := range tasks {
		tasks[i] = domain.ProjectTask{
			ProjectID: 10, PlanID: 101, ID: 300 + i, Title: "Task",
			Resources: []domain.ProjectTaskResource{{UID: myUID, FullName: "Me"}},
		}
	}
	tasksByPlan := map[int][]domain.ProjectTask{101: tasks}
	stub := &callTrackingStub{plans: plans, tasksByPlan: tasksByPlan, callCounts: map[int]int{}}
	var buf bytes.Buffer
	err := runTaskListMine(context.Background(), &buf, stub, "default", myUID, 3, false)
	require.NoError(t, err)
	// With limit=3 we should see at most 3 task rows beyond the header.
	lines := 0
	for _, c := range buf.String() {
		if c == '\n' {
			lines++
		}
	}
	// Header + 3 task rows = 4 lines (rough check)
	require.LessOrEqual(t, lines, 5)
}

func TestTaskList_Mine_NoTasksMessage(t *testing.T) {
	plans := []domain.ProjectPlan{
		{ID: 101, ProjectID: 10, MyTaskCount: 0},
	}
	stub := &callTrackingStub{plans: plans, tasksByPlan: map[int][]domain.ProjectTask{}, callCounts: map[int]int{}}
	var buf bytes.Buffer
	err := runTaskListMine(context.Background(), &buf, stub, "default", "uid-me", 50, false)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no projects/plans assigned to you")
}

func TestTaskList_RequiresPlanWhenProjectGiven(t *testing.T) {
	cmd := newTaskListCmd(nil)
	cmd.SetArgs([]string{"259"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--plan is required")
	require.Contains(t, err.Error(), "tdx project plan list 259")
}

func TestTaskList_MineMutuallyExclusiveWithProjectID(t *testing.T) {
	cmd := newTaskListCmd(nil)
	cmd.SetArgs([]string{"259", "--mine"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestTaskList_MineMutuallyExclusiveWithPlan(t *testing.T) {
	cmd := newTaskListCmd(nil)
	cmd.SetArgs([]string{"--mine", "--plan", "1292"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestTaskShow_RendersTask(t *testing.T) {
	stub := &stubProjectsvc{
		task: domain.ProjectTask{
			ProjectID: 259, PlanID: 1292, ID: 4938,
			Title: "Configure backups", Status: "InProcess",
			EstimatedHours: 8.0, ActualHours: 4.0,
			Resources: []domain.ProjectTaskResource{
				{UID: "uid-a", FullName: "Alice", RoleName: "Engineer", PercentAssigned: 100},
			},
		},
	}
	var buf bytes.Buffer
	err := runTaskShow(context.Background(), &buf, stub, "default", 259, 1292, 4938, false)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "TASK 4938")
	require.Contains(t, out, "Configure backups")
	require.Contains(t, out, "Alice")
	require.Equal(t, 259, stub.lastProjectID)
	require.Equal(t, 1292, stub.lastPlanID)
	require.Equal(t, 4938, stub.lastTaskID)
}

func TestTaskShow_JSONEnvelope(t *testing.T) {
	stub := &stubProjectsvc{
		task: domain.ProjectTask{ProjectID: 259, PlanID: 1292, ID: 4938, Title: "T"},
	}
	var buf bytes.Buffer
	err := runTaskShow(context.Background(), &buf, stub, "default", 259, 1292, 4938, true)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, `tdx.v1.projectTask`)
	require.Contains(t, out, `4938`)
}

// callTrackingStub implements projectsvcAPI for task --mine tests.
type callTrackingStub struct {
	plans       []domain.ProjectPlan
	tasksByPlan map[int][]domain.ProjectTask
	callCounts  map[int]int
	mu          sync.Mutex
}

func (s *callTrackingStub) ListMine(_ context.Context, _ string) ([]domain.ProjectPlan, error) {
	return s.plans, nil
}
func (s *callTrackingStub) Search(_ context.Context, _ string, _ domain.ProjectSearchFilter) ([]domain.Project, error) {
	return nil, nil
}
func (s *callTrackingStub) Get(_ context.Context, _ string, _ int) (domain.Project, error) {
	return domain.Project{}, nil
}
func (s *callTrackingStub) SearchPlans(_ context.Context, _ string, _ int, _ string, _ bool) ([]domain.ProjectPlan, error) {
	return nil, nil
}
func (s *callTrackingStub) ListTasks(_ context.Context, _ string, _ int, planID int) ([]domain.ProjectTask, error) {
	s.mu.Lock()
	s.callCounts[planID]++
	s.mu.Unlock()
	return s.tasksByPlan[planID], nil
}
func (s *callTrackingStub) GetTask(_ context.Context, _ string, _, _, _ int) (domain.ProjectTask, error) {
	return domain.ProjectTask{}, nil
}
func (s *callTrackingStub) ListProjectTypes(_ context.Context, _ string, _ bool) ([]domain.ProjectType, error) {
	return nil, nil
}
func (s *callTrackingStub) ResolveTypeByName(_ context.Context, _ string, _ string) (domain.ProjectType, error) {
	return domain.ProjectType{}, nil
}
func (s *callTrackingStub) GetFeed(_ context.Context, _ string, _ int) ([]domain.ProjectFeedEntry, error) {
	return nil, nil
}
func (s *callTrackingStub) AddFeed(_ context.Context, _ string, _ int, _ string, _ bool, _ []string) (int, error) {
	return 0, nil
}
func (s *callTrackingStub) GetTaskFeed(_ context.Context, _ string, _, _, _ int) ([]domain.ProjectFeedEntry, error) {
	return nil, nil
}
func (s *callTrackingStub) AddTaskFeed(_ context.Context, _ string, _, _, _ int, _ string, _ bool, _ []string) (int, error) {
	return 0, nil
}
