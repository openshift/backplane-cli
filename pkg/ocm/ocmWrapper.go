package ocm

import (
	"fmt"
	"strings"

	"github.com/openshift-online/ocm-cli/pkg/ocm"
	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	logger "github.com/sirupsen/logrus"

	"gopkg.in/AlecAivazis/survey.v1"
)

// OCM Wrapper to abstract ocm sdk interface
// Provides a minimal interface for backplane-cli to function

type OCMInterface interface {
	IsClusterHibernating(clusterID string) (bool, error)
	GetTargetCluster(clusterKey string) (clusterID, clusterName string, err error)
	GetManagingCluster(clusterKey string) (clusterID, clusterName string, err error)
	GetOCMAccessToken() (*string, error)
	GetServiceCluster(clusterKey string) (clusterID, clusterName string, err error)
	GetClusterInfoByID(clusterID string) (*cmv1.Cluster, error)
	IsProduction() (bool, error)
	GetPullSecret() (string, error)
	GetStsSupportJumpRoleARN(ocmConnection *ocmsdk.Connection, clusterID string) (string, error)
	GetOCMEnvironment() (*cmv1.Environment, error)
}

const (
	ClustersPageSize = 50
)

type DefaultOCMInterfaceImpl struct{}

var DefaultOCMInterface OCMInterface = &DefaultOCMInterfaceImpl{}

// IsClusterHibernating returns a boolean to indicate whether the cluster is hibernating
func (*DefaultOCMInterfaceImpl) IsClusterHibernating(clusterID string) (bool, error) {
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return false, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()
	res, err := connection.ClustersMgmt().V1().Clusters().Cluster(clusterID).Get().Send()
	if err != nil {
		return false, fmt.Errorf("unable get get cluster status: %v", err)
	}

	cluster := res.Body()
	return cluster.Status().State() == cmv1.ClusterStateHibernating, nil
}

// GetTargetCluster returns one single cluster based on the search key and survery.
func (*DefaultOCMInterfaceImpl) GetTargetCluster(clusterKey string) (clusterID, clusterName string, err error) {
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
			clusterID = v.ID()
			clusterName = v.Name()
		}
	} else {
		cluster, err := doSurvey(clusters)
		if err != nil {
			return "", "", fmt.Errorf("can't find cluster: %v", err)
		}
		clusterID = cluster.ID()
		clusterName = cluster.Name()
	}
	return clusterID, clusterName, nil
}

// GetManagingCluster returns the managing cluster (hive shard or hypershift management cluster)
// for the given clusterID
func (o *DefaultOCMInterfaceImpl) GetManagingCluster(targetClusterID string) (clusterID, clusterName string, err error) {
	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return "", "", fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	var managingCluster string
	// Check if cluster has hypershift enabled
	hypershiftResp, err := connection.ClustersMgmt().V1().Clusters().
		Cluster(targetClusterID).
		Hypershift().
		Get().
		Send()
	if err == nil && hypershiftResp != nil {
		managingCluster = hypershiftResp.Body().ManagementCluster()
	} else {
		// Get the client for the resource that manages the collection of clusters:
		clusterCollection := connection.ClustersMgmt().V1().Clusters()
		resource := clusterCollection.Cluster(targetClusterID).ProvisionShard()
		response, err := resource.Get().Send()
		if err != nil {
			return "", "", fmt.Errorf("failed to find management cluster for cluster %s: %v", targetClusterID, err)
		}
		shard := response.Body()
		hiveAPI := shard.HiveConfig().Server()

		// Now find the proper cluster name based on the API URL of the provision shard
		mcResp, err := clusterCollection.
			List().
			Parameter("search", fmt.Sprintf("api.url='%s'", hiveAPI)).
			Send()

		if err != nil {
			return "", "", fmt.Errorf("failed to find management cluster for cluster %s: %v", targetClusterID, err)
		}
		if mcResp.Items().Len() == 0 {
			return "", "", fmt.Errorf("failed to find management cluster for cluster %s in %s env", targetClusterID, connection.URL())
		}
		managingCluster = mcResp.Items().Get(0).Name()
	}

	if managingCluster == "" {
		return "", "", fmt.Errorf("failed to lookup managing cluster for cluster %s", targetClusterID)
	}

	mcid, _, err := o.GetTargetCluster(managingCluster)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup managing cluster %s: %v", managingCluster, err)
	}
	return mcid, managingCluster, nil
}

