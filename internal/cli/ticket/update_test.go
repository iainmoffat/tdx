package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

func TestBuildTicketPatchOpsAllFields(t *testing.T) {
	title := "T"
	desc := "D"
	tid := 5
	aid := 10
	uid := "uid-x"
	gid := 100
	pid := 3
	ops := buildTicketPatchOps(ticketUpdateFields{
		title: &title, description: &desc,
		typeID: &tid, accountID: &aid,
		requestorUID: &uid, groupID: &gid, priorityID: &pid,
	})
	if len(ops) != 7 {
		t.Fatalf("want 7 ops, got %d: %+v", len(ops), ops)
	}
	wantPaths := []string{"/Title", "/Description", "/TypeID", "/AccountID", "/RequestorUid", "/ResponsibleGroupID", "/PriorityID"}
	for i, op := range ops {
		if op.Op != "replace" {
			t.Errorf("ops[%d].Op = %q, want replace", i, op.Op)
		}
		if op.Path != wantPaths[i] {
			t.Errorf("ops[%d].Path = %q, want %q", i, op.Path, wantPaths[i])
		}
	}
}

func TestBuildTicketPatchOpsNoFields(t *testing.T) {
	ops := buildTicketPatchOps(ticketUpdateFields{})
	if len(ops) != 0 {
		t.Fatalf("want 0 ops, got %d", len(ops))
	}
}

func TestBuildTicketPatchOpsEmptyDescriptionStillEmits(t *testing.T) {
	empty := ""
	ops := buildTicketPatchOps(ticketUpdateFields{description: &empty})
	if len(ops) != 1 {
		t.Fatalf("want 1 op, got %d", len(ops))
	}
	if ops[0].Path != "/Description" {
		t.Errorf("path: %q", ops[0].Path)
	}
	if v, _ := ops[0].Value.(string); v != "" {
		t.Errorf("value should be empty string, got %v", ops[0].Value)
	}
}

func TestTicketUpdateFieldsHasAny(t *testing.T) {
	if (ticketUpdateFields{}).hasAny() {
		t.Error("empty fields should not hasAny()")
	}
	title := "T"
	if !(ticketUpdateFields{title: &title}).hasAny() {
		t.Error("title-only should hasAny()")
	}
}

func TestResolveUpdateFieldsTitleAndDescription(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		title: "Hello", titleSet: true, description: "", descriptionSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.title == nil || *got.title != "Hello" {
		t.Errorf("title: %v", got.title)
	}
	if got.description == nil || *got.description != "" {
		t.Errorf("empty description should be set, got %v", got.description)
	}
}

func TestResolveUpdateFieldsTypeByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedType: domain.TicketType{ID: 7, Name: "Incident"}}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		typeArg: "Incident",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.typeID == nil || *got.typeID != 7 {
		t.Errorf("typeID: %v", got.typeID)
	}
}

func TestResolveUpdateFieldsTypeByNumericArg(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		typeArg: "7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.typeID == nil || *got.typeID != 7 {
		t.Errorf("typeID: %v", got.typeID)
	}
}

func TestResolveUpdateFieldsAccountByName(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{resolvedAccount: peoplesvc.Account{ID: 1566, Name: "Test Acct"}}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		accountArg: "Test Acct",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.accountID == nil || *got.accountID != 1566 {
		t.Errorf("accountID: %v", got.accountID)
	}
}

func TestResolveUpdateFieldsRequestorMe(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		requestorArg: "me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.requestorUID == nil || *got.requestorUID != "uid-me" {
		t.Errorf("requestorUID: %v", got.requestorUID)
	}
}

func TestResolveUpdateFieldsGroupByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedGroup: domain.TicketGroup{ID: 100, Name: "Linux Team"}}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		groupArg: "Linux Team",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.groupID == nil || *got.groupID != 100 {
		t.Errorf("groupID: %v", got.groupID)
	}
}

