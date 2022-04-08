package vaultsecure

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	vault "github.com/hashicorp/vault/api"
	"os"
	"strings"
	"testing"
)

var testIAMClient *iam.Client
var testVaultClient *vault.Client

func init() {
	if os.Getenv(resource.EnvTfAcc) == "" {
		return
	}

	ctx := context.Background()

	// Create IAM test client
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic("failed to create IAM test client: " + err.Error())
	}

	testIAMClient = iam.NewFromConfig(awsConfig)

	// Create Vault test client
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = os.Getenv("VAULT_ADDR")

	testVaultClient, err = vault.NewClient(vaultConfig)
	if err != nil {
		panic("failed to create Vault test client: " + err.Error())
	}
}

func TestAccResourceAwsSecretAccessKeyType_basic(t *testing.T) {
	iamUsername := testAccCreateIAMUser(t)
	awsSecretEnginePath := testAccCreateAWSSecretEngine(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceAwsSecretAccessKeyType_basic(iamUsername, awsSecretEnginePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckExposedAWSAccessKeyIDExistsAndIsOnlyOne(iamUsername),
					testAccCheckExposedAWSAccessKeyIDMatchesVaultConfiguration(awsSecretEnginePath),
					testAccCheckExposedAWSAndVaultAccessKeyIDAreEqual,
				),
			},
			// rotate the root credentials of the vault engine. This will actually check if the configured
			// access/secret keys are working correctly!
			{
				Config: testAccResourceAwsSecretAccessKeyType_basic(iamUsername, awsSecretEnginePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccRotateRoot(awsSecretEnginePath),
				),
			},
			// Execute apply once more to check if it can take ownership of a rotated access key
			{
				Config: testAccResourceAwsSecretAccessKeyType_basic(iamUsername, awsSecretEnginePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckExposedAWSAccessKeyIDExistsAndIsOnlyOne(iamUsername),
					testAccCheckExposedAWSAccessKeyIDMatchesVaultConfiguration(awsSecretEnginePath),
					testAccCheckExposedAWSAndVaultAccessKeyIDAreEqual,
				),
			},
		},
	})
}

// testAccCreateIAMUser creates an IAM user in AWS with a random name that
// has a policy attached which allows to rotate its own access keys
func testAccCreateIAMUser(t *testing.T) string {
	ctx := context.Background()
	iamUsername := addRandomSuffix("test-user")

	// Create the IAM user
	user, err := testIAMClient.CreateUser(ctx, &iam.CreateUserInput{
		UserName: aws.String(iamUsername),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		paginator := iam.NewListAccessKeysPaginator(testIAMClient, &iam.ListAccessKeysInput{UserName: aws.String(iamUsername)})

		for paginator.HasMorePages() {
			keys, err := paginator.NextPage(ctx)
			if err != nil {
				t.Fatal(err)
			}

			for _, key := range keys.AccessKeyMetadata {
				_, err := testIAMClient.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
					AccessKeyId: key.AccessKeyId,
					UserName:    aws.String(iamUsername),
				})
				if err != nil {
					t.Fatal(err)
				}
				t.Errorf("deleted a dangling access key (ID: %s) from the test user. This should not happen", *key.AccessKeyId)
			}
		}

		_, err = testIAMClient.DeleteUser(ctx, &iam.DeleteUserInput{
			UserName: aws.String(iamUsername),
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// Attach a policy to allow key rotation
	policyDocument := strings.TrimSpace(fmt.Sprintf(`
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
			"Resource": "%s"
		}
	]
}
	`, *user.User.Arn))

	_, err = testIAMClient.PutUserPolicy(ctx, &iam.PutUserPolicyInput{
		UserName:       aws.String(iamUsername),
		PolicyName:     aws.String("allow-self-rotation"),
		PolicyDocument: aws.String(policyDocument),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, err = testIAMClient.DeleteUserPolicy(ctx, &iam.DeleteUserPolicyInput{
			UserName:   aws.String(iamUsername),
			PolicyName: aws.String("allow-self-rotation"),
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	return iamUsername
}

func testAccCreateAWSSecretEngine(t *testing.T) string {
	backendPath := addRandomSuffix("aws")

	_, err := testVaultClient.Logical().Write(fmt.Sprintf("sys/mounts/%s", backendPath), map[string]interface{}{
		"type": "aws",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, err = testVaultClient.Logical().Delete(fmt.Sprintf("sys/mounts/%s", backendPath))
		if err != nil {
			t.Fatal(err)
		}
	})

	return backendPath
}

func testAccCheckExposedAWSAccessKeyIDExistsAndIsOnlyOne(iamUsername string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState := s.RootModule().Resources["vaultsecure_aws_secret_access_key.this"]
		accessKeyID := resourceState.Primary.Attributes["aws_access_key_id"]

		keys, err := testIAMClient.ListAccessKeys(context.Background(), &iam.ListAccessKeysInput{
			UserName: aws.String(iamUsername),
		})
		if err != nil {
			return err
		}
		if len(keys.AccessKeyMetadata) != 1 {
			return fmt.Errorf("only one access key is expected to exist, found %d", len(keys.AccessKeyMetadata))
		}

		if *keys.AccessKeyMetadata[0].AccessKeyId != accessKeyID {
			return fmt.Errorf("the access key existing for the IAM user (%s) does not match the access key exposed by the resource (%s)", *keys.AccessKeyMetadata[0].AccessKeyId, accessKeyID)
		}

		return nil
	}
}

func testAccCheckExposedAWSAccessKeyIDMatchesVaultConfiguration(enginePath string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState := s.RootModule().Resources["vaultsecure_aws_secret_access_key.this"]
		stateAccessKeyID := resourceState.Primary.Attributes["aws_access_key_id"]

		read, err := testVaultClient.Logical().Read(fmt.Sprintf("%s/config/root", enginePath))
		if err != nil {
			return err
		}
		vaultAccessKeyID := read.Data["access_key"].(string)

		if stateAccessKeyID != vaultAccessKeyID {
			return fmt.Errorf("the access key exposed by the resource (%s) does not match the access key configured in Vault (%s)", stateAccessKeyID, vaultAccessKeyID)
		}

		return nil
	}
}

func testAccCheckExposedAWSAndVaultAccessKeyIDAreEqual(s *terraform.State) error {
	resourceState := s.RootModule().Resources["vaultsecure_aws_secret_access_key.this"]

	awsAccessKeyID := resourceState.Primary.Attributes["aws_access_key_id"]
	vaultAccessKeyID := resourceState.Primary.Attributes["vault_access_key_id"]

	if awsAccessKeyID != vaultAccessKeyID {
		return fmt.Errorf("the aws access key exposed by the resource (%s) is not equal to the exposed vault aws access key (%s)", awsAccessKeyID, vaultAccessKeyID)
	}

	return nil
}

func testAccRotateRoot(enginePath string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, err := testVaultClient.Logical().Write(fmt.Sprintf("%s/config/rotate-root", enginePath),
			map[string]interface{}{})
		return err
	}
}

func testAccResourceAwsSecretAccessKeyType_basic(iamUsername string, enginePath string) string {
	return fmt.Sprintf(`
resource "vaultsecure_aws_secret_access_key" "this" {
  aws_iam_username = "%s"
  vault_engine_path = "%s"
}`, iamUsername, enginePath)
}
