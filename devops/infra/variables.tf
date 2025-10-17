variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "cluster_name" {
  description = "ECS cluster name"
  type        = string
  default     = "faceit-cluster"
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t4g.nano"
}

variable "account_id" {
  description = "AWS account ID"
  type        = string
  default     = "506636091874"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "postgres_user" {
  description = "PostgreSQL user"
  type        = string
  sensitive   = true
}

variable "postgres_password" {
  description = "PostgreSQL password"
  type        = string
  sensitive   = true
}

variable "db_volume_size" {
  description = "Size of the EBS volume for database in GB"
  type        = number
  default     = 20
}