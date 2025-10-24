package testjob

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/konfig"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	MetadataYaml = `
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
)

var _ = Describe("testJob create command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils

		testClusterID string
		testToken     string
		trueClusterID string
		proxyURI      string
		tempDir       string
		sourceDir     string
		workingDir    string

		fakeResp *http.Response

		sut    *cobra.Command
		ocmEnv *cmv1.Environment
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		workingDir = konfig.CurrentWorkingDir()

		tempDir, _ = os.MkdirTemp("", "createJobTest")

		_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(MetadataYaml), 0600)
		_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo hello"), 0600)

		_ = os.Chdir(tempDir)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		testClusterID = "test123"
		testToken = "hello123"
		trueClusterID = "trueID123"
		proxyURI = "https://shard.apps"

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
		_ = os.Setenv(info.BackplaneURLEnvName, proxyURI)
		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
	})

	AfterEach(func() {
		_ = os.Setenv(info.BackplaneURLEnvName, "")
		_ = os.RemoveAll(tempDir)
		// Clear kube config file
		utils.RemoveTempKubeConfig()
		mockCtrl.Finish()
	})

	Context("create test job", func() {
		It("when running with a simple case should work as expected", func() {

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect the base image when supplied as a flag", func() {

			baseImgOverride := "quay.io/foo/bar"
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--base-image-override", baseImgOverride})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should be able to use the specified script dir", func() {

			sourceDir, _ = os.MkdirTemp("", "manualScriptDir")
			_ = os.WriteFile(path.Join(sourceDir, "metadata.yaml"), []byte(MetadataYaml), 0600)
			_ = os.WriteFile(path.Join(sourceDir, "script.sh"), []byte("echo hello"), 0600)
			defer func() { _ = os.RemoveAll(sourceDir) }()

			_ = os.Chdir(workingDir)

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--source-dir", sourceDir})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should return with correct error message when the given source dir is incorrect", func() {
			nonExistDir := "testDir"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--source-dir", nonExistDir})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("does not exist or it is not a directory"))
		})

		It("Should able use the current logged in cluster if non specified and retrieve from config file", func() {
			_ = os.Setenv(info.BackplaneURLEnvName, "https://api-backplane.apps.something.com")
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq("configcluster")).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://api-backplane.apps.something.com").Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), "configcluster", gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create"})
			err = sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when metadata is not found/invalid", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			_ = os.Remove(path.Join(tempDir, "metadata.yaml"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when script file is not found/invalid", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not run in production environment", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(true, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not run in production environment", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(true, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should import and inline a library file in the same directory", func() {
			script := `#!/bin/bash
set -eo pipefail

source /managed-scripts/lib.sh

echo_touch "Hello"
`
			lib := fmt.Sprintf(`function echo_touch () {
    echo $1 > %s/ran_function
}
`, tempDir)

			GetGitRepoPath = exec.Command("echo", tempDir) //nolint:gosec
			// tmp/createJobTest3397561583
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte(script), 0600)
			_ = os.Mkdir(path.Join(tempDir, "scripts"), 0755) //nolint:gosec
			_ = os.WriteFile(path.Join(tempDir, "scripts", "lib.sh"), []byte(lib), 0600)
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail if a required script parameter is missing", func() {
			// Add a required env to metadata
			newMetadata := `
file: script.sh
name: example
description: just an example
author: dude
allowedGroups:
  - SREP
envs:
  - key: REQUIRED_VAR
    description: "A required parameter"
    optional: false
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
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(newMetadata), 0600)

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("missing required parameter"))
		})

		It("should fail the parameter validation if an invalid parameter is provided", func() {
			// Add an optional env to metadata
			newMetadata := `
file: script.sh
name: example
description: just an example
author: dude
allowedGroups:
  - SREP
envs:
  - key: SOME_VAR
    description: "Some parameter"
    optional: true
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
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(newMetadata), 0600)

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "-p", "INVALID_ENV=123"})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid parameter"))
		})

		It("should pass parameter validation if all parameters entered are valid", func() {
			// Add an optional env to metadata
			newMetadata := `
file: script.sh
name: example
description: just an example
author: dude
allowedGroups:
  - SREP
envs:
  - key: VALID_PARAMETER
    description: "A valid parameter"
    optional: true
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
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(newMetadata), 0600)

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "-p", "VALID_PARAMETER=abc"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

	})
})
