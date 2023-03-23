package console

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks "github.com/openshift/backplane-cli/pkg/utils/mocks"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	"os/exec"
	"path/filepath"
)

var _ = Describe("console command", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *mocks.MockOCMInterface
		

		capturedCommands [][]string

		testToken   string
		pullSecret  string
		clusterID   string
        clusterInfo *cmv1.Cluster
		

	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = mocks.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		createClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
			return testclient.NewSimpleClientset(&appsv1.DeploymentList{Items: []appsv1.Deployment{{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "openshift-console",
					Name:      "console",
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
		capturedCommands = nil
		createCommand = func(prog string, args ...string) *exec.Cmd {
			command := []string{prog}
			command = append(command, args...)
			capturedCommands = append(capturedCommands, command)

			return exec.Command("true")
		}
		

		consoleArgs.port = "12345"

		ConsoleCmd.SetArgs([]string{"console"})
	
		testToken = "hello123"
		pullSecret = "testpullsecret"
		clusterID = "configcluster"
		
		
		
	})

	AfterEach(func() {

		mockCtrl.Finish()
	})

	setupConfig := func() {
        err := utils.CreateTempKubeConfig(nil)
        Expect(err).To(BeNil())
    }

	checkCapturedCommands := func() {
		Expect(len(capturedCommands)).To(Equal(2))

		home, err := homedir.Dir()
		Expect(err).To(BeNil())
		authFile := filepath.Join(home, ".kube/ocm-pull-secret/config.json")

		Expect(capturedCommands[0]).To(Equal([]string{
			"podman", "pull", "--quiet", "--authfile", authFile, "testrepo.com/test/console:latest",
		}))
		Expect(capturedCommands[1]).To(Equal([]string{
			"podman", "run", "--rm", "--name", "console-cluster123", "-p", "127.0.0.1:12345:12345", "--authfile", authFile, "testrepo.com/test/console:latest",
			"/opt/bridge/bin/bridge", "--public-dir=/opt/bridge/static", "-base-address", "http://127.0.0.1:12345", "-branding", "dedicated",
			"-documentation-base-url", "https://docs.openshift.com/dedicated/4/", "-user-settings-location", "localstorage", "-user-auth", "disabled", "-k8s-mode",
			"off-cluster", "-k8s-auth", "bearer-token", "-k8s-mode-off-cluster-endpoint", "https://api-backplane.apps.something.com/backplane/cluster/cluster123",
			"-k8s-mode-off-cluster-alertmanager", "https://api-backplane.apps.something.com/backplane/alertmanager/cluster123", "-k8s-mode-off-cluster-thanos",
			"https://api-backplane.apps.something.com/backplane/thanos/cluster123", "-k8s-auth-bearer-token", testToken, "-listen", "http://0.0.0.0:12345",
		}))
		clusterInfo, _ = cmv1.NewCluster().
			CloudProvider(cmv1.NewCloudProvider().ID("aws")).
			Product(cmv1.NewProduct().ID("dedicated")).
			AdditionalTrustBundle("REDACTED").
	        Proxy(cmv1.NewProxy().HTTPProxy("http://my.proxy:80").HTTPSProxy("https://my.proxy:443")).Build()
	}

	Context("when backplane login has just been done", func() {
		It("should start console server", func() {
			
			setupConfig()
 
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetPullSecret().Return(&pullSecret, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
					
			err := ConsoleCmd.Execute()

			Expect(err).To(BeNil())

			checkCapturedCommands()
		})
	})
	

	// This test verifies that the console container is still started the same way after issuing a
	// 'oc project <namespace id>' command.
	//
	// In particular this test checks that the name of container started by the 'ocm backplane console'
	// command is based on the cluster id and not on the supposed cluster name extracted from kube config.
	//
	// Indeed 'oc' client is actually connected to the hive cluster which proxy commands to the targeted
	// OSD cluster.
	// Issuing a 'oc project <namespace id>' will create a new context with a new cluster in kube config...
	// but the name of the newly created cluster config will be based on the hive cluster URL:
	// - Which does not contain any bit of information concerning the OSD cluster name.
	// - Which contains ':' char which is an invalid char in a container name.

	Context("when namespace is no more the default one", func() {
		It("should start console server", func() {

			setupConfig()

			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetPullSecret().Return(&pullSecret, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			
			

			err := ConsoleCmd.Execute()

			Expect(err).To(BeNil())

			checkCapturedCommands()
		})
	})

	Context("when kube config is invalid", func() {
		It("should start not console server", func() {
			
			setupConfig()

			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
            mockOcmInterface.EXPECT().GetPullSecret().Return(&pullSecret, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			
		
			err := ConsoleCmd.Execute()

			Expect(err).ToNot(BeNil())
			Expect(len(capturedCommands)).To(Equal(0))
		})
	})
})
