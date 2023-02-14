package utils

import (
	"context"
	BackplaneApi "gitlab.cee.redhat.com/service/backplane-cli/pkg/client"
	"gitlab.cee.redhat.com/service/backplane-cli/pkg/info"
	"net/http"
)

type ClientUtils interface {
	MakeBackplaneAPIClient(base string) (BackplaneApi.ClientWithResponsesInterface, error)
	MakeBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientWithResponsesInterface, error)
	MakeRawBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientInterface, error)
	MakeRawBackplaneAPIClient(base string) (BackplaneApi.ClientInterface, error)
}

type DefaultClientUtilsImpl struct{}

var DefaultClientUtils ClientUtils = &DefaultClientUtilsImpl{}

func (_ *DefaultClientUtilsImpl) MakeRawBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientInterface, error) {
	co := func(client *BackplaneApi.Client) error {
		client.RequestEditors = append(client.RequestEditors, func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+accessToken)
			req.Header.Set("User-Agent", "backplane-cli"+info.Version)
			return nil
		})
		return nil
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

func (_ *DefaultClientUtilsImpl) MakeBackplaneAPIClientWithAccessToken(base, accessToken string) (BackplaneApi.ClientWithResponsesInterface, error) {
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
