package session

import (
	"bufio"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/cli/globalflags"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	"github.com/spf13/cobra"
)

var _ = Describe("Backplane Session Unit test", func() {
	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *mocks2.MockOCMInterface
		mockClientUtil     *mocks2.MockClientUtils

		options   Options
		bpSession BackplaneSession
		cmd       *cobra.Command

		testClusterId   string
		testToken       string
		trueClusterId   string
		backplaneAPIUri string

		fakeResp *http.Response
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

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

		testClusterId = "test123"
		testToken = "hello123"
		trueClusterId = "trueID123"
		backplaneAPIUri = "https://api.integration.backplane.example.com"

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		os.Setenv(info.BACKPLANE_URL_ENV_NAME, backplaneAPIUri)
	})

	AfterEach(func() {
		bpSession = BackplaneSession{}
	})

	Context("check Backplane session setup", func() {
		It("Check backplane session default files", func() {
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

			mockOcmInterface.EXPECT().GetTargetCluster(options.Alias).Return("", "", errors.New("err")).AnyTimes()

			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("invalid cluster Id my-session"))
		})

		It("should fail for empty session name ", func() {
			options.Alias = ""
			options.ClusterId = ""

			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("ClusterId or Alias required"))
		})

		It("should use clusterID when alias is empty ", func() {
			options.Alias = ""
			options.ClusterId = testClusterId

			mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(options.ClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil).AnyTimes()

			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).To(BeNil())
			Expect(bpSession.Path).Should(ContainSubstring(testClusterId))
		})

		It("should contains cluster env vars ", func() {
			options.Alias = "test-env"
			options.ClusterId = testClusterId

			mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(options.ClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil).AnyTimes()

			err := bpSession.RunCommand(cmd, []string{})
			Expect(err).To(BeNil())
			Expect(bpSession.Path).Should(ContainSubstring("test-env"))

			envFile, err := os.Open(filepath.Join(bpSession.Path, ".ocenv"))
			Expect(err).To(BeNil())
			scanner := bufio.NewScanner(envFile)
			for scanner.Scan() {
				// check osEnv file contains KUBECONFIG and value contains session name and cluster-id
				if strings.Contains(scanner.Text(), "KUBECONFIG") {
					Expect(scanner.Text()).Should(ContainSubstring("test-env/" + trueClusterId))
				}

				// check osEnv file contains CLUSTERID and value contains cluster-id
				if strings.Contains(scanner.Text(), "CLUSTERID") {
					Expect(scanner.Text()).Should(ContainSubstring(trueClusterId))
				}

				// check osEnv file contains CLUSTERNAME and value contains cluster-name
				if strings.Contains(scanner.Text(), "CLUSTERNAME") {
					Expect(scanner.Text()).Should(ContainSubstring(testClusterId))
				}
			}
		})
	})

	Context("check Backplane session delete", func() {
		It("Session should delete ", func() {
			options.Alias = "my-session"
			options.ClusterId = testClusterId

			mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(options.ClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(trueClusterId).Return(trueClusterId, testClusterId, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil).AnyTimes()

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
