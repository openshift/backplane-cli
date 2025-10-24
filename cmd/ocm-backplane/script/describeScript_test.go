package script

import (
	"errors"
	"net/http"
	"os"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	bpclient "github.com/openshift/backplane-api/pkg/client"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var _ = Describe("describe script command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils

		testClusterID  string
		testToken      string
		trueClusterID  string
		testScriptName string
		proxyURI       string
		fakeResp       *http.Response
		sut            *cobra.Command
		ocmEnv         *cmv1.Environment
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		testClusterID = "test123"
		testToken = "hello123"
		trueClusterID = "trueID123"
		testScriptName = "CEE/abc"

		proxyURI = "https://shard.apps"

		sut = NewScriptCmd()

		fakeResp = &http.Response{
			Body: MakeIoReader(`
[
{
  "allowedGroups":["CEE"],
  "author":"author",
  "canonicalName":"CEE/abc",
  "description":"desc",
  "language":"Python",
  "path":"something",
  "permalink":"https://link",
  "rbac": {},
  "envs": [{"key":"var1","description":"variable 1","optional":false}]
}
]
`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")
		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)

		_ = os.Setenv(info.BackplaneURLEnvName, proxyURI)
		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
	})

	AfterEach(func() {
		_ = os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})

	Context("describe script", func() {
		It("when running with a simple case should work as expected", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockClient.EXPECT().GetScriptsByCluster(gomock.Any(), trueClusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockClient.EXPECT().GetScriptsByCluster(gomock.Any(), trueClusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterID, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("Should able use the current logged in cluster if non specified and retrieve from config file", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq("configcluster")).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://api-backplane.apps.something.com").Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetTargetCluster("configcluster").Return(trueClusterID, testClusterID, nil)
			mockClient.EXPECT().GetScriptsByCluster(gomock.Any(), trueClusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName})
			err = sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockClient.EXPECT().GetScriptsByCluster(gomock.Any(), trueClusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &testScriptName}).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when backplane returns a blank script list with 200 status", func() {
			fakeRespBlank := &http.Response{
				Body:       MakeIoReader(`[]`),
				Header:     map[string][]string{},
				StatusCode: http.StatusOK,
			}
			fakeRespBlank.Header.Add("Content-Type", "json")

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockClient.EXPECT().GetScriptsByCluster(gomock.Any(), trueClusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &testScriptName}).Return(fakeRespBlank, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should still work if the result does not contain envs", func() {
			fakeRespNoEnv := &http.Response{
				Body: MakeIoReader(`[
{
  "allowedGroups":["CEE"],
  "author":"author",
  "canonicalName":"CEE/abc",
  "description":"desc",
  "language":"Python",
  "path":"something",
  "permalink":"https://link",
  "rbac": {}
}
]`),
				Header:     map[string][]string{},
				StatusCode: http.StatusOK,
			}
			fakeRespNoEnv.Header.Add("Content-Type", "json")

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockClient.EXPECT().GetScriptsByCluster(gomock.Any(), trueClusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &testScriptName}).Return(fakeRespNoEnv, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should not work when backplane returns a non parsable response with 200 return", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockClient.EXPECT().GetScriptsByCluster(gomock.Any(), trueClusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})
	})
})
