package entry

import (
	"fmt"
	"io"

	"github.com/iainmoffat/tdx/internal/domain"
)

// printEntry writes a human-readable entry summary to w.
func printEntry(w io.Writer, entry domain.TimeEntry) {
	_, _ = fmt.Fprintf(w, "entry:        %d\n", entry.ID)
	_, _ = fmt.Fprintf(w, "date:         %s\n", entry.Date.Format("2006-01-02"))
	_, _ = fmt.Fprintf(w, "hours:        %.2f\n", entry.Hours())
	_, _ = fmt.Fprintf(w, "minutes:      %d\n", entry.Minutes)
	_, _ = fmt.Fprintf(w, "type:         %s\n", entry.TimeType.Name)
	_, _ = fmt.Fprintf(w, "target:       %s\n", targetLabel(entry.Target))
	if entry.Description != "" {
		_, _ = fmt.Fprintf(w, "description:  %s\n", entry.Description)
	}
	_, _ = fmt.Fprintf(w, "status:       %s\n", entry.ReportStatus)
	_, _ = fmt.Fprintf(w, "billable:     %t\n", entry.Billable)
}
