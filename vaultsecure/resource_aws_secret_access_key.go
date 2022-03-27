package vaultsecure

import (
	"context"
	"errors"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"strings"
	"time"
)

var ErrAccessKeyNotFound = errors.New("AWS access key with the given AwsAccessKeyID was not found within the given user")

type resourceAwsSecretAccessKeyType struct{}

func (r resourceAwsSecretAccessKeyType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},

			"aws_iam_username": {
				Type:        types.StringType,
				Required:    true,
				Description: "Username of the AWS IAM user.",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.RequiresReplace(),
				},
			},
			"aws_access_key_id": {
				Type:     types.StringType,
				Computed: true,
			},
			"aws_access_key_creation_date": {
				Type:        types.StringType,
				Computed:    true,
				Description: "Contains the date (in RFC3339 format) when the access key was created",
			},

			"vault_engine_path": {
				Type:        types.StringType,
				Required:    true,
				Description: "Path to the AWS Secret engine in Vault.",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.RequiresReplace(),
				},
			},
			"vault_access_key_id": {
				Type:     types.StringType,
				Computed: true,
			},
		},
	}, nil
}

func (r resourceAwsSecretAccessKeyType) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return resourceAwsSecretAccessKey{
		p: *(p.(*provider)),
	}, nil
}

type resourceAwsSecretAccessKey struct {
	p provider
}

// ModifyPlan checks if the access key that is used by the vault engine is identical to the one we track in AWS
//
// If not, we want to force a replacement of the resource, as we are not sure that the access key used by Vault
// is working correctly (it might have an invalid secret key set).
func (r resourceAwsSecretAccessKey) ModifyPlan(ctx context.Context, req tfsdk.ModifyResourcePlanRequest, resp *tfsdk.ModifyResourcePlanResponse) {
	if req.State.Raw.IsNull() {
		// if we're creating the resource, no need to delete and recreate it
		return
	}

	if req.Plan.Raw.IsNull() {
		// if we're deleting the resource, no need to delete and recreate it
		return
	}

	var plan AwsSecretAccessKey
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.AwsAccessKeyID.Equal(plan.VaultAccessKeyID) {
		return
	}
	tflog.Info(ctx, "The AWS access key that was managed by this resource is no longer the one that is configured in Vault")

	// not sure if there is a more "type-safe" way to get the attribute path...
	// i.e. in a way that would cause compile-time errors if the attribute is renamed
	resp.RequiresReplace = append(resp.RequiresReplace,
		tftypes.NewAttributePath().WithAttributeName("vault_access_key_id"))

	plan.AwsAccessKeyID.Unknown = true
	plan.AwsAccessKeyCreationDate.Unknown = true
	plan.VaultAccessKeyID.Unknown = true

	diags = resp.Plan.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceAwsSecretAccessKey) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	var plan AwsSecretAccessKey
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := r.p.iam.ListAccessKeys(ctx, &iam.ListAccessKeysInput{UserName: aws.String(plan.AwsIamUsername.Value), MaxItems: aws.Int32(1)})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to get existing access keys",
			fmt.Sprintf("Failed to fetch the list of existing access keys for the given IAM user: %v", err),
		)
		return
	}
	if len(keys.AccessKeyMetadata) > 0 {
		resp.Diagnostics.AddError(
			"Existing access key detected",
			"At least one existing access key was found on the specified IAM user. This is not allowed, as the access key "+
				"will be created by this resources and rotated by Vault.",
		)
		return
	}

	key, err := r.p.iam.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{UserName: aws.String(plan.AwsIamUsername.Value)})
	if err != nil {
		return
	}

	state := AwsSecretAccessKey{
		ID: types.String{Value: fmt.Sprintf("%s:%s", plan.VaultEnginePath.Value, plan.AwsIamUsername.Value)},

		AwsAccessKeyID:           types.String{Value: *key.AccessKey.AccessKeyId},
		AwsIamUsername:           types.String{Value: *key.AccessKey.UserName},
		AwsAccessKeyCreationDate: types.String{Value: key.AccessKey.CreateDate.Format(time.RFC3339)},

		VaultEnginePath:  plan.VaultEnginePath,
		VaultAccessKeyID: types.String{Null: true},
	}
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set Access Key in AWS Secret Engine
	secretEngineData := map[string]interface{}{
		"access_key": *key.AccessKey.AccessKeyId,
		"secret_key": *key.AccessKey.SecretAccessKey,
	}
	_, err = r.p.vault.Logical().Write(
		fmt.Sprintf("%s/config/root", plan.VaultEnginePath.Value), secretEngineData)
	if err != nil {
		resp.Diagnostics.AddError("Error writing access key to the AWS backend", err.Error())
		return
	}
	tflog.Info(ctx, "AWS access key id before rotation", map[string]interface{}{
		"access_key_id": *key.AccessKey.AccessKeyId,
	})

	// Rotate the access key using the Vault API
	// We need to retry this, as IAM might take a few seconds to become consistent
	err = retry.Do(
		func() error {
			_, err = r.p.vault.Logical().Write(
				fmt.Sprintf("%s/config/rotate-root", plan.VaultEnginePath.Value), map[string]interface{}{})

			return err
		},
		retry.Delay(3*time.Second),
		retry.Attempts(5),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error rotating the access key in Vault", err.Error())
		return
	}

	// Fetch the ID of the new AWS access key that was created from Vault - as we want to take ownership of that one
	vResp, err := r.p.vault.Logical().Read(fmt.Sprintf("%s/config/root", plan.VaultEnginePath.Value))
	if err != nil {
		resp.Diagnostics.AddError("Error reading the new access key AwsAccessKeyID from Vault", err.Error())
		return
	}
	state.AwsAccessKeyID = types.String{Value: vResp.Data["access_key"].(string)}

	err = r.refreshState(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Could not refresh the state data", err.Error())
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceAwsSecretAccessKey) refreshState(ctx context.Context, state *AwsSecretAccessKey) error {
	// Find the Access Key in AWS and refresh its creation date
	creationDate, err := getAwsAccessKeyCreationDate(ctx, r.p.iam, state.AwsIamUsername.Value, state.AwsAccessKeyID.Value)
	if err != nil {
		return err
	}
	state.AwsAccessKeyCreationDate = types.String{Value: creationDate.Format(time.RFC3339)}

	// Refresh the access key ID that is configured in the vault engine
	vRead, err := r.p.vault.Logical().Read(fmt.Sprintf("%s/config/root", state.VaultEnginePath.Value))
	if err != nil {
		return err
	}
	state.VaultAccessKeyID = types.String{Value: vRead.Data["access_key"].(string)}

	return nil
}

