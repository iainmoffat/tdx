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

func TestRunTaskListEmpty(t *testing.T) {
	stub := &stubTicketsvc{tasks: nil}
	var buf bytes.Buffer
	if err := runTaskList(context.Background(), &buf, stub, "default", 31, 100, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no tasks found") {
		t.Errorf("empty msg: %s", buf.String())
	}
}

func TestRunTaskListTable(t *testing.T) {
	stub := &stubTicketsvc{tasks: []domain.TicketTask{
		{ID: 1, TicketID: 100, Title: "Step 1", PercentComplete: 50, EstimatedMinutes: 60, ActualMinutes: 30, ResponsibleName: "Alice"},
		{ID: 2, TicketID: 100, Title: "Step 2", PercentComplete: 0, ResponsibleGroupName: "Linux Team"},
	}}
	var buf bytes.Buffer
	if err := runTaskList(context.Background(), &buf, stub, "default", 31, 100, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Step 1", "50%", "Alice", "Step 2", "Linux Team (group)"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunTaskListJSON(t *testing.T) {
	stub := &stubTicketsvc{tasks: []domain.TicketTask{{ID: 1, TicketID: 100, Title: "T"}}}
	var buf bytes.Buffer
	if err := runTaskList(context.Background(), &buf, stub, "default", 31, 100, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketTaskList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunTaskShow(t *testing.T) {
	stub := &stubTicketsvc{task: domain.TicketTask{
		ID: 5, TicketID: 100, Title: "Investigate",
		PercentComplete: 75, EstimatedMinutes: 240, ActualMinutes: 90,
		ResponsibleName: "Alice",
		CreatedDate:     time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC), CreatedName: "Alice",
		Description: "find the issue",
	}}
	var buf bytes.Buffer
	if err := runTaskShow(context.Background(), &buf, stub, "default", 31, 100, 5, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#100 / task #5", "Investigate", "75%", "Alice", "find the issue", "EST: 4h", "ACT: 1h 30m"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunTaskShowJSON(t *testing.T) {
	stub := &stubTicketsvc{task: domain.TicketTask{ID: 5, TicketID: 100, Title: "Investigate", PercentComplete: 75}}
	var buf bytes.Buffer
	if err := runTaskShow(context.Background(), &buf, stub, "default", 31, 100, 5, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketTask" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunTaskFeedRendersEntries(t *testing.T) {
	stub := &stubTicketsvc{taskFeed: []domain.TicketFeedEntry{
		{ID: 200, AuthorName: "Alice", Body: "halfway", EventKind: "comment", CreatedAt: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	}}
	var buf bytes.Buffer
	if err := runTaskFeed(context.Background(), &buf, stub, "default", 31, 100, 5, 0, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Alice", "comment", "halfway"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q: %s", want, buf.String())
		}
	}
}

func TestRunTaskFeedEmpty(t *testing.T) {
	stub := &stubTicketsvc{taskFeed: nil}
	var buf bytes.Buffer
	_ = runTaskFeed(context.Background(), &buf, stub, "default", 31, 100, 5, 0, false)
	if !strings.Contains(buf.String(), "no feed entries") {
		t.Errorf("empty: %s", buf.String())
	}
}

func TestRunTaskUpdateWithComment(t *testing.T) {
	stub := &stubTicketsvc{taskFeedAddedID: 555}
	var buf bytes.Buffer
	pc := 50
	if err := runTaskUpdate(context.Background(), &buf, stub, "default", 31, 100, 5, "halfway", &pc, 0, false, nil); err != nil {
		t.Fatal(err)
	}
	if stub.lastTaskUpdate.Body != "halfway" {
		t.Errorf("body: %q", stub.lastTaskUpdate.Body)
	}
	if stub.lastTaskUpdate.PercentComplete == nil || *stub.lastTaskUpdate.PercentComplete != 50 {
		t.Errorf("percent: %v", stub.lastTaskUpdate.PercentComplete)
	}
	if !strings.Contains(buf.String(), "555") || !strings.Contains(buf.String(), "percent=50%") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestRunTaskUpdateHoursWorkedNoteInformational(t *testing.T) {
	stub := &stubTicketsvc{taskFeedAddedID: 1}
	var buf bytes.Buffer
	_ = runTaskUpdate(context.Background(), &buf, stub, "default", 31, 100, 5, "", nil, 0.5, false, nil)
	if stub.lastTaskUpdate.HoursWorked != 0.5 {
		t.Errorf("hours: %v", stub.lastTaskUpdate.HoursWorked)
	}
	if !strings.Contains(buf.String(), "informational") {
		t.Errorf("expected 'informational' label in output: %s", buf.String())
	}
}

func TestNewTaskUpdateCmdRequiresYes(t *testing.T) {
	cmd := newTaskUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--comment", "hi"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}

func TestNewTaskUpdateCmdRejectsPercentAndComplete(t *testing.T) {
	cmd := newTaskUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--percent", "50", "--complete", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("want mutex error, got %v", err)
	}
}

func TestNewTaskUpdateCmdRejectsAllEmpty(t *testing.T) {
	cmd := newTaskUpdateCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("want all-empty error, got %v", err)
	}
}

func TestNewTaskUpdateCmdCompleteSetsHundred(t *testing.T) {
	stub := &stubTicketsvc{taskFeedAddedID: 1}
	cmd := newTaskUpdateCmd(stub)
	cmd.SetArgs([]string{"100", "5", "--complete", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stub.lastTaskUpdate.PercentComplete == nil || *stub.lastTaskUpdate.PercentComplete != 100 {
		t.Errorf("--complete should set percent=100; got %v", stub.lastTaskUpdate.PercentComplete)
	}
}

func TestRunTaskLog(t *testing.T) {
	stub := &stubTimesvc{
		types:      []domain.TimeType{{ID: 7, Name: "Development", Billable: true}},
		addedEntry: domain.TimeEntry{ID: 9001},
	}
	var buf bytes.Buffer
	err := runTaskLog(context.Background(), &buf, stub, taskLogArgs{
		profile: "default", authedUID: "uid-me", appID: 31, ticketID: 100, taskID: 5,
		minutes: 30, typeName: "Development",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.lastInput.Target.Kind != domain.TargetTicketTask {
		t.Errorf("Target.Kind: %s", stub.lastInput.Target.Kind)
	}
	if stub.lastInput.Target.TaskID != 5 {
		t.Errorf("Target.TaskID: %d", stub.lastInput.Target.TaskID)
	}
	if stub.lastInput.Target.ItemID != 100 {
		t.Errorf("Target.ItemID: %d", stub.lastInput.Target.ItemID)
	}
	if !strings.Contains(buf.String(), "task #5") || !strings.Contains(buf.String(), "9001") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestNewTaskLogCmdRequiresYes(t *testing.T) {
	cmd := newTaskLogCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"100", "5", "--minutes", "30", "--type", "Development"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}
