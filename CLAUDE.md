# TerraSense — Claude Code Context

## Project
Conversational infrastructure provisioning. Natural language → Terraform HCL → Checkov validation → human approval portal.

## Working directory
`/Users/hashir/TerraSense`

## Git identity
- name: Hashir Zahoor
- email: hashirzahoorurrahm@mail.adelphi.edu
- GitHub username: hashir-zahoor-kh

## Development Protocol
- **Stub-first:** write all signatures before any implementation
- **One function per turn** — stop after each, wait for confirmation
- **Read only files explicitly listed** for each task
- **After each passing test:** git commit immediately
- **Output format:** PASS or FAIL only. No explanations. No summaries.
- **DB port is 5433**, not 5432
- **Never run `terraform apply`** — plan only. Apply runs only on human approval via `POST /api/v1/changes/{id}/approve`
- **Mock Anthropic SDK in all tests** — no real API calls in the test suite
- **No ORM** — raw SQL with pgx. Platform engineers write SQL.
- **Typed errors everywhere** — no bare `fmt.Errorf` without a caller-checkable type
- **JSON schemas on all LLM outputs** — unmarshal into structs, return error if malformed
- **Pause after every phase** — no chaining phases without confirmation
- **Commit messages** must not mention Claude, Claude Code, or any AI tool. Write commit messages as if a human engineer authored every change.

## Phase status
- [x] Phase 1 — Environment Setup (commit 79188a4)
- [x] Phase 2 — Database Models (commit 171a80e)
- [ ] Phase 3 — Core Tools & Correction Loop
- [ ] Phase 4 — Go HTTP API
- [ ] Phase 5 — Portal (minimal)
- [ ] Phase 6 — Seed Data
- [ ] Phase 7 — End-to-End Test
- [ ] Phase 8 — README + Deploy
