package pagerduty

import (
	"context"

	pdApi "github.com/PagerDuty/go-pagerduty"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pdMock "github.com/openshift/backplane-cli/pkg/pagerduty/mocks"
)

var _ = Describe("Pagerduty", func() {
	var (
		mockCtrl        *gomock.Controller
		mockPdClient    *pdMock.MockPagerDutyClient
		pagerDuty       *PagerDuty
		testIncidentId  string
		testClusterId   string
		testServiceId   string
		testClusterName string
		testAlertName   string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockPdClient = pdMock.NewMockPagerDutyClient(mockCtrl)
		pagerDuty = NewPagerDuty(mockPdClient)
		testIncidentId = "incident-id-000"
		testClusterId = "cluster-id-000"
		testServiceId = "service-id-000"
		testClusterName = "test-cluster-name"
		testAlertName = "alert-error-budget"
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("When pagerduty client executes", func() {
		It("Should return incidents", func() {

			// Mock alert response
			alertResponse := &pdApi.ListAlertsResponse{
				Alerts: []pdApi.IncidentAlert{
					alert(
						testIncidentId,
						testServiceId,
						testAlertName,
						testClusterId,
						"triggered",
					),
				},
			}

			// Mock the service response
			serviceResponse := &pdApi.Service{
				Description: testClusterName,
			}

			mockPdClient.EXPECT().ListIncidentAlerts(testIncidentId).Return(alertResponse, nil).Times(1)
			mockPdClient.EXPECT().GetServiceWithContext(context.TODO(), testServiceId, gomock.Any()).Return(serviceResponse, nil).Times(1)

			alerts, err := pagerDuty.GetIncidentAlerts(testIncidentId)
			Expect(err).To(BeNil())
			Expect(len(alerts)).To(Equal(1))

		})

		It("Should return cluster-id for a incident", func() {

			// Mock alert response
			alertResponse := &pdApi.ListAlertsResponse{
				Alerts: []pdApi.IncidentAlert{
					alert(
						testIncidentId,
						testServiceId,
						testAlertName,
						testClusterId,
						"triggered",
					),
				},
			}

			// Mock the service response
			serviceResponse := &pdApi.Service{
				Description: testClusterName,
			}

			mockPdClient.EXPECT().ListIncidentAlerts(testIncidentId).Return(alertResponse, nil).Times(1)
			mockPdClient.EXPECT().GetServiceWithContext(context.TODO(), testServiceId, gomock.Any()).Return(serviceResponse, nil).Times(1)

			clusterId, err := pagerDuty.GetClusterID(testIncidentId)
			Expect(err).To(BeNil())
			Expect(clusterId).To(Equal(testClusterId))
		})

		It("Should return empty cluster-id for non matching incident id", func() {

			// Mock alert response
			alertResponse := &pdApi.ListAlertsResponse{
				Alerts: []pdApi.IncidentAlert{},
			}

			mockPdClient.EXPECT().ListIncidentAlerts(testIncidentId).Return(alertResponse, nil).Times(1)

			clusterId, err := pagerDuty.GetClusterID(testIncidentId)
			Expect(err).NotTo(BeNil())
			Expect(clusterId).To(Equal(""))
		})

		It("Should return first cluster-id for multiple incident", func() {

			// Mock alert response
			alertResponse := &pdApi.ListAlertsResponse{
				Alerts: []pdApi.IncidentAlert{
					alert(
						testIncidentId,
						testServiceId,
						testAlertName,
						testClusterId,
						"triggered",
					),
					alert(
						"incident-id-001",
						testServiceId,
						testAlertName,
						testClusterId,
						"triggered",
					),
				},
			}

			// Mock the service response
			serviceResponse := &pdApi.Service{
				Description: testClusterName,
			}

			mockPdClient.EXPECT().ListIncidentAlerts(testIncidentId).Return(alertResponse, nil).Times(1)
			mockPdClient.EXPECT().GetServiceWithContext(context.TODO(), testServiceId, gomock.Any()).Return(serviceResponse, nil).Times(2)

			clusterId, err := pagerDuty.GetClusterID(testIncidentId)
			Expect(err).To(BeNil())
			Expect(clusterId).To(Equal(testClusterId))
		})

	})
})

// incident retuns a pagerduty incident object with pre-configured data.
func incident(incidentID uint) pdApi.Incident {
	return pdApi.Incident{
		IncidentNumber: incidentID,
		Urgency:        "high",
	}
}

// alert retuns a pagerduty alert object with pre-configured data.
func alert(incidentID string, serviceID string, name string, clusterID string, status string) pdApi.IncidentAlert {
	return pdApi.IncidentAlert{

		Incident: pdApi.APIReference{
			ID: incidentID,
		},

		Service: pdApi.APIObject{
			ID: serviceID,
		},

		APIObject: pdApi.APIObject{
			Summary: name,
		},

		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"cluster_id": clusterID,
			},
		},

		Status: status,
	}
}
