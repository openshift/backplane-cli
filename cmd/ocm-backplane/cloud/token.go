package cloud

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/openshift/backplane-cli/pkg/awsutil"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var tokenArgs struct {
	roleArn string
	output  string
}

var TokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Generates a session token for the given role ARN",
	Long: `Generates a session token for the given role ARN.

This command is the equivalent of running "aws sts assume-role-with-web-identity --role-arn [role-arn] --web-identity-token [ocm token] --role-session-name [email from OCM token]" behind the scenes,
where the ocm token used is the result of running "ocm token".

This command will output the "Credentials" property of that call in formatted JSON.`,
	Example: "backplane cloud token --role-arn arn:aws:iam::1234567890:role/read-only -oenv",
	Args:    cobra.NoArgs,
	RunE:    runToken,
}

func init() {
	flags := TokenCmd.Flags()
	flags.StringVar(&tokenArgs.roleArn, "role-arn", "", "The arn of the role for which to get credentials.")
	flags.StringVarP(&tokenArgs.output, "output", "o", "env", "Format the output of the console response.")
}

func runToken(*cobra.Command, []string) error {
	ocmToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return fmt.Errorf("failed to retrieve OCM token: %w", err)
	}

	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return fmt.Errorf("error retrieving backplane configuration: %w", err)
	}
	svc, err := awsutil.StsClientWithProxy(bpConfig.ProxyURL)
	if err != nil {
		return fmt.Errorf("error creating STS client: %w", err)
	}

	result, err := awsutil.AssumeRoleWithJWT(*ocmToken, tokenArgs.roleArn, svc)
	if err != nil {
		return fmt.Errorf("failed to assume role with JWT: %w", err)
	}

	credsResponse := awsutil.AWSCredentialsResponse{
		AccessKeyID:     *result.AccessKeyId,
		SecretAccessKey: *result.SecretAccessKey,
		SessionToken:    *result.SessionToken,
		Expiration:      result.Expiration.String(),
	}

	formattedResult, err := credsResponse.RenderOutput(tokenArgs.output)
	if err != nil {
		return fmt.Errorf("failed to format output correctly: %w", err)
	}

	fmt.Println(formattedResult)
	return nil
}
