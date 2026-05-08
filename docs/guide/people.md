# tdx people

`tdx people` exposes TD's read-only people-discovery surfaces so you can find UIDs, see who reports to whom, and look up the exact account / pool names that `--account` and `--resource-pool` selectors expect.

## Contents

- [tdx people search](#tdx-people-search)
- [tdx people show](#tdx-people-show)
- [tdx people accounts](#tdx-people-accounts)
- [tdx people pools](#tdx-people-pools)

---

## tdx people search

```bash
# Default: staff only (IsEmployee=true client-side filter)
tdx people search "John Smith"

# Cap results (default 25, max 100)
tdx people search "Smith" --limit 50

# Include portal client users (default excludes them)
tdx people search "Smith" --include-clients

# JSON envelope (schema: tdx.v1.peopleSearchResult)
tdx people search "Smith" --json
```

Backed by `GET /api/people/lookup?searchText=&maxResults=` — the autocomplete
endpoint that actually honors the searchText (the `NameLike` field on
`POST /api/people/search` is silently ignored). Columns: UID NAME EMAIL
ACCOUNT MANAGER TITLE. UIDs are shown as the first 8 characters in the
table; the full UID is in JSON output.

## tdx people show

```bash
tdx people show d44687e1-1a09-ef11-86d4-df13b8e4e655

# JSON envelope (schema: tdx.v1.person)
tdx people show <UID> --json
```

Pretty-prints UID, name, email, active/employee flags, title, account,
resource pool, and manager (with email). Reuses `GET /api/people/{uid}`.

## tdx people accounts

### tdx people accounts list

```bash
tdx people accounts list             # text, sorted by name
tdx people accounts list --json      # tdx.v1.accountList
```

Columns: ID, NAME, MANAGER, ACTIVE. UFL has 6404 accounts so expect a
long table; pipe through `grep` to narrow.

#### How `--account` resolves names

`--account NAME` resolves the name to an account ID via
`POST /api/accounts/search` and then asks TD to filter people-search
server-side via `AccountIDs`. The lookup is exact-match,
case-insensitive. If no account matches (or multiple do), the command
errors out with a candidate count.

## tdx people pools

### tdx people pools list

```bash
tdx people pools list                # text, sorted by name
tdx people pools list --json         # tdx.v1.resourcePoolList
```

Columns: ID, NAME, MANAGER, REQ-APPROVAL, ACTIVE. The pool name printed
here is exactly what `--resource-pool NAME` accepts (TD's data sometimes
includes trailing whitespace; tdx trims it on read).
