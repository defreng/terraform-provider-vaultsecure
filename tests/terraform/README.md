# CI Testing Infrastructure

This terraform code is used to setup an AWS account that can be used to execute the acceptance tests from the GitHub CI.

Namely, it creates an IAM role that can be assumed from the GitHub CI with enough permissions to create/delete IAM users within the acceptance tests.

## Create/update the infrastructure

The terraform code must be executed by a maintainer with his own (human) AWS credentials on his local machine. The Terraform state is maintained in [Terraform Cloud](https://app.terraform.io/app/defreng/workspaces/vaultsecure).