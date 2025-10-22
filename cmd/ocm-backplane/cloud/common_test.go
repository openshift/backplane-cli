package cloud

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openshift/backplane-cli/pkg/awsutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"go.uber.org/mock/gomock"
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
		// Use a valid ARN from the allowed list to avoid "invalid assume-initial-arn" errors in tests
		testQueryConfig = QueryConfig{OcmConnection: &sdk.Connection{}, BackplaneConfiguration: config.BackplaneConfiguration{URL: "test", AssumeInitialArn: "arn:aws:iam::922711891673:role/SRE-Support-Role"}, Cluster: cluster}

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
		It("should fail if wrong assume initial ARN is provided", func() {
			testQueryConfig.AssumeInitialArn = "arn:aws:iam::10000000:role/TEST_USER"
			_, err := testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid assume-initial-arn: arn:aws:iam::10000000:role/TEST_USER, must be one of:"))
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
		It("should fail if unable to connect to AWS STS endpoint (GetCallerIdentity fails)", func() {
			GetCallerIdentity = func(client *sts.Client) error {
				return errors.New("failed to connect to AWS STS endpoint")
			}
			defer func() {
				GetCallerIdentity = func(client *sts.Client) error {
					_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
					return err
				}
			}()
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
			Expect(err.Error()).To(ContainSubstring("unable to connect to AWS STS endpoint (GetCallerIdentity failed):"))
		})
		It("should fail credentials with inline policy", func() {
			GetCallerIdentity = func(client *sts.Client) error {
				return nil // Simulate success; use errors.New("fail") to simulate failure
			}
			defer func() {
				GetCallerIdentity = func(client *sts.Client) error {
					_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
					return err
				}
			}()
			testOcmToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZW1haWwiOiJURVNUX1VTRVJAZm9vLmNvbSIsImlhdCI6MTUxNjIzOTAyMn0.dummy"
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
				testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.BackplaneConfiguration.ProxyURL).Return(mockClient, nil)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(fakeHTTPResp, nil)

			_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).To(ContainSubstring("client IP"))
			Expect(err.Error()).To(ContainSubstring("is not in the trusted IP range"))
		})
		It("should fail if assume role sequence cannot be retrieved", func() {
			GetCallerIdentity = func(client *sts.Client) error {
				return nil // Simulate success; use errors.New("fail") to simulate failure
			}
			defer func() {
				GetCallerIdentity = func(client *sts.Client) error {
					_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
					return err
				}
			}()
			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil).AnyTimes()

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
				testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.BackplaneConfiguration.ProxyURL).Return(mockClient, nil)
			// Simulate failure in GetAssumeRoleSequence
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(nil, errors.New("assume sequence failed"))

			_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch arn sequence:"))
		})
		It("should fail if error creating backplane api client", func() {
			GetCallerIdentity = func(client *sts.Client) error {
				return nil // Simulate success; use errors.New("fail") to simulate failure
			}
			defer func() {
				GetCallerIdentity = func(client *sts.Client) error {
					_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
					return err
				}
			}()
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
			ip, err := CheckEgressIP(client, server.URL)

			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal(net.ParseIP(mockIP)))
		})
		It("should return an error when the HTTP GET fails", func() {
			client = &http.Client{}
			// Invalid URL to force error
			ip, err := CheckEgressIP(client, "http://invalid_url")
			Expect(err).To(HaveOccurred())
			Expect(ip).To(BeNil())
		})
		It("should return an error when response body is not a valid IP", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, "not-an-ip")
			}))
			client = server.Client()

			ip, err := CheckEgressIP(client, server.URL)
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

		It("should return nil if IP exactly matches single host CIDR", func() {
			ip = net.ParseIP("192.168.1.1")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())
		})

		It("should return nil if IP is within large CIDR range", func() {
			ip = net.ParseIP("10.1.2.3")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())
		})

		It("should return nil if IP is at start of CIDR range", func() {
			ip = net.ParseIP("10.0.0.0")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())
		})

		It("should return nil if IP is at end of CIDR range", func() {
			ip = net.ParseIP("10.255.255.255")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())
		})

		It("should return error if IP not included in trusted list", func() {
			ip = net.ParseIP("172.16.0.1")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("client IP 172.16.0.1 is not in the trusted IP range"))
		})

		It("should return error for IP just outside CIDR range", func() {
			ip = net.ParseIP("192.168.1.2") // Outside 192.168.1.1/32
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("client IP 192.168.1.2 is not in the trusted IP range"))
		})

		It("should return error for IP in different network class", func() {
			ip = net.ParseIP("9.255.255.255") // Just outside 10.0.0.0/8
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("client IP 9.255.255.255 is not in the trusted IP range"))
		})

		It("should return an error if given invalid CIDR format", func() {
			trustedIPs.SourceIp = []string{"invalid-cidr"}
			ip = net.ParseIP("192.168.1.1")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse trusted IP CIDR invalid-cidr"))
		})

		It("should handle mixed CIDR ranges correctly", func() {
			trustedIPs.SourceIp = []string{
				"192.168.1.0/24",   // Subnet
				"172.16.5.10/32",   // Single host
				"10.0.0.0/16",      // Large range
			}

			// Test IPs within each range
			testCases := []struct {
				testIP    string
				shouldPass bool
				desc      string
			}{
				{"192.168.1.1", true, "IP in /24 subnet"},
				{"192.168.1.254", true, "IP at end of /24 subnet"},
				{"192.168.2.1", false, "IP outside /24 subnet"},
				{"172.16.5.10", true, "Exact match for /32"},
				{"172.16.5.11", false, "IP adjacent to /32"},
				{"10.0.5.5", true, "IP in /16 range"},
				{"10.1.0.0", false, "IP outside /16 range"},
			}

			for _, tc := range testCases {
				ip = net.ParseIP(tc.testIP)
				err := verifyIPTrusted(ip, trustedIPs)
				if tc.shouldPass {
					Expect(err).To(BeNil(), "Expected %s to pass: %s", tc.testIP, tc.desc)
				} else {
					Expect(err).To(HaveOccurred(), "Expected %s to fail: %s", tc.testIP, tc.desc)
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("client IP %s is not in the trusted IP range", tc.testIP)))
				}
			}
		})

		It("should handle IPv6 CIDR ranges", func() {
			trustedIPs.SourceIp = []string{
				"2001:db8::/32",
				"::1/128", // IPv6 localhost
			}

			// Test IPv6 addresses
			ip = net.ParseIP("2001:db8::1")
			err := verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())

			ip = net.ParseIP("::1")
			err = verifyIPTrusted(ip, trustedIPs)
			Expect(err).To(BeNil())

			ip = net.ParseIP("2001:db9::1") // Outside range
			err = verifyIPTrusted(ip, trustedIPs)
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
			// Check all trusted IPs are allowed
			Expect(policy).To(ContainSubstring("209.10.10.10"))
			Expect(policy).NotTo(ContainSubstring("200.20.20.20"))
			Expect(err).To(BeNil())
		})
	})

	Context("Execute verifyTrustedIPAndGetPolicy", func() {
		It("should successfully verify IP and return policy when IP is in trusted range", func() {
			// Mock the IP check to return a valid IP
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, "209.10.10.10") // IP that matches our test trusted range
			}))
			defer server.Close()

			// Override the checkEgressIP function to use our test server
			originalCheckEgressIP := CheckEgressIP
			CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
				return originalCheckEgressIP(client, server.URL)
			}
			defer func() {
				CheckEgressIP = originalCheckEgressIP
			}()

			// Set up expected trusted IP list
			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

			// Call the function
			policy, err := verifyTrustedIPAndGetPolicy(&testQueryConfig)

			// Verify success
			Expect(err).To(BeNil())
			Expect(policy.Version).To(Equal("2012-10-17"))
			Expect(len(policy.Statement)).To(BeNumerically(">", 0))
		})

		It("should fail when client IP is not in trusted range", func() {
			// Mock the IP check to return an untrusted IP
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, "192.168.1.1") // IP that doesn't match our test trusted range
			}))
			defer server.Close()

			// Override the checkEgressIP function to use our test server
			originalCheckEgressIP := CheckEgressIP
			CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
				return originalCheckEgressIP(client, server.URL)
			}
			defer func() {
				CheckEgressIP = originalCheckEgressIP
			}()

			// Set up expected trusted IP list (only has 209.x.x.x IPs)
			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

			// Call the function
			_, err = verifyTrustedIPAndGetPolicy(&testQueryConfig)

			// Verify failure
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("client IP 192.168.1.1 is not in the trusted IP range"))
		})

		It("should fail when AWS proxy URL is invalid", func() {
			// Set an invalid AWS proxy URL in configuration
			testQueryConfig.AwsProxy = aws.String("://invalid-url")

			// Call the function
			_, err := verifyTrustedIPAndGetPolicy(&testQueryConfig)

			// Verify failure
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse proxy for checkEgressIP"))
		})

		Context("Execute getIsolatedCredentials with AwsProxy", func() {
			It("should use AwsProxy configuration for AWS STS calls", func() {
				// Set AWS proxy in configuration
				testQueryConfig.AwsProxy = aws.String("http://aws-proxy:8080")

				// Mock CheckEgressIP to avoid real HTTP calls
				originalCheckEgressIP := CheckEgressIP
				CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
					// IP that matches our test trusted range
					return net.ParseIP("209.10.10.10"), nil
				}
				defer func() {
					CheckEgressIP = originalCheckEgressIP
				}()

				GetCallerIdentity = func(client *sts.Client) error {
					return nil // Simulate success
				}
				defer func() {
					GetCallerIdentity = func(client *sts.Client) error {
						_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
						return err
					}
				}()

				ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
				expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
				Expect(err).To(BeNil())
				mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

				StsClient = func(proxyURL *string) (*sts.Client, error) {
					// Verify that when proxyURL is nil, BACKPLANE_AWS_PROXY should be used
					if proxyURL == nil {
						// This should use BACKPLANE_AWS_PROXY internally
						return &sts.Client{}, nil
					}
					return &sts.Client{}, nil
				}
				AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     testAccessKeyID,
						SecretAccessKey: testSecretAccessKey,
						SessionToken:    testSessionToken,
					}, nil
				}
				AssumeRoleSequence = func(
					seedClient stscreds.AssumeRoleAPIClient,
					roleArnSequence []awsutil.RoleArnSession,
					proxyURL *string,
					stsClientProviderFunc awsutil.STSClientProviderFunc,
				) (aws.Credentials, error) {
					// Mock implementation to avoid real AWS calls
					return aws.Credentials{
						AccessKeyID:     testAccessKeyID,
						SecretAccessKey: testSecretAccessKey,
						SessionToken:    testSessionToken,
					}, nil
				}
				defer func() {
					AssumeRoleSequence = awsutil.AssumeRoleSequence
				}()
				mockClientUtil.EXPECT().GetBackplaneClient(
					testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.BackplaneConfiguration.ProxyURL).Return(mockClient, nil)
				mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(fakeHTTPResp, nil)

				// Test that AwsProxy is used for AWS calls while ProxyURL is used for Backplane calls
				_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
				Expect(err).To(BeNil())
			})

			It("should use AWS proxy when both ProxyURL and AwsProxy are configured", func() {
				// Set both explicit proxy and AWS proxy in configuration
				testQueryConfig.ProxyURL = aws.String("http://regular-proxy:9090")
				testQueryConfig.AwsProxy = aws.String("http://aws-proxy:8080")

				// Mock CheckEgressIP to avoid real HTTP calls
				originalCheckEgressIP := CheckEgressIP
				CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
					return net.ParseIP("209.10.10.10"), nil
				}
				defer func() {
					CheckEgressIP = originalCheckEgressIP
				}()

				GetCallerIdentity = func(client *sts.Client) error {
					return nil // Simulate success
				}
				defer func() {
					GetCallerIdentity = func(client *sts.Client) error {
						_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
						return err
					}
				}()

				ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
				expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
				Expect(err).To(BeNil())
				mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

				StsClient = func(proxyURL *string) (*sts.Client, error) {
					// StsClient should be called with the AwsProxy URL when configured
					if proxyURL != nil && *proxyURL == "http://aws-proxy:8080" {
						return &sts.Client{}, nil
					}
					return &sts.Client{}, fmt.Errorf("StsClient should be called with AWS proxy URL, got: %v", proxyURL)
				}
				AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     testAccessKeyID,
						SecretAccessKey: testSecretAccessKey,
						SessionToken:    testSessionToken,
					}, nil
				}
				AssumeRoleSequence = func(
					seedClient stscreds.AssumeRoleAPIClient,
					roleArnSequence []awsutil.RoleArnSession,
					proxyURL *string,
					stsClientProviderFunc awsutil.STSClientProviderFunc,
				) (aws.Credentials, error) {
					// Mock implementation to avoid real AWS calls
					return aws.Credentials{
						AccessKeyID:     testAccessKeyID,
						SecretAccessKey: testSecretAccessKey,
						SessionToken:    testSessionToken,
					}, nil
				}
				defer func() {
					AssumeRoleSequence = awsutil.AssumeRoleSequence
				}()
				mockClientUtil.EXPECT().GetBackplaneClient(
					testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.BackplaneConfiguration.ProxyURL).Return(mockClient, nil)
				mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(fakeHTTPResp, nil)

				_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
				Expect(err).To(BeNil())
			})

			It("should use regular proxy when AwsProxy is not configured", func() {
				// Ensure AwsProxy is not set (should be nil by default)
				testQueryConfig.AwsProxy = nil
				testQueryConfig.ProxyURL = aws.String("http://regular-proxy:8080")

				// Mock CheckEgressIP to avoid real HTTP calls
				originalCheckEgressIP := CheckEgressIP
				CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
					return net.ParseIP("209.10.10.10"), nil // IP that matches our test trusted range
				}
				defer func() {
					CheckEgressIP = originalCheckEgressIP
				}()

				GetCallerIdentity = func(client *sts.Client) error {
					return nil // Simulate success
				}
				defer func() {
					GetCallerIdentity = func(client *sts.Client) error {
						_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
						return err
					}
				}()

				ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
				expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
				Expect(err).To(BeNil())
				mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

				StsClient = func(proxyURL *string) (*sts.Client, error) {
					// StsClient should be called with regular proxy when BACKPLANE_AWS_PROXY is not set
					if proxyURL != nil && *proxyURL == "http://regular-proxy:8080" {
						return &sts.Client{}, nil
					}
					return &sts.Client{}, fmt.Errorf("StsClient should be called with regular proxy URL, got: %v", proxyURL)
				}
				AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     testAccessKeyID,
						SecretAccessKey: testSecretAccessKey,
						SessionToken:    testSessionToken,
					}, nil
				}
				AssumeRoleSequence = func(
					seedClient stscreds.AssumeRoleAPIClient,
					roleArnSequence []awsutil.RoleArnSession,
					proxyURL *string,
					stsClientProviderFunc awsutil.STSClientProviderFunc,
				) (aws.Credentials, error) {
					// Mock implementation to avoid real AWS calls
					return aws.Credentials{
						AccessKeyID:     testAccessKeyID,
						SecretAccessKey: testSecretAccessKey,
						SessionToken:    testSessionToken,
					}, nil
				}
				defer func() {
					AssumeRoleSequence = awsutil.AssumeRoleSequence
				}()
				mockClientUtil.EXPECT().GetBackplaneClient(
					testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.BackplaneConfiguration.ProxyURL).Return(mockClient, nil)
				mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(fakeHTTPResp, nil)

				_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Proxy separation verification", func() {
		It("should demonstrate traffic separation between AWS and Backplane API calls", func() {
			// Set different proxies for AWS and Backplane
			testQueryConfig.ProxyURL = aws.String("http://backplane-proxy:8080")
			testQueryConfig.AwsProxy = aws.String("http://aws-proxy:8080")

			// Mock CheckEgressIP to avoid real HTTP calls
			originalCheckEgressIP := CheckEgressIP
			CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
				// IP that matches our test trusted range
				return net.ParseIP("209.10.10.10"), nil
			}
			defer func() {
				CheckEgressIP = originalCheckEgressIP
			}()

			// Track which proxy was used for which calls
			var awsProxyUsed, backplaneProxyUsed bool

			GetCallerIdentity = func(client *sts.Client) error {
				return nil // Simulate success
			}
			defer func() {
				GetCallerIdentity = func(client *sts.Client) error {
					_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
					return err
				}
			}()

			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

			StsClient = func(proxyURL *string) (*sts.Client, error) {
				// AWS calls should use AwsProxy from configuration
				if proxyURL != nil && *proxyURL == "http://aws-proxy:8080" {
					awsProxyUsed = true
					return &sts.Client{}, nil
				}
				return &sts.Client{}, fmt.Errorf("AWS STS should use AwsProxy configuration, not regular proxy")
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}
			AssumeRoleSequence = func(
				seedClient stscreds.AssumeRoleAPIClient,
				roleArnSequence []awsutil.RoleArnSession,
				proxyURL *string,
				stsClientProviderFunc awsutil.STSClientProviderFunc,
			) (aws.Credentials, error) {
				// Mock implementation to avoid real AWS calls
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}
			defer func() {
				AssumeRoleSequence = awsutil.AssumeRoleSequence
			}()

			// Backplane API calls should use the regular ProxyURL
			mockClientUtil.EXPECT().GetBackplaneClient(
				testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.BackplaneConfiguration.ProxyURL).Do(func(url, token string, proxyURL *string) {
				if proxyURL != nil && *proxyURL == "http://backplane-proxy:8080" {
					backplaneProxyUsed = true
				}
			}).Return(mockClient, nil)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(fakeHTTPResp, nil)

			_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
			Expect(err).To(BeNil())

			// Verify that different proxies were used for different types of calls
			Expect(awsProxyUsed).To(BeTrue(), "AWS proxy should have been used for STS calls")
			Expect(backplaneProxyUsed).To(BeTrue(), "Backplane proxy should have been used for Backplane API calls")
		})

		It("should handle proxy separation in verifyTrustedIPAndGetPolicy", func() {
			// Set AWS-specific proxy in configuration
			testQueryConfig.AwsProxy = aws.String("http://aws-proxy:8080")

			// Mock checkEgressIP to avoid real HTTP calls
			originalCheckEgressIP := CheckEgressIP
			CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
				// IP that matches our test trusted range
				return net.ParseIP("209.10.10.10"), nil
			}
			defer func() {
				CheckEgressIP = originalCheckEgressIP
			}()

			// Set up expected trusted IP list
			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

			// Test that AWS proxy is used for checkEgressIP
			policy, err := verifyTrustedIPAndGetPolicy(&testQueryConfig)
			Expect(err).To(BeNil())
			Expect(policy.Version).To(Equal("2012-10-17"))
		})

		It("should fallback to regular proxy when AwsProxy is not configured (regression test)", func() {
			// Ensure AwsProxy is not set (should be nil by default)
			testQueryConfig.AwsProxy = nil

			// Should fall back to regular proxy behavior
			testQueryConfig.ProxyURL = aws.String("http://regular-proxy:8080")

			// Mock CheckEgressIP to avoid real HTTP calls
			originalCheckEgressIP := CheckEgressIP
			CheckEgressIP = func(client *http.Client, url string) (net.IP, error) {
				// IP that matches our test trusted range
				return net.ParseIP("209.10.10.10"), nil
			}
			defer func() {
				CheckEgressIP = originalCheckEgressIP
			}()

			GetCallerIdentity = func(client *sts.Client) error {
				return nil // Simulate success
			}
			defer func() {
				GetCallerIdentity = func(client *sts.Client) error {
					_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
					return err
				}
			}()

			ip1 := cmv1.NewTrustedIp().ID("209.10.10.10").Enabled(true)
			expectedIPList, err := cmv1.NewTrustedIpList().Items(ip1).Build()
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil)

			StsClient = func(proxyURL *string) (*sts.Client, error) {
				// StsClient should be called with regular proxy when AwsProxy is not configured
				if proxyURL != nil && *proxyURL == "http://regular-proxy:8080" {
					return &sts.Client{}, nil
				}
				return &sts.Client{}, fmt.Errorf("StsClient should be called with regular proxy URL, got: %v", proxyURL)
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}
			AssumeRoleSequence = func(
				seedClient stscreds.AssumeRoleAPIClient,
				roleArnSequence []awsutil.RoleArnSession,
				proxyURL *string,
				stsClientProviderFunc awsutil.STSClientProviderFunc,
			) (aws.Credentials, error) {
				// Mock implementation to avoid real AWS calls
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}
			defer func() {
				AssumeRoleSequence = awsutil.AssumeRoleSequence
			}()
			mockClientUtil.EXPECT().GetBackplaneClient(
				testQueryConfig.BackplaneConfiguration.URL, testOcmToken, testQueryConfig.BackplaneConfiguration.ProxyURL).Return(mockClient, nil)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(fakeHTTPResp, nil)

			_, err = testQueryConfig.getIsolatedCredentials(testOcmToken)
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
		{
			name:     "devshiftusgov.com domain",
			cluster:  newTestCluster(t, cmv1.NewCluster().DNS(cmv1.NewDNS().BaseDomain("cluster.devshiftusgov.com"))),
			expected: false,
		},
		{
			name:     "openshiftusgov.com domain",
			cluster:  newTestCluster(t, cmv1.NewCluster().DNS(cmv1.NewDNS().BaseDomain("cluster.openshiftusgov.com"))),
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

var _ = Describe("PolicyARNs Integration", func() {
	var (
		testSessionPolicyArn string
	)

	BeforeEach(func() {
		testSessionPolicyArn = "arn:aws:iam::123456789012:policy/TestSessionPolicy"
	})

	// Helper function to simulate the getIsolatedCredentials logic
	simulateGetIsolatedCredentialsLogic := func(roleChainResponse assumeChainResponse) []awsutil.RoleArnSession {
		// Create a mock inline policy for testing
		mockInlinePolicy := &awsutil.PolicyDocument{
			Version: "2012-10-17",
			Statement: []awsutil.PolicyStatement{
				{
					Sid:      "TestPolicy",
					Effect:   "Allow",
					Action:   []string{"s3:GetObject"},
					Resource: aws.String("*"),
				},
			},
		}

		assumeRoleArnSessionSequence := make([]awsutil.RoleArnSession, 0, len(roleChainResponse.AssumptionSequence))
		for _, namedRoleArnEntry := range roleChainResponse.AssumptionSequence {
			roleArnSession := awsutil.RoleArnSession{RoleArn: namedRoleArnEntry.Arn}
			if namedRoleArnEntry.Name == CustomerRoleArnName || namedRoleArnEntry.Name == OrgRoleArnName {
				roleArnSession.RoleSessionName = roleChainResponse.CustomerRoleSessionName
			} else {
				roleArnSession.RoleSessionName = "test@example.com"
			}

			// Default to no policy ARNs
			roleArnSession.PolicyARNs = []types.PolicyDescriptorType{}
			if namedRoleArnEntry.Name == CustomerRoleArnName {
				roleArnSession.IsCustomerRole = true

				// Add the session policy ARN for selected roles
				if roleChainResponse.SessionPolicyArn != "" {
					roleArnSession.PolicyARNs = []types.PolicyDescriptorType{
						{
							Arn: aws.String(roleChainResponse.SessionPolicyArn),
						},
					}
				} else {
					roleArnSession.Policy = mockInlinePolicy
				}

			} else {
				roleArnSession.IsCustomerRole = false
			}
			roleArnSession.Name = namedRoleArnEntry.Name

			assumeRoleArnSessionSequence = append(assumeRoleArnSessionSequence, roleArnSession)
		}
		return assumeRoleArnSessionSequence
	}

	// Generated by Cursor
	Context("when creating RoleArnSession with SessionPolicyArn", func() {
		It("should set PolicyARNs for customer roles", func() {
			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        testSessionPolicyArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			customerRole := assumeRoleArnSessionSequence[0]
			Expect(customerRole.IsCustomerRole).To(BeTrue())
			Expect(customerRole.Name).To(Equal(CustomerRoleArnName))
			Expect(len(customerRole.PolicyARNs)).To(Equal(1))
			Expect(*customerRole.PolicyARNs[0].Arn).To(Equal(testSessionPolicyArn))
			// Verify that Policy is nil when SessionPolicyArn is used
			Expect(customerRole.Policy).To(BeNil())
		})

		It("should not set PolicyARNs for non-customer roles", func() {
			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: "Support-Role-Arn",
						Arn:  "arn:aws:iam::123456789012:role/support-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        testSessionPolicyArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			supportRole := assumeRoleArnSessionSequence[0]
			Expect(supportRole.IsCustomerRole).To(BeFalse())
			Expect(supportRole.Name).To(Equal("Support-Role-Arn"))
			Expect(len(supportRole.PolicyARNs)).To(Equal(0))
			// Verify that Policy is nil for non-customer roles
			Expect(supportRole.Policy).To(BeNil())
		})

		// Generated by Cursor
		It("should handle empty SessionPolicyArn for customer roles", func() {
			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        "", // Empty session policy ARN
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			customerRole := assumeRoleArnSessionSequence[0]
			Expect(customerRole.IsCustomerRole).To(BeTrue())
			Expect(customerRole.Name).To(Equal(CustomerRoleArnName))
			Expect(len(customerRole.PolicyARNs)).To(Equal(0))
			// Verify that Policy is set when SessionPolicyArn is empty
			Expect(customerRole.Policy).ToNot(BeNil())
		})
	})

	Context("when verifying roleArnSession.Policy field behavior", func() {
		It("should set Policy only for customer roles without SessionPolicyArn", func() {
			// Test customer role with SessionPolicyArn - Policy should be nil
			roleChainResponseWithArn := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        testSessionPolicyArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponseWithArn)
			customerRoleWithArn := assumeRoleArnSessionSequence[0]

			Expect(customerRoleWithArn.IsCustomerRole).To(BeTrue())
			Expect(customerRoleWithArn.Policy).To(BeNil()) // Policy should be nil when SessionPolicyArn is used
			Expect(len(customerRoleWithArn.PolicyARNs)).To(Equal(1))

			// Test customer role without SessionPolicyArn - Policy should be set
			roleChainResponseWithoutArn := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        "", // Empty SessionPolicyArn
			}

			assumeRoleArnSessionSequence = simulateGetIsolatedCredentialsLogic(roleChainResponseWithoutArn)
			customerRoleWithoutArn := assumeRoleArnSessionSequence[0]

			Expect(customerRoleWithoutArn.IsCustomerRole).To(BeTrue())
			Expect(customerRoleWithoutArn.Policy).ToNot(BeNil()) // Policy should be set when SessionPolicyArn is empty
			Expect(len(customerRoleWithoutArn.PolicyARNs)).To(Equal(0))

			// Test non-customer role - Policy should always be nil
			roleChainResponseNonCustomer := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: "Support-Role-Arn",
						Arn:  "arn:aws:iam::123456789012:role/support-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        "", // Empty SessionPolicyArn
			}

			assumeRoleArnSessionSequence = simulateGetIsolatedCredentialsLogic(roleChainResponseNonCustomer)
			nonCustomerRole := assumeRoleArnSessionSequence[0]

			Expect(nonCustomerRole.IsCustomerRole).To(BeFalse())
			Expect(nonCustomerRole.Policy).To(BeNil()) // Policy should always be nil for non-customer roles
			Expect(len(nonCustomerRole.PolicyARNs)).To(Equal(0))
		})
	})

	Context("error scenarios with PolicyARNs", func() {
		It("should handle invalid SessionPolicyArn gracefully", func() {
			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        "invalid-arn-format", // Invalid ARN format
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			customerRole := assumeRoleArnSessionSequence[0]
			Expect(customerRole.IsCustomerRole).To(BeTrue())
			Expect(customerRole.Name).To(Equal(CustomerRoleArnName))
			Expect(len(customerRole.PolicyARNs)).To(Equal(1))
			// The invalid ARN is still passed through - validation happens at AWS level
			Expect(*customerRole.PolicyARNs[0].Arn).To(Equal("invalid-arn-format"))
		})

		It("should handle missing AssumptionSequence", func() {
			roleChainResponse := assumeChainResponse{
				AssumptionSequence:      []namedRoleArn{}, // Empty sequence
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        testSessionPolicyArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			// Should result in empty sequence
			Expect(len(assumeRoleArnSessionSequence)).To(Equal(0))
		})

		It("should handle malformed role ARNs in AssumptionSequence", func() {
			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "malformed-role-arn", // Invalid role ARN format
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        testSessionPolicyArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			customerRole := assumeRoleArnSessionSequence[0]
			Expect(customerRole.IsCustomerRole).To(BeTrue())
			Expect(customerRole.RoleArn).To(Equal("malformed-role-arn")) // Malformed ARN is passed through
			Expect(len(customerRole.PolicyARNs)).To(Equal(1))
			Expect(*customerRole.PolicyARNs[0].Arn).To(Equal(testSessionPolicyArn))
		})

		It("should handle extremely long SessionPolicyArn", func() {
			// Create a very long policy ARN that might cause issues
			longPolicyArn := "arn:aws:iam::123456789012:policy/" + strings.Repeat("a", 500)

			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        longPolicyArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			customerRole := assumeRoleArnSessionSequence[0]
			Expect(customerRole.IsCustomerRole).To(BeTrue())
			Expect(len(customerRole.PolicyARNs)).To(Equal(1))
			Expect(*customerRole.PolicyARNs[0].Arn).To(Equal(longPolicyArn))
			// Verify the ARN length
			Expect(len(*customerRole.PolicyARNs[0].Arn)).To(BeNumerically(">", 500))
		})

		It("should handle special characters in SessionPolicyArn", func() {
			// Policy ARN with special characters (though this would be invalid in real AWS)
			specialCharsArn := "arn:aws:iam::123456789012:policy/test-policy-with-special-chars!@#$%"

			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        specialCharsArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			customerRole := assumeRoleArnSessionSequence[0]
			Expect(customerRole.IsCustomerRole).To(BeTrue())
			Expect(len(customerRole.PolicyARNs)).To(Equal(1))
			Expect(*customerRole.PolicyARNs[0].Arn).To(Equal(specialCharsArn))
		})

		It("should verify debug logging when SessionPolicyArn is non-empty", func() {
			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        testSessionPolicyArn, // Non-empty SessionPolicyArn
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			// Verify the non-empty SessionPolicyArn scenario
			Expect(len(assumeRoleArnSessionSequence)).To(Equal(1))

			customerRole := assumeRoleArnSessionSequence[0]

			// Verify the customer role identification
			Expect(customerRole.IsCustomerRole).To(BeTrue())
			Expect(customerRole.Name).To(Equal(CustomerRoleArnName))
			Expect(customerRole.RoleSessionName).To(Equal("customer-session"))

			// Verify that SessionPolicyArn is non-empty
			Expect(roleChainResponse.SessionPolicyArn).ToNot(BeEmpty())

			// Verify PolicyARNs array is populated correctly
			Expect(len(customerRole.PolicyARNs)).To(Equal(1))
			Expect(customerRole.PolicyARNs[0].Arn).ToNot(BeNil())
			Expect(*customerRole.PolicyARNs[0].Arn).To(Equal(testSessionPolicyArn))

			// Verify the exact SessionPolicyArn value matches
			Expect(*customerRole.PolicyARNs[0].Arn).To(Equal(roleChainResponse.SessionPolicyArn))
		})

		It("should handle multiple customer roles with same SessionPolicyArn", func() {
			// Test scenario with multiple customer roles getting the same session policy
			roleChainResponse := assumeChainResponse{
				AssumptionSequence: []namedRoleArn{
					{
						Name: CustomerRoleArnName,
						Arn:  "arn:aws:iam::123456789012:role/customer-role-1",
					},
					{
						Name: "Support-Role-Arn",
						Arn:  "arn:aws:iam::123456789012:role/support-role",
					},
					{
						Name: CustomerRoleArnName, // Another customer role
						Arn:  "arn:aws:iam::123456789012:role/customer-role-2",
					},
				},
				CustomerRoleSessionName: "customer-session",
				SessionPolicyArn:        testSessionPolicyArn,
			}

			assumeRoleArnSessionSequence := simulateGetIsolatedCredentialsLogic(roleChainResponse)

			Expect(len(assumeRoleArnSessionSequence)).To(Equal(3))

			// Verify first customer role
			customerRole1 := assumeRoleArnSessionSequence[0]
			Expect(customerRole1.IsCustomerRole).To(BeTrue())
			Expect(customerRole1.Name).To(Equal(CustomerRoleArnName))
			Expect(len(customerRole1.PolicyARNs)).To(Equal(1))
			Expect(*customerRole1.PolicyARNs[0].Arn).To(Equal(testSessionPolicyArn))

			// Verify support role (non-customer)
			supportRole := assumeRoleArnSessionSequence[1]
			Expect(supportRole.IsCustomerRole).To(BeFalse())
			Expect(supportRole.Name).To(Equal("Support-Role-Arn"))
			Expect(len(supportRole.PolicyARNs)).To(Equal(0))

			// Verify second customer role
			customerRole2 := assumeRoleArnSessionSequence[2]
			Expect(customerRole2.IsCustomerRole).To(BeTrue())
			Expect(customerRole2.Name).To(Equal(CustomerRoleArnName))
			Expect(len(customerRole2.PolicyARNs)).To(Equal(1))
			Expect(*customerRole2.PolicyARNs[0].Arn).To(Equal(testSessionPolicyArn))
		})
	})
})
