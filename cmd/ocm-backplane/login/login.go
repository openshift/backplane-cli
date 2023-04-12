package login

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
	"github.com/openshift/backplane-cli/pkg/utils"
)

// Environment variable that for setting PS1
const EnvPs1 = "KUBE_PS1_CLUSTER_FUNCTION"

var (
	args struct {
		manager bool
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
		Example:      " backplane login <id>\n backplane login %test%\n backplane login <external_id>",
		Args:         cobra.ExactArgs(1),
		RunE:         runLogin,
		SilenceUsage: true,
	}
)

func init() {
	flags := LoginCmd.Flags()
	// Add global flags
	//globalflags.AddGlobalFlags(flags, globalOpts)
	globalflags.AddGlobalFlags(LoginCmd, globalOpts)

	// Add login cmd specific flags
	flags.BoolVar(
		&args.manager,
		"manager",
		false,
		"Login to management cluster instead of the cluster itself.",
	)
}

func runLogin(cmd *cobra.Command, argv []string) (err error) {
	var clusterKey string

	// Get The cluster ID
	if len(argv) == 1 {
		// if explicitly one cluster key given, use it to log in.
		clusterKey = argv[0]
		logger.WithField("Search Key", clusterKey).Debugln("Finding target cluster")
	} else if len(argv) == 0 {
		// if no args given, try to log into the cluster that the user is logged into
		clusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
		if err != nil {
			return err
		}
		clusterKey = clusterInfo.ClusterID
	}

	// Get Backplane configuration
	bpConfig, err := config.GetBackplaneConfiguration()

	if err != nil {
		return err
	}

	// Set proxy url to http client
	proxyUrl := globalOpts.ProxyURL
	if proxyUrl != "" {
		err = utils.DefaultClientUtils.SetClientProxyUrl(proxyUrl)

		if err != nil {
			return err
		}
		logger.Debugf("Using backplane Proxy URL: %s\n", proxyUrl)
	}
	if len(proxyUrl) == 0 {
		proxyUrl = bpConfig.ProxyURL
	}

	clusterId, clusterName, err := utils.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}

	logger.WithFields(logger.Fields{
		"ID":   clusterId,
		"Name": clusterName}).Infoln("Target cluster")

	if args.manager {
		logger.WithField("Cluster ID", clusterId).Debugln("Finding managing cluster")
		clusterId, clusterName, err = utils.DefaultOCMInterface.GetManagingCluster(clusterId)
		if err != nil {
			return err
		}

		logger.WithFields(logger.Fields{
			"ID":   clusterId,
			"Name": clusterName}).Infoln("Management cluster")
	}

	// Get Backplane URL
	bpUrl := globalOpts.BackplaneURL
	if bpUrl == "" {
		bpUrl = bpConfig.URL
	}

	if bpUrl == "" {
		return fmt.Errorf("can't find backplane url: %s", bpConfig.URL)
	}

	logger.Debugf("Using backplane URL: %s\n", bpUrl)

	// Get ocm access token
	logger.Debugln("Finding ocm token")
	accessToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}
	logger.Debugln("Found OCM access token")

	// Query backplane-api for proxy url
	bpAPIClusterUrl, err := doLogin(bpUrl, clusterId, *accessToken)
	if err != nil {
		// If login failed, we try to find out if the cluster is hibernating
		isHibernating, _ := utils.DefaultOCMInterface.IsClusterHibernating(clusterId)
		if isHibernating {
			// Hibernating, print an error
			return fmt.Errorf("cluster %s is hibernating, login failed", clusterKey)
		}
		// Otherwise, return the failure
		return fmt.Errorf("can't login to cluster: %v", err)
	}
	logger.WithField("URL", bpAPIClusterUrl).Debugln("Proxy")

	cf := genericclioptions.NewConfigFlags(true)
	rc, err := cf.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}
	// Check PS1 env is set or not

	EnvPs1, ok := os.LookupEnv(EnvPs1)
	if !ok {
		logger.Warn("Env KUBE_PS1_CLUSTER_FUNCTION is not detected. It is recommended to set PS1 to learn which cluster you are operating on, refer https://github.com/openshift/backplane-cli/blob/main/docs/PS1-setup.md. ", EnvPs1)
	}

	// Add a new cluster & context & user
	logger.Debugln("Writing OCM configuration ")

	targetCluster := api.NewCluster()
	targetUser := api.NewAuthInfo()
	targetContext := api.NewContext()

	targetCluster.Server = bpAPIClusterUrl

	// Add proxy URL to target cluster

	if proxyUrl != "" {
		targetCluster.ProxyURL = proxyUrl
	}

	targetUserNickName := getUsernameFromJWT(*accessToken)
	execConfig := &api.ExecConfig{}

	ocmEnv := &api.ExecEnvVar{}
	ocmConfigVal, hasOcmEnv := os.LookupEnv("OCM_CONFIG")
	if hasOcmEnv {
		ocmEnv.Name = "OCM_CONFIG"
		ocmEnv.Value = ocmConfigVal
		execConfig.Env = []api.ExecEnvVar{*ocmEnv}
	}

	targetUser.Token = *accessToken

	targetContext.AuthInfo = targetUserNickName
	targetContext.Cluster = clusterName
	targetContext.Namespace = "default"
	targetContextNickName := getContextNickname(targetContext.Namespace, targetContext.Cluster, targetContext.AuthInfo)

	// Put user, cluster, context into rawconfig
	rc.Clusters[targetContext.Cluster] = targetCluster
	rc.AuthInfos[targetUserNickName] = targetUser
	rc.Contexts[targetContextNickName] = targetContext
	rc.CurrentContext = targetContextNickName

	// Save the config.
	configAccess := clientcmd.NewDefaultPathOptions()
	err = clientcmd.ModifyConfig(configAccess, rc, true)
	logger.Debugln("Wrote OCM configuration")

	return err
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
func doLogin(api, clusterid, accessToken string) (string, error) {

	client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(api, accessToken)

	if err != nil {
		return "", fmt.Errorf("unable to create backplane api client")
	}

	logger.WithField("URL", globalOpts.BackplaneURL).Debugln("GetProxyURL")
	resp, err := client.LoginCluster(context.TODO(), clusterid)

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
