package project

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestComment_RequiresYes(t *testing.T) {
	cmd := newCommentCmd(&stubProjectsvc{})
	cmd.SetArgs([]string{"259", "--message", "hello"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}

func TestComment_RequiresMessage(t *testing.T) {
	cmd := newCommentCmd(&stubProjectsvc{})
	cmd.SetArgs([]string{"259", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--message") {
		t.Fatalf("want --message error, got %v", err)
	}
}

func TestComment_BuildsExpectedCall(t *testing.T) {
	stub := &stubProjectsvc{feedAddedID: 9999}
	var buf bytes.Buffer
	err := runProjectComment(context.Background(), &buf, stub, "default", 259, "my comment", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stub.lastFeedMessage != "my comment" {
		t.Errorf("message not passed: %q", stub.lastFeedMessage)
	}
	if stub.lastFeedPrivate {
		t.Error("isPrivate should be false")
	}
	if stub.lastProjectID != 259 {
		t.Errorf("projectID: %d", stub.lastProjectID)
	}
}

func TestComment_Private(t *testing.T) {
	stub := &stubProjectsvc{feedAddedID: 1234}
	var buf bytes.Buffer
	err := runProjectComment(context.Background(), &buf, stub, "default", 259, "internal note", true, []string{"uid-x"})
	if err != nil {
		t.Fatal(err)
	}
	if !stub.lastFeedPrivate {
		t.Error("isPrivate not passed through")
	}
	if len(stub.lastFeedNotify) != 1 || stub.lastFeedNotify[0] != "uid-x" {
		t.Errorf("notify: %v", stub.lastFeedNotify)
	}
}

func TestComment_HappyPath(t *testing.T) {
	stub := &stubProjectsvc{feedAddedID: 5678}
	var buf bytes.Buffer
	err := runProjectComment(context.Background(), &buf, stub, "default", 259, "hello", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"posted comment on project 259", "feed entry 5678"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %s", want, out)
		}
	}
}

func TestComment_ResolvesNotifyMe(t *testing.T) {
	// "me" resolution is done in the cobra RunE, which requires config; test
	// the raw service call with an already-resolved UID instead.
	stub := &stubProjectsvc{feedAddedID: 100}
	var buf bytes.Buffer
	err := runProjectComment(context.Background(), &buf, stub, "default", 259, "msg", false, []string{"aaaa-bbbb-cccc-dddd-eeee"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.lastFeedNotify) != 1 || stub.lastFeedNotify[0] != "aaaa-bbbb-cccc-dddd-eeee" {
		t.Errorf("notify: %v", stub.lastFeedNotify)
	}
}

// Ensure the stub properly implements the interface for all new methods.
var _ projectsvcAPI = (*stubProjectsvc)(nil)

// Extra: verify the domain type is accepted here.
var _ = domain.ProjectFeedEntry{}
