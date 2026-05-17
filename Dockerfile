# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /terrasense ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
# Python base gives us pip for Checkov without a separate install layer.
FROM python:3.12-slim

ARG TERRAFORM_VERSION=1.9.8

# Terraform
RUN apt-get update && apt-get install -y --no-install-recommends wget unzip ca-certificates \
    && wget -q "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip" \
    && unzip "terraform_${TERRAFORM_VERSION}_linux_amd64.zip" -d /usr/local/bin/ \
    && rm "terraform_${TERRAFORM_VERSION}_linux_amd64.zip" \
    && apt-get purge -y --auto-remove wget unzip \
    && rm -rf /var/lib/apt/lists/*

# Checkov
RUN pip install --no-cache-dir checkov

WORKDIR /app
COPY --from=builder /terrasense .

RUN mkdir -p /tmp/terrasense_workspaces

EXPOSE 8000
CMD ["./terrasense"]
