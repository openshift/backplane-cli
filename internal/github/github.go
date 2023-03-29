package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/openshift/backplane-cli/internal/upgrade"
	"github.com/pkg/errors"
)

var (
	ErrRequestFailed   = errors.New("request failed")
	ErrArchiveNotFound = errors.New("archive not found")
)

const (
	gitHubApiEndPoint = "https://api.github.com/repos/openshift/backplane-cli"
	assetTemplateName = "ocm-backplane_%s_%s_%s.tar.gz" // version, GOOS, GOARCH
)

func NewClient(opts ...ClientOption) *Client {
	var cfg ClientConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Client{
		cfg: cfg,
	}
}

// GetLatestVersion returns latest version from the github API
func (c *Client) GetLatestVersion(ctx context.Context) (latest upgrade.Release, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	data, err := c.get(
		ctx,
		fmt.Sprintf("%s/releases/latest", c.cfg.BaseURL),
	)
	if err != nil {
		return latest, err
	}

	// Release type filters the latest release tag name and asserts
	githubReleaseResponse := upgrade.Release{}
	err = json.Unmarshal(data, &githubReleaseResponse)
	if err != nil {
		return latest, err
	}

	return githubReleaseResponse, nil
}

// GetReleaseArchive returns archive based on the OS type and arc
func (c *Client) GetReleaseArchive(ctx context.Context, latestVersion upgrade.Release) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	osConfig := OSConfig{
		OSType: runtime.GOOS,
		OSArch: runtime.GOARCH,
	}

	// Find browser download url based on OS type and arc
	assetURL, ok := osConfig.FindAssetURL(latestVersion)
	if !ok {
		return nil, ErrArchiveNotFound
	}

	archiveData, err := c.get(ctx, assetURL)
	if err != nil {
		return nil, fmt.Errorf("requesting release archive: %w", err)
	}

	return archiveData, nil
}

// Get data from the API providing the context and base url
// Context allowed to cancel when calling to the API
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	res, err := c.cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("running request for %q: %w", url, err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, ErrRequestFailed
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	return data, nil
}

// CheckConnection is validating connection to the API end point
func (c *Client) CheckConnection() error {
	url, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("parsing base url: %w", err)
	}

	if _, err := net.LookupIP(url.Host); err != nil {
		return fmt.Errorf("looking up host %q: %w", url.Host, err)
	}

	return nil
}

// Client defines the configuration for GitHub client
type Client struct {
	cfg ClientConfig
}

// ClientConfig defines http client and base url for the API
type ClientConfig struct {
	Client  http.Client
	BaseURL string
}

func (c *ClientConfig) Option(opts ...ClientOption) {
	for _, opt := range opts {
		opt.ConfigureClient(c)
	}
}
func (c *ClientConfig) Default() {
	if c.BaseURL == "" {
		c.BaseURL = gitHubApiEndPoint
	}
}

type ClientOption interface {
	ConfigureClient(*ClientConfig)
}

// OSConfig defines os type and arch
type OSConfig struct {
	OSType string
	OSArch string
}

// FindAssetURL returns the matching browser download url and matching status
func (osConfig *OSConfig) FindAssetURL(latestVersion upgrade.Release) (string, bool) {
	for _, asset := range latestVersion.Assets {
		if osConfig.isMatchingArchive(asset, latestVersion.TagName) {
			return asset.DownloadUrl, true
		}

	}

	return "", false
}

// isMatchingArchive checks asset name match with mapped OS type and arch
func (osConfig *OSConfig) isMatchingArchive(asset upgrade.ReleaseAsset, version string) bool {
	name := fmt.Sprintf(
		assetTemplateName,
		strings.TrimPrefix(version, "v"),
		mapOS(osConfig.OSType),
		mapArch(osConfig.OSArch),
	)

	return asset.Name == name
}

// mapArch returns desired arch name
func mapArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}

// mapOS returns matching lowercase OS type name
func mapOS(goos string) string {
	switch goos {
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	case "windows":
		return "Windows"
	default:
		return ""
	}
}
