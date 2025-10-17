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

variable "discord_bot_token" {
  type      = string
  sensitive = true
}

variable "name" {
  type = string
  default = "faceit-discord-bot"
}

variable "env" {
  type = string
  default = "dev"
}


variable "discord_guild_id" {
  type = string
}

variable "discord_app_id" {
  type = string
}

variable "voice_category_id" {
  type = string
}

variable "afk_channel_id" {
  type = string
}

variable "admin_role_ids" {
  type = string
}

variable "faceit_api_key" {
  type      = string
  sensitive = true
}

variable "faceit_hub_id" {
  type = string
}

variable "webhook_header_value" {
  type      = string
  sensitive = true
}

variable "webhook_header_name" {
  type = string
}

variable "faceit_oauth_client_id" {
  type    = string
  default = ""
}

variable "faceit_oauth_client_secret" {
  type      = string
  sensitive = true
  default   = ""
}

variable "faceit_oauth_redirect_url" {
  type    = string
  default = ""
}

variable "http_addr" {
  type    = string
  default = ":8080"
}
