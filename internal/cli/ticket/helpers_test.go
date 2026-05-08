package ticket

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestResolvePrincipalMe(t *testing.T) {
	got, err := resolvePrincipal(context.Background(), nil, "default", "uid-of-me", "me")
	if err != nil {
		t.Fatal(err)
	}
	if got != "uid-of-me" {
		t.Fatalf("want uid-of-me, got %s", got)
	}
}

func TestResolvePrincipalMeNoAuth(t *testing.T) {
	_, err := resolvePrincipal(context.Background(), nil, "default", "", "me")
	if err == nil || !strings.Contains(err.Error(), "authenticated session") {
		t.Fatalf("want auth-required error, got %v", err)
	}
}

func TestResolvePrincipalRawUID(t *testing.T) {
	uid := "12345678-1234-1234-1234-123456789012"
	got, err := resolvePrincipal(context.Background(), nil, "default", "", uid)
	if err != nil {
		t.Fatal(err)
	}
	if got != uid {
		t.Fatalf("want %s, got %s", uid, got)
	}
}

func TestResolvePrincipalEmailLookupSingleMatch(t *testing.T) {
	stub := &stubPeoplesvc{
		users: []domain.User{{UID: "alice-uid", FullName: "Alice", Email: "alice@uf.edu"}},
	}
	got, err := resolvePrincipal(context.Background(), stub, "default", "", "alice@uf.edu")
	if err != nil {
		t.Fatal(err)
	}
	if got != "alice-uid" {
		t.Fatalf("want alice-uid, got %s", got)
	}
}

func TestResolvePrincipalEmptyArg(t *testing.T) {
	_, err := resolvePrincipal(context.Background(), nil, "default", "", "")
	if err == nil {
		t.Fatal("expected error for empty arg")
	}
}

func TestResolvePrincipalAmbiguous(t *testing.T) {
	stub := &stubPeoplesvc{
		users: []domain.User{
			{UID: "a", FullName: "Alice", Email: "alice@x"},
			{UID: "b", FullName: "Bob", Email: "bob@x"},
		},
	}
	_, err := resolvePrincipal(context.Background(), stub, "default", "", "a")
	if err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("want ambiguous error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Alice") || !strings.Contains(err.Error(), "Bob") {
		t.Errorf("error should list candidates: %v", err)
	}
}

func TestResolvePrincipalNoMatch(t *testing.T) {
	stub := &stubPeoplesvc{users: nil}
	_, err := resolvePrincipal(context.Background(), stub, "default", "", "nobody")
	if err == nil || !strings.Contains(err.Error(), "no user matches") {
		t.Fatalf("want no-match error, got %v", err)
	}
}

func TestResolvePrincipalLookupErrorPropagates(t *testing.T) {
	stub := &stubPeoplesvc{err: errors.New("boom")}
	_, err := resolvePrincipal(context.Background(), stub, "default", "", "alice")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want propagated error, got %v", err)
	}
}

func TestParseStatusArgNumeric(t *testing.T) {
	id, name := parseStatusArg("4")
	if id != 4 || name != "" {
		t.Fatalf("got (%d, %q), want (4, \"\")", id, name)
	}
}

func TestParseStatusArgName(t *testing.T) {
	id, name := parseStatusArg("In Progress")
	if id != 0 || name != "In Progress" {
		t.Fatalf("got (%d, %q), want (0, \"In Progress\")", id, name)
	}
}

func TestParseStatusArgTrimsWhitespace(t *testing.T) {
	id, name := parseStatusArg("  4  ")
	if id != 4 || name != "" {
		t.Fatalf("got (%d, %q), want (4, \"\")", id, name)
	}
}

func TestPartialResultBannerEmpty(t *testing.T) {
	if got := partialResultBanner(0); got != "" {
		t.Errorf("want empty for 0 rows, got %q", got)
	}
}

func TestPartialResultBannerWithRows(t *testing.T) {
	got := partialResultBanner(7)
	if !strings.Contains(got, "7 row") || !strings.Contains(got, "tdx ticket show") {
		t.Errorf("banner: %q", got)
	}
}

// stubPeoplesvc satisfies peoplesvcAPI for tests.
type stubPeoplesvc struct {
	users       []domain.User
	err         error
	searchUsers []domain.User
	searchErr   error
	lastFilter  domain.UserFilter
}

func (s *stubPeoplesvc) LookupPeople(_ context.Context, _ string, _ string, _ int) ([]domain.User, error) {
	return s.users, s.err
}

func (s *stubPeoplesvc) SearchUsers(_ context.Context, _ string, filter domain.UserFilter) ([]domain.User, error) {
	s.lastFilter = filter
	return s.searchUsers, s.searchErr
}

func TestExpandManagersToReportsEmpty(t *testing.T) {
	got, err := expandManagersToReports(context.Background(), &stubPeoplesvc{}, "default", "uid-me", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

func TestExpandManagersToReportsMe(t *testing.T) {
	stub := &stubPeoplesvc{
		searchUsers: []domain.User{
			{UID: "report-1", FullName: "Alice", ReportsToUID: "uid-me"},
			{UID: "report-2", FullName: "Bob", ReportsToUID: "uid-me"},
			{UID: "other-1", FullName: "Carol", ReportsToUID: "uid-someone-else"},
		},
	}
	got, err := expandManagersToReports(context.Background(), stub, "default", "uid-me", []string{"me"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 reports, got %d: %v", len(got), got)
	}
	want := map[string]bool{"report-1": true, "report-2": true}
	for _, uid := range got {
		if !want[uid] {
			t.Errorf("unexpected uid: %s", uid)
		}
	}
	if stub.lastFilter.Employee == nil || !*stub.lastFilter.Employee {
		t.Errorf("Employee filter not set: %+v", stub.lastFilter)
	}
	if stub.lastFilter.Limit != 5000 {
		t.Errorf("Limit: got %d, want 5000", stub.lastFilter.Limit)
	}
}

func TestExpandManagersToReportsMultipleManagers(t *testing.T) {
	stub := &stubPeoplesvc{
		users: []domain.User{{UID: "alice-uid", FullName: "Alice", Email: "alice@x"}},
		searchUsers: []domain.User{
			{UID: "u1", ReportsToUID: "alice-uid"},
			{UID: "u2", ReportsToUID: "uid-me"},
			{UID: "u3", ReportsToUID: "alice-uid"},
			{UID: "u4", ReportsToUID: "someone-else"},
		},
	}
	got, err := expandManagersToReports(context.Background(), stub, "default", "uid-me", []string{"me", "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 reports, got %d: %v", len(got), got)
	}
}

func TestExpandManagersToReportsErrorPropagates(t *testing.T) {
	stub := &stubPeoplesvc{searchErr: errors.New("boom")}
	_, err := expandManagersToReports(context.Background(), stub, "default", "uid-me", []string{"me"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want propagated error, got %v", err)
	}
}
