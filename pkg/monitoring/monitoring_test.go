package monitoring

import (
	"net/http"
	"os"

	"net/http/httptest"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("Backplane Monitoring Unit test", func() {
	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *mocks2.MockOCMInterface
		mockClientUtil     *mocks2.MockClientUtils

		testClusterId   string
		testToken       string
		trueClusterId   string
		backplaneAPIUri string

		fakeResp *http.Response

		testKubeCfg       api.Config
		clusterVersion412 *cmv1.Cluster
		clusterVersion410 *cmv1.Cluster
		client            Client
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

		testClusterId = "test123"
		testToken = "hello123"
		trueClusterId = "trueID123"
		backplaneAPIUri = "https://api.integration.backplane.example.com"

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		clusterVersion412, _ = cmv1.NewCluster().
			OpenshiftVersion("4.12.0").
			CloudProvider(cmv1.NewCloudProvider().ID("aws")).
			Product(cmv1.NewProduct().ID("dedicated")).
			AdditionalTrustBundle("REDACTED").
			Proxy(cmv1.NewProxy().HTTPProxy("http://my.proxy:80").HTTPSProxy("https://my.proxy:443")).Build()

		clusterVersion410, _ = cmv1.NewCluster().
			OpenshiftVersion("4.10.0").
			CloudProvider(cmv1.NewCloudProvider().ID("aws")).
			Product(cmv1.NewProduct().ID("dedicated")).
			AdditionalTrustBundle("REDACTED").
			Proxy(cmv1.NewProxy().HTTPProxy("http://my.proxy:80").HTTPSProxy("https://my.proxy:443")).Build()

		testKubeCfg = api.Config{
			Kind:        "Config",
			APIVersion:  "v1",
			Preferences: api.Preferences{},
			Clusters: map[string]*api.Cluster{
				"testcluster": {
					Server: "https://api-backplane.apps.something.com/backplane/cluster/test123",
				},
				"api-backplane.apps.something.com:443": { // Remark that the cluster name does not match the cluster ID in below URL
					Server: "https://api-backplane.apps.something.com/backplane/cluster/test123",
				},
			},
			AuthInfos: map[string]*api.AuthInfo{
				"testauth": {
					Token: "token123",
				},
			},
			Contexts: map[string]*api.Context{
				"default/testcluster/testauth": {
					Cluster:   "testcluster",
					AuthInfo:  "testauth",
					Namespace: "default",
				},
				"custom-context": {
					Cluster:   "api-backplane.apps.something.com:443",
					AuthInfo:  "testauth",
					Namespace: "test-namespace",
				},
			},
			CurrentContext: "default/testcluster/testauth",
			Extensions:     nil,
		}

		err := utils.CreateTempKubeConfig(&testKubeCfg)
		Expect(err).To(BeNil())

		os.Setenv(info.BACKPLANE_URL_ENV_NAME, backplaneAPIUri)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("check Backplane monitoring", func() {
		It("should fail for empty monitoring name", func() {

			client = NewClient("", client.http)
			err := client.RunMonitoring("")

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("monitoring type is empty"))
		})

		It("should fail for cluster version greater than 4.11 and openshift monitoring namespace ", func() {

			mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(testClusterId).Return(clusterVersion412, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil).AnyTimes()

			MonitoringOpts.Namespace = OPENSHIFT_MONITORING_NS

			// check for prometheus

			client = NewClient("", client.http)
			err := client.RunMonitoring(PROMETHEUS)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("this cluster's version is 4.11 or greater."))
			Expect(err.Error()).Should(ContainSubstring("Prometheus, AlertManager and Grafana monitoring UIs are deprecated"))

			// check for Grafana
			client = NewClient("", client.http)
			err = client.RunMonitoring(GRAFANA)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("this cluster's version is 4.11 or greater."))
			Expect(err.Error()).Should(ContainSubstring("Prometheus, AlertManager and Grafana monitoring UIs are deprecated"))

			// check for Alertmanager
			client = NewClient("", client.http)
			err = client.RunMonitoring(ALERTMANAGER)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("this cluster's version is 4.11 or greater."))
			Expect(err.Error()).Should(ContainSubstring("Prometheus, AlertManager and Grafana monitoring UIs are deprecated"))
		})

		It("should serve thanos monitoring dashboard for cluster version greater than 4.11", func() {
			mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(testClusterId).Return(clusterVersion412, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil).AnyTimes()

			MonitoringOpts.KeepAlive = false

			svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("dummy data"))
			}))

			defer svr.Close()

			client = NewClient(svr.URL, *svr.Client())
			err := client.RunMonitoring(THANOS)
			Expect(err).To(BeNil())
		})

		It("should serve monitoring dashboard for cluster version lower than 4.11 and openshift monitoring namespace ", func() {

			mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(testClusterId).Return(clusterVersion410, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil).AnyTimes()

			MonitoringOpts.Namespace = OPENSHIFT_MONITORING_NS
			MonitoringOpts.KeepAlive = false
			svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("dummy data"))
			}))
			defer svr.Close()

			// check for prometheus
			client = NewClient(svr.URL, *svr.Client())
			err := client.RunMonitoring(PROMETHEUS)
			Expect(err).To(BeNil())

			// check for Thanos
			client = NewClient(svr.URL, *svr.Client())
			err = client.RunMonitoring(THANOS)
			Expect(err).To(BeNil())

			// check for Alertmanager
			client = NewClient(svr.URL, *svr.Client())
			err = client.RunMonitoring(ALERTMANAGER)
			Expect(err).To(BeNil())
		})
	})
})
