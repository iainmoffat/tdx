package editor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func testTemplate() domain.Template {
	return domain.Template{
		SchemaVersion: 1,
		Name:          "test-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-01",
				Label:    "Admin",
				Target:   domain.Target{Kind: domain.TargetProjectTask, GroupName: "UFIT"},
				TimeType: domain.TimeType{ID: 5, Name: "Standard"},
				Hours:    domain.WeekHours{Mon: 8.0, Tue: 8.0, Wed: 8.0, Thu: 8.0, Fri: 8.0},
			},
			{
				ID:       "row-02",
				Label:    "Docker",
				Target:   domain.Target{Kind: domain.TargetProjectTask, GroupName: "Ops"},
				TimeType: domain.TimeType{ID: 5, Name: "Standard"},
				Hours:    domain.WeekHours{Tue: 1.0},
			},
		},
	}
}

func TestGetIndex_ServesHTML(t *testing.T) {
	srv := newServer(testTemplate(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "tdx template editor")
	require.Contains(t, w.Body.String(), "row-01")
	require.Contains(t, w.Body.String(), "Admin")
}

func TestGetTemplate_ReturnsJSON(t *testing.T) {
	srv := newServer(testTemplate(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/template", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp templateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "test-tmpl", resp.Name)
	require.Len(t, resp.Rows, 2)
	require.Equal(t, "row-01", resp.Rows[0].ID)
	require.Equal(t, 8.0, resp.Rows[0].Hours.Mon)
}

func TestPostSave_UpdatesTemplate(t *testing.T) {
	var saved *domain.Template
	saveFn := func(tmpl domain.Template) error {
		saved = &tmpl
		return nil
	}
	srv := newServer(testTemplate(), saveFn)

	body := `{"rows":[
		{"id":"row-01","hours":{"sun":0,"mon":4,"tue":4,"wed":4,"thu":4,"fri":4,"sat":0}},
		{"id":"row-02","hours":{"sun":0,"mon":0,"tue":2,"wed":0,"thu":0,"fri":0,"sat":0}}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, saved)
	require.InDelta(t, 4.0, saved.Rows[0].Hours.Mon, 0.001)
	require.InDelta(t, 2.0, saved.Rows[1].Hours.Tue, 0.001)
}

func TestPostCancel_NoSave(t *testing.T) {
	saveCalled := false
	saveFn := func(tmpl domain.Template) error {
		saveCalled = true
		return nil
	}
	srv := newServer(testTemplate(), saveFn)

	req := httptest.NewRequest(http.MethodPost, "/api/cancel", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, saveCalled)
}
