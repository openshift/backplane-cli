package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
)

//nolint:gosec
var _ = Describe("Cloud console command", func() {

	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *mocks2.MockOCMInterface
		mockClientUtil     *mocks2.MockClientUtils
		mockClusterUtils   *mocks2.MockClusterUtils

		trueClusterID string
		proxyURI      string
		credentialAWS string
		credentialGcp string

		fakeAWSResp           *http.Response
		fakeGCloudResp        *http.Response
		fakeBrokenAWSResp     *http.Response
		fakeBrokenGCPResp     *http.Response
		fakeMalformedJSONResp *http.Response
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

		mockClusterUtils = mocks2.NewMockClusterUtils(mockCtrl)
		utils.DefaultClusterUtils = mockClusterUtils
		utils.DefaultClientUtils = mockClientUtil

		trueClusterID = "trueID123"
		proxyURI = "https://shard.apps"
		credentialAWS = "fake aws credential"
		credentialGcp = "fake gcp credential"

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		// Define fake AWS response
		fakeAWSResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"credentials":"proxy", "message":"msg", "JSON200":"%s"}`, credentialAWS),
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeAWSResp.Header.Add("Content-Type", "json")

		// Define fake AWS response
		fakeGCloudResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"proxy_uri":"proxy", "message":"msg", "JSON200":"%s"}`, credentialGcp),
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeGCloudResp.Header.Add("Content-Type", "json")

		fakeMalformedJSONResp = &http.Response{
			Body: MakeIoReader(
				`{"proxy_uri":proxy"}`,
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeMalformedJSONResp.Header.Add("Content-Type", "json")

		// Define broken AWS response
		// https://stackoverflow.com/questions/32708717/go-when-will-json-unmarshal-to-struct-return-error
		resp, _ := json.Marshal(map[string]string{"credentials": "foo", "clusterID": "bar"})
		fakeBrokenAWSResp = &http.Response{
			Body:       io.NopCloser(bytes.NewReader(resp)),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeBrokenAWSResp.Header.Add("Content-Type", "json")

		// Define broken gcp response
		// https://stackoverflow.com/questions/32708717/go-when-will-json-unmarshal-to-struct-return-error
		// Define fake AWS response
		resp, _ = json.Marshal(map[string]string{"credentials": "foo", "clusterID": "bar"})
		fakeBrokenGCPResp = &http.Response{
			Body:       io.NopCloser(bytes.NewReader(resp)),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeBrokenGCPResp.Header.Add("Content-Type", "json")

		resp, _ = json.Marshal(map[string]string{"credentials": "foo", "clusterID": "bar"})
		fakeBrokenGCPResp = &http.Response{
			Body:       io.NopCloser(bytes.NewReader(resp)),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeBrokenGCPResp.Header.Add("Content-Type", "json")

		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false

		// Disabled log output
		log.SetOutput(io.Discard)
		os.Setenv(info.BackplaneURLEnvName, proxyURI)
	})

	AfterEach(func() {
		os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})

	Context("test get Cloud Credential", func() {

		It("should return AWS cloud credential", func() {

			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterID).Return(fakeAWSResp, nil)

			crdentialResponse, err := GetCloudCredentials(proxyURI, trueClusterID)
			Expect(err).To(BeNil())

			Expect(crdentialResponse.JSON200).NotTo(BeNil())

		})

		It("should fail when AWS Unavailable", func() {
			fakeAWSResp.StatusCode = http.StatusInternalServerError

			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterID).Return(fakeAWSResp, nil)
			_, err := GetCloudCredentials(proxyURI, trueClusterID)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).To(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail when GCP Unavailable", func() {
			fakeGCloudResp.StatusCode = http.StatusInternalServerError

			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterID).Return(fakeGCloudResp, nil)
			_, err := GetCloudCredentials(proxyURI, trueClusterID)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).To(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail when we can't parse the response from backplane", func() {
			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterID).Return(fakeMalformedJSONResp, nil)
			_, err := GetCloudCredentials(proxyURI, trueClusterID)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).To(ContainSubstring(fmt.Errorf("unable to parse response body from backplane:\n  Status Code: %d", 200).Error()))

		})

		It("should fail for unauthorized BP-API", func() {
			fakeAWSResp.StatusCode = http.StatusUnauthorized

			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterID).Return(fakeAWSResp, nil)
			_, err := GetCloudCredentials(proxyURI, trueClusterID)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 401\n"))

		})
	})

	Context("test runCredentials", func() {
		Context("one argument is given", func() {
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

			It("returns an error if GetBackplaneURL returns an error and credentialArgs.backplaneURL is empty", func() {
				GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
					return config.BackplaneConfiguration{}, errors.New("error bp url")

				}

				mockClusterUtils.EXPECT().GetCloudProvider(gomock.Any())

				mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return("foo", "bar", nil).AnyTimes()
				mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, nil).AnyTimes()
				Expect(runCredentials(&cobra.Command{}, []string{"cluster-key"})).To(Equal(
					fmt.Errorf("can't find backplane url: %w", errors.New("error bp url")),
				))
			})

			It("returns an error if we can't umarshal the cloud credentials response on aws", func() {
				GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
					return config.BackplaneConfiguration{
						URL: "https://foo.bar",
					}, nil
				}

				mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return("foo", "bar", nil).AnyTimes()
				mockClusterUtils.EXPECT().GetCloudProvider(gomock.Any()).Return("aws").AnyTimes()
				mockClientUtil.EXPECT().GetBackplaneClient("https://foo.bar").Return(mockClient, nil).Times(1)

				mockClient.EXPECT().GetCloudCredentials(gomock.Any(), "foo").Return(fakeBrokenAWSResp, nil).Times(1)
				mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, nil).AnyTimes()
				Expect(runCredentials(&cobra.Command{}, []string{"foo"}).Error()).To(ContainSubstring("unable to unmarshal AWS credentials response from backplane"))
			})

			It("returns an error if we can't umarshal the cloud credentials response on gcp", func() {
				GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
					return config.BackplaneConfiguration{
						URL: "https://foo.bar",
					}, nil
				}

				mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return("foo", "bar", nil).AnyTimes()
				mockClusterUtils.EXPECT().GetCloudProvider(gomock.Any()).Return("gcp").AnyTimes()
				mockClientUtil.EXPECT().GetBackplaneClient("https://foo.bar").Return(mockClient, nil).Times(1)

				mockClient.EXPECT().GetCloudCredentials(gomock.Any(), "foo").Return(fakeBrokenGCPResp, nil).Times(1)
				mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, nil).AnyTimes()
				Expect(runCredentials(&cobra.Command{}, []string{"foo"}).Error()).To(ContainSubstring("unable to unmarshal GCP credentials response from backplane"))
			})

			It("returns an error if there is an unknown cloud provider", func() {
				GetBackplaneConfiguration = func() (bpConfig config.BackplaneConfiguration, err error) {
					return config.BackplaneConfiguration{
						URL: "https://foo.bar",
					}, nil
				}
				mockClient.EXPECT().GetCloudCredentials(gomock.Any(), "foo").Return(fakeBrokenGCPResp, nil).Times(1)
				mockClientUtil.EXPECT().GetBackplaneClient("https://foo.bar").Return(mockClient, nil).Times(1)
				mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return("foo", "bar", nil).AnyTimes()
				mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(&cmv1.Cluster{}, nil).AnyTimes()
				mockClusterUtils.EXPECT().GetCloudProvider(gomock.Any()).Return("azure").AnyTimes()

				Expect(runCredentials(&cobra.Command{}, []string{"cluster-key"})).To(Equal(
					fmt.Errorf("unsupported cloud provider: %s", "azure"),
				))
			})
		})

		Context("no arguments are given", func() {
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
		})

		It("errors if more than one cluster keys are given", func() {
			err := runCredentials(&cobra.Command{}, []string{"two", "cluster-keys"})
			Expect(err).To(Equal(fmt.Errorf("expected exactly one cluster")))
		})
	})

	Context("test renderCloudCredentials", func() {
		creds := bpCredentials.AWSCredentialsResponse{
			AccessKeyID:     "foo",
			SecretAccessKey: "bar",
			SessionToken:    "baz",
			Region:          "quux",
		}

		It("prints the format export if the env output flag is supplied", func() {
			export := creds.FmtExport()

			Expect(renderCloudCredentials("env", &creds)).To(Equal(export))
		})

		It("prints the yaml export if the env output flag is supplied", func() {
			yamlBytes, _ := yaml.Marshal(creds)
			Expect(renderCloudCredentials("yaml", &creds)).To(ContainSubstring(string(yamlBytes)))
		})

		It("prints the yaml export if the env output flag is supplied", func() {
			yamlBytes, _ := yaml.Marshal(creds)
			Expect(renderCloudCredentials("yaml", &creds)).To(ContainSubstring(string(yamlBytes)))
		})

		It("prints the json export if the json output flagis supplied", func() {
			jsonBytes, _ := json.Marshal(creds)
			Expect(renderCloudCredentials("json", &creds)).To(ContainSubstring(string(jsonBytes)))
		})

		It("prints the default string export if no output flag is supplied", func() {
			Expect(renderCloudCredentials("", &creds)).To(ContainSubstring(creds.String()))
		})
	})
	Context("TestAWSCredentialsResponseString(", func() {
		It("It formats the output correctly", func() {
			r := &bpCredentials.AWSCredentialsResponse{
				AccessKeyID:     "12345",
				SecretAccessKey: "56789",
				SessionToken:    "sessiontoken",
				Region:          "region",
				Expiration:      "expir",
			}

			formattedcreds := `Temporary Credentials:
  AccessKeyID: 12345
  SecretAccessKey: 56789
  SessionToken: sessiontoken
  Region: region
  Expires: expir`
			Expect(formattedcreds).To(Equal(r.String()))
		})
	})
	Context("TestGCPCredentialsResponseString", func() {
		It("It formats the output correctly", func() {
			r := &bpCredentials.GCPCredentialsResponse{
				ProjectID: "foo",
			}
			expect := `If this is your first time, run "gcloud auth login" and then
gcloud config set project foo`

			Expect(expect).To(Equal(r.String()))
		})
	})
	Context("TestAWSCredentialsResponseFmtEformattedcredsxport", func() {
		It("It formats the output correctly", func() {
			r := &bpCredentials.AWSCredentialsResponse{
				AccessKeyID:     "foo",
				SecretAccessKey: "bar",
				SessionToken:    "baz",
				Region:          "quux",
				Expiration:      "now",
			}

			awsExportOut := `export AWS_ACCESS_KEY_ID=foo
export AWS_SECRET_ACCESS_KEY=bar
export AWS_SESSION_TOKEN=baz
export AWS_DEFAULT_REGION=quux`
			Expect(awsExportOut).To(Equal(r.FmtExport()))
		})
	})
	Context("TestGCPCredentialsResponseFmtExport", func() {
		It("It formats the output correctly", func() {
			r := &bpCredentials.GCPCredentialsResponse{
				ProjectID: "foo",
			}

			gcpExportFormatOut := `export CLOUDSDK_CORE_PROJECT=foo`

			Expect(gcpExportFormatOut).To(Equal(r.FmtExport()))
		})
	})
})
