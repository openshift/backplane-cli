package logout

import (
	"reflect"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
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

func TestLogoutCmd(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			config, _ := clientcmd.Load([]byte(tt.yamlFile))
			err := utils.CreateTempKubeConfig(config)

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
