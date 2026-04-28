package editor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	"github.com/stretchr/testify/require"
)

func testSheet() editor.Sheet {
	return editor.Sheet{
		Name: "test-sheet",
		Rows: []editor.SheetRow{
			{
				ID:        "row-01",
				Label:     "Admin",
				GroupName: "UFIT",
				TypeName:  "Standard",
				Hours:     domain.WeekHours{Mon: 8.0, Tue: 8.0, Wed: 8.0, Thu: 8.0, Fri: 8.0},
			},
			{
				ID:        "row-02",
				Label:     "Docker",
				GroupName: "Ops",
				TypeName:  "Standard",
				Hours:     domain.WeekHours{Tue: 1.0},
			},
		},
	}
}

func TestGetIndex_ServesHTML(t *testing.T) {
	srv := newServer(testSheet(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "row-01")
	require.Contains(t, w.Body.String(), "Admin")
}

func TestGetSheet_ReturnsJSON(t *testing.T) {
	srv := newServer(testSheet(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/template", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp templateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "test-sheet", resp.Name)
	require.Len(t, resp.Rows, 2)
	require.Equal(t, "row-01", resp.Rows[0].ID)
	require.InDelta(t, 8.0, resp.Rows[0].Hours.Mon, 0.001)
}

func TestPostSave_UpdatesSheet(t *testing.T) {
	var saved *editor.Sheet
	saveFn := func(s editor.Sheet) error {
		copy := s
		saved = &copy
		return nil
	}
	srv := newServer(testSheet(), saveFn)

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
	saveFn := func(s editor.Sheet) error {
		saveCalled = true
		return nil
	}
	srv := newServer(testSheet(), saveFn)

	req := httptest.NewRequest(http.MethodPost, "/api/cancel", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, saveCalled)
}
