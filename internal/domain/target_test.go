package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetKind_IsKnown(t *testing.T) {
	known := []TargetKind{
		TargetTicket, TargetTicketTask,
		TargetProject, TargetProjectTask, TargetProjectIssue,
		TargetWorkspace, TargetTimeOff, TargetPortfolio, TargetRequest,
	}
	for _, k := range known {
		require.True(t, k.IsKnown(), "expected %q to be known", k)
	}
	require.False(t, TargetKind("nonsense").IsKnown())
}

func TestTargetKind_SupportsComponentLookup(t *testing.T) {
	// These kinds have a /TDWebApi/api/time/types/component/... endpoint.
	supported := []TargetKind{
		TargetTicket, TargetTicketTask,
		TargetProject, TargetProjectTask, TargetProjectIssue,
		TargetWorkspace, TargetTimeOff, TargetRequest,
	}
	for _, k := range supported {
		require.True(t, k.SupportsComponentLookup(),
			"expected %q to support component lookup", k)
	}
	// Portfolio has no /component/portfolio/ endpoint.
	require.False(t, TargetPortfolio.SupportsComponentLookup())
	require.False(t, TargetKind("nonsense").SupportsComponentLookup())
}

func TestTarget_Validate(t *testing.T) {
	cases := []struct {
		name    string
		target  Target
		wantErr bool
	}{
		{"valid ticket", Target{Kind: TargetTicket, AppID: 42, ItemID: 12345}, false},
		{"missing kind", Target{AppID: 42, ItemID: 12345}, true},
		{"unknown kind", Target{Kind: "bogus", AppID: 42, ItemID: 12345}, true},
		{"missing appID", Target{Kind: TargetTicket, ItemID: 12345}, true},
		{"missing itemID", Target{Kind: TargetTicket, AppID: 42}, true},
		{"ticket task requires taskID", Target{Kind: TargetTicketTask, AppID: 42, ItemID: 12345}, true},
		{"ticket task valid", Target{Kind: TargetTicketTask, AppID: 42, ItemID: 12345, TaskID: 7}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.target.Validate()
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidTarget)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
