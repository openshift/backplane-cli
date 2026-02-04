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

// DoCreateRemediation creates a remediation instance for a cluster using the Backplane API.
// It sends a request to create a remediation and returns the proxy URI and remediation instance ID.
// The function takes API endpoint, cluster ID, access token, and remediation name as parameters.
func DoCreateRemediation(api string, clusterID string, accessToken string, createRemediationParams *BackplaneApi.CreateRemediationParams) (proxyURI string, remediationInstanceID string, err error) {
	client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return "", "", fmt.Errorf("unable to create backplane api client")
	}

	resp, err := client.CreateRemediation(context.TODO(), clusterID, createRemediationParams)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", utils.TryPrintAPIError(resp, false)
	}

	remediationResponse, err := BackplaneApi.ParseCreateRemediationResponse(resp)

	if err != nil {
		return "", "", fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	return api + *remediationResponse.JSON200.ProxyUri, remediationResponse.JSON200.RemediationInstanceId, nil
}

// CreateRemediationWithConn creates a remediation instance and returns a configured Kubernetes client.
// This function can be used to programmatically interact with the Backplane API.
// It creates a rest.Config that can be used with Kubernetes client libraries.
func CreateRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, createRemediationParams *BackplaneApi.CreateRemediationParams) (config *rest.Config, remediationInstanceID string, err error) {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return nil, "", err
	}

	bpAPIClusterURL, remediationInstanceID, err := DoCreateRemediation(bp.URL, clusterID, *accessToken, createRemediationParams)
	if err != nil {
		return nil, "", err
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
	return cfg, remediationInstanceID, nil
}

// DoDeleteRemediation deletes a remediation instance using the Backplane API.
// It takes the API endpoint, cluster ID, access token, and remediation instance ID as parameters.
// Returns an error if the deletion fails or if the API returns a non-success status.
func DoDeleteRemediation(api string, clusterID string, accessToken string, remediationInstanceID string) error {
	client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return fmt.Errorf("unable to create backplane api client")
	}

	resp, err := client.DeleteRemediation(context.TODO(), clusterID, &BackplaneApi.DeleteRemediationParams{RemediationInstanceId: remediationInstanceID})
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return utils.TryPrintAPIError(resp, false)
	}

	return nil
}

// DeleteRemediationWithConn can be used to programtically interact with backplaneapi
func DeleteRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, remediationInstanceID string) error {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return err
	}

	return DoDeleteRemediation(bp.URL, clusterID, *accessToken, remediationInstanceID)
}
