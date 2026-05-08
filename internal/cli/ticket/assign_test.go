package ticket

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunTicketAssignByUID(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{ID: 1, ResponsibleUID: "alice-uid", ResponsibleName: "Alice"}}
	var buf bytes.Buffer
	if err := runTicketAssign(context.Background(), &buf, stub, "default", 31, 1, "alice-uid", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "→ Alice") {
		t.Errorf("output: %s", buf.String())
	}
	if stub.lastPatchOps[0].Path != "/ResponsibleUid" {
		t.Errorf("patch path: %s", stub.lastPatchOps[0].Path)
	}
	if stub.lastPatchOps[0].Value != "alice-uid" {
		t.Errorf("patch value: %v", stub.lastPatchOps[0].Value)
	}
}

func TestRunTicketAssignWithComment(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{ResponsibleName: "Alice"}, feedAddedID: 7}
	var buf bytes.Buffer
	_ = runTicketAssign(context.Background(), &buf, stub, "default", 31, 1, "alice-uid", "handing off")
	if stub.lastFeedBody != "handing off" {
		t.Errorf("comment not posted: %q", stub.lastFeedBody)
	}
}

func TestRunTicketAssignFallsBackToUIDWhenNoName(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{ResponsibleUID: "uid-x", ResponsibleName: ""}}
	var buf bytes.Buffer
	_ = runTicketAssign(context.Background(), &buf, stub, "default", 31, 1, "uid-x", "")
	if !strings.Contains(buf.String(), "uid-x") {
		t.Errorf("expected uid in output: %s", buf.String())
	}
}

func TestNewAssignCmdRequiresYes(t *testing.T) {
	cmd := newAssignCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"1", "uid-x"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}
