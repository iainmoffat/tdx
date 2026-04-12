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

func TestAddCmd_TicketSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":10,"Name":"Development","Active":true,"Billable":false}]`))

		case r.URL.Path == "/TDWebApi/api/time/locked":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ID":1,"PeriodStartDate":"2026-04-05T00:00:00Z","PeriodEndDate":"2026-04-11T00:00:00Z","Status":0,"Times":[],"TimeReportUid":"user-abc","UserFullName":"Test User","MinutesBillable":0,"MinutesNonBillable":0,"MinutesTotal":0,"TimeEntriesCount":0}`))

		case r.URL.Path == "/TDWebApi/api/time" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":999}],"Failed":[]}`))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID": 999,
				"ItemID": 100,
				"ItemTitle": "Some ticket",
				"AppID": 5,
				"Component": 9,
				"TicketID": 100,
				"TimeDate": "2026-04-11T00:00:00Z",
				"Minutes": 60,
				"Description": "did work",
				"TimeTypeID": 10,
				"TimeTypeName": "Development",
				"Status": 0,
				"Billable": false
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
	cmd.SetArgs([]string{
		"add",
		"--date", "2026-04-11",
		"--hours", "1",
		"--type", "Development",
		"--ticket", "100",
		"--app", "5",
		"-d", "did work",
	})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "created entry 999")
}

func TestAddCmd_ProjectTaskSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":10,"Name":"Development","IsActive":true,"IsBillable":false}]`))

		case r.URL.Path == "/TDWebApi/api/time/locked":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ID":1,"PeriodStartDate":"2026-04-05T00:00:00Z","PeriodEndDate":"2026-04-11T00:00:00Z","Status":0,"Times":[],"TimeReportUid":"user-abc","UserFullName":"Test User","MinutesBillable":0,"MinutesNonBillable":0,"MinutesTotal":0,"TimeEntriesCount":0}`))

		case r.URL.Path == "/TDWebApi/api/time" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":888}],"Failed":[]}`))

		case r.URL.Path == "/TDWebApi/api/time/888" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID": 888,
				"ItemID": 2612,
				"ItemTitle": "Task",
				"AppID": 0,
				"Component": 2,
				"TicketID": 0,
				"ProjectID": 54,
				"ProjectName": "Proj",
				"PlanID": 2091,
				"TimeDate": "2026-04-11T00:00:00Z",
				"Minutes": 30,
				"Description": "task work",
				"TimeTypeID": 10,
				"TimeTypeName": "Development",
				"Status": 0,
				"Billable": false
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
	cmd.SetArgs([]string{
		"add",
		"--date", "2026-04-11",
		"--minutes", "30",
		"--type", "Development",
		"--project", "54",
		"--plan", "2091",
		"--task", "2612",
		"-d", "task work",
	})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "created entry 888")
}

func TestAddCmd_LockedDayRejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":10,"Name":"Development","IsActive":true,"IsBillable":false}]`))

		case "/TDWebApi/api/time/locked":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`["2026-04-11T00:00:00Z"]`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"add",
		"--date", "2026-04-11",
		"--hours", "1",
		"--type", "Development",
		"--ticket", "100",
		"--app", "5",
	})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "day is locked")
}

func TestAddCmd_MissingRequiredFlags(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing date",
			args:    []string{"add", "--hours", "1", "--type", "Dev", "--ticket", "1", "--app", "1"},
			wantErr: "--date is required",
		},
		{
			name:    "missing hours and minutes",
			args:    []string{"add", "--date", "2026-04-11", "--type", "Dev", "--ticket", "1", "--app", "1"},
			wantErr: "exactly one of --hours or --minutes",
		},
		{
			name:    "both hours and minutes",
			args:    []string{"add", "--date", "2026-04-11", "--hours", "1", "--minutes", "30", "--type", "Dev", "--ticket", "1", "--app", "1"},
			wantErr: "exactly one of --hours or --minutes",
		},
		{
			name:    "missing type",
			args:    []string{"add", "--date", "2026-04-11", "--hours", "1", "--ticket", "1", "--app", "1"},
			wantErr: "--type is required",
		},
		{
			name:    "no target",
			args:    []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev"},
			wantErr: "exactly one of --ticket, --project, or --workspace",
		},
		{
			name:    "ticket without app",
			args:    []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev", "--ticket", "1"},
			wantErr: "--app is required with --ticket",
		},
		{
			name:    "plan without task",
			args:    []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev", "--project", "1", "--plan", "2"},
			wantErr: "--plan requires both --project and --task",
		},
		{
			name:    "task without plan",
			args:    []string{"add", "--date", "2026-04-11", "--hours", "1", "--type", "Dev", "--project", "1", "--task", "3"},
			wantErr: "--task with --project requires --plan",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedProfile(t, "http://127.0.0.1/")

			var out bytes.Buffer
			cmd := NewCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			require.Error(t, err)
			require.Contains(t, err.Error()+out.String(), tc.wantErr)
		})
	}
}

func TestAddCmd_DryRun(t *testing.T) {
	var postCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":10,"Name":"Development","Active":true,"Billable":false}]`))

		case r.URL.Path == "/TDWebApi/api/time/locked":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ID":1,"PeriodStartDate":"2026-04-05T00:00:00Z","PeriodEndDate":"2026-04-11T00:00:00Z","Status":0,"Times":[],"TimeReportUid":"user-abc","UserFullName":"Test User","MinutesBillable":0,"MinutesNonBillable":0,"MinutesTotal":0,"TimeEntriesCount":0}`))

		case r.URL.Path == "/TDWebApi/api/time" && r.Method == "POST":
			postCalled.Store(true)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":999}],"Failed":[]}`))

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
	cmd.SetArgs([]string{
		"add",
		"--date", "2026-04-11",
		"--hours", "1",
		"--type", "Development",
		"--ticket", "100",
		"--app", "5",
		"-d", "did work",
		"--dry-run",
	})
	require.NoError(t, cmd.Execute())

	require.False(t, postCalled.Load(), "POST should not be called during dry run")
	got := out.String()
	require.Contains(t, got, "dry run")
}
