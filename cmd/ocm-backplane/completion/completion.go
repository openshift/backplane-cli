package completion

import (
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	completionExample = templates.Examples(`
		# Installing bash completion on macOS using homebrew
		## If running Bash 3.2 included with macOS
		    brew install bash-completion
		## or, if running Bash 4.1+
		    brew install bash-completion@2


		# Installing bash completion on Linux
		## If bash-completion is not installed on Linux, please install the 'bash-completion' package
		## via your distribution's package manager.
		## Load the ocm-backplane completion code for bash into the current shell
		    source <(ocm-backplane completion bash)
		## Write bash completion code to a file and source if from .bash_profile
		    ocm-backplane completion bash > ~/.completion.bash.inc
		    printf "
		      # ocm-backplane shell completion
		      source '$HOME/.completion.bash.inc'
		      " >> $HOME/.bash_profile
		    source $HOME/.bash_profile


		# Load the ocm-backplane completion code for zsh[1] into the current shell
		    source <(ocm-backplane completion zsh)
		# Set the ocm-backplane completion code for zsh[1] to autoload on startup
		    ocm-backplane completion zsh > "${fpath[1]}/_ocm-backplane"`)
)

func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "completion SHELL",
		Short:             "Output shell completion code for the specified shell (bash or zsh)",
		Example:           completionExample,
		DisableAutoGenTag: true,
		Args:              cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:         []string{"bash", "zsh"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Parent().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Parent().GenZshCompletion(cmd.OutOrStdout())
			default:
				return cmdutil.UsageErrorf(cmd, "Unsupported shell type %q.", args[0])
			}
		},
	}
	return cmd
}
