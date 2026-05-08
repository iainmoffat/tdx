package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunStatusesListTable(t *testing.T) {
	stub := &stubTicketsvc{statuses: []domain.TicketStatus{
		{ID: 1, Name: "New", IsDefault: true, Order: 1.0},
		{ID: 5, Name: "Closed", IsClosed: true, Order: 5.0},
	}}
	var buf bytes.Buffer
	if err := runStatusesList(context.Background(), &buf, stub, "default", 31, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"1", "New", "5", "Closed"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
	// Closed's IS-CLOSED column should say "yes"
	if !strings.Contains(out, "yes") {
		t.Error("expected 'yes' for IsClosed/IsDefault")
	}
}

func TestRunStatusesListJSON(t *testing.T) {
	stub := &stubTicketsvc{statuses: []domain.TicketStatus{{ID: 1, Name: "New", Order: 1.0}}}
	var buf bytes.Buffer
	if err := runStatusesList(context.Background(), &buf, stub, "default", 31, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketStatusList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunStatusesListEmpty(t *testing.T) {
	stub := &stubTicketsvc{statuses: nil}
	var buf bytes.Buffer
	_ = runStatusesList(context.Background(), &buf, stub, "default", 31, false)
	if !strings.Contains(buf.String(), "no ticket statuses found") {
		t.Errorf("empty msg: %s", buf.String())
	}
}
