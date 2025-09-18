package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/ssm/mocks"
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
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface
		mockSSMClient    *mocks.MockSSMClient
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface
		mockSSMClient = mocks.NewMockSSMClient(mockCtrl)

		// Override CreateClientSet to return a fake Kubernetes client
		CreateClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
			return fake.NewSimpleClientset(), nil
		}

		// Override NewFromConfig to return the mock SSM client
		NewFromConfig = func(cfg aws.Config) SSMClient {
			return mockSSMClient
		}
	})

	AfterEach(func() {
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

		BeforeEach(func() {
			// Override CreateClientSet to return a fake Kubernetes client
			CreateClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return fake.NewSimpleClientset(testNode), nil
			}
		})

		It("should successfully extract instance ID", func() {
			instanceID, err := GetInstanceID("test-node", &rest.Config{})
			Expect(err).ToNot(HaveOccurred())
			Expect(instanceID).To(Equal("i-1234567890abcdef0"))
		})

		It("should handle missing provider ID", func() {
			CreateClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return fake.NewSimpleClientset(&v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				}), nil
			}
			_, err := GetInstanceID("test-node", &rest.Config{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("providerID is not set"))
		})

		It("returns an error if node name is incorrect", func() {
			CreateClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return fake.NewSimpleClientset(), nil
			}
			_, err := GetInstanceID("invalid-node", &rest.Config{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get node invalid-node"))
		})
	})

	var _ = Describe("SSM command", func() {
		var (
			originalExecCommand func(string, ...string) *exec.Cmd
			cmdArgs             []string
			testNode            *v1.Node
			fakeProxyURL        string
		)

		BeforeEach(func() {
			cmdArgs = []string{}
			testNode = &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
				},
			}

			// Mock Kubernetes client with test node
			CreateClientSet = func(c *rest.Config) (kubernetes.Interface, error) {
				return fake.NewSimpleClientset(testNode), nil
			}

			// Mock exec command
			originalExecCommand = ExecCommand
			ExecCommand = func(name string, arg ...string) *exec.Cmd {
				cmdArgs = append([]string{name}, arg...)
				return exec.Command("echo", "mock command")
			}

			// Mock credentials
			fakeProxyURL = "https://proxy.example.com:8080"
			GetBackplaneConfiguration = func() (config.BackplaneConfiguration, error) {
				return config.BackplaneConfiguration{
					ProxyURL: &fakeProxyURL,
				}, nil
			}
			FetchCloudCredentials = func() (*bpCredentials.AWSCredentialsResponse, error) {
				return &bpCredentials.AWSCredentialsResponse{
					AccessKeyID:     "TEST_ACCESS_KEY",
					SecretAccessKey: "TEST_SECRET_KEY",
					SessionToken:    "TEST_SESSION_TOKEN",
					Region:          "us-west-2",
				}, nil
			}

			ssmArgs.node = "test-node"
		})

		AfterEach(func() {
			ExecCommand = originalExecCommand
			ssmArgs.node = ""
			os.Unsetenv("AWS_ACCESS_KEY_ID")
			os.Unsetenv("AWS_SECRET_ACCESS_KEY")
			os.Unsetenv("AWS_SESSION_TOKEN")
		})

		Context("startSSMsession", func() {
			It("should execute session-manager-plugin with correct arguments for a regular (interacting) session", func() {
				// Mock SSM client call
				mockSSMClient.EXPECT().StartSession(
					context.TODO(),
					&ssm.StartSessionInput{
						Target: aws.String("i-1234567890abcdef0"),
					},
				).Return(&ssm.StartSessionOutput{
					SessionId:  aws.String("test-session-id"),
					StreamUrl:  aws.String("test-stream-url"),
					TokenValue: aws.String("test-token-value"),
				}, nil)

				err := startSSMsession(&cobra.Command{}, nil)
				Expect(err).ToNot(HaveOccurred())

				// Verify ENV variables
				Expect(os.Getenv("AWS_ACCESS_KEY_ID")).To(Equal("TEST_ACCESS_KEY"))
				Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).To(Equal("TEST_SECRET_KEY"))
				Expect(os.Getenv("AWS_SESSION_TOKEN")).To(Equal("TEST_SESSION_TOKEN"))

				// Verify command structure
				Expect(cmdArgs).To(HaveLen(4))
				Expect(cmdArgs[0]).To(Equal("session-manager-plugin"))

				// Validate session details
				var sessionDetails map[string]string
				Expect(json.Unmarshal([]byte(cmdArgs[1]), &sessionDetails)).To(Succeed())
				Expect(sessionDetails["SessionId"]).To(Equal("test-session-id"))
				Expect(sessionDetails).To(HaveKey("StreamUrl"))
				Expect(sessionDetails).To(HaveKey("TokenValue"))
			})

			It("Should execute a non-interactive session when provided a command to execute", func() {
				command := []string{"free", "-m"}

				// Mock SSM client call
				mockSSMClient.EXPECT().StartSession(
					context.TODO(),
					&ssm.StartSessionInput{
						Target:       aws.String("i-1234567890abcdef0"),
						DocumentName: aws.String("AWS-StartInteractiveCommand"),
						Parameters:   map[string][]string{"command": {strings.Join(command, " ")}},
					},
				).Return(&ssm.StartSessionOutput{
					SessionId:  aws.String("test-session-id"),
					StreamUrl:  aws.String("test-stream-url"),
					TokenValue: aws.String("test-token-value"),
				}, nil)

				err := startSSMsession(&cobra.Command{}, command)
				Expect(err).ToNot(HaveOccurred())

				// Verify command struture
				Expect(cmdArgs[0]).To(Equal("session-manager-plugin"))
				var sessionDetails map[string]string
				Expect(json.Unmarshal([]byte(cmdArgs[1]), &sessionDetails)).To(Succeed())
				Expect(sessionDetails["SessionId"]).To(Equal("test-session-id"))

			})

			It("should handle credential fetch errors", func() {
				FetchCloudCredentials = func() (*bpCredentials.AWSCredentialsResponse, error) {
					return nil, fmt.Errorf("backplane config error")
				}

				err := startSSMsession(&cobra.Command{}, nil)
				Expect(err).To(MatchError("failed to fetch cloud credentials: backplane config error"))
			})

			It("fails on invalid proxy URL", func() {
				originalGetBackplaneConfig := GetBackplaneConfiguration
				GetBackplaneConfiguration = func() (config.BackplaneConfiguration, error) {
					invalidURL := "invalid-proxy-format"
					return config.BackplaneConfiguration{
						ProxyURL: &invalidURL,
					}, nil
				}
				defer func() { GetBackplaneConfiguration = originalGetBackplaneConfig }()

				err := startSSMsession(&cobra.Command{}, nil)
				Expect(err).To(MatchError(ContainSubstring("invalid proxy URL")))
			})

			It("fails when proxy URL is nil", func() {
				originalGetBackplaneConfig := GetBackplaneConfiguration
				GetBackplaneConfiguration = func() (config.BackplaneConfiguration, error) {
					return config.BackplaneConfiguration{
						ProxyURL: nil,
					}, nil
				}
				defer func() { GetBackplaneConfiguration = originalGetBackplaneConfig }()

				err := startSSMsession(&cobra.Command{}, nil)
				Expect(err).To(MatchError(ContainSubstring("proxy URL is not set")))
			})

			It("should handle missing node name", func() {
				ssmArgs.node = ""
				err := startSSMsession(&cobra.Command{}, nil)
				Expect(err).To(MatchError("--node flag is required"))
			})
		})
	})
})
