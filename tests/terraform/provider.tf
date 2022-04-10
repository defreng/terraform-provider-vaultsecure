terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.9"
    }
    local = {
      source = "hashicorp/local"
      version = "~> 2.2"
    }
  }

  cloud {
    organization = "defreng"

    workspaces {
      name = "vaultsecure"
    }
  }
}

locals {
  github_owner = "defreng"
  github_repository = "terraform-provider-vaultsecure"
}

provider "aws" {
  region = "us-east-1"

  allowed_account_ids = [
    "405416892799",  // vaultsecure account
  ]
}

data "aws_caller_identity" "this" {}

data "aws_region" "current" {}