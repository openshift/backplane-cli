package pagerduty

import (
	"context"
	"fmt"

	pdApi "github.com/PagerDuty/go-pagerduty"
)

// PagerDutyClient is an interface for the actual PD API
type PagerDutyClient interface {
	Connect(authToken string, options ...pdApi.ClientOptions) error
	ListIncidents(pdApi.ListIncidentsOptions) (*pdApi.ListIncidentsResponse, error)
	ListIncidentAlerts(incidentID string) (*pdApi.ListAlertsResponse, error)
	GetServiceWithContext(ctx context.Context, serviceID string, opts *pdApi.GetServiceOptions) (*pdApi.Service, error)
}

type DefaultPagerDutyClientImpl struct {
	client *pdApi.Client
}

// NewClient creates an instance of PDClient that is then used to connect to the actual pagerduty client.
func NewClient() *DefaultPagerDutyClientImpl {
	return &DefaultPagerDutyClientImpl{}
}

// Connect uses the information stored in new client to create a new PagerDuty connection.
// It returns the PDClient object with pagerduty API connection initialized.
func (c *DefaultPagerDutyClientImpl) Connect(authToken string, options ...pdApi.ClientOptions) error {

	if authToken == "" {
		return fmt.Errorf("empty pagerduty token")
	}

	// Create a new PagerDuty API client
	c.client = pdApi.NewClient(authToken, options...)

	return nil
}

func (c *DefaultPagerDutyClientImpl) ListIncidents(opts pdApi.ListIncidentsOptions) (*pdApi.ListIncidentsResponse, error) {
	return c.client.ListIncidentsWithContext(context.TODO(), opts)
}

func (c *DefaultPagerDutyClientImpl) ListIncidentAlerts(incidentID string) (*pdApi.ListAlertsResponse, error) {
	return c.client.ListIncidentAlerts(incidentID)
}

func (c *DefaultPagerDutyClientImpl) GetServiceWithContext(ctx context.Context, serviceID string, opts *pdApi.GetServiceOptions) (*pdApi.Service, error) {
	return c.client.GetServiceWithContext(ctx, serviceID, opts)
}
