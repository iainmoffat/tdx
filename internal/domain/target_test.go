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
		TargetProject, TargetProjectIssue,
		TargetWorkspace, TargetTimeOff, TargetRequest,
	}
	for _, k := range supported {
		require.True(t, k.SupportsComponentLookup(),
			"expected %q to support component lookup", k)
	}
	// Portfolio has no /component/portfolio/ endpoint.
	require.False(t, TargetPortfolio.SupportsComponentLookup())
	// ProjectTask requires a PlanID not yet modelled in Target.
	require.False(t, TargetProjectTask.SupportsComponentLookup())
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
		{"project task requires taskID", Target{Kind: TargetProjectTask, AppID: 42, ItemID: 12345}, true},
		{"project task valid", Target{Kind: TargetProjectTask, AppID: 42, ItemID: 12345, TaskID: 7}, false},
		// Ticket kinds require AppID; non-ticket kinds do not.
		{"ticket requires appID", Target{Kind: TargetTicket, ItemID: 12345}, true},
		{"ticketTask requires appID", Target{Kind: TargetTicketTask, ItemID: 12345, TaskID: 7}, true},
		// Project and related kinds legitimately have AppID=0.
		{"project valid without appID", Target{Kind: TargetProject, ItemID: 100}, false},
		{"projectTask valid without appID", Target{Kind: TargetProjectTask, ItemID: 100, TaskID: 5}, false},
		{"projectIssue valid without appID", Target{Kind: TargetProjectIssue, ItemID: 100}, false},
		{"workspace valid without appID", Target{Kind: TargetWorkspace, ItemID: 100}, false},
		{"timeoff valid without appID", Target{Kind: TargetTimeOff, ItemID: 100}, false},
		{"portfolio valid without appID", Target{Kind: TargetPortfolio, ItemID: 100}, false},
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
