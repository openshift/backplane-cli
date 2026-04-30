package container

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
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

		_ = os.Setenv("CONTAINER_ENGINE", PODMAN)

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
		_ = os.Setenv("HTTPS_PROXY", "")
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

	Context("when checking Rosetta on macOS Podman", func() {
		It("should execute podman machine ssh command to check binfmt_misc", func() {
			capturedCommands = nil
			checkRosettaEnabled()
			Expect(len(capturedCommands)).To(Equal(1))
			command := capturedCommands[0]
			Expect(command[0]).To(Equal(PODMAN))
			Expect(command[1]).To(Equal("machine"))
			Expect(command[2]).To(Equal("ssh"))
			Expect(strings.Join(command[3:], " ")).To(Equal("ls /proc/sys/fs/binfmt_misc/"))
		})
	})

	Context("when running console container on macOS", func() {
		ce := podmanMac{}
		It("should check Rosetta before running the container", func() {
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunConsoleContainer("console", "8888", args, envvars)
			Expect(err).To(BeNil())
			// Should have 2 commands: 1 for Rosetta check, 1 for running container
			Expect(len(capturedCommands)).To(BeNumerically(">=", 2))
			// First command should be Rosetta check
			rosettaCheckCmd := capturedCommands[0]
			Expect(rosettaCheckCmd[0]).To(Equal(PODMAN))
			Expect(rosettaCheckCmd[1]).To(Equal("machine"))
			Expect(rosettaCheckCmd[2]).To(Equal("ssh"))
			// Last command should be the actual container run
			runCmd := capturedCommands[len(capturedCommands)-1]
			fullCommand := strings.Join(runCmd, " ")
			Expect(fullCommand).To(ContainSubstring("arg1"))
			Expect(fullCommand).To(ContainSubstring("--env"))
			Expect(fullCommand).To(ContainSubstring("testkey=testval"))
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
			Expect(len(capturedCommands)).To(BeNumerically(">=", 1))
			// Find the run command (should be the last one)
			fullCommand := strings.Join(capturedCommands[len(capturedCommands)-1], " ")
			// arg
			Expect(fullCommand).To(ContainSubstring("arg1"))
			// env var
			Expect(fullCommand).To(ContainSubstring("--env"))
			Expect(fullCommand).To(ContainSubstring("testkey=testval"))
		})
	})

	Context("when running monitoring plugin container in podman", func() {
		It("should pass argments and environment variable to podman if specified", func() {
			ce := podmanMac{}
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunMonitorPlugin("monitoring-plugin-1234", "console-1234", "/tmp/nginx.conf", args, envvars)
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			fullCommand := strings.Join(capturedCommands[0], " ")
			// arg
			Expect(fullCommand).To(ContainSubstring("arg1"))
			// env var
			Expect(fullCommand).To(ContainSubstring("--env"))
			Expect(fullCommand).To(ContainSubstring("testkey=testval"))
		})
		It("should not mount the nginx conf file if the path is empty - Mac", func() {
			ce := podmanMac{}
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunMonitorPlugin("monitoring-plugin-1234", "console-1234", "", args, envvars)
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			fullCommand := strings.Join(capturedCommands[0], " ")
			Expect(fullCommand).ToNot(ContainSubstring("--mount"))
		})
		It("should not mount the nginx conf file if the path is empty - Linux", func() {
			ce := podmanLinux{}
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunMonitorPlugin("monitoring-plugin-1234", "console-1234", "", args, envvars)
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			fullCommand := strings.Join(capturedCommands[0], " ")
			Expect(fullCommand).ToNot(ContainSubstring("--mount"))
		})
	})

	Context("when running monitoring plugin container in docker", func() {
		It("should pass argments and environment variable to docker if specified", func() {
			ce := dockerLinux{}
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunMonitorPlugin("monitoring-plugin-1234", "console-1234", "/tmp/nginx.conf", args, envvars)
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			fullCommand := strings.Join(capturedCommands[0], " ")
			// arg
			Expect(fullCommand).To(ContainSubstring("arg1"))
			// env var
			Expect(fullCommand).To(ContainSubstring("--env"))
			Expect(fullCommand).To(ContainSubstring("testkey=testval"))
		})
		It("should not mount the nginx conf file if the path is empty - Linux", func() {
			ce := dockerLinux{}
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunMonitorPlugin("monitoring-plugin-1234", "console-1234", "", args, envvars)
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			fullCommand := strings.Join(capturedCommands[0], " ")
			Expect(fullCommand).ToNot(ContainSubstring("--volume"))
		})
		It("should not mount the nginx conf file if the path is empty - Mac", func() {
			ce := dockerMac{}
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			capturedCommands = nil
			args := []string{"arg1"}
			envvars := []EnvVar{{Key: "testkey", Value: "testval"}}
			err := ce.RunMonitorPlugin("monitoring-plugin-1234", "console-1234", "", args, envvars)
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			fullCommand := strings.Join(capturedCommands[0], " ")
			Expect(fullCommand).ToNot(ContainSubstring("--volume"))
		})
	})
})
