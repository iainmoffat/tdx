package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListTimeTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/types" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"ID":1,"Name":"Development","Code":"DEV","HelpText":"Dev work","IsBillable":false,"IsLimited":false,"IsActive":true},
				{"ID":2,"Name":"Meetings","Code":"MTG","HelpText":"Meetings","IsBillable":false,"IsLimited":false,"IsActive":true}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := listTypesHandler(svcs)
	result, _, err := handler(context.Background(), nil, listTypesArgs{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var types []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &types))
	require.Len(t, types, 2)
	require.Equal(t, "Development", types[0]["name"])
	require.Equal(t, "Meetings", types[1]["name"])
}

func TestTypesForTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/types/component/app/42/ticket/12345" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"ID":1,"Name":"Development","Code":"DEV","HelpText":"Dev work","IsBillable":false,"IsLimited":false,"IsActive":true}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := typesForTargetHandler(svcs)
	result, _, err := handler(context.Background(), nil, typesForTargetArgs{
		Kind:   "ticket",
		ItemID: 12345,
		AppID:  42,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var types []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &types))
	require.Len(t, types, 1)
	require.Equal(t, "Development", types[0]["name"])
}
