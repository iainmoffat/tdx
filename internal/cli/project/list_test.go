package project

import (
	"bytes"
	"context"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestList_DefaultRendersTable(t *testing.T) {
	stub := &stubProjectsvc{
		plans: []domain.ProjectPlan{
			{ID: 1292, ProjectID: 259, ProjectName: "Disaster Recovery", Title: "FY2026 Plan",
				Type: domain.PlanWaterfall, MyTaskCount: 3, TaskCount: 12},
			{ID: 1300, ProjectID: 260, ProjectName: "Alpha Project", Title: "Alpha Waterfall",
				Type: domain.PlanWaterfall, MyTaskCount: 0, TaskCount: 5},
		},
	}
	var buf bytes.Buffer
	err := runProjectList(context.Background(), &buf, stub, "default", 50, false)
	require.NoError(t, err)
	out := buf.String()
	// Should contain project IDs
	require.Contains(t, out, "259")
	require.Contains(t, out, "260")
	// Should contain plan titles
	require.Contains(t, out, "FY2026 Plan")
	// Should contain MY-TASKS column header
	require.Contains(t, out, "MY-TASKS")
	// Should contain plan type
	require.Contains(t, out, "waterfall")
}

func TestList_JSONEnvelope(t *testing.T) {
	stub := &stubProjectsvc{
		plans: []domain.ProjectPlan{
			{ID: 1292, ProjectID: 259, ProjectName: "DR", Title: "Plan A", Type: domain.PlanWaterfall},
		},
	}
	var buf bytes.Buffer
	err := runProjectList(context.Background(), &buf, stub, "default", 50, true)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, `tdx.v1.projectPlanList`)
	require.Contains(t, out, `1292`)
	require.Contains(t, out, `259`)
}

func TestList_Empty(t *testing.T) {
	stub := &stubProjectsvc{plans: nil}
	var buf bytes.Buffer
	err := runProjectList(context.Background(), &buf, stub, "default", 50, false)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no projects found")
}

func TestList_RespectsLimit(t *testing.T) {
	plans := make([]domain.ProjectPlan, 10)
	for i := range plans {
		plans[i] = domain.ProjectPlan{ID: i + 1, ProjectID: 100 + i, ProjectName: "P", Title: "T"}
	}
	stub := &stubProjectsvc{plans: plans}
	var buf bytes.Buffer
	err := runProjectList(context.Background(), &buf, stub, "default", 3, false)
	require.NoError(t, err)
	out := buf.String()
	// Only 3 rows (+ header) — check by counting occurrences of waterfall (type 0 = unknown(0))
	// Just check it doesn't contain plan ID 4+
	require.NotContains(t, out, " 4 ") // 4th plan ID would appear
}
