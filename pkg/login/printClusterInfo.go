package login

import (
	"fmt"

	"github.com/openshift/backplane-cli/pkg/ocm"
	logger "github.com/sirupsen/logrus"
)

//displayClusterInfo retrieves and displays basic information about the target cluster.

func PrintClusterInfo(clusterID string) error {
	logger := logger.WithField("clusterID", clusterID)

	// Retrieve cluster information
	clusterInfo, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return fmt.Errorf("error retrieving cluster info: %v", err)
	}

	// Display cluster information
	fmt.Printf("\n%-25s %s\n", "Cluster ID:", clusterInfo.ID())
	fmt.Printf("%-25s %s\n", "Cluster Name:", clusterInfo.Name())
	fmt.Printf("%-25s %s\n", "Cluster Status:", clusterInfo.State())
	fmt.Printf("%-25s %s\n", "Cluster Region:", clusterInfo.Region().ID())
	fmt.Printf("%-25s %s\n", "Cluster Provider:", clusterInfo.CloudProvider().ID())
	fmt.Printf("%-25s %t\n", "Hypershift Enabled:", clusterInfo.Hypershift().Enabled())
	fmt.Printf("%-25s %s\n", "Version:", clusterInfo.OpenshiftVersion())
	GetLimitedSupportStatus(clusterID)
	PrintAccessProtectionStatus(clusterID)

	logger.Info("Basic cluster information displayed.")
	return nil

}

// PrintAccessProtectionStatus retrieves and displays the access protection status of the target cluster.

func PrintAccessProtectionStatus(clusterID string) {
	ocmConnection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		fmt.Println("Error setting up OCM connection: ", err)
		return
	}
	if ocmConnection != nil {
		defer ocmConnection.Close()
	}

	enabled, _ := ocm.DefaultOCMInterface.IsClusterAccessProtectionEnabled(ocmConnection, clusterID)
	if enabled {
		fmt.Printf("%-25s %s\n", "Access Protection:", "Enabled\n")
	} else {
		fmt.Printf("%-25s %s\n", "Access Protection:", "Disabled\n")
	}

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
