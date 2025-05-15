package login

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/jira"
	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/pagerduty"
	"github.com/openshift/backplane-cli/pkg/utils"
)

// Environment variable that for setting PS1
const (
	EnvPs1                      = "KUBE_PS1_CLUSTER_FUNCTION"
	LoginTypeClusterID          = "cluster-id"
	LoginTypeExistingKubeConfig = "kube-config"
	LoginTypePagerduty          = "pagerduty"
	LoginTypeJira               = "jira"
)

var govcloud bool

var govcloudCmd = &cobra.Command{
	Use:   "govcloud",
	Short: "govcloud",
	Run: func(cmd *cobra.Command, args []string) {
		// Check if the flag was explicitly set
		govcloudFlag, err := cmd.Flags().GetBool("govcloud")
		if err != nil {
			fmt.Println("Error retrieving govcloud flag:", err)
			return
		}

		if cmd.Flags().Changed("govcloud") {
			fmt.Printf("GovCloud mode is set to: %v\n", govcloudFlag)
		} else {
			fmt.Println("GovCloud mode is disabled")
			fmt.Printf("GovCloud mode is set to: %v\n", govcloudFlag)
		}
	},
}

var (
	args struct {
		multiCluster     bool
		kubeConfigPath   string
		pd               string
		defaultNamespace string
		ohss             string
		clusterInfo      bool
		remediation      string
		govcloud         bool
	}

	// loginType derive the login type based on flags and args
	// set default login type as cluster-id
	loginType = LoginTypeClusterID

	globalOpts = &globalflags.GlobalOptions{}

	// LoginCmd represents the login command
	LoginCmd = &cobra.Command{
		Use:   "login <CLUSTERID|EXTERNAL_ID|CLUSTER_NAME|CLUSTER_NAME_SEARCH>",
		Short: "Login to a target cluster",
		Long: `Running login command will send a request to backplane api
		using OCM token. The backplane api will return a proxy url for
		target cluster. The url will be written to kubeconfig, so we can
		run oc command later to operate the target cluster.`,
		Example: " backplane login <id>\n backplane login %test%\n backplane login <external_id>\n backplane login --pd <incident-id>",
		Args: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Lookup("pd").Changed || cmd.Flags().Lookup("ohss").Changed {
				if err := cobra.ExactArgs(0)(cmd, args); err != nil {
					return err
				}
			} else {
				if err := cobra.ExactArgs(1)(cmd, args); err != nil {
					return err
				}
			}
			return nil
		},
		PreRunE:      preLogin,
		RunE:         runLogin,
		SilenceUsage: true,
	}

	//ohss service
	ohssService *jira.OHSSService
)

func init() {
	flags := LoginCmd.Flags()
	// Add global flags
	globalflags.AddGlobalFlags(LoginCmd, globalOpts)

	flags.BoolVarP(
		&args.multiCluster,
		"multi",
		"m",
		false,
		"Enable multi-cluster login.",
	)

	flags.StringVar(
		&args.kubeConfigPath,
		"kube-path",
		"",
		"Save kube configuration in the specific path when login to multi clusters.",
	)

	flags.StringVar(
		&args.pd,
		"pd",
		"",
		"Login using PagerDuty incident id or pagerduty url.",
	)
	flags.StringVarP(
		&args.defaultNamespace,
		"namespace",
		"n",
		"default",
		"The default namespace for a user to execute commands in",
	)
	flags.StringVar(
		&args.ohss,
		"ohss",
		"",
		"Login using JIRA Id",
	)
	flags.BoolVar(
		&args.clusterInfo,
		"cluster-info",
		false, "Print basic cluster information after login",
	)
}

