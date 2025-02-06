package cloud

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

var _ = Describe("SSM command", func() {
	var (
		mockCtrl           *gomock.Controller
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *ocmMock.MockOCMInterface
		mockClientUtil     *backplaneapiMock.MockClientUtils
	)
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface
		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil
		backplaneapi.DefaultClientUtils = mockClientUtil
		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)
	})
	AfterEach(func() {
		os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})
	Context("Test FetchCloudCredentials function", func() {
		It("returns the clusterinfo from GetBackplaneClusterFromConfig", func() {
			GetBackplaneClusterFromConfig = func() (utils.BackplaneCluster, error) {
				return utils.BackplaneCluster{
					ClusterID: "mockcluster",
				}, nil
			}
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, errors.New("error")).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster("mockcluster").Times(1)
			_ = runCredentials(&cobra.Command{}, []string{})
		})
		It("returns an error from GetBackplaneClusterFromConfig", func() {
			GetBackplaneClusterFromConfig = func() (utils.BackplaneCluster, error) {
				return utils.BackplaneCluster{}, errors.New("bp error")
			}
			Expect(runCredentials(&cobra.Command{}, []string{})).To(Equal(errors.New("bp error")))
		})
		It("returns an error if GetTargetCluster returns an error", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return("", "", errors.New("getTargetCluster error")).AnyTimes()
			Expect(runCredentials(&cobra.Command{}, []string{"cluster-key"})).Error()
		})
		It("returns an error if GetClusterInfoByID returns an error", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return("foo", "bar", nil).AnyTimes()
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, errors.New("error")).AnyTimes()
			Expect(runCredentials(&cobra.Command{}, []string{"cluster-key"})).To(Equal(
				fmt.Errorf("failed to get cluster info for %s: %w", "foo", errors.New("error")),
			))
		})
	})
	Context("Test GetInstanceID function", func() {
		var testNode = &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
			Spec: v1.NodeSpec{
				ProviderID: "aws:///us-west-2/i-1234567890abcdef0",
			},
		}
		It("should successfully extract instance ID", func() {
			CreateClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return fake.NewSimpleClientset(testNode), nil
			}
			instanceID, err := GetInstanceID("test-node", &rest.Config{})
			Expect(err).ToNot(HaveOccurred())
			Expect(instanceID).To(Equal("i-1234567890abcdef0"))
		})
		It("should handle missing provider ID", func() {
			CreateClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return fake.NewSimpleClientset(&v1.Node{}), nil
			}
			_, err := GetInstanceID("test-node", &rest.Config{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("providerID is not set"))
		})
		It("returns an error if node name is incorrect", func() {
			ssmArgs.node = "invalid-node"
			err := StartSSMsession(&cobra.Command{}, []string{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get instance ID for node"))
		})
	})
	// Context("Check StartSSMsession function", func() {
	// 	fakeAWSCredentialsResponse := &bpCredentials.AWSCredentialsResponse{
	// 		AccessKeyID:     "foo",
	// 		SecretAccessKey: "bar",
	// 		SessionToken:    "baz",
	// 		Region:          "quux",
	// 	}
	// 	It("should execute correct command with session details", func() {
	// 		// Mock session details
	// 		mockSession := &ssm.StartSessionOutput{
	// 			SessionId:  aws.String("test-session-id"),
	// 			StreamUrl:  aws.String("test-stream-url"),
	// 			TokenValue: aws.String("test-token-value"),
	// 		}
	// 		// Mock exec command
	// 		executed := false
	// 		ExecCommand = func(name string, arg ...string) *exec.Cmd {
	// 			executed = true
	// 			// Verify command components
	// 			Expect(name).To(Equal("session-manager-plugin"))
	// 			Expect(arg).To(HaveLen(3))
	// 			// Verify session JSON
	// 			var sessionData map[string]string
	// 			err := json.Unmarshal([]byte(arg[0]), &sessionData)
	// 			Expect(err).ToNot(HaveOccurred())
	// 			Expect(sessionData).To(Equal(map[string]string{
	// 				"SessionId":  *mockSession.SessionId,
	// 				"StreamUrl":  *mockSession.StreamUrl,
	// 				"TokenValue": *mockSession.TokenValue,
	// 			}))
	// 			return exec.Command("echo", "mock command")
	// 		}
	// 		// Execute plugin command directly
	// 		sessionJSON, _ := json.Marshal(map[string]string{
	// 			"SessionId":  *mockSession.SessionId,
	// 			"StreamUrl":  *mockSession.StreamUrl,
	// 			"TokenValue": *mockSession.TokenValue,
	// 		})
	// 		cmd := exec.Command("session-manager-plugin", string(sessionJSON), fakeAWSCredentialsResponse.Region, "StartSession")
	// 		err := cmd.Run()
	// 		Expect(err).ToNot(HaveOccurred())
	// 		Expect(executed).To(BeTrue())
	// 	})
	// 	It("returns an error if region name is incorrect", func() {
	// 		mockregion := fakeAWSCredentialsResponse.Region
	// 		Expect(mockregion, "us-east-1").To(Equal(
	// 			fmt.Errorf("Expected region is %s, got wrong region %w", "us-east-1", errors.New("error")),
	// 		))
	// 	})
	// 	It("returns an error if node name is passed empty", func() {
	// 		ssmArgs.node = ""
	// 		err := StartSSMsession(&cobra.Command{}, []string{})
	// 		Expect(err).To(HaveOccurred())
	// 		Expect(err.Error()).To(Equal("--node flag is required"))
	// 	})
	// })
})
