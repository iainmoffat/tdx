package project

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// stubDraftsvc implements draftsvcShowAPI for tests.
type stubDraftsvc struct {
	draft domain.WeekDraft
	err   error
}

func (s *stubDraftsvc) LoadDraft(_ string, _ time.Time, _ string) (domain.WeekDraft, error) {
	return s.draft, s.err
}

func TestShow_RendersProjectDetail(t *testing.T) {
	stub := &stubProjectsvc{
		project: domain.Project{
			ID: 259, Name: "Sample Recovery", StatusName: "Executing",
			ManagerName: "Pat Manager", SponsorName: "Sam Sponsor",
			PercentComplete: 96.0, IsActive: true,
			EstimatedHours: 320.0, ActualHours: 58.0,
		},
	}
	drafts := &stubDraftsvc{}
	var buf bytes.Buffer
	err := runProjectShow(context.Background(), &buf, stub, drafts, "default", 259, false)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "PROJECT 259")
	require.Contains(t, out, "Sample Recovery")
	require.Contains(t, out, "Executing")
	require.Contains(t, out, "Pat Manager")
	require.Contains(t, out, "96.0%")
}

func TestShow_JSONEnvelope(t *testing.T) {
	stub := &stubProjectsvc{
		project: domain.Project{ID: 259, Name: "DR"},
	}
	drafts := &stubDraftsvc{}
	var buf bytes.Buffer
	err := runProjectShow(context.Background(), &buf, stub, drafts, "default", 259, true)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, `tdx.v1.project`)
	require.Contains(t, out, `259`)
}

func TestShow_ThisWeekCrossover(t *testing.T) {
	stub := &stubProjectsvc{
		project: domain.Project{ID: 259, Name: "DR"},
	}
	drafts := &stubDraftsvc{
		draft: domain.WeekDraft{
			Rows: []domain.DraftRow{
				{
					Target: domain.Target{Kind: domain.TargetProjectTask, ProjectID: 259, ItemID: 1},
					Cells:  []domain.DraftCell{{Hours: 2.0}},
				},
				{
					Target: domain.Target{Kind: domain.TargetTicket, ItemID: 999},
					Cells:  []domain.DraftCell{{Hours: 1.0}},
				},
			},
		},
	}
	var buf bytes.Buffer
	err := runProjectShow(context.Background(), &buf, stub, drafts, "default", 259, false)
	require.NoError(t, err)
	out := buf.String()
	// This week crossover should show 2.00h
	require.Contains(t, out, "This week")
	require.Contains(t, out, "2.00h")
}
