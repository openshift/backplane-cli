package console

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/container"
	ceMock "github.com/openshift/backplane-cli/pkg/container/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func TestIt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Console Test Suite")
}

var _ = Describe("console command", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface
		mockEngine       *ceMock.MockContainerEngine

		pullSecret  string
		clusterID   string
		proxyURL    string
		fakeToken   string
		testKubeCfg api.Config
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		mockEngine = ceMock.NewMockContainerEngine(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		os.Setenv("CONTAINER_ENGINE", PODMAN)

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
	Context("Should perform existing pods cleanup before starting the console", func() {
		It("Should not return an error if no pods are found", func() {
			setupConfig()
			createPathPodman()
			o := newConsoleOptions()
			ce := mockEngine
			mockEngine.EXPECT().ContainerIsExist(gomock.Any()).Return(false, nil).AnyTimes()
			err := o.beforeStartCleanUp(ce)
			Expect(err).To(BeNil())
		})

		It("Should stop containers if console and monitoring plugin containers are present", func() {
			setupConfig()
			createPathPodman()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(nil, nil).AnyTimes()
			o := newConsoleOptions()
			ce := mockEngine

			mockEngine.EXPECT().ContainerIsExist(gomock.Any()).Return(true, nil).AnyTimes()
			mockEngine.EXPECT().StopContainer(gomock.Any()).Return(nil).AnyTimes()

			err := o.beforeStartCleanUp(ce)
			Expect(err).To(BeNil())
		})
	})

	Context("when console command executes", func() {
		It("should read the openbrowser variable from environment variables and it is true", func() {
			setupConfig()
			os.Setenv(EnvBrowserDefault, "true")
			o := newConsoleOptions()
			err := o.determineOpenBrowser()
			os.Setenv(EnvBrowserDefault, "")
			Expect(err).To(BeNil())
			Expect(o.openBrowser).To(BeTrue())
		})

		It("should read the openbrowser variable from environment variables and it is false", func() {
			setupConfig()
			os.Setenv(EnvBrowserDefault, "false")
			o := newConsoleOptions()
			err := o.determineOpenBrowser()
			os.Setenv(EnvBrowserDefault, "")
			Expect(err).To(BeNil())
			Expect(o.openBrowser).To(BeFalse())
		})

		It("should read the openbrowser variable from environment variables and we it is undefined", func() {
			setupConfig()
			os.Setenv(EnvBrowserDefault, "")
			o := newConsoleOptions()
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
			o := newConsoleOptions()
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
			o := newConsoleOptions()
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
			o := newConsoleOptions()
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginPort()
			Expect(err).To(BeNil())
			Expect(len(o.monitorPluginPort)).To(Equal(0))
		})

		It("should not lookup the monitoring plugin image", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := newConsoleOptions()
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginImage(nil)
			Expect(err).To(BeNil())
			Expect(len(o.monitorPluginImage)).To(Equal(0))
		})

		It("should not add monitoring plugin to console arguments", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := newConsoleOptions()
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
			o := newConsoleOptions()
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
			o := newConsoleOptions()
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginImage(nil)
			Expect(err).To(BeNil())
			Expect(o.monitorPluginImage).To(Equal("testrepo.com/test/monitorplugin:latest"))
		})

		It("should add monitoring plugin to console arguments", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := newConsoleOptions()
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			plugins, err := o.getPlugins()
			Expect(err).To(BeNil())
			Expect(plugins).To(ContainSubstring("monitoring-plugin"))
		})
	})

	Context("For cluster version greater or equal 4.17", func() {
		clusterInfo, _ := cmv1.NewCluster().
			CloudProvider(cmv1.NewCloudProvider().ID("aws")).
			Product(cmv1.NewProduct().ID("dedicated")).
			AdditionalTrustBundle("REDACTED").
			Proxy(cmv1.NewProxy().HTTPProxy("http://my.proxy:80").HTTPSProxy("https://my.proxy:443")).
			OpenshiftVersion("4.17.1").Build()

		It("should assgin a default port for monitoring plugin", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := newConsoleOptions()
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginPort()
			Expect(err).To(BeNil())
			Expect(o.monitorPluginPort).To(Equal(DefaultMonitoringPluginPort))
		})

		It("should add monitoring plugin to console arguments", func() {
			setupConfig()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := newConsoleOptions()
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			plugins, err := o.getPlugins()
			Expect(err).To(BeNil())
			Expect(plugins).To(ContainSubstring("monitoring-plugin"))
		})

		It("should run the monitoring plugin with an environment variable that specifies the default port", func() {
			setupConfig()
			ce := mockEngine
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			o := newConsoleOptions()
			err := o.determineNeedMonitorPlugin()
			Expect(err).To(BeNil())
			err = o.determineMonitorPluginPort()
			Expect(err).To(BeNil())

			ce.EXPECT().RunMonitorPlugin(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
				func(containerName, consoleContainerName, nginxConf string, pluginArgs []string, envVars []container.EnvVar) {
					Expect(envVars).To(ContainElement(container.EnvVar{
						Key:   "PORT",
						Value: DefaultMonitoringPluginPort,
					}))
				}).Return(nil).Times(1)
			// to make it compatible, here it should still mount the nginx config because some 4.17 version may still need nginx.
			ce.EXPECT().PutFileToMount(gomock.Any(), gomock.Any()).AnyTimes()
			err = o.runMonitorPlugin(ce)
			Expect(err).To(BeNil())
		})
	})

	Context("An container is created to run the console, prior to doing that we need to check if container distro is supported", func() {
		It("In the case we explicitly specify Podman, the code should return support for Podman", func() {

			engineFactory = func(osName, engineName string) (container.ContainerEngine, error) {
				Expect(engineName).To(Equal(PODMAN))
				return mockEngine, nil
			}

			oldpath := createPathPodman()

			o := newConsoleOptions()
			o.containerEngineFlag = PODMAN
			_, err := o.getContainerEngineImpl()
			Expect(err).To(BeNil())

			setPath(oldpath)
		})

		It("In the case we explicitly specify Docker, the code should return support for Docker", func() {

			engineFactory = func(osName, engineName string) (container.ContainerEngine, error) {
				Expect(engineName).To(Equal(DOCKER))
				return mockEngine, nil
			}

			oldpath := createPathDocker()
			o := newConsoleOptions()
			o.containerEngineFlag = DOCKER
			_, err1 := o.getContainerEngineImpl()
			Expect(err1).To(BeNil())

			setPath(oldpath)
		})

		It("Test the situation where the environment variable is not a supported value", func() {
			o := newConsoleOptions()
			o.containerEngineFlag = "FOO"
			_, err4 := o.getContainerEngineImpl()
			Expect(err4).To(MatchError(ContainSubstring("container engine can only be one of podman|docker")))
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
			o := newConsoleOptions()
			o.terminationFunction = &execActionOnTermMockStruct{}
			o.port = "1337"
			o.url = "http://127.0.0.2:1447"
			o.openBrowser = false
			o.containerEngineFlag = DOCKER
			engineFactory = func(osName, engineName string) (container.ContainerEngine, error) {
				return mockEngine, nil
			}
			mockEngine.EXPECT().RunConsoleContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockEngine.EXPECT().RunMonitorPlugin(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			ce, err := o.getContainerEngineImpl()
			Expect(err).To(BeNil())

			errs := make(chan error)
			o.runContainers(ce, errs)

			Expect(errs).To(BeEmpty())
			os.Setenv("BACKPLANE_DEFAULT_OPEN_BROWSER", "")
			setPath(oldpath)
		})
	})
	Context("tests the branding config", func() {
		Context("tests the version parser", func() {
			It("should successfully parse a valid version", func() {
				version, err := parseVersion("4.19.0")
				Expect(err).To(BeNil())
				Expect(version.X).To(Equal(4))
				Expect(version.Y).To(Equal(19))
				Expect(version.Z).To(Equal(0))
			})
			It("should parse a valid patch version", func() {
				version, err := parseVersion("4.19.0-rc-0.12")
				Expect(err).To(BeNil())
				Expect(version.X).To(Equal(4))
				Expect(version.Y).To(Equal(19))
				Expect(version.Z).To(Equal(0))
				Expect(version.Patch).To(Equal("rc-0.12"))
			})
			It("should strip any 'v' prefix string", func() {
				version, err := parseVersion("v4.19.0")
				Expect(err).To(BeNil())
				Expect(version.X).To(Equal(4))
				Expect(version.Y).To(Equal(19))
				Expect(version.Z).To(Equal(0))
			})
			Context("version string error handling", func() {
				It("should error on an invalid version string", func() {
					_, err := parseVersion("someinvalidstirng")
					Expect(err).NotTo(BeNil())

					_, err = parseVersion("4")
					Expect(err).NotTo(BeNil())

					_, err = parseVersion("4.19")
					Expect(err).NotTo(BeNil())
				})
				It("should error on a x string", func() {
					_, err := parseVersion("4a.19.0")
					Expect(err).NotTo(BeNil())
				})
				It("should error on a y string", func() {
					_, err := parseVersion("4.19m.0")
					Expect(err).NotTo(BeNil())
				})
				It("should error on a z string", func() {
					_, err := parseVersion("4.19.0z")
					Expect(err).NotTo(BeNil())
				})
			})
		})
		Context("tests the branding switching logic", func() {
			It("should get dedicated brand", func() {
				c, _ := cmv1.NewCluster().
					Product(cmv1.NewProduct().ID("dedicated")).
					OpenshiftVersion("4.13.0").Build()
				bc, err := getBrandingConfig(c)
				Expect(err).To(BeNil())
				Expect(bc.Product).To(Equal("dedicated"))
				Expect(bc.DocsURL).To(ContainSubstring("openshift_dedicated"))
			})
			It("should get dedicated brand for >4.14 ROSA clusters", func() {
				c, _ := cmv1.NewCluster().
					Product(cmv1.NewProduct().ID("rosa")).
					OpenshiftVersion("4.13.0").Build()
				bc, err := getBrandingConfig(c)
				Expect(err.Error()).To(ContainSubstring("version is less than"))
				Expect(bc.Product).To(Equal("dedicated"))
				Expect(bc.DocsURL).To(ContainSubstring("openshift_dedicated"))
			})
			It("should get rosa classic", func() {
				c, _ := cmv1.NewCluster().
					Product(cmv1.NewProduct().ID("rosa")).
					Hypershift(cmv1.NewHypershift().Enabled(false)).
					OpenshiftVersion("4.18.0").Build()
				bc, err := getBrandingConfig(c)
				Expect(err).To(BeNil())
				Expect(bc.Product).To(Equal("rosa"))
				Expect(bc.DocsURL).To(ContainSubstring("red_hat_openshift_service_on_aws"))
				Expect(bc.DocsURL).To(ContainSubstring("classic_architecture"))
			})
			It("should get rosa hcp", func() {
				c, _ := cmv1.NewCluster().
					Product(cmv1.NewProduct().ID("rosa")).
					Hypershift(cmv1.NewHypershift().Enabled(true)).
					OpenshiftVersion("4.18.0").Build()
				bc, err := getBrandingConfig(c)
				Expect(err).To(BeNil())
				Expect(bc.Product).To(Equal("rosa"))
				Expect(bc.DocsURL).To(ContainSubstring("red_hat_openshift_service_on_aws"))
				Expect(bc.DocsURL).NotTo(ContainSubstring("classic_architecture"))
			})
			It("should default to hcp docs when it cannot determine hypershift", func() {
				c, _ := cmv1.NewCluster().
					Product(cmv1.NewProduct().ID("rosa")).
					OpenshiftVersion("4.18.0").Build()
				bc, err := getBrandingConfig(c)
				Expect(err).NotTo(BeNil())
				Expect(bc.DocsURL).To(ContainSubstring("red_hat_openshift_service_on_aws"))
				Expect(bc.DocsURL).NotTo(ContainSubstring("classic_architecture"))
			})
			It("should default to dedicated when it cannot get product", func() {
				c, _ := cmv1.NewCluster().Build()
				bc, err := getBrandingConfig(c)
				Expect(err).NotTo(BeNil())
				Expect(bc.Product).To(Equal("dedicated"))
			})
			It("should default to dedicated when it cannot get version", func() {
				c, _ := cmv1.NewCluster().
					Product(cmv1.NewProduct().ID("rosa")).
					Build()
				bc, err := getBrandingConfig(c)
				Expect(err).NotTo(BeNil())
				Expect(bc.Product).To(Equal("dedicated"))
			})
			It("should default to dedicated when it cannot parse version", func() {
				c, _ := cmv1.NewCluster().
					Product(cmv1.NewProduct().ID("rosa")).
					OpenshiftVersion("someinvalidversion").Build()
				bc, err := getBrandingConfig(c)
				Expect(err).NotTo(BeNil())
				Expect(bc.Product).To(Equal("dedicated"))
			})
		})
	})
})

