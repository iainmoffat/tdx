package entry

import (
	"fmt"
	"io"

	"github.com/ipm/tdx/internal/domain"
)

// printEntry writes a human-readable entry summary to w.
func printEntry(w io.Writer, entry domain.TimeEntry) {
	fmt.Fprintf(w, "entry:        %d\n", entry.ID)
	fmt.Fprintf(w, "date:         %s\n", entry.Date.Format("2006-01-02"))
	fmt.Fprintf(w, "hours:        %.2f\n", entry.Hours())
	fmt.Fprintf(w, "minutes:      %d\n", entry.Minutes)
	fmt.Fprintf(w, "type:         %s\n", entry.TimeType.Name)
	fmt.Fprintf(w, "target:       %s\n", targetLabel(entry.Target))
	if entry.Description != "" {
		fmt.Fprintf(w, "description:  %s\n", entry.Description)
	}
	fmt.Fprintf(w, "status:       %s\n", entry.ReportStatus)
	fmt.Fprintf(w, "billable:     %t\n", entry.Billable)
}
