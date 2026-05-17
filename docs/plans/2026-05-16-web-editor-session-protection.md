# Web Editor Session Protection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lock down the per-edit-session HTTP server in `internal/web/editor/` so only the browser tab opened from the CLI can call `/api/save` and `/api/cancel`. Three defenses: per-session nonce (X-Tdx-Session header), Origin + Host check, Content-Type: application/json requirement. Plus an explicit `127.0.0.1` bind.

**Architecture:** A 32-byte random nonce is generated when `Run()` starts and stored on the `server` struct alongside the captured listen address. The page URL passed to `openBrowser` carries the nonce as `?s=`; `handleIndex` validates it and injects the nonce into the rendered HTML via a `<meta>` tag. Every `/api/*` request goes through a single `checkAPIRequest` helper that gates on Host, Origin, X-Tdx-Session, and (optionally) Content-Type. The browser JS reads the nonce from the meta tag and adds the header to every fetch.

**Tech Stack:** Go 1.26.2; `crypto/rand`, `crypto/subtle`, `encoding/base64` (all stdlib); testify/require. No new deps.

**Spec:** [`docs/specs/2026-05-16-web-editor-session-protection.md`](../specs/2026-05-16-web-editor-session-protection.md)

---

## File Structure

After this plan completes:

```
internal/
└── web/
    └── editor/
        ├── server.go                # MODIFY: server struct gains nonce+listenAddr;
        │                            #         Run() generates nonce + binds 127.0.0.1;
        │                            #         handleIndex validates ?s=; checkAPIRequest helper
        ├── server_test.go           # MODIFY: existing tests use new helpers;
        │                            #         12 new reject-path tests
        ├── embed.go                 # MODIFY: injectTemplateData also injects nonce <meta>
        └── static/
            └── editor.html          # MODIFY: <meta name="tdx-session"> placeholder;
                                     #         JS reads it; X-Tdx-Session header on all fetches;
                                     #         /api/cancel gains Content-Type + body

docs/
└── manual-tests/
    └── 2026-05-16-web-editor-session-protection-walkthrough.md   # CREATE
```

## Branch + Versioning

- Branch: `web-editor-session-protection` (Task 0)
- Version: **v0.22.0** — minor; behavior tightening. Existing CLI usage unchanged (the JS is updated as part of this PR).

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. (Main is at `2a61306` — the spec commit.)

- [ ] **Step 2: Create branch**

```bash
git checkout -b web-editor-session-protection
```

Expected: `Switched to a new branch 'web-editor-session-protection'`.

---

## Task 1: `server` struct + test helpers + Run() nonce/bind

This task changes Run()'s wire-level setup (nonce generation, 127.0.0.1 bind, ?s= URL) and adds new fields to the `server` struct. It does NOT yet wire any checks into the handlers — those come in Tasks 3 and 4. Behavior at this point: the page URL gains a `?s=NONCE` query string that is silently ignored by `handleIndex`; all `/api/*` routes still accept any request. Pre-existing tests continue to pass after we update them to use the new test helpers.

**Files:**
- Modify: `internal/web/editor/server.go`
- Modify: `internal/web/editor/server_test.go`

- [ ] **Step 1: Update the `server` struct**

In `internal/web/editor/server.go`, replace the existing `server` struct (around lines 19-24) with:

```go
// server holds the state for a single edit session.
type server struct {
	sheet      editor.Sheet
	save       SaveFn
	shutdown   chan result
	nonce      string // 43-char base64url-encoded 32-byte random; required on every /api/* request
	listenAddr string // "127.0.0.1:PORT" — captured after net.Listen; used in Host/Origin checks
}
```

- [ ] **Step 2: Update `Run()` for nonce generation and explicit IPv4 bind**

Replace the existing `Run` (around lines 160-185) with:

