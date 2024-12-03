package remediation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
	logger "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

// TODO are we missusing the backplane api package? We have generated functions like backplaneapi.CreateRemediationWithResponse which already reads the body. All the utils functions do read the body again. Failing my calls here.
func DoCreateRemediation(api string, clusterID string, accessToken string, remediationName string) (proxyURI string, err error) {
	client, err := backplaneapi.DefaultClientUtils.MakeBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return "", fmt.Errorf("unable to create backplane api client")
	}

	logger.Debug("Sending request...")
	resp, err := client.CreateRemediationWithResponse(context.TODO(), clusterID, &BackplaneApi.CreateRemediationParams{Remediation: remediationName})
	if err != nil {
		logger.Debug("unexpected...")
		return "", err
	}

	// TODO figure out the error handling here
	if resp.StatusCode() != http.StatusOK {
		// logger.Debugf("Unmarshal error resp body: %s", resp.Body)
		var dest BackplaneApi.Error
		if err := json.Unmarshal(resp.Body, &dest); err != nil {
			// Avoid squashing the HTTP response info with Unmarshal err...
			logger.Debugf("Unmarshaled %s", *dest.Message)

			bodyStr := strings.ReplaceAll(string(resp.Body[:]), "\n", " ")
			err := fmt.Errorf("code:'%d'; failed to unmarshal response:'%s'; %w", resp.StatusCode(), bodyStr, err)
			return "", err
		}
		return "", errors.New(*dest.Message)

	}
	return api + *resp.JSON200.ProxyUri, nil
}

// CreateRemediationWithConn can be used to programtically interact with backplaneapi
func CreateRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, remediationName string) (config *rest.Config, serviceAccountName string, err error) {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return nil, "", err
	}

	bpAPIClusterURL, err := DoCreateRemediation(bp.URL, clusterID, *accessToken, remediationName)
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
	return cfg, "", nil
}

func DoDeleteRemediation(api string, clusterID string, accessToken string, remediation string) error {
	client, err := backplaneapi.DefaultClientUtils.MakeBackplaneAPIClientWithAccessToken(api, accessToken)
	if err != nil {
		return fmt.Errorf("unable to create backplane api client")
	}

	resp, err := client.DeleteRemediationWithResponse(context.TODO(), clusterID, &BackplaneApi.DeleteRemediationParams{Remediation: &remediation})
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		// logger.Debugf("Unmarshal error resp body: %s", resp.Body)
		var dest BackplaneApi.Error
		if err := json.Unmarshal(resp.Body, &dest); err != nil {
			// Avoid squashing the HTTP response info with Unmarshal err...
			logger.Debugf("Unmarshaled %s", *dest.Message)

			bodyStr := strings.ReplaceAll(string(resp.Body[:]), "\n", " ")
			err := fmt.Errorf("code:'%d'; failed to unmarshal response:'%s'; %w", resp.StatusCode(), bodyStr, err)
			return err
		}
		return errors.New(*dest.Message)
	}

	return nil
}

// DeleteRemediationWithConn can be used to programtically interact with backplaneapi
func DeleteRemediationWithConn(bp config.BackplaneConfiguration, ocmConnection *ocmsdk.Connection, clusterID string, remediationSA string) error {
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
		return err
	}

	return DoDeleteRemediation(bp.URL, clusterID, *accessToken, remediationSA)
}
