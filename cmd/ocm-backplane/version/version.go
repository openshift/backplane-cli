package version

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift/backplane-cli/pkg/info"
)

// versionResponse is necessary for the JSON version response. It uses the three
// variables that get set during the build.
type versionResponse struct {
	Commit  string `json:"commit"`
	Version string `json:"version"`
	Latest  string `json:"latest"`
}


var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version",
	Long:  "Prints the version number of Backplane CLI",
	RunE:  runVersion,
}

func runVersion(cmd *cobra.Command, args []string) error {
	gitCommit := "unknown"

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				gitCommit = setting.Value
				break
			}
		}
	}

	latest, _ := info.GetLatestVersion() 
	ver, err := json.MarshalIndent(&versionResponse{
		Commit:  gitCommit,
		Version: info.Version,
		Latest:  strings.TrimPrefix(latest, "v"),
	}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(ver))
	return nil
}


