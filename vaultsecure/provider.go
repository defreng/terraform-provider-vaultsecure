package vaultsecure

import (
	"context"
	"fmt"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	vault "github.com/hashicorp/vault/api"
)

func New() tfsdk.Provider {
	return &provider{}
}

type provider struct {
	iam   *iam.Client
	sts   *sts.Client
	vault *vault.Client
}

func (p *provider) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"vault_address": {
				Type:     types.StringType,
				Optional: true,
			},
			"vault_namespace": {
				Type:     types.StringType,
				Optional: true,
			},
		},
	}, nil
}

// Provider schema struct
type providerData struct {
	VaultAddress   types.String `tfsdk:"vault_address"`
	VaultNamespace types.String `tfsdk:"vault_namespace"`
}

func (p *provider) Configure(ctx context.Context, req tfsdk.ConfigureProviderRequest, resp *tfsdk.ConfigureProviderResponse) {
	// Retrieve provider data from configuration
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Load AWS Configuration
	// ... as we only access global AWS services (IAM, STS), we don't care about the region
	cfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create AWS configuration",
			fmt.Sprintf("Received an error while loading the AWS configuration: %v", err),
		)
		return
	}
	p.iam = iam.NewFromConfig(cfg)
	p.sts = sts.NewFromConfig(cfg)

	// Load Vault Configuration
	vaultConfig := vault.DefaultConfig()
	if !config.VaultAddress.Null {
		vaultConfig.Address = config.VaultAddress.Value
	}

	p.vault, err = vault.NewClient(vaultConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create Vault client",
			err.Error(),
		)
		return
	}

	if !config.VaultNamespace.Null {
		p.vault.SetNamespace(config.VaultNamespace.Value)
	}
}

// GetResources - Defines provider resources
func (p *provider) GetResources(_ context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
	return map[string]tfsdk.ResourceType{
		"vaultsecure_aws_secret_access_key": resourceAwsSecretAccessKeyType{},
	}, nil
}

// GetDataSources - Defines provider data sources
func (p *provider) GetDataSources(_ context.Context) (map[string]tfsdk.DataSourceType, diag.Diagnostics) {
	return map[string]tfsdk.DataSourceType{}, nil
}
