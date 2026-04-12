package entry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteCmd_SingleSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)

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
	cmd.SetArgs([]string{"delete", "999"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "deleted entry 999")
}

func TestDeleteCmd_SingleNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/9999" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNotFound)

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
	cmd.SetArgs([]string{"delete", "9999"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestDeleteCmd_MultiSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/delete" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1},{"Index":1,"ID":2},{"Index":2,"ID":3}],"Failed":[]}`))

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
	cmd.SetArgs([]string{"delete", "1", "2", "3"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "deleted 3 entries")
}

func TestDeleteCmd_MultiPartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/delete" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1}],"Failed":[{"TimeEntryID":2,"ErrorMessage":"entry locked"}]}`))

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
	cmd.SetArgs([]string{"delete", "1", "2"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "partial")
}

func TestDeleteCmd_DryRunSingle(t *testing.T) {
	var deleteCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(wireEntryFixture))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","Active":true,"Billable":false}]`))

		case r.URL.Path == "/TDWebApi/api/time/999" && r.Method == http.MethodDelete:
			deleteCalled.Store(true)
			w.WriteHeader(http.StatusOK)

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
	cmd.SetArgs([]string{"delete", "999", "--dry-run"})
	require.NoError(t, cmd.Execute())

	require.False(t, deleteCalled.Load(), "DELETE should not be called during dry run")
	got := out.String()
	require.Contains(t, got, "dry run")
}
