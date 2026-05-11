package week

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	"github.com/stretchr/testify/require"
)

func TestNewEditCmd_FlagsRegistered(t *testing.T) {
	cmd := newEditCmd()
	require.NotNil(t, cmd)
	require.Equal(t, "edit [date[/name]]", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("web"), "--web flag missing")
	require.NotNil(t, cmd.Flags().Lookup("profile"), "--profile flag missing")
	require.NoError(t, cmd.Args(cmd, []string{}), "should accept zero args")
	require.NoError(t, cmd.Args(cmd, []string{"2026-05-04"}), "should accept one arg")
}

func TestDraftToSheet_DenseHoursFromSparseCells(t *testing.T) {
	weekStart := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	draft := domain.WeekDraft{
		Name: "default", WeekStart: weekStart,
		Rows: []domain.DraftRow{
			{
				ID:    "row-01",
				Label: "Admin",
				Target: domain.Target{
					GroupName: "Sample Department Administration", DisplayRef: "plan/2075/task/2076",
				},
				TimeType: domain.TimeType{Name: "Standard Activities"},
				Cells: []domain.DraftCell{
					{Day: time.Monday, Hours: 4, SourceEntryID: 100},
					{Day: time.Tuesday, Hours: 2}, // local-only addition
				},
			},
		},
	}

	sheet := draftToSheet(draft)
	require.Equal(t, "default", sheet.Name)
	require.Len(t, sheet.Rows, 1)
	require.Equal(t, "row-01", sheet.Rows[0].ID)
	require.Equal(t, "Admin", sheet.Rows[0].Label)
	require.Equal(t, "Sample Department Administration", sheet.Rows[0].GroupName)
	require.Equal(t, "Standard Activities", sheet.Rows[0].TypeName)
	require.InDelta(t, 4.0, sheet.Rows[0].Hours.Mon, 0.001)
	require.InDelta(t, 2.0, sheet.Rows[0].Hours.Tue, 0.001)
	require.InDelta(t, 0.0, sheet.Rows[0].Hours.Wed, 0.001, "absent cells map to 0")
}

func TestApplySheetToDraft_AddNewCell(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 4, Wed: 3}},
		},
	}
	applySheetToDraft(sheet, &draft)

	cells := draft.Rows[0].Cells
	require.Len(t, cells, 2)
	// Cells sorted by Day.
	require.Equal(t, time.Monday, cells[0].Day)
	require.InDelta(t, 4.0, cells[0].Hours, 0.001)
	require.Equal(t, 100, cells[0].SourceEntryID)
	require.Equal(t, time.Wednesday, cells[1].Day)
	require.InDelta(t, 3.0, cells[1].Hours, 0.001)
	require.Equal(t, 0, cells[1].SourceEntryID, "new cell has no source ID")
}

func TestApplySheetToDraft_PreserveSourceIDOnZero(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 0}},
		},
	}
	applySheetToDraft(sheet, &draft)

	require.Len(t, draft.Rows[0].Cells, 1)
	require.InDelta(t, 0.0, draft.Rows[0].Cells[0].Hours, 0.001)
	require.Equal(t, 100, draft.Rows[0].Cells[0].SourceEntryID,
		"zeroing a pulled cell preserves SourceEntryID (delete-on-push)")
}

func TestApplySheetToDraft_DropLocalOnlyCellOnZero(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
				{Day: time.Tuesday, Hours: 2}, // local-only add
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 4, Tue: 0}},
		},
	}
	applySheetToDraft(sheet, &draft)

	require.Len(t, draft.Rows[0].Cells, 1, "local-only cell zeroed → dropped")
	require.Equal(t, time.Monday, draft.Rows[0].Cells[0].Day)
	require.Equal(t, 100, draft.Rows[0].Cells[0].SourceEntryID)
}

func TestApplySheetToDraft_PreservePerCellMetadata(t *testing.T) {
	desc := "review meeting"
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{
					Day: time.Monday, Hours: 4, SourceEntryID: 100,
					PerCell: &domain.PerCell{Description: &desc},
				},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 6}},
		},
	}
	applySheetToDraft(sheet, &draft)

	cell := draft.Rows[0].Cells[0]
	require.InDelta(t, 6.0, cell.Hours, 0.001)
	require.NotNil(t, cell.PerCell)
	require.NotNil(t, cell.PerCell.Description)
	require.Equal(t, "review meeting", *cell.PerCell.Description, "PerCell metadata preserved")
}

func TestApplySheetToDraft_ZeroAbsentCellNoOp(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 4, Wed: 0}},
		},
	}
	applySheetToDraft(sheet, &draft)

	require.Len(t, draft.Rows[0].Cells, 1, "absent cell + zero hours = no-op")
}
