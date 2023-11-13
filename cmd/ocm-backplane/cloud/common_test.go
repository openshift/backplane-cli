package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/awsutil"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
)

//nolint:gosec
var _ = Describe("getIsolatedCredentials", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *mocks2.MockOCMInterface
		mockClientUtil   *mocks2.MockClientUtils
		mockClient       *mocks.MockClientInterface

		testOcmToken        string
		testClusterID       string
		testAccessKeyID     string
		testSecretAccessKey string
		testSessionToken    string
		testExpiration      time.Time
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())

		mockClient = mocks.NewMockClientInterface(mockCtrl)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

		testOcmToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZW1haWwiOiJ0ZXN0QGZvby5jb20iLCJpYXQiOjE1MTYyMzkwMjJ9.5NG4wFhitEKZyzftSwU67kx4JVTEWcEoKhCl_AFp8T4"
		testClusterID = "test123"
		testAccessKeyID = "test-access-key-id"
		testSecretAccessKey = "test-secret-access-key"
		testSessionToken = "test-session-token"
		testExpiration = time.UnixMilli(1691606228384)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Execute getIsolatedCredentials", func() {
		It("should return AWS STS credentials", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
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
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"assumption_sequence":[{"name": "name_one", "arn": "arn_one"},{"name": "name_two", "arn": "arn_two"}]}`)),
			}, nil).Times(1)
			AssumeRoleSequence = func(roleSessionName string, seedClient stscreds.AssumeRoleAPIClient, roleArnSequence []string, proxyURL string, stsClientProviderFunc awsutil.STSClientProviderFunc) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
					Expires:         testExpiration,
				}, nil
			}

			credentials, err := getIsolatedCredentials(testClusterID)
			Expect(err).To(BeNil())
			Expect(credentials).To(Equal(aws.Credentials{
				AccessKeyID:     testAccessKeyID,
				SecretAccessKey: testSecretAccessKey,
				SessionToken:    testSessionToken,
				Source:          "",
				CanExpire:       false,
				Expires:         testExpiration,
			}))
		})
		It("should fail if no argument is provided", func() {
			_, err := getIsolatedCredentials("")
			Expect(err).To(Equal(fmt.Errorf("must provide non-empty cluster ID")))
		})
		It("should fail if cannot retrieve OCM token", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("foo")).Times(1)

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to retrieve OCM token: foo"))
		})
		It("should fail if cannot retrieve backplane configuration", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{}, errors.New("oops")
			}

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("error retrieving backplane configuration: oops"))
		})
		It("should fail if backplane configuration does not contain value for AssumeInitialArn", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "",
				}, nil
			}

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("backplane config is missing required `assume-initial-arn` property"))
		})
		It("should fail if cannot create sts client with proxy", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return nil, errors.New(":(")
			}

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to create sts client: :("))
		})
		It("should fail if initial role cannot be assumed with JWT", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{}, errors.New("failure")
			}

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to assume role using JWT: failure"))
		})
		It("should fail if email cannot be pulled off JWT", func() {
			testOcmToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     testAccessKeyID,
					SecretAccessKey: testSecretAccessKey,
					SessionToken:    testSessionToken,
				}, nil
			}

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("unable to extract email from given token: no field email on given token"))
		})
		It("should fail if error creating backplane api client", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
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
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(nil, errors.New("foo")).Times(1)

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to create backplane client with access token: foo"))
		})
		It("should fail if cannot retrieve role sequence", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
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
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(nil, errors.New("error")).Times(1)

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to fetch arn sequence: error"))
		})
		It("should fail if fetching assume role sequence doesn't return a 200 status code", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
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
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 401,
				Status:     "Unauthorized",
				Body:       io.NopCloser(strings.NewReader(`{"assumption_sequence":[{"name": "name_one", "arn": "arn_one"},{"name": "name_two", "arn": "arn_two"}]}`)),
			}, nil).Times(1)

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to fetch arn sequence: Unauthorized"))
		})
		It("should fail if it cannot unmarshal backplane API response", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
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
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil).Times(1)

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to unmarshal response: unexpected end of JSON input"))
		})
		It("should fail if it cannot assume the role sequence", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:              "testUrl.com",
					ProxyURL:         "testProxyUrl.com",
					AssumeInitialArn: "arn:aws:iam::123456789:role/ManagedOpenShift-Support-Role",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
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
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"assumption_sequence":[{"name": "name_one", "arn": "arn_one"},{"name": "name_two", "arn": "arn_two"}]}`)),
			}, nil).Times(1)
			AssumeRoleSequence = func(roleSessionName string, seedClient stscreds.AssumeRoleAPIClient, roleArnSequence []string, proxyURL string, stsClientProviderFunc awsutil.STSClientProviderFunc) (aws.Credentials, error) {
				return aws.Credentials{}, errors.New("oops")
			}

			_, err := getIsolatedCredentials(testClusterID)
			Expect(err.Error()).To(Equal("failed to assume role sequence: oops"))
		})
	})

})

