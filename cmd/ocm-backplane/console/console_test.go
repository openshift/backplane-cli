package console

import (
	"os"
	"reflect"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"

	"os/exec"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var _ = Describe("console command", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface

		capturedCommands [][]string

		pullSecret  string
		clusterID   string
		proxyURL    string
		fakeToken   string
		testKubeCfg api.Config
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

		pullSecret = "testpullsecret"
		clusterID = "cluster123"
		proxyURL = "https://my.proxy:443"
		fakeToken = "faketokenpleaseignore"

		testKubeCfg = api.Config{
			Kind:        "Config",
			APIVersion:  "v1",
			Preferences: api.Preferences{},
			Clusters: map[string]*api.Cluster{
				"testcluster": {
					Server: "https://api-backplane.apps.something.com/backplane/cluster/cluster123",
				},
				"api-backplane.apps.something.com:443": { // Remark that the cluster name does not match the cluster ID in below URL
					Server: "https://api-backplane.apps.something.com/backplane/cluster/cluster123",
				},
			},
			AuthInfos: map[string]*api.AuthInfo{
				"testauth": {
					Token: "token123",
				},
			},
			Contexts: map[string]*api.Context{
				"default/testcluster/testauth": {
					Cluster:   "testcluster",
					AuthInfo:  "testauth",
					Namespace: "default",
				},
				"custom-context": {
					Cluster:   "api-backplane.apps.something.com:443",
					AuthInfo:  "testauth",
					Namespace: "test-namespace",
				},
			},
			CurrentContext: "default/testcluster/testauth",
			Extensions:     nil,
		}

		dirName, _ := os.MkdirTemp("", ".kube")

		pullSecretConfigDirectory = dirName
	})

	AfterEach(func() {
		os.Setenv("HTTPS_PROXY", "")
		mockCtrl.Finish()
		utils.RemoveTempKubeConfig()
	})

	setupConfig := func() {
		err := os.Setenv(info.BackplaneProxyEnvName, proxyURL)
		Expect(err).To(BeNil())

		err = utils.CreateTempKubeConfig(&testKubeCfg)
		Expect(err).To(BeNil())
	}

	Context("when console command executes", func() {
		It("should read the openbrowser variable from environment variables and it is true", func() {
			setupConfig()
			os.Setenv(EnvBrowserDefault, "true")
			o := consoleOptions{}
			err := o.determineOpenBrowser()
			os.Setenv(EnvBrowserDefault, "")
			Expect(err).To(BeNil())
			Expect(o.openBrowser).To(BeTrue())
		})

		It("should read the openbrowser variable from environment variables and it is false", func() {
			setupConfig()
			os.Setenv(EnvBrowserDefault, "false")
			o := consoleOptions{}
			err := o.determineOpenBrowser()
			os.Setenv(EnvBrowserDefault, "")
			Expect(err).To(BeNil())
			Expect(o.openBrowser).To(BeFalse())
		})

		It("should read the openbrowser variable from environment variables and we it is undefined", func() {
			setupConfig()
			os.Setenv(EnvBrowserDefault, "")
			o := consoleOptions{}
			err := o.determineOpenBrowser()
			Expect(err).To(MatchError(ContainSubstring("unable to parse boolean value from environment variable")))
		})

		It("should use the specified port for listen", func() {
			setupConfig()
			o := consoleOptions{port: "5555"}
			err := o.determineListenPort()
			Expect(err).To(BeNil())
			Expect(o.port).To(Equal("5555"))
		})

		It("should pick a random port for listen if not specified", func() {
			setupConfig()
			o := consoleOptions{}
			err := o.determineListenPort()
			Expect(err).To(BeNil())
			Expect(len(o.port)).ToNot(Equal(0))
		})

		It("should throw an error when port is not an integer", func() {
			setupConfig()
			o := consoleOptions{port: "verysusport"}
			err := o.determineListenPort()
			Expect(err).To(MatchError("port should be an integer"))
			Expect(len(o.port)).ToNot(Equal(0))
		})

		It("should fetch the console image from the cluster", func() {
			createClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return testclient.NewSimpleClientset(&appsv1.DeploymentList{Items: []appsv1.Deployment{{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ConsoleNS,
						Name:      ConsoleDeployment,
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:  "console",
									Image: "testrepo.com/test/console:latest",
								}},
							},
						},
					},
				}}}), nil
			}
			setupConfig()
			o := consoleOptions{}
			// for testing, we don't need a real rest.Config
			err := o.determineImage(nil)
			Expect(err).To(BeNil())
			Expect(o.image).To(Equal("testrepo.com/test/console:latest"))
		})
	})

	Context("For cluster version below 4.14", func() {
		clusterInfo, _ := cmv1.NewCluster().
			CloudProvider(cmv1.NewCloudProvider().ID("aws")).
			Product(cmv1.NewProduct().ID("dedicated")).
			AdditionalTrustBundle("REDACTED").
			Proxy(cmv1.NewProxy().HTTPProxy("http://my.proxy:80").HTTPSProxy("https://my.proxy:443")).
			OpenshiftVersion("4.13.0").Build()

		It("should not assgin a port for monitoring plugin", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := consoleOptions{}
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginPort()
			Expect(err).To(BeNil())
			Expect(len(o.monitorPluginPort)).To(Equal(0))
		})

		It("should not lookup the monitoring plugin image", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := consoleOptions{}
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginImage(nil)
			Expect(err).To(BeNil())
			Expect(len(o.monitorPluginImage)).To(Equal(0))
		})

		It("should not add monitoring plugin to console arguments", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := consoleOptions{}
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			plugins, err := o.getPlugins()
			Expect(err).To(BeNil())
			Expect(plugins).ToNot(ContainSubstring("monitoring-plugin"))
		})
	})

	Context("For cluster version above 4.14", func() {
		clusterInfo, _ := cmv1.NewCluster().
			CloudProvider(cmv1.NewCloudProvider().ID("aws")).
			Product(cmv1.NewProduct().ID("dedicated")).
			AdditionalTrustBundle("REDACTED").
			Proxy(cmv1.NewProxy().HTTPProxy("http://my.proxy:80").HTTPSProxy("https://my.proxy:443")).
			OpenshiftVersion("4.14.8").Build()

		It("should assgin a port for monitoring plugin", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := consoleOptions{}
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginPort()
			Expect(err).To(BeNil())
			Expect(len(o.monitorPluginPort)).ToNot(Equal(0))
		})

		It("should lookup the monitoring plugin image from cluster", func() {
			createClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return testclient.NewSimpleClientset(&appsv1.DeploymentList{Items: []appsv1.Deployment{{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: MonitoringNS,
						Name:      MonitoringPluginDeployment,
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:  "monitoring-plugin",
									Image: "testrepo.com/test/monitorplugin:latest",
								}},
							},
						},
					},
				}}}), nil
			}
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := consoleOptions{}
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginImage(nil)
			Expect(err).To(BeNil())
			Expect(o.monitorPluginImage).To(Equal("testrepo.com/test/monitorplugin:latest"))
		})

		It("should add monitoring plugin to console arguments", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := consoleOptions{}
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			plugins, err := o.getPlugins()
			Expect(err).To(BeNil())
			Expect(plugins).To(ContainSubstring("monitoring-plugin"))
		})
	})

	Context("when running podman on MacOS", func() {
		ce := podmanMac{}
		It("should put the file via ssh to the VM", func() {
			capturedCommands = nil
			err := ce.putFileToMount("testfile", []byte("test"))
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
			err := ce.pullImage("testimage")
			Expect(err).To(BeNil())
			Expect(len(capturedCommands)).To(Equal(1))
			command := capturedCommands[0]
			// executing docker --config xxxx pull
			Expect(len(command)).To(BeNumerically(">", 3))
			Expect(command[1]).To(ContainSubstring("config"))
		})
	})

	Context("An container is created to run the console, prior to doing that we need to check if container distro is supported", func() {

		ced := &dockerLinux{}
		cep := &podmanLinux{}

		// Consider refactoring the following function as it isn't being used
		It("Check for the existance of a invalid container engine", func() {
			vld, err := validateContainerEngine("Foo Bar")
			Expect(vld).To(BeFalse())
			Expect(err).To(MatchError(ContainSubstring("container engine can only be one of")))
		})

		// For some strange reason reflect of cei returns a pointer

		It("In the case we explicitly specify Podman, the code should return support for Podman", func() {
			oldpath := createPathPodman()
			o := consoleOptions{}
			o.containerEngineFlag = PODMAN
			cei, err := o.getContainerEngineImpl()
			Expect(err).To(BeNil())
			Expect(reflect.TypeOf(cei) == reflect.TypeOf(cep)).To(BeTrue())
			removePath(oldpath)
		})

		It("In the case we explicitly specify Docker, the code should return support for Docker", func() {
			oldpath := createPathDocker()
			o := consoleOptions{}
			o.containerEngineFlag = DOCKER
			cei1, err1 := o.getContainerEngineImpl()
			Expect(err1).To(BeNil())
			Expect(reflect.TypeOf(cei1) == reflect.TypeOf(ced)).To(BeTrue())
			removePath(oldpath)
		})

		It("Test if environment varible could be read by the code to identify what container engine to use", func() {
			oldpath := createPathPodman()
			o := consoleOptions{}
			os.Setenv("CONTAINER_ENGINE", PODMAN)
			o.containerEngineFlag = ""
			cei2, err2 := o.getContainerEngineImpl()
			Expect(err2).To(BeNil())
			Expect(reflect.TypeOf(cei2) == reflect.TypeOf(cep)).To(BeTrue())
			removePath(oldpath)
		})

		It("Test the situation where the environment varible is something else", func() {
			o := consoleOptions{}
			o.containerEngineFlag = "FOO"
			os.Setenv("BACKPLANE_DEFAULT_OPEN_BROWSER", "FALSE")
			_, err4 := o.getContainerEngineImpl()
			Expect(err4).To(MatchError(ContainSubstring("container engine can only be one of podman|docker")))
			os.Setenv("BACKPLANE_DEFAULT_OPEN_BROWSER", "")
		})
	})

	// Putting everything together, we could call the .run function to test the particular functionality
	Context("Once we have validated container runtimes, we need to start the containers in which the console resides", func() {

		It("Prior to running the console we need to create a cobra.command object and verify the flag entries are created", func() {
			consoleCmd := NewConsoleCmd()
			flags := consoleCmd.Flags()
			Expect(reflect.TypeOf(flags) == reflect.TypeOf(&pflag.FlagSet{})).To(BeTrue())
		})

		It("The run function checks configurations and runs the container", func() {
			// createPathPodman()
			oldpath := createPathDocker()
			// Create a new client set or deployment to be read
			createClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return testclient.NewSimpleClientset(&appsv1.DeploymentList{Items: []appsv1.Deployment{{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ConsoleNS,
						Name:      ConsoleDeployment,
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:  "console",
									Image: "testrepo.com/test/console:latest",
								}},
							},
						},
					},
				}}}), nil
			}

			// Define a Cluster info, 4.13 since we're not testing monitoring in this scenario
			clusterInfo, _ := cmv1.NewCluster().
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				Product(cmv1.NewProduct().ID("dedicated")).
				AdditionalTrustBundle("REDACTED").
				Proxy(cmv1.NewProxy().HTTPProxy("http://my.proxy:80").HTTPSProxy("https://my.proxy:443")).
				OpenshiftVersion("4.13.0").Build()

			// Set Browser opening to false
			os.Setenv("BACKPLANE_DEFAULT_OPEN_BROWSER", "FALSE")
			setupConfig()

			// Set some mock varibles,
			mockOcmInterface.EXPECT().GetPullSecret().Return(pullSecret, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&fakeToken, nil).AnyTimes()
			// Create a new ocmEnvironment for mock use
			ocmEnvironment, _ := cmv1.NewEnvironment().BackplaneURL("fakeBackPlaneUrl").Build()
			// Tell mockOCM interface to return ocnEnvironment
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnvironment, nil).AnyTimes()
			o := consoleOptions{terminationFunction: &execActionOnTermMockStruct{}}
			o.port = "1337"
			o.url = "http://127.0.0.2:1447"
			o.openBrowser = false
			o.containerEngineFlag = DOCKER
			ce, err := o.getContainerEngineImpl()
			Expect(err).To(BeNil())
			done := make(chan bool)
			errs := make(chan error)
			o.runContainers(ce, done, errs)
			Expect(errs).To(BeEmpty())
			os.Setenv("BACKPLANE_DEFAULT_OPEN_BROWSER", "")
			removePath(oldpath)
		})
	})
})
