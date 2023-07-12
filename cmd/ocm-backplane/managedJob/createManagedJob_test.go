package managedJob

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("managedJob create command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *mocks2.MockOCMInterface
		mockClientUtil   *mocks2.MockClientUtils

		testClusterId string
		testToken     string
		trueClusterId string
		proxyUri      string

		fakeResp        *http.Response
		fakeJobResp     *http.Response
		jobResponseBody string

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

		sut = NewManagedJobCmd()

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"jobId":"jid","jobStatus":{},"message":"msg","userMD5":"md5"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		// fake job response
		jobResponseBody = `
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
		"status": "%s"
	}
}
		`
		fakeJobResp = &http.Response{
			Body:       MakeIoReader(fmt.Sprintf(jobResponseBody, "Succeeded")),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeJobResp.Header.Add("Content-Type", "json")

		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)

		os.Setenv(info.BACKPLANE_URL_ENV_NAME, proxyUri)
	})

	AfterEach(func() {
		os.Setenv(info.BACKPLANE_URL_ENV_NAME, "")
		mockCtrl.Finish()
	})

	Context("create managed job", func() {
		It("when running with a simple case should work as expected", func() {
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterId, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterId, gomock.Any()).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterId})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should wait for job to be finished", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, nil)
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterId, gomock.Eq("jid")).Return(fakeJobResp, nil)

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterId, "--url", "https://newbackplane.url", "--wait"})

			outPuts := bytes.NewBufferString("")
			sut.SetOut(outPuts)
			err := sut.Execute()

			Expect(err).To(BeNil())

			outPutText, _ := ioutil.ReadAll(outPuts)
			Expect(string(outPutText)).Should(ContainSubstring("Job Succeeded"))
		})

		It("should timeout if job waiting in pending status for long time", func() {
			fakeJobResp = &http.Response{
				Body:       MakeIoReader(fmt.Sprintf(jobResponseBody, "Pending")),
				Header:     map[string][]string{},
				StatusCode: http.StatusOK,
			}
			fakeJobResp.Header.Add("Content-Type", "json")

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, nil).AnyTimes()
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterId, gomock.Eq("jid")).Return(fakeJobResp, nil).AnyTimes()

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterId, "--url", "https://newbackplane.url", "--wait"})
			err := sut.Execute()

			Expect(err).NotTo(BeNil())
		})

		It("should show a job failed message if job stuck in Failed status", func() {
			fakeJobResp = &http.Response{
				Body:       MakeIoReader(fmt.Sprintf(jobResponseBody, "Failed")),
				Header:     map[string][]string{},
				StatusCode: http.StatusOK,
			}
			fakeJobResp.Header.Add("Content-Type", "json")

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterId, gomock.Any()).Return(fakeResp, nil).AnyTimes()
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterId, gomock.Eq("jid")).Return(fakeJobResp, nil).AnyTimes()

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterId, "--url", "https://newbackplane.url", "--wait"})

			outPuts := bytes.NewBufferString("")
			sut.SetOut(outPuts)
			err := sut.Execute()

			Expect(err).To(BeNil())

			outPutText, _ := ioutil.ReadAll(outPuts)
			Expect(string(outPutText)).Should(ContainSubstring("Job Failed"))
		})
	})
})
