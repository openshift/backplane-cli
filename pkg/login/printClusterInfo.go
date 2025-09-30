package login

import (
	"fmt"

	"github.com/openshift/backplane-cli/pkg/ocm"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

//displayClusterInfo retrieves and displays basic information about the target cluster.

func PrintClusterInfo(clusterID string) error {
	logger := logger.WithField("clusterID", clusterID)

	// Retrieve cluster information
	clusterInfo, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return fmt.Errorf("error retrieving cluster info: %w", err)
	}

	// Display cluster information
	printClusterField("Cluster ID:", clusterInfo.ID())
	printClusterField("Cluster Name:", clusterInfo.Name())
	printClusterField("Cluster Status:", clusterInfo.State())
	printClusterField("Cluster Region:", clusterInfo.Region().ID())
	printClusterField("Cluster Provider:", clusterInfo.CloudProvider().ID())
	printClusterField("Hypershift Enabled:", clusterInfo.Hypershift().Enabled())
	printClusterField("Version:", clusterInfo.OpenshiftVersion())
	GetLimitedSupportStatus(clusterID)
	GetAccessProtectionStatus(clusterID)

	logger.Info("Basic cluster information displayed.")
	return nil
}

// GetAccessProtectionStatus retrieves and displays the access protection status for a cluster.
// It checks if the cluster has access protection enabled (not available for govcloud).
// Returns the status as a string and prints it to stdout.
func GetAccessProtectionStatus(clusterID string) string {
	ocmConnection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		logger.Error("Error setting up OCM connection: ", err)
		return "Error setting up OCM connection: " + err.Error()
	}

	accessProtectionStatus := "Disabled"

	if !(viper.GetBool("govcloud")) {
		enabled, err := ocm.DefaultOCMInterface.IsClusterAccessProtectionEnabled(ocmConnection, clusterID)
		if err != nil {
			fmt.Println("Error retrieving access protection status: ", err)
			return "Error retrieving access protection status: " + err.Error()
		}
		if enabled {
			accessProtectionStatus = "Enabled"
		}
	}

	fmt.Printf("%-25s %s\n", "Access Protection:", accessProtectionStatus)

	return accessProtectionStatus
}

// GetLimitedSupportStatus retrieves and displays the limited support status for a cluster.
// It checks the cluster's limited support reason count and displays the appropriate status.
// Returns the count as a string and prints the status to stdout.
func GetLimitedSupportStatus(clusterID string) string {
	clusterInfo, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return "Error retrieving cluster info: " + err.Error()
	}
	if clusterInfo.Status().LimitedSupportReasonCount() != 0 {
		fmt.Printf("%-25s %s", "Limited Support Status: ", "Limited Support\n")
	} else {
		fmt.Printf("%-25s %s", "Limited Support Status: ", "Fully Supported\n")
	}
	return fmt.Sprintf("%d", clusterInfo.Status().LimitedSupportReasonCount())
}

// printClusterField prints a cluster field with consistent formatting.
// It uses a fixed width for field names to ensure aligned output.
func printClusterField(fieldName string, value interface{}) {
	fmt.Printf("%-25s %v\n", fieldName, value)
}
