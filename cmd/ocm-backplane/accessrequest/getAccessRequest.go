package accessrequest

import (
	"fmt"

	"github.com/openshift/backplane-cli/pkg/accessrequest"

	ocmcli "github.com/openshift-online/ocm-cli/pkg/ocm"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// newGetAccessRequestCmd returns cobra command
func newGetAccessRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "get",
		Short:         "Get the active (pending or approved) access request",
		Args:          cobra.ExactArgs(0),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runGetAccessRequest,
	}

	return cmd
}

// runGetAccessRequest retrieves the active access request and print it
func runGetAccessRequest(cmd *cobra.Command, args []string) error {
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
		return err
	}

	if accessRequest == nil {
		logger.Warnf("no pending or approved access request for cluster '%s'", clusterID)
		fmt.Printf("To get denied or expired access requests, run: ocm get /api/access_transparency/v1/access_requests -p search=\"cluster_id='%s'\"\n", clusterID)
	} else {
		accessrequest.PrintAccessRequest(clusterID, accessRequest)
	}

	return nil
}
