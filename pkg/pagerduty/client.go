package pagerduty

import (
	"context"
	"net/http"

	pdApi "github.com/PagerDuty/go-pagerduty"
)

// PagerDutyClient is an interface for the actual PD API
type PagerDutyClient interface {
	ListIncidents(pdApi.ListIncidentsOptions) (*pdApi.ListIncidentsResponse, error)
	ListIncidentAlerts(incidentId string) (*pdApi.ListAlertsResponse, error)
	GetCurrentUser(pdApi.GetCurrentUserOptions) (*pdApi.User, error)
	GetIncidentAlert(incidentID, alertID string) (*pdApi.IncidentAlertResponse, *http.Response, error)
	GetService(serviceID string, opts *pdApi.GetServiceOptions) (*pdApi.Service, error)
	GetServiceWithContext(ctx context.Context, serviceID string, opts *pdApi.GetServiceOptions) (*pdApi.Service, error)
	ListOnCalls(opts pdApi.ListOnCallOptions) (*pdApi.ListOnCallsResponse, error)
}

type PDClient struct {
	PdClient PagerDutyClient
}

// NewClient creates an instance of PDClient that is then used to connect to the actual pagerduty client.
func NewClient() *PDClient {
	return &PDClient{}
}

// Connect uses the information stored in new client to create a new PagerDuty connection.
// It returns the PDClient object with pagerduty API connection initialized.
func (pd *PDClient) Connect(authToken string, options ...pdApi.ClientOptions) (client *PDClient, err error) {

	// Create a new PagerDuty API client
	pdApi.NewClient(authToken, options...)

	return pd, nil
}

func (c *PDClient) ListIncidents(opts pdApi.ListIncidentsOptions) (*pdApi.ListIncidentsResponse, error) {
	return c.PdClient.ListIncidents(opts)
}

func (c *PDClient) ListIncidentAlerts(incidentID string) (*pdApi.ListAlertsResponse, error) {
	return c.PdClient.ListIncidentAlerts(incidentID)
}

func (c *PDClient) GetCurrentUser(opts pdApi.GetCurrentUserOptions) (*pdApi.User, error) {
	return c.PdClient.GetCurrentUser(opts)
}

func (c *PDClient) GetIncidentAlert(incidentID, alertID string) (*pdApi.IncidentAlertResponse, *http.Response, error) {
	return c.PdClient.GetIncidentAlert(incidentID, alertID)
}

func (c *PDClient) GetService(serviceID string, opts *pdApi.GetServiceOptions) (*pdApi.Service, error) {
	return c.PdClient.GetService(serviceID, opts)
}

func (c *PDClient) GetServiceWithContext(ctx context.Context, serviceID string, opts *pdApi.GetServiceOptions) (*pdApi.Service, error) {
	return c.PdClient.GetServiceWithContext(ctx, serviceID, opts)
}

func (c *PDClient) ListOnCalls(opts pdApi.ListOnCallOptions) (*pdApi.ListOnCallsResponse, error) {
	return c.PdClient.ListOnCalls(opts)
}
