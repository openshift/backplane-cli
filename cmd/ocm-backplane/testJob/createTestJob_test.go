package testJob

import (
	"errors"
	"net/http"
	"os"
	"path"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
)

const MetadataYaml = `
file: script.sh
name: example
description: just an example
author: dude
allowedGroups: 
  - SREP
rbac:
    roles:
      - namespace: "kube-system"
        rules:
          - verbs:
            - "*"
            apiGroups:
            - ""
            resources:
            - "*"
            resourceNames:
            - "*"
    clusterRoleRules:
        - verbs:
            - "*"
          apiGroups:
            - ""
          resources:
            - "*"
          resourceNames:
            - "*"
language: bash
`

var _ = Describe("testJob create command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *mocks2.MockOCMInterface
		mockClientUtil   *mocks2.MockClientUtils

		testClusterId string
		testToken     string
		trueClusterId string
		proxyUri      string
		tempDir       string
		testKubeCfg   api.Config
		/*testKubeCfg = api.Config{
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
		}*/

		fakeResp *http.Response

		sut *cobra.Command
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		tempDir, _ = os.MkdirTemp("", "createJobTest")

		_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(MetadataYaml), 0755)
		_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo hello"), 0755)

		_ = os.Chdir(tempDir)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

		testClusterId = "test123"
		testToken = "hello123"
		trueClusterId = "trueID123"
		proxyUri = "https://shard.apps"

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

		sut = NewTestJobCommand()

		fakeResp = &http.Response{
			Body: MakeIoReader(`
{"testId":"tid",
"logs":"",
"message":"",
"status":"Pending"
}
`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		mockCtrl.Finish()
	})

	Context("create test job", func() {
		It("when running with a simple case should work as expected", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterId, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("Should able use the current logged in cluster if non specified and retrieve from config file", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			pathOptions := clientcmd.NewDefaultPathOptions()
			err := clientcmd.ModifyConfig(pathOptions, testKubeCfg, true)
			clientcmd.UseModifyConfigLock = false
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetBackplaneURL().Return("https://api-backplane.apps.something.com", nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq("configcluster")).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://api-backplane.apps.something.com").Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), "configcluster", gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create"})
			err = sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterId, gomock.Any()).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when metadata is not found/invalid", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)

			_ = os.Remove(path.Join(tempDir, "metadata.yaml"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when script file is not found/invalid", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not run in production environment", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(true, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})
	})
})
