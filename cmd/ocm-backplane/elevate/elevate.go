package elevate

import (
	"github.com/openshift/backplane-cli/pkg/elevate"
	"github.com/spf13/cobra"
)

var ElevateCmd = &cobra.Command{
	Use:   "elevate [<REASON> [<COMMAND>]]",
	Short: "Give a justification for elevating privileges to backplane-cluster-admin and attach it to your user object",
	Long: `Elevate to backplane-cluster-admin, and give a reason to do so.
This will then be forwarded to your audit collection backend of your choice as the 'Impersonate-User-Extra' HTTP header, which can then be used for tracking, compliance, and security reasons.
The command creates a temporary kubeconfig and clusterrole for your user, to allow you to add the extra header to your Kube API request.
The provided reason will be store for 20 minutes in order to be used by future elevate commands if the next provided reason is empty.
If the provided reason is empty and no elevation with reason has been done in the last 20 min, and if also the stdin and stderr are not redirection,
then a prompt will be done to enter a none empty reason that will be also stored for future elevation.
If no COMMAND (and eventualy also REASON) is/are provided then the command will just be used to initialize elevate context for future elevate command.`,
	Example:      "ocm backplane elevate <reason> -- get po -A",
	RunE:         runElevate,
	SilenceUsage: true,
}

func runElevate(cmd *cobra.Command, argv []string) error {
	return elevate.RunElevate(argv)
}