```go
// Run starts the HTTP server, opens the browser, and blocks until save
// or cancel. Returns whether the sheet was saved.
func Run(sheet editor.Sheet, save SaveFn) (Result, error) {
	srv := newServer(sheet, save)

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return Result{}, fmt.Errorf("nonce: %w", err)
	}
	srv.nonce = base64.RawURLEncoding.EncodeToString(nonce)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Result{}, fmt.Errorf("listen: %w", err)
	}
	srv.listenAddr = listener.Addr().String()

	url := fmt.Sprintf("http://%s/?s=%s", srv.listenAddr, srv.nonce)

	httpSrv := &http.Server{Handler: srv.handler()}
	go func() { _ = httpSrv.Serve(listener) }()

	if err := openBrowser(url); err != nil {
		_, _ = fmt.Printf("Could not open browser: %v\nOpen %s manually.\n", err, url)
	}

	res := <-srv.shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	return Result{Saved: res.saved}, res.err
}
```

Add the imports at the top of the file (replace the existing import block):

```go
import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/iainmoffat/tdx/internal/tui/editor"
)
```

- [ ] **Step 3: Add test helpers**

At the top of `internal/web/editor/server_test.go` (after the `testSheet` helper, around line 36), add:

```go
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
```

Add `"io"` to the file's import block.

- [ ] **Step 4: Update the 4 existing tests to use the helpers**

Replace the existing tests (`TestGetIndex_ServesHTML`, `TestGetSheet_ReturnsJSON`, `TestPostSave_UpdatesSheet`, `TestPostCancel_NoSave`) so they construct via `newServerWithNonce` and `newAPIRequest`. The index test additionally must include the `?s=test-nonce` query.

```go
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

	// Cancel now sends a JSON body so Content-Type check passes.
	req := newAPIRequest(t, http.MethodPost, "/api/cancel", "{}")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, saveCalled)
}
```

- [ ] **Step 5: Run tests — must PASS**

```bash
go test ./internal/web/editor/... -v
```

All 4 existing tests pass. (No reject-path tests yet; the handlers don't validate anything until Tasks 2 and 3.)

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 6: Commit**

```bash
git add internal/web/editor/server.go internal/web/editor/server_test.go
git commit -m "feat(web/editor): generate per-session nonce, bind 127.0.0.1, prep test helpers"
```

No `Co-Authored-By` trailer.

---

## Task 2: handleIndex validates `?s=` and HTML injects `<meta>` nonce

**Files:**
- Modify: `internal/web/editor/server.go` (`handleIndex`)
- Modify: `internal/web/editor/embed.go` (`injectTemplateData` extends to also inject nonce)
- Modify: `internal/web/editor/static/editor.html` (add `<meta>` placeholder)
- Modify: `internal/web/editor/server_test.go`

- [ ] **Step 1: Add the `<meta>` placeholder in the HTML**

In `internal/web/editor/static/editor.html`, after line 5 (the viewport meta), insert a new line:

```html
  <meta name="tdx-session" content="__TDX_SESSION__">
```

So lines 3-7 read:

```html
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="tdx-session" content="__TDX_SESSION__">
  <title>tdx template editor</title>
```

- [ ] **Step 2: Extend `injectTemplateData` to also inject the nonce**

In `internal/web/editor/embed.go`, replace the existing `injectTemplateData` with:

```go
// injectTemplateData replaces the placeholders in the HTML with actual
// template JSON data and the per-session nonce.
func injectTemplateData(html string, resp templateResponse, nonce string) (string, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(string(data), `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	out := strings.Replace(html, `"__TEMPLATE_JSON__"`, `"`+escaped+`"`, 1)
	out = strings.Replace(out, `__TDX_SESSION__`, htmlAttrEscape(nonce), 1)
	return out, nil
}