// TODO there is something about the proxy config in relation to overriding with --url
// if i give localhost in the url it still tries to use proxy from .config/backplane/env.json
func runLogin(cmd *cobra.Command, argv []string) (err error) {
	var clusterKey string
	var elevateReason string
	logger.Debugf("Running Login Command ...")
	logger.Debugf("Checking Backplane Version")
	utils.CheckBackplaneVersion(cmd)

	logger.Debugf("Extracting Backplane configuration")
	// Get Backplane configuration
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return err
	}
	logger.Debugf("Backplane Config File Contains: %v \n", bpConfig)

	// login to the cluster based on login type
	logger.Debugf("Extracting Backplane Cluster ID")
	switch loginType {
	case LoginTypePagerduty:
		info, err := getClusterInfoFromPagerduty(bpConfig)
		if err != nil {
			return err
		}
		clusterKey = info.ClusterID
		elevateReason = info.WebURL
	case LoginTypeJira:
		ohssIssue, err := getClusterInfoFromJira()
		if err != nil {
			return err
		}
		if ohssIssue.ClusterID == "" {
			return fmt.Errorf("clusterID cannot be detected for JIRA issue:%s", args.ohss)
		}
		clusterKey = ohssIssue.ClusterID
		elevateReason = ohssIssue.WebURL
	case LoginTypeClusterID:
		logger.Debugf("Cluster Key is given in argument")
		clusterKey = argv[0]
	case LoginTypeExistingKubeConfig:
		clusterKey, err = getClusterIDFromExistingKubeConfig()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("login type cannot be detected")
	}

	logger.Debugf("Backplane Cluster Key is: %v \n", clusterKey)

	// Set proxy url to http client
	proxyURL := globalOpts.ProxyURL

	if !(bpConfig.Govcloud) {
		logger.Debugln("Setting Proxy URL from global options")

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
	} else {
		logger.Debugln("govcloud identified, no proxy to use")
	}

	logger.Debugln("Extracting target cluster ID and name")
	clusterID, clusterName, err := ocm.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}

	logger.WithFields(logger.Fields{
		"ID":   clusterID,
		"Name": clusterName}).Infoln("Target cluster")

	if args.clusterInfo {
		if err := login.PrintClusterInfo(clusterID); err != nil {
			return fmt.Errorf("failed to print cluster info: %v", err)
		}
	}

	if bpConfig.DisplayClusterInfo {
		if err := login.PrintClusterInfo(clusterID); err != nil {
			return fmt.Errorf("failed to print cluster info: %v", err)
		}
	}

	if globalOpts.Manager {
		logger.WithField("Cluster ID", clusterID).Debugln("Finding managing cluster")
		var isHostedControlPlane bool
		targetClusterID := clusterID

		clusterID, clusterName, isHostedControlPlane, err = ocm.DefaultOCMInterface.GetManagingCluster(clusterID)
		if err != nil {
			return err
		}

		logger.Debugf("Managing clusterID is : %v \n", clusterID)
		logger.Debugf("Managing cluster name is : %v \n", clusterName)

		logger.WithFields(logger.Fields{
			"ID":   clusterID,
			"Name": clusterName}).Infoln("Management cluster")

		logger.Debugln("Finding K8s namespaces")
		// Print the related namespace if login to manager cluster
		var namespaces map[string]string
		namespaces, err = listNamespaces(targetClusterID, isHostedControlPlane)
		if err != nil {
			return err
		}
		fmt.Println("Execute the following command to export the list of associated namespaces for your given cluster")
		for key, ns := range namespaces {
			fmt.Printf("\texport %s=%s\n", key, ns)
		}
	}

	if globalOpts.Service {
		logger.WithField("Cluster ID", clusterID).Debugln("Finding service cluster")
		targetClusterID := clusterID
		_, managingClusterName, isHostedControlPlane, err := ocm.DefaultOCMInterface.GetManagingCluster(clusterID)
		if err != nil {
			return err
		}
		clusterID, clusterName, err = ocm.DefaultOCMInterface.GetServiceCluster(clusterID)
		if err != nil {
			return err
		}
		logger.Debugf("Service clusterID is : %v \n", clusterID)
		logger.Debugf("Service cluster name is : %v \n", clusterName)

		logger.WithFields(logger.Fields{
			"ID":   clusterID,
			"Name": clusterName}).Infoln("Service cluster")

		if !isHostedControlPlane {
			return fmt.Errorf("manifestworks are only available for hosted control plane clusters")
		}
		listManifestWork := fmt.Sprintf("oc get manifestworks -n %s -l api.openshift.com/id=%s", managingClusterName, targetClusterID)

		fmt.Println("A list of associated manifestwork for your given cluster can be found using:")
		fmt.Println("\t", listManifestWork)
		fmt.Println("\nThe MC Namespace for the cluster can be exported using:")
		fmt.Printf("\texport MC_NAME=%s\n", managingClusterName)
	}

	logger.Debugln("Validating K8s Config Path")
	// validate kubeconfig save path when login into multi clusters
	if args.kubeConfigPath != "" {
		if !args.multiCluster {
			return fmt.Errorf("can't save the kube config into a specific location if multi-cluster is not enabled. Please specify --multi flag")
		}
		if _, err := os.Stat(args.kubeConfigPath); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("the save path for the kubeconfig does not exist")
		}
	}

	logger.Debugln("Extracting backplane URL")
	// Get Backplane URL
	bpURL := globalOpts.BackplaneURL
	if bpURL == "" {
		bpURL = bpConfig.URL
	}

	if bpURL == "" {
		return errors.New("empty backplane url - check your backplane-cli configuration")
	}

	logger.Debugf("Using backplane URL: %s\n", bpURL)
	backplaneResolution, err := getBackplaneCNAME(bpURL)
	if err != nil {
		logger.Warn(err.Error())
	} else {
		logger.Debugf("Backplane URL resolves to %s \n", backplaneResolution)
		logger.Debugf("To know the associated Backplane instance please refer to the Backplane Source wiki")
	}

	// Get ocm access token
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}

	logger.Debugln("Check for Cluster Hibernation")
	// Not great if there's an error checking if the cluster is hibernating, but ignore it for now and continue
	if isHibernating, _ := ocm.DefaultOCMInterface.IsClusterHibernating(clusterID); isHibernating {
		// If it is hibernating, don't try to connect as it will fail
		return fmt.Errorf("cluster %s is hibernating, login failed", clusterKey)
	}

	logger.WithFields(logger.Fields{
		"bpURL":     bpURL,
		"clusterID": clusterID,
	}).Debugln("Query backplane-api for proxy url of our target cluster")
	// Query backplane-api for proxy url
	bpAPIClusterURL, err := doLogin(bpURL, clusterID, *accessToken)
	if err != nil {
		// Declare helperMsg
		helperMsg := "\n\033[1mNOTE: To troubleshoot the connectivity issues, please run `ocm-backplane health-check`\033[0m\n\n"

		// Check API connection with configured proxy
		if connErr := bpConfig.CheckAPIConnection(); connErr != nil {
			return fmt.Errorf("cannot connect to Backplane API URL: %v.\n%s", connErr, helperMsg)
		}

		return fmt.Errorf("login Attempt Failed: %v.\n%s", err, helperMsg)
	}

	logger.WithField("URL", bpAPIClusterURL).Debugln("Proxy")

	logger.Debugln("Generating a new K8s cluster config file")
	cf := genericclioptions.NewConfigFlags(true)
	rc, err := cf.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}
	logger.Debugf("API Config Generated %+v \n", rc)

	logger.Debugln("Check for PS1 ENV varible")
	// Check PS1 env is set or not
	EnvPs1, ok := os.LookupEnv(EnvPs1)
	if !ok {
		logger.Warn("Env KUBE_PS1_CLUSTER_FUNCTION is not detected. It is recommended to set PS1 to learn which cluster you are operating on, refer https://github.com/openshift/backplane-cli/blob/main/docs/PS1-setup.md", EnvPs1)
	}

	// Add a new cluster & context & user
	logger.Debugln("Writing OCM configuration ")

	targetCluster := api.NewCluster()
	targetUser := api.NewAuthInfo()
	targetContext := api.NewContext()

	targetCluster.Server = bpAPIClusterURL

	// Add proxy URL to target cluster

	if proxyURL != "" {
		targetCluster.ProxyURL = proxyURL
	}

	targetUserNickName := utils.GetUsernameFromJWT(*accessToken)

	targetUser.Token = *accessToken

	targetContext.AuthInfo = targetUserNickName
	targetContext.Cluster = clusterName

	if isValidKubernetesNamespace(args.defaultNamespace) {
		logger.Debugln("Validating argument passed as namespace")
		targetContext.Namespace = args.defaultNamespace
	} else {
		return fmt.Errorf("%v is not a valid namespace", args.defaultNamespace)
	}

	targetContextNickName := utils.GetContextNickname(targetContext.Namespace, targetContext.Cluster, targetContext.AuthInfo)

	// Put user, cluster, context into rawconfig
	rc.Clusters[targetContext.Cluster] = targetCluster
	rc.AuthInfos[targetUserNickName] = targetUser
	rc.Contexts[targetContextNickName] = targetContext
	rc.CurrentContext = targetContextNickName

	// Add elevate reason to kubeconfig context
	if elevateReason != "" {
		elevationReasons, err := login.SaveElevateContextReasons(rc, elevateReason)
		if err != nil {
			return err
		}
		logger.Infof("save elevate reason: %s\n", elevationReasons)
	}

	logger.Debugln("Saving new API config")
	// Save the config
	if err = login.SaveKubeConfig(clusterID, rc, args.multiCluster, args.kubeConfigPath); err != nil {
		return err
	}

	// We return without error from here because the user is still successfully logged in
	// however, we just cannot check for other logged in users for some reason. Therefore,
	// we should still return that this command exited successfully, as checking for
	// the other logged in users should not give an SRE the idea they cannot use the credentials.

	logger.Debugln("Checking for other backplane sessions...")
	cfg, _ := BuildRestConfig(bpAPIClusterURL, accessToken, proxyURL)
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.WithField("error", err).Debug("Unable to build kube client from rest config to check for other Backplane sessions")
		logger.Warn("Can not check for other Backplane sessions. You should still be logged in")
		return nil
	}

	sessions, err := login.FindOtherSessions(clientset, cfg, targetUserNickName)
	if err != nil {
		logger.Debug("error", err)
		logger.Warn("Could not check for other Backplane sessions. You should still be logged in")
		return nil
	}

	login.PrintSessions(os.Stdout, sessions)

	return nil
}