func (r resourceAwsSecretAccessKey) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state AwsSecretAccessKey
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.refreshState(ctx, &state)
	if errors.Is(err, ErrAccessKeyNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not refresh the state data", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceAwsSecretAccessKey) Update(_ context.Context, _ tfsdk.UpdateResourceRequest, _ *tfsdk.UpdateResourceResponse) {
	panic("update method not implemented as all possible changes should result in resource replacement")
}

func (r resourceAwsSecretAccessKey) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var state AwsSecretAccessKey
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.p.iam.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
		UserName:    aws.String(state.AwsIamUsername.Value),
		AccessKeyId: aws.String(state.AwsAccessKeyID.Value),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not delete access key",
			err.Error(),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

// ImportState expects the following conditions to be met:
// - The Vault AWS secret engine is configured with an access key ID
// - The AWS IAM user has a single access key configured, identical to the one in Vault
//
// If all checks succeed, we will also perform a access key rotation before finishing the import
func (r resourceAwsSecretAccessKey) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	idParts := strings.Split(req.ID, ":")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError("Invalid ID format",
			fmt.Sprintf("Expected import identifier to be in the format '<vault_engine_path>:<aws_iam_username>', got '%s'", req.ID))
		return
	}

	state := AwsSecretAccessKey{
		ID:              types.String{Value: req.ID},
		VaultEnginePath: types.String{Value: idParts[0]},
		AwsIamUsername:  types.String{Value: idParts[1]},
	}

	// Read used access key ID from Vault
	vResp, err := r.p.vault.Logical().Read(fmt.Sprintf("%s/config/root", state.VaultEnginePath.Value))
	if err != nil {
		resp.Diagnostics.AddError("Error reading the access key ID from Vault", err.Error())
		return
	}
	state.VaultAccessKeyID = types.String{Value: vResp.Data["access_key"].(string)}

	// Ensure that the IAM user has a single access key configured which ID is identical to the one in Vault
	iamResp, err := r.p.iam.ListAccessKeys(ctx, &iam.ListAccessKeysInput{
		UserName: aws.String(state.AwsIamUsername.Value),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error listing the access keys of the IAM user", err.Error())
		return
	}
	if len(iamResp.AccessKeyMetadata) != 1 {
		resp.Diagnostics.AddError("The IAM user does have more than a single access key configured", "")
		return
	}
	if *iamResp.AccessKeyMetadata[0].AccessKeyId != state.VaultAccessKeyID.Value {
		resp.Diagnostics.AddError("The access key ID of the IAM user is not identical to the one configured in Vault", "")
		return
	}
	state.AwsAccessKeyID = types.String{Value: *iamResp.AccessKeyMetadata[0].AccessKeyId}

	// As we are not sure if the access key secret was leaked outside of Vault, we will trigger a key rotation now
	_, err = r.p.vault.Logical().Write(
		fmt.Sprintf("%s/config/rotate-root", state.VaultEnginePath.Value), map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Error rotating the access key ID in Vault", err.Error())
		return
	}

	// Fetch the ID of the new AWS access key that was created from Vault - as we want to take ownership of that one
	vResp, err = r.p.vault.Logical().Read(fmt.Sprintf("%s/config/root", state.VaultEnginePath.Value))
	if err != nil {
		resp.Diagnostics.AddError("Error reading the new access key AwsAccessKeyID from Vault", err.Error())
		return
	}
	state.AwsAccessKeyID = types.String{Value: vResp.Data["access_key"].(string)}

	err = r.refreshState(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Could not refresh the state data", err.Error())
		return
	}

	diags := resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func getAwsAccessKeyCreationDate(ctx context.Context, iamClient *iam.Client, username string, accessKeyID string) (*time.Time, error) {
	paginator := iam.NewListAccessKeysPaginator(iamClient, &iam.ListAccessKeysInput{UserName: aws.String(username)})

	for paginator.HasMorePages() {
		keys, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, key := range keys.AccessKeyMetadata {
			if *key.AccessKeyId == accessKeyID {
				return key.CreateDate, nil
			}
		}
	}

	return nil, ErrAccessKeyNotFound
}
