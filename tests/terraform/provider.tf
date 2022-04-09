terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.9"
    }
    github = {
      source  = "integrations/github"
      version = "~> 4.23"
    }
  }

  cloud {
    organization = "defreng"

    workspaces {
      name = "vaultsecure"
    }
  }
}

provider "aws" {
  region = "us-east-1"

  allowed_account_ids = [
    "405416892799",  // vaultsecure account
  ]
}

data "aws_caller_identity" "this" {}

data "aws_region" "current" {}