package globalflags

import (
	"github.com/spf13/cobra"
)

// GlobalOptions defines all available commands
type GlobalOptions struct {
	BackplaneURL string
	ProxyURL     string
}

func AddGlobalFlags(cmd *cobra.Command, opts *GlobalOptions) {
	cmd.PersistentFlags().StringVar(
		&opts.BackplaneURL,
		"url",
		"",
		"URL of backplane API",
	)
	cmd.PersistentFlags().StringVar(
		&opts.ProxyURL,
		"proxy",
		"",
		"URL of HTTPS proxy",
	)
}
