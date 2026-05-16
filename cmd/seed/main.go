package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
	"github.com/joho/godotenv"
)

func strPtr(s string) *string { return &s }

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	pool, err := db.Connect(dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	moduleStore := store.NewModuleStore(pool)

	modules := []models.TerraformModule{
		{
			Name:        "s3-secure-bucket",
			Description: strPtr("S3 bucket with versioning, AES-256 encryption, and all public access blocked"),
			HCLTemplate: `resource "aws_s3_bucket" "this" {
  bucket = var.bucket_name

  tags = var.tags
}

resource "aws_s3_bucket_versioning" "this" {
  bucket = aws_s3_bucket.this.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "this" {
  bucket = aws_s3_bucket.this.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "this" {
  bucket                  = aws_s3_bucket.this.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

variable "bucket_name" { type = string }
variable "tags"        { type = map(string); default = {} }`,
			ResourceTypes: []string{"aws_s3_bucket", "aws_s3_bucket_versioning", "aws_s3_bucket_server_side_encryption_configuration", "aws_s3_bucket_public_access_block"},
		},
		{
			Name:        "ec2-secure-instance",
			Description: strPtr("EC2 instance with no public IP, port 443 ingress only, and an IAM instance profile"),
			HCLTemplate: `resource "aws_security_group" "this" {
  name        = "${var.name}-sg"
  description = "Allow HTTPS inbound only"
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

resource "aws_iam_instance_profile" "this" {
  name = "${var.name}-profile"
  role = var.iam_role_name
}

resource "aws_instance" "this" {
  ami                         = var.ami_id
  instance_type               = var.instance_type
  subnet_id                   = var.subnet_id
  vpc_security_group_ids      = [aws_security_group.this.id]
  iam_instance_profile        = aws_iam_instance_profile.this.name
  associate_public_ip_address = false

  tags = merge(var.tags, { Name = var.name })
}

variable "name"          { type = string }
variable "ami_id"        { type = string }
variable "instance_type" { type = string; default = "t3.micro" }
variable "vpc_id"        { type = string }
variable "subnet_id"     { type = string }
variable "iam_role_name" { type = string }
variable "tags"          { type = map(string); default = {} }`,
			ResourceTypes: []string{"aws_instance", "aws_security_group", "aws_iam_instance_profile"},
		},
		{
			Name:        "rds-postgres-secure",
			Description: strPtr("RDS PostgreSQL with encryption at rest, no public access, SSL enforced, and 7-day backups"),
			HCLTemplate: `resource "aws_db_subnet_group" "this" {
  name       = "${var.identifier}-subnet-group"
  subnet_ids = var.subnet_ids
}

resource "aws_security_group" "this" {
  name        = "${var.identifier}-sg"
  description = "RDS PostgreSQL access"
  vpc_id      = var.vpc_id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = var.allowed_security_group_ids
  }
}

resource "aws_db_parameter_group" "this" {
  name   = "${var.identifier}-params"
  family = "postgres15"

  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
}

resource "aws_db_instance" "this" {
  identifier              = var.identifier
  engine                  = "postgres"
  engine_version          = "15"
  instance_class          = var.instance_class
  allocated_storage       = var.allocated_storage
  storage_encrypted       = true
  username                = var.username
  password                = var.password
  db_subnet_group_name    = aws_db_subnet_group.this.name
  vpc_security_group_ids  = [aws_security_group.this.id]
  parameter_group_name    = aws_db_parameter_group.this.name
  publicly_accessible     = false
  backup_retention_period = 7
  skip_final_snapshot     = false
  final_snapshot_identifier = "${var.identifier}-final"

  tags = var.tags
}

variable "identifier"                  { type = string }
variable "vpc_id"                      { type = string }
variable "subnet_ids"                  { type = list(string) }
variable "allowed_security_group_ids"  { type = list(string); default = [] }
variable "instance_class"              { type = string; default = "db.t3.micro" }
variable "allocated_storage"           { type = number; default = 20 }
variable "username"                    { type = string }
variable "password"                    { type = string; sensitive = true }
variable "tags"                        { type = map(string); default = {} }`,
			ResourceTypes: []string{"aws_db_instance", "aws_db_subnet_group", "aws_db_parameter_group", "aws_security_group"},
		},
		{
			Name:        "s3-cloudfront-distribution",
			Description: strPtr("S3 origin with CloudFront distribution using origin access control and HTTPS only"),
			HCLTemplate: `resource "aws_s3_bucket" "origin" {
  bucket = var.bucket_name
  tags   = var.tags
}

resource "aws_s3_bucket_public_access_block" "origin" {
  bucket                  = aws_s3_bucket.origin.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudfront_origin_access_control" "this" {
  name                              = "${var.bucket_name}-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "this" {
  enabled             = true
  default_root_object = "index.html"

  origin {
    domain_name              = aws_s3_bucket.origin.bucket_regional_domain_name
    origin_id                = "s3-${var.bucket_name}"
    origin_access_control_id = aws_cloudfront_origin_access_control.this.id
  }

  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-${var.bucket_name}"
    viewer_protocol_policy = "redirect-to-https"
    forwarded_values {
      query_string = false
      cookies { forward = "none" }
    }
  }

  restrictions {
    geo_restriction { restriction_type = "none" }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
    minimum_protocol_version       = "TLSv1.2_2021"
  }

  tags = var.tags
}

resource "aws_s3_bucket_policy" "origin" {
  bucket = aws_s3_bucket.origin.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "AllowCloudFrontOAC"
      Effect    = "Allow"
      Principal = { Service = "cloudfront.amazonaws.com" }
      Action    = "s3:GetObject"
      Resource  = "${aws_s3_bucket.origin.arn}/*"
      Condition = {
        StringEquals = {
          "AWS:SourceArn" = aws_cloudfront_distribution.this.arn
        }
      }
    }]
  })
}

variable "bucket_name" { type = string }
variable "tags"        { type = map(string); default = {} }`,
			ResourceTypes: []string{"aws_s3_bucket", "aws_cloudfront_distribution", "aws_cloudfront_origin_access_control", "aws_s3_bucket_policy"},
		},
		{
			Name:        "vpc-public-private",
			Description: strPtr("VPC with public and private subnets across two AZs and a NAT gateway for private egress"),
			HCLTemplate: `resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = merge(var.tags, { Name = var.name })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = merge(var.tags, { Name = "${var.name}-igw" })
}

resource "aws_subnet" "public" {
  count             = 2
  vpc_id            = aws_vpc.this.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 4, count.index)
  availability_zone = var.availability_zones[count.index]
  map_public_ip_on_launch = false
  tags = merge(var.tags, { Name = "${var.name}-public-${count.index}" })
}

resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.this.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 4, count.index + 2)
  availability_zone = var.availability_zones[count.index]
  tags = merge(var.tags, { Name = "${var.name}-private-${count.index}" })
}

resource "aws_eip" "nat" {
  domain = "vpc"
  tags   = merge(var.tags, { Name = "${var.name}-nat-eip" })
}

resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id
  tags          = merge(var.tags, { Name = "${var.name}-nat" })
  depends_on    = [aws_internet_gateway.this]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
  tags = merge(var.tags, { Name = "${var.name}-public-rt" })
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this.id
  }
  tags = merge(var.tags, { Name = "${var.name}-private-rt" })
}

resource "aws_route_table_association" "private" {
  count          = 2
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

variable "name"               { type = string }
variable "vpc_cidr"           { type = string; default = "10.0.0.0/16" }
variable "availability_zones" { type = list(string) }
variable "tags"               { type = map(string); default = {} }`,
			ResourceTypes: []string{"aws_vpc", "aws_subnet", "aws_internet_gateway", "aws_nat_gateway", "aws_route_table", "aws_eip"},
		},
	}

	ctx := context.Background()
	if err := moduleStore.BulkInsert(ctx, modules); err != nil {
		log.Fatalf("bulk insert modules: %v", err)
	}

	fmt.Printf("seeded %d terraform modules\n", len(modules))
	for _, m := range modules {
		fmt.Printf("  - %s\n", m.Name)
	}
}
