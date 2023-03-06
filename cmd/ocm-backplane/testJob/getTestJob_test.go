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

var _ = Describe("testJob get command", func() {

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

		fakeResp *http.Response

		sut *cobra.Command
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		tempDir, _ = os.MkdirTemp("", "createJobTest")

		_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(MetadataYaml), 0600)
		_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo hello"), 0600)

		_ = os.Chdir(tempDir)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

		testClusterId = "test123"
		testToken = "hello123"
		trueClusterId = "trueID123"
		proxyUri = "https://shard.apps"

		sut = NewTestJobCommand()

		fakeResp = &http.Response{
			Body: MakeIoReader(`
{"testId":"tid",
"logs":"",
"message":"",
"status":"Running"
}
`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")
		// Clear config file

	})

	AfterEach(func() {

		_ = os.RemoveAll(tempDir)
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		mockCtrl.Finish()
	})

	Context("get test job", func() {
		It("when running with a simple case should work as expected", func() {
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			mockClient.EXPECT().GetTestScriptRun(gomock.Any(), trueClusterId, gomock.Eq("tid")).Return(fakeResp, nil)

			sut.SetArgs([]string{"get", "tid", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().GetTestScriptRun(gomock.Any(), trueClusterId, gomock.Eq("tid")).Return(fakeResp, nil)

			sut.SetArgs([]string{"get", "tid", "--cluster-id", testClusterId, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			mockClient.EXPECT().GetTestScriptRun(gomock.Any(), trueClusterId, gomock.Eq("tid")).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"get", "tid", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().GetTestScriptRun(gomock.Any(), trueClusterId, gomock.Eq("tid")).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"get", "tid", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

	})
})
