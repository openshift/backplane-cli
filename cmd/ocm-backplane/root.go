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
	"strings"

	"github.com/openshift/backplane-cli/cmd/ocm-backplane/cloud"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/config"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/console"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/elevate"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/logout"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/managedJob"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/monitoring"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/script"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/session"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/status"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/testJob"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/upgrade"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Created so that multiple inputs can be accepted
type levelFlag log.Level

func (l *levelFlag) String() string {
	// change this, this is just can example to satisfy the interface
	return log.Level(*l).String()
}

func (l *levelFlag) Set(value string) error {
	lvl, err := log.ParseLevel(strings.TrimSpace(value))
	if err == nil {
		*l = levelFlag(lvl)
	}
	return err
}

func (l *levelFlag) Type() string {
	return "string"
}

var (
	// some defaults for configuration
	defaultLogLevel = log.WarnLevel.String()
	logLevel        levelFlag
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
	// Set default log level
	_ = logLevel.Set(defaultLogLevel)
	logLevelFlag := rootCmd.PersistentFlags().VarPF(&logLevel, "verbosity", "v", "Verbosity level: panic, fatal, error, warn, info, debug. Providing no level string will select info.")
	logLevelFlag.NoOptDefVal = log.InfoLevel.String()

	// Register sub-commands
	rootCmd.AddCommand(console.ConsoleCmd)
	rootCmd.AddCommand(config.NewConfigCmd())
	rootCmd.AddCommand(cloud.CloudCmd)
	rootCmd.AddCommand(elevate.ElevateCmd)
	rootCmd.AddCommand(login.LoginCmd)
	rootCmd.AddCommand(logout.LogoutCmd)
	rootCmd.AddCommand(managedJob.NewManagedJobCmd())
	rootCmd.AddCommand(script.NewScriptCmd())
	rootCmd.AddCommand(status.StatusCmd)
	rootCmd.AddCommand(session.NewCmdSession())
	rootCmd.AddCommand(testJob.NewTestJobCommand())
	rootCmd.AddCommand(upgrade.UpgradeCmd)
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(monitoring.MonitoringCmd)
}
