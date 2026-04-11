# Phase 2 — Manual Read Operations Walkthrough

This document exercises the Phase 2 read-only time commands against a real
TeamDynamix tenant.

## Prerequisites

- A built `tdx` binary (`go build ./cmd/tdx`).
- A valid API token for your TD user.
- Phase 1 already passed its walkthrough on this machine, so `tdx auth login`
  is known to work.

## Walkthrough

1. **Confirm the binary version.**
   ```
   ./tdx version
   ```
   Expected: `tdx 0.1.0-dev`.

2. **Sign in (skip if already signed in).**
   ```
   ./tdx auth login --profile default --url https://ufl.teamdynamix.com/
   ```
   Paste your API token when prompted.

3. **Confirm auth status now shows identity.**
   ```
   ./tdx auth status
   ```
   Expected:
   ```
   profile:  default
   tenant:   https://ufl.teamdynamix.com/
   state:    authenticated
   token:    valid
   user:     <Your Full Name>
   email:    <your email>
   ```
   If `user:` is absent or shows `(lookup failed: ...)`, the whoami endpoint
   is returning something unexpected — STOP and investigate.

4. **List time types.**
   ```
   ./tdx time type list
   ```
   Expected: a table of time types with ID, NAME, BILLABLE, LIMITED,
   DESCRIPTION columns. At least one row.

5. **List time types as JSON.**
   ```
   ./tdx time type list --json | head -20
   ```
   Expected: pretty-printed JSON with `"schema": "tdx.v1.timeTypes"`.

6. **List this week's entries (default filter).**
   ```
   ./tdx time entry list
   ```
   Expected: a flat table of entries from Sun through Sat of the current
   week, filtered to your user. If you have no entries yet, the table
   still prints the header and a `TOTAL 0.00` row.

7. **List entries for a specific week.**
   ```
   ./tdx time entry list --week 2026-04-08
   ```
   Expected: same shape, for the week of 2026-04-05 through 2026-04-11.

8. **List entries filtered by ticket.**
   Find a ticket ID you've logged time against (from step 6's output —
   the `TARGET` column shows `#<id>`). Pass it with the correct app ID
   (visible in the JSON output of step 6 under `target.appID`):
   ```
   ./tdx time entry list --ticket <ID> --app <APP_ID>
   ```
   Expected: only entries against that ticket.

9. **Show a single entry.**
   Pick any entry ID from the lists above:
   ```
   ./tdx time entry show <ENTRY_ID>
   ```
   Expected: a detail block with entry:, date:, hours:, minutes:, type:,
   target:, description:, status:, billable: lines.

10. **Show this week as a grid.**
    ```
    ./tdx time week show
    ```
    Expected: a Sun..Sat grid with your logged time rolled up by
    (target, type), empty cells as `.`, and a DAY TOTAL row at the
    bottom.

11. **Show a specific week's grid.**
    ```
    ./tdx time week show 2026-04-08
    ```
    Expected: same layout, for the week of 2026-04-05 through 2026-04-11.

12. **List locked days in the current week.**
    ```
    ./tdx time week locked
    ```
    Expected: either a list of ISO dates or `no locked days in range`.

13. **Look up time types for a specific ticket.**
    Use the same ticket + app from step 8:
    ```
    ./tdx time type for ticket <ID> --app <APP_ID>
    ```
    Expected: a table of time types valid for that ticket (usually a
    subset of the full list from step 4).

14. **JSON sanity check for agents.**
    ```
    ./tdx time entry list --json
    ./tdx time week show --json
    ./tdx time type list --json
    ```
    All three should produce pretty-printed JSON with a `schema` field
    at the top.

## Failure cases to try

- **Ticket without app.**
  ```
  ./tdx time entry list --ticket 12345
  ```
  Expected: `--ticket requires --app` error, exit code 2.

- **Unknown time type name.**
  ```
  ./tdx time entry list --type NONSENSE
  ```
  Expected: `no time type named "NONSENSE"` error, exit code 1.

- **Unknown kind on type for.**
  ```
  ./tdx time type for nonsense 1 --app 42
  ```
  Expected: usage error listing the supported kinds.

- **Entry that does not exist.**
  ```
  ./tdx time entry show 999999999
  ```
  Expected: `entry 999999999 not found`, exit code 1.

## Notes

- All dates are interpreted in America/New_York regardless of your laptop
  clock. If you travel and queries look off by a day, that is why.
- The `--limit 100` default applies to `tdx time entry list`. If you have
  more than 100 entries in a range, pass a larger `--limit` to see them
  all (up to TD's server-side cap of 1000).
- Phase 2 is read-only. Any `add`, `update`, or `delete` command will
  print "unknown command" — those ship in Phase 3.
