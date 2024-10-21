package login

import (
        "context"
        "errors"
        "fmt"
        "net"
        "net/http"
        "net/url"
        "os"
        "regexp"
        "strings"

        "github.com/golang-jwt/jwt/v4"
        logger "github.com/sirupsen/logrus"
        "github.com/spf13/cobra"
        "k8s.io/cli-runtime/pkg/genericclioptions"
        "k8s.io/client-go/rest"
        "k8s.io/client-go/tools/clientcmd/api"

        BackplaneApi "github.com/openshift/backplane-api/pkg/client"

        ocmsdk "github.com/openshift-online/ocm-sdk-go"
        "github.com/openshift/backplane-cli/pkg/backplaneapi"
        "github.com/openshift/backplane-cli/pkg/cli/config"
        "github.com/openshift/backplane-cli/pkg/cli/globalflags"
        "github.com/openshift/backplane-cli/pkg/jira"
        "github.com/openshift/backplane-cli/pkg/login"
        "github.com/openshift/backplane-cli/pkg/ocm"
        "github.com/openshift/backplane-cli/pkg/pagerduty"
        "github.com/openshift/backplane-cli/pkg/utils"

)


// GetRestConfig returns a client-go *rest.Config which can be used to programmatically interact with the
// Kubernetes API of a provided clusterID
func displyIfClusterAccessProtectionEnabled(ocmConnection *ocmsdk.Connection, clusterID string) (*rest.Config, error) {
	cluster, err := ocm.DefaultOCMInterface.IsClusterAccessProtectionEnabled(ocmConnection, clusterID)
	if err != nil {
			return nil, err 
	}

	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessTokenWithConn(ocmConnection)
	if err != nil {
			return nil, err
	}

	bpAPIClusterURL, err := doLogin(bp.URL, clusterID, *accessToken)
	if err != nil {
			return nil, fmt.Errorf("failed to backplane login to cluster %s: %v", cluster.Name(), err)
	}

	cfg := &rest.Config{
			Host:        bpAPIClusterURL,
			BearerToken: *accessToken,
	}

	if bp.ProxyURL != nil {
			cfg.Proxy = func(*http.Request) (*url.URL, error) {
					return url.Parse(*bp.ProxyURL)
			}
	}

	return cfg, nil
}




// displayClusterInfo retrieves and displays basic information about the target cluster.
func displayClusterInfo(clusterID string) error {
	logger := logger.WithField("clusterID", clusterID)

	// Retrieve cluster information
	clusterInfo, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
			return fmt.Errorf("error retrieving cluster info: %v", err)
	}

	// Display cluster information
	fmt.Printf("Cluster ID: %s\n", clusterInfo.ID())
	fmt.Printf("Cluster Name: %s\n", clusterInfo.Name())
	fmt.Printf("Cluster Status: %s\n", clusterInfo.Status().State())
	fmt.Printf("Cluster Region: %s\n", clusterInfo.Region().ID())
	fmt.Printf("Cluster Provider: %s\n", clusterInfo.CloudProvider().ID())

	logger.Info("Basic cluster information displayed.")
	return nil
}	

// to call the current function
	if err = displayClusterInfo(clusterID); err != nil {
		return err
	}


