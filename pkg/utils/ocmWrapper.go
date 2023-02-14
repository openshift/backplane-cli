package utils

import (
	"fmt"
	"strings"

	"github.com/openshift-online/ocm-cli/pkg/ocm"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	logger "github.com/sirupsen/logrus"

	"gopkg.in/AlecAivazis/survey.v1"
)

// OCM Wrapper to abstract ocm sdk interface
// Provides a minimal interface for backplane-cli to function

type OCMInterface interface {
	GetBackplaneShard(clusterId string) (string, error)
	GetAllBackplaneShards() ([]string, error)
	IsClusterHibernating(clusterId string) (bool, error)
	GetTargetCluster(clusterKey string) (clusterId, clusterName string, err error)
	GetOCMAccessToken() (*string, error)
	GetClusterInfoByID(clusterId string) (*cmv1.Cluster, error)
	IsProduction() (bool, error)
}

type DefaultOCMInterfaceImpl struct{}

var DefaultOCMInterface OCMInterface = &DefaultOCMInterfaceImpl{}

// GetBackplaneShard returns the backplane url in the hive shard of the target cluster.
func (*DefaultOCMInterfaceImpl) GetBackplaneShard(clusterId string) (shardURL string, err error) {
	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return "", fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	// Check if cluster has hypershift enabled
	hyperShiftEnabled, err := CheckHypershift(clusterId)
	if err != nil {
		return "", fmt.Errorf("failed to check if the cluster is hyperShift enabled: %v", err)
	}

	hiveURL := ""
	if hyperShiftEnabled {
		// Detect the environment to find the correct Hive URL
		environment := GetOCMEnvironmentFromCoonectionURL(connection.URL())
		if len(environment) == 0 {
			return "", fmt.Errorf("could not determine environment for cluster ID: %s", clusterId)
		}

		hiveURL, err = GetHiveUrlForHyperShiftByEnvironment(environment)
		if len(hiveURL) == 0 || err != nil {
			return "", fmt.Errorf("failed to acquire hypershift enabled cluster's hive url: %v", err)
		}
	} else {
		// Get the client for the resource that manages the collection of clusters:
		clusterCollection := connection.ClustersMgmt().V1().Clusters()
		resource := clusterCollection.Cluster(clusterId).ProvisionShard()
		response, err := resource.Get().Send()
		if err != nil {
			return "", fmt.Errorf("failed to get hive shard for cluster %s: %v", clusterId, err)
		}
		shard := response.Body()
		hiveURL = shard.HiveConfig().Server()
	}

	if len(shardReplaceMap[hiveURL]) > 0 {
		hiveURL = shardReplaceMap[hiveURL]
	}

	shardURL = getBackplaneShardURLFromHiveURL(hiveURL)
	return shardURL, nil
}

// GetAllBackplaneShards returns all the backplane url.
func (o *DefaultOCMInterfaceImpl) GetAllBackplaneShards() (shardURLs []string, err error) {

	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return shardURLs, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	shardsResponse, err := connection.ClustersMgmt().V1().ProvisionShards().List().Send()
	if err != nil {
		return shardURLs, fmt.Errorf("failed to get provision shards: %v", err)
	}
	shardItems, ok := shardsResponse.GetItems()
	if !ok {
		return shardURLs, fmt.Errorf("failed to get provision shard items")
	}

	shardItems.Each(func(item *cmv1.ProvisionShard) bool {
		if item.Status() == "offline" {
			// skip offline shards
			return true
		}

		hiveURL := item.HiveConfig().Server()
		if len(hiveURL) == 0 {
			// skip shards without hive url
			// TODO(cblecker): this will likely need to change for hypershift
			return true
		}

		if len(shardReplaceMap[hiveURL]) > 0 {
			hiveURL = shardReplaceMap[hiveURL]
		}
		shardURL := getBackplaneShardURLFromHiveURL(hiveURL)
		shardURLs = append(shardURLs, shardURL)
		return true
	})
	return shardURLs, nil
}

// IsClusterHibernating returns a boolean to indicate whether the cluster is hibernating
func (*DefaultOCMInterfaceImpl) IsClusterHibernating(clusterId string) (bool, error) {
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return false, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()
	res, err := connection.ClustersMgmt().V1().Clusters().Cluster(clusterId).Get().Send()
	if err != nil {
		return false, fmt.Errorf("unable get get cluster status: %v", err)
	}

	cluster := res.Body()
	return cluster.Status().State() == cmv1.ClusterStateHibernating, nil
}

