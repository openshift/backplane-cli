package script

import (
	"io"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	kubeYaml = `
apiVersion: v1
clusters:
- cluster:
    server: https://api-backplane.apps.something.com/backplane/cluster/configcluster
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
)

func TestScriptCmdSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManagedJob Test Suite")
}

func MakeIoReader(s string) io.ReadCloser {
	r := io.NopCloser(strings.NewReader(s)) // r type is io.ReadCloser
	return r
}