// htmlAttrEscape escapes a string for use inside a double-quoted HTML
// attribute. The nonce is base64-RawURL (A-Za-z0-9_-) so no replacements
// occur in practice, but the helper exists to make the safety property
// explicit and future-proof.
func htmlAttrEscape(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	return s
}
```

- [ ] **Step 3: Write failing tests for handleIndex**

Append to `internal/web/editor/server_test.go`:

```go
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
```

- [ ] **Step 4: Run tests — must FAIL**

```bash
go test ./internal/web/editor/... -run TestGetIndex -v
```

The `RejectsMissing` and `RejectsWrong` tests fail (handler doesn't check yet). The existing `TestGetIndex_ServesHTML` and the new `AcceptsValid` fail with a compile error or a missing-meta-tag error because `injectTemplateData`'s new signature isn't called with a nonce yet.

- [ ] **Step 5: Update `handleIndex` and its caller**

In `internal/web/editor/server.go`, replace `handleIndex` (around lines 75-87) with:

```go
func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Validate ?s= query param (constant-time). Browser navigation does not
	// send Origin / custom headers, so this is the gate that establishes the
	// session for the JS.
	got := r.URL.Query().Get("s")
	if subtle.ConstantTimeCompare([]byte(got), []byte(s.nonce)) != 1 {
		http.Error(w, "forbidden: invalid session", http.StatusForbidden)
		return
	}
	html, err := injectTemplateData(editorHTML, s.toResponse(), s.nonce)
	if err != nil {
		http.Error(w, "failed to render editor", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}
```

Add `"crypto/subtle"` to the file's import block.

- [ ] **Step 6: Run tests — must PASS**

```bash
go test ./internal/web/editor/... -v
```

All 4 existing + 3 new GET-index tests pass. The other tests (`TestGetSheet_*`, `TestPostSave_*`, `TestPostCancel_*`) still pass — they don't yet exercise `/api/*` checks, which come in Task 3.

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 7: Commit**

```bash
git add internal/web/editor/server.go internal/web/editor/embed.go \
        internal/web/editor/static/editor.html internal/web/editor/server_test.go
git commit -m "feat(web/editor): validate session nonce on GET / and inject into HTML"
```

---

## Task 3: `checkAPIRequest` helper + apply to GetSheet, Save, Cancel

**Files:**
- Modify: `internal/web/editor/server.go`
- Modify: `internal/web/editor/server_test.go`

- [ ] **Step 1: Write failing reject-path tests**

Append to `internal/web/editor/server_test.go`:

```go
func TestPostSave_RejectsMissingSession(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/save", `{"rows":[]}`)
	req.Header.Del("X-Tdx-Session")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "invalid session")
}

func TestPostSave_RejectsWrongSession(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/save", `{"rows":[]}`)
	req.Header.Set("X-Tdx-Session", "wrong")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestPostSave_RejectsMissingOrigin(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/save", `{"rows":[]}`)
	req.Header.Del("Origin")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "origin mismatch")
}

func TestPostSave_RejectsWrongOrigin(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/save", `{"rows":[]}`)
	req.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestPostSave_RejectsWrongHost(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/save", `{"rows":[]}`)
	req.Host = "evil.example"
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "host mismatch")
}

func TestPostSave_RejectsNonJSON(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/save", `{"rows":[]}`)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "content-type")
}

func TestPostSave_AcceptsJSONWithCharset(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), func(editor.Sheet) error { return nil })
	req := newAPIRequest(t, http.MethodPost, "/api/save", `{"rows":[]}`)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPostCancel_RejectsMissingSession(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/cancel", `{}`)
	req.Header.Del("X-Tdx-Session")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestPostCancel_RejectsMissingContentType(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodPost, "/api/cancel", `{}`)
	req.Header.Del("Content-Type")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetSheet_RejectsMissingSession(t *testing.T) {
	srv := newServerWithNonce(t, testSheet(), nil)
	req := newAPIRequest(t, http.MethodGet, "/api/template", "")
	req.Header.Del("X-Tdx-Session")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}
```

- [ ] **Step 2: Run tests — must FAIL**

```bash
go test ./internal/web/editor/... -run 'TestPostSave_Rejects|TestPostSave_AcceptsJSONWithCharset|TestPostCancel_Rejects|TestGetSheet_RejectsMissingSession' -v
```

All 10 new tests fail.

- [ ] **Step 3: Add `checkAPIRequest` and apply it to the three handlers**

In `internal/web/editor/server.go`, add the helper above `handleGetSheet` (so it lives next to its callers):

```go
// checkAPIRequest validates that an /api/* request comes from the
// session-authorized browser tab opened by Run(). Writes a 403 and returns
// false on any failure; otherwise returns true.
//
// Reject reasons are deliberately terse — same one-liner shape on each
// failure mode so an attacker probing the gate cannot learn which check
// failed.
func (s *server) checkAPIRequest(w http.ResponseWriter, r *http.Request, requireJSON bool) bool {
	if r.Host != s.listenAddr {
		http.Error(w, "forbidden: host mismatch", http.StatusForbidden)
		return false
	}
	if r.Header.Get("Origin") != "http://"+s.listenAddr {
		http.Error(w, "forbidden: origin mismatch", http.StatusForbidden)
		return false
	}
	if subtle.ConstantTimeCompare(
		[]byte(r.Header.Get("X-Tdx-Session")),
		[]byte(s.nonce),
	) != 1 {
		http.Error(w, "forbidden: invalid session", http.StatusForbidden)
		return false
	}
	if requireJSON {
		ct := r.Header.Get("Content-Type")
		if i := strings.IndexByte(ct, ';'); i >= 0 {
			ct = strings.TrimSpace(ct[:i])
		}
		if ct != "application/json" {
			http.Error(w, "forbidden: content-type must be application/json", http.StatusForbidden)
			return false
		}
	}
	return true
}
```

Update `handleGetSheet` to call it first (`requireJSON=false`):

```go
func (s *server) handleGetSheet(w http.ResponseWriter, r *http.Request) {
	if !s.checkAPIRequest(w, r, false) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.toResponse())
}
```

Update `handleSave` (replace the existing function around lines 103-144):

```go
func (s *server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkAPIRequest(w, r, true) {
		return
	}

	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	byID := make(map[string]hoursJSON, len(req.Rows))
	for _, row := range req.Rows {
		byID[row.ID] = row.Hours
	}

	for i, row := range s.sheet.Rows {
		if h, ok := byID[row.ID]; ok {
			s.sheet.Rows[i].Hours.Sun = h.Sun
			s.sheet.Rows[i].Hours.Mon = h.Mon
			s.sheet.Rows[i].Hours.Tue = h.Tue
			s.sheet.Rows[i].Hours.Wed = h.Wed
			s.sheet.Rows[i].Hours.Thu = h.Thu
			s.sheet.Rows[i].Hours.Fri = h.Fri
			s.sheet.Rows[i].Hours.Sat = h.Sat
		}
	}

	if s.save != nil {
		if err := s.save(s.sheet); err != nil {
			http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	select {
	case s.shutdown <- result{saved: true}:
	default:
	}
}
```

Update `handleCancel` (replace the existing function around lines 146-156):

```go
func (s *server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkAPIRequest(w, r, true) {
		return
	}
	w.WriteHeader(http.StatusOK)
	select {
	case s.shutdown <- result{saved: false}:
	default:
	}
}
```

Add `"strings"` to the file's imports.

- [ ] **Step 4: Run tests — must PASS**

```bash
go test ./internal/web/editor/... -v
```

All 17 tests pass: 4 existing/updated + 3 from Task 2 + 10 from this task (9 reject paths plus `_AcceptsJSONWithCharset`).

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/web/editor/server.go internal/web/editor/server_test.go
git commit -m "feat(web/editor): gate /api/* on session, origin, host, content-type"
```

---

## Task 4: Update the browser JS to send the session header and cancel body

**Files:**
- Modify: `internal/web/editor/static/editor.html`

No tests for this — the JS runs in a real browser, not in Go test infrastructure. The Task 7 manual walkthrough validates end-to-end.

- [ ] **Step 1: Read the existing JS to find the right insertion point**

```bash
sed -n '305,320p' /Users/ipm/code/tdx/internal/web/editor/static/editor.html
sed -n '820,860p' /Users/ipm/code/tdx/internal/web/editor/static/editor.html
```

Confirm the `const TEMPLATE_DATA` location (around line 312) and the two `fetch('/api/...')` call sites (around 824 and 854).

- [ ] **Step 2: Add a session constant near the existing TEMPLATE_DATA**

In `internal/web/editor/static/editor.html`, immediately AFTER the existing line:

```js
    const TEMPLATE_DATA = "__TEMPLATE_JSON__";
```

Add:

```js
    const TDX_SESSION = document.querySelector('meta[name="tdx-session"]').content;
```

- [ ] **Step 3: Update the `/api/save` fetch to include the session header**

Find the existing fetch call (around line 824):

```js
        const res = await fetch('/api/save', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json' },
          body:    JSON.stringify(payload)
        });
```

Replace with:

```js
        const res = await fetch('/api/save', {
          method:  'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Tdx-Session': TDX_SESSION,
          },
          body:    JSON.stringify(payload)
        });
```

- [ ] **Step 4: Update the `/api/cancel` fetch to include header + Content-Type + body**

Find the existing fetch call (around line 854):

```js
        await fetch('/api/cancel', { method: 'POST' });
```

Replace with:

```js
        await fetch('/api/cancel', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Tdx-Session': TDX_SESSION,
          },
          body: '{}',
        });
```

- [ ] **Step 5: Check that the `/api/template` GET also sends the session header**

```bash
grep -n "api/template\|fetch(.*api" /Users/ipm/code/tdx/internal/web/editor/static/editor.html
```

If the HTML fetches `/api/template` anywhere, update its fetch call to include the header. If the HTML only consumes data via the inline `TEMPLATE_DATA` constant and never fetches `/api/template` (likely the current state), no change needed — but the endpoint still requires the header for any future caller. Document in a one-line comment on the const if no caller exists today.

- [ ] **Step 6: Smoke check the page loads**

```bash
go test ./internal/web/editor/... -v
```

No new tests in this task. All previous tests pass.

`gofmt -l ./internal/...` empty; `go vet ./...` clean.

- [ ] **Step 7: Commit**

```bash
git add internal/web/editor/static/editor.html
git commit -m "feat(web/editor): send X-Tdx-Session header on /api/save and /api/cancel"
```

---

## Task 5: Full test + lint sweep

**Files:** none

- [ ] **Step 1: Run the full suite**

```bash
go test ./... -race && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green. No gofmt output. No vet warnings. No lint warnings.

- [ ] **Step 2: If failures appear, fix in place and commit per-issue**

Common gotchas:
- `crypto/rand` import path: `crypto/rand` not `math/rand`.
- Unused imports if the implementer's structure differs from the plan.
- Any pre-existing test that constructs `server` directly without `nonce`/`listenAddr` — should already be caught and updated in Task 1 step 4.

---

## Task 6: Manual walkthrough doc

**Files:**
- Create: `docs/manual-tests/2026-05-16-web-editor-session-protection-walkthrough.md`

- [ ] **Step 1: Write walkthrough**

Create `docs/manual-tests/2026-05-16-web-editor-session-protection-walkthrough.md`:

```markdown
# Web Editor Session Protection Walkthrough (v0.22.0)

Spec: [`docs/specs/2026-05-16-web-editor-session-protection.md`](../specs/2026-05-16-web-editor-session-protection.md)

## Step 1: Normal save round-trip

    tdx time template edit --web my-week

Expected:
- A browser tab opens at `http://127.0.0.1:PORT/?s=NONCE`.
- Edit a cell; click "Save Template".
- The page shows "Saved!".
- Terminal exits cleanly.

## Step 2: Normal cancel

    tdx time template edit --web my-week

Click "Cancel". Expected: "Cancelled."; terminal exits with no save.

## Step 3: Direct curl without headers → 403

In a separate terminal, while the editor is open, copy the address (e.g. `127.0.0.1:53219`) from the browser bar and run:

    curl -i -X POST http://127.0.0.1:53219/api/save -d '{"rows":[]}'

Expected: HTTP/1.1 403 Forbidden; body contains `host mismatch` or `origin mismatch` (whichever check fires first — both are absent on a bare curl).

## Step 4: Curl with all headers but wrong session → 403

    curl -i -X POST http://127.0.0.1:53219/api/save \
      -H "Host: 127.0.0.1:53219" \
      -H "Origin: http://127.0.0.1:53219" \
      -H "Content-Type: application/json" \
      -H "X-Tdx-Session: nope" \
      -d '{"rows":[]}'

Expected: 403; body `invalid session`.

## Step 5: GET / without ?s= → 403

    curl -i http://127.0.0.1:53219/

Expected: 403; body `invalid session`.

## Step 6: GET / with valid ?s= → 200 + session meta in HTML

Capture the URL the CLI printed (or read it from the browser bar):

    curl -is 'http://127.0.0.1:53219/?s=THE_NONCE' | grep tdx-session

Expected: `<meta name="tdx-session" content="THE_NONCE">` appears in the HTML.

## Step 7: localhost vs 127.0.0.1 (best-effort)

On a system where `localhost` resolves to IPv6 `::1`:

    curl -i http://localhost:53219/?s=THE_NONCE

May fail to connect (the server bound only to 127.0.0.1) or hit the wrong listener. Either way, the IPv4-loopback bind is doing its job.
```

- [ ] **Step 2: Commit**

```bash
git add docs/manual-tests/2026-05-16-web-editor-session-protection-walkthrough.md
git commit -m "docs: walkthrough for web editor session protection (v0.22.0)"
```

---

## Task 7: Push branch and create PR

**Files:** none

- [ ] **Step 1: Push branch**

```bash
git push -u origin web-editor-session-protection
```

- [ ] **Step 2: Create PR**

Write the body to `/tmp/pr-body-phase3.md`:

```markdown
## Summary

Phase 3 of the security hardening rollup. Addresses audit finding #2 (Medium: local web editor has no session token or Origin defense).

- New per-session 32-byte random nonce stored on the `server` struct; embedded in the page URL (`?s=`) and the rendered HTML (`<meta name="tdx-session">`).
- New `checkAPIRequest` helper gates every `/api/*` route on Host, Origin, X-Tdx-Session header (constant-time compare), and (for POST) Content-Type: application/json.
- `Run()` binds explicitly to `127.0.0.1:0` instead of `localhost:0` so we never accidentally bind IPv6.
- HTML JS reads the nonce from the meta tag and sends it on every fetch; `/api/cancel` now sends a JSON body.

## Test plan

- [x] `go test ./... -race` green
- [x] `go vet ./...`, `gofmt -l .`, `golangci-lint run ./...` all clean
- [ ] Live manual walkthrough at `docs/manual-tests/2026-05-16-web-editor-session-protection-walkthrough.md`

Closes: security audit finding #2.

Spec: `docs/specs/2026-05-16-web-editor-session-protection.md`
```

Then:

```bash
gh pr create --title "Web editor session protection (security hardening phase 3)" --body-file /tmp/pr-body-phase3.md
rm /tmp/pr-body-phase3.md
```

---

## Self-Review Notes

- [ ] Spec coverage:
  - Nonce generation (`crypto/rand`, base64.RawURLEncoded) — Task 1.
  - `127.0.0.1` bind — Task 1.
  - Page URL `?s=NONCE` — Task 1.
  - `<meta>` injection in HTML — Task 2.
  - `handleIndex` validates `?s=` constant-time — Task 2.
  - `checkAPIRequest` helper with Host, Origin, session, Content-Type — Task 3.
  - JS reads meta and sends X-Tdx-Session on `/api/save` and `/api/cancel` — Task 4.
  - 12 reject-path tests — spread across Task 2 (3 GET-index tests) and Task 3 (10 API tests). Note: spec listed `TestGetIndex_RejectsMissingSessionQuery` + `TestGetIndex_AcceptsValidSessionQuery` plus a `TestGetIndex_RejectsWrongSessionQuery` only implicitly; plan adds all three.
- [ ] No placeholders. Each step has concrete code or commands.
- [ ] Type consistency: `nonce`, `listenAddr`, `X-Tdx-Session`, `TDX_SESSION`, `__TDX_SESSION__` are spelled consistently across tasks.
- [ ] Each task is self-contained and commits independently.
