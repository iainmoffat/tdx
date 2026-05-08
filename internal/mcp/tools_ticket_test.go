package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

// mcpTicketHarness builds a Services backed by a test HTTP server,
// including a Tickets service pointing at the same temp dir.
func mcpTicketHarness(t *testing.T, tenantURL string) Services {
	t.Helper()
	dir := t.TempDir()
	paths := config.Paths{
		Root:               dir,
		ConfigFile:         filepath.Join(dir, "config.yaml"),
		CredentialsFile:    filepath.Join(dir, "credentials.yaml"),
		LegacyTemplatesDir: filepath.Join(dir, "templates"),
	}
	ps := config.NewProfileStore(paths)
	require.NoError(t, ps.AddProfile(domain.Profile{
		Name:          "default",
		TenantBaseURL: tenantURL,
		TicketAppID:   101,
	}))
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "good-token"))

	return Services{
		Auth:     authsvc.New(paths),
		Time:     timesvc.New(paths),
		Template: tmplsvc.New(paths, timesvc.New(paths)),
		Drafts:   draftsvc.NewService(paths, timesvc.New(paths)),
		People:   peoplesvc.New(paths),
		Tickets:  ticketsvc.New(paths),
		Profile:  "default",
	}
}

// TestRegisterTicketTools_NoPanic verifies that registering all ticket tools
// does not panic with an empty Services{}.
func TestRegisterTicketTools_NoPanic(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTicketTools(srv, Services{})
	RegisterTicketMutatingTools(srv, Services{})
	// If we get here, no panic.
}

// TestListTicketApps_SchemaName verifies the schema name and envelope shape.
func TestListTicketApps_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/applications":
			_, _ = w.Write([]byte(`[
				{"AppID":101,"Name":"Service Desk","Active":true,"AppClass":"TDTickets","Type":"TDNext"},
				{"AppID":200,"Name":"Projects","Active":true,"AppClass":"TDProjects","Type":"TDNext"}
			]`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpTicketHarness(t, ts.URL)
	res, _, err := listTicketAppsHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listTicketAppsArgs{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.ticketAppList", got["schema"])
	apps, ok := got["apps"].([]any)
	require.True(t, ok)
	require.Len(t, apps, 1, "only ticketing apps (AppClass containing 'ticket') returned")
}

// TestSearchTickets_SchemaName verifies the ticketList schema envelope.
func TestSearchTickets_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/101/tickets/search":
			_, _ = w.Write([]byte(`[
				{"ID":42,"Title":"Fix the thing","StatusName":"Open","TypeName":"Incident","ResponsibleFullName":"Bob","RequestorName":"Alice"}
			]`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpTicketHarness(t, ts.URL)
	res, _, err := searchTicketsHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, searchTicketsArgs{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.ticketList", got["schema"])
	tickets, ok := got["tickets"].([]any)
	require.True(t, ok)
	require.Len(t, tickets, 1)
	first, ok := tickets[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Fix the thing", first["title"])
}

// TestGetTicket_SchemaName verifies the ticket schema envelope.
func TestGetTicket_SchemaName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/101/tickets/42":
			_, _ = w.Write([]byte(`{"ID":42,"Title":"Fix the thing","StatusName":"Open"}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svcs := mcpTicketHarness(t, ts.URL)
	res, _, err := getTicketHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, getTicketArgs{ID: 42})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.ticket", got["schema"])
	ticket, ok := got["ticket"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Fix the thing", ticket["title"])
}

// TestAddTicketComment_RequiresConfirm verifies that omitting confirm=true
// returns an error result without posting.
func TestAddTicketComment_RequiresConfirm(t *testing.T) {
	posted := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svcs := mcpTicketHarness(t, ts.URL)
	res, _, err := addTicketCommentHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, addTicketCommentArgs{
		ID:       42,
		Comments: "test comment",
		Confirm:  false, // explicitly not confirmed
	})
	require.NoError(t, err)
	require.True(t, res.IsError, "should be error result when confirm=false")
	require.False(t, posted, "should not have POSTed to the server")
}

// TestUpdateTicketStatus_RequiresConfirm verifies the confirm gate.
func TestUpdateTicketStatus_RequiresConfirm(t *testing.T) {
	svcs := mcpTicketHarness(t, "http://localhost:0") // no requests expected
	res, _, err := updateTicketStatusHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, updateTicketStatusArgs{
		ID:       42,
		StatusID: 5,
		Confirm:  false,
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
}

// TestLogTicketTime_RequiresConfirm verifies the confirm gate for time logging.
func TestLogTicketTime_RequiresConfirm(t *testing.T) {
	svcs := mcpTicketHarness(t, "http://localhost:0")
	res, _, err := logTicketTimeHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, logTicketTimeArgs{
		ID:      42,
		Hours:   1.0,
		TypeID:  10,
		Confirm: false,
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
}