func TestResolveUpdateFieldsPriority(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	got, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		priorityID: 3, prioritySet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.priorityID == nil || *got.priorityID != 3 {
		t.Errorf("priorityID: %v", got.priorityID)
	}
}

func TestResolveUpdateFieldsTypeResolverErrorPropagates(t *testing.T) {
	stub := &stubTicketsvc{err: errors.New("boom")}
	people := &stubPeoplesvc{}
	_, err := resolveUpdateFields(context.Background(), stub, people, "default", "uid-me", 31, rawUpdateFlags{
		typeArg: "Nonsense",
	})
	if err == nil || !strings.Contains(err.Error(), "--type") {
		t.Fatalf("want propagated --type error, got %v", err)
	}
}

func TestChangedFieldsSummaryUsesNamesFromUpdatedTicket(t *testing.T) {
	tid := 7
	pid := 3
	got := changedFieldsSummary(ticketUpdateFields{typeID: &tid, priorityID: &pid}, domain.Ticket{
		TypeName: "Incident", PriorityName: "High",
	})
	if !strings.Contains(got, "type=Incident") {
		t.Errorf("missing type=Incident: %s", got)
	}
	if !strings.Contains(got, "priority=High") {
		t.Errorf("missing priority=High: %s", got)
	}
}

func TestChangedFieldsSummaryFallsBackToIDWhenNameMissing(t *testing.T) {
	tid := 7
	got := changedFieldsSummary(ticketUpdateFields{typeID: &tid}, domain.Ticket{})
	if !strings.Contains(got, "type-id=7") {
		t.Errorf("expected fallback to type-id=7: %s", got)
	}
}

func TestRunTicketUpdateSuccess(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{ID: 100, Title: "After", TypeName: "Incident"}}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		title: "After", titleSet: true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ticket #100 updated") {
		t.Errorf("output: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `title="After"`) {
		t.Errorf("expected title in summary: %s", buf.String())
	}
	if len(stub.lastPatchOps) != 1 || stub.lastPatchOps[0].Path != "/Title" {
		t.Errorf("patch ops: %+v", stub.lastPatchOps)
	}
}

func TestRunTicketUpdateWithComment(t *testing.T) {
	stub := &stubTicketsvc{
		patched:     domain.Ticket{ID: 100, Title: "T"},
		feedAddedID: 999,
	}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		title: "T", titleSet: true, comment: "fyi",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if stub.lastFeedBody != "fyi" {
		t.Errorf("comment not posted: %q", stub.lastFeedBody)
	}
	if !strings.Contains(buf.String(), "feed entry 999") {
		t.Errorf("expected feed-entry mention: %s", buf.String())
	}
}

func TestRunTicketUpdateNothingToUpdate(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	err := runTicketUpdate(context.Background(), io.Discard, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{}, false)
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("want nothing-to-update error, got %v", err)
	}
}

func TestRunTicketUpdateCommentOnlyDoesNotPatch(t *testing.T) {
	stub := &stubTicketsvc{
		ticket:      domain.Ticket{ID: 100, Title: "T"},
		feedAddedID: 555,
	}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		comment: "just a note",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.lastPatchOps) != 0 {
		t.Errorf("PatchTicket should NOT have been called; got ops %+v", stub.lastPatchOps)
	}
	if stub.lastFeedBody != "just a note" {
		t.Errorf("comment not posted: %q", stub.lastFeedBody)
	}
}

func TestRunTicketUpdateJSON(t *testing.T) {
	stub := &stubTicketsvc{patched: domain.Ticket{ID: 100, Title: "T"}}
	people := &stubPeoplesvc{}
	var buf bytes.Buffer
	err := runTicketUpdate(context.Background(), &buf, stub, people, "default", "uid-me", 31, 100, rawUpdateFlags{
		title: "T", titleSet: true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticket" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestNewUpdateCmdRequiresYes(t *testing.T) {
	cmd := newUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "--title", "x"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}
