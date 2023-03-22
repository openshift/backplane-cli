package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openshift-online/ocm-cli/pkg/ocm"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/info"
	logger "github.com/sirupsen/logrus"

	"gopkg.in/AlecAivazis/survey.v1"
)

// OCM Wrapper to abstract ocm sdk interface
// Provides a minimal interface for backplane-cli to function

type OCMInterface interface {
	IsClusterHibernating(clusterId string) (bool, error)
	GetTargetCluster(clusterKey string) (clusterId, clusterName string, err error)
	GetManagingCluster(clusterKey string) (clusterId, clusterName string, err error)
	GetOCMAccessToken() (*string, error)
	GetClusterInfoByID(clusterId string) (*cmv1.Cluster, error)
	GetBackplaneURL() (string, error)
	IsProduction() (bool, error)
}

type DefaultOCMInterfaceImpl struct{}

var DefaultOCMInterface OCMInterface = &DefaultOCMInterfaceImpl{}

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

// GetManagingCluster returns the managing cluster (hive shard or hypershift management cluster)
// for the given clusterId
func (o *DefaultOCMInterfaceImpl) GetManagingCluster(targetClusterId string) (clusterId, clusterName string, err error) {
	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return "", "", fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	var managingCluster string
	// Check if cluster has hypershift enabled
	hypershiftResp, err := connection.ClustersMgmt().V1().Clusters().
		Cluster(targetClusterId).
		Hypershift().
		Get().
		Send()
	if err == nil && hypershiftResp != nil {
		managingCluster = hypershiftResp.Body().ManagementCluster()
	} else {
		// Get the client for the resource that manages the collection of clusters:
		clusterCollection := connection.ClustersMgmt().V1().Clusters()
		resource := clusterCollection.Cluster(targetClusterId).ProvisionShard()
		response, err := resource.Get().Send()
		if err != nil {
			return "", "", fmt.Errorf("failed to find management cluster for cluster %s: %v", targetClusterId, err)
		}
		shard := response.Body()
		hiveAPI := shard.HiveConfig().Server()

		// Now find the proper cluster name based on the API URL of the provision shard
		mcResp, err := clusterCollection.
			List().
			Parameter("search", fmt.Sprintf("api.url='%s'", hiveAPI)).
			Send()
		if err != nil || mcResp.Items().Len() == 0 {
			return "", "", fmt.Errorf("failed to find find management cluster for cluster %s: %v", targetClusterId, err)
		}
		managingCluster = mcResp.Items().Get(0).Name()
	}

	if managingCluster == "" {
		return "", "", fmt.Errorf("failed to lookup managing cluster for cluster %s", targetClusterId)
	}

	mcid, _, err := o.GetTargetCluster(managingCluster)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup managing cluster %s: %v", managingCluster, err)
	}
	return mcid, managingCluster, nil
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

// GetBackplaneURL from local settings
func (*DefaultOCMInterfaceImpl) GetBackplaneURL() (string, error) {
	// get backplane URL from BACKPLANE_URL env variables
	bpURL, hasURL := os.LookupEnv(info.BACKPLANE_URL_ENV_NAME)
	if hasURL {
		if bpURL != "" {
			return bpURL, nil
		} else {
			return "", fmt.Errorf("%s env variable is empty", info.BACKPLANE_URL_ENV_NAME)
		}
	} else {
		// get backplane URL for user home folder .backplane.json file
		filePath := config.GetBackplaneConfigFile()
		if _, err := os.Stat(filePath); err == nil {
			file, err := os.Open(filePath)

			if err != nil {
				return "", fmt.Errorf("failed to read file %s : %v", filePath, err)
			}

			defer file.Close()
			decoder := json.NewDecoder(file)
			configuration := config.BackplaneConfiguration{}
			err = decoder.Decode(&configuration)

			if err != nil {
				return "", fmt.Errorf("failed to decode file %s : %v", filePath, err)
			}
			return configuration.URL, nil
		}

	}
	return "", nil
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
