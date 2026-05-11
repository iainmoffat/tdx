package project

import (
	"bytes"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPlanList_RendersTable(t *testing.T) {
	stub := &stubProjectsvc{
		plans: []domain.ProjectPlan{
			{ID: 1292, ProjectID: 259, ProjectName: "DR", Title: "FY2026 Waterfall",
				Type: domain.PlanWaterfall, MyTaskCount: 3, TaskCount: 12},
			{ID: 1300, ProjectID: 259, ProjectName: "DR", Title: "DR Cardwall",
				Type: domain.PlanCardwall, MyTaskCount: 0, TaskCount: 5},
		},
	}
	var buf bytes.Buffer
	err := runPlanList(nil, &buf, stub, "default", 259, "", false, false)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "1292")
	require.Contains(t, out, "waterfall")
	require.Contains(t, out, "cardwall")
	require.Contains(t, out, "FY2026 Waterfall")
}

func TestPlanList_JSONEnvelope(t *testing.T) {
	stub := &stubProjectsvc{
		plans: []domain.ProjectPlan{
			{ID: 1292, ProjectID: 259, ProjectName: "DR", Title: "Plan A"},
		},
	}
	var buf bytes.Buffer
	err := runPlanList(nil, &buf, stub, "default", 259, "", false, true)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, `tdx.v1.projectPlanList`)
	require.Contains(t, out, `1292`)
}

func TestPlanList_PassesNameLike(t *testing.T) {
	stub := &stubProjectsvc{plans: nil}
	var buf bytes.Buffer
	_ = runPlanList(nil, &buf, stub, "default", 259, "FY2026", false, false)
	require.Equal(t, "FY2026", stub.lastNameLike)
	require.Equal(t, 259, stub.lastProjectID)
}

func TestPlanList_Empty(t *testing.T) {
	stub := &stubProjectsvc{plans: nil}
	var buf bytes.Buffer
	err := runPlanList(nil, &buf, stub, "default", 259, "", false, false)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no plans found for project 259")
}
