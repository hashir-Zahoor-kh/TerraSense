package demo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type row struct {
	NaturalLanguageReq string
	GeneratedHCL       string
	PlanOutput         string
	CheckovScore       int
	CheckovWarnings    []models.CheckovWarning
	CorrectionAttempts int
}

var demoRows = []row{
	{
		NaturalLanguageReq: "Create a secure S3 bucket for storing application logs with versioning enabled",
		GeneratedHCL: `resource "aws_s3_bucket" "app_logs" {
  bucket = "acme-app-logs-prod"
  tags   = { Environment = "production", Team = "platform" }
}

resource "aws_s3_bucket_versioning" "app_logs" {
  bucket = aws_s3_bucket.app_logs.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "app_logs" {
  bucket = aws_s3_bucket.app_logs.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
  }
}

resource "aws_s3_bucket_public_access_block" "app_logs" {
  bucket                  = aws_s3_bucket.app_logs.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}`,
		PlanOutput:   "Plan: 4 to add, 0 to change, 0 to destroy.",
		CheckovScore: 85,
		CheckovWarnings: []models.CheckovWarning{
			{
				CheckID:   "CKV_AWS_21",
				CheckType: "terraform",
				Resource:  "aws_s3_bucket.app_logs",
				Message:   "Ensure S3 bucket has MFA delete enabled",
				Severity:  "LOW",
			},
		},
		CorrectionAttempts: 0,
	},
	{
		NaturalLanguageReq: "Provision an EC2 instance for running a Node.js API server in the production VPC",
		GeneratedHCL: `resource "aws_security_group" "api_server" {
  name        = "api-server-sg"
  description = "API server security group"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "api_server" {
  ami                         = "ami-0c55b159cbfafe1f0"
  instance_type               = "t3.small"
  subnet_id                   = var.private_subnet_id
  vpc_security_group_ids      = [aws_security_group.api_server.id]
  associate_public_ip_address = false

  tags = { Name = "api-server", Environment = "production" }
}`,
		PlanOutput:   "Plan: 2 to add, 0 to change, 0 to destroy.",
		CheckovScore: 72,
		CheckovWarnings: []models.CheckovWarning{
			{
				CheckID:   "CKV_AWS_8",
				CheckType: "terraform",
				Resource:  "aws_instance.api_server",
				Message:   "Ensure that EC2 instances have detailed monitoring enabled",
				Severity:  "LOW",
			},
			{
				CheckID:   "CKV_AWS_135",
				CheckType: "terraform",
				Resource:  "aws_instance.api_server",
				Message:   "Ensure that EC2 is EBS optimized",
				Severity:  "LOW",
			},
			{
				CheckID:   "CKV2_AWS_41",
				CheckType: "terraform",
				Resource:  "aws_instance.api_server",
				Message:   "Ensure an IAM instance profile is attached to the EC2 instance",
				Severity:  "MEDIUM",
			},
		},
		CorrectionAttempts: 1,
	},
	{
		NaturalLanguageReq: "Set up a PostgreSQL RDS instance for the user authentication service with daily backups",
		GeneratedHCL: `resource "aws_db_instance" "auth_db" {
  identifier        = "auth-db-prod"
  engine            = "postgres"
  engine_version    = "15"
  instance_class    = "db.t3.micro"
  allocated_storage = 20
  username          = "authsvc"
  password          = var.db_password
  skip_final_snapshot = true

  tags = { Service = "auth", Environment = "production" }
}`,
		PlanOutput:   "Plan: 1 to add, 0 to change, 0 to destroy.",
		CheckovScore: 54,
		CheckovWarnings: []models.CheckovWarning{
			{
				CheckID:   "CKV_AWS_17",
				CheckType: "terraform",
				Resource:  "aws_db_instance.auth_db",
				Message:   "Ensure that RDS database is not publicly accessible",
				Severity:  "HIGH",
			},
			{
				CheckID:   "CKV_AWS_16",
				CheckType: "terraform",
				Resource:  "aws_db_instance.auth_db",
				Message:   "Ensure that RDS database has encryption at-rest enabled",
				Severity:  "HIGH",
			},
			{
				CheckID:   "CKV_AWS_157",
				CheckType: "terraform",
				Resource:  "aws_db_instance.auth_db",
				Message:   "Ensure that RDS instances have Multi-AZ enabled",
				Severity:  "MEDIUM",
			},
			{
				CheckID:   "CKV_AWS_293",
				CheckType: "terraform",
				Resource:  "aws_db_instance.auth_db",
				Message:   "Ensure that AWS RDS database instance has deletion protection enabled",
				Severity:  "MEDIUM",
			},
		},
		CorrectionAttempts: 2,
	},
}

// SeedDemoRequests deletes all existing infra_requests and inserts the 3 demo rows.
func SeedDemoRequests(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "DELETE FROM audit_logs"); err != nil {
		return fmt.Errorf("delete audit_logs: %w", err)
	}

	if _, err := tx.Exec(ctx, "DELETE FROM infra_requests"); err != nil {
		return fmt.Errorf("delete infra_requests: %w", err)
	}

	const q = `
		INSERT INTO infra_requests
			(natural_language_req, generated_hcl, terraform_plan_output,
			 checkov_score, checkov_warnings, correction_attempts, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')`

	for _, r := range demoRows {
		warningsJSON, err := json.Marshal(r.CheckovWarnings)
		if err != nil {
			return fmt.Errorf("marshal warnings for %q: %w", r.NaturalLanguageReq, err)
		}
		if _, err := tx.Exec(ctx, q,
			r.NaturalLanguageReq,
			r.GeneratedHCL,
			r.PlanOutput,
			r.CheckovScore,
			warningsJSON,
			r.CorrectionAttempts,
		); err != nil {
			return fmt.Errorf("insert demo row %q: %w", r.NaturalLanguageReq, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
