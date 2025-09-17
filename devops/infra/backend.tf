terraform {
  backend "s3" {
    bucket         = "faceit-terraform-state"
    key            = "infra/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-state-lock"
    profile        = "cpx-valero"
  }
}