package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

//nolint:gosec
var _ = Describe("Cloud console command", func() {

	var (
		mockCtrl           *gomock.Controller
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *ocmMock.MockOCMInterface
		mockClientUtil     *backplaneapiMock.MockClientUtils
		mockClusterUtils   *mocks2.MockClusterUtils

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
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		mockClusterUtils = mocks2.NewMockClusterUtils(mockCtrl)
		utils.DefaultClusterUtils = mockClusterUtils
		backplaneapi.DefaultClientUtils = mockClientUtil

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

		// Define fake GCP response
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
		_ = os.Setenv(info.BackplaneURLEnvName, proxyURI)
	})

	AfterEach(func() {
		_ = os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
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
	})
})

func TestRenderCloudCredentials(t *testing.T) {
	fakeAWSCredentialsResponse := &bpCredentials.AWSCredentialsResponse{
		AccessKeyID:     "foo",
		SecretAccessKey: "bar",
		SessionToken:    "baz",
		Region:          "quux",
	}

	fakeGCPCredentialsResponse := &bpCredentials.GCPCredentialsResponse{
		ProjectID: "foo",
	}

	tests := []struct {
		name         string
		outputFormat string
		creds        bpCredentials.Response
		expected     string
	}{
		{
			name:         "AWS empty",
			outputFormat: "",
			creds:        fakeAWSCredentialsResponse,
			expected:     fakeAWSCredentialsResponse.String(),
		},
		{
			name:         "AWS env",
			outputFormat: "env",
			creds:        fakeAWSCredentialsResponse,
			expected:     fakeAWSCredentialsResponse.FmtExport(),
		},
		{
			name:         "AWS json",
			outputFormat: "json",
			creds:        fakeAWSCredentialsResponse,
			expected:     `{"AccessKeyID":"foo","SecretAccessKey":"bar","SessionToken":"baz","Region":"quux","Expiration":""}`,
		},
		{
			name:         "AWS yaml",
			outputFormat: "yaml",
			creds:        fakeAWSCredentialsResponse,
			expected: `AccessKeyID: foo
Expiration: ""
Region: quux
SecretAccessKey: bar
SessionToken: baz
`,
		},
		{
			name:         "GCP env",
			outputFormat: "env",
			creds:        fakeGCPCredentialsResponse,
			expected:     fakeGCPCredentialsResponse.FmtExport(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, _ := renderCloudCredentials(test.outputFormat, test.creds)
			if test.expected != actual {
				t.Errorf("expected: %v, got: %v", test.expected, actual)
			}
		})
	}
}
