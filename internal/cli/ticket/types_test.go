package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunTypesListTable(t *testing.T) {
	stub := &stubTicketsvc{types: []domain.TicketType{
		{ID: 10, Name: "Incident", Description: "Issue", Active: true},
		{ID: 11, Name: "Service Request", Active: true},
	}}
	var buf bytes.Buffer
	if err := runTypesList(context.Background(), &buf, stub, "default", 31, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"10", "Incident", "11", "Service Request"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunTypesListJSON(t *testing.T) {
	stub := &stubTicketsvc{types: []domain.TicketType{{ID: 10, Name: "Incident", Active: true}}}
	var buf bytes.Buffer
	if err := runTypesList(context.Background(), &buf, stub, "default", 31, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketTypeList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunTypesListEmpty(t *testing.T) {
	stub := &stubTicketsvc{types: nil}
	var buf bytes.Buffer
	_ = runTypesList(context.Background(), &buf, stub, "default", 31, false)
	if !strings.Contains(buf.String(), "no ticket types found") {
		t.Errorf("empty msg: %s", buf.String())
	}
}
