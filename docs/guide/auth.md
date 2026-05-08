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
