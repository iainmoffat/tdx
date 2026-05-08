package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunGroupsListTable(t *testing.T) {
	stub := &stubTicketsvc{groups: []domain.TicketGroup{
		{ID: 100, Name: "Linux Team", Active: true},
		{ID: 101, Name: "Network Ops", Active: true},
		{ID: 102, Name: "Archived", Active: false},
	}}
	var buf bytes.Buffer
	if err := runGroupsList(context.Background(), &buf, stub, "default", false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"100", "Linux Team", "101", "Network Ops", "102", "Archived"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunGroupsListJSON(t *testing.T) {
	stub := &stubTicketsvc{groups: []domain.TicketGroup{{ID: 100, Name: "Linux Team", Active: true}}}
	var buf bytes.Buffer
	if err := runGroupsList(context.Background(), &buf, stub, "default", true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketGroupList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunGroupsListEmpty(t *testing.T) {
	stub := &stubTicketsvc{groups: nil}
	var buf bytes.Buffer
	_ = runGroupsList(context.Background(), &buf, stub, "default", false)
	if !strings.Contains(buf.String(), "no ticket groups found") {
		t.Errorf("empty msg: %s", buf.String())
	}
}
