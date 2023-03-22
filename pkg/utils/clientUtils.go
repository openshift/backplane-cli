package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/info"
	logger "github.com/sirupsen/logrus"
)

type ClientUtils interface {
	MakeBackplaneAPIClient(base string) (BackplaneApi.ClientWithResponsesInterface, error)
	MakeBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientWithResponsesInterface, error)
	MakeRawBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientInterface, error)
	MakeRawBackplaneAPIClient(base string) (BackplaneApi.ClientInterface, error)
	GetBackplaneClient(backplaneURL string) (client BackplaneApi.ClientInterface, err error)
	SetClientProxyUrl(proxyUrl string) error
}

type DefaultClientUtilsImpl struct{}

var (
	DefaultClientUtils ClientUtils = &DefaultClientUtilsImpl{}
	clientProxyUrl     string
)

func (*DefaultClientUtilsImpl) MakeRawBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientInterface, error) {
	co := func(client *BackplaneApi.Client) error {
		client.RequestEditors = append(client.RequestEditors, func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+accessToken)
			req.Header.Set("User-Agent", "backplane-cli"+info.Version)
			return nil
		})
		return nil
	}

	if clientProxyUrl != "" {
		proxyUrl, err := url.Parse(clientProxyUrl)
		if err != nil {
			return nil, err
		}
		http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
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

// getBackplaneClient returns authnicated Backplane API client
func (s *DefaultClientUtilsImpl) GetBackplaneClient(backplaneURL string) (client BackplaneApi.ClientInterface, err error) {
	if backplaneURL == "" {
		backplaneURL, err = DefaultOCMInterface.GetBackplaneURL()
		if err != nil || backplaneURL == "" {
			return client, fmt.Errorf("can't find backplane url: %w", err)
		}
		logger.Infof("Using backplane URL: %s\n", backplaneURL)
	}

	// Get backplane API client
	logger.Debugln("Finding ocm token")
	accessToken, err := DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return client, err
	}
	logger.Debugln("Found OCM access token")

	logger.Debugln("Getting client")
	backplaneClient, err := DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(backplaneURL, *accessToken)
	if err != nil {
		return client, fmt.Errorf("unable to create backplane api client: %w", err)
	}
	logger.Debugln("Got Client")

	return backplaneClient, nil
}

// Set client proxy url for http transport
func (*DefaultClientUtilsImpl) SetClientProxyUrl(proxyUrl string) error {
	if proxyUrl == "" {
		return errors.New("proxy Url is empty")
	}
	clientProxyUrl = proxyUrl
	return nil
}
