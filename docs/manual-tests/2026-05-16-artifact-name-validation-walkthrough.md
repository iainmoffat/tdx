# Artifact Name Validation Walkthrough (v0.21.0)

Spec: [`docs/specs/2026-05-16-artifact-name-validation.md`](../specs/2026-05-16-artifact-name-validation.md)

## Step 1: CLI template — traversal in name

    tdx time template show ../../credentials

Expected: Exit 1; stderr `invalid_artifact_name: name contains invalid character '/' at position 2`.

## Step 2: CLI draft — traversal via ref

    tdx time week pull 2026-04-12/../../foo

Expected: Exit 1; stderr `invalid_artifact_name: name contains invalid character '/' at position 2`.

## Step 3: CLI auth — traversal in profile use

    tdx auth profile use ..

Expected: Exit 1; stderr `invalid_artifact_name: name may not start with "."`.

## Step 4: Profile add with reserved name

    tdx auth profile add CON --tenant https://example.com

Expected: Exit 1; stderr `invalid_artifact_name: "CON" is a reserved name`.

## Step 5: MCP — invalid template name

Via Claude or any MCP client, call `get_template` (or `apply_template`, `delete_template`) with `name: "../../credentials"`. The tool result should be an error containing `invalid_artifact_name: name contains invalid character '/' at position 2`. The LLM should be able to retry with a valid name.

## Step 6: MCP — invalid draft ref

Call `get_week_draft` with `ref: "2026-04-12/../../foo"`. Tool result error contains `invalid_artifact_name`.

## Step 7: Existing artifacts unaffected

    tdx time template show my-week
    tdx time week show 2026-04-12

Expected: Exit 0; normal output. No regression on existing well-behaved names.

## Step 8: Long name rejected

    tdx time template show $(head -c 100 /dev/urandom | base64 | tr -d '/+=' | head -c 100)

Expected: Exit 1; stderr `invalid_artifact_name: name exceeds 64 characters`.
