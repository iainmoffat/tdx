package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// applyTestServer returns an httptest.Server that serves the endpoints needed
// by reconcile: getuser, week report, locked days, and time types.
func applyTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1}`))

		case r.Method == http.MethodGet && len(r.URL.Path) > len("/TDWebApi/api/time/report/"):
			// Empty week — no existing entries.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ID": 1,
				"PeriodStartDate": "2026-04-05T00:00:00Z",
				"PeriodEndDate": "2026-04-11T00:00:00Z",
				"Status": 0,
				"TimeReportUid": "uid-abc",
				"UserFullName": "Test User",
				"MinutesBillable": 0,
				"MinutesNonBillable": 0,
				"MinutesTotal": 0,
				"TimeEntriesCount": 0,
				"Times": []
			}`))

		case r.URL.Path == "/TDWebApi/api/time/locked":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Dev","IsActive":true}]`))

		default:
			t.Logf("applyTestServer: unhandled %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
}

func saveTestTemplate(t *testing.T, svcs Services) {
	t.Helper()
	require.NoError(t, svcs.Template.Store().Save(domain.Template{
		SchemaVersion: 1,
		Name:          "test-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-01",
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType: domain.TimeType{ID: 5, Name: "Dev"},
				Hours: domain.WeekHours{
					Mon: 2.0, Tue: 2.0, Wed: 2.0, Thu: 2.0, Fri: 2.0,
				},
				Description: "work",
			},
		},
	}))
}

func TestCompareTemplate(t *testing.T) {
	srv := applyTestServer(t)
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	saveTestTemplate(t, svcs)

	handler := compareHandler(svcs)
	result, _, err := handler(context.Background(), nil, compareArgs{
		Name: "test-tmpl",
		Week: "2026-04-08",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	text := extractText(t, result)
	var resp reconcileResult
	require.NoError(t, json.Unmarshal([]byte(text), &resp))
	require.NotEmpty(t, resp.Actions, "expected actions in diff")
	require.Equal(t, 5, resp.Creates)
	require.Equal(t, 0, resp.Skips)
	require.NotEmpty(t, resp.DiffHash)
}

func TestPreviewApply(t *testing.T) {
	srv := applyTestServer(t)
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	saveTestTemplate(t, svcs)

	handler := previewHandler(svcs)
	result, _, err := handler(context.Background(), nil, previewArgs{
		Name: "test-tmpl",
		Week: "2026-04-08",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	text := extractText(t, result)
	var resp reconcileResult
	require.NoError(t, json.Unmarshal([]byte(text), &resp))
	require.NotEmpty(t, resp.DiffHash)
	require.Equal(t, 5, resp.Creates)
}

func TestParseDaysFilter(t *testing.T) {
	tests := []struct {
		input string
		want  int
		err   bool
	}{
		{"", 0, false},
		{"mon-thu", 4, false},
		{"mon,wed,fri", 3, false},
		{"xyz", 0, true},
		{"mon-xyz", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			days, err := parseDaysFilter(tt.input)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, days, tt.want)
			}
		})
	}
}

func TestParseOverrides(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		overrides, err := parseOverrides([]string{"row-01:fri=4", "row-02:mon=1.5"})
		require.NoError(t, err)
		require.Len(t, overrides, 2)
	})

	t.Run("invalid_format", func(t *testing.T) {
		_, err := parseOverrides([]string{"bad"})
		require.Error(t, err)
	})

	t.Run("invalid_day", func(t *testing.T) {
		_, err := parseOverrides([]string{"row-01:xyz=4"})
		require.Error(t, err)
	})

	t.Run("invalid_hours", func(t *testing.T) {
		_, err := parseOverrides([]string{"row-01:fri=abc"})
		require.Error(t, err)
	})
}
