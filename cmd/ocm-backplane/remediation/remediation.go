package remediation

import (
	"fmt"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/remediation"
	"github.com/openshift/backplane-cli/pkg/utils"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
	proxyURI, err := remediation.DoCreateRemediation(backplaneHost, clusterID, *accessToken, remediationName)
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
	err = remediation.DoDeleteRemediation(backplaneHost, clusterID, *accessToken, remediationSA)
	// ======== Render Results ========
	if err != nil {
		return err
	}

	fmt.Printf("Deleted remediation RBAC on cluster %s\n", clusterID)
	return nil
}
