package globalflags

import (
	"github.com/spf13/cobra"
)

// GlobalOptions defines all available commands
type GlobalOptions struct {
	BackplaneURL string
	ProxyURL     string
	Manager      bool
	Service      bool
}

func AddGlobalFlags(cmd *cobra.Command, opts *GlobalOptions) {
	cmd.PersistentFlags().StringVar(
		&opts.BackplaneURL,
		"url",
		"",
		"URL of backplane API. Must be an HTTPS URL.",
	)
	cmd.PersistentFlags().StringVar(
		&opts.ProxyURL,
		"proxy",
		"",
		"URL of HTTPS proxy",
	)
	cmd.PersistentFlags().BoolVar(
		&opts.Manager,
		"manager",
		false,
		"Login to management cluster instead of the cluster itself.",
	)
	cmd.PersistentFlags().BoolVar(
		&opts.Service,
		"service",
		false,
		"Login to service cluster for the given hosted cluster or management cluster.",
	)
}
