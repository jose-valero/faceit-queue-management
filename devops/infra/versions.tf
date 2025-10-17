terraform {
  required_version = "1.13.2"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.14.0"
    }
  }
}

provider "aws" {
  region  = var.aws_region
  profile = "cpx-valero"
}