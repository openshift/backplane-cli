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
	Short: "Upgrades the current backplane-cli to the latest version",
	Long:  long(),

	RunE: runUpgrade,
	Args: cobra.ArbitraryArgs,

	SilenceUsage: true,
}

func runUpgrade(cmd *cobra.Command, _ []string) error {

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	git := github.NewClient()

	if err := git.CheckConnection(); err != nil {
		return fmt.Errorf("checking connection to the git server: %w", err)
	}

	// Get the latest version from the GitHub API
	latestVersion, err := git.GetLatestVersion(ctx)
	if err != nil {
		return err
	}

	// Check if the local version is already up-to-date
	if latestVersion.TagName == info.Version {
		fmt.Printf("Already up-to-date. Current version: %s\n", info.Version)
		return nil
	}
	
	fmt.Printf("A new version (%s) of Backplane CLI is available.\n", latestVersion.TagName)
	fmt.Println("Please run the command 'ocm backplane upgrade' to upgrade to the latest version.")
	
	upgrade := upgrade.NewCmd(git)

	return upgrade.UpgradePlugin(ctx, latestVersion.TagName)
}
