package entry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntryShow_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/987654", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"TimeID": 987654,
			"ItemID": 12345,
			"ItemTitle": "Ingest pipeline",
			"AppID": 42,
			"Component": 9,
			"TicketID": 12345,
			"TimeDate": "2026-04-06T00:00:00Z",
			"Minutes": 120,
			"Description": "Investigating the ingest bug",
			"TimeTypeID": 1,
			"TimeTypeName": "Development",
			"Status": 0
		}`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "987654"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "entry:        987654")
	require.Contains(t, got, "date:         2026-04-06")
	require.Contains(t, got, "hours:        2.00")
	require.Contains(t, got, "type:         Development")
	require.Contains(t, got, "target:       #12345 Ingest pipeline")
	require.Contains(t, got, "description:  Investigating the ingest bug")
}

func TestEntryShow_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	cmd := NewCmd()
	cmd.SetArgs([]string{"show", "999"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "entry 999 not found")
}

func TestEntryShow_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-06T00:00:00Z","Minutes":120,"TimeTypeID":1,"TimeTypeName":"Development","Status":0}`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "1", "--json"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), `"schema": "tdx.v1.entry"`)
}

func TestEntryShow_RejectsNonPositiveID(t *testing.T) {
	seedProfile(t, "http://127.0.0.1/")

	cases := []struct {
		name string
		args []string
	}{
		{"zero", []string{"show", "0"}},
		{"negative", []string{"show", "--", "-5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := NewCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			require.Error(t, err)
			require.Contains(t, err.Error(), "must be a positive integer")
		})
	}
}
