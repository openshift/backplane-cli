package session

import (
	"bufio"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("Backplane Session Unit test", func() {
	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils

		options   Options
		bpSession BackplaneSession
		cmd       *cobra.Command

		testClusterID string
		testToken     string
		trueClusterID string

		fakeResp *http.Response
		ocmEnv   *cmv1.Environment
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		options = Options{
			GlobalOpts: &globalflags.GlobalOptions{},
		}

		// create temp session
		sessionPath, err := os.MkdirTemp("", "bp-session")
		Expect(err).To(BeNil())

		bpSession = BackplaneSession{
			Options: &options,
			Path:    sessionPath,
		}

		cmd = &cobra.Command{
			Use: "session",
		}

		testClusterID = "test123"
		testToken = "hello123"
		trueClusterID = "trueID123"
		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"https://api.integration.backplane.example.com", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")
		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://api.integration.backplane.example.com").Build()
	})

	AfterEach(func() {
		bpSession = BackplaneSession{}
	})

	Context("check Backplane session setup", func() {
		It("Check backplane session default files", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(options.Alias).Return("", "", errors.New("err")).AnyTimes()
			err := bpSession.Setup()

			Expect(err).To(BeNil())

			// Check history file
			historyFile, err := os.Stat(filepath.Join(bpSession.Path, ".history"))
			Expect(err).To(BeNil())
			Expect(historyFile).NotTo(BeNil())

			// check bash env file
			ocEnvFile, err := os.Stat(filepath.Join(bpSession.Path, ".ocenv"))
			Expect(err).To(BeNil())
			Expect(ocEnvFile).NotTo(BeNil())

			// check zsh env file
			zshEnvFile, err := os.Stat(filepath.Join(bpSession.Path, ".zshenv"))
			Expect(err).To(BeNil())
			Expect(zshEnvFile).NotTo(BeNil())
		})

		It("Check backplane session folder permissions", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(options.Alias).Return("", "", errors.New("err")).AnyTimes()
			err := bpSession.Setup()

			Expect(err).To(BeNil())

			// Write Kube config file to session folder
			kubeConfigWrite, err := os.Create(filepath.Join(bpSession.Path, "config"))
			Expect(err).To(BeNil())
			Expect(kubeConfigWrite).NotTo(BeNil())

			// Read Kube config file from session folder
			kubeConfigRead, err := os.Stat(filepath.Join(bpSession.Path, "config"))
			Expect(err).To(BeNil())
			Expect(kubeConfigRead).NotTo(BeNil())

		})
	})

	Context("test Backplane Run Command", func() {
		It("should fail for invalid cluster alias name ", func() {
			options.Alias = "my-session"

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(options.Alias).Return("", "", errors.New("err")).AnyTimes()

			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("failed to find cluster my-session"))
		})

		It("should fail for empty session name ", func() {
			options.Alias = ""
			options.ClusterID = ""

			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("ClusterID or Alias required"))
		})

		It("should contains cluster env vars ", func() {
			options.Alias = "test-env"
			options.ClusterID = testClusterID

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster("mc_id").Return("mc_id", "mc_name", nil).AnyTimes()
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterID).Return("mc_id", "mc_name", nil).AnyTimes()
			mockOcmInterface.EXPECT().GetManagingCluster(testClusterID).Return("mc_id", "mc_name", nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Any()).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(gomock.Any(), testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), "mc_id").Return(func() *http.Response {
				response := http.Response{
					Body:       ioutil.NopCloser(strings.NewReader(`{"proxy_uri":"https://api.integration.backplane.example.com"}`)),
					Header:     map[string][]string{},
					StatusCode: http.StatusOK,
				}
				response.Header.Add("Content-Type", "json")
				return &response
			}(), nil).AnyTimes()

			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).To(BeNil())
			Expect(bpSession.Path).Should(ContainSubstring(options.Alias))

			envFile, err := os.Open(filepath.Join(bpSession.Path, ".ocenv"))
			Expect(err).To(BeNil())
			scanner := bufio.NewScanner(envFile)
			for scanner.Scan() {
				// check osEnv file contains KUBECONFIG and value contains session name and cluster-id
				if strings.Contains(scanner.Text(), "KUBECONFIG") {
					Expect(scanner.Text()).Should(ContainSubstring("test-env/" + trueClusterID))
				}

				// check osEnv file contains CLUSTERID and value contains cluster-id
				if strings.Contains(scanner.Text(), "CLUSTERID") {
					Expect(scanner.Text()).Should(ContainSubstring(trueClusterID))
				}

				// check osEnv file contains CLUSTERNAME and value contains cluster-name
				if strings.Contains(scanner.Text(), "CLUSTERNAME") {
					Expect(scanner.Text()).Should(ContainSubstring(testClusterID))
				}
			}
		})
	})

	Context("check Backplane session delete", func() {
		It("Session should delete ", func() {
			options.Alias = "my-session"
			options.ClusterID = testClusterID

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster("mc_id").Return("mc_id", "mc_name", nil).AnyTimes()
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterID).Return("mc_id", "mc_name", nil).AnyTimes()
			mockOcmInterface.EXPECT().GetManagingCluster(testClusterID).Return("mc_id", "mc_name", nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Any()).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(gomock.Any(), testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), "mc_id").Return(func() *http.Response {
				response := http.Response{
					Body:       ioutil.NopCloser(strings.NewReader(`{"proxy_uri":"https://api.integration.backplane.example.com"}`)),
					Header:     map[string][]string{},
					StatusCode: http.StatusOK,
				}
				response.Header.Add("Content-Type", "json")
				return &response
			}(), nil).AnyTimes()
			// Create the session
			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).To(BeNil())
			Expect(bpSession.Path).Should(ContainSubstring("my-session"))

			// Delete the session
			options.DeleteSession = true
			err = bpSession.RunCommand(cmd, []string{})
			Expect(err).To(BeNil())

			_, err = os.Stat(bpSession.Path)
			Expect(err.Error()).Should(ContainSubstring("no such file or directory"))
		})
	})
})
