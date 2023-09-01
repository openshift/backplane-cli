package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/openshift/backplane-cli/pkg/awsutil"
	"github.com/openshift/backplane-cli/pkg/utils"
	"github.com/spf13/cobra"
)

const (
	DefaultInitialRoleArn = "arn:aws:iam::922711891673:role/SRE-Support-Role"
)

var assumeArgs struct {
	initialRoleArn string
	output         string
	debugFile      string
	console        bool
}

var StsClientWithProxy = awsutil.StsClientWithProxy
var AssumeRoleWithJWT = awsutil.AssumeRoleWithJWT
var NewStaticCredentialsProvider = credentials.NewStaticCredentialsProvider
var AssumeRoleSequence = awsutil.AssumeRoleSequence

var AssumeCmd = &cobra.Command{
	Use:   "assume [CLUSTERID|EXTERNAL_ID|CLUSTER_NAME|CLUSTER_NAME_SEARCH]",
	Short: "Performs the assume role chaining necessary to generate temporary access to the customer's AWS account",
	Long: `Performs the assume role chaining necessary to generate temporary access to the customer's AWS account

This command is the equivalent of running "aws sts assume-role-with-web-identity --initial-role-arn [role-arn] --web-identity-token [ocm token] --role-session-name [email from OCM token]" behind the scenes,
where the ocm token used is the result of running "ocm token". Then, the command makes a call to the backplane API to get the necessary jump roles for the cluster's account. It then calls the
equivalent of "aws sts assume-role --initial-role-arn [role-arn] --role-session-name [email from OCM token]" repeatedly for each role arn in the chain, using the previous role's credentials to assume the next
role in the chain.

This command will output sts credentials for the target role in the given cluster in formatted JSON. If no "role-arn" is provided, a default role will be used.
`,
	Example: `With default role:
backplane cloud assume e3b2fdc5-d9a7-435e-8870-312689cfb29c -oenv

With given role:
backplane cloud assume e3b2fdc5-d9a7-435e-8870-312689cfb29c --initial-role-arn arn:aws:iam::1234567890:role/read-only -oenv

With a debug file:
backplane cloud assume e3b2fdc5-d9a7-435e-8870-312689cfb29c --debug-file test_arns

As console url:
backplane cloud assume e3b2fdc5-d9a7-435e-8870-312689cfb29c --console`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAssume,
}

func init() {
	flags := AssumeCmd.Flags()
	flags.StringVar(&assumeArgs.initialRoleArn, "initial-role-arn", DefaultInitialRoleArn, "The arn of the role for which to start the role assume process.")
	flags.StringVarP(&assumeArgs.output, "output", "o", "env", "Format the output of the console response.")
	flags.StringVar(&assumeArgs.debugFile, "debug-file", "", "A file containing the list of ARNs to assume in order, not including the initial role ARN. Providing this flag will bypass calls to the backplane API to retrieve the assume role chain. The file should be a plain text file with each ARN on a new line.")
	flags.BoolVar(&assumeArgs.console, "console", false, "Outputs a console url to access the targeted cluster instead of the STS credentials.")
}

type assumeChainResponse struct {
	AssumptionSequence []namedRoleArn `json:"assumptionSequence"`
}

type namedRoleArn struct {
	Name string `json:"name"`
	Arn  string `json:"arn"`
}

func runAssume(_ *cobra.Command, args []string) error {
	if len(args) == 0 && assumeArgs.debugFile == "" {
		return fmt.Errorf("must provide either cluster ID as an argument, or --debug-file as a flag")
	}

	ocmToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return fmt.Errorf("failed to retrieve OCM token: %w", err)
	}

	bpConfig, err := GetBackplaneConfiguration()
	if err != nil {
		return fmt.Errorf("error retrieving backplane configuration: %w", err)
	}

	initialClient, err := StsClientWithProxy(bpConfig.ProxyURL)
	if err != nil {
		return fmt.Errorf("failed to create sts client: %w", err)
	}
	seedCredentials, err := AssumeRoleWithJWT(*ocmToken, assumeArgs.initialRoleArn, initialClient)
	if err != nil {
		return fmt.Errorf("failed to assume role using JWT: %w", err)
	}

	email, err := utils.GetStringFieldFromJWT(*ocmToken, "email")
	if err != nil {
		return fmt.Errorf("unable to extract email from given token: %w", err)
	}

	seedClient := sts.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: NewStaticCredentialsProvider(*seedCredentials.AccessKeyId, *seedCredentials.SecretAccessKey, *seedCredentials.SessionToken),
	})

	var roleAssumeSequence []string
	if assumeArgs.debugFile == "" {
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
		if response.StatusCode != 200 {
			return fmt.Errorf("failed to fetch arn sequence: %v", response.Status)
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

		roleAssumeSequence = make([]string, 0, len(roleChainResponse.AssumptionSequence))
		for _, namedRoleArn := range roleChainResponse.AssumptionSequence {
			roleAssumeSequence = append(roleAssumeSequence, namedRoleArn.Arn)
		}
	} else {
		arnBytes, err := os.ReadFile(assumeArgs.debugFile)
		if err != nil {
			return fmt.Errorf("failed to read file %v: %w", assumeArgs.debugFile, err)
		}

		roleAssumeSequence = append(roleAssumeSequence, strings.Split(string(arnBytes), "\n")...)
	}

	targetCredentials, err := AssumeRoleSequence(email, seedClient, roleAssumeSequence, bpConfig.ProxyURL, awsutil.DefaultSTSClientProviderFunc)
	if err != nil {
		return fmt.Errorf("failed to assume role sequence: %w", err)
	}

	if assumeArgs.console {
		resp, err := awsutil.GetSigninToken(targetCredentials)
		if err != nil {
			return fmt.Errorf("failed to get signin token from AWS: %w", err)
		}

		signInFederationURL, err := awsutil.GetConsoleURL(resp.SigninToken)
		if err != nil {
			return fmt.Errorf("failed to generate console url: %w", err)
		}

		fmt.Printf("The AWS Console URL is:\n%s\n", signInFederationURL.String())
	} else {
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
	}
	return nil
}
