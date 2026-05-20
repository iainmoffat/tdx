# tdx auth

Authenticate against a TeamDynamix tenant and manage profiles for multiple tenants.

## Contents

- [tdx auth login](#tdx-auth-login)
- [tdx auth status](#tdx-auth-status)
- [tdx auth logout](#tdx-auth-logout)
- [tdx auth profile](#tdx-auth-profile)

---

## tdx auth login

### First login

```bash
tdx auth login --url https://yourorg.teamdynamix.com/
```

You'll be prompted to paste a TD API token. To get one, log in to
TeamDynamix in your browser and navigate to your profile's API token view.

### SSO login

If your organization uses SSO, the `--sso` flag opens the TD SSO endpoint
in your browser. Complete the SSO flow, then copy the token displayed:

```bash
tdx auth login --sso
```

The SSO endpoint issues a fresh 24-hour JWT each time it's called with a
valid TD session cookie.

### Scripted login

For CI or scripts, pipe the token via stdin:

```bash
echo "$TOKEN" | tdx auth login --token-stdin --profile default --url https://yourorg.teamdynamix.com/
```

## tdx auth status

```bash
tdx auth status
```

Shows your user identity, tenant URL, and profile name.

## tdx auth logout

```bash
tdx auth logout
```

Removes the stored token and profile.

---

## tdx auth profile

Profiles let you manage multiple TD tenants or accounts. Each profile stores
a tenant URL and authentication token independently.

### tdx auth profile add

```bash
tdx auth profile add work --url https://work.teamdynamix.com/
tdx auth profile add personal --url https://personal.teamdynamix.com/
```

The first profile added automatically becomes the default.

### tdx auth profile list

```bash
tdx auth profile list
```

The default profile is marked with `*`.

### tdx auth profile use

The `tdx auth profile use <name>` command sets the default profile; the `--profile <name>` global flag overrides for a single invocation.

```bash
tdx auth profile use personal
```

Every command accepts `--profile`:

```bash
tdx time entry list --profile work
tdx time entry list --profile personal
```

### tdx auth profile remove

```bash
tdx auth profile remove personal
```

---

## Credential storage

By default, `tdx` stores your bearer token in the OS keychain:

- **macOS** — Keychain Access.app, under the "tdx" service.
- **Linux** — Secret Service (GNOME Keyring, KWallet) via D-Bus.
- **Windows** — Credential Manager, target name `tdx:<profile>`.

This replaces the older plaintext `~/.config/tdx/credentials.yaml`. If you upgrade from a previous tdx version that wrote tokens to YAML, the next `tdx` command auto-migrates each token into the keychain and prints a one-time notice to stderr. The YAML file is removed when it becomes empty.

### Choosing the backend

Set `TDX_TOKEN_BACKEND` in your shell:

- `auto` (default, or unset) — try keychain; fall back to YAML if the keychain isn't available (headless servers, etc.). A stderr notice fires on every fallback so you know to opt in explicitly.
- `keychain` — keychain only. The command fails if the keychain isn't available, rather than silently writing plaintext.
- `yaml` — YAML only, no keychain. Use this on headless servers, in CI, or when you've deliberately decided plaintext-with-0600 is the right trade-off for your environment.

### Logout / clear

`tdx auth logout` clears the token from both the active backend and the YAML fallback, so there's no stale entry left behind from a previous migration.

### Token scope and rotation

Bearer tokens issued by TeamDynamix are JWTs with a 24-hour lifetime and no refresh mechanism. When `tdx auth status` reports the token as expired, re-run `tdx auth login` (or `tdx auth login --sso`) to mint a fresh one.
