package utils

import (
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	loggedInYamlSingle = `
apiVersion: v1
clusters:
- cluster:
    server: https://api-backplane.apps.hivep01ue1.b6s7.p1.openshiftapps.com/backplane/cluster/1f0o1maej9brj6j9k6ehbe7rm0k2lng7/
  name: hivep02ue1
contexts:
- context:
    cluster: hivep02ue1
    namespace: default
    user: example.openshift
  name: default/hivep02ue1/example.openshift
current-context: default/hivep02ue1/example.openshift
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
	dirname := filepath.Dir(kubeconfigPath)
	err := os.MkdirAll(dirname, 0644)
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
			ClusterURL:    "https://api-backplane.apps.hivep01ue1.b6s7.p1.openshiftapps.com/backplane/cluster/1f0o1maej9brj6j9k6ehbe7rm0k2lng7/",
			BackplaneHost: "api-backplane.apps.hivep01ue1.b6s7.p1.openshiftapps.com",
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