// GetTargetCluster returns one single cluster based on the search key and survery.
func (*DefaultOCMInterfaceImpl) GetTargetCluster(clusterKey string) (clusterId, clusterName string, err error) {
	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return "", "", fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()
	// Get the client for the resource that manages the collection of clusters:
	clusterCollection := connection.ClustersMgmt().V1().Clusters()
	clusters, err := getClusters(clusterCollection, clusterKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to get cluster '%s': %v", clusterKey, err)
	}

	if len(clusters) == 1 {
		for _, v := range clusters {
			clusterId = v.ID()
			clusterName = v.Name()
		}
	} else {
		cluster, err := doSurvey(clusters)
		if err != nil {
			return "", "", fmt.Errorf("can't find cluster: %v", err)
		}
		clusterId = cluster.ID()
		clusterName = cluster.Name()
	}
	return clusterId, clusterName, nil
}

func (*DefaultOCMInterfaceImpl) GetOCMAccessToken() (*string, error) {
	// Get ocm access token
	logger.Debugln("Finding ocm token")
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()
	accessToken, _, err := connection.Tokens()
	if err != nil {
		return nil, err
	}

	logger.Debugln("Found OCM access token")
	accessToken = strings.TrimSuffix(accessToken, "\n")

	return &accessToken, nil
}

// GetClusterInfoByID calls the OCM to retrieve the cluster info
// for a given internal cluster id.
func (*DefaultOCMInterfaceImpl) GetClusterInfoByID(clusterId string) (*cmv1.Cluster, error) {
	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	// Get the cluster info based on the cluster id
	response, err := connection.ClustersMgmt().V1().Clusters().Cluster(clusterId).Get().Send()
	if err != nil {
		return nil, fmt.Errorf("can't retrieve cluster for id '%s': %v", clusterId, err)
	}
	cluster, ok := response.GetBody()
	if !ok {
		return nil, fmt.Errorf("can't retrieve cluster for id '%s', nil value", clusterId)
	}
	return cluster, nil
}

// IsProduction checks if OCM is currently in production env
func (*DefaultOCMInterfaceImpl) IsProduction() (bool, error) {
	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return false, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	return connection.URL() == "https://api.openshift.com", nil
}

func getClusters(client *cmv1.ClustersClient, clusterKey string) ([]*cmv1.Cluster, error) {

	var clusters []*cmv1.Cluster
	query := fmt.Sprintf(
		"(id = '%s' or name like '%s' or external_id = '%s')",
		clusterKey, clusterKey, clusterKey,
	)
	response, err := client.List().
		Search(query).
		Page(1).
		Size(ClustersPageSize).
		Send()
	if err != nil {
		return nil, fmt.Errorf("failed to locate cluster '%s': %v", clusterKey, err)
	}

	clusters = response.Items().Slice()

	switch response.Total() {
	case 0:
		return nil, fmt.Errorf("there is no cluster with identifier or name '%s'", clusterKey)
	default:
		return clusters, nil
	}
}

// doSurvey will ask user to choose one if there are more than one clusters match the query
func doSurvey(clusters []*cmv1.Cluster) (cluster *cmv1.Cluster, err error) {
	clusterList := []string{}
	for _, v := range clusters {
		clusterList = append(clusterList, fmt.Sprintf("Name: %s, ID: %s", v.Name(), v.ID()))
	}
	choice := ""
	prompt := &survey.Select{
		Message: "Please choose a cluster:",
		Options: clusterList,
		Default: clusterList[0],
	}
	survey.PageSize = ClustersPageSize
	err = survey.AskOne(prompt, &choice, func(ans interface{}) error {
		choice := ans.(string)
		found := false
		for _, v := range clusters {
			if strings.Contains(choice, v.ID()) {
				found = true
				cluster = v
			}
		}
		if !found {
			return fmt.Errorf("the cluster you choose is not valid: %s", choice)
		}
		return nil
	})
	return cluster, err
}

// This return backplane shard url from an existing hive api url
func getBackplaneShardURLFromHiveURL(hiveURL string) string {
	if len(hiveURL) == 0 {
		return ""
	}

	shardURL := strings.Replace(hiveURL, "https://api", "https://api-backplane.apps", 1)
	shardURL = strings.Replace(shardURL, ":6443", "", 1)
	return shardURL
}
