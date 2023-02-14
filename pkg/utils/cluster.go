package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
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

func GetBackplaneClusterFromClusterKey(clusterKey, bpUrl string) (BackplaneCluster, error) {
	logger.WithField("SearchKey", clusterKey).Debugln("Finding target cluster")
	clusterID, clusterName, err := DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return BackplaneCluster{}, err
	}
	if len(bpUrl) == 0 {
		bpUrl, err = DefaultOCMInterface.GetBackplaneShard(clusterID)
		if err != nil {
			return BackplaneCluster{}, err
		}
	}

	cluster := BackplaneCluster{
		ClusterID:     clusterID,
		BackplaneHost: bpUrl,
		ClusterURL:    fmt.Sprintf("%s/backplane/cluster/%s", bpUrl, clusterID),
	}
	logger.WithFields(logger.Fields{
		"ClusterID":     cluster.ClusterID,
		"BackplaneHost": cluster.BackplaneHost,
		"ClusterURL":    cluster.ClusterURL,
		"Name":          clusterName}).Debugln("Found target cluster")
	return cluster, nil
}

// GetBackplaneCluster returns BackplaneCluster, if clusterKey is present it will try to search for cluster otherwise it will load cluster from the kube config file.
func GetBackplaneCluster(cluster, bpUrl string) (BackplaneCluster, error) {
	if len(cluster) > 0 {
		return GetBackplaneClusterFromClusterKey(cluster, bpUrl)
	}
	return GetBackplaneClusterFromConfig()
}

// check if the cluster is Hypershift
func CheckHypershift(params ...string) (bool, error) {
	clusterId := ""
	if len(params) > 0 && params[0] != "" {
		clusterId = params[0]
	} else {
		currentClusterInfo, err := GetBackplaneClusterFromConfig()
		if err != nil {
			return false, err
		}
		clusterId = currentClusterInfo.ClusterID
	}

	currentCluster, err := DefaultOCMInterface.GetClusterInfoByID(clusterId)
	if err != nil {
		return false, err
	}
	val, ok := currentCluster.GetHypershift()
	if ok && val.Enabled() {
		return true, nil
	}
	return false, nil
}

func GetHiveUrlForHyperShiftByEnvironment(environment string) (string, error) {
	hivesForCurrentEnvironement := GetHiveUrlListForEnvironment(environment)
	hiveIndex, err := GetRandomNumberWithin(len(hivesForCurrentEnvironement))
	if err != nil {
		return "", err
	}
	if len(hivesForCurrentEnvironement) > 0 {
		return hivesForCurrentEnvironement[hiveIndex], nil
	}

	return "", err
}

func GetHiveUrlListForEnvironment(environment string) []string {
	hyperShiftHives := make(map[string][]string)
	hyperShiftHives["integration"] = []string{
		"https://api.hivei01ue1.f7i5.p1.openshiftapps.com:6443",
	}
	hyperShiftHives["staging"] = []string{
		"https://api.hives02ue1.j5l5.p1.openshiftapps.com:6443",
	}
	hyperShiftHives["production"] = []string{
		"https://api.hivep01ue1.b6s7.p1.openshiftapps.com:6443",
		"https://api.hivep04ew2.byo5.p1.openshiftapps.com:6443",
	}

	if hives, ok := hyperShiftHives[environment]; ok {
		return hives
	}

	return []string{}
}

func GetRandomNumberWithin(max int) (int, error) {
	value, err := rand.Int(rand.Reader, big.NewInt((int64(max))))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}
