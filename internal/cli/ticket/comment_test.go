package ticket

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunTicketCommentSuccess(t *testing.T) {
	stub := &stubTicketsvc{feedAddedID: 555}
	var buf bytes.Buffer
	if err := runTicketComment(context.Background(), &buf, stub, "default", 31, 12345, "test message", false, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"#12345", "555", "public"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
	if stub.lastFeedBody != "test message" {
		t.Errorf("body not passed through: %q", stub.lastFeedBody)
	}
}

func TestRunTicketCommentPrivate(t *testing.T) {
	stub := &stubTicketsvc{feedAddedID: 999}
	var buf bytes.Buffer
	if err := runTicketComment(context.Background(), &buf, stub, "default", 31, 1, "internal", true, []string{"uid-x"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "private") {
		t.Errorf("expected 'private' in output: %s", buf.String())
	}
	if !stub.lastFeedPrivate {
		t.Errorf("isPrivate flag not passed")
	}
	if len(stub.lastFeedNotify) != 1 || stub.lastFeedNotify[0] != "uid-x" {
		t.Errorf("notify not passed: %v", stub.lastFeedNotify)
	}
}

func TestNewCommentCmdRequiresYes(t *testing.T) {
	cmd := newCommentCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"123", "hi"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes-required error, got %v", err)
	}
}
