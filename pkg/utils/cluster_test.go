// to avoid import cycles
package utils_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/clientcmd"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	loggedInYamlSingle = `
apiVersion: v1
clusters:
- cluster:
    server: https://api-backplane.apps.com/backplane/cluster/1f0o1maej9brj6j9k6ehbe7rm0k2lng7/
  name: dummy_cluster
contexts:
- context:
    cluster: dummy_cluster
    namespace: default
    user: example.openshift
  name: default/openshift
current-context: default/openshift
kind: Config
preferences: {}
users:
- name: example.openshift
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
      - /bin/echo nothing
      command: bash
      env: null
- name: blue-user
  user:
    token: blue-token
- name: green-user
  user:
    client-certificate: path/to/my/client/cert
    client-key: path/to/my/client/key
`

	loggedInNotBackplane = `
apiVersion: v1
clusters:
- cluster:
    server: https://myopenshiftcluster.openshiftapps.com
  name: myopenshiftcluster
contexts:
- context:
    cluster: myopenshiftcluster
    namespace: default
    user: example.openshift
  name: default/myopenshiftcluster/example.openshift
current-context: default/myopenshiftcluster/example.openshift
kind: Config
preferences: {}
users:
- name: example.openshift
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
      - /bin/echo nothing
      command: bash
      env: null
`

	invalidYaml = `
hello: world
`
)

func TestGetBackplaneClusterFromConfig(t *testing.T) {
	tests := []struct {
		config string
		expect utils.BackplaneCluster
	}{{
		config: loggedInYamlSingle,
		expect: utils.BackplaneCluster{
			ClusterID:     "1f0o1maej9brj6j9k6ehbe7rm0k2lng7",
			ClusterURL:    "https://api-backplane.apps.com/backplane/cluster/1f0o1maej9brj6j9k6ehbe7rm0k2lng7/",
			BackplaneHost: "api-backplane.apps.com",
		},
	}}

	for n, tt := range tests {
		config, _ := clientcmd.Load([]byte(tt.config))
		_ = utils.CreateTempKubeConfig(config)
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			result, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
			if err != nil {
				t.Errorf("%e", err)
			}
			if reflect.DeepEqual(result, tt.expect) {
				t.Errorf("Expecting: %s, but get: %s", tt.expect, result)
			}
		})
	}

	testErr := []struct {
		config string
	}{
		{
			config: loggedInNotBackplane,
		},
		{
			config: invalidYaml,
		},
	}

	for n, tt := range testErr {
		config, _ := clientcmd.Load([]byte(tt.config))
		_ = utils.CreateTempKubeConfig(config)
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			_, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
			if err == nil {
				t.Errorf("Expected error")
			}
		})
	}
}

func TestGetClusterIDAndHostFromClusterURL(t *testing.T) {
	tests := []struct {
		inp  string
		out0 string
		out1 string
	}{
		{
			inp:  "https://example.com/backplane/cluster/abcd123",
			out0: "abcd123",
			out1: "https://example.com",
		},
		{
			inp:  "http://example.com/foo/backplane/cluster/abcd123",
			out0: "abcd123",
			out1: "https://example.com",
		},
		{
			inp:  "https://api-backplane.apps.com/backplane/cluster/abcd123/",
			out0: "abcd123",
			out1: "https://api-backplane.apps.com",
		},
	}

	for n, tt := range tests {
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			o0, o1, err := utils.DefaultClusterUtils.GetClusterIDAndHostFromClusterURL(tt.inp)
			if err != nil {
				t.Errorf("%e", err)
			}
			if o0 != tt.out0 {
				t.Errorf("Expecting: %s, but get: %s", tt.out0, o0)
			}

			if o1 != tt.out1 {
				t.Errorf("Expecting: %s, but get: %s", tt.out1, o1)
			}
		})
	}

	testsErr := []struct {
		inp string
	}{
		{
			"magict@@@@!HAAHAH!#@$SDHBVDZNBZVCKZKKZK()*I&UYLKJLNp/////////////things.com/backplane/cluster/abc",
		},
		{
			"https://things.com/somethingelse/cluster/abc",
		},
		{
			"https://things.com/backplane/notcluster/abc",
		},
		{
			"https://things.com/backplane/cluster/",
		},
	}

	for n, tt := range testsErr {
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			_, _, err := utils.DefaultClusterUtils.GetClusterIDAndHostFromClusterURL(tt.inp)
			if err == nil {
				t.Errorf("expecting error for %s", tt.inp)
			}

		})
	}
}

func TestGetBackplaneClusterFromClusterKey(t *testing.T) {

	mockCtrl := gomock.NewController(t)

	mockOcmInterface := ocmMock.NewMockOCMInterface(mockCtrl)

	// So we can clean up at the end
	tempDefaultOCMInterface := ocm.DefaultOCMInterface

	ocm.DefaultOCMInterface = mockOcmInterface

	t.Run("it returns a cluster struct from a valid cluster key", func(_ *testing.T) {
		ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
		mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
		_ = os.Setenv(info.BackplaneURLEnvName, "https://backplane-url.cluster-key.redhat.com")
		mockOcmInterface.EXPECT().GetTargetCluster("cluster-key").Return("1234", "cluster-key", nil)

		cluster, err := utils.DefaultClusterUtils.GetBackplaneClusterFromClusterKey("cluster-key")

		expectedCluster := utils.BackplaneCluster{
			ClusterID:     "1234",
			BackplaneHost: "https://backplane-url.cluster-key.redhat.com",
			ClusterURL:    fmt.Sprintf("%s/backplane/cluster/%s", "https://backplane-url.cluster-key.redhat.com", "1234"),
		}

		if err != nil {
			t.Errorf("expected errorResp %v, got %v", nil, err)
		}

		if !reflect.DeepEqual(cluster, expectedCluster) {
			t.Errorf("expected clusters %v and %v to be equal", cluster, expectedCluster)
		}
	})

	ocm.DefaultOCMInterface = tempDefaultOCMInterface
}
