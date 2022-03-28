---
page_title: "Provider: VaultSecure"
subcategory: ""
description: |-
Set Vault AWS secret backend root credentials without leaking them into the Terraform state
---

# VaultSecure Provider

This provider is used to securely setup AWS secret backends in Vault **without storing an AWS access key and its secret in the Terraform state**.

It does this by creating and managing an AWS access key, and directly storing the secret key in the Vault secret backend configuration. On top of that, after passing the secret key to Vault, it also uses the [key rotation feature in Vault](https://www.vaultproject.io/api-docs/secret/aws#rotate-root-iam-credentials) to ensure that the secret key is not known to any entity outside of Vault and AWS.

## Example Usage

```terraform
provider "vaultsecure" {
  // The provider also needs to authenticate with AWS, which currently 
  // is only supported with the AWS_* environment variables.
  // Set them correctly (i.e. your AWS_PROFILE)
  
  // The Vault token must be provided
  // in the VAULT_TOKEN environment variable
  vault_address = "https://myvaultserver.com:8200"
}
```

## Schema

### Required

- **vault_address** (String) The URL of the Vault server (e.g. `https://myvaultserver.com:8200`)

### Optional

- **vault_namespace** (String, Optional) Vault namespace that should be used (defaults to `null`)