// GetServiceCluster gets the service cluster for a given hpyershift hosted cluster
func (*DefaultOCMInterfaceImpl) GetServiceCluster(targetClusterID string) (clusterID, clusterName string, err error) {
	// Create the client for the OCM API
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return "", "", fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	var svcClusterID, svcClusterName, mgmtCluster string
	// If given cluster is hypershift hosted cluster
	hypershiftResp, err := connection.ClustersMgmt().V1().Clusters().
		Cluster(targetClusterID).
		Hypershift().
		Get().
		Send()
	if err == nil && hypershiftResp != nil {
		mgmtCluster = hypershiftResp.Body().ManagementCluster()
	} else {
		// If given cluster is management cluster
		mgmtClusterResp, err := connection.OSDFleetMgmt().V1().ManagementClusters().List().
			Parameter("search", fmt.Sprintf("cluster_id='%s'", targetClusterID)).
			Send()
		if err == nil && mgmtClusterResp.Size() > 0 {
			mgmtCluster = mgmtClusterResp.Items().Get(0).Name()
		}
	}

	if mgmtCluster == "" {
		return "", "", fmt.Errorf("failed to lookup managing cluster for cluster %s", targetClusterID)
	}

	// Get the osd_fleet_mgmt reference for the given mgmt_cluster
	ofmResp, err := connection.OSDFleetMgmt().V1().ManagementClusters().
		List().
		Parameter("search", fmt.Sprintf("name='%s'", mgmtCluster)).
		Send()
	if err != nil {
		return "", "", fmt.Errorf("failed to get the fleet manager information for management cluster %s", mgmtCluster)
	}

	if kind := ofmResp.Items().Get(0).Parent().Kind(); kind == "ServiceCluster" {
		svcClusterName = ofmResp.Items().Get(0).Parent().Name()
		svcClusterResp, err := connection.ClustersMgmt().V1().Clusters().List().
			Parameter("search", fmt.Sprintf("name='%s'", svcClusterName)).Send()
		if err != nil {
			return "", "", fmt.Errorf("failed to get the service cluster id")
		}
		svcClusterID = svcClusterResp.Items().Get(0).ID()
	}

	return svcClusterID, svcClusterName, nil
}

// GetOCMAccessToken initiates the OCM connection and returns the access token
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

// GetPullSecret returns pull secret from OCM
func (*DefaultOCMInterfaceImpl) GetPullSecret() (string, error) {

	// Get ocm access token
	logger.Debugln("Finding ocm token")
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return "", fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()
	response, err := connection.Post().Path("/api/accounts_mgmt/v1/access_token").Send()
	if err != nil {
		return "", fmt.Errorf("failed to get pull secret from ocm: %v", err)
	}

	logger.Debugln("Found pull secret from ocm")
	pullSecret := response.String()

	return pullSecret, nil
}

// GetClusterInfoByID calls the OCM to retrieve the cluster info
// for a given internal cluster id.
func (*DefaultOCMInterfaceImpl) GetClusterInfoByID(clusterID string) (*cmv1.Cluster, error) {
	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	// Get the cluster info based on the cluster id
	response, err := connection.ClustersMgmt().V1().Clusters().Cluster(clusterID).Get().Send()
	if err != nil {
		return nil, fmt.Errorf("can't retrieve cluster for id '%s': %v", clusterID, err)
	}
	cluster, ok := response.GetBody()
	if !ok {
		return nil, fmt.Errorf("can't retrieve cluster for id '%s', nil value", clusterID)
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

func (*DefaultOCMInterfaceImpl) GetStsSupportJumpRoleARN(ocmConnection *ocmsdk.Connection, clusterID string) (string, error) {
	response, err := ocmConnection.ClustersMgmt().V1().Clusters().Cluster(clusterID).StsSupportJumpRole().Get().Send()
	if err != nil {
		return "", fmt.Errorf("failed to get STS Support Jump Role for cluster %v, %w", clusterID, err)
	}
	return response.Body().RoleArn(), nil
}

// GetBackplaneURL returns the Backplane API URL based on the OCM env
func (*DefaultOCMInterfaceImpl) GetOCMEnvironment() (*cmv1.Environment, error) {
	// Create the client for the OCM API
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	responseEnv, err := connection.ClustersMgmt().V1().Environment().Get().Send()
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster environment %w", err)
	}

	return responseEnv.Body(), nil
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
