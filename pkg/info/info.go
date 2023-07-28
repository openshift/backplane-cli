// This file contains information about backplane-cli.

package info

import (
	"fmt"
)

const (
	// Environment Variables
	BackplaneURLEnvName        = "BACKPLANE_URL"
	BackplaneProxyEnvName      = "HTTPS_PROXY"
	BackplaneConfigPathEnvName = "BACKPLANE_CONFIG"
	BackplaneKubeconfigEnvName = "KUBECONFIG"

	// Configuration
	BackplaneConfigDefaultFilePath = ".config/backplane"
	BackplaneConfigDefaultFileName = "config.json"

	// Session
	BackplaneDefaultSessionDirectory = "backplane"

	// GitHub API get fetch the latest tag
	UpstreamReleaseAPI = "https://api.github.com/repos/openshift/backplane-cli/releases/latest"

	// Upstream git module
	UpstreamGitModule = "https://github.com/openshift/backplane-cli/cmd/ocm-backplane"

	// GitHub README page
	UpstreamREADMETemplate = "https://github.com/openshift/backplane-cli/-/blob/%s/README.md"

	// GitHub Host
	GitHubHost = "github.com"
)

var (
	// Version of the backplane-cli
	// This will be set via Goreleaser during the build process
	Version string

	UpstreamREADMETagged = fmt.Sprintf(UpstreamREADMETemplate, Version)
)
