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

	"github.com/golang-jwt/jwt/v4"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/pagerduty"
	"github.com/openshift/backplane-cli/pkg/utils"
)

// Environment variable that for setting PS1
const EnvPs1 = "KUBE_PS1_CLUSTER_FUNCTION"

var (
	args struct {
		multiCluster     bool
		kubeConfigPath   string
		pd               string
		defaultNamespace string
	}

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
			if cmd.Flags().Lookup("pd").Changed {
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
		RunE:         runLogin,
		SilenceUsage: true,
	}
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
		"Login using PagerDuty incident id or html_url.",
	)
	flags.StringVarP(
		&args.defaultNamespace,
		"namespace",
		"n",
		"default",
		"The  default namespace for a user to execute commands in",
	)

}

func runLogin(cmd *cobra.Command, argv []string) (err error) {
	var clusterKey string
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

	logger.Debugf("Extracting Backplane Cluster ID")
	// Currently go-pagerduty pkg does not include incident id validation.
	if args.pd != "" {
		pdClient, err := pagerduty.NewWithToken(bpConfig.PagerDutyAPIKey)
		if err != nil {
			return fmt.Errorf("could not initialize the client: %w", err)
		}
		if strings.Contains(args.pd, "/incidents/") {
			incidentID := args.pd[strings.LastIndex(args.pd, "/")+1:]
			clusterKey, err = pdClient.GetClusterID(incidentID)
			if err != nil {
				return err
			}
		} else {
			clusterKey, err = pdClient.GetClusterID(args.pd)
			if err != nil {
				return err
			}
		}
	}

	// Get the cluster ID only if it hasn't been populated by PagerDuty.
	if len(argv) == 1 {
		// if explicitly one cluster key given, use it to log in.
		logger.Debugf("Cluster Key is given in argument")
		clusterKey = argv[0]
		logger.WithField("Search Key", clusterKey).Debugln("Finding target cluster")

	} else if len(argv) == 0 && args.pd == "" {
		// if no args given, try to log into the cluster that the user is logged into
		logger.Debugf("Finding Clustrer Key from current cluster")
		clusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
		if err != nil {
			return err
		}
		clusterKey = clusterInfo.ClusterID
		logger.Debugf("Backplane Cluster Infromation data extracted: %+v\n", clusterInfo)
	}
	logger.Debugf("Backplane Cluster Key is: %v \n", clusterKey)

	logger.Debugln("Setting Proxy URL from global options")
	// Set proxy url to http client
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

	logger.WithFields(logger.Fields{
		"ID":   clusterID,
		"Name": clusterName}).Infoln("Target cluster")

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
		var namespaces []string
		namespaces, err = listNamespaces(targetClusterID, isHostedControlPlane)
		if err != nil {
			return err
		}
		fmt.Println("A list of associated namespaces for your given cluster:")
		for _, ns := range namespaces {
			fmt.Println("	" + ns)
		}
	}

	if globalOpts.Service {
		logger.WithField("Cluster ID", clusterID).Debugln("Finding service cluster")
		clusterID, clusterName, err = ocm.DefaultOCMInterface.GetServiceCluster(clusterID)
		if err != nil {
			return err
		}
		logger.Debugf("Service clusterID is : %v \n", clusterID)
		logger.Debugf("Service cluster name is : %v \n", clusterName)

		logger.WithFields(logger.Fields{
			"ID":   clusterID,
			"Name": clusterName}).Infoln("Service cluster")
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
		// If login failed, we try to find out if the cluster is hibernating
		isHibernating, _ := ocm.DefaultOCMInterface.IsClusterHibernating(clusterID)
		if isHibernating {
			// Hibernating, print an error
			return fmt.Errorf("cluster %s is hibernating, login failed", clusterKey)
		}
		// Check API connection with configured proxy
		if connErr := bpConfig.CheckAPIConnection(); connErr != nil {
			return fmt.Errorf("cannot connect to backplane API URL, check if you need to use a proxy/VPN to access backplane: %v", connErr)
		}

		// Otherwise, return the failure
		return fmt.Errorf("can't login to cluster: %v", err)
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

	targetUserNickName := getUsernameFromJWT(*accessToken)

	targetUser.Token = *accessToken

	targetContext.AuthInfo = targetUserNickName
	targetContext.Cluster = clusterName

	if isValidKubernetesNamespace(args.defaultNamespace) {
		logger.Debugln("Validating argument passed as namespace")
		targetContext.Namespace = args.defaultNamespace
	} else {
		return fmt.Errorf("%v is not a valid namespace", args.defaultNamespace)
	}

	targetContextNickName := getContextNickname(targetContext.Namespace, targetContext.Cluster, targetContext.AuthInfo)

	// Put user, cluster, context into rawconfig
	rc.Clusters[targetContext.Cluster] = targetCluster
	rc.AuthInfos[targetUserNickName] = targetUser
	rc.Contexts[targetContextNickName] = targetContext
	rc.CurrentContext = targetContextNickName

	logger.Debugln("Saving new API config")
	// Save the config
	err = login.SaveKubeConfig(clusterID, rc, args.multiCluster, args.kubeConfigPath)

	return err
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

	cfg := &rest.Config{
		Host:        bpAPIClusterURL,
		BearerToken: *accessToken,
	}

	if bp.ProxyURL != nil {
		cfg.Proxy = func(*http.Request) (*url.URL, error) {
			return url.Parse(*bp.ProxyURL)
		}
	}

	return cfg, nil
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

// getContextNickname returns a nickname of a context
func getContextNickname(namespace, clusterNick, userNick string) string {
	tokens := strings.SplitN(userNick, "/", 2)
	return namespace + "/" + clusterNick + "/" + tokens[0]
}

// getUsernameFromJWT returns the username extracted from JWT token
func getUsernameFromJWT(token string) string {
	var jwtToken *jwt.Token
	var err error
	parser := new(jwt.Parser)
	jwtToken, _, err = parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "anonymous"
	}
	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return "anonymous"
	}
	claim, ok := claims["username"]
	if !ok {
		return "anonymous"
	}
	return claim.(string)
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

	if resp.StatusCode != http.StatusOK {
		return "", utils.TryPrintAPIError(resp, false)
	}

	loginResp, err := BackplaneApi.ParseLoginClusterResponse(resp)

	if err != nil {
		return "", fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	return api + *loginResp.JSON200.ProxyUri, nil
}

func listNamespaces(clusterID string, isHostedControlPlane bool) ([]string, error) {

	env, err := ocm.DefaultOCMInterface.GetOCMEnvironment()
	if err != nil {
		return []string{}, err
	}
	envName := env.Name()

	clusterInfo, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return []string{}, err
	}

	klusterletPrefix := "klusterlet-"
	hivePrefix := fmt.Sprintf("uhc-%s-", envName)
	hcpPrefix := fmt.Sprintf("ocm-%s-", envName)

	var nsList []string

	if isHostedControlPlane {
		nsList = []string{
			klusterletPrefix + clusterID,
			hcpPrefix + clusterID,
			hcpPrefix + clusterID + "-" + clusterInfo.DomainPrefix(),
		}
	} else {
		nsList = []string{
			hivePrefix + clusterID,
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
