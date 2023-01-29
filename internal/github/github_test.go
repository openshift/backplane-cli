package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift/backplane-cli/internal/github"
	"github.com/openshift/backplane-cli/internal/upgrade"
)

func TestGetLatestVersion(t *testing.T) {

	for name, tc := range map[string]struct {
		Actual          upgrade.Release
		ExpectedVersion string
		ExpectedAssets  []upgrade.ReleaseAsset
	}{
		"no releases versions": {
			Actual: upgrade.Release{
				TagName: "",
				Assets:  []upgrade.ReleaseAsset{},
			},
			ExpectedVersion: "",
			ExpectedAssets:  []upgrade.ReleaseAsset{},
		},
		"latest releases versions": {
			Actual: upgrade.Release{
				TagName: "v0.0.1",
				Assets:  []upgrade.ReleaseAsset{},
			},
			ExpectedVersion: "v0.0.1",
			ExpectedAssets:  []upgrade.ReleaseAsset{},
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {

			t.Parallel()

			expected := tc.Actual
			data, _ := json.Marshal(tc.Actual)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(data)
			}))

			defer srv.Close()

			client := github.NewClient(
				github.WithBaseURL(srv.URL),
				github.WithClient(*srv.Client()),
			)
			res, err := client.GetLatestVersion(context.Background())
			if err != nil {
				t.Errorf("expected err to be nil got %v", err)
			}
			if res.TagName != tc.ExpectedVersion {
				t.Errorf("expected res to be %s got %s", expected, res)
			}

		})
	}
}

func TestFindAssetURL(t *testing.T) {

	for name, tc := range map[string]struct {
		LatestRelease upgrade.Release
		osConfig      github.OSConfig
		match         bool
		ExpectedAsert string
	}{
		"no marching assert": {
			LatestRelease: upgrade.Release{
				TagName: "",
				Assets:  []upgrade.ReleaseAsset{},
			},
			osConfig:      github.OSConfig{},
			match:         false,
			ExpectedAsert: "",
		},
		"check matching asert for Mac": {
			LatestRelease: upgrade.Release{
				TagName: "v0.0.1",
				Assets: []upgrade.ReleaseAsset{
					{
						Name:        "backplane-cli_0.0.1_Darwin_arm64.tar.gz",
						DownloadUrl: "https://github.com/openshift/backplane-cli/releases/download/v0.0.1/backplane-cli_0.0.1_Darwin_arm64.tar.gz",
					},
				},
			},
			osConfig: github.OSConfig{
				OSType: "darwin",
				OSArch: "arm64",
			},
			match:         true,
			ExpectedAsert: "https://github.com/openshift/backplane-cli/releases/download/v0.0.1/backplane-cli_0.0.1_Darwin_arm64.tar.gz",
		},
		"check matching asert for Linux": {
			LatestRelease: upgrade.Release{
				TagName: "v0.0.1",
				Assets: []upgrade.ReleaseAsset{
					{
						Name:        "backplane-cli_0.0.1_Linux_arm64.tar.gz",
						DownloadUrl: "https://github.com/openshift/backplane-cli/releases/download/v0.0.1/backplane-cli_0.0.1_Linux_arm64.tar.gz",
					},
				},
			},
			osConfig: github.OSConfig{
				OSType: "linux",
				OSArch: "arm64",
			},
			match:         true,
			ExpectedAsert: "https://github.com/openshift/backplane-cli/releases/download/v0.0.1/backplane-cli_0.0.1_Linux_arm64.tar.gz",
		},
		"check asert for unsupported OS ": {
			LatestRelease: upgrade.Release{
				TagName: "v0.0.1",
				Assets: []upgrade.ReleaseAsset{
					{
						Name:        "backplane-cli_0.0.1_Linux_arm64.tar.gz",
						DownloadUrl: "https://github.com/openshift/backplane-cli/releases/download/v0.0.1/backplane-cli_0.0.1_Linux_arm64.tar.gz",
					},
					{
						Name:        "backplane-cli_0.0.1_Darwin_arm64.tar.gz",
						DownloadUrl: "https://github.com/openshift/backplane-cli/releases/download/v0.0.1/backplane-cli_0.0.1_Darwin_arm64.tar.gz",
					},
				},
			},
			osConfig: github.OSConfig{
				OSType: "windows",
				OSArch: "arm64",
			},
			match:         false,
			ExpectedAsert: "",
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {

			t.Parallel()
			downloadUrl, assertMatch := tc.osConfig.FindAssetURL(tc.LatestRelease)

			if assertMatch != tc.match {
				t.Errorf("expected res to be %t got %t", assertMatch, tc.match)
			}

			if downloadUrl != tc.ExpectedAsert {
				t.Errorf("expected res to be %s got %s", tc.ExpectedAsert, downloadUrl)
			}

		})
	}
}
