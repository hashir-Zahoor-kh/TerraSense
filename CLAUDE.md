# TerraSense — Claude Code Context

## Project Overview
TerraSense is a conversational infrastructure provisioning system. Engineers 
describe infrastructure in plain English, an LLM generates Terraform HCL, a 
self-correction loop fixes security and syntax failures automatically, and a 
human approves or rejects through a minimal portal before anything touches 
production. Backend is Go, database is PostgreSQL on port 5433, all LLM calls 
use the Anthropic SDK.

## Architecture
- cmd/server — entry point
- internal/config — env-based config
- internal/db — pgx connection pool + migrations
- internal/models — struct definitions
- internal/store — InfraRequestStore, AuditLogStore, ModuleStore (raw pgx)
- internal/tools — StateReader, ModuleLister, HCLGenerator, PlanExecutor
- internal/validators — CheckovRunner
- internal/correction — self-correction loop (the key feature)
- internal/api — chi router, 5 handlers, middleware
- portal — Next.js (2 pages: pending list, review/approve)

## Working Directory
`/Users/hashir/TerraSense`

## Git Identity
- name: Hashir Zahoor
- email: hashirzahoorurrahm@mail.adelphi.edu
- GitHub: hashir-zahoor-kh

## Development Protocol
- **Stub-first:** write all signatures before any implementation
- **One function per turn** — stop after each, wait for confirmation
- **Read only files explicitly listed** for each task
- **After each passing test:** git commit immediately
- **Output format:** PASS or FAIL only. No explanations. No summaries.
- **DB port is 5433**, not 5432
- **Never run `terraform apply`** — plan only. Apply runs only on human approval via `POST /api/v1/changes/{id}/approve`
- **Mock Anthropic SDK in all tests** — no real API calls in the test suite
- **No ORM** — raw SQL with pgx
- **Typed errors everywhere**
- **JSON schemas on all LLM outputs** — unmarshal into structs, error if malformed
- **Pause after every phase** — no chaining without confirmation
- **Commit messages** must not mention Claude, Claude Code, or any AI tool

## File Location Rule
Always write files directly to `/Users/hashir/TerraSense`.
Never use git worktrees. If a worktree exists, copy files to main before committing.

## Phase Status
- [x] Phase 1 — Environment Setup (commit 79188a4)
- [x] Phase 2 — Database Models (commit 171a80e)
- [x] Phase 3 — Tools + Correction Loop (commit f7de7fe)
- [x] Phase 4 — HTTP API (commit 31f31cf)
- [x] Phase 5 — Portal (commit ec92041)
- [x] Phase 6 — Seed Data (commit 5d56f85)
- [ ] Phase 7 — End-to-End Test
- [ ] Phase 8 — README + Deploy

## Up Next
Phase 7 — End-to-End Test
