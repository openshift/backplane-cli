package remediation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

var (
	globalOpts = &globalflags.GlobalOptions{}
)

func NewRemediationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "remediation",
		Short:        "Create and delete remediation resources",
		SilenceUsage: true,
	}

	globalflags.AddGlobalFlags(cmd, globalOpts)

	// cluster-id Flag
	cmd.PersistentFlags().StringP("cluster-id", "c", "", "Cluster ID could be cluster name, id or external-id")

	// raw Flag
	cmd.PersistentFlags().Bool("raw", false, "Prints the raw response returned by the backplane API")
	cmd.AddCommand(newCreateRemediationCmd())
	cmd.AddCommand(newDeleteRemediationCmd())
	return cmd
}

func newCreateRemediationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create",
		Short:        "create remediation SA and RBAC",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ======== Parsing Flags ========
			// Cluster ID flag
			clusterKey, err := cmd.Flags().GetString("cluster-id")
			if err != nil {
				return err
			}

			// URL flag
			urlFlag, err := cmd.Flags().GetString("url")
			if err != nil {
				return err
			}

			return runCreateRemediation(args, clusterKey, urlFlag)
		},
	}
	return cmd
}

func newDeleteRemediationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete",
		Short:        "Delete remediation SA and RBAC",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ======== Parsing Flags ========
			// Cluster ID flag
			clusterKey, err := cmd.Flags().GetString("cluster-id")
			if err != nil {
				return err
			}

			// URL flag
			urlFlag, err := cmd.Flags().GetString("url")
			if err != nil {
				return err
			}

			return runDeleteRemediation(args, clusterKey, urlFlag)
		},
	}
	return cmd
}

func runCreateRemediation(args []string, clusterKey string, urlFlag string) error {
	// ======== Initialize backplaneURL ========
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return err
	}

	bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(clusterKey)
	if err != nil {
		return err
	}

	backplaneHost := bpConfig.URL

	clusterID := bpCluster.ClusterID

	if urlFlag != "" {
		backplaneHost = urlFlag
	}

	// ======== Parsing Args ========
	if len(args) < 1 {
		return fmt.Errorf("missing remediation name as an argument")
	}
	remediationName := args[0]

	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}
	proxyURI, err := doCreateRemediation(backplaneHost, clusterID, *accessToken, remediationName)
	// ======== Render Results ========
	if err != nil {
		return err
	}

	logger.Infof("Created remediation RBAC and serviceaccount\nuri: %s", proxyURI)
	// TODO needs code to create local kubeconfig factorized from login command
	// So that we are "logged in"
	// CAD uses the programmatic endpoint and resulting kubeconfig directly

	// Add a new cluster & context & user
	logger.Debugln("Writing OCM configuration ")

	targetCluster := api.NewCluster()
	targetUser := api.NewAuthInfo()
	targetContext := api.NewContext()

	targetCluster.Server = proxyURI

	// Add proxy URL to target cluster
	proxyURL := globalOpts.ProxyURL
	if proxyURL != "" {
		err = backplaneapi.DefaultClientUtils.SetClientProxyURL(proxyURL)

		if err != nil {
			return err
		}
		logger.Debugf("Using backplane Proxy URL: %s\n", proxyURL)
	}

	if bpConfig.ProxyURL != nil {
		proxyURL = *bpConfig.ProxyURL
		logger.Debugln("backplane configuration file also contains a proxy url, using that one instead")
		logger.Debugf("New backplane Proxy URL: %s\n", proxyURL)
	}

	logger.Debugln("Extracting target cluster ID and name")
	clusterID, clusterName, err := ocm.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}
	if proxyURL != "" {
		targetCluster.ProxyURL = proxyURL
	}

	targetUserNickName := utils.GetUsernameFromJWT(*accessToken)

	targetUser.Token = *accessToken

	targetContext.AuthInfo = targetUserNickName
	targetContext.Cluster = clusterName

	targetContext.Namespace = "default"

	targetContextNickName := utils.GetContextNickname(targetContext.Namespace, targetContext.Cluster, targetContext.AuthInfo)

	cf := genericclioptions.NewConfigFlags(true)
	rc, err := cf.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}
	// Put user, cluster, context into rawconfig
	rc.Clusters[targetContext.Cluster] = targetCluster
	rc.AuthInfos[targetUserNickName] = targetUser
	rc.Contexts[targetContextNickName] = targetContext
	rc.CurrentContext = targetContextNickName

	logger.Debugln("Saving new API config")
	// Save the config
	if err = login.SaveKubeConfig(clusterID, rc, false, ""); err != nil {
		return err
	}

	return nil
}

