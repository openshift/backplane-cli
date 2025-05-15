/*
Copyright © 2020 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/openshift/backplane-cli/cmd/ocm-backplane/accessrequest"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/cloud"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/config"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/console"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/elevate"
	healthcheck "github.com/openshift/backplane-cli/cmd/ocm-backplane/healthcheck"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/logout"
	managedjob "github.com/openshift/backplane-cli/cmd/ocm-backplane/managedJob"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/monitoring"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/remediation"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/script"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/session"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/status"
	testjob "github.com/openshift/backplane-cli/cmd/ocm-backplane/testJob"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/upgrade"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/version"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ocm-backplane",
	Short: "backplane plugin for OCM",
	Long: `This is a binary for backplane plugin.
       The current function ocm-backplane provides is to login a cluster,
       which get a proxy url from backplane for the target cluster.
	   After login, users can use oc command to operate the target cluster`,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Errorln(err.Error())
		os.Exit(1)
	}
}

func init() {
	// Add Verbosity flag for all commands
	globalflags.AddVerbosityFlag(rootCmd)

	// Add the --govcloud flag as a global flag
	rootCmd.PersistentFlags().Bool("govcloud", false, "Enable GovCloud mode")

	// Bind the flag to Viper for global access
	viper.BindPFlag("govcloud", rootCmd.PersistentFlags().Lookup("govcloud"))

	// Register sub-commands
	rootCmd.AddCommand(accessrequest.NewAccessRequestCmd())
	rootCmd.AddCommand(console.NewConsoleCmd())
	rootCmd.AddCommand(config.NewConfigCmd())
	rootCmd.AddCommand(cloud.CloudCmd)
	rootCmd.AddCommand(elevate.ElevateCmd)
	rootCmd.AddCommand(login.LoginCmd)
	rootCmd.AddCommand(logout.LogoutCmd)
	rootCmd.AddCommand(managedjob.NewManagedJobCmd())
	rootCmd.AddCommand(script.NewScriptCmd())
	rootCmd.AddCommand(status.StatusCmd)
	rootCmd.AddCommand(session.NewCmdSession())
	rootCmd.AddCommand(testjob.NewTestJobCommand())
	rootCmd.AddCommand(upgrade.UpgradeCmd)
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(monitoring.MonitoringCmd)
	rootCmd.AddCommand(healthcheck.HealthCheckCmd)
	rootCmd.AddCommand(remediation.NewRemediationCmd())
}
