package vaultsecure

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type AwsSecretAccessKey struct {
	ID types.String `tfsdk:"id"`

	AwsIamUsername           types.String `tfsdk:"aws_iam_username"`
	AwsAccessKeyID           types.String `tfsdk:"aws_access_key_id"`
	AwsAccessKeyCreationDate types.String `tfsdk:"aws_access_key_creation_date"`

	VaultEnginePath  types.String `tfsdk:"vault_engine_path"`
	VaultAccessKeyID types.String `tfsdk:"vault_access_key_id"`
}
