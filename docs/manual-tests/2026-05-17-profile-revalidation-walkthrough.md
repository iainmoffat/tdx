# Profile Revalidation Walkthrough (v0.23.0)

Spec: [`docs/specs/2026-05-17-profile-revalidation.md`](../specs/2026-05-17-profile-revalidation.md)

## Setup

Back up your real config first:

    cp ~/.config/tdx/config.yaml ~/.config/tdx/config.yaml.bak

## Step 1: Flip HTTPS to HTTP — Load skips and warns

Edit `~/.config/tdx/config.yaml` and change the active profile's `tenantBaseURL`
from `https://yourorg.teamdynamix.com/` (or whatever you have) to
`http://attacker.example/`.

Run:

    tdx auth status

Expected:
- stderr: `warning: skipping invalid profile "<name>": invalid profile: tenantBaseURL must use https`
- the command fails (likely "profile not found: <name>" because Load skipped it
  and ResolveProfile can't find the default).

## Step 2: file:// scheme — same outcome

Edit `tenantBaseURL` to `file:///etc/passwd`. Run `tdx auth status` again.

Expected: same warning shape; same failure.

## Step 3: Restore — everything works

    cp ~/.config/tdx/config.yaml.bak ~/.config/tdx/config.yaml

Run:

    tdx auth status

Expected: normal output. No stderr warnings.

## Step 4: tdx.NewClient guard — direct check

Not a CLI test, but worth noting: the same policy now lives in `tdx.NewClient`.
Any future code path that constructs a client with a non-HTTPS, non-loopback
URL fails fast at construction.
