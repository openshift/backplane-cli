package rw

import (
	"fmt"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
	"github.com/openshift/backplane-cli/pkg/utils"
)

// RwCmd represents the rw command
var RwCmd = &cobra.Command{
	Use:   "rw",
	Short: "Re-login to the current cluster with read-write access",
	Long: `Re-login to the current cluster with read-write access.

This command is a helper that detects the currently logged-in cluster from your
kubeconfig and re-authenticates with read-write permissions. This is equivalent
to running 'ocm-backplane login --rw <cluster-id>' but without needing to specify
the cluster ID again.`,
	Example:      "ocm backplane rw",
	RunE:         runRw,
	SilenceUsage: true,
}

func runRw(cmd *cobra.Command, argv []string) error {
	logger.Debugln("Getting current cluster from kubeconfig")

	// Get the current cluster from kubeconfig
	clusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
	if err != nil {
		return fmt.Errorf("failed to get current cluster from kubeconfig: %w\nPlease make sure you are logged in to a cluster first", err)
	}

	logger.WithField("ClusterID", clusterInfo.ClusterID).Infoln("Re-logging in with read-write access")

	// Set the --rw flag for login
	err = login.LoginCmd.Flags().Set("rw", "true")
	if err != nil {
		return fmt.Errorf("failed to set rw flag: %w", err)
	}

	// Ensure we reset the flag after execution to avoid side effects
	defer func() {
		_ = login.LoginCmd.Flags().Set("rw", "false")
	}()

	// Execute login command with the current cluster ID
	err = login.LoginCmd.RunE(cmd, []string{clusterInfo.ClusterID})
	if err != nil {
		return fmt.Errorf("failed to re-login with read-write access: %w", err)
	}

	fmt.Printf("Successfully re-logged in to cluster %s with read-write access\n", clusterInfo.ClusterID)

	return nil
}
