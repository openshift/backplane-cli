package pagerduty

import (
	"context"
	"fmt"
	"time"

	"github.com/PagerDuty/go-pagerduty"
)

// PagerDutyClient defines the methods needed from the PagerDuty client.
type PagerDutyClient interface {
	GetClusterIDFromAlertList(alertList *pagerduty.ListAlertsResponse) (string, error)
	GetClusterIDFromAlert(alert *pagerduty.IncidentAlert) (string, error)
	GetClusterID(pdClient *pagerduty.Client, incidentID string) (string, error)
	CreateIncident(description string) (string, error)
}

// RealPagerDutyClient implements the PagerDutyClient interface using the real PagerDuty client.
type RealPagerDutyClient struct {
	PdClient *pagerduty.Client
}

// Alert struct represents the data contained in an alert.
type Alert struct {
	ID         string
	IncidentID string
	CreatedAt  time.Time
}

// NewWithToken initializes a new PDClient.
// The token can be created using the docs https://support.pagerduty.com/docs/api-access-keys#section-generate-a-user-token-rest-api-key.
func NewWithToken(authToken string, options ...pagerduty.ClientOptions) (*RealPagerDutyClient, error) {
	c := RealPagerDutyClient{
		PdClient: pagerduty.NewClient(authToken, options...),
	}

	return &c, nil
}

func (c *RealPagerDutyClient) GetClusterIDFromAlertList(alertList *pagerduty.ListAlertsResponse) (string, error) {
	if len(alertList.Alerts) == 0 {
		return "", fmt.Errorf("no alerts found for the given incident ID")

	} else if len(alertList.Alerts) == 1 {
		clusterID, err := c.GetClusterIDFromAlert(&alertList.Alerts[0])
		if err != nil {
			return "", err
		}
		return clusterID, nil

	} else if len(alertList.Alerts) > 1 {
		prevClusterID, err := c.GetClusterIDFromAlert(&alertList.Alerts[0])
		if err != nil {
			return "", err
		}
		// Check if all alerts in the list have the same cluster ID
		for _, alert := range alertList.Alerts {
			currentAlert := alert
			if currentClusterID, err := c.GetClusterIDFromAlert(&currentAlert); err != nil {
				return "", err
			} else if currentClusterID != prevClusterID {
				return "", fmt.Errorf("not all alerts have the same cluster ID")
			}
		}
		return prevClusterID, nil
	}

	return "", fmt.Errorf("unable to retrieve list of pagerduty alerts")
}

// getClusterIDFromAlert extracts the cluster ID from a PagerDuty incident alert.
// It expects the alert's body to have a Common Event Format.
func (c *RealPagerDutyClient) GetClusterIDFromAlert(alert *pagerduty.IncidentAlert) (string, error) {
	if alert == nil || alert.Body == nil {
		return "", fmt.Errorf("given alert or it's body is empty")
	}

	cefDetails, ok := alert.Body["cef_details"].(map[string]interface{})
	if !ok || cefDetails == nil {
		return "", fmt.Errorf("missing or invalid Common Event Format Details of given alert")
	}

	detailsValue, ok := cefDetails["details"]
	if !ok || detailsValue == nil {
		return "", fmt.Errorf("missing or invalid 'details' field in Common Event Format Details")
	}

	details, ok := detailsValue.(map[string]interface{})
	if !ok || details == nil {
		return "", fmt.Errorf("'details' field is not a map[string]interface{} in Common Event Format Details")
	}

	clusterID, ok := details["cluster_id"].(string)
	if !ok || clusterID == "" {
		return "", fmt.Errorf("missing or invalid 'cluster_id' field in CEF details")
	}
	return clusterID, nil
}

func (c *RealPagerDutyClient) GetClusterID(incidentID string) (string, error) {
	incidentAlerts, err := c.PdClient.ListIncidentAlertsWithContext(context.TODO(), incidentID, pagerduty.ListIncidentAlertsOptions{})
	if err != nil {
		return "", err
	}

	clusterID, err := c.GetClusterIDFromAlertList(incidentAlerts)
	if err != nil {
		return "", err
	}

	return clusterID, nil
}
