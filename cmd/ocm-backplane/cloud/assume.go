package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"

	"github.com/openshift/backplane-cli/pkg/awsutil"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	DefaultRoleArn = "arn:aws:iam::922711891673:role/SRE-Support-Role"
)

var assumeArgs struct {
	roleArn string
	output  string
}

var AssumeCmd = &cobra.Command{
	Use:   "assume [CLUSTERID|EXTERNAL_ID|CLUSTER_NAME|CLUSTER_NAME_SEARCH]",
	Short: "Performs the assume role chaining necessary to generate temporary access to the customer's AWS account",
	Long: `Performs the assume role chaining necessary to generate temporary access to the customer's AWS account

This command is the equivalent of running "aws sts assume-role-with-web-identity --role-arn [role-arn] --web-identity-token [ocm token] --role-session-name [email from OCM token]" behind the scenes,
where the ocm token used is the result of running "ocm token". Then, the command makes a call to the backplane API to get the necessary jump roles for the cluster's account. It then calls the
equivalent of "aws sts assume-role --role-arn [role-arn] --role-session-name [email from OCM token]" repeatedly for each role arn in the chain, using the previous role's credentials to assume the next
role in the chain.

This command will output sts credentials for the target role in the given cluster in formatted JSON. If no "role-arn" is provided, a default role will be used.
`,
	Example: `With default role:
backplane cloud assume e3b2fdc5-d9a7-435e-8870-312689cfb29c -oenv

With given role:
backplane cloud assume e3b2fdc5-d9a7-435e-8870-312689cfb29c --role-arn arn:aws:iam::1234567890:role/read-only -oenv`,
	Args: cobra.ExactArgs(1),
	RunE: runAssume,
}

func init() {
	flags := AssumeCmd.Flags()
	flags.StringVar(&assumeArgs.roleArn, "role-arn", DefaultRoleArn, "The arn of the role for which to start the role assume process.")
	flags.StringVarP(&assumeArgs.output, "output", "o", "env", "Format the output of the console response.")
}

type assumeChainResponse struct {
	AssumptionSequence []namedRoleArn `json:"assumption_sequence"`
}

type namedRoleArn struct {
	Name string `json:"name"`
	Arn  string `json:"arn"`
}

func runAssume(_ *cobra.Command, args []string) error {
	ocmToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return fmt.Errorf("failed to retrieve OCM token: %w", err)
	}

	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return fmt.Errorf("error retrieving backplane configuration: %w", err)
	}

	initialClient, err := awsutil.StsClientWithProxy(bpConfig.ProxyURL)
	if err != nil {
		return fmt.Errorf("failed to create sts client: %w", err)
	}
	seedCredentials, err := awsutil.AssumeRoleWithJWT(*ocmToken, assumeArgs.roleArn, initialClient)
	if err != nil {
		return fmt.Errorf("failed to assume role using JWT: %w", err)
	}

	clusterID, _, err := utils.DefaultOCMInterface.GetTargetCluster(args[0])
	if err != nil {
		return fmt.Errorf("failed to get target cluster: %w", err)
	}

	backplaneClient, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(bpConfig.URL, *ocmToken)
	if err != nil {
		return fmt.Errorf("failed to create backplane client with access token: %w", err)
	}

	response, err := backplaneClient.GetAssumeRoleSequence(context.TODO(), clusterID)
	if err != nil {
		return fmt.Errorf("failed to fetch arn sequence: %w", err)
	}

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read backplane API response body: %w", err)
	}

	roleChainResponse := &assumeChainResponse{}
	err = json.Unmarshal(bytes, roleChainResponse)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	roleAssumeSequence := make([]string, 0, len(roleChainResponse.AssumptionSequence))
	for _, namedRoleArn := range roleChainResponse.AssumptionSequence {
		roleAssumeSequence = append(roleAssumeSequence, namedRoleArn.Arn)
	}

	email, err := utils.GetStringFieldFromJWT(*ocmToken, "email")
	if err != nil {
		return fmt.Errorf("unable to extract email from given token: %w", err)
	}

	seedClient := sts.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(*seedCredentials.AccessKeyId, *seedCredentials.SecretAccessKey, *seedCredentials.SessionToken),
	})
	targetCredentials, err := awsutil.AssumeRoleSequence(email, seedClient, roleAssumeSequence, bpConfig.ProxyURL, awsutil.DefaultSTSClientProviderFunc)
	if err != nil {
		return fmt.Errorf("failed to assume role sequence: %w", err)
	}

	credsResponse := awsutil.AWSCredentialsResponse{
		AccessKeyID:     *targetCredentials.AccessKeyId,
		SecretAccessKey: *targetCredentials.SecretAccessKey,
		SessionToken:    *targetCredentials.SessionToken,
		Expiration:      targetCredentials.Expiration.String(),
	}
	formattedResult, err := credsResponse.RenderOutput(assumeArgs.output)
	if err != nil {
		return fmt.Errorf("failed to format output correctly: %w", err)
	}

	fmt.Println(formattedResult)
	return nil
}
