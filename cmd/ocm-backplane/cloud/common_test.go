package cloud

import (
	"context"
	"errors"
	"fmt"
	"github.com/openshift/backplane-cli/pkg/awsutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

//nolint:gosec
var _ = Describe("getIsolatedCredentials", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils
		mockClient       *mocks.MockClientInterface

		testOcmToken        string
		testClusterID       string
		testAccessKeyID     string
		testSecretAccessKey string
		testSessionToken    string
		testQueryConfig     QueryConfig
		fakeHTTPResp        *http.Response
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClient = mocks.NewMockClientInterface(mockCtrl)

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		testOcmToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZW1haWwiOiJ0ZXN0QGZvby5jb20iLCJpYXQiOjE1MTYyMzkwMjJ9.5NG4wFhitEKZyzftSwU67kx4JVTEWcEoKhCl_AFp8T4"
		testClusterID = "test123"
		testAccessKeyID = "test-access-key-id"
		testSecretAccessKey = "test-secret-access-key"
		testSessionToken = "test-session-token"

		stsBuilder := &cmv1.STSBuilder{}
		stsBuilder.Enabled(true)

		awsBuilder := &cmv1.AWSBuilder{}
		awsBuilder.STS(stsBuilder)

		clusterBuilder := cmv1.ClusterBuilder{}
		clusterBuilder.AWS(awsBuilder)
		clusterBuilder.ID(testClusterID)

		cluster, _ := clusterBuilder.Build()
		testQueryConfig = QueryConfig{OcmConnection: &sdk.Connection{}, BackplaneConfiguration: config.BackplaneConfiguration{URL: "test", AssumeInitialArn: "test"}, Cluster: cluster}

		fakeHTTPResp = &http.Response{
			Body: MakeIoReader(
				`{"assumptionSequence":[{"name":"SRE-Role-Arn","arn":"arn:aws:iam::10000000:role/TEST_USER"},
				{"name":"Org-Role-Arn","arn":"arn:aws:iam::10000000:role/TEST_USER"},
				{"name":"Target-Role-Arn","arn":"arn:aws:iam::10000000:role/TEST_USER"}],
				"customerRoleSessionName":"b7bb29e58495b17412e15701cea805b7"}`,
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Execute getIsolatedCredentials", func() {
		It("should fail if empty cluster ID is provided", func() {
			clusterBuilder := cmv1.ClusterBuilder{}
			clusterBuilder.ID("")
			cluster, _ := clusterBuilder.Build()
			testQueryConfig.Cluster = cluster

			_, err := testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err).To(Equal(fmt.Errorf("must provide non-empty cluster ID")))
		})
		It("should fail if cannot create sts client with proxy", func() {
			StsClient = func(proxyURL *string) (*sts.Client, error) {
				return nil, errors.New(":(")
			}

			_, err := testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err.Error()).To(Equal("failed to create sts client: :("))
		})
		It("should fail if initial role cannot be assumed with JWT", func() {
			StsClient = func(proxyURL *string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{}, errors.New("failure")
			}

			_, err := testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err.Error()).To(Equal("failed to assume role using JWT: failure"))
		})
		It("should fail if email cannot be pulled off JWT", func() {
			testOcmToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

			StsClient = func(proxyURL *string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}

			_, err := testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err.Error()).To(Equal("unable to extract email from given token: no field email on given token"))
		})
		It("should fail credentials with inline policy", func() {

			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			ip2 := cmv1.NewTrustedIp().ID("200.20.20.20").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1, ip2).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

			StsClient = func(proxyURL *string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}
			mockClientUtil.EXPECT().GetBackplaneClient(
				testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.ProxyURL).Return(mockClient, nil)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(fakeHTTPResp, nil)

			_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to assume role sequence:"))
		})
		It("should fail if error creating backplane api client", func() {
			StsClient = func(proxyURL *string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}
			NewStaticCredentialsProvider = func(key, secret, session string) credentials.StaticCredentialsProvider {
				return credentials.StaticCredentialsProvider{}
			}
			mockClientUtil.EXPECT().GetBackplaneClient(testQueryConfig.BackplaneConfiguration.URL, testOcmToken, nil).Return(nil, errors.New("foo")).Times(1)

			_, err := testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err.Error()).To(Equal("failed to create backplane client with access token: foo"))
		})

	})
	Context("Check the client egress IP", func() {
		var (
			client *http.Client
			server *httptest.Server
		)
		It("Should return the correct client IP", func() {
			mockIP := "1.1.1.1"
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, err := writer.Write([]byte(mockIP))
				if err != nil {
					return
				}
			}))
			defer server.Close()

			client := &http.Client{}
			ip, err := checkEgressIP(client, server.URL)

			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal(net.ParseIP(mockIP)))
		})
		It("should return an error when the HTTP GET fails", func() {
			client = &http.Client{}
			// Invalid URL to force error
			ip, err := checkEgressIP(client, "http://invalid_url")
			Expect(err).To(HaveOccurred())
			Expect(ip).To(BeNil())
		})
		It("should return an error when response body is not a valid IP", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "not-an-ip")
			}))
			client = server.Client()

			ip, err := checkEgressIP(client, server.URL)
			Expect(err).To(MatchError(ContainSubstring("failed to parse IP")))
			Expect(ip).To(BeNil())
		})

	})

	Context("Compare IP with trusted list", func() {
		var (
			ip         net.IP
			trustedIPs awsutil.IPAddress
		)

		BeforeEach(func() {
			trustedIPs = awsutil.IPAddress{
				SourceIp: []string{
					"192.168.1.1/32",
					"10.0.0.0/8",
				},
			}
		})

		It("should return nil if IP included in trusted list", func() {
			ip = net.ParseIP("192.168.1.1")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())
		})
		It("should return nil if IP not included in trusted list", func() {
			ip = net.ParseIP("172.16.0.1")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())
		})
		It("should return an error if give the wrong format", func() {
			trustedIPs.SourceIp = []string{"invalid-cidr"}
			ip = net.ParseIP("192.168.1.1")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(HaveOccurred())
		})
	})
	Context("Execute getTrustedIpInlinePolicy", func() {

		It("should Return inline policy with TrustedIP list", func() {
			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			ip2 := cmv1.NewTrustedIp().ID("200.20.20.20").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1, ip2).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)
			IPList, _ := getTrustedIPList(testQueryConfig.OcmConnection)
			policy, _ := getTrustedIPInlinePolicy(IPList)
			//Only allow 209 IP
			Expect(policy).To(ContainSubstring("209.10.10.10"))
			//Not allow 200 IP
			Expect(policy).NotTo(ContainSubstring("200.20.20.20"))
			Expect(err).To(BeNil())
		})
	})
})

