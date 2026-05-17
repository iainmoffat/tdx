# Security Hardening — Web Editor Session Protection

**Date:** 2026-05-16
**Phase:** 3 of 7 (security hardening rollup; see [Phase 1 spec](2026-05-15-report-fanout-caps.md) for the full roadmap)
**Goal:** Lock down the per-edit-session HTTP server in `internal/web/editor/` so that only the browser tab opened from the CLI can call `/api/save` and `/api/cancel`.

## Motivation

A security audit on 2026-05-15 identified the web editor as a Medium-severity finding (`#2`). The editor binds to localhost on a random port and exposes `POST /api/save` and `POST /api/cancel` with **no authentication**:

- Any other local process can scan loopback ports, find the editor, and POST to `/api/save` with crafted JSON — silently mutating the user's draft / template before persistence.
- A malicious page in the user's browser (other tab, browser extension) can issue a CSRF POST to the editor's port. DNS rebinding can route an external attacker's request to `localhost`.
- The current `Content-Type` header on `/api/save` is set by the JS but **not validated** server-side; a simple form POST trivially bypasses it.

The editor runs only during `tdx time week edit --web` and `tdx time template edit --web`, so the exposure window is short. But the persistence path writes to the user's profile config dir, so successful exploitation is durable.

## Threat model

