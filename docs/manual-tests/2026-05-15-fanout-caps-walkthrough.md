# Fan-Out Caps Walkthrough (v0.20.0)

Spec: [`docs/specs/2026-05-15-report-fanout-caps.md`](../specs/2026-05-15-report-fanout-caps.md)

## Step 1: `tdx time report status` week cap

```
tdx time report status --user me --from 2020-01-01 --to 2030-01-01
```

Expected:
- Exit 1
- Stderr contains `fanout_limit_exceeded: weeks=` and `max=52`

## Step 2: `tdx time report status` --limit cap

```
tdx time report status --user me --week 2026-04-12 --limit 5000
```

Expected: Exit 1; stderr contains `fanout_limit_exceeded: limit=5000 max=1000`.

## Step 3: `tdx time report status` boundary (52 weeks ✓)

```
tdx time report status --user me --from 2026-01-04 --to 2026-12-26 --json
```

Expected: Exit 0; JSON envelope has 52 weeks × 1 user = 52 rows (or fewer with `--include-zero=false`).

## Step 4: `tdx project time` week cap

```
tdx project time 259 --user me --from 2020-01-01 --to 2030-01-01
```

Expected: Exit 1; stderr `fanout_limit_exceeded: weeks=...`.

## Step 5: `tdx project time --all-users` user cap

This step needs a project with >1000 resources, which may not exist on the test tenant. If so, skip and note in this doc.

```
tdx project time <big-project-id> --all-users --week 2026-04-12
```

Expected (if applicable): Exit 1; stderr `fanout_limit_exceeded: users=...`.

## Step 6: MCP error shape

Via Claude or any MCP client, call `get_time_status_report` with a 10-year `from`/`to` range. The tool result should be an error containing `fanout_limit_exceeded: weeks=...`. The LLM should be able to retry with a narrower range.

Same for `get_project_time_review` with a 10-year range.

## Step 7: Within-cap calls unaffected

```
tdx time report status --manager me --week 2026-04-12 --json
```

Expected: Exit 0; output unchanged from v0.19.0 behavior.