// newTestCluster assembles a *cmv1.Cluster while handling the error to help out with inline test-case generation
func newTestCluster(t *testing.T, cb *cmv1.ClusterBuilder) *cmv1.Cluster {
	cluster, err := cb.Build()
	if err != nil {
		t.Fatalf("failed to build cluster: %s", err)
	}

	return cluster
}

func TestIsIsolatedBackplaneAccess(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *cmv1.Cluster
		expected bool
	}{
		{
			name:     "AWS non-STS",
			cluster:  newTestCluster(t, cmv1.NewCluster().AWS(cmv1.NewAWS().STS(cmv1.NewSTS().Enabled(false)))),
			expected: false,
		},
		{
			name:     "GCP",
			cluster:  newTestCluster(t, cmv1.NewCluster().GCP(cmv1.NewGCP())),
			expected: false,
		},
	}

	//cmv1.NewStsSupportJumpRole().RoleArn(OldFlowSupportRole)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := isIsolatedBackplaneAccess(test.cluster, &sdk.Connection{})
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			if test.expected != actual {
				t.Errorf("expected: %v, got: %v", test.expected, actual)
			}
		})
	}
}

var _ = Describe("isIsolatedBackplaneAccess", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface

		testClusterID string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		testClusterID = "test123"
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Execute isIsolatedBackplaneAccess", func() {
		It("returns true immediately if the cluster is hypershift enabled", func() {
			hyperShiftBuilder := &cmv1.HypershiftBuilder{}
			hyperShiftBuilder.Enabled(true)

			clusterBuilder := cmv1.ClusterBuilder{}
			clusterBuilder.Hypershift(hyperShiftBuilder)

			cluster, _ := clusterBuilder.Build()
			result, err := isIsolatedBackplaneAccess(cluster, nil)

			Expect(result).To(Equal(true))
			Expect(err).To(BeNil())
		})
		It("returns an error if fails to get STS Support Jump Role from OCM for STS enabled cluster", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(&sdk.Connection{}, testClusterID).Return("", errors.New("oops"))

			stsBuilder := &cmv1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &cmv1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := cmv1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			_, err := isIsolatedBackplaneAccess(cluster, &sdk.Connection{})

			Expect(err).NotTo(BeNil())
		})
		It("returns an error if fails to parse STS Support Jump Role from OCM for STS enabled cluster", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(&sdk.Connection{}, testClusterID).Return("not-an-arn", nil)

			stsBuilder := &cmv1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &cmv1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := cmv1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			_, err := isIsolatedBackplaneAccess(cluster, &sdk.Connection{})

			Expect(err).NotTo(BeNil())
		})
		It("returns false with no error for STS enabled cluster with ARN that matches old support flow ARN", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(&sdk.Connection{}, testClusterID).Return("arn:aws:iam::123456789:role/RH-Technical-Support-Access", nil)

			stsBuilder := &cmv1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &cmv1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := cmv1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			result, err := isIsolatedBackplaneAccess(cluster, &sdk.Connection{})

			Expect(result).To(Equal(false))
			Expect(err).To(BeNil())
		})
		It("returns true with no error for STS enabled cluster with ARN that doesn't match old support flow ARN", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(&sdk.Connection{}, testClusterID).Return("arn:aws:iam::123456789:role/RH-Technical-Support-12345", nil)

			stsBuilder := &cmv1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &cmv1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := cmv1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			result, err := isIsolatedBackplaneAccess(cluster, &sdk.Connection{})

			Expect(result).To(Equal(true))
			Expect(err).To(BeNil())
		})
	})
})