- **Local malicious process** with read access to the user's loopback — can connect to the random port. Defended by the per-session nonce: the process knows the port but not the secret.
- **Malicious page in another browser tab** — can guess the port and issue cross-origin POSTs. Defended by the Origin check (the malicious page's Origin will not match).
- **DNS rebinding** — an external site rebinds its hostname to 127.0.0.1, then issues requests with `Host: evil.example`. Defended by the Host check.
- **Out of scope:** kernel-level attacks; a malicious process running as the same user with full filesystem access (already game over).

## Policy: three layered defenses

All three are required; each addresses a distinct attack vector.

1. **Per-session nonce** — server generates a 32-byte cryptographically random nonce on `Run()` start. Embedded in the page URL passed to `openBrowser`, validated on `GET /`, and required back on every `/api/*` request via the `X-Tdx-Session` header.
2. **Origin + Host check on /api/\*** — both headers must exactly match the server's `127.0.0.1:PORT`. Browsers automatically send Origin on programmatic POSTs with custom headers, so our own page passes; cross-origin / rebinding attacks fail.
3. **Content-Type: application/json on POST routes** — `/api/save` and `/api/cancel` require this header. Simple-form POSTs without this header cannot reach these handlers under standard browser CORS preflight rules.

Plus: change `net.Listen("tcp", "localhost:0")` to `net.Listen("tcp", "127.0.0.1:0")` so the listener pins to IPv4 loopback. `localhost` can resolve to `::1` on some macOS/Linux configurations, which would mismatch the literal `127.0.0.1:PORT` we use in the URL and header checks.

## Nonce generation, embedding, delivery

### Server side

```go
// internal/web/editor/server.go (Run)
nonce := make([]byte, 32)
if _, err := rand.Read(nonce); err != nil {
    return Result{}, fmt.Errorf("nonce: %w", err)
}
srv.nonce = base64.RawURLEncoding.EncodeToString(nonce)  // 43 chars
```

The nonce lives on the `server` struct, alongside `listenAddr` (captured after `net.Listen`):

```go
type server struct {
    sheet      editor.Sheet
    save       SaveFn
    shutdown   chan result
    nonce      string  // per-session secret
    listenAddr string  // "127.0.0.1:PORT" — used in Host/Origin checks
}
```

`Run()` builds the URL passed to `openBrowser`:

```go
url := fmt.Sprintf("http://%s/?s=%s", addr, srv.nonce)
```

`handleIndex` validates `r.URL.Query().Get("s") == s.nonce` (constant-time). If not, returns 403 (`forbidden: invalid session`).

`handleIndex` injects the nonce into the rendered HTML via the existing `injectTemplateData` mechanism, extended to also inject a `<meta name="tdx-session" content="NONCE">` tag.

### Browser side

The HTML's JavaScript reads the nonce on page load:

```js
const session = document.querySelector('meta[name="tdx-session"]').content;
```

Every `fetch('/api/...')` call adds `X-Tdx-Session: session` to its headers. The `/api/cancel` fetch (currently bare `{method: 'POST'}`) gains the same header **and** `Content-Type: application/json` (with `body: '{}'`) so the same helper can require both.

### Constant-time comparison

The nonce check uses `crypto/subtle.ConstantTimeCompare`. Overkill for loopback but trivial to do right; consistent with established security practice.

## Origin / Host / Content-Type checks on `/api/*`

Single helper applied to all three API handlers:

```go
// returns true if the request passes all gates, false if it has already
// written a 403 response.
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

Applied as the first thing in:

- `handleGetSheet` — `requireJSON=false` (GET, no body)
- `handleSave` — `requireJSON=true`
- `handleCancel` — `requireJSON=true`

`handleIndex` (the `GET /` page render) does NOT use this helper. It checks only the `?s=` query param via constant-time compare, because:

- The browser never sends `Origin` on a top-level navigation.
- This is the first request and establishes the session for the JS.

Error responses are deliberately terse — same shape every time so an attacker can't probe to learn which check failed. Each is a single line of plain text.

## Bind to `127.0.0.1`

Three small changes in `Run()`:

```go
listener, err := net.Listen("tcp", "127.0.0.1:0")   // was "localhost:0"
if err != nil {
    return Result{}, fmt.Errorf("listen: %w", err)
}
addr := listener.Addr().String()                     // "127.0.0.1:53219"
srv.listenAddr = addr
url := fmt.Sprintf("http://%s/?s=%s", addr, srv.nonce)
```

The existing browser-fallback `fmt.Printf("Could not open browser: %v\nOpen %s manually.\n", err, url)` continues to work — the user-pasted URL carries the nonce.

## Tests

### Updates to existing tests

The existing `server_test.go` uses `httptest.NewRequest` against handlers directly (bypassing `Run`). After this change, each `/api/*` request must carry the new headers. Add helpers:

```go
func newServerWithNonce(t *testing.T, sheet editor.Sheet, save SaveFn) *server {
    t.Helper()
    s := newServer(sheet, save)
    s.nonce = "test-nonce"
    s.listenAddr = "127.0.0.1:8080"
    return s
}

func newAPIRequest(t *testing.T, method, path string, body string) *http.Request {
    t.Helper()
    var rdr io.Reader
    if body != "" { rdr = strings.NewReader(body) }
    r := httptest.NewRequest(method, path, rdr)
    r.Host = "127.0.0.1:8080"
    r.Header.Set("Origin", "http://127.0.0.1:8080")
    r.Header.Set("X-Tdx-Session", "test-nonce")
    if method == http.MethodPost {
        r.Header.Set("Content-Type", "application/json")
    }
    return r
}
```

Update `TestGetSheet_ReturnsJSON`, `TestPostSave_UpdatesSheet`, `TestPostCancel_NoSave` to use the helpers. They continue to pass.

### New tests for each reject path

| Test | Setup | Expected |
|---|---|---|
| `TestPostSave_RejectsMissingSession` | no `X-Tdx-Session` header | 403, body contains `invalid session` |
| `TestPostSave_RejectsWrongSession` | header value differs | 403 |
| `TestPostSave_RejectsMissingOrigin` | no Origin header | 403, body contains `origin mismatch` |
| `TestPostSave_RejectsWrongOrigin` | `Origin: http://evil.example` | 403 |
| `TestPostSave_RejectsWrongHost` | `r.Host = "evil.example"` | 403, body contains `host mismatch` |
| `TestPostSave_RejectsNonJSON` | `Content-Type: text/plain` | 403, body contains `content-type` |
| `TestPostSave_AcceptsJSONWithCharset` | `Content-Type: application/json; charset=utf-8` | 200 (charset suffix stripped) |
| `TestPostCancel_RejectsMissingSession` | mirror of save's missing-session | 403 |
| `TestPostCancel_RejectsMissingContentType` | no Content-Type | 403 |
| `TestGetSheet_RejectsMissingSession` | GET with no X-Tdx-Session | 403 |
| `TestGetIndex_RejectsMissingSessionQuery` | `GET /` with no `?s=` | 403 |
| `TestGetIndex_AcceptsValidSessionQuery` | `GET /?s=test-nonce` | 200, body contains `<meta name="tdx-session"` |

### Walkthrough doc

`docs/manual-tests/2026-05-16-web-editor-session-protection-walkthrough.md`:

- Start an edit session: `tdx time template edit --web my-week`
- In the browser, click Save → succeeds (normal path).
- In a SECOND tab, navigate to `http://127.0.0.1:PORT/` (without `?s=`) → see 403.
- From a terminal, `curl -X POST http://127.0.0.1:PORT/api/save -d '{}'` → 403 missing/invalid headers.

## Branch / version

- Branch: `web-editor-session-protection`
- Version: **v0.22.0** — minor; behavior tightening. The web editor previously accepted unauthenticated POSTs; after this PR, only the browser tab opened from the CLI can interact. Standard usage is unchanged (the JS in `editor.html` does its part).

## Out of scope

- HTTPS for loopback (no real benefit on a single-process local server).
- Multi-session — still exactly one active editor at a time.
- Findings #3, #5, #6, #7 — remain roadmap phases.

## Open questions

None. All design decisions settled in the 2026-05-16 brainstorming session.