// BuildRestConfig takes a host, token and optional proxy URL and generates a rest config
func BuildRestConfig(host string, token *string, proxyURL string) (*rest.Config, error) {
	cfg := &rest.Config{
		Host:        host,
		BearerToken: *token,
	}
	if proxyURL != "" {
		cfg.Proxy = func(*http.Request) (*url.URL, error) {
			return url.Parse(proxyURL)
		}
	}
	return cfg, nil
}

// GetRestConfig returns a client-go *rest.Config which can be used to programmatically interact with the
// Kubernetes API of a provided clusterID
func GetRestConfig(bp config.BackplaneConfiguration, clusterID string) (*rest.Config, error) {
	cluster, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return nil, err
	}

	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return nil, err
	}

	bpAPIClusterURL, err := doLogin(bp.URL, clusterID, *accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to backplane login to cluster %s: %v", cluster.Name(), err)
	}

	proxyURL := ""
	if bp.ProxyURL != nil {
		proxyURL = *bp.ProxyURL
	}

	return BuildRestConfig(bpAPIClusterURL, accessToken, proxyURL)
}

// GetRestConfig returns a client-go *rest.Config which can be used to programmatically interact with the
// Kubernetes API of a provided clusterID
func GetRestConfigWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string) (*rest.Config, error) {
	cluster, err := ocm.DefaultOCMInterface.GetClusterInfoByIDWithConn(ocmConnection, clusterID)
	if err != nil {
		return nil, err
	}

	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return nil, err
	}

	bpAPIClusterURL, err := doLogin(bp.URL, clusterID, *accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to backplane login to cluster %s: %v", cluster.Name(), err)
	}

	proxyURL := ""
	if bp.ProxyURL != nil {
		proxyURL = *bp.ProxyURL
	}

	return BuildRestConfig(bpAPIClusterURL, accessToken, proxyURL)
}

