# Phase 6 — Polish, Docs, Packaging Implementation Plan

> **For agentic workers:** execute tasks in numbered order. Most tasks create
> config/docs files (no TDD needed). Tasks 7-8 involve Go code and follow TDD.
> Never amend commits — always create new ones. Branch: `phase-6-polish` off
> `phase-5-mcp`.

**Design spec:** `docs/specs/2026-04-11-tdx-phase-6-polish-design.md`

## Goal

Make tdx ship-ready: tenant-agnostic defaults, README, CI, goreleaser + Homebrew
tap, shell completions, test coverage reporting, and a recorded demo script.

## Architecture

Phase 6 is a collection of independent polish tasks, not a single feature.
Build order matters only for the first two steps (abstraction before README).
The rest can be done in any order.

## Tech Stack

Existing Go 1.24+ codebase. New tools: `golangci-lint`, `goreleaser`,
GitHub Actions, `asciinema`. No new Go dependencies (shell completions
use Cobra's built-in generators).

## Tasks

1. Tenant abstraction (walkthrough + comments)
2. README
3. Makefile
4. Linting config + fixes
5. CI workflow
6. Goreleaser + release workflow
7. Shell completions (TDD)
8. Test coverage + reconciler edge cases
9. Asciinema demo script
10. Final verification

See design spec for full details on each task.
