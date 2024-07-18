package accessrequest

import (
	"fmt"

	"github.com/openshift/backplane-cli/pkg/accessrequest"

	ocmcli "github.com/openshift-online/ocm-cli/pkg/ocm"
	"github.com/spf13/cobra"
)

// newExpireAccessRequestCmd returns cobra command
func newExpireAccessRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "expire",
		Short:         "Expire the active (pending or approved) access request",
		Args:          cobra.ExactArgs(0),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runExpireAccessRequest,
	}

	return cmd
}

// runExpireAccessRequest retrieves the active access request and expire it
func runExpireAccessRequest(cmd *cobra.Command, args []string) error {
	clusterID, err := accessrequest.GetClusterID(cmd)
	if err != nil {
		return fmt.Errorf("failed to compute cluster ID: %v", err)
	}

	ocmConnection, err := ocmcli.NewConnection().Build()
	if err != nil {
		return fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer ocmConnection.Close()

	accessRequest, err := accessrequest.GetAccessRequest(ocmConnection, clusterID)

	if err != nil {
		return fmt.Errorf("failed to retrieve access request: %v", err)
	}

	if accessRequest == nil {
		return fmt.Errorf("no pending or approved access request for cluster '%s'", clusterID)
	}

	err = accessrequest.ExpireAccessRequest(ocmConnection, accessRequest)
	if err != nil {
		return err
	}

	fmt.Printf("Access request '%s' has been expired\n", accessRequest.HREF())

	return nil
}
