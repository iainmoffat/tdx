package projectsvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetFeed_DecodesEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/projects/259/feed", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		_, _ = w.Write([]byte(`[
			{"ID":1782210,"Body":"Changed Portfolio from A to B","CreatedUid":"uid-sys","CreatedFullName":"API User","CreatedDate":"2026-05-07T18:35:38.877","UpdateType":3,"IsPrivate":false,"LikesCount":0,"RepliesCount":0},
			{"ID":1782180,"Body":"Backup review complete","CreatedUid":"uid-user","CreatedFullName":"Sample User","CreatedDate":"2026-05-06T14:12:00","UpdateType":1,"IsPrivate":false,"LikesCount":1,"RepliesCount":2}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)

	entries, err := svc.GetFeed(context.Background(), prof, 259)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// System event
	e0 := entries[0]
	require.Equal(t, 1782210, e0.ID)
	require.Equal(t, "uid-sys", e0.CreatedByUID)
	require.Equal(t, "API User", e0.CreatedByName)
	require.Equal(t, 3, e0.UpdateType)
	require.Equal(t, "system", e0.UpdateTypeLabel())
	require.False(t, e0.CreatedDate.IsZero())

	// Comment
	e1 := entries[1]
	require.Equal(t, 1782180, e1.ID)
	require.Equal(t, "uid-user", e1.CreatedByUID)
	require.Equal(t, 1, e1.UpdateType)
	require.Equal(t, "comment", e1.UpdateTypeLabel())
	require.Equal(t, 1, e1.LikesCount)
	require.Equal(t, 2, e1.RepliesCount)
}

func TestAddFeed_PostsExpectedBody(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/projects/259/feed", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID":9999,"Body":"hello"}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)

	id, err := svc.AddFeed(context.Background(), prof, 259, "hello world", false, []string{"uid-a", "uid-b"})
	require.NoError(t, err)
	require.Equal(t, 9999, id)

	var sent map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &sent))
	// Project feed POST uses "Body" field, not "Comments" — confirmed by live probe.
	require.Equal(t, "hello world", sent["Body"])
	require.Nil(t, sent["Comments"], "project feed POST must NOT send Comments")
	require.Equal(t, false, sent["IsPrivate"])
	notify, _ := sent["Notify"].([]interface{})
	require.Len(t, notify, 2)
	require.Equal(t, "uid-a", notify[0])
}

func TestGetTaskFeed_DecodesEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/projects/259/plans/1292/tasks/4938/feed", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		_, _ = w.Write([]byte(`[
			{"ID":1782260,"Body":"Task comment","CreatedUid":"uid-user","CreatedFullName":"Dev User","CreatedDate":"2026-05-10T10:00:00","UpdateType":1}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)

	entries, err := svc.GetTaskFeed(context.Background(), prof, 259, 1292, 4938)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, 1782260, entries[0].ID)
	require.Equal(t, "Dev User", entries[0].CreatedByName)
	require.Equal(t, "comment", entries[0].UpdateTypeLabel())
}

func TestAddTaskFeed_PostsExpectedBody(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/projects/259/plans/1292/tasks/4938/feed", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID":8888,"Body":"task note"}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)

	id, err := svc.AddTaskFeed(context.Background(), prof, 259, 1292, 4938, "task note", true, nil)
	require.NoError(t, err)
	require.Equal(t, 8888, id)

	var sent map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &sent))
	require.Equal(t, "task note", sent["Comments"])
	require.Equal(t, true, sent["IsPrivate"])
}
