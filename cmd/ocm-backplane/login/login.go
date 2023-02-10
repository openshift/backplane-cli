/*
Copyright © 2020 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package login

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	BackplaneApi "github.com/openshift/backplane-cli/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var (
	args struct {
		backplaneURL string
	}
)

// LoginCmd represents the login command
var LoginCmd = &cobra.Command{
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

func init() {
	flags := LoginCmd.Flags()
	flags.StringVar(
		&args.backplaneURL,
		"url",
		"", // Change the default url when backplane api is ready.
		"Specify backplane url. Default: The corresponding hive shard of the target cluster.",
	)
}

func runLogin(cmd *cobra.Command, argv []string) error {
	if len(argv) != 1 {
		return fmt.Errorf("expected exactly one cluster")
	}
	clusterKey := argv[0]
	logger.WithField("Search Key", clusterKey).Debugln("Finding target cluster")
	targetClusterID, clusterName, err := utils.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}
	logger.WithFields(logger.Fields{
		"ID":   targetClusterID,
		"Name": clusterName}).Infoln("Target cluster")

	// Lookup backplane url
	if args.backplaneURL == "" {
		return errors.New("No backplane URL supplied")
	}
	logger.Infof("Using backplane URL: %s\n", args.backplaneURL)

	// Get ocm access token
	logger.Debugln("Finding ocm token")
	accessToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}
	logger.Debugln("Found OCM access token")

	// Query backplane-api for proxy url
	proxyURL, err := doLogin(targetClusterID, *accessToken)
	if err != nil {
		// If login failed, we try to find out if the cluster is hibernating
		isHibernating, _ := utils.DefaultOCMInterface.IsClusterHibernating(targetClusterID)
		if isHibernating {
			// Hibernating, print an error
			return fmt.Errorf("cluster %s is hibernating, login failed", clusterKey)
		}
		// Otherwise, return the failure
		return fmt.Errorf("can't login to cluster: %v", err)
	}
	logger.WithField("URL", proxyURL).Debugln("Proxy")

	cf := genericclioptions.NewConfigFlags(true)
	rc, err := cf.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}

	// Add a new cluster & context & user
	logger.Debugln("Writing OCM configuration ")

	targetCluster := api.NewCluster()
	targetUser := api.NewAuthInfo()
	targetContext := api.NewContext()

	targetCluster.Server = proxyURL

	targetUserNickName := getUsernameFromJWT(*accessToken)
	execConfig := &api.ExecConfig{}
	scriptName, err := createTokenScriptIfNotExist()
	if err != nil {
		return err
	}
	if len(scriptName) == 0 {
		return fmt.Errorf("failed to create token script")
	}
	execConfig.Command = "bash"
	execConfig.Args = []string{scriptName}
	execConfig.APIVersion = "client.authentication.k8s.io/v1beta1"

	ocmEnv := &api.ExecEnvVar{}
	ocmConfigVal, hasOcmEnv := os.LookupEnv("OCM_CONFIG")
	if hasOcmEnv {
		ocmEnv.Name = "OCM_CONFIG"
		ocmEnv.Value = ocmConfigVal
		execConfig.Env = []api.ExecEnvVar{*ocmEnv}
	}

	targetUser.Exec = execConfig

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
func doLogin(clusterid, accessToken string) (string, error) {
	client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(args.backplaneURL, accessToken)

	if err != nil {
		return "", fmt.Errorf("unable to create backplane api client")
	}

	logger.WithField("URL", args.backplaneURL).Debugln("GetProxyURL")
	resp, err := client.LoginCluster(context.TODO(), clusterid)

	// Print the whole response if we can't parse it. Eg. 5xx error from http server.
	if err != nil {
		// trying to determine the error
		errBody := err.Error()
		if strings.Contains(errBody, "dial tcp") && strings.Contains(errBody, "i/o timeout") {
			// Likely tunnel problem
			return "", fmt.Errorf("unable to connect to backplane api, please check if the tunnel is running")
		}

		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", utils.TryPrintAPIError(resp, false)
	}

	loginResp, err := BackplaneApi.ParseLoginClusterResponse(resp)

	if err != nil {
		return "", fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d\n", resp.StatusCode)
	}

	return args.backplaneURL + *loginResp.JSON200.ProxyUri, nil
}

// createTokenScriptIfNotExist creates the exec script file for use in kubeconfig,
// so there's no need to login everytime the access token expires.
func createTokenScriptIfNotExist() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("can't get user homedir. Error: %s", err.Error())
	}

	filename := homedir + "/.kube/ocm-token"
	_, err = os.Stat(filename)

	if !os.IsNotExist(err) {
		return filename, nil
	}

	if err := os.MkdirAll(homedir+"/.kube/", 0750); err != nil {
		return "", err
	}

	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}

	defer func() {
		err = file.Close()
	}()

	script :=
		`#!/bin/bash
token="$(ocm token)"
cat <<TOKEN
{
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "kind": "ExecCredential",
  "status": {
    "token": "$token"
  }
}
TOKEN`

	fmt.Fprint(file, script)
	err = os.Chmod(filename, 0600)
	if err != nil {
		return "", err
	}

	return filename, err
}
