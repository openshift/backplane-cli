package config

import (
	"fmt"
	"os"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("troubleshoot command", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface

		printedCorrects []string
		printedWrongs   []string
		printedNotices  []string
		printedNormals  []string
	)

	// wrong backplane configuration for testing.
	// it misses a comma in json.
	wrongBPConfig := `
	{
		"proxy-url": "http://example:8888"
		"assume-initial-arn": "arn:aws:iam::12345678:role/Support-Role"
	}
	`
	goodBPConfig := `
	{
		"proxy-url": "http://example:8888",
		"assume-initial-arn": "arn:aws:iam::12345678:role/Support-Role"
	}
	`
	ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()

	setupBPconfig := func(config string) {
		f, err := os.CreateTemp("", "test-bp-config")
		Expect(err).To(BeNil())

		_, err = f.WriteString(config)
		Expect(err).To(BeNil())

		err = f.Close()
		Expect(err).To(BeNil())

		// set backplane config env with temp config file
		os.Setenv("BACKPLANE_CONFIG", f.Name())
	}

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		printedCorrects = []string{}
		printedWrongs = []string{}
		printedNotices = []string{}
		printedNormals = []string{}

		printCorrect = func(format string, a ...any) {
			content := fmt.Sprintf(format, a...)
			printedCorrects = append(printedCorrects, content)
		}
		printWrong = func(format string, a ...any) {
			content := fmt.Sprintf(format, a...)
			printedWrongs = append(printedWrongs, content)
		}
		printNotice = func(format string, a ...any) {
			content := fmt.Sprintf(format, a...)
			printedNotices = append(printedNotices, content)
		}
		printf = func(format string, a ...any) {
			content := fmt.Sprintf(format, a...)
			printedNormals = append(printedNormals, content)
		}
	})
	AfterEach(func() {})
	Context("For backplane configuration", func() {
		It("should print error if backplane configuration is wrong", func() {
			setupBPconfig(wrongBPConfig)
			o := troubleshootOptions{}
			err := o.checkBPCli()
			Expect(err).To(BeNil())
			Expect(len(printedWrongs)).To(Equal(1))
			Expect(printedWrongs[0]).To(ContainSubstring("Failed to read backplane-cli config file"))
		})
		It("should print the proxy url if backplane configuration is correct", func() {
			setupBPconfig(goodBPConfig)
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			o := troubleshootOptions{}
			getBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				result := "http://example:8888"
				bpConfig.ProxyURL = &result
				return bpConfig, nil
			}
			err := o.checkBPCli()
			Expect(err).To(BeNil())
			Expect(len(printedCorrects)).To(Equal(2))
			Expect(printedCorrects[1]).To(ContainSubstring("http://example:8888"))
		})
	})
	Context("For OCM environment", func() {
		It("should print error if anything wrong", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(nil, fmt.Errorf("something wrong")).AnyTimes()
			o := troubleshootOptions{}
			err := o.checkOCM()
			Expect(err).To(BeNil())
			Expect(len(printedWrongs)).To(Equal(1))
			Expect(printedWrongs[0]).To(ContainSubstring("something wrong"))
		})
		It("should print the backplane url of the current OCM environment", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			o := troubleshootOptions{}
			err := o.checkOCM()
			Expect(err).To(BeNil())
			Expect(len(printedCorrects)).To(Equal(1))
			Expect(printedCorrects[0]).To(ContainSubstring("https://backplane.example.com"))
		})
	})
	Context("For OC environment", func() {
		testKubeConfig := api.Config{
			Kind:        "Config",
			APIVersion:  "v1",
			Preferences: api.Preferences{},
			Clusters: map[string]*api.Cluster{
				"dummy_cluster": {
					Server:   "https://backplane.example.com/backplane/cluster/dummycluster",
					ProxyURL: "https://proxy.example.com",
				},
			},
			Contexts: map[string]*api.Context{
				"default/test123/anonymous": {
					Cluster:   "dummy_cluster",
					Namespace: "default",
				},
			},
			CurrentContext: "default/test123/anonymous",
		}
		It("should print error if anything wrong in oc config", func() {
			os.Setenv("KUBECONFIG", "/fake/path")
			o := troubleshootOptions{}
			err := o.checkOC()
			Expect(err).To(BeNil())
			Expect(printedWrongs[0]).To(ContainSubstring("Failed to get OC configuration"))
		})
		It("should print the server url in oc config", func() {
			err := utils.CreateTempKubeConfig(&testKubeConfig)
			Expect(err).To(BeNil())
			o := troubleshootOptions{}
			err = o.checkOC()
			Expect(err).To(BeNil())
			Expect(len(printedCorrects)).To(BeNumerically(">=", 1))
			Expect(printedCorrects[0]).To(ContainSubstring("https://backplane.example.com/backplane/cluster/dummycluster"))
		})
		It("should print the proxy url in oc config", func() {
			// the github CI doesn't have the OC command, need to mock it.
			execOCProxy = func() ([]byte, error) {
				return []byte("https://proxy.example.com"), nil
			}
			err := utils.CreateTempKubeConfig(&testKubeConfig)
			Expect(err).To(BeNil())
			o := troubleshootOptions{}
			err = o.checkOC()
			Expect(err).To(BeNil())
			Expect(len(printedCorrects)).To(BeNumerically(">=", 2))
			Expect(printedCorrects[1]).To(ContainSubstring("https://proxy.example.com"))
		})
	})
})
