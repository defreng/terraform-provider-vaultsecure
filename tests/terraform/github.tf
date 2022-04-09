locals {
  github_repository = "terraform-provider-vaultsecure"
}

resource "github_actions_secret" "aws_region" {
  repository      = local.github_repository
  secret_name     = "AWS_REGION"
  plaintext_value = data.aws_region.current.name
}

resource "github_actions_secret" "aws_role_to_assume" {
  repository      = local.github_repository
  secret_name     = "AWS_ROLE_TO_ASSUME"
  plaintext_value = aws_iam_role.github_ci.arn
}

resource "github_actions_secret" "aws_testuser_name_prefix" {
  repository      = local.github_repository
  secret_name     = "AWS_TESTUSER_NAME_PREFIX"
  plaintext_value = local.testuser_name_prefix
}

resource "github_actions_secret" "aws_testuser_permissions_boundary_arn" {
  repository      = local.github_repository
  secret_name     = "AWS_TESTUSER_PERMISSIONS_BOUNDARY_ARN"
  plaintext_value = aws_iam_policy.testuser_boundary.arn
}