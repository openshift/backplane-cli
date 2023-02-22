package managedJob

import (
	"errors"
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("managedJob delete command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *mocks2.MockOCMInterface
		mockClientUtil   *mocks2.MockClientUtils

		testClusterId string
		testToken     string
		trueClusterId string
		proxyUri      string
		testJobId     string

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
		testJobId = "jid123"

		sut = NewManagedJobCmd()

		fakeResp = &http.Response{
			Body:       MakeIoReader(""),
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

	Context("delete managed job", func() {
		It("when running with a simple case should work as expected", func() {
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)

			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterId, gomock.Eq(testJobId)).Return(fakeResp, nil)

			sut.SetArgs([]string{"delete", testJobId, "--cluster-id", testClusterId, "-y"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().GetBackplaneURL().Return("https://newbackplane.url", nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterId, gomock.Eq(testJobId)).Return(fakeResp, nil)

			sut.SetArgs([]string{"delete", testJobId, "--cluster-id", testClusterId, "--url", "https://newbackplane.url", "-y"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterId, gomock.Eq(testJobId)).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"delete", testJobId, "--cluster-id", testClusterId, "-y"})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not work when backplane returns a non parsable response with 200 return", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyUri).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().DeleteJob(gomock.Any(), trueClusterId, gomock.Eq(testJobId)).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"delete", testJobId, "--cluster-id", testClusterId, "-y"})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})
	})
})