func runDeleteRemediation(args []string, clusterKey string, urlFlag string) error {
	// ======== Initialize backplaneURL ========
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return err
	}

	bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(clusterKey)
	if err != nil {
		return err
	}

	backplaneHost := bpConfig.URL

	clusterID := bpCluster.ClusterID

	if urlFlag != "" {
		backplaneHost = urlFlag
	}

	// ======== Parsing Args ========
	if len(args) < 1 {
		return fmt.Errorf("missing remediations service account name as an argument")
	}
	remediationSA := args[0]

	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}
	// ======== Call Endpoint ========
	err = doDeleteRemediation(backplaneHost, clusterID, *accessToken, remediationSA)
	// ======== Render Results ========
	if err != nil {
		return err
	}

	fmt.Printf("Deleted remediation RBAC on cluster %s\n", clusterID)
	return nil
}

// TODO are we missusing the backplane api package? We have generated functions like backplaneapi.CreateRemediationWithResponse which already reads the body. All the utils functions do read the body again. Failing my calls here.
func doCreateRemediation(api string, clusterID string, accessToken string, remediationName string) (proxyURI string, err error) {
	client, err := backplaneapi.DefaultClientUtils.MakeBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return "", fmt.Errorf("unable to create backplane api client")
	}

	logger.Debug("Sending request...")
	resp, err := client.CreateRemediationWithResponse(context.TODO(), clusterID, &BackplaneApi.CreateRemediationParams{Remediation: remediationName})
	if err != nil {
		logger.Debug("unexpected...")
		return "", err
	}

	// TODO figure out the error handling here
	if resp.StatusCode() != http.StatusOK {
		// logger.Debugf("Unmarshal error resp body: %s", resp.Body)
		var dest BackplaneApi.Error
		if err := json.Unmarshal(resp.Body, &dest); err != nil {
			// Avoid squashing the HTTP response info with Unmarshal err...
			logger.Debugf("Unmarshaled %s", *dest.Message)

			bodyStr := strings.ReplaceAll(string(resp.Body[:]), "\n", " ")
			err := fmt.Errorf("code:'%d'; failed to unmarshal response:'%s'; %w", resp.StatusCode(), bodyStr, err)
			return "", err
		}
		return "", errors.New(*dest.Message)

	}
	return api + *resp.JSON200.ProxyUri, nil
}

// CreateRemediationWithConn can be used to programtically interact with backplaneapi
func CreateRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, remediationName string) (config *rest.Config, serviceAccountName string, err error) {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return nil, "", err
	}

	bpAPIClusterURL, err := doCreateRemediation(bp.URL, clusterID, *accessToken, remediationName)
	if err != nil {
		return nil, "", err
	}

	cfg := &rest.Config{
		Host:        bpAPIClusterURL,
		BearerToken: *accessToken,
	}

	if bp.ProxyURL != nil {
		cfg.Proxy = func(r *http.Request) (*url.URL, error) {
			return url.Parse(*bp.ProxyURL)
		}
	}
	return cfg, "", nil
}

func doDeleteRemediation(api string, clusterID string, accessToken string, remediation string) error {
	client, err := backplaneapi.DefaultClientUtils.MakeBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return fmt.Errorf("unable to create backplane api client")
	}

	resp, err := client.DeleteRemediationWithResponse(context.TODO(), clusterID, &BackplaneApi.DeleteRemediationParams{Remediation: &remediation})
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		// logger.Debugf("Unmarshal error resp body: %s", resp.Body)
		var dest BackplaneApi.Error
		if err := json.Unmarshal(resp.Body, &dest); err != nil {
			// Avoid squashing the HTTP response info with Unmarshal err...
			logger.Debugf("Unmarshaled %s", *dest.Message)

			bodyStr := strings.ReplaceAll(string(resp.Body[:]), "\n", " ")
			err := fmt.Errorf("code:'%d'; failed to unmarshal response:'%s'; %w", resp.StatusCode(), bodyStr, err)
			return err
		}
		return errors.New(*dest.Message)
	}

	return nil
}

// DeleteRemediationWithConn can be used to programtically interact with backplaneapi
func DeleteRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, remediationSA string) error {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return err
	}

	return doDeleteRemediation(bp.URL, clusterID, *accessToken, remediationSA)
}
