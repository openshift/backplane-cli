package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/openshift/backplane-cli/pkg/utils/mocks"
	"k8s.io/client-go/tools/clientcmd"
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

func writeKubeconfigYaml(s string) error {
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GlobalFile
	clientcmd.ModifyConfig()
	dirname := filepath.Dir(kubeconfigPath)
	err := os.MkdirAll(dirname, 0744)
	if err != nil {
		return err
	}
	f, err := os.Create(kubeconfigPath)
	if err != nil {
		return err
	}
	_, err = f.WriteString(s)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	return nil
}

func TestGetBackplaneClusterFromConfig(t *testing.T) {
	tests := []struct {
		config string
		expect BackplaneCluster
	}{{
		config: loggedInYamlSingle,
		expect: BackplaneCluster{
			ClusterID:     "1f0o1maej9brj6j9k6ehbe7rm0k2lng7",
			ClusterURL:    "https://api-backplane.apps.com/backplane/cluster/1f0o1maej9brj6j9k6ehbe7rm0k2lng7/",
			BackplaneHost: "api-backplane.apps.com",
		},
	}}

	for n, tt := range tests {
		_ = writeKubeconfigYaml(tt.config)
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			result, err := GetBackplaneClusterFromConfig()
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
		_ = writeKubeconfigYaml(tt.config)
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			_, err := GetBackplaneClusterFromConfig()
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
			o0, o1, err := GetClusterIDAndHostFromClusterURL(tt.inp)
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
			_, _, err := GetClusterIDAndHostFromClusterURL(tt.inp)
			if err == nil {
				t.Errorf("expecting error for %s", tt.inp)
			}

		})
	}
}

func TestGetBackplaneClusterFromClusterKey(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	mockOcmInterface := mocks.NewMockOCMInterface(mockCtrl)

	// So we can clean up at the end
	tempDefaultOCMInterface := DefaultOCMInterface

	DefaultOCMInterface = mockOcmInterface

	t.Run("it errors if BACKPLANE_URL_ENV_NAME is empty", func(_ *testing.T) {
		backplaneConfigPath := "~/.backplane.json"
		errorResp := fmt.Errorf("failed to read file %s : %v", backplaneConfigPath, errors.New("File not found"))
		mockOcmInterface.EXPECT().GetBackplaneURL().Return("", errorResp)
		mockOcmInterface.EXPECT().GetTargetCluster("cluster-key").Return("1234", "cluster-key", nil)

		_, err := GetBackplaneClusterFromClusterKey("cluster-key")
		if err != errorResp {
			t.Errorf("expected errorResp %v, got %v", errorResp, err)
		}
	})

	t.Run("it errors if BACKPLANE_CONFIG_FILE_PATH cannot be decoded", func(_ *testing.T) {
		backplaneConfigPath := "~/.backplane.json"
		errorResp := fmt.Errorf("failed to decode file %s : %v", backplaneConfigPath, errors.New("File could not be decoded"))
		mockOcmInterface.EXPECT().GetBackplaneURL().Return("", errorResp)
		mockOcmInterface.EXPECT().GetTargetCluster("cluster-key").Return("1234", "cluster-key", nil)

		_, err := GetBackplaneClusterFromClusterKey("cluster-key")
		if err != errorResp {
			t.Errorf("expected errorResp %v, got %v", errorResp, err)
		}
	})

	t.Run("it errors if BACKPLANE_URL_ENV_NAME is empty", func(_ *testing.T) {
		errorResp := fmt.Errorf("%s env variable is empty", "BACKPLANE_URL")
		mockOcmInterface.EXPECT().GetBackplaneURL().Return("", errorResp)
		mockOcmInterface.EXPECT().GetTargetCluster("cluster-key").Return("1234", "cluster-key", nil)

		_, err := GetBackplaneClusterFromClusterKey("cluster-key")
		if err != errorResp {
			t.Errorf("expected errorResp %v, got %v", errorResp, err)
		}
	})

	t.Run("it returns a cluster struct from a valid cluster key", func(_ *testing.T) {
		mockOcmInterface.EXPECT().GetBackplaneURL().Return("https://backplane-url.cluster-key.redhat.com", nil)
		mockOcmInterface.EXPECT().GetTargetCluster("cluster-key").Return("1234", "cluster-key", nil)

		cluster, err := GetBackplaneClusterFromClusterKey("cluster-key")

		expectedCluster := BackplaneCluster{
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

	DefaultOCMInterface = tempDefaultOCMInterface
}
