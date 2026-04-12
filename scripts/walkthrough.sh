#!/usr/bin/env bash
# scripts/walkthrough.sh
#
# Drives the Phase 2 walkthrough commands non-interactively against the live
# UFL TeamDynamix tenant. Used by Claude (and humans) to validate the binary
# after each phase of work without manual command typing.
#
# Required env vars:
#   TDX_WALKTHROUGH_TOKEN  — a valid TD API JWT (e.g. fetched via tdx auth login --sso)
#
# Optional env vars:
#   TDX_WALKTHROUGH_URL    — tenant base URL (default https://ufl.teamdynamix.com/)
#   TDX_WALKTHROUGH_WEEK   — a week date with known data (default 2026-04-01)
#   TDX_WALKTHROUGH_BIN    — path to the tdx binary (default ./tdx)
#
# Behavior:
#   - Builds the tdx binary if it does not exist at $TDX_WALKTHROUGH_BIN
#   - Creates a temp TDX_CONFIG_HOME so it does not pollute ~/.config/tdx
#   - Runs each step, asserts expected substring or exit code, exits 1 on failure
#   - Cleans up the temp dir + the built binary on exit (if it built one)
#   - Prints a per-step pass/fail summary

set -euo pipefail

# ---------- config ----------
TENANT_URL="${TDX_WALKTHROUGH_URL:-https://ufl.teamdynamix.com/}"
WEEK_DATE="${TDX_WALKTHROUGH_WEEK:-2026-04-01}"
BIN="${TDX_WALKTHROUGH_BIN:-./tdx}"
BUILT_OUR_OWN_BIN=0

if [[ -z "${TDX_WALKTHROUGH_TOKEN:-}" ]]; then
  echo "ERROR: TDX_WALKTHROUGH_TOKEN env var is required" >&2
  exit 2
fi

# ---------- temp config dir ----------
WALK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tdx-walkthrough-XXXXXX")"
export TDX_CONFIG_HOME="$WALK_DIR"

cleanup() {
  rm -rf "$WALK_DIR"
  if [[ "$BUILT_OUR_OWN_BIN" -eq 1 && -f "$BIN" ]]; then
    rm -f "$BIN"
  fi
}
trap cleanup EXIT INT TERM

# ---------- build binary if needed ----------
if [[ ! -x "$BIN" ]]; then
  echo "Building $BIN..."
  go build -o "$BIN" ./cmd/tdx
  BUILT_OUR_OWN_BIN=1
fi

# ---------- step harness ----------
PASS_COUNT=0
FAIL_COUNT=0

step() {
  local name="$1"
  local cmd="$2"
  local expect_substring="${3:-}"
  local expect_exit="${4:-0}"

  echo
  echo "=== $name ==="
  set +e
  local output
  output="$(eval "$cmd" 2>&1)"
  local rc=$?
  set -e

  local ok=1
  if [[ "$rc" -ne "$expect_exit" ]]; then
    ok=0
    echo "FAIL: exit code $rc (expected $expect_exit)"
  fi
  if [[ -n "$expect_substring" ]] && ! grep -q -- "$expect_substring" <<<"$output"; then
    ok=0
    echo "FAIL: expected substring not found: $expect_substring"
  fi

  if [[ "$ok" -eq 1 ]]; then
    PASS_COUNT=$((PASS_COUNT + 1))
    echo "PASS"
  else
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo "--- output ---"
    echo "$output"
    echo "--- end output ---"
  fi
}

# ---------- login ----------
echo "Logging in via --token-stdin..."
echo "$TDX_WALKTHROUGH_TOKEN" | "$BIN" auth login --token-stdin --profile default --url "$TENANT_URL"

# ---------- walkthrough steps ----------
step "Step 1: version" \
  "$BIN version" \
  "tdx 0.1.0-dev"

step "Step 3: auth status" \
  "$BIN auth status" \
  "user:"

step "Step 4: type list" \
  "$BIN time type list" \
  "ID  NAME"

step "Step 5: type list --json" \
  "$BIN time type list --json" \
  '"schema": "tdx.v1.timeTypes"'

step "Step 6: entry list (default week, may be empty)" \
  "$BIN time entry list" \
  "TOTAL"

step "Step 7: entry list (specific week with known data)" \
  "$BIN time entry list --week $WEEK_DATE" \
  "TOTAL"

step "Step 10: week show (default)" \
  "$BIN time week show" \
  "DAY TOTAL"

step "Step 11: week show specific week" \
  "$BIN time week show $WEEK_DATE" \
  "DAY TOTAL"

step "Step 12: week locked" \
  "$BIN time week locked" \
  ""

step "Step 14a: entry list --json" \
  "$BIN time entry list --json" \
  '"schema": "tdx.v1.entryList"'

step "Step 14b: week show --json" \
  "$BIN time week show --json" \
  '"schema": "tdx.v1.weekReport"'

step "Step 14c: type list --json" \
  "$BIN time type list --json" \
  '"schema": "tdx.v1.timeTypes"'

# ---------- failure cases ----------
step "F-A: ticket without app" \
  "$BIN time entry list --ticket 12345" \
  "--ticket requires --app" \
  1

step "F-B: unknown type name" \
  "$BIN time entry list --type NONSENSE" \
  'no time type named "NONSENSE"' \
  1

step "F-C: unknown kind" \
  "$BIN time type for nonsense 1 --app 42" \
  'unknown kind' \
  1

step "F-D: nonexistent entry" \
  "$BIN time entry show 999999999" \
  "entry 999999999 not found" \
  1

# ---------- summary ----------
echo
echo "================================"
echo "PASS: $PASS_COUNT  FAIL: $FAIL_COUNT"
echo "================================"

if [[ "$FAIL_COUNT" -gt 0 ]]; then
  exit 1
fi
