package cloud

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/awsutil"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	"io"
	"net/http"
	"strings"
	"time"
)

//nolint:gosec
var _ = Describe("Cloud assume command", func() {

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
		testClusterName     string
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
		testClusterName = "test-cluster"
		testExpiration = time.UnixMilli(1691606228384)
	})

	Context("Execute cloud assume command", func() {
		It("should return AWS STS credentials", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}
			NewStaticCredentialsProvider = func(key, secret, session string) credentials.StaticCredentialsProvider {
				return credentials.StaticCredentialsProvider{}
			}
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterName, nil).Times(1)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"assumption_sequence":[{"name": "name_one", "arn": "arn_one"},{"name": "name_two", "arn": "arn_two"}]}`)),
			}, nil).Times(1)
			AssumeRoleSequence = func(roleSessionName string, seedClient awsutil.STSRoleAssumer, roleArnSequence []string, proxyURL string, stsClientProviderFunc awsutil.STSClientProviderFunc) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
					Expiration:      &testExpiration,
				}, nil
			}

			err := runAssume(nil, []string{testClusterID})
			Expect(err).To(BeNil())
		})
		It("should fail if no argument or debug file is provided", func() {
			err := runAssume(nil, []string{})
			Expect(err).To(Equal(fmt.Errorf("must provide either cluster ID as an argument, or --debug-file as a flag")))
		})
		It("should fail if cannot retrieve OCM token", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("foo")).Times(1)

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to retrieve OCM token: foo"))
		})
		It("should fail if cannot retrieve backplane configuration", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{}, errors.New("oops")
			}

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("error retrieving backplane configuration: oops"))
		})
		It("should fail if cannot create create sts client with proxy", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return nil, errors.New(":(")
			}

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to create sts client: :("))
		})
		It("should fail if initial role cannot be assumed with JWT", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return nil, errors.New("failure")
			}

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to assume role using JWT: failure"))
		})
		It("should fail if email cannot be pulled off JWT", func() {
			testOcmToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("unable to extract email from given token: no field email on given token"))
		})
		It("should fail if cluster cannot be retrieved from OCM", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return("", "", errors.New("oh no")).Times(1)

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to get target cluster: oh no"))
		})
		It("should fail if error creating backplane api client", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}
			NewStaticCredentialsProvider = func(key, secret, session string) credentials.StaticCredentialsProvider {
				return credentials.StaticCredentialsProvider{}
			}
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterName, nil).Times(1)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(nil, errors.New("foo")).Times(1)

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to create backplane client with access token: foo"))
		})
		It("should fail if cannot retrieve role sequence", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}
			NewStaticCredentialsProvider = func(key, secret, session string) credentials.StaticCredentialsProvider {
				return credentials.StaticCredentialsProvider{}
			}
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterName, nil).Times(1)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(nil, errors.New("error")).Times(1)

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to fetch arn sequence: error"))
		})
		It("should fail if fetching assume role sequence doesn't return a 200 status code", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}
			NewStaticCredentialsProvider = func(key, secret, session string) credentials.StaticCredentialsProvider {
				return credentials.StaticCredentialsProvider{}
			}
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterName, nil).Times(1)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 401,
				Status:     "Unauthorized",
				Body:       io.NopCloser(strings.NewReader(`{"assumption_sequence":[{"name": "name_one", "arn": "arn_one"},{"name": "name_two", "arn": "arn_two"}]}`)),
			}, nil).Times(1)

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to fetch arn sequence: Unauthorized"))
		})
		It("should fail if it cannot unmarshal backplane API response", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}
			NewStaticCredentialsProvider = func(key, secret, session string) credentials.StaticCredentialsProvider {
				return credentials.StaticCredentialsProvider{}
			}
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterName, nil).Times(1)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil).Times(1)

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to unmarshal response: unexpected end of JSON input"))
		})
		It("should fail if it cannot assume the role sequence", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testOcmToken, nil).Times(1)
			GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
				return config.BackplaneConfiguration{
					URL:      "testUrl.com",
					ProxyURL: "testProxyUrl.com",
				}, nil
			}
			StsClientWithProxy = func(proxyURL string) (*sts.Client, error) {
				return &sts.Client{}, nil
			}
			AssumeRoleWithJWT = func(jwt string, roleArn string, stsClient awsutil.STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
				return &types.Credentials{
					AccessKeyId:     &testAccessKeyID,
					SecretAccessKey: &testSecretAccessKey,
					SessionToken:    &testSessionToken,
				}, nil
			}
			NewStaticCredentialsProvider = func(key, secret, session string) credentials.StaticCredentialsProvider {
				return credentials.StaticCredentialsProvider{}
			}
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterName, nil).Times(1)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("testUrl.com", testOcmToken).Return(mockClient, nil).Times(1)
			mockClient.EXPECT().GetAssumeRoleSequence(context.TODO(), testClusterID).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"assumption_sequence":[{"name": "name_one", "arn": "arn_one"},{"name": "name_two", "arn": "arn_two"}]}`)),
			}, nil).Times(1)
			AssumeRoleSequence = func(roleSessionName string, seedClient awsutil.STSRoleAssumer, roleArnSequence []string, proxyURL string, stsClientProviderFunc awsutil.STSClientProviderFunc) (*types.Credentials, error) {
				return nil, errors.New("oops")
			}

			err := runAssume(nil, []string{testClusterID})
			Expect(err.Error()).To(Equal("failed to assume role sequence: oops"))
		})
	})
})
