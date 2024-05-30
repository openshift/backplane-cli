package managedjob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("managedJob create command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils
		mockCluster              *cmv1.Cluster

		testClusterID string
		testToken     string
		trueClusterID string
		proxyURI      string

		fakeResp        *http.Response
		fakeJobResp     *http.Response
		fakeLoginResp     *http.Response
		jobResponseBody string

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
		mockCluster = &cmv1.Cluster{}
		isManagementCluster = func () (bool, error) { return true, nil}
		listDynaKube = func(cl client.Client,ctx context.Context, u client.ObjectList, opts ...client.ListOption) error{
			ux := u.(*unstructured.Unstructured)

			st := map[string]interface{} {  
				"spec": map[string]interface{}{
					"apiUrl": "https://staging.live.dynatrace.com/api",
				}, 
			}

			ux.Object["items"] = []interface{}{
				st,
			}
			return nil
		}

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

		// fake response for login to get login.GetRestConfig
		fakeLoginResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeLoginResp.Header.Add("Content-Type", "json")

		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)

		os.Setenv(info.BackplaneURLEnvName, proxyURI)
		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
	})

	AfterEach(func() {
		os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})

	Context("create managed job", func() {
		It("when running with a simple case should work as expected", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(mockCluster, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeLoginResp, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			// Then it will look for the backplane shard
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(mockCluster, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeLoginResp, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(gomock.Any()).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should wait for job to be finished", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(mockCluster, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeLoginResp, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq("jid")).Return(fakeJobResp, nil)

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID, "--url", "https://newbackplane.url", "--wait"})

			outPuts := bytes.NewBufferString("")
			sut.SetOut(outPuts)
			err := sut.Execute()

			Expect(err).To(BeNil())

			outPutText, _ := io.ReadAll(outPuts)
			Expect(string(outPutText)).Should(ContainSubstring("Job Succeeded"))
		})

		It("should timeout if job waiting in pending status for long time", func() {
			fakeJobResp = &http.Response{
				Body:       MakeIoReader(fmt.Sprintf(jobResponseBody, "Pending")),
				Header:     map[string][]string{},
				StatusCode: http.StatusOK,
			}
			fakeJobResp.Header.Add("Content-Type", "json")

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(mockCluster, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeLoginResp, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil).AnyTimes()
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq("jid")).Return(fakeJobResp, nil).AnyTimes()

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID, "--url", "https://newbackplane.url", "--wait"})
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

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(mockCluster, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeLoginResp, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil).AnyTimes()
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq("jid")).Return(fakeJobResp, nil).AnyTimes()

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID, "--url", "https://newbackplane.url", "--wait"})

			outPuts := bytes.NewBufferString("")
			sut.SetOut(outPuts)
			err := sut.Execute()

			Expect(err).To(BeNil())

			outPutText, _ := io.ReadAll(outPuts)
			Expect(string(outPutText)).Should(ContainSubstring("Job Failed"))
		})

		It("should stream the log of the job if it is in running status", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(mockCluster, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeLoginResp, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq("jid")).Return(fakeJobResp, nil)
			mockClient.EXPECT().GetJobLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fakeJobResp, nil)

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID, "--url", "https://newbackplane.url", "--logs"})

			outPuts := bytes.NewBufferString("")
			sut.SetOut(outPuts)
			err := sut.Execute()

			Expect(err).To(BeNil())

			outPutText, _ := io.ReadAll(outPuts)
			Expect(string(outPutText)).Should(ContainSubstring("fetching logs for"))
		})

		It("should exit if the job cannot be ready", func() {
			fakeJobResp = &http.Response{
				Body:       MakeIoReader(fmt.Sprintf(jobResponseBody, "Pending")),
				Header:     map[string][]string{},
				StatusCode: http.StatusOK,
			}
			fakeJobResp.Header.Add("Content-Type", "json")
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(mockCluster, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeLoginResp, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateJob(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)
			mockClient.EXPECT().GetRun(gomock.Any(), trueClusterID, gomock.Eq("jid")).Return(fakeJobResp, nil).AnyTimes()

			sut.SetArgs([]string{"create", "SREP/something", "--cluster-id", testClusterID, "--url", "https://newbackplane.url", "--logs"})

			err := sut.Execute()

			Expect(err).NotTo(BeNil())
		})
	})
})
