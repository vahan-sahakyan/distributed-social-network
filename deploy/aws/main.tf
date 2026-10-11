# one EC2 node running k3s with the prod env (gitops bootstrap/root.yaml); no load balancer,
# no NAT, no Elastic IP. Costs and rules in deploy/aws/README.md

terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.68"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.4"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.9"
    }
  }

  # shared by every machine, in the bucket from state/ (README):
  # terraform init -backend-config="bucket=dsn-tfstate-<account id>"
  backend "s3" {
    key          = "node/terraform.tfstate"
    region       = "us-east-1"
    use_lockfile = true
  }
}

# ---------------------------------------------------------------------------
# variables
# ---------------------------------------------------------------------------

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "project" {
  description = "Name prefix and Project tag for every resource"
  type        = string
  default     = "dsn"
}

variable "aws_account_id" {
  description = "The only AWS account terraform may touch (guards against the wrong credentials)"
  type        = string
}

variable "instance_type" {
  description = "Largest Free plan type: 2 vCPU, 8 GB, no CPU credits to run out of"
  type        = string
  default     = "m7i-flex.large"

  validation {
    condition     = contains(["m7i-flex.large"], var.instance_type)
    error_message = "The prod profile needs 8 GB; m7i-flex.large is the only Free plan type that has it."
  }
}

variable "root_volume_gb" {
  description = "Root gp3 volume: OS, images and every PVC (k3s local-path)"
  type        = number
  default     = 30
}

variable "ssh_cidr" {
  description = "The only CIDR allowed on port 22, your own /32"
  type        = string

  validation {
    condition     = endswith(var.ssh_cidr, "/32") && var.ssh_cidr != "0.0.0.0/32"
    error_message = "Use a single address: curl -s https://checkip.amazonaws.com, plus /32."
  }
}

variable "budget_email" {
  description = "Email that receives the budget alerts"
  type        = string
}

variable "budget_usd" {
  description = "Monthly gross cost (credits excluded) across the account that triggers an alert"
  type        = number
  default     = 100
}

# ---------------------------------------------------------------------------
# provider
# ---------------------------------------------------------------------------

provider "aws" {
  region              = var.region
  allowed_account_ids = [var.aws_account_id]

  default_tags {
    tags = {
      Project   = var.project
      ManagedBy = "terraform"
    }
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

# latest Amazon Linux 2023 x86_64; m7i-flex is Intel
data "aws_ssm_parameter" "al2023" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
}

# ---------------------------------------------------------------------------
# network: own vpc (free) with one public subnet; no nat gateway, it bills hourly
# ---------------------------------------------------------------------------

resource "aws_vpc" "main" {
  cidr_block           = "10.1.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = "${var.project}-vpc" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = { Name = "${var.project}-igw" }
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.1.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = { Name = "${var.project}-public" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = { Name = "${var.project}-public" }
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# ---------------------------------------------------------------------------
# security group: ssh from one address, http/https from anywhere. The Kubernetes
# API stays closed; kubectl goes through an ssh tunnel
# ---------------------------------------------------------------------------

resource "aws_security_group" "node" {
  name        = "${var.project}-node"
  description = "SSH from the owner, HTTP and HTTPS to the k3s node"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${var.project}-node" }
}

resource "aws_vpc_security_group_ingress_rule" "ssh" {
  security_group_id = aws_security_group.node.id
  description       = "SSH"
  cidr_ipv4         = var.ssh_cidr
  ip_protocol       = "tcp"
  from_port         = 22
  to_port           = 22
}

resource "aws_vpc_security_group_ingress_rule" "http" {
  security_group_id = aws_security_group.node.id
  description       = "HTTP, redirects to HTTPS"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = 80
  to_port           = 80
}

resource "aws_vpc_security_group_ingress_rule" "https" {
  security_group_id = aws_security_group.node.id
  description       = "HTTPS"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.node.id
  description       = "All outbound"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

# ---------------------------------------------------------------------------
# ssh key: generated here, written as a 0600 .pem next to this file (git-ignored)
# ---------------------------------------------------------------------------

resource "tls_private_key" "ssh" {
  algorithm = "ED25519"
}

resource "aws_key_pair" "ssh" {
  key_name   = "${var.project}-key"
  public_key = tls_private_key.ssh.public_key_openssh
}

resource "local_sensitive_file" "ssh_pem" {
  filename        = "${path.module}/${var.project}.pem"
  content         = tls_private_key.ssh.private_key_openssh
  file_permission = "0600"
}

# ---------------------------------------------------------------------------
# the node
# ---------------------------------------------------------------------------

resource "aws_instance" "node" {
  ami                    = data.aws_ssm_parameter.al2023.insecure_value
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.node.id]
  key_name               = aws_key_pair.ssh.key_name

  # one auto-assigned public ipv4: it changes on stop/start, but unlike an Elastic IP
  # it stops billing while the node is stopped. The app works on any address
  associate_public_ip_address = true

  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 2
  }

  root_block_device {
    volume_size           = var.root_volume_gb
    volume_type           = "gp3"
    encrypted             = true
    delete_on_termination = true
  }

  user_data = file("${path.module}/user_data.sh")

  # replacing the node wipes every volume: a newer AMI or an edited first-boot script
  # must not do it silently
  lifecycle {
    ignore_changes = [ami, user_data]
  }

  tags = { Name = "${var.project}-node" }
}

# ---------------------------------------------------------------------------
# credit burn guardrail: gross cost (credits excluded), since on the Free plan net cost
# stays $0 until the credits run out and the account closes. Account-wide, so it also
# counts anything else running in the account
# ---------------------------------------------------------------------------

resource "aws_budgets_budget" "credit_burn" {
  name         = "${var.project}-credit-burn"
  budget_type  = "COST"
  limit_amount = format("%.1f", var.budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  cost_types {
    include_credit = false
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    notification_type          = "ACTUAL"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    subscriber_email_addresses = [var.budget_email]
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    notification_type          = "FORECASTED"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    subscriber_email_addresses = [var.budget_email]
  }
}

# ---------------------------------------------------------------------------
# outputs
# ---------------------------------------------------------------------------

output "instance_id" {
  value = aws_instance.node.id
}

output "public_ip" {
  value = aws_instance.node.public_ip
}

output "ssh_command" {
  value = "ssh -i ${local_sensitive_file.ssh_pem.filename} ec2-user@${aws_instance.node.public_ip}"
}

output "app_url" {
  value = "https://${aws_instance.node.public_ip}"
}
