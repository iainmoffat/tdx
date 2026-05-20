# OS Keychain Token Storage Walkthrough (v0.24.0)

Spec: [`docs/specs/2026-05-20-keychain-token-storage.md`](../specs/2026-05-20-keychain-token-storage.md)

## Setup

Save your current credentials.yaml (if any):

    cp ~/.config/tdx/credentials.yaml ~/.config/tdx/credentials.yaml.bak 2>/dev/null || true

## Step 1: Auto-migration from existing YAML (macOS)

If you have an existing token in YAML, run any tdx command:

    tdx auth status

Expected:
- stderr: `notice: migrated token for profile "default" from credentials.yaml to OS keychain`
- `~/.config/tdx/credentials.yaml` is removed (was the only profile) OR no longer contains that profile.
- macOS Keychain Access.app shows a new entry under "tdx" service. Verify with:
  ```
  security find-generic-password -s tdx -a default -w
  ```
  (prints the token)

## Step 2: Re-run is silent

    tdx auth status

Expected: normal output; no migration notice this time.

## Step 3: Fresh login lands directly in keychain

    tdx auth logout
    security find-generic-password -s tdx -a default -w 2>&1 | grep -q "could not be found" && echo "logged out (keychain cleared)"
    tdx auth login   # paste a fresh JWT
    security find-generic-password -s tdx -a default -w   # token present

Expected: each command does what its echo says.

## Step 4: Force yaml backend

After Step 3 the token lives only in the keychain — the YAML file is gone.
To exercise yaml mode you must seed it explicitly:

    TDX_TOKEN_BACKEND=yaml tdx auth login   # paste a fresh JWT
    ls -la ~/.config/tdx/credentials.yaml   # file exists, contains the token
    TDX_TOKEN_BACKEND=yaml tdx auth status  # normal output, reading from YAML

The keychain still has its own copy from Step 3 — unset `TDX_TOKEN_BACKEND`
and tdx reads from keychain again. The two backends are independent.

## Step 5: Force keychain strict mode

    TDX_TOKEN_BACKEND=keychain tdx auth status

Expected on macOS: normal output (keychain available, token there).

To simulate keychain unavailable (Linux servers without D-Bus): unset D-Bus and force keychain mode:

    DBUS_SESSION_BUS_ADDRESS=disabled TDX_TOKEN_BACKEND=keychain tdx auth status

Expected: `error: TDX_TOKEN_BACKEND=keychain but keychain is unavailable: ...` and exit 1.

## Step 6: Auto-fallback stderr notice (Linux server simulation)

    DBUS_SESSION_BUS_ADDRESS=disabled tdx auth status

Expected: stderr `notice: keychain unavailable, using credentials.yaml (set TDX_TOKEN_BACKEND=yaml to silence)` followed by the normal status output (which uses the YAML fallback).

## Cleanup

    cp ~/.config/tdx/credentials.yaml.bak ~/.config/tdx/credentials.yaml 2>/dev/null || true
