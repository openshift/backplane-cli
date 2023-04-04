package utils

import (
	"fmt"
	"net/url"
	"regexp"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	logger "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

type BackplaneCluster struct {
	ClusterID     string
	ClusterURL    string // for e.g. https://api-backplane.apps.com/backplane/cluster/<cluster-id>/
	BackplaneHost string // for e.g. https://api-backplane.apps.com
}

type ClusterUtils interface {
	GetClusterIDAndHostFromClusterURL(clusterURL string) (string, string, error)
	GetBackplaneClusterFromConfig() (BackplaneCluster, error)
	GetBackplaneClusterFromClusterKey(clusterKey string) (BackplaneCluster, error)
	GetCloudProvider(cluster *cmv1.Cluster) string
	GetBackplaneCluster(params ...string) (BackplaneCluster, error)
}

type DefaultClusterUtilsImpl struct {}

var (
	DefaultClusterUtils ClusterUtils = &DefaultClusterUtilsImpl{}
)

// Cluster URL format: https://api-backplane.apps.com/backplane/cluster/<cluster-id>/
func (s *DefaultClusterUtilsImpl) GetClusterIDAndHostFromClusterURL(clusterURL string) (string, string, error) {
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

func (s *DefaultClusterUtilsImpl) GetBackplaneClusterFromConfig() (BackplaneCluster, error) {
	logger.Debugln("Finding target cluster from kube config")
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultPathOptions().GetDefaultFilename())
	if err != nil {
		return BackplaneCluster{}, err
	}

	clusterID, backplaneHost, err := s.GetClusterIDAndHostFromClusterURL(cfg.Host)
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

func (s *DefaultClusterUtilsImpl) GetBackplaneClusterFromClusterKey(clusterKey string) (BackplaneCluster, error) {
	logger.WithField("SearchKey", clusterKey).Debugln("Finding target cluster")
	clusterID, clusterName, err := DefaultOCMInterface.GetTargetCluster(clusterKey)

	if err != nil {
		return BackplaneCluster{}, err
	}

	bpConfig, err := config.GetBackplaneConfiguration()

	backplaneURL := bpConfig.URL

	if err != nil {
		return BackplaneCluster{}, err
	}
	cluster := BackplaneCluster{
		ClusterID:     clusterID,
		BackplaneHost: backplaneURL,
		ClusterURL:    fmt.Sprintf("%s/backplane/cluster/%s", backplaneURL, clusterID),
	}
	logger.WithFields(logger.Fields{
		"ClusterID":     cluster.ClusterID,
		"BackplaneHost": cluster.BackplaneHost,
		"ClusterURL":    cluster.ClusterURL,
		"Name":          clusterName}).Debugln("Found target cluster")
	return cluster, nil
}

// GetBackplaneCluster returns BackplaneCluster, if clusterKey is present it will try to search for cluster otherwise it will load cluster from the kube config file.
func (s *DefaultClusterUtilsImpl) GetBackplaneCluster(params ...string) (BackplaneCluster, error) {
	if len(params) > 0 && params[0] != "" {
		return s.GetBackplaneClusterFromClusterKey(params[0])
	}
	return s.GetBackplaneClusterFromConfig()
}

func (s *DefaultClusterUtilsImpl) GetCloudProvider(cluster *cmv1.Cluster) string {
	return cluster.CloudProvider().ID()
}
