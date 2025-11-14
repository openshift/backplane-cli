package pagerduty

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pdApi "github.com/PagerDuty/go-pagerduty"
	logger "github.com/sirupsen/logrus"
)

// Alert struct represents the data contained in an alert.
type Alert struct {
	ID          string
	Name        string
	IncidentID  string
	Severity    string
	Status      string
	CreatedAt   time.Time
	WebURL      string
	ClusterID   string
	ClusterName string
}

const (
	// PagerDuty Incident Statuses
	StatusTriggered    = "triggered"
	StatusAcknowledged = "acknowledged"
	StatusHigh         = "high"
	StatusLow          = "low"
)

type PagerDuty struct {
	client PagerDutyClient
}

// NewPagerDuty creates a new PagerDuty instance with the provided client.
// This constructor allows for dependency injection and easier testing.
func NewPagerDuty(client PagerDutyClient) *PagerDuty {
	return &PagerDuty{
		client: client,
	}
}

// NewWithToken initializes a new PDClient.
// The token can be created using the docs https://support.pagerduty.com/docs/api-access-keys#section-generate-a-user-token-rest-api-key.
func NewWithToken(authToken string, options ...pdApi.ClientOptions) (*PagerDuty, error) {

	pd := NewClient()
	err := pd.Connect(authToken, options...)

	if err != nil {
		return nil, err
	}

	return &PagerDuty{
		client: pd,
	}, nil

}

// GetIncidentAlerts returns all the alerts belonging to a particular incident.
func (pd *PagerDuty) GetIncidentAlerts(incidentID string) ([]Alert, error) {
	var alerts []Alert

	// Fetch alerts related to an incident via pagerduty API
	incidentAlerts, err := pd.client.ListIncidentAlerts(incidentID)

	if err != nil {
		var aerr pdApi.APIError

		if errors.As(err, &aerr) {
			if aerr.RateLimited() {
				return nil, fmt.Errorf("API rate limited")
			}

			return nil, fmt.Errorf("status code: %d, error: %s", aerr.StatusCode, err)
		}
	}

	for _, alert := range incidentAlerts.Alerts {
		currentAlert := alert
		formatAlert, err := pd.formatAlert(&currentAlert)

		if err != nil {
			return nil, err
		}
		alerts = append(alerts, formatAlert)
	}
	return alerts, nil
}

func (pd *PagerDuty) formatAlert(alert *pdApi.IncidentAlert) (formatAlert Alert, err error) {
	formatAlert.IncidentID = alert.Incident.ID

	formatAlert.Name = alert.Summary
	formatAlert.Status = alert.Status
	formatAlert.WebURL = alert.HTMLURL

	// Check if alert.Body and alert.Body["details"] exist
	if alert.Body == nil {
		return formatAlert, fmt.Errorf("unable to parse alert: alert body is missing")
	}

	details, ok := alert.Body["details"]
	if !ok || details == nil {
		return formatAlert, fmt.Errorf("unable to parse alert: alert details are missing")
	}

	detailsMap, ok := details.(map[string]interface{})
	if !ok {
		return formatAlert, fmt.Errorf("unable to parse alert: alert details have unexpected format")
	}

	// Check if the alert is of type 'Missing cluster'
	isCHGM := detailsMap["notes"]

	if isCHGM != nil {
		notes := strings.Split(fmt.Sprint(detailsMap["notes"]), "\n")
		fmt.Print(notes)
		formatAlert.ClusterID = strings.Replace(notes[0], "cluster_id: ", "", 1)

		nameValue, ok := detailsMap["name"]
		if ok && nameValue != nil {
			formatAlert.ClusterName = strings.Split(fmt.Sprint(nameValue), ".")[0]
		}

	} else {
		clusterIDValue, ok := detailsMap["cluster_id"]
		if ok && clusterIDValue != nil {
			formatAlert.ClusterID = fmt.Sprint(clusterIDValue)
		}

		formatAlert.ClusterName, err = pd.GetClusterName(alert.Service.ID)
		if err != nil {
			logger.Warnf("Failed to get cluster name for service %s: %v", alert.Service.ID, err)
		}
	}

	// If there's no cluster ID related to the given alert
	if formatAlert.ClusterID == "" {
		formatAlert.ClusterID = "N/A"
	}

	// If there's no cluster name related to the given alert
	if formatAlert.ClusterName == "" {
		formatAlert.ClusterName = "N/A"
	}

	return formatAlert, nil
}

// GetClusterName interacts with the PD service endpoint and returns the cluster name string.
func (pd *PagerDuty) GetClusterName(serviceID string) (string, error) {
	service, err := pd.client.GetServiceWithContext(context.TODO(), serviceID, &pdApi.GetServiceOptions{})

	if err != nil {
		return "", err
	}

	clusterName := strings.Split(service.Description, " ")[0]

	return clusterName, nil
}

// GetClusterInfoFromIncident retrieves the cluster ID associated with the given incident ID.
func (pd *PagerDuty) GetClusterInfoFromIncident(incidentID string) (info Alert, err error) {
	incidentAlerts, err := pd.GetIncidentAlerts(incidentID)
	if err != nil {
		return info, err
	}

	switch len(incidentAlerts) {
	case 0:
		return info, fmt.Errorf("no alerts found for the given incident ID")
	case 1:
		return incidentAlerts[0], nil
	default:
		currentClusterID := incidentAlerts[0].ClusterID
		for _, alert := range incidentAlerts {

			if currentClusterID != alert.ClusterID {
				return info, fmt.Errorf("not all alerts have the same cluster ID")
			}

		}
		return incidentAlerts[0], nil
	}

}
