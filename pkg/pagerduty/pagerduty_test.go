package pagerduty

import (
	"time"

	pagerduty "github.com/PagerDuty/go-pagerduty"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pdMock "github.com/openshift/backplane-cli/pkg/pagerduty/mock"
)

var _ = Describe("Pagerduty", func() {
	var (
		mockCtrl          *gomock.Controller
		mockPdClient      *pdMock.MockPagerDutyClient
		clusterID         string
		mockIncident      *Alert
		mockIncidentAlert *pagerduty.IncidentAlert
		mockAlertList     *pagerduty.ListAlertsResponse
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockPdClient = pdMock.NewMockPagerDutyClient(mockCtrl)
		mockIncident = &Alert{
			ID:         "Test",
			IncidentID: "TestIncidentID",
			CreatedAt:  time.Now(),
		}
		mockIncidentAlert = &pagerduty.IncidentAlert{
			Body: map[string]interface{}{
				"cef_details": "details",
				"details":     "cluster_id",
				"cluster_id":  "TestClsuterID",
			},
			Incident: pagerduty.APIReference{
				ID: "TestIncidentID",
			},
		}
		mockAlertList = &pagerduty.ListAlertsResponse{
			Alerts: []pagerduty.IncidentAlert{
				*mockIncidentAlert,
			},
		}
		clusterID = "TestClusterID"
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("When pagerduty client executes", func() {
		It("Should get Cluster ID from Alert List", func() {
			mockPdClient.EXPECT().GetClusterIDFromAlertList(mockAlertList).Return(clusterID, nil)
			Expect(len(mockAlertList.Alerts)).ToNot(Equal(0))
			testClusterID, err := mockPdClient.GetClusterIDFromAlert(&mockAlertList.Alerts[0])
			Expect(err).To(BeNil())
			Expect(testClusterID).To(Equal(clusterID))
		})

		It("Should get Cluster ID from Alert", func() {
			mockPdClient.EXPECT().GetClusterIDFromAlert(mockIncidentAlert).Return(clusterID, nil)
			Expect(mockIncidentAlert.Body).ToNot(Equal(nil))
			cefDetails, err := mockIncidentAlert.Body["cef_details"].(map[string]interface{})
			Expect(err).To(BeFalse())
			detailsValue, err := cefDetails["details"]
			Expect(err).To(BeFalse())
			details, err := detailsValue.(map[string]interface{})
			Expect(err).To(BeFalse())
			getClusterID, err := details["cluster_id"].(string)
			Expect(err).To(BeFalse())
			Expect(getClusterID).To(Equal(clusterID))
		})

		It("Should get Cluster ID", func() {
			mockPdClient.EXPECT().GetClusterID(mockPdClient, mockIncident.IncidentID).Return(clusterID, nil)
		})
	})
})