type execActionOnTermMockStruct struct{}

func (e *execActionOnTermMockStruct) execActionOnTerminationFunction(action postTerminateFunc) error {
	return action()
}

func createPathPodman() string {
	return createPath("podman")
}

func createPathDocker() string {
	return createPath("docker")
}

// TODO: this creates a local tmp bin dir and puts files in it so tests find a given binary (docker/podman etc).
// Refactor those test sites to utilize a fake filesystem or other mechanism that doesn't rely on local state.
func createPath(binary string) string {
	oldpath := os.Getenv("PATH")
	setPath(oldpath + ":/tmp/tmp_bin")
	err := os.MkdirAll("/tmp/tmp_bin", 0777)
	if err != nil {
		fmt.Printf("Failed to create the directory: %v\n", err)
	}
	dFile, err := os.CreateTemp("/tmp/tmp_bin", "")
	if err != nil {
		fmt.Printf("Failed to create the file: %v\n", err)
	}
	if err := os.Rename(dFile.Name(), "/tmp/tmp_bin/"+binary); err != nil {
		fmt.Printf("Failed to rename the file: %v\n", err)
	}
	if err := os.Chmod("/tmp/tmp_bin/"+binary, 0777); err != nil {
		fmt.Printf("Failed to chmod the file: %v\n", err)
	}
	return oldpath
}

func setPath(oldPath string) {
	_ = os.Setenv("PATH", oldPath)
}
