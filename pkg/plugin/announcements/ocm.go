package announcements

import (
	"fmt"

	sdk "github.com/openshift-online/ocm-sdk-go"
	amv1 "github.com/openshift-online/ocm-sdk-go/accountsmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/ocm"
)

type clusterRecord struct {
	ClusterID  string
	ExternalID string
	OrgName    string
	Version    string
	Product    string
}

// Create a Cluster Record Object required for the handover announcements
func createClusterRecord(clusterKey string) (*clusterRecord, error) {
	var ClusterRecord *clusterRecord = &clusterRecord{}
	connection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		return ClusterRecord, fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()
	// Directly Querying for the Cluster Object as this has been identified to be the correct one
	res, err := connection.ClustersMgmt().V1().Clusters().Cluster(clusterKey).Get().Send()
	if err != nil {
		return ClusterRecord, fmt.Errorf("unable get get cluster status: %v", err)
	}

	cluster := res.Body()
	org, err := getOrganization(connection, clusterKey)
	if err != nil {
		return ClusterRecord, fmt.Errorf("failed to get org '%s': %v", clusterKey, err)
	}

	ClusterRecord.ClusterID = cluster.ID()
	ClusterRecord.ExternalID = cluster.ExternalID()
	ClusterRecord.Version = cluster.Version().RawID()
	ClusterRecord.Product = determineClusterProduct(cluster.Product().ID(), cluster.Hypershift().Enabled())
	ClusterRecord.OrgName = org.Name()

	return ClusterRecord, nil
}

// Get the organization for the cluster

func getOrganization(connection *sdk.Connection, key string) (*amv1.Organization, error) {
	subscription, err := getSubscription(connection, key)
	if err != nil {
		return nil, err
	}
	orgResponse, err := connection.AccountsMgmt().V1().Organizations().Organization(subscription.OrganizationID()).Get().Send()
	if err != nil {
		return nil, err
	}
	return orgResponse.Body(), nil
}

// Get the subscription for the cluster

func getSubscription(connection *sdk.Connection, key string) (subscription *amv1.Subscription, err error) {
	// Prepare the resources that we will be using:
	subsResource := connection.AccountsMgmt().V1().Subscriptions()

	// Try to find a matching subscription:
	subsSearch := fmt.Sprintf(
		"(display_name = '%s' or cluster_id = '%s' or external_cluster_id = '%s' or id = '%s')",
		key, key, key, key)
	subsListResponse, err := subsResource.List().Parameter("search", subsSearch).Send()
	if err != nil {
		err = fmt.Errorf("can't retrieve subscription for key '%s': %v", key, err)
		return
	}

	// If there is exactly one matching subscription then return the corresponding cluster:
	subsTotal := subsListResponse.Total()
	if subsTotal == 1 {
		return subsListResponse.Items().Get(0), nil
	}

	// If there are multiple subscriptions that match the key then we should report it as
	// an error:
	if subsTotal > 1 {
		err = fmt.Errorf(
			"there are %d subscriptions with cluster identifier or name '%s'",
			subsTotal, key,
		)
		return
	}
	// If we are here then there are no subscriptions matching the passed key:
	err = fmt.Errorf(
		"there are no subscriptions with identifier or name '%s'",
		key,
	)
	return
}
