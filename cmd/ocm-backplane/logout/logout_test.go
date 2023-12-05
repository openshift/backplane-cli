package logout

import (
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func MakeIoReader(s string) io.ReadCloser {
	r := io.NopCloser(strings.NewReader(s)) // r type is io.ReadCloser
	return r
}

var _ = Describe("Logout command", func() {

	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *ocmMock.MockOCMInterface
		mockClientUtil     *backplaneapiMock.MockClientUtils

		testClusterID   string
		testToken       string
		trueClusterID   string
		backplaneAPIURI string

		fakeResp *http.Response

		loginCmd *cobra.Command

		kubeConfig                 api.Config
		loggedInNotBackplaneConfig api.Config
		ocmEnv                     *cmv1.Environment
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		testClusterID = "test123"
		testToken = "hello123"
		trueClusterID = "trueID123"
		backplaneAPIURI = "https://api.integration.backplane.example.com"

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		loginCmd = login.LoginCmd

		kubeConfig = api.Config{
			Kind:        "Config",
			APIVersion:  "v1",
			Preferences: api.Preferences{},
			Clusters: map[string]*api.Cluster{
				"dummy_cluster": {
					Server: "https://api.backplane.apps.something3.com/backplane/cluster/configcluster",
				},
			},
			Contexts: map[string]*api.Context{
				"default/test123/anonymous": {
					Cluster:   "dummy_cluster",
					Namespace: "default",
				},
			},
			CurrentContext: "default/test123/anonymous",
		}

		loggedInNotBackplaneConfig = api.Config{
			Kind:        "Config",
			APIVersion:  "v1",
			Preferences: api.Preferences{},
			Clusters: map[string]*api.Cluster{
				"myopenshiftcluster": {
					Server: "https://myopenshiftcluster.openshiftapps.com",
				},
			},
			Contexts: map[string]*api.Context{
				"default/myopenshiftcluster/example.openshift": {
					Cluster:   "myopenshiftcluster",
					Namespace: "default",
				},
			},
			CurrentContext: "default/myopenshiftcluster/example.openshift",
		}

		os.Setenv(info.BackplaneURLEnvName, backplaneAPIURI)
		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
	})

	AfterEach(func() {
		utils.RemoveTempKubeConfig()
		os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})

	Context("Test logout", func() {

		It("should be able to logout after login ", func() {

			err := utils.CreateTempKubeConfig(&kubeConfig)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil).AnyTimes()

			loginCmd.SetArgs([]string{testClusterID})
			err = loginCmd.Execute()

			Expect(err).To(BeNil())

			err = runLogout(nil, make([]string, 0))

			Expect(err).To(BeNil())

		})

		It("should not alter non backplane login", func() {

			err := utils.CreateTempKubeConfig(&loggedInNotBackplaneConfig)

			Expect(err).To(BeNil())

			initial, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())

			err = runLogout(nil, make([]string, 0))

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("you're not logged in using backplane"))

			after, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())

			Expect(initial).To(Equal(after))
		})

		It("should fail for empty kubeconfig yaml", func() {
			config, err := clientcmd.Load([]byte(""))
			Expect(err).To(BeNil())
			err = utils.CreateTempKubeConfig(config)

			Expect(err).To(BeNil())

			err = runLogout(nil, make([]string, 0))
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("current context does not exist"))
		})

		It("should fail for invalid kubeconfig yaml", func() {
			config, err := clientcmd.Load([]byte("hello: world"))
			Expect(err).To(BeNil())
			err = utils.CreateTempKubeConfig(config)

			Expect(err).To(BeNil())

			err = runLogout(nil, make([]string, 0))
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("current context does not exist"))

		})
	})
})
