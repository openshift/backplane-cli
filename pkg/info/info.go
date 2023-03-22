// This file contains information about backplane-cli.

package info

import (
	"fmt"
)

const (
	// Version of the backplane-cli
	Version = "0.0.0"

	BACKPLANE_URL_ENV_NAME             = "BACKPLANE_URL"
	BACKPLANE_PROXY_ENV_NAME           = "HTTPS_PROXY"
	BACKPLANE_CONFIG_PATH_ENV_NAME     = "BACKPLANE_CONFIG"
	BACKPLANE_CONFIG_DEFAULT_FILE_NAME = ".backplane.json"

	// GitHub API get fetch the latest tag
	UpstreamReleaseAPI = "https://api.github.com/repos/openshift/backplane-cli/releases/latest"

	// Upstream git module
	UpstreamGitModule = "https://github.com/openshift/backplane-cli/cmd/ocm-backplane"

	// GitHub README page
	UpstreamREADMETemplate = "https://github.com/openshift/backplane-cli/-/blob/%s/README.md"

	// GitHub Host
	GitHubHost = "github.com"
)

var UpstreamREADMETagged = fmt.Sprintf(UpstreamREADMETemplate, Version)
