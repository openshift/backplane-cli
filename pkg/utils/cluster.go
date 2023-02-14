package utils

import (
	"fmt"
	"net/url"
	"regexp"

	logger "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

type BackplaneCluster struct {
	ClusterID     string
	ClusterURL    string // for e.g. https://api-backplane.apps.com/backplane/cluster/<cluster-id>/
	BackplaneHost string // for e.g. https://api-backplane.apps.com
}

// Cluster URL format: https://api-backplane.apps.com/backplane/cluster/<cluster-id>/
func GetClusterIDAndHostFromClusterURL(clusterURL string) (string, string, error) {
	parsedURL, err := url.Parse(clusterURL)
	if err != nil {
		return "", "", err
	}
	backplaneHost := "https://" + parsedURL.Host
	re := regexp.MustCompile(ClusterIDRegexp)
	matches := re.FindStringSubmatch(parsedURL.Path)

	if len(matches) < 2 {
		return "", backplaneHost, fmt.Errorf("couldn't find cluster-id from the backplane cluster url")
	}
	clusterID := matches[1] // first capturing group
	return clusterID, backplaneHost, nil
}

func GetBackplaneClusterFromConfig() (BackplaneCluster, error) {
	logger.Debugln("Finding target cluster from kube config")
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultPathOptions().GetDefaultFilename())
	if err != nil {
		return BackplaneCluster{}, err
	}

	backplaneServerRegex := regexp.MustCompile(BackplaneApiUrlRegexp)
	if !backplaneServerRegex.MatchString(cfg.Host) {
		return BackplaneCluster{}, fmt.Errorf("the api server is not a backplane url, please make sure you login to the cluster using backplane")
	}
	clusterID, backplaneHost, err := GetClusterIDAndHostFromClusterURL(cfg.Host)
	if err != nil {
		return BackplaneCluster{}, err
	}
	cluster := BackplaneCluster{
		ClusterID:     clusterID,
		BackplaneHost: backplaneHost,
		ClusterURL:    cfg.Host,
	}
	logger.WithFields(logger.Fields{
		"ClusterID":     cluster.ClusterID,
		"BackplaneHost": cluster.BackplaneHost,
		"ClusterURL":    cluster.ClusterURL}).Debugln("Found target cluster")
	return cluster, nil
}

func GetBackplaneClusterFromClusterKey(clusterKey string) (BackplaneCluster, error) {
	logger.WithField("SearchKey", clusterKey).Debugln("Finding target cluster")
	clusterID, clusterName, err := DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return BackplaneCluster{}, err
	}

	// TODO: Deprecate tunneling
	//backplaneHost, err := DefaultOCMInterface.GetBackplaneShard(clusterID)
	backplaneHost := ""
	if err != nil {
		return BackplaneCluster{}, err
	}
	cluster := BackplaneCluster{
		ClusterID:     clusterID,
		BackplaneHost: backplaneHost,
		ClusterURL:    fmt.Sprintf("%s/backplane/cluster/%s", backplaneHost, clusterID),
	}
	logger.WithFields(logger.Fields{
		"ClusterID":     cluster.ClusterID,
		"BackplaneHost": cluster.BackplaneHost,
		"ClusterURL":    cluster.ClusterURL,
		"Name":          clusterName}).Debugln("Found target cluster")
	return cluster, nil
}

// GetBackplaneCluster returns BackplaneCluster, if clusterKey is present it will try to search for cluster otherwise it will load cluster from the kube config file.
func GetBackplaneCluster(params ...string) (BackplaneCluster, error) {
	if len(params) > 0 && params[0] != "" {
		return GetBackplaneClusterFromClusterKey(params[0])
	}
	return GetBackplaneClusterFromConfig()
}
