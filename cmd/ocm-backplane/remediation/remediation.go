package remediation

import (
	"fmt"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

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
		Use:   "create REMEDIATION_NAME",
		Short: "Instantiate a new remediation instance",
		Long:  "Instantiate a new remediation instance from the given remediation name - also create the SA & the RBAC on the target cluster for the new remediation instance",
		Args:  cobra.ExactArgs(1),
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
		Use:   "delete REMEDIATION_INSTANCE_ID",
		Short: "Delete an existing remediation instance",
		Long:  "Delete the remediation instance referenced by the given id - also delete the SA & the RBAC linked to the remediation instance on the target cluster",
		Args:  cobra.ExactArgs(1),
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

	bpURL := bpConfig.URL

	clusterID := bpCluster.ClusterID

	if urlFlag != "" {
		bpURL = urlFlag
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
	proxyURI, remediationInstanceID, err := remediation.DoCreateRemediation(bpURL, clusterID, *accessToken, &BackplaneApi.CreateRemediationParams{RemediationName: remediationName})
	// ======== Render Results ========
	if err != nil {
		return err
	}

	defer func() {
		fmt.Printf("Remediation instance id: %s\n", remediationInstanceID)
		fmt.Println("Use this id when deleting the remediation instance with 'ocm-backplane remediation delete'")
	}()

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

	fmt.Printf("Created remediation RBAC. You are logged in as remediation: %s\n", remediationName)

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

	bpURL := bpConfig.URL

	clusterID := bpCluster.ClusterID

	if urlFlag != "" {
		bpURL = urlFlag
	}

	// ======== Parsing Args ========
	if len(args) < 1 {
		return fmt.Errorf("missing remediations service account name as an argument")
	}
	remediationInstanceID := args[0]

	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}

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
	// ======== Call Endpoint ========
	err = remediation.DoDeleteRemediation(bpURL, clusterID, *accessToken, remediationInstanceID)
	// ======== Render Results ========
	if err != nil {
		return err
	}

	fmt.Printf("Deleted remediation RBAC on cluster %s\n", clusterID)
	return nil
}
