package ticket

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func TestRunTicketStatusByName(t *testing.T) {
	stub := &stubTicketsvc{
		resolvedStatus: domain.TicketStatus{ID: 5, Name: "Closed"},
		patched:        domain.Ticket{ID: 12345, StatusName: "Closed"},
	}
	var buf bytes.Buffer
	if err := runTicketStatus(context.Background(), &buf, stub, "default", 31, 12345, 0, "Closed", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "→ Closed") {
		t.Errorf("output: %s", buf.String())
	}
	if len(stub.lastPatchOps) != 1 || stub.lastPatchOps[0].Path != "/StatusID" {
		t.Errorf("patch ops: %+v", stub.lastPatchOps)
	}
	if v, ok := stub.lastPatchOps[0].Value.(int); !ok || v != 5 {
		t.Errorf("value: %v", stub.lastPatchOps[0].Value)
	}
}

func TestRunTicketStatusByID(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{StatusName: "In Progress"}}
	var buf bytes.Buffer
	_ = runTicketStatus(context.Background(), &buf, stub, "default", 31, 1, 7, "", "")
	if v, _ := stub.lastPatchOps[0].Value.(int); v != 7 {
		t.Errorf("explicit id should be used: %v", stub.lastPatchOps[0].Value)
	}
}

func TestRunTicketStatusWithComment(t *testing.T) {
	stub := &stubTicketsvc{
		resolvedStatus: domain.TicketStatus{ID: 5, Name: "Closed"},
		patched:        domain.Ticket{StatusName: "Closed"},
		feedAddedID:    99,
	}
	var buf bytes.Buffer
	_ = runTicketStatus(context.Background(), &buf, stub, "default", 31, 1, 0, "Closed", "Resolved by patch")
	if stub.lastFeedBody != "Resolved by patch" {
		t.Errorf("comment not posted: %q", stub.lastFeedBody)
	}
}

func TestNewStatusCmdRequiresYes(t *testing.T) {
	cmd := newStatusCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"1", "Closed"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}

// Quiet unused-import warning — ticketsvc is referenced via stub_test.go's PatchOp.
var _ = ticketsvc.PatchOp{}
