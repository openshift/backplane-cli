package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	logger "github.com/sirupsen/logrus"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/info"
)

type ClientUtils interface {
	MakeBackplaneAPIClient(base string) (BackplaneApi.ClientWithResponsesInterface, error)
	MakeBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientWithResponsesInterface, error)
	MakeRawBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientInterface, error)
	MakeRawBackplaneAPIClient(base string) (BackplaneApi.ClientInterface, error)
	GetBackplaneClient(backplaneURL string, ocmToken *string) (client BackplaneApi.ClientInterface, err error)
	SetClientProxyURL(proxyURL string) error
}

type DefaultClientUtilsImpl struct {
	clientProxyURL string
}

var (
	DefaultClientUtils ClientUtils = &DefaultClientUtilsImpl{}
)

func (s *DefaultClientUtilsImpl) MakeRawBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientInterface, error) {
	co := func(client *BackplaneApi.Client) error {
		client.RequestEditors = append(client.RequestEditors, func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+accessToken)
			req.Header.Set("User-Agent", "backplane-cli"+info.Version)
			return nil
		})
		return nil
	}

	// Inject client Proxy Url from config
	if s.clientProxyURL == "" {
		bpConfig, err := config.GetBackplaneConfiguration()
		if err != nil {
			return nil, err
		}
		s.clientProxyURL = bpConfig.ProxyURL
	}

	// Update http proxy transport
	if s.clientProxyURL != "" {
		proxyURL, err := url.Parse(s.clientProxyURL)
		if err != nil {
			return nil, err
		}
		http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}

		logger.Debugf("Using backplane Proxy URL: %s\n", s.clientProxyURL)
	}

	return BackplaneApi.NewClient(base, co)
}

func (s *DefaultClientUtilsImpl) MakeRawBackplaneAPIClient(base string) (BackplaneApi.ClientInterface, error) {
	token, err := DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return nil, err
	}
	return s.MakeRawBackplaneAPIClientWithAccessToken(base, *token)
}

func (*DefaultClientUtilsImpl) MakeBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientWithResponsesInterface, error) {
	co := func(client *BackplaneApi.Client) error {
		client.RequestEditors = append(client.RequestEditors, func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+accessToken)
			req.Header.Set("User-Agent", "backplane-cli"+info.Version)
			return nil
		})
		return nil
	}

	return BackplaneApi.NewClientWithResponses(base, co)
}

func (s *DefaultClientUtilsImpl) MakeBackplaneAPIClient(base string) (BackplaneApi.ClientWithResponsesInterface, error) {
	token, err := DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return nil, err
	}
	return s.MakeBackplaneAPIClientWithAccessToken(base, *token)
}

// GetBackplaneClient returns authenticated Backplane API client
func (s *DefaultClientUtilsImpl) GetBackplaneClient(backplaneURL string, ocmToken *string) (client BackplaneApi.ClientInterface, err error) {
	if backplaneURL == "" {
		bpConfig, err := config.GetBackplaneConfiguration()
		backplaneURL = bpConfig.URL
		if err != nil || backplaneURL == "" {
			return client, fmt.Errorf("can't find backplane url: %w", err)
		}
		logger.Infof("Using backplane URL: %s\n", backplaneURL)
	}

	logger.Debugln("Getting client")
	backplaneClient, err := DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(backplaneURL, *ocmToken)
	if err != nil {
		return client, fmt.Errorf("unable to create backplane api client: %w", err)
	}
	logger.Debugln("Got Client")

	return backplaneClient, nil
}

// SetClientProxyURL Set client proxy url for http transport
func (s *DefaultClientUtilsImpl) SetClientProxyURL(proxyURL string) error {
	if proxyURL == "" {
		return errors.New("proxy URL is empty")
	}
	s.clientProxyURL = proxyURL
	return nil
}