// GetRestConfigAsUser returns a client-go *rest.Config like GetRestConfig, but supports configuring an
// impersonation username. Commonly, this is "backplane-cluster-admin"
// best practice would be to add at least one elevationReason in order to justity the impersonation
func GetRestConfigAsUser(bp config.BackplaneConfiguration, clusterID, username string, elevationReasons ...string) (*rest.Config, error) {
	cfg, err := GetRestConfig(bp, clusterID)
	if err != nil {
		return nil, err
	}

	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: username,
	}

	if len(elevationReasons) > 0 {
		cfg.Impersonate.Extra = map[string][]string{"reason": elevationReasons}
	}

	return cfg, nil
}

// doLogin returns the proxy url for the target cluster.
func doLogin(api, clusterID, accessToken string) (string, error) {

	client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(api, accessToken)

	if err != nil {
		return "", fmt.Errorf("unable to create backplane api client")
	}

	resp, err := client.LoginCluster(context.TODO(), clusterID)
	// Print the whole response if we can't parse it. Eg. 5xx error from http server.
	if err != nil {
		// trying to determine the error
		errBody := err.Error()
		if strings.Contains(errBody, "dial tcp") && strings.Contains(errBody, "i/o timeout") {
			return "", fmt.Errorf("unable to connect to backplane api")
		}

		return "", err
	}

	err = backplaneapi.CheckResponseDeprecation(resp)
	if errors.Is(err, backplaneapi.ErrDeprecation) {
		logger.Warnf("The server indicated that backplane-cli version %s is deprecated. Please update as soon as possible.", info.DefaultInfoService.GetVersion())
	}

	if resp.StatusCode != http.StatusOK {
		return "", utils.TryPrintAPIError(resp, false)
	}

	loginResp, err := BackplaneApi.ParseLoginClusterResponse(resp)

	if err != nil {
		return "", fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	return api + *loginResp.JSON200.ProxyUri, nil
}

func listNamespaces(clusterID string, isHostedControlPlane bool) (map[string]string, error) {

	env, err := ocm.DefaultOCMInterface.GetOCMEnvironment()
	if err != nil {
		return map[string]string{}, err
	}
	envName := env.Name()

	if envName == "integration" {
		envName = "int"
	}

	clusterInfo, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return map[string]string{}, err
	}

	klusterletPrefix := "klusterlet-"
	hivePrefix := fmt.Sprintf("uhc-%s-", envName)
	hcpPrefix := fmt.Sprintf("ocm-%s-", envName)

	var nsList map[string]string
	if isHostedControlPlane {
		nsList = map[string]string{
			"KLUSTERLET_NS": klusterletPrefix + clusterID,
			"HC_NAMESPACE":  hcpPrefix + clusterID,
			"HCP_NAMESPACE": hcpPrefix + clusterID + "-" + clusterInfo.DomainPrefix(),
		}
	} else {
		nsList = map[string]string{
			"HIVE_NS": hivePrefix + clusterID,
		}
	}

	return nsList, nil
}

