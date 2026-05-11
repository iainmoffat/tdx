package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectPlanType_String(t *testing.T) {
	require.Equal(t, "waterfall", PlanWaterfall.String())
	require.Equal(t, "cardwall", PlanCardwall.String())
	require.Equal(t, "unknown(0)", ProjectPlanType(0).String())
	require.Equal(t, "unknown(99)", ProjectPlanType(99).String())
}

func TestProjectTask_AssignedTo_CaseInsensitive(t *testing.T) {
	me := "aaaaaaaa-1234-5678-9abc-def012345678"
	task := ProjectTask{
		Resources: []ProjectTaskResource{
			{UID: "ABCD1234-AAAA-BBBB-CCCC-DDDDEEEEFFFF", FullName: "Someone"},
			{UID: strings.ToUpper(me), FullName: "Me"},
		},
	}
	require.True(t, task.AssignedTo(me))
	require.True(t, task.AssignedTo(strings.ToUpper(me)))
	require.False(t, task.AssignedTo("nobody"))
	require.False(t, task.AssignedTo(""))
}

func TestProjectTask_AssignedTo_EmptyResources(t *testing.T) {
	task := ProjectTask{}
	require.False(t, task.AssignedTo("anything"))
}
