package week

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestParseCellWrite_Valid(t *testing.T) {
	w, err := parseCellWrite("row-01:mon=8")
	if err != nil {
		t.Fatal(err)
	}
	if w.RowID != "row-01" || w.Day != time.Monday || w.Hours != 8 {
		t.Errorf("got %+v", w)
	}
}

func TestParseCellWrite_Invalid(t *testing.T) {
	cases := []string{"row-01", "row-01:mon", "row-01:badday=8", "row-01:mon=notanumber"}
	for _, c := range cases {
		if _, err := parseCellWrite(c); err == nil {
			t.Errorf("%q: expected error", c)
		}
	}
}

func TestApplyCellWrite_UpdatesExistingCell(t *testing.T) {
	d := domain.WeekDraft{
		Rows: []domain.DraftRow{{
			ID:    "row-01",
			Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4.0}},
		}},
	}
	if !applyCellWrite(&d, cellWrite{RowID: "row-01", Day: time.Monday, Hours: 8.0}) {
		t.Fatal("expected success")
	}
	if d.Rows[0].Cells[0].Hours != 8.0 {
		t.Errorf("hours = %v", d.Rows[0].Cells[0].Hours)
	}
}

func TestApplyCellWrite_AddsNewCell(t *testing.T) {
	d := domain.WeekDraft{
		Rows: []domain.DraftRow{{ID: "row-01"}},
	}
	if !applyCellWrite(&d, cellWrite{RowID: "row-01", Day: time.Friday, Hours: 4.0}) {
		t.Fatal("expected success")
	}
	if len(d.Rows[0].Cells) != 1 || d.Rows[0].Cells[0].Day != time.Friday {
		t.Errorf("cells = %+v", d.Rows[0].Cells)
	}
}

func TestApplyCellWrite_UnknownRow(t *testing.T) {
	d := domain.WeekDraft{Rows: []domain.DraftRow{{ID: "row-01"}}}
	if applyCellWrite(&d, cellWrite{RowID: "row-99", Day: time.Monday, Hours: 1.0}) {
		t.Errorf("expected false for unknown row")
	}
}
