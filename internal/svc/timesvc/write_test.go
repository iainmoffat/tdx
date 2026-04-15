package timesvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAddEntry_Success(t *testing.T) {
	postCalled := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time":
			postCalled++
			body, _ := io.ReadAll(r.Body)
			var entries []wireTimeEntryWrite
			require.NoError(t, json.Unmarshal(body, &entries))
			require.Len(t, entries, 1, "POST body must be 1-element array")
			require.Equal(t, "uid-abc", entries[0].Uid)
			require.Equal(t, componentTaskTime, entries[0].Component)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":999}],"Failed":[]}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":2612,"ItemTitle":"Test Task",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":0,"AppName":"None","Component":2,
				"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
				"PlanID":2091,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":15.0,"Description":"test desc",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":70,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	input := domain.EntryInput{
		UserUID:     "uid-abc",
		Date:        time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Minutes:     15,
		TimeTypeID:  5,
		Billable:    false,
		Target:      domain.Target{Kind: domain.TargetProjectTask, ItemID: 2091, TaskID: 2612, ProjectID: 54},
		Description: "test desc",
	}
	entry, err := svc.AddEntry(context.Background(), profile, input)
	require.NoError(t, err)
	require.Equal(t, 999, entry.ID)
	require.Equal(t, 15, entry.Minutes)
	require.Equal(t, 1, postCalled)
}

func TestAddEntry_ServerFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Succeeded":[],"Failed":[{"Index":0,"TimeEntryID":0,"ErrorMessage":"Day is locked","ErrorCode":40,"ErrorCodeName":"DayLocked"}]}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	input := domain.EntryInput{
		UserUID:    "uid-abc",
		Date:       time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Minutes:    15,
		TimeTypeID: 5,
		Target:     domain.Target{Kind: domain.TargetTicket, AppID: 5, ItemID: 100},
	}
	_, err := svc.AddEntry(context.Background(), profile, input)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "Day is locked"), "error should contain 'Day is locked', got: %v", err)
}

func TestUpdateEntry_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":2612,"ItemTitle":"Test Task",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":0,"AppName":"None","Component":2,
				"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
				"PlanID":2091,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"original",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":70,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodPut && r.URL.Path == "/TDWebApi/api/time/999":
			body, _ := io.ReadAll(r.Body)
			require.True(t, strings.Contains(string(body), `"Description":"updated"`),
				"PUT body should contain updated description, got: %s", body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":2612,"ItemTitle":"Test Task",
				"Uid":"uid-abc","TimeTypeID":5,"TimeTypeName":"","Billable":false,
				"AppID":0,"AppName":"None","Component":2,
				"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
				"PlanID":2091,"TimeDate":"2026-04-11T00:00:00Z",
				"Minutes":60.0,"Description":"updated",
				"Status":0,"StatusDate":"0001-01-01T00:00:00",
				"PortfolioID":0,"Limited":false,"FunctionalRoleId":70,
				"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Development","IsActive":true}]`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	desc := "updated"
	update := domain.EntryUpdate{Description: &desc}
	entry, err := svc.UpdateEntry(context.Background(), profile, 999, update)
	require.NoError(t, err)
	require.Equal(t, 999, entry.ID)
	require.Equal(t, "updated", entry.Description)
}

func TestUpdateEntry_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`"Time entry not found."`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	desc := "updated"
	update := domain.EntryUpdate{Description: &desc}
	_, err := svc.UpdateEntry(context.Background(), profile, 9999, update)
	require.Error(t, err)
}

func TestDeleteEntry_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/TDWebApi/api/time/999", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	err := svc.DeleteEntry(context.Background(), profile, 999)
	require.NoError(t, err)
}

func TestDeleteEntry_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	err := svc.DeleteEntry(context.Background(), profile, 9999)
	require.Error(t, err)
}

func TestDeleteEntries_AllSucceed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/TDWebApi/api/time/delete", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1},{"Index":1,"ID":2}],"Failed":[]}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	result, err := svc.DeleteEntries(context.Background(), profile, []int{1, 2})
	require.NoError(t, err)
	require.True(t, result.FullSuccess())
	require.Len(t, result.Succeeded, 2)
}

func TestDeleteEntries_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":1}],"Failed":[{"Index":1,"TimeEntryID":2,"ErrorMessage":"Could not find a time entry with an ID of 2","ErrorCode":10,"ErrorCodeName":"InvalidTimeEntryID"}]}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	result, err := svc.DeleteEntries(context.Background(), profile, []int{1, 2})
	require.NoError(t, err)
	require.True(t, result.PartialSuccess())
	require.Len(t, result.Failed, 1)
	require.Equal(t, 2, result.Failed[0].ID)
}

func TestDeleteEntries_AutoSplitAt50(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var ids []int
		require.NoError(t, json.Unmarshal(body, &ids))
		require.LessOrEqual(t, len(ids), 50, "batch size must not exceed 50")

		var succeeded []wireBulkSuccess
		for i, id := range ids {
			succeeded = append(succeeded, wireBulkSuccess{Index: i, ID: id})
		}
		result := wireBulkResult{Succeeded: succeeded}
		resp, _ := json.Marshal(result)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
	}))
	defer srv.Close()

	ids := make([]int, 51)
	for i := range ids {
		ids[i] = i + 1
	}

	svc, profile := harness(t, srv.URL)
	result, err := svc.DeleteEntries(context.Background(), profile, ids)
	require.NoError(t, err)
	require.Equal(t, 2, callCount, "51 IDs should require 2 API calls")
	require.Len(t, result.Succeeded, 51)
}
