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

func GetAccessProtectionStatus(clusterID string) string {
	ocmConnection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		logger.Error("Error setting up OCM connection: ", err)
		return "Error setting up OCM connection: " + err.Error()
	}
	if ocmConnection != nil {
		defer ocmConnection.Close()
	}

	accessProtectionStatus := "Disabled"

	if !(viper.GetBool("govcloud")){
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

func printClusterField(fieldName string, value interface{}) {
	fmt.Printf("%-25s %v\n", fieldName, value)
}
