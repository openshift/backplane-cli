// This file contains information about backplane-cli.

package info

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

const (
	BACKPLANE_URL_ENV_NAME             = "BACKPLANE_URL"
	BACKPLANE_PROXY_ENV_NAME           = "HTTPS_PROXY"
	BACKPLANE_CONFIG_PATH_ENV_NAME     = "BACKPLANE_CONFIG"
	BACKPLANE_CONFIG_DEFAULT_FILE_PATH = ".config/backplane"
	BACKPLANE_CONFIG_DEFAULT_FILE_NAME = "config.json"

	// GitHub API get fetch the latest tag
	UpstreamReleaseAPI = "https://api.github.com/repos/openshift/backplane-cli/releases/latest"

	// Upstream git module
	UpstreamGitModule = "https://github.com/openshift/backplane-cli/cmd/ocm-backplane"

	// GitHub README page
	UpstreamREADMETemplate = "https://github.com/openshift/backplane-cli/-/blob/%s/README.md"

	VersionAddressTemplate = "https://github.com/openshift/backplane-cli/releases/download/v%s/backplane-cli_%s_%s_%s.tar.gz" // version, version, GOOS, GOARCH

	// GitHub Host
	GitHubHost = "github.com"
)

var (

	GitCommit string


	Version string
)


type gitHubResponse struct {
	TagName string `json:"tag_name"`
}


func GetLatestVersion() (latest string, err error) {
	client := http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest(http.MethodGet, UpstreamReleaseAPI, nil)
	if err != nil {
		return latest, err
	}

	res, err := client.Do(req)
	if err != nil {
		return latest, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return latest, err
	}

	githubResp := gitHubResponse{}
	err = json.Unmarshal(body, &githubResp)
	if err != nil {
		return latest, err
	}

	return githubResp.TagName, nil
}
