locals {
  testuser_name_prefix = "test-vault-root"
}

resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  thumbprint_list = [
    "6938fd4d98bab03faadb97b34396831e3780aea1",  // fetched on Apr. 10, 2022
  ]

  client_id_list  = ["sts.amazonaws.com"]
}

resource "aws_iam_role" "github_ci" {
  name = "github-ci"
  description = "Role that can be assumed by the GitHub CI to run acceptance tests against a real IAM instance"

  assume_role_policy = <<EOT
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "${aws_iam_openid_connect_provider.github.arn}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "ForAllValues:StringLike": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
          "token.actions.githubusercontent.com:sub": "repo:${local.github_owner}/${local.github_repository}:*"
        }
      }
    }
  ]
}
EOT
}

resource "aws_iam_role_policy" "testuser_management" {
  role = aws_iam_role.github_ci.id
  name = "testuser-management"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "iam:CreateUser"
      ],
      "Resource": [
        "arn:aws:iam::${data.aws_caller_identity.this.account_id}:user/${local.testuser_name_prefix}-*"
      ],
      "Condition": {
        "StringEquals": {
          "iam:PermissionsBoundary": "${aws_iam_policy.testuser_boundary.arn}"
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "iam:CreateAccessKey",
        "iam:ListAccessKeys",
        "iam:DeleteAccessKey",
        "iam:PutUserPolicy",
        "iam:GetUserPolicy",
        "iam:DeleteUserPolicy",
        "iam:DeleteUser",
        "iam:GetUser"
      ],
      "Resource": [
        "arn:aws:iam::${data.aws_caller_identity.this.account_id}:user/${local.testuser_name_prefix}-*"
      ]
    }
  ]
}
EOF
}

resource "aws_iam_policy" "testuser_boundary" {
  name        = "github-ci-testuser-boundaries"
  description = "Permission boundaries which must be applied to every user created by ${aws_iam_role.github_ci.name}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "iam:CreateAccessKey",
        "iam:DeleteAccessKey",
        "iam:GetUser"
      ],
      "Resource": "arn:aws:iam::${data.aws_caller_identity.this.account_id}:user/$${aws:username}"
    }
  ]
}
EOF
}
