package logout

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	loggedInYamlSingle = `
apiVersion: v1
clusters:
- cluster:
    server: https://api-backplane.apps.01ue1.b6s7.p1.openshiftapps.com/backplane/cluster/1f0o1maej9brj6j9k6ehbe7rm0k2lng7/
  name: dummy_cluster
contexts:
- context:
    cluster: 2ue1
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

type VerifyFunc func(api.Config, api.Config, *testing.T, []error)

func defaultTestFunc(t *testing.T) []error {
	err := runLogout(nil, make([]string, 0))
	return []error{err}
}

var tests = []struct {
	name       string
	yamlFile   string
	testFunc   func(*testing.T) []error
	verifyFunc VerifyFunc
}{
	{
		name:     "Test logout do not alter non backplane login",
		yamlFile: loggedInNotBackplane,
		testFunc: defaultTestFunc,
		verifyFunc: func(initial api.Config, after api.Config, t *testing.T, errs []error) {
			if !reflect.DeepEqual(after, initial) {
				t.Error("logout cmd altered non-backplane clusters")
			}
			if len(errs) != 1 && errs[0].Error() != "you're not logged in using backplane, skipping" {
				t.Errorf("Unexpected errors: %v", errs)
			}
		},
	},
	{
		name:     "Test logout delete backplane logins context",
		yamlFile: loggedInYamlSingle,
		testFunc: defaultTestFunc,
		verifyFunc: func(initial api.Config, after api.Config, t *testing.T, errs []error) {
			if len(after.Contexts) != 0 {
				t.Error("Resulting context length unexpected")
			}

			if after.CurrentContext != "" {
				t.Error("current-context unexpected")
			}

			if len(after.AuthInfos) != 2 {
				t.Errorf("User info is wrong")
			}

			if after.AuthInfos["example.openshift"] != nil {
				t.Errorf("User not deleted")
			}

			if len(errs) != 1 && errs[0] != nil {
				t.Errorf("Unexpected errors: %v", errs)
			}
		},
	},
	{
		name:     "Test logout twice",
		yamlFile: loggedInYamlSingle,
		testFunc: func(t *testing.T) []error {
			errors := make([]error, 0)
			errors = append(errors, defaultTestFunc(t)...)
			errors = append(errors, defaultTestFunc(t)...)
			return errors
		},
		verifyFunc: func(initial api.Config, after api.Config, t *testing.T, errs []error) {
			if len(after.Contexts) != 0 {
				t.Error("Resulting context length unexpected")
			}

			if after.CurrentContext != "" {
				t.Error("current-context unexpected")
			}

			if len(after.AuthInfos) != 2 {
				t.Errorf("User info is wrong")
			}

			if after.AuthInfos["example.openshift"] != nil {
				t.Errorf("User not deleted")
			}

			if len(errs) != 2 {
				t.Errorf("Unexpected errors: %v", errs)
			}

			if errs[0] != nil {
				t.Errorf("Unexpected error %v", errs[0].Error())
			}

			if errs[1].Error() != "current context does not exist, skipping" {
				t.Errorf("Unexpected errors: %v", errs)
			}
		},
	},
	{
		name:     "Test logout empty kubeconfig yaml",
		yamlFile: "",
		testFunc: defaultTestFunc,
		verifyFunc: func(initial api.Config, after api.Config, t *testing.T, errs []error) {
			if len(after.Contexts) != 0 {
				t.Error("Resulting context length unexpected")
			}

			if after.CurrentContext != "" {
				t.Error("current-context unexpected")
			}

			if len(after.AuthInfos) != 0 {
				t.Errorf("User info is wrong")
			}

			if len(errs) != 1 && errs[0] != nil {
				t.Errorf("Unexpected error %v", errs[0].Error())
			}
		},
	},
	{
		name:     "Test logout invalid kubeconfig yaml",
		yamlFile: invalidYaml,
		testFunc: defaultTestFunc,
		verifyFunc: func(initial api.Config, after api.Config, t *testing.T, errs []error) {
			if len(after.Contexts) != 0 {
				t.Error("Resulting context length unexpected")
			}

			if after.CurrentContext != "" {
				t.Error("current-context unexpected")
			}

			if len(after.AuthInfos) != 0 {
				t.Errorf("User info is wrong")
			}

			if len(errs) != 1 && errs[0] != nil {
				t.Errorf("Unexpected error %v", errs[0].Error())
			}
		},
	},
}

func writeKubeconfigYaml(s string) error {
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GlobalFile
	dirname := filepath.Dir(kubeconfigPath)
	err := os.MkdirAll(dirname, os.ModePerm)
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

func TestLogoutCmd(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeKubeconfigYaml(tt.yamlFile)
			if err != nil {
				t.Error(err)
				return
			}

			initial, err := utils.ReadKubeconfigRaw()
			if err != nil {
				t.Error(err)
				return
			}

			errs := tt.testFunc(t)

			after, err := utils.ReadKubeconfigRaw()
			if err != nil {
				t.Error(err)
				return
			}

			tt.verifyFunc(initial, after, t, errs)
		},
		)
	}
}
