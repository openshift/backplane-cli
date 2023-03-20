package elevate

import (
	"github.com/openshift/backplane-cli/pkg/elevate"
	"github.com/spf13/cobra"
)

var ElevateCmd = &cobra.Command{
	Use:          "elevate <REASON> <COMMAND>",
	Short:        "Give a justification for elevating privileges to backplane-cluster-admin and attach it to your user object",
	Long:         `Elevate to backplane-cluster-admin, and give a reason to do so. This will then be forwarded to your audit collection backend of your choice as the 'Impersonate-User-Extra' HTTP header, which can then be used for tracking, compliance, and security reasons. The command creates a temporary kubeconfig and clusterrole for your user, to allow you to add the extra header to your Kube API request.`,
	Example:      "ocm backplane elevate <reason> -- get po -A",
	Args:         cobra.MinimumNArgs(2),
	RunE:         runElevate,
	SilenceUsage: true,
}

func runElevate(cmd *cobra.Command, argv []string) error {
	return elevate.RunElevate(argv)
}
