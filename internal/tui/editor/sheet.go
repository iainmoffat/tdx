package editor

import "github.com/iainmoffat/tdx/internal/domain"

// Sheet is the editor's view of a hours-by-day grid. The TUI and web
// editors both operate on Sheet; adapters convert from Template, WeekDraft,
// or any other source.
//
// Sheet does not know what it represents. Name is the editor title;
// Rows are presented in display order (caller-determined).
type Sheet struct {
	Name string
	Rows []SheetRow
}

// SheetRow is one row of a Sheet — typically one (target, type, billable)
// triple, but the editor only cares about the ID for matching on save.
type SheetRow struct {
	ID         string
	Label      string
	GroupName  string
	DisplayRef string
	TypeName   string
	Hours      domain.WeekHours
}
