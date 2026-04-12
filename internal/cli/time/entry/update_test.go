package entry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// wireEntryFixture is the canonical wire JSON for entry 999 used across
// update tests. Date is 2026-04-11 which falls inside the report week below.
const wireEntryFixture = `{
	"TimeID":999,"ItemID":100,"ItemTitle":"Ticket",
	"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
	"AppID":5,"AppName":"App","Component":9,
	"TicketID":100,"ProjectID":0,"ProjectName":"",
	"PlanID":0,"TimeDate":"2026-04-11T00:00:00Z",
	"Minutes":60.0,"Description":"original",
	"Status":0,"StatusDate":"0001-01-01T00:00:00",
	"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
	"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
}`

const wireReportFixture = `{"ID":1,"PeriodStartDate":"2026-04-05T00:00:00Z","PeriodEndDate":"2026-04-11T00:00:00Z","Status":0,"Times":[],"TimeReportUid":"uid","UserFullName":"User","MinutesBillable":0,"MinutesNonBillable":0,"MinutesTotal":0,"TimeEntriesCount":0}`

func TestUpdateCmd_Description(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","Active":true,"Billable":false}]`))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(wireEntryFixture))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/locked"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(wireReportFixture))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":100,"ItemTitle":"Ticket",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"Development","Billable":false,
				"AppID":5,"AppName":"App","Component":9,
				"TicketID":100,"ProjectID":0,"ProjectName":"",
				"PlanID":0,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"updated desc",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "999", "-d", "updated desc"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "updated entry 999")
}

func TestUpdateCmd_NothingToUpdate(t *testing.T) {
	// No server needed — validation is local before any network calls.
	seedProfile(t, "http://127.0.0.1/")

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "999"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error()+out.String(), "nothing to update")
}

func TestUpdateCmd_DryRun(t *testing.T) {
	var putCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","Active":true,"Billable":false}]`))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(wireEntryFixture))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/locked"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(wireReportFixture))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == http.MethodPut:
			putCalled.Store(true)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(wireEntryFixture))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "999", "-d", "new desc", "--dry-run"})
	require.NoError(t, cmd.Execute())

	require.False(t, putCalled.Load(), "PUT should not be called during dry run")
	got := out.String()
	require.Contains(t, got, "dry run")
}
