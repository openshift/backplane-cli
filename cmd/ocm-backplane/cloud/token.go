package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/openshift/backplane-cli/pkg/utils"
	"github.com/spf13/cobra"
)

var tokenArgs struct {
	roleArn string
}

var TokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Generates a session token for the given role ARN",
	Long: `Generates a session token for the given role ARN.

This command is the equivalent of running "aws sts assume-role-with-web-identity --role-arn [role-arn] --web-identity-token [ocm token] --role-session-name [email from OCM token]" behind the scenes,
where the ocm token used is the result of running "ocm token".

This command will output the "Credentials" property of that call in formatted JSON.`,
	Example: "backplane cloud token --role-arn arn:aws:iam::1234567890:role/read-only",
	Args:    cobra.NoArgs,
	RunE:    runToken,
}

func init() {
	flags := TokenCmd.Flags()
	flags.StringVar(&tokenArgs.roleArn, "role-arn", "", "")
}

func runToken(*cobra.Command, []string) error {
	ocmToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return fmt.Errorf("failed to retrieve OCM token: %w", err)
	}

	svc, err := MakeStsService()
	if err != nil {
		return fmt.Errorf("unable to create aws session: %w", err)
	}

	result, err := GetStsCredentials(*ocmToken, tokenArgs.roleArn, svc)
	if err != nil {
		return err
	}

	marshalledResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("error formatting result: %w", err)
	}

	fmt.Println(string(marshalledResult))
	return nil
}

func MakeStsService() (*sts.Client, error) {
	// IAM is global, but this config needs a region. Give it any valid region.
	return sts.NewFromConfig(aws.Config{Region: "us-east-1"}), nil
}

type STSRoleAssumer interface {
	AssumeRoleWithWebIdentity(ctx context.Context, params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleWithWebIdentityOutput, error)
}

func GetStsCredentials(ocmToken string, roleArn string, svc STSRoleAssumer) (*types.Credentials, error) {
	email, err := utils.GetFieldFromJWT(ocmToken, "email")
	if err != nil {
		return nil, fmt.Errorf("unable to extract email from given token: %w", err)
	}
	input := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(roleArn),
		RoleSessionName:  aws.String(email),
		WebIdentityToken: aws.String(ocmToken),
	}

	result, err := svc.AssumeRoleWithWebIdentity(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("unable to assume the given role with the token provided: %w", err)
	}

	return result.Credentials, nil
}
