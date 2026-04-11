# Phase 1 — Manual Auth Walkthrough

This document exercises the Phase 1 auth flow against a real TeamDynamix tenant.

## Prerequisites

- A built `tdx` binary (`go build ./cmd/tdx`).
- Access to a TeamDynamix tenant (default: `https://ufl.teamdynamix.com/`).
- A valid API token for your TD user (obtain from the TD web UI — exact location
  depends on your tenant configuration, typically under Profile → API Tokens).

## Walkthrough

1. **Verify the binary runs.**
   ```
   ./tdx version
   ```
   Expected: `tdx 0.1.0-dev`.

2. **Check the config paths.**
   ```
   ./tdx config path
   ```
   Expected: four lines showing `root`, `config`, `credentials`, `templates` all
   under `~/.config/tdx/`.

3. **Initialise the config directory.**
   ```
   ./tdx config init
   ```
   Expected: `initialised /Users/you/.config/tdx`.

4. **Check that no profiles exist.**
   ```
   ./tdx config show
   ```
   Expected: `no profiles configured`.

5. **Sign in with a paste token.**
   ```
   ./tdx auth login --profile default --url https://ufl.teamdynamix.com/
   ```
   Expected: a prompt asking for the token. Paste the token (it will not
   echo) and press Enter. On success: `signed in as profile "default" (https://ufl.teamdynamix.com/)`.

6. **Confirm status reports valid.**
   ```
   ./tdx auth status
   ```
   Expected:
   ```
   profile:  default
   tenant:   https://ufl.teamdynamix.com/
   state:    authenticated
   token:    valid
   ```

7. **List profiles to confirm persistence.**
   ```
   ./tdx auth profile list
   ```
   Expected: `* default  https://ufl.teamdynamix.com/`.

8. **Verify credentials file permissions.**
   ```
   ls -l ~/.config/tdx/credentials.yaml
   ```
   Expected: `-rw-------` (mode 0600).

9. **Log out.**
   ```
   ./tdx auth logout
   ```
   Expected: `logged out of profile "default"`.

10. **Re-check status.**
    ```
    ./tdx auth status
    ```
    Expected:
    ```
    profile:  default
    tenant:   https://ufl.teamdynamix.com/
    state:    not authenticated
              run 'tdx auth login' to sign in
    ```

## Failure cases to try

- **Bad token.** Enter a random string at the login prompt. Expected: `invalid token: server rejected token`.
- **Wrong tenant URL.** Pass `--url https://wrong.teamdynamix.com/`. Expected: an HTTP error surfaced clearly.
- **Missing URL and no existing profile.** Run `./tdx auth login` on a fresh system. Expected: defaults to `https://ufl.teamdynamix.com/` with a `stderr` notice.

## Notes

- Phase 1 does not implement any ergonomic browser SSO flow. Phase 1B is the
  follow-up for that, once the UFL SSO callback mechanism is verified.
- Phase 1 does not fetch user identity. `tdx auth status` will gain a "signed
  in as …" line in Phase 2 once a whoami endpoint is confirmed.
