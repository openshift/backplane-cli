package elevate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/openshift/backplane-cli/pkg/login"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
)

func fakeExecCommandError(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessError", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...) //#nosec: G204
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func fakeExecCommandSuccess(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessSuccess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...) //#nosec: G204
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

var fakeAPIConfig = api.Config{
	Kind:        "Config",
	APIVersion:  "v1",
	Preferences: api.Preferences{},
	Clusters: map[string]*api.Cluster{
		"dummy_cluster": {
			Server: "https://api-backplane.apps.something.com/backplane/cluster/configcluster",
		},
	},
	AuthInfos: map[string]*api.AuthInfo{
		"anonymous": {},
	},
	Contexts: map[string]*api.Context{
		"default/test123/anonymous": {
			Cluster:   "dummy_cluster",
			Namespace: "default",
			AuthInfo:  "anonymous",
		},
	},
	CurrentContext: "default/test123/anonymous",
}

const (
	elevateExtensionName             = "ElevateContext"
	elevateExtensionRetentionMinutes = 20
)

func fakeReadKubeConfigRaw() (api.Config, error) {
	return *fakeAPIConfig.DeepCopy(), nil
}

func fakeReadKubeConfigRawWithReasons(lastUsedMinutes time.Duration) func() (api.Config, error) {
	return func() (api.Config, error) {
		config := *fakeAPIConfig.DeepCopy()
		config.Contexts[config.CurrentContext].Extensions = map[string]runtime.Object{
			elevateExtensionName: &login.ElevateContext{
				Reasons:  []string{"dymmy reason"},
				LastUsed: time.Now().Add(-lastUsedMinutes * time.Minute),
			},
		}
		return config, nil
	}
}

func TestHelperProcessError(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(1)
}

func TestHelperProcessSuccess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	_, _ = fmt.Fprintf(os.Stdout, "")
	os.Exit(0)
}

func TestAddElevationReasonToRawKubeconfig(t *testing.T) {
	fakeAPIConfigNoUser := *fakeAPIConfig.DeepCopy()
	delete(fakeAPIConfigNoUser.AuthInfos, "anonymous")
	fakeAPIConfigNoUser.Contexts["default/test123/anonymous"].AuthInfo = ""

	t.Run("It returns an error if there is no current kubeconfig context", func(t *testing.T) {
		if err := login.AddElevationReasonsToRawKubeconfig(api.Config{}, []string{"Production cluster"}); err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("it returns an error if there is no user info in kubeconfig", func(t *testing.T) {
		if err := login.AddElevationReasonsToRawKubeconfig(fakeAPIConfigNoUser, []string{"Production cluster"}); err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("it succeeds if the auth info exists for the current context", func(t *testing.T) {
		if err := login.AddElevationReasonsToRawKubeconfig(fakeAPIConfig, []string{"Production cluster"}); err != nil {
			t.Error("Expected no errors, got", err)
		}
	})
}

func TestRunElevate(t *testing.T) {
	// TODO: Utilize a fake filesystem for tests and stop relying on local system state
	tmpDir, err := os.MkdirTemp("", "backplane-cli-tests")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// We utilize the fact that elevate honors KUBECONFIG in order to not clobber the users real ~/.kube/config
	kubeconfigPath := tmpDir + "config"
	_ = os.Setenv("KUBECONFIG", kubeconfigPath)

	t.Run("It returns an error if we cannot load the kubeconfig", func(t *testing.T) {
		ExecCmd = exec.Command
		ReadKubeConfigRaw = func() (api.Config, error) {
			return *api.NewConfig(), errors.New("cannot load kfg")
		}
		if err := RunElevate([]string{}); err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("It returns an error if kubeconfig has no current context", func(t *testing.T) {
		ExecCmd = exec.Command
		ReadKubeConfigRaw = func() (api.Config, error) {
			return *api.NewConfig(), nil
		}
		if err := RunElevate([]string{"reason", "get", "pods"}); err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("It returns an error if the exec command has errors", func(t *testing.T) {
		ExecCmd = fakeExecCommandError
		ReadKubeConfigRaw = fakeReadKubeConfigRaw
		if err := RunElevate([]string{"reason", "get", "pods"}); err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("It returns an error if reason is empty and no ElevateContext", func(t *testing.T) {
		ExecCmd = fakeExecCommandSuccess
		ReadKubeConfigRaw = fakeReadKubeConfigRaw
		if err := RunElevate([]string{"", "get", "pods"}); err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("It returns an error if reason is empty and ElevateContext present with Reasons to old", func(t *testing.T) {
		ExecCmd = fakeExecCommandSuccess
		ReadKubeConfigRaw = fakeReadKubeConfigRawWithReasons(elevateExtensionRetentionMinutes + 1)
		if err := RunElevate([]string{"", "get", "pods"}); err == nil {
			t.Error("Expected err, got nil")
		}
	})
}
