# TerraSense — Claude Code Context

## Project
Conversational infrastructure provisioning. Natural language → Terraform HCL → Checkov validation → human approval portal.

## Working directory
`/Users/hashir/TerraSense`

## Git identity
- name: hashir-zahoor-kh
- email: hashirzahoorurrahm@mail.adelphi.edu

## Key rules
- Never run `terraform apply` outside of the explicit human-approval flow in `POST /api/v1/changes/{id}/approve`
- Mock Anthropic API in all tests — no real API calls in test suite
- JSON schemas enforced on all LLM outputs — never trust free-form text
- Pause after every phase and wait for confirmation before continuing

## Phase status
- [ ] Phase 1 — Environment Setup
- [ ] Phase 2 — Database Models
- [ ] Phase 3 — MCP Server Tools
- [ ] Phase 4 — FastAPI REST API
- [ ] Phase 5 — Next.js Portal
- [ ] Phase 6 — Sample Terraform Modules
- [ ] Phase 7 — End-to-End Test
- [ ] Phase 8 — README + Deployment