var _ = Describe("isIsolatedBackplaneAccess", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *mocks2.MockOCMInterface

		testClusterID string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		testClusterID = "test123"
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Execute isIsolatedBackplaneAccess", func() {
		It("returns false with no error if cluster has no AWS field", func() {
			result, err := isIsolatedBackplaneAccess(&v1.Cluster{})

			Expect(result).To(Equal(false))
			Expect(err).To(BeNil())
		})
		It("returns false with no error if cluster AWS field has no STS field", func() {
			clusterBuilder := v1.ClusterBuilder{}
			clusterBuilder.AWS(&v1.AWSBuilder{})
			cluster, _ := clusterBuilder.Build()
			result, err := isIsolatedBackplaneAccess(cluster)

			Expect(result).To(Equal(false))
			Expect(err).To(BeNil())
		})
		It("returns false with no error if cluster is non-STS enabled", func() {
			stsBuilder := &v1.STSBuilder{}
			stsBuilder.Enabled(false)

			awsBuilder := &v1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := v1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)

			cluster, _ := clusterBuilder.Build()
			result, err := isIsolatedBackplaneAccess(cluster)

			Expect(result).To(Equal(false))
			Expect(err).To(BeNil())
		})
		It("returns an error if fails to get STS Support Jump Role from OCM for STS enabled cluster", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(testClusterID).Return("", errors.New("oops"))

			stsBuilder := &v1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &v1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := v1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			_, err := isIsolatedBackplaneAccess(cluster)

			Expect(err).NotTo(BeNil())
		})
		It("returns an error if fails to parse STS Support Jump Role from OCM for STS enabled cluster", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(testClusterID).Return("not-an-arn", nil)

			stsBuilder := &v1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &v1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := v1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			_, err := isIsolatedBackplaneAccess(cluster)

			Expect(err).NotTo(BeNil())
		})
		It("returns false with no error for STS enabled cluster with ARN that matches old support flow ARN", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(testClusterID).Return("arn:aws:iam::123456789:role/RH-Technical-Support-Access", nil)

			stsBuilder := &v1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &v1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := v1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			result, err := isIsolatedBackplaneAccess(cluster)

			Expect(result).To(Equal(false))
			Expect(err).To(BeNil())
		})
		It("returns true with no error for STS enabled cluster with ARN that doesn't match old support flow ARN", func() {
			mockOcmInterface.EXPECT().GetStsSupportJumpRoleARN(testClusterID).Return("arn:aws:iam::123456789:role/RH-Technical-Support-12345", nil)

			stsBuilder := &v1.STSBuilder{}
			stsBuilder.Enabled(true)

			awsBuilder := &v1.AWSBuilder{}
			awsBuilder.STS(stsBuilder)

			clusterBuilder := v1.ClusterBuilder{}
			clusterBuilder.AWS(awsBuilder)
			clusterBuilder.ID(testClusterID)

			cluster, _ := clusterBuilder.Build()
			result, err := isIsolatedBackplaneAccess(cluster)

			Expect(result).To(Equal(true))
			Expect(err).To(BeNil())
		})
	})
})
