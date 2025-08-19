// This file contains information about backplane-cli.

package info

import (
	"fmt"
	"strings"
)

const (
	// Environment Variables
	BackplaneURLEnvName        = "BACKPLANE_URL"
	BackplaneProxyEnvName      = "HTTPS_PROXY"
	BackplaneAWSProxyEnvName   = "BACKPLANE_AWS_PROXY"
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

	// Nginx configuration template for monitoring-plugin
	MonitoringPluginNginxConfigTemplate = `
	error_log /dev/stdout info;
	events {}
	http {
  	include            /etc/nginx/mime.types;
  	default_type       application/octet-stream;
  	keepalive_timeout  65;
  	server {
    	listen              %s;
    	root                /usr/share/nginx/html;
  	}
	}
	`

	MonitoringPluginNginxConfigFilename = "monitoring-plugin-nginx-%s.conf"
)

var (
	// Version of the backplane-cli
	// This will be set via Goreleaser during the build process
	Version string

	UpstreamREADMETagged = fmt.Sprintf(UpstreamREADMETemplate, Version)
)

type InfoService interface {
	// get the current binary version from available sources
	GetVersion() string
}

type DefaultInfoServiceImpl struct {
}

func (i *DefaultInfoServiceImpl) GetVersion() string {
	// If the Version is set by Goreleaser, return it directly.
	if Version != "" {
		return Version
	}

	// otherwise, return the build info from Go build if available.
	buildInfo, available := DefaultBuildInfoService.GetBuildInfo()
	if available {
		return strings.TrimLeft(buildInfo.Main.Version, "v")
	}

	return "unknown"
}

var DefaultInfoService InfoService = &DefaultInfoServiceImpl{}
