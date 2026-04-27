package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestListTemplates(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")
	store := svcs.Template.Store()

	require.NoError(t, store.Save("default", domain.Template{
		SchemaVersion: 1,
		Name:          "alpha",
		Rows: []domain.TemplateRow{
			{ID: "r1", Hours: domain.WeekHours{Mon: 8}},
		},
	}))
	require.NoError(t, store.Save("default", domain.Template{
		SchemaVersion: 1,
		Name:          "beta",
		Rows: []domain.TemplateRow{
			{ID: "r2", Hours: domain.WeekHours{Tue: 4}},
		},
	}))

	handler := listTemplatesHandler(svcs)
	result, _, err := handler(context.Background(), nil, listTemplatesArgs{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var templates []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &templates))
	require.Len(t, templates, 2)
}

func TestGetTemplate(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")
	store := svcs.Template.Store()

	require.NoError(t, store.Save("default", domain.Template{
		SchemaVersion: 1,
		Name:          "test",
		Rows: []domain.TemplateRow{
			{ID: "r1", Hours: domain.WeekHours{Mon: 1}},
		},
	}))

	handler := getTemplateHandler(svcs)
	result, _, err := handler(context.Background(), nil, getTemplateArgs{Name: "test"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var tmpl map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &tmpl))
	require.Equal(t, "test", tmpl["name"])
}

func TestGetTemplate_NotFound(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")

	handler := getTemplateHandler(svcs)
	result, _, err := handler(context.Background(), nil, getTemplateArgs{Name: "nonexistent"})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestCreateTemplate_WithConfirm(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")

	handler := createTemplateHandler(svcs)
	rowsJSON := `[{"id":"row-01","target":{"kind":"project","itemID":54},"timeType":{"id":5,"name":"Dev"},"hours":{"mon":8}}]`

	result, _, err := handler(context.Background(), nil, createTemplateArgs{
		Name:        "new-tmpl",
		Description: "A test template",
		Rows:        rowsJSON,
		Confirm:     true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	// Verify saved to store.
	tmpl, loadErr := svcs.Template.Store().Load("default", "new-tmpl")
	require.NoError(t, loadErr)
	require.Equal(t, "new-tmpl", tmpl.Name)
	require.Equal(t, "A test template", tmpl.Description)
	require.Len(t, tmpl.Rows, 1)
}

func TestCreateTemplate_WithoutConfirm(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")

	handler := createTemplateHandler(svcs)
	result, _, err := handler(context.Background(), nil, createTemplateArgs{
		Name:    "new-tmpl",
		Rows:    `[{"id":"row-01","target":{"kind":"project","itemID":54},"timeType":{"id":5},"hours":{"mon":8}}]`,
		Confirm: false,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestDeleteTemplate_WithConfirm(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")

	// Save a template first.
	require.NoError(t, svcs.Template.Store().Save("default", domain.Template{
		SchemaVersion: 1,
		Name:          "to-delete",
		Rows: []domain.TemplateRow{
			{ID: "r1", Hours: domain.WeekHours{Mon: 8}},
		},
	}))

	handler := deleteTemplateHandler(svcs)
	result, _, err := handler(context.Background(), nil, deleteTemplateArgs{
		Name:    "to-delete",
		Confirm: true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	// Verify removed from store.
	require.False(t, svcs.Template.Store().Exists("default", "to-delete"))
}

func TestDeriveTemplate_WithConfirm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && len(r.URL.Path) > len("/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ID": 1,
				"PeriodStartDate": "2026-04-05T00:00:00Z",
				"PeriodEndDate": "2026-04-11T00:00:00Z",
				"Status": 0,
				"TimeReportUid": "uid-abc",
				"UserFullName": "Test User",
				"MinutesBillable": 0,
				"MinutesNonBillable": 480,
				"MinutesTotal": 480,
				"TimeEntriesCount": 1,
				"Times": [
					{"TimeID":1,"ItemID":54,"ItemTitle":"Project","AppID":0,"AppName":"","Component":1,"TicketID":0,"ProjectID":54,"TimeDate":"2026-04-07T00:00:00Z","Minutes":480,"TimeTypeID":5,"TimeTypeName":"Dev","Billable":false,"Status":0,"Uid":"uid-abc","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"}
				]
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Dev","IsActive":true}]`))

		default:
			t.Logf("unhandled: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := deriveTemplateHandler(svcs)
	result, _, err := handler(context.Background(), nil, deriveTemplateArgs{
		Name:     "derived-tmpl",
		FromWeek: "2026-04-08",
		Confirm:  true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	// Verify template was saved.
	tmpl, loadErr := svcs.Template.Store().Load("default", "derived-tmpl")
	require.NoError(t, loadErr)
	require.Equal(t, "derived-tmpl", tmpl.Name)
	require.NotEmpty(t, tmpl.Rows)
}
