package globalflags

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type levelFlag log.Level

var (
	// some defaults for configuration
	defaultLogLevel = log.WarnLevel.String()
	logLevel        levelFlag
)

// String returns log level string
func (l *levelFlag) String() string {
	return log.Level(*l).String()
}

// Set updates the log level
func (l *levelFlag) Set(value string) error {
	lvl, err := log.ParseLevel(strings.TrimSpace(value))
	if err == nil {
		*l = levelFlag(lvl)
	}
	log.SetLevel(lvl)
	return err
}

// Type defines log level type
func (l *levelFlag) Type() string {
	return "string"
}

// AddVerbosityFlag add Persistent verbosity flag
func AddVerbosityFlag(cmd *cobra.Command) {
	// Set default log level
	_ = logLevel.Set(defaultLogLevel)
	logLevelFlag := cmd.PersistentFlags().VarPF(
		&logLevel,
		"verbosity",
		"v",
		"Verbosity level: panic, fatal, error, warn, info, debug",
	)
	logLevelFlag.NoOptDefVal = log.InfoLevel.String()
}