// getBackplaneCNAME returns the DNS/CNAME resolution of the ocm backplane URL
func getBackplaneCNAME(backplaneURL string) (string, error) {
	backplaneDomain, err := url.Parse(backplaneURL)
	if err != nil {
		return "", fmt.Errorf("unable to extract the fqdn from the %s", backplaneURL)
	}
	fqdn := backplaneDomain.Hostname()
	resolution, err := net.LookupCNAME(fqdn)
	if err != nil {
		return "", fmt.Errorf("unable to resolve the %s", fqdn)
	}
	return resolution, nil
}

// isValidNamespace validates the input string against  Kubernetes namespace rules.( RFC 1123 )
func isValidKubernetesNamespace(namespace string) bool {
	// RFC 1123 compliant regex pattern)
	pattern := `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	return regexp.MustCompile(pattern).MatchString(namespace)
}

// preLogin will execute before the command
func preLogin(cmd *cobra.Command, argv []string) (err error) {

	switch len(argv) {
	case 1:
		loginType = LoginTypeClusterID

	case 0:
		if args.pd == "" && args.ohss == "" {
			loginType = LoginTypeExistingKubeConfig
		} else if args.ohss != "" {
			loginType = LoginTypeJira
		} else if args.pd != "" {
			loginType = LoginTypePagerduty
		}
	}

	return nil
}

// getClusterInfoFromPagerduty returns a pagerduty.Alert from Pagerduty incident,
// which contains alert info including the cluster id.
func getClusterInfoFromPagerduty(bpConfig config.BackplaneConfiguration) (alert pagerduty.Alert, err error) {
	if bpConfig.PagerDutyAPIKey == "" {
		return alert, fmt.Errorf("please make sure the PD API Key is configured correctly in the config file")
	}
	pdClient, err := pagerduty.NewWithToken(bpConfig.PagerDutyAPIKey)
	if err != nil {
		return alert, fmt.Errorf("could not initialize the client: %w", err)
	}
	if strings.Contains(args.pd, "/incidents/") {
		incidentID := args.pd[strings.LastIndex(args.pd, "/")+1:]
		alert, err = pdClient.GetClusterInfoFromIncident(incidentID)
		if err != nil {
			return alert, err
		}
	} else {
		alert, err = pdClient.GetClusterInfoFromIncident(args.pd)
		if err != nil {
			return alert, err
		}
	}
	return alert, nil
}

// getClusterInfoFromJira returns a cluster info OHSS card
func getClusterInfoFromJira() (ohss jira.OHSSIssue, err error) {
	if ohssService == nil {
		ohssService = jira.NewOHSSService(jira.DefaultIssueService)
	}

	ohss, err = ohssService.GetIssue(args.ohss)
	if err != nil {
		return ohss, err
	}

	return ohss, nil
}

// getClusterIDFromExistingKubeConfig returns clusterId from kubeconfig
func getClusterIDFromExistingKubeConfig() (string, error) {
	var clusterKey string
	logger.Debugf("Finding Clustrer Key from current cluster")
	clusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
	if err != nil {
		return "", err
	}
	clusterKey = clusterInfo.ClusterID
	logger.Debugf("Backplane Cluster Infromation data extracted: %+v\n", clusterInfo)
	return clusterKey, nil
}
