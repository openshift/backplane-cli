package upgrade

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
)

// Release type defines the tag and its assets
type Release struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// ReleaseAsset defines the name and download url
type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// GitServer defines generic functions for getting the latest release and archive
type GitServer interface {
	GetLatestVersion(ctx context.Context) (Release, error)
	GetReleaseArchive(ctx context.Context, release Release) ([]byte, error)
}

type Writer interface {
	Write(path string, data []byte) error
}

func NewCmd(git GitServer, opts ...CmdOption) *Cmd {
	var cfg CmdConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Cmd{
		cfg: cfg,

		git: git,
	}
}

type Cmd struct {
	cfg CmdConfig

	git GitServer
}

// UpgradePlugin upgrade OS binary based on the latest version
func (c *Cmd) UpgradePlugin(ctx context.Context, currentVersion string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	latestVersion, err := c.git.GetLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting latest version for project '%s/%s': %w", c.cfg.Org, c.cfg.Repo, err)
	}

	proceed, err := shouldUpdate(currentVersion, latestVersion)
	if err != nil {
		return fmt.Errorf("comparing current version to latest: %w", err)
	}

	if !proceed {
		_, err := fmt.Fprintln(c.cfg.Out, "No upgrade available.")

		return err
	}

	if confirmed := confirmUpgrade(latestVersion, c); !confirmed {
		_, err := fmt.Fprintln(c.cfg.Out, "Upgrade cancelled.")

		return err
	}

	latestBin, err := c.getLatestBinary(ctx, latestVersion)
	if err != nil {
		return fmt.Errorf("retrieving latest binary: %w", err)
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("retrieving the current executable path: %w", err)
	}

	if err := c.cfg.Writer.Write(binPath, latestBin); err != nil {
		return fmt.Errorf("writing new binary: %w", err)
	}
	successMessage := fmt.Sprintf("Backplane CLI has been upgraded to %s", latestVersion.TagName)
	_, _ = fmt.Fprintln(c.cfg.Out, successMessage)

	return nil
}

// getLatestBinary returns the data based on the release version
func (c *Cmd) getLatestBinary(ctx context.Context, release Release) ([]byte, error) {

	rawArc, err := c.git.GetReleaseArchive(ctx, release)

	if err != nil {
		return nil, fmt.Errorf("getting release archive for project '%s/%s' at version %q: %w", c.cfg.Org, c.cfg.Repo, release.TagName, err)
	}

	unzipped, err := gzip.NewReader(bytes.NewBuffer(rawArc))
	if err != nil {
		return nil, fmt.Errorf("reading zipped archive: %w", err)
	}

	arc := tar.NewReader(unzipped)

	for {
		next, err := arc.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("binary not found")
		} else if err != nil {
			return nil, fmt.Errorf("searching tar archive: %w", err)
		}
		if next.Name == c.cfg.BinaryName {
			break
		}
	}

	res, err := io.ReadAll(arc)
	if err != nil {
		return nil, fmt.Errorf("reading binary from archive: %w", err)
	}

	return res, nil
}

func shouldUpdate(current string, latest Release) (bool, error) {
	curVer, err := semver.NewVersion(current)
	if err != nil {
		return false, fmt.Errorf("parsing current version %q: %w", current, err)
	}

	latestVer, err := semver.NewVersion(latest.TagName)
	if err != nil {
		return false, fmt.Errorf("parsing latest version %q: %w", latest, err)
	}

	return curVer.LessThan(latestVer), nil
}

func confirmUpgrade(latest Release, c *Cmd) bool {

	message := fmt.Sprintf(
		"A newer version %q is available.\nWould you like to upgrade? (y/N)", latest.TagName,
	)
	_, _ = fmt.Fprintln(c.cfg.Out, message)

	input, _ := c.cfg.Reader.ReadString('\n')

	input = strings.TrimSpace(input)

	return strings.EqualFold(input, "y")
}

type CmdConfig struct {
	Log    logrus.FieldLogger
	Out    io.Writer
	Writer Writer
	Reader *bufio.Reader

	BinaryName string
	Org        string
	Repo       string
}

func (c *CmdConfig) Option(opts ...CmdOption) {
	for _, opt := range opts {
		opt.ConfigureCmd(c)
	}
}

func (c *CmdConfig) Default() {
	if c.Log == nil {
		c.Log = logrus.New()
	}

	if c.Out == nil {
		c.Out = os.Stdout
	}

	if c.Writer == nil {
		c.Writer = NewSafeWriter()
	}

	if c.Reader == nil {
		c.Reader = bufio.NewReader(os.Stdin)
	}

	if c.BinaryName == "" {
		c.BinaryName = "ocm-backplane"
	}

	if c.Org == "" {
		c.Org = "openshift"
	}

	if c.Repo == "" {
		c.Repo = "backplane-cli"
	}
}

type CmdOption interface {
	ConfigureCmd(*CmdConfig)
}
