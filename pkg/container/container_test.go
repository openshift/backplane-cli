package container

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func TestIt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Console Container Test Suite")
}

var _ = Describe("console container implementation", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface

		capturedCommands [][]string

		pullSecret string
	)
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		os.Setenv("CONTAINER_ENGINE", PODMAN)

		capturedCommands = nil
		createCommand = func(prog string, args ...string) *exec.Cmd {
			command := []string{prog}
			command = append(command, args...)
			capturedCommands = append(capturedCommands, command)

			return exec.Command("true")
		}

		createCommand = func(prog string, args ...string) *exec.Cmd {
			command := []string{prog}
			command = append(command, args...)
			capturedCommands = append(capturedCommands, command)

			return exec.Command("true")
		}

		pullSecret = "testpullsecret"
		dirName, _ := os.MkdirTemp("", ".kube")
		pullSecretConfigDirectory = dirName
	})

	AfterEach(func() {
		os.Setenv("HTTPS_PROXY", "")
		mockCtrl.Finish()
		utils.RemoveTempKubeConfig()
	})

	Context("when running podman on MacOS", func() {
		ce := podmanMac{}
		It("should put the file via ssh to the VM", func() {
			capturedCommands = nil
			err := ce.PutFileToMount("testfile", []byte("test"))
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			command := capturedCommands[0]
			// executing with bash -c xxxx
			Expect(len(command)).To(Equal(3))
			Expect(command[2]).To(ContainSubstring("ssh"))
		})
	})

	Context("when running docker", func() {
		ce := dockerLinux{}
		It("should specify the --config option right after the subcommand", func() {
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			err := ce.PullImage("testimage")
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			command := capturedCommands[0]
			// executing docker --config xxxx pull
			Expect(len(command)).To(BeNumerically(">", 3))
			Expect(command[1]).To(ContainSubstring("config"))
		})
	})

	Context("when getting a container engine implementation", func() {
		It("should return an error if no such implementation", func() {
			ce, err := NewEngine("UnknownOS", "UnknownContainer")
			Expect(ce).To(BeNil())
			Expect(err).ToNot(BeNil())
		})
	})

	Context("when running console container", func() {
		ce := podmanMac{}
		It("should pass argments and environment variable if specified", func() {
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunConsoleContainer("console", "8888", args, envvars)
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			fullCommand := strings.Join(capturedCommands[0], " ")
			// arg
			Expect(fullCommand).To(ContainSubstring("arg1"))
			// env var
			Expect(fullCommand).To(ContainSubstring("--env"))
			Expect(fullCommand).To(ContainSubstring("testkey=testval"))
		})
	})

})
