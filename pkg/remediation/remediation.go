package remediation

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
	"k8s.io/client-go/rest"
)

func DoCreateRemediation(api string, clusterID string, accessToken string, remediationName string) (proxyURI string, err error) {
	client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return "", fmt.Errorf("unable to create backplane api client")
	}

	resp, err := client.CreateRemediation(context.TODO(), clusterID, &BackplaneApi.CreateRemediationParams{Remediation: remediationName})
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", utils.TryPrintAPIError(resp, false)
	}

	remediationResponse, err := BackplaneApi.ParseCreateRemediationResponse(resp)

	if err != nil {
		return "", fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	return api + *remediationResponse.JSON200.ProxyUri, nil
}

// CreateRemediationWithConn can be used to programtically interact with backplaneapi
func CreateRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, remediationName string) (config *rest.Config, err error) {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return nil, err
	}

	bpAPIClusterURL, err := DoCreateRemediation(bp.URL, clusterID, *accessToken, remediationName)
	if err != nil {
		return nil, err
	}

	cfg := &rest.Config{
		Host:        bpAPIClusterURL,
		BearerToken: *accessToken,
	}

	if bp.ProxyURL != nil {
		cfg.Proxy = func(r *http.Request) (*url.URL, error) {
			return url.Parse(*bp.ProxyURL)
		}
	}
	return cfg, nil
}

func DoDeleteRemediation(api string, clusterID string, accessToken string, remediation string) error {
	client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return fmt.Errorf("unable to create backplane api client")
	}

	resp, err := client.DeleteRemediation(context.TODO(), clusterID, &BackplaneApi.DeleteRemediationParams{Remediation: &remediation})
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return utils.TryPrintAPIError(resp, false)
	}

	return nil
}

// DeleteRemediationWithConn can be used to programtically interact with backplaneapi
func DeleteRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, remediation string) error {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return err
	}

	return DoDeleteRemediation(bp.URL, clusterID, *accessToken, remediation)
}
