resource "aws_iam_user" "vault" {
  name = "vault-root-test"
}

resource "aws_iam_user_policy" "rotate_self" {
  name = "allow-self-rotation"
  user = aws_iam_user.vault.name

  policy = <<EOT
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
      "Resource": "${aws_iam_user.vault.arn}"
    }
  ]
}
EOT
}

resource "vault_mount" "aws" {
  path = "aws-test"
  type = "aws"
}

resource "vaultsecure_aws_secret_access_key" "this" {
  aws_iam_username = aws_iam_user.vault.name
  vault_engine_path = vault_mount.aws.path
}