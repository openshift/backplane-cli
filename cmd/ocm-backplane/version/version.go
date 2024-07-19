package version

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift/backplane-cli/pkg/info"
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version",
	Long:  `Display the version of Backplane CLI`,
	RunE:  runVersion,
}

func runVersion(cmd *cobra.Command, argv []string) error {
	// Print the version
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", info.DefaultInfoService.GetVersion())

	return nil
}
