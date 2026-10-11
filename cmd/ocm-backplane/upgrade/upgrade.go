package upgrade

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift/backplane-cli/internal/github"
	"github.com/openshift/backplane-cli/internal/upgrade"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/spf13/cobra"
)

func long() string {
	return strings.Join([]string{
		"Upgrades the latest version release based on",
		"your machine's OS and architecture.",
	}, " ")
}

var UpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the current backplane-cli to the latest version",
	Long:  long(),

	RunE: runUpgrade,
	Args: cobra.ArbitraryArgs,

	SilenceUsage: true,
}

func runUpgrade(cmd *cobra.Command, _ []string) error {

	if err := validateReleaseVersion(info.Version); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	git := github.NewClient()

	if err := git.CheckConnection(); err != nil {
		return fmt.Errorf("checking connection to the git server: %w", err)
	}

	upgrade := upgrade.NewCmd(git)

	return upgrade.UpgradePlugin(ctx, info.Version)
}

// validateReleaseVersion ensures the binary carries the release version metadata
// that the native self-upgrade relies on for the SemVer comparison. This metadata
// is injected via ldflags only when building official release binaries, so a CLI
// built without that injection (commonly a local source build) has an empty
// version and cannot self-upgrade.
func validateReleaseVersion(version string) error {
	if version == "" {
		return fmt.Errorf("release version metadata is missing; this commonly happens when the CLI is built from source without release version injection. " +
			"Native self-upgrade requires release version metadata. " +
			"Install or update Backplane CLI through an official release channel")
	}

	return nil
}
