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

Expected: HTTP/1.1 403 Forbidden; body contains `origin mismatch` (curl always sends a Host header so Host matches; Origin is absent so the Origin check fires first).

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
