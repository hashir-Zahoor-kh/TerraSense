# TerraSense

TerraSense is a conversational infrastructure provisioning system. An engineer describes what they want in plain English — "a private S3 bucket with versioning and server-side encryption" — and the system generates valid, security-checked Terraform HCL, runs a self-correction loop to fix any issues automatically, and then waits for a human to approve or reject before anything touches a real environment. The backend is written in Go, state is stored in PostgreSQL, and all LLM calls go through the Anthropic SDK.

---

## The Problem

LLM-generated Terraform is useful but unreliable on its own. A model will produce plausible-looking HCL that has public S3 buckets, open security group rules, missing encryption, or syntax errors that only surface at plan time. Running that output directly with `terraform apply` means infrastructure defects ship silently. TerraSense adds two layers between the model output and production: an automated correction loop that catches and fixes policy and syntax failures before a human ever sees the plan, and a mandatory approval gate where a human reviews the final plan before apply runs.

---

## Architecture

```
                        +-------------------+
                        |  Engineer (curl)  |
                        +--------+----------+
                                 |
                         POST /api/v1/requests
                                 |
                        +--------v----------+
                        |    Go API Server  |
                        |  (chi router)     |
                        +--------+----------+
                                 |
                        +--------v----------+
                        |  HCL Generator    |  <-- Anthropic SDK (Claude)
                        |  (tools/hclgen)   |
                        +--------+----------+
                                 |
                   +-------------v--------------+
                   |     Self-Correction Loop    |
                   |   (internal/correction)     |
                   |                             |
                   |  1. Run Checkov             |
                   |  2. If failures, send       |
                   |     violations back to LLM  |
                   |  3. LLM produces new HCL    |
                   |  4. Repeat up to N times    |
                   |  5. Fail if threshold not   |
                   |     met after max attempts  |
                   +-------------+---------------+
                                 |
                        +--------v----------+
                        |  Terraform Plan   |
                        |  (plan only,      |
                        |   never apply)    |
                        +--------+----------+
                                 |
                        +--------v----------+
                        |  Approval Portal  |
                        |  (Next.js)        |
                        |  Human reviews    |
                        |  plan + HCL       |
                        +--------+----------+
                                 |
               +-----------------+-----------------+
               |                                   |
        +------v------+                   +--------v------+
        |   Approve   |                   |    Reject     |
        |             |                   |               |
        | POST        |                   | POST          |
        | /approve    |                   | /reject       |
        +------+------+                   +---------------+
               |
        +------v----------+
        | Terraform Apply  |
        | (only path that  |
        | touches infra)   |
        +------------------+
```

---

## The Self-Correction Loop

This is the central feature. When an engineer submits a request, the system generates HCL and immediately runs it through Checkov, a static analysis tool that checks Terraform for security policy violations. If Checkov reports failures, the system does not stop and surface the errors to the engineer. Instead, it sends the original HCL and the structured list of violations back to the LLM and asks it to produce a corrected version that fixes each identified issue. The corrected HCL goes through Checkov again. This cycle repeats up to a configurable maximum number of attempts.

If the HCL passes Checkov with a severity threshold (no HIGH or CRITICAL findings by default), the loop exits with a clean plan. If the maximum attempts are exhausted without reaching the threshold, the request fails with a typed error that surfaces which checks could not be resolved. The audit log records every iteration: which attempt it was, which checks failed, and what the LLM returned. This means you have a full trace of how the final HCL was arrived at, not just the end result.

The correction loop is in `internal/correction`. The Checkov integration is in `internal/validators`. The HCL generator is in `internal/tools`.

---

## Security Model

**Approval gate.** `terraform apply` never runs without a human action. The server executes plan only. Apply is triggered exclusively by `POST /api/v1/changes/{id}/approve`, which requires an API key and records the approver identity in the audit log. Rejections are similarly recorded.

**Checkov threshold.** By default the correction loop will not pass a plan that has any HIGH or CRITICAL Checkov findings. This threshold is configurable via environment variable. The loop retries automatically; the approval gate is the backstop if the loop cannot fully resolve a plan.

**API key middleware.** All mutating endpoints (`POST /requests`, `POST /approve`, `POST /reject`, `POST /reset`) require a bearer token that matches `API_KEY` in the server's environment. Unauthenticated requests receive 401 before any handler logic runs. Read endpoints (`GET /requests`, `GET /requests/{id}`) are unauthenticated to allow the portal to render without credentials.

**Subprocess timeouts.** Checkov and Terraform are invoked as subprocesses with explicit context deadlines. A hung subprocess does not block a request handler indefinitely. Timeouts are set in `internal/validators` and `internal/tools`.

**Workspace isolation.** Each infrastructure request gets its own temporary working directory on disk. Terraform state, plan files, and generated HCL for one request cannot interfere with another. Directories are cleaned up after the request completes or fails.

---

## Local Setup

### Prerequisites

- Go 1.22+
- Docker and Docker Compose
- Node.js 20+ (for the portal)
- Terraform 1.6+
- Checkov (`pip install checkov`)
- An Anthropic API key

### Environment variables

Create a `.env` file in the project root:

```
DATABASE_URL=postgres://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable
ANTHROPIC_API_KEY=sk-ant-...
API_KEY=local-dev-key
PORT=8000
CHECKOV_THRESHOLD=HIGH
CORRECTION_MAX_ATTEMPTS=3
```

### Start the database

```bash
docker-compose up -d
```

This starts PostgreSQL on port 5433 with the `terrasense` database, user, and password.

### Run migrations

Migrations run automatically on server start. If you need to run them manually:

```bash
go run ./cmd/server --migrate-only
```

### Seed demo data

```bash
curl -s -X POST http://localhost:8000/api/v1/demo/seed \
  -H "Authorization: Bearer local-dev-key"
```

This inserts a set of example requests in various states (pending, approved, rejected) so the portal has something to display immediately.

To reset the database back to the seeded state:

```bash
curl -s -X POST http://localhost:8000/api/v1/demo/reset \
  -H "Authorization: Bearer local-dev-key"
```

### Start the server

```bash
go run ./cmd/server
```

The API listens on `http://localhost:8000`.

### Start the portal

```bash
cd portal
npm install
npm run dev
```

The portal is available at `http://localhost:3000`.

---

## Submitting Your First Request

```bash
curl -s -X POST http://localhost:8000/api/v1/requests \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer local-dev-key" \
  -d '{
    "description": "a private S3 bucket named my-app-assets with versioning enabled and AES256 server-side encryption"
  }' | jq .
```

The response includes the request ID, generated HCL, Checkov result, correction attempt count, and current status. If the plan is clean, the request moves to `pending_approval` and appears in the portal at `http://localhost:3000`.

To approve:

```bash
curl -s -X POST http://localhost:8000/api/v1/changes/{id}/approve \
  -H "Authorization: Bearer local-dev-key" | jq .
```

---

## Future Work

- **OPA policy-as-code.** Replace the Checkov threshold with Open Policy Agent rules checked into the repository alongside the infrastructure code. Teams could write and version their own policy constraints.
- **Atlantis integration.** Route approved plans through Atlantis instead of running apply directly from the server, so plan/apply history integrates with existing GitOps workflows.
- **Slack approval workflow.** Send the plan summary to a Slack channel and allow engineers to approve or reject with a button click, rather than through the web portal.
- **Multi-cloud.** The HCL generator currently targets AWS. Extending to Azure and GCP requires module definitions for those providers and updated Checkov policies.
