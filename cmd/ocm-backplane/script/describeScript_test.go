package script

import (
	"errors"
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	bpclient "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
)

var _ = Describe("describe script command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *mocks2.MockOCMInterface
		mockClientUtil   *mocks2.MockClientUtils

		testClusterId  string
		testToken      string
		trueClusterId  string
		proxyUri       string
		testKubeCfg    api.Config
		testScriptName string

		fakeResp *http.Response

		sut *cobra.Command
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

		testClusterId = "test123"
		testToken = "hello123"
		trueClusterId = "trueID123"
		proxyUri = "https://shard.apps"
		testScriptName = "CEE/abc"

		testKubeCfg = api.Config{
			Kind:        "Config",
			APIVersion:  "v1",
			Preferences: api.Preferences{},
			Clusters: map[string]*api.Cluster{
				"testcluster": {
					Server: "https://api-backplane.apps.something.com/backplane/cluster/configcluster",
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
			},
			CurrentContext: "default/testcluster/testauth",
			Extensions:     nil,
		}

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
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("describe script", func() {
		It("when running with a simple case should work as expected", func() {
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().GetScripts(gomock.Any(), &bpclient.GetScriptsParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().GetScripts(gomock.Any(), &bpclient.GetScriptsParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterId, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("Should able use the current logged in cluster if non specified and retrieve from config file", func() {
			pathOptions := clientcmd.NewDefaultPathOptions()
			err := clientcmd.ModifyConfig(pathOptions, testKubeCfg, true)
			clientcmd.UseModifyConfigLock = false
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq("configcluster")).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://api-backplane.apps.something.com").Return(mockClient, nil)
			mockClient.EXPECT().GetScripts(gomock.Any(), &bpclient.GetScriptsParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName})
			err = sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().GetScripts(gomock.Any(), &bpclient.GetScriptsParams{Scriptname: &testScriptName}).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterId})
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

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().GetScripts(gomock.Any(), &bpclient.GetScriptsParams{Scriptname: &testScriptName}).Return(fakeRespBlank, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterId})
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
			
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().GetScripts(gomock.Any(), &bpclient.GetScriptsParams{Scriptname: &testScriptName}).Return(fakeRespNoEnv, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should not work when backplane returns a non parsable response with 200 return", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().GetScripts(gomock.Any(), &bpclient.GetScriptsParams{Scriptname: &testScriptName}).Return(fakeResp, nil)

			sut.SetArgs([]string{"describe", testScriptName, "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})
	})
})
