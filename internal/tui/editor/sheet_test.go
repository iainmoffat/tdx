package editor

import (
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSheet_StructHasExpectedFields(t *testing.T) {
	s := Sheet{
		Name: "demo",
		Rows: []SheetRow{
			{
				ID:         "row-01",
				Label:      "Admin",
				GroupName:  "Sample Department Administration",
				DisplayRef: "plan/2075/task/2076",
				TypeName:   "Standard Activities",
				Hours:      domain.WeekHours{Mon: 4.0},
			},
		},
	}
	require.Equal(t, "demo", s.Name)
	require.Len(t, s.Rows, 1)
	require.Equal(t, "row-01", s.Rows[0].ID)
	require.Equal(t, "Admin", s.Rows[0].Label)
	require.Equal(t, "Sample Department Administration", s.Rows[0].GroupName)
	require.Equal(t, "plan/2075/task/2076", s.Rows[0].DisplayRef)
	require.Equal(t, "Standard Activities", s.Rows[0].TypeName)
	require.InDelta(t, 4.0, s.Rows[0].Hours.Mon, 0.001)
}
