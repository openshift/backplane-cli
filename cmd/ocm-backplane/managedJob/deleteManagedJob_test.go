package managedjob

import (
	"errors"
	"net/http"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("managedJob delete command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils

		testClusterID string
		testToken     string
		trueClusterID string
		proxyURI      string
		testJobID     string

		fakeResp *http.Response

		sut *cobra.Command
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
		proxyURI = "https://shard.apps"
		testJobID = "jid123"

		sut = NewManagedJobCmd()

		fakeResp = &http.Response{
			Body:       MakeIoReader(""),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")
		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)

		os.Setenv(info.BackplaneURLEnvName, proxyURI)
	})

	AfterEach(func() {
		os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})

	Context("delete managed job", func() {
		It("when running with a simple case should work as expected", func() {
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(fakeResp, nil)

			sut.SetArgs([]string{"delete", testJobID, "--cluster-id", testClusterID, "-y"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(fakeResp, nil)

			sut.SetArgs([]string{"delete", testJobID, "--cluster-id", testClusterID, "--url", "https://newbackplane.url", "-y"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"delete", testJobID, "--cluster-id", testClusterID, "-y"})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not work when backplane returns a non parsable response with 200 return", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"delete", testJobID, "--cluster-id", testClusterID, "-y"})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})
	})
})
