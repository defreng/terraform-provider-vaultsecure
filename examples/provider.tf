terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
      version = "~> 4.2"
    }
    vaultsecure = {
      source = "defreng/vaultsecure"
    }
    vault = {
      source  = "hashicorp/vault"
      version = "~> 3.3"
    }
  }

  required_version = ">= 1.1.0"
}

provider "aws" {
  region = "eu-central-1"
}

locals {
  vault_address = "https://iot-core-poc.vault.93bc5f50-771e-4248-a342-6081d237d622.aws.hashicorp.cloud:8200/"
  vault_namespace = "admin"
}

provider "vault" {
  address = local.vault_address
  namespace = local.vault_namespace
}

provider "vaultsecure" {
  vault_address = local.vault_address
  vault_namespace = local.vault_namespace
}