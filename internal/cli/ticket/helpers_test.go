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
	users []domain.User
	err   error
}

func (s *stubPeoplesvc) LookupPeople(_ context.Context, _ string, _ string, _ int) ([]domain.User, error) {
	return s.users, s.err
}
