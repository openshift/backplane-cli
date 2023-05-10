package session

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/backplane-cli/pkg/cli/session"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/spf13/cobra"
)

func NewCmdSession() *cobra.Command {
	options := session.Options{}

	session := session.BackplaneSession{
		Options: &options,
	}
	sessionCmd := &cobra.Command{
		Use:               "session [flags] [session-alias]",
		Short:             "Create an isolated environment to interact with a cluster in its own directory.",
		Args:              cobra.MaximumNArgs(1),
		DisableAutoGenTag: true,
		RunE:              session.RunCommand,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			validEnvs := []string{}
			files, err := os.ReadDir(filepath.Join(os.Getenv("HOME"), info.BACKPLANE_DEFAULT_SESSION_DIRECTORY))
			if err != nil {
				return validEnvs, cobra.ShellCompDirectiveNoFileComp
			}
			for _, f := range files {
				if f.IsDir() && strings.HasPrefix(f.Name(), toComplete) {
					validEnvs = append(validEnvs, f.Name())
				}
			}

			return validEnvs, cobra.ShellCompDirectiveNoFileComp
		},
	}
	sessionCmd.Flags().BoolVarP(
		&options.DeleteSession,
		"delete",
		"d",
		false,
		"Delete session",
	)

	sessionCmd.Flags().StringVarP(
		&options.ClusterId,
		"cluster-id",
		"c",
		"",
		"The cluster to create the session for",
	)

	return sessionCmd
}
