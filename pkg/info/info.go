// This file contains information about backplane-cli.

package info

import (
	"fmt"
)

const (
	// Environment Variables
	BACKPLANE_URL_ENV_NAME         = "BACKPLANE_URL"
	BACKPLANE_PROXY_ENV_NAME       = "HTTPS_PROXY"
	BACKPLANE_CONFIG_PATH_ENV_NAME = "BACKPLANE_CONFIG"

	// Configuration
	BACKPLANE_CONFIG_DEFAULT_FILE_PATH = ".config/backplane"
	BACKPLANE_CONFIG_DEFAULT_FILE_NAME = "config.json"

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
