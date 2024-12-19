package main

import (
	"fmt"
	"os"

	"github.com/openshift/backplane-cli/cmd/ocm-backplane/console"
	logger "github.com/sirupsen/logrus"
)

func main() {
	// Set the logging level to Debug
	logger.SetLevel(logger.DebugLevel)

	// Your existing initialization code
	rootCmd := console.NewConsoleCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
