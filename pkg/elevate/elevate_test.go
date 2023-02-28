package elevate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestRunElevate(t *testing.T) {
	type args struct {
		e                 ElevateConfig
		argv              []string
		ExecCmd           func(name string, arg ...string) *exec.Cmd
		OsRemove          func(name string) error
		ReadKubeConfigRaw func() (api.Config, error)
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "It errors if we cannot load the kubeconfig",
			args: args{
				ExecCmd:  exec.Command,
				OsRemove: os.Remove,
				ReadKubeConfigRaw: func() (api.Config, error) {
					return *api.NewConfig(), errors.New("cannot load kfg")
				},
				e: ElevateConfig{

					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig:       AddElevationReasonToRawKubeconfig,
					WriteConfigToTempKubeconfig:             WriteConfigToTempKubeconfig,
				},
				argv: []string{},
			},
			wantErr: true,
		},
		{
			name: "It errors if kubeconfig has no current context",
			args: args{
				ExecCmd:  exec.Command,
				OsRemove: os.Remove,
				ReadKubeConfigRaw: func() (api.Config, error) {
					return *api.NewConfig(), nil
				},
				e: ElevateConfig{
					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig: func(config api.Config, elevationReason string) error {
						return errors.New("kfg has no current context")
					},
					WriteConfigToTempKubeconfig: WriteConfigToTempKubeconfig,
				},
				argv: []string{"oc", "get pods"},
			},
			wantErr: true,
		},
		{
			name: "It errors if we can't write the new kubeconfig to a temporary location",
			args: args{
				ExecCmd:  exec.Command,
				OsRemove: os.Remove,
				ReadKubeConfigRaw: func() (api.Config, error) {
					return *api.NewConfig(), nil
				},
				e: ElevateConfig{
					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig: func(config api.Config, elevationReason string) error {
						return nil
					},
					WriteConfigToTempKubeconfig: func(config api.Config) (string, error) {
						return "", errors.New("cannot write temporary kfg")
					},
				},
				argv: []string{"oc", "get pods"},
			},
			wantErr: true,
		},
		{
			name: "It errors if we can't add the new impersonation role to the current user",
			args: args{
				ExecCmd:  exec.Command,
				OsRemove: os.Remove,
				ReadKubeConfigRaw: func() (api.Config, error) {
					return *api.NewConfig(), nil
				},
				e: ElevateConfig{
					GetCurrentUsername: GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: func(e ElevateConfig, config api.Config) error {
						return errors.New("Cannot add role to current user")
					},
					AddElevationReasonToRawKubeconfig: func(config api.Config, elevationReason string) error {
						return nil
					},
					WriteConfigToTempKubeconfig: func(config api.Config) (string, error) {
						return "tmp/location", nil
					},
				},
				argv: []string{"oc", "get pods"},
			},
			wantErr: true,
		},
		{
			name: "It errors if the exec command errors",
			args: args{
				ExecCmd:  fakeExecCommandError,
				OsRemove: os.Remove,
				ReadKubeConfigRaw: func() (api.Config, error) {
					return *api.NewConfig(), nil
				},
				e: ElevateConfig{
					GetCurrentUsername: GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: func(e ElevateConfig, config api.Config) error {
						return nil
					},
					AddElevationReasonToRawKubeconfig: func(config api.Config, elevationReason string) error {
						return nil
					},
					WriteConfigToTempKubeconfig: func(config api.Config) (string, error) {
						return "tmp/location", nil
					},
				},
				argv: []string{"oc", "get pods"},
			},
			wantErr: true,
		},
		{
			name: "It errors if the command succeeds but we can't remove the tmp kubeconfig",
			args: args{
				ExecCmd:  fakeExecCommandSuccess,
				OsRemove: func(name string) error { return errors.New("cannot remove tmp kubeconfig") },
				ReadKubeConfigRaw: func() (api.Config, error) {
					return *api.NewConfig(), nil
				},
				e: ElevateConfig{
					GetCurrentUsername: GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: func(e ElevateConfig, config api.Config) error {
						return nil
					},
					AddElevationReasonToRawKubeconfig: func(config api.Config, elevationReason string) error {
						return nil
					},
					WriteConfigToTempKubeconfig: func(config api.Config) (string, error) {
						return "tmp/location", nil
					},
				},
				argv: []string{"oc", "get pods"},
			},
			wantErr: true,
		},
		{
			name: "It suceeds if the command succeeds and we can clean up the tmp kubeconfig",
			args: args{
				ExecCmd:  fakeExecCommandSuccess,
				OsRemove: func(name string) error { return nil },
				ReadKubeConfigRaw: func() (api.Config, error) {
					return *api.NewConfig(), nil
				},
				e: ElevateConfig{
					GetCurrentUsername: GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: func(e ElevateConfig, config api.Config) error {
						return nil
					},
					AddElevationReasonToRawKubeconfig: func(config api.Config, elevationReason string) error {
						return nil
					},
					WriteConfigToTempKubeconfig: func(config api.Config) (string, error) {
						return "tmp/location", nil
					},
				},
				argv: []string{"oc", "get pods"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ExecCmd = tt.args.ExecCmd
			OsRemove = tt.args.OsRemove
			ReadKubeConfigRaw = tt.args.ReadKubeConfigRaw
			if err := RunElevate(tt.args.e, tt.args.argv); (err != nil) != tt.wantErr {
				t.Errorf("RunElevate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetCurrentUsername(t *testing.T) {
	type args struct {
		ExecCmd func(name string, arg ...string) *exec.Cmd
		e       ElevateConfig
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "It errors if oc whoami gives an error",
			args: args{
				ExecCmd: fakeExecCommandError,
				e: ElevateConfig{
					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig:       AddElevationReasonToRawKubeconfig,
					WriteConfigToTempKubeconfig:             WriteConfigToTempKubeconfig,
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "It errors if username is empty",
			args: args{
				ExecCmd: fakeExecCommandSuccess,
				e: ElevateConfig{
					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig:       AddElevationReasonToRawKubeconfig,
					WriteConfigToTempKubeconfig:             WriteConfigToTempKubeconfig,
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "It is successful if it returns a username string from oc whoami",
			args: args{
				ExecCmd: fakeExecCommandSuccessUsernameReturn,
				e: ElevateConfig{
					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig:       AddElevationReasonToRawKubeconfig,
					WriteConfigToTempKubeconfig:             WriteConfigToTempKubeconfig,
				},
			},
			want:    "username",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ExecCmd = tt.args.ExecCmd
			got, err := GetCurrentUsername(tt.args.e)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCurrentUsername() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetCurrentUsername() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddImpersonationReasonRoleToCurrentUser(t *testing.T) {
	type args struct {
		e      ElevateConfig
		config api.Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "It returns an error if we cannot load the kubeconfig from file",
			args: args{
				e: ElevateConfig{
					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig:       AddElevationReasonToRawKubeconfig,
					WriteConfigToTempKubeconfig:             WriteConfigToTempKubeconfig,
				},
			},
			wantErr: true,
		},
		{
			name: "It returns an error if we cannot load the client set from the kubeconfig supplied",
			args: args{
				e: ElevateConfig{
					GetCurrentUsername:                      GetCurrentUsername,
					AddImpersonationReasonRoleToCurrentUser: AddImpersonationReasonRoleToCurrentUser,
					AddElevationReasonToRawKubeconfig:       AddElevationReasonToRawKubeconfig,
					WriteConfigToTempKubeconfig:             WriteConfigToTempKubeconfig,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddImpersonationReasonRoleToCurrentUser(tt.args.e, tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("AddImpersonationReasonRoleToCurrentUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddElevationReasonToRawKubeconfig(t *testing.T) {
	type args struct {
		config          api.Config
		elevationReason string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "It throws an error if there is no current kubeconfig context",
			args: args{
				config:          api.Config{},
				elevationReason: "Production cluster",
			},
			wantErr: true,
		},
		{
			name: "it returns an error if there is no user info in kubeconfig",
			args: args{
				config: api.Config{
					Kind:        "Config",
					APIVersion:  "v1",
					Preferences: api.Preferences{},
					Clusters: map[string]*api.Cluster{
						"dummy_cluster": {
							Server: "https://api-backplane.apps.something.com/backplane/cluster/configcluster",
						},
					},
					Contexts: map[string]*api.Context{
						"default/test123/anonymous": {
							Cluster:   "dummy_cluster",
							Namespace: "default",
						},
					},
					CurrentContext: "default/test123/anonymous",
				},
				elevationReason: "Production cluster",
			},
			wantErr: true,
		},
		{
			name: "it succeeds if the auth info exists for the current context",
			args: args{
				config: api.Config{
					Kind:        "Config",
					APIVersion:  "v1",
					Preferences: api.Preferences{},
					Clusters: map[string]*api.Cluster{
						"dummy_cluster": {
							Server: "https://api-backplane.apps.something.com/backplane/cluster/configcluster",
						},
					},
					AuthInfos: map[string]*api.AuthInfo{
						"anonymous": {
							LocationOfOrigin: "England",
						},
					},
					Contexts: map[string]*api.Context{
						"default/test123/anonymous": {
							Cluster:   "dummy_cluster",
							Namespace: "default",
							AuthInfo:  "anonymous",
						},
					},
					CurrentContext: "default/test123/anonymous",
				},
				elevationReason: "Production cluster",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddElevationReasonToRawKubeconfig(tt.args.config, tt.args.elevationReason); (err != nil) != tt.wantErr {
				t.Errorf("AddElevationReasonToRawKubeconfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteConfigToTempKubeconfig(t *testing.T) {
	type args struct {
		config api.Config
	}
	tests := []struct {
		name                  string
		args                  args
		want                  string
		createTemp            func(dir string, pattern string) (*os.File, error)
		writeKubeconfigToFile func(config api.Config, filename string) error
		wantErr               bool
	}{
		{
			name: "It returns an error and empty string if we cannot create a temporary file",
			args: args{
				api.Config{},
			},
			want: "",
			createTemp: func(dir string, pattern string) (*os.File, error) {
				return &os.File{}, errors.New("Could not create temp file")
			},
			writeKubeconfigToFile: clientcmd.WriteToFile,
			wantErr:               true,
		},
		{
			name: "It returns an error if we can't write to a temp kubeconfig",
			args: args{
				api.Config{},
			},
			want:       "",
			createTemp: os.CreateTemp,
			writeKubeconfigToFile: func(config api.Config, filename string) error {
				return errors.New("Could not write temp kubeconfig to file")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CreateTemp = tt.createTemp
			WriteKubeconfigToFile = tt.writeKubeconfigToFile
			got, err := WriteConfigToTempKubeconfig(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteConfigToTempKubeconfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("WriteConfigToTempKubeconfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func fakeExecCommandError(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessError", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func fakeExecCommandSuccess(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessSuccess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func fakeExecCommandSuccessUsernameReturn(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessSuccessUsernameReturn", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
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
	fmt.Fprintf(os.Stdout, "")
	os.Exit(0)
}

func TestHelperProcessSuccessUsernameReturn(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprintf(os.Stdout, "username")
	os.Exit(0)
}
