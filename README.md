# terraform-provider-vaultsecure

This provider is used to securely setup [AWS secret engines](https://www.vaultproject.io/docs/secrets/aws) in Vault, without storing an AWS access key and its secret in the Terraform state.

## Usage

Check out the documentation at: https://registry.terraform.io/providers/defreng/vaultsecure/latest/docs

## Future work

Some ideas for future improvements or features 🤓 

* Improve tests
  * Measure test coverage
  * Add tests that cover import of the resource
  * Add tests that check behavior when the engine_path or iam username is changed
* add support also for other cloud secret backends (gcp, azure, alicloud, ...)