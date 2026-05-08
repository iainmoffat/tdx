package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestBuildSearchFilterDefaultsToMe(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-of-me", 31, nil, nil, nil, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 1 || filter.AssigneeUIDs[0] != "uid-of-me" {
		t.Errorf("default assignee not me: %+v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterExplicitAssigneeOverridesDefault(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	uid := "12345678-1234-1234-1234-123456789012"
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-of-me", 31, nil, []string{uid}, nil, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 1 || filter.AssigneeUIDs[0] != uid {
		t.Errorf("explicit override not respected: %+v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterMeKeyword(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-of-me", 31, nil, []string{"me"}, nil, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if filter.AssigneeUIDs[0] != "uid-of-me" {
		t.Errorf("'me' should resolve: %+v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterStatusByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedStatus: domain.TicketStatus{ID: 7, Name: "In Progress"}}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "me", 31, []string{"In Progress"}, nil, nil, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.StatusIDs) != 1 || filter.StatusIDs[0] != 7 {
		t.Errorf("status name not resolved: %+v", filter.StatusIDs)
	}
}

func TestBuildSearchFilterStatusByID(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "me", 31, []string{"5"}, nil, nil, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.StatusIDs) != 1 || filter.StatusIDs[0] != 5 {
		t.Errorf("numeric status not preserved: %+v", filter.StatusIDs)
	}
}

func TestBuildSearchFilterClampsLimit(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	for _, c := range []struct{ in, want int }{{0, 50}, {-1, 50}, {500, 500}, {2000, 1000}} {
		f, _ := buildSearchFilter(context.Background(), stub, people, "default", "me", 31, nil, []string{"me"}, nil, nil, nil, "", "", c.in, false)
		if f.Limit != c.want {
			t.Errorf("limit %d → %d, want %d", c.in, f.Limit, c.want)
		}
	}
}

func TestRunTicketSearchEmpty(t *testing.T) {
	stub := &stubTicketsvc{tickets: nil}
	var buf bytes.Buffer
	if err := runTicketSearch(context.Background(), &buf, stub, "default", domain.TicketSearchFilter{AppID: 31}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no tickets matched") {
		t.Errorf("empty msg: %s", buf.String())
	}
}

func TestRunTicketSearchTable(t *testing.T) {
	stub := &stubTicketsvc{tickets: []domain.Ticket{
		{ID: 100, Title: "Help me", StatusName: "Open", TypeName: "Incident",
			ResponsibleName: "Alice", RequestorName: "Bob",
			ModifiedDate: time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)},
	}}
	var buf bytes.Buffer
	if err := runTicketSearch(context.Background(), &buf, stub, "default", domain.TicketSearchFilter{AppID: 31}, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"100", "Help me", "Open", "Alice", "Bob", "2026-05-08", "partial"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestRunTicketSearchJSON(t *testing.T) {
	stub := &stubTicketsvc{tickets: []domain.Ticket{{ID: 100, Title: "T"}}}
	var buf bytes.Buffer
	if err := runTicketSearch(context.Background(), &buf, stub, "default", domain.TicketSearchFilter{AppID: 31}, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunSavedSearchListEmpty(t *testing.T) {
	stub := &stubTicketsvc{savedSearches: nil}
	var buf bytes.Buffer
	if err := runSavedSearchList(context.Background(), &buf, stub, "default", 31, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no saved searches found") {
		t.Errorf("empty: %s", buf.String())
	}
}

func TestRunSavedSearchListTable(t *testing.T) {
	stub := &stubTicketsvc{savedSearches: []domain.TicketSavedSearch{
		{ID: 7, Name: "My Open", OwnerName: "Alice"},
	}}
	var buf bytes.Buffer
	if err := runSavedSearchList(context.Background(), &buf, stub, "default", 31, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"7", "My Open", "Alice"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunSavedSearchRunByName(t *testing.T) {
	stub := &stubTicketsvc{
		resolvedSaved: domain.TicketSavedSearch{ID: 7, Name: "My Open"},
		savedResults:  []domain.Ticket{{ID: 1, Title: "X", StatusName: "Open"}},
	}
	var buf bytes.Buffer
	if err := runSavedSearchRun(context.Background(), &buf, stub, "default", 31, "My Open", 50, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "X") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestBuildSearchFilterGroupByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedGroup: domain.TicketGroup{ID: 100, Name: "Linux Team"}}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, []string{"Linux Team"}, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.ResponsibilityGroupIDs) != 1 || filter.ResponsibilityGroupIDs[0] != 100 {
		t.Errorf("group ID not resolved: %+v", filter.ResponsibilityGroupIDs)
	}
	if len(filter.AssigneeUIDs) != 0 {
		t.Errorf("default-to-me should not fire when --responsibility-group is set; got %v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterGroupByID(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, []string{"42"}, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.ResponsibilityGroupIDs) != 1 || filter.ResponsibilityGroupIDs[0] != 42 {
		t.Errorf("numeric group not preserved: %+v", filter.ResponsibilityGroupIDs)
	}
}

func TestBuildSearchFilterManagerExpandsToReports(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{
		searchUsers: []domain.User{
			{UID: "report-1", ReportsToUID: "uid-me"},
			{UID: "report-2", ReportsToUID: "uid-me"},
			{UID: "other", ReportsToUID: "someone"},
		},
	}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, nil, []string{"me"}, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 2 {
		t.Fatalf("want 2 reports, got %d: %v", len(filter.AssigneeUIDs), filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterManagerSuppressesDefaultToMe(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{searchUsers: []domain.User{}}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, nil, nil, nil, []string{"me"}, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 0 {
		t.Errorf("should be empty (no reports), got %v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterManagerMergesWithAssignees(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{
		searchUsers: []domain.User{{UID: "report-1", ReportsToUID: "uid-me"}},
	}
	uid := "12345678-1234-1234-1234-123456789012"
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, []string{uid}, nil, nil, []string{"me"}, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 2 {
		t.Fatalf("want 2 (raw + report), got %d: %v", len(filter.AssigneeUIDs), filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterMultipleSelectorsCombine(t *testing.T) {
	stub := &stubTicketsvc{resolvedGroup: domain.TicketGroup{ID: 200, Name: "Foo"}}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-me", 31,
		nil, []string{"me"}, nil, []string{"Foo"}, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 1 || filter.AssigneeUIDs[0] != "uid-me" {
		t.Errorf("assignees: %v", filter.AssigneeUIDs)
	}
	if len(filter.ResponsibilityGroupIDs) != 1 || filter.ResponsibilityGroupIDs[0] != 200 {
		t.Errorf("groups: %v", filter.ResponsibilityGroupIDs)
	}
}
