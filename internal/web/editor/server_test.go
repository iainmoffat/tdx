package editor

import (
	"encoding/json"
	"io"
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
				GroupName: "Sample Dept",
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

const (
	testNonce      = "test-nonce"
	testListenAddr = "127.0.0.1:8080"
)

// newServerWithNonce constructs a server for testing with deterministic
// nonce and listenAddr so the handler-level helpers can be exercised
// without going through Run().
func newServerWithNonce(t *testing.T, sheet editor.Sheet, save SaveFn) *server {
	t.Helper()
	s := newServer(sheet, save)
	s.nonce = testNonce
	s.listenAddr = testListenAddr
	return s
}

// newAPIRequest builds a request to /api/* with the headers an authorized
// browser session would send. Individual reject-path tests override one
// field at a time.
func newAPIRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, path, rdr)
	r.Host = testListenAddr
	r.Header.Set("Origin", "http://"+testListenAddr)
	r.Header.Set("X-Tdx-Session", testNonce)
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func TestGetIndex_ServesHTML(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := httptest.NewRequest(http.MethodGet, "/?s="+testNonce, nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "row-01")
	require.Contains(t, w.Body.String(), "Admin")
}

func TestGetSheet_ReturnsJSON(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodGet, "/api/template", "")
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
	srv := newServerWithNonce(t, testSheet(), saveFn)

	body := `{"rows":[
		{"id":"row-01","hours":{"sun":0,"mon":4,"tue":4,"wed":4,"thu":4,"fri":4,"sat":0}},
		{"id":"row-02","hours":{"sun":0,"mon":0,"tue":2,"wed":0,"thu":0,"fri":0,"sat":0}}
	]}`
	req := newAPIRequest(t, http.MethodPost, "/api/save", body)
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
	srv := newServerWithNonce(t, testSheet(), saveFn)

	// Cancel now sends a JSON body so Content-Type check passes when Task 3 lands.
	req := newAPIRequest(t, http.MethodPost, "/api/cancel", "{}")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, saveCalled)
}

func TestGetIndex_RejectsMissingSessionQuery(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "invalid session")
}

func TestGetIndex_RejectsWrongSessionQuery(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := httptest.NewRequest(http.MethodGet, "/?s=wrong", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetIndex_AcceptsValidSessionQuery(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := httptest.NewRequest(http.MethodGet, "/?s="+testNonce, nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `<meta name="tdx-session" content="`+testNonce+`">`)
}
