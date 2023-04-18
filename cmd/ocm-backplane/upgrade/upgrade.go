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

	latest, err := info.GetLatestVersion()
	if err != nil && info.Version != latest {
		fmt.Print("You are using the old version of backplane-cli, please upgrade to latest vesrion using 'ocm-backplane upgrade' .")
	}
	upgrade := upgrade.NewCmd(git)

	return upgrade.UpgradePlugin(ctx, info.Version)
}
