package timesvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestEncodeTarget_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		target domain.Target
	}{
		{
			name:   "ticket",
			target: domain.Target{Kind: domain.TargetTicket, AppID: 5, ItemID: 100},
		},
		{
			name:   "ticketTask",
			target: domain.Target{Kind: domain.TargetTicketTask, AppID: 5, ItemID: 100, TaskID: 200},
		},
		{
			name:   "project",
			target: domain.Target{Kind: domain.TargetProject, ItemID: 54},
		},
		{
			name:   "projectTask",
			target: domain.Target{Kind: domain.TargetProjectTask, ItemID: 2091, TaskID: 2612, ProjectID: 54},
		},
		{
			name:   "projectIssue",
			target: domain.Target{Kind: domain.TargetProjectIssue, ItemID: 300, ProjectID: 54},
		},
		{
			name:   "workspace",
			target: domain.Target{Kind: domain.TargetWorkspace, ItemID: 10},
		},
		{
			name:   "timeoff",
			target: domain.Target{Kind: domain.TargetTimeOff, ItemID: 10},
		},
		{
			name:   "portfolio",
			target: domain.Target{Kind: domain.TargetPortfolio, ItemID: 10},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w wireTimeEntryWrite
			err := encodeTarget(tt.target, &w)
			require.NoError(t, err)

			// Build a wireTimeEntry from the write struct to decode back.
			wire := wireTimeEntry{
				Component:   w.Component,
				TicketID:    w.TicketID,
				ProjectID:   w.ProjectID,
				PlanID:      w.PlanID,
				PortfolioID: w.PortfolioID,
				ItemID:      w.ItemID,
				AppID:       w.AppID,
			}
			got, err := decodeTarget(wire)
			require.NoError(t, err)
			require.Equal(t, tt.target.Kind, got.Kind, "kind")
			require.Equal(t, tt.target.ItemID, got.ItemID, "itemID")
			require.Equal(t, tt.target.TaskID, got.TaskID, "taskID")
			require.Equal(t, tt.target.AppID, got.AppID, "appID")
			require.Equal(t, tt.target.ProjectID, got.ProjectID, "projectID")
		})
	}
}

func TestEncodeTarget_UnsupportedKind(t *testing.T) {
	var w wireTimeEntryWrite
	err := encodeTarget(domain.Target{Kind: domain.TargetKind("bogus"), ItemID: 1}, &w)
	require.Error(t, err)
}

func TestEncodeEntryWrite(t *testing.T) {
	input := domain.EntryInput{
		UserUID:     "uid-123",
		Date:        time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Minutes:     90,
		TimeTypeID:  5,
		Billable:    true,
		Target:      domain.Target{Kind: domain.TargetProjectTask, ItemID: 2091, TaskID: 2612, ProjectID: 54},
		Description: "did some work",
	}
	w, err := encodeEntryWrite(input)
	require.NoError(t, err)
	require.Equal(t, "uid-123", w.Uid)
	require.Equal(t, "2026-04-11T00:00:00", w.TimeDate)
	require.Equal(t, 90.0, w.Minutes)
	require.Equal(t, 5, w.TimeTypeID)
	require.True(t, w.Billable)
	require.Equal(t, componentTaskTime, w.Component)
	require.Equal(t, 2091, w.PlanID)
	require.Equal(t, 2612, w.ItemID)
	require.Equal(t, 54, w.ProjectID)
	require.Equal(t, "did some work", w.Description)
}
