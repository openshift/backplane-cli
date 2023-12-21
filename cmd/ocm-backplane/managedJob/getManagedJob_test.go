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

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("managedJob get command", func() {

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

		fakeResp         *http.Response
		fakeRespMultiple *http.Response

		sut    *cobra.Command
		ocmEnv *cmv1.Environment
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
			Body: MakeIoReader(`
{
  "jobId": "jid123",
  "message": "msg",
  "userMD5": "md5",
  "jobStatus": {
     "envs": [],
     "namespace": "ns",
     "script": {
       "canonicalName": "SREP/example"
    },
     "start": "2012-12-11T00:00:00+00:00",
     "status": "Running"
  }
}
`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		fakeRespMultiple = &http.Response{
			Body: MakeIoReader(`
[
{
  "jobId": "jid123",
  "message": "msg",
  "userMD5": "md5",
  "jobStatus": {
     "envs": [],
     "namespace": "ns",
     "script": {
       "canonicalName": "SREP/example"
    },
     "start": "2012-12-11T00:00:00+00:00",
     "status": "Running"
  }
},

{
  "jobId": "jid456",
  "message": "msg",
  "userMD5": "md5",
  "jobStatus": {
     "envs": [],
     "namespace": "ns",
     "script": {
       "canonicalName": "SREP/example"
    },
     "start": "2012-12-12T00:00:00+00:00",
     "status": "Running"
  }
}
]
`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeRespMultiple.Header.Add("Content-Type", "json")
		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)

		os.Setenv(info.BackplaneURLEnvName, proxyURI)
		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
	})

	AfterEach(func() {
		os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})

	Context("get a single managed job", func() {
		It("when running with a simple case should work as expected for single job", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(fakeResp, nil)

			sut.SetArgs([]string{"get", testJobID, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(fakeResp, nil)

			sut.SetArgs([]string{"get", testJobID, "--cluster-id", testClusterID, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"get", testJobID, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not work when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq(testJobID)).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"get", testJobID, "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})
	})

	Context("get a all managed jobs", func() {
		It("when running with a simple case should work as expected", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().GetAllJobs(gomock.Any(), trueClusterID).Return(fakeRespMultiple, nil)

			sut.SetArgs([]string{"get", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().GetAllJobs(gomock.Any(), trueClusterID).Return(fakeRespMultiple, nil)

			sut.SetArgs([]string{"get", "--cluster-id", testClusterID, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().GetAllJobs(gomock.Any(), trueClusterID).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"get", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not work when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			fakeRespMultiple.Body = MakeIoReader("Sad")
			mockClient.EXPECT().GetAllJobs(gomock.Any(), trueClusterID).Return(fakeRespMultiple, errors.New("err"))

			sut.SetArgs([]string{"get", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})
	})
})
