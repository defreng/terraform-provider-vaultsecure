# Resource `vaultsecure_aws_secret_access_key`

This resource creates an AWS access key and secret key pair that is only known to Vault and AWS. The access key will be configured as the root credentials in the given AWS secret engine.

It does so by creating a new AWS access key for the given IAM user, and then directly passing it into the AWS secret engine configuration. After doing so, it calls the Vault [root credential rotation API](https://www.vaultproject.io/api-docs/secret/aws#rotate-root-iam-credentials) to internally rotate the AWS secret engine's root credentials. This even renders the access key invalid that was known to this provider (in memory only). Finally, this resource will be 'taking ownership' of the new access key that was created by Vault (only knowing its ID) and tracking it in the Terraform state. As such, removing the resource will remove the access key from AWS and Vault.

-> **Note:** This resource is designed to silently take over ownership of a new access key if it was rotated using the [rotate-root](https://www.vaultproject.io/api-docs/secret/aws#rotate-root-iam-credentials) API of Vault in between terraform executions.

## Example Usage

```terraform
// Create the AWS resources (IAM root user and required policies to allow key rotation)
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

// Mount an AWS secret engine in Vault
resource "vault_mount" "aws" {
  path = "aws-test"
  type = "aws"
}

// Use this resource to create an AWS access key and configure it in the Vault secret engine
resource "vaultsecure_aws_secret_access_key" "this" {
  aws_iam_username = aws_iam_user.vault.name
  vault_engine_path = vault_mount.aws.path
}
```

## Argument Reference

- `aws_iam_username` - (Required) Username of the IAM root use that should be used by the Vault AWS secret engine
- `vault_engine_path` - (Required) Path of the Vault secret engine that should be configured with an access key to the given IAM user

## Import

Import is supported using the following syntax:

```shell
# An existing access key can be imported using an ID made up of '<vault_engine_path>:<aws_iam_username>', e.g.
terraform import vaultsecure_aws_secret_access_key.this "aws-test:vault-root-test"
```

If all conditions are met and the import can be performed successfully, the provider will trigger a credential rotation using the [rotate-root](https://www.vaultproject.io/api-docs/secret/aws#rotate-root-iam-credentials) API of Vault, to ensure that the secret key is only known to Vault.