package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunAppListTable(t *testing.T) {
	stub := &stubTicketsvc{
		apps: []domain.TicketApp{
			{ID: 31, Name: "Service Desk", Description: "Help desk", Active: true},
			{ID: 71, Name: "Project Tickets", Description: "PM", Active: true},
		},
	}
	var buf bytes.Buffer
	if err := runAppList(context.Background(), &buf, stub, "default", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"31", "Service Desk", "71", "Project Tickets"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestRunAppListJSON(t *testing.T) {
	stub := &stubTicketsvc{apps: []domain.TicketApp{{ID: 31, Name: "Service Desk", Active: true, AppType: "TDNext"}}}
	var buf bytes.Buffer
	if err := runAppList(context.Background(), &buf, stub, "default", true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got["schema"] != "tdx.v1.ticketAppList" {
		t.Fatalf("schema: %v", got["schema"])
	}
	apps, _ := got["apps"].([]interface{})
	if len(apps) != 1 {
		t.Fatalf("apps len: %v", apps)
	}
}

func TestRunAppListEmpty(t *testing.T) {
	stub := &stubTicketsvc{apps: nil}
	var buf bytes.Buffer
	if err := runAppList(context.Background(), &buf, stub, "default", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no ticket apps found") {
		t.Errorf("missing empty message: %s", buf.String())
	}
}

// stubProfileStore satisfies profileStoreAPI for tests of runAppUse/runAppShow.
type stubProfileStore struct {
	profile   domain.Profile
	updatedID int
	getErr    error
	updateErr error
}

func (s *stubProfileStore) GetProfile(_ string) (domain.Profile, error) {
	return s.profile, s.getErr
}
func (s *stubProfileStore) UpdateProfile(p domain.Profile) error {
	s.updatedID = p.TicketAppID
	return s.updateErr
}

func TestRunAppUseSuccess(t *testing.T) {
	store := &stubProfileStore{profile: domain.Profile{Name: "default", TenantBaseURL: "https://x.example.com/"}}
	var buf bytes.Buffer
	if err := runAppUse(&buf, store, "default", 42); err != nil {
		t.Fatal(err)
	}
	if store.updatedID != 42 {
		t.Errorf("UpdateProfile called with wrong id: %d", store.updatedID)
	}
	if !strings.Contains(buf.String(), "→ app 42") {
		t.Errorf("output: %s", buf.String())
	}
}

func TestRunAppUseGetError(t *testing.T) {
	store := &stubProfileStore{getErr: errors.New("boom")}
	err := runAppUse(&bytes.Buffer{}, store, "default", 1)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want propagated error, got %v", err)
	}
}

func TestRunAppShowSet(t *testing.T) {
	store := &stubProfileStore{profile: domain.Profile{Name: "default", TenantBaseURL: "https://x.example.com/", TicketAppID: 31}}
	var buf bytes.Buffer
	if err := runAppShow(&buf, store, "default"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "31") {
		t.Errorf("output should mention 31: %s", buf.String())
	}
}

func TestRunAppShowUnset(t *testing.T) {
	store := &stubProfileStore{profile: domain.Profile{Name: "default", TenantBaseURL: "https://x.example.com/"}}
	var buf bytes.Buffer
	if err := runAppShow(&buf, store, "default"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no ticket app set") {
		t.Errorf("output: %s", buf.String())
	}
}
