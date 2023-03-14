package elevate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/openshift/backplane-cli/pkg/utils"
	logger "github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const ElevateBackplaneReasonClusterRole string = "elevate-backplane-cluster-admin-reason"
const ImpersonationUser string = "backplane-cluster-admin"

var CreateTemp = os.CreateTemp
var OsRemove = os.Remove
var ExecCmd = exec.Command
var NewClientset = kubernetes.NewForConfig
var BuildConfigFromFlags = clientcmd.BuildConfigFromFlags
var ReadKubeConfigRaw = utils.ReadKubeconfigRaw
var WriteKubeconfigToFile = clientcmd.WriteToFile

type ElevateConfig struct {
	GetCurrentUsername                      func(e ElevateConfig) (string, error)
	AddImpersonationReasonRoleToCurrentUser func(e ElevateConfig, config api.Config) error
	AddElevationReasonToRawKubeconfig       func(config api.Config, elevationReason string) error
	WriteConfigToTempKubeconfig             func(config api.Config) (string, error)
}

func GetCurrentUsername(e ElevateConfig) (string, error) {
	cmd := ExecCmd("oc", "whoami")
	buf := bytes.NewBuffer([]byte{})
	cmd.Stdout = buf

	err := cmd.Run()
	if err != nil {
		return "", err
	}
	username := strings.TrimSpace(buf.String())
	if username == "" {
		return "", errors.New("Username cannot be empty")
	}

	return username, nil
}

func AddImpersonationReasonRoleToCurrentUser(e ElevateConfig, config api.Config) error {
	clientCfg, err := BuildConfigFromFlags("", clientcmd.NewDefaultPathOptions().GetDefaultFilename())
	if err != nil {
		return err
	}
	clientCfg.Impersonate.UserName = ImpersonationUser

	clientSet, err := NewClientset(clientCfg)
	if err != nil {
		return err
	}
	ctx := context.TODO()

	err = clientSet.RbacV1().ClusterRoles().Delete(ctx, ElevateBackplaneReasonClusterRole, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	err = clientSet.RbacV1().ClusterRoleBindings().Delete(ctx, ElevateBackplaneReasonClusterRole, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	role, err := clientSet.RbacV1().ClusterRoles().Create(
		ctx,
		&v1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ElevateBackplaneReasonClusterRole,
			},
			Rules: []v1.PolicyRule{
				{
					Verbs:     []string{"impersonate"},
					APIGroups: []string{"authentication.k8s.io"},
					Resources: []string{"userextras/reason"},
				},
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return err
	}

	username, err := e.GetCurrentUsername(e)
	if err != nil {
		return err
	}

	_, err = clientSet.RbacV1().ClusterRoleBindings().Create(
		ctx,
		&v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: ElevateBackplaneReasonClusterRole,
			},
			RoleRef: v1.RoleRef{
				APIGroup: clientSet.RbacV1().RESTClient().APIVersion().Group,
				Kind:     "ClusterRole",
				Name:     role.Name,
			},
			Subjects: []v1.Subject{
				{
					Kind:     "User",
					APIGroup: clientSet.RbacV1().RESTClient().APIVersion().Group,
					Name:     username,
				},
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return err
	}

	return nil
}

func AddElevationReasonToRawKubeconfig(config api.Config, elevationReason string) error {
	logger.Debugln("Adding reason for backplane-cluster-admin elevation")
	if config.Contexts[config.CurrentContext] == nil {
		return errors.New("No current kubeconfig context")
	}

	currentCtxUsername := config.Contexts[config.CurrentContext].AuthInfo

	if config.AuthInfos[currentCtxUsername] == nil {
		return errors.New("No current user information")
	}

	if config.AuthInfos[currentCtxUsername].ImpersonateUserExtra == nil {
		config.AuthInfos[currentCtxUsername].ImpersonateUserExtra = make(map[string][]string)
	}

	config.AuthInfos[currentCtxUsername].ImpersonateUserExtra["reason"] = []string{elevationReason}
	config.AuthInfos[currentCtxUsername].Impersonate = "backplane-cluster-admin"

	return nil
}

func WriteConfigToTempKubeconfig(config api.Config) (string, error) {
	logger.Debugln("Writing to temporary kubeconfig")
	tmpConfig, err := CreateTemp("", "")

	if err != nil {
		return "", err
	}

	if err := WriteKubeconfigToFile(config, tmpConfig.Name()); err != nil {
		tmpConfig.Close()
		return "", err
	}

	tmpConfig.Close()

	return tmpConfig.Name(), nil
}

func RunElevate(e ElevateConfig, argv []string) error {
	logger.Debugln("Finding target cluster from kubeconfig")
	config, err := ReadKubeConfigRaw()

	if err != nil {
		return err
	}

	err = e.AddElevationReasonToRawKubeconfig(config, argv[0])
	if err != nil {
		return err
	}

	tmpConfigPath, err := e.WriteConfigToTempKubeconfig(config)
	if err != nil {
		return err
	}

	elevateCmd := argv[1:]

	logger.Debugln("Executing command with temporary kubeconfig as backplane-cluster-admin")
	ocCmd := ExecCmd("oc", elevateCmd...)

	tmpKubeEnvPath := fmt.Sprintf("KUBECONFIG=%s", tmpConfigPath)

	ocCmd.Env = append(ocCmd.Env, os.Environ()...)
	ocCmd.Env = append(ocCmd.Env, tmpKubeEnvPath)
	ocCmd.Stderr = os.Stderr
	ocCmd.Stdout = os.Stdout

	logger.Debug("Adding impersonation RBAC allow permissions to kubeconfig")
	err = e.AddImpersonationReasonRoleToCurrentUser(e, config)
	if err != nil {
		return err
	}

	err = ocCmd.Run()
	if err != nil {
		logger.Debugln("Command error; Cleaning up temporary kubeconfig")
		if fileErr := OsRemove(tmpConfigPath); fileErr != nil {
			return fmt.Errorf("%v \n %v", err, fileErr)
		}

		return err
	}

	logger.Debugln("Cleaning up temporary kubeconfig")
	if err := OsRemove(tmpConfigPath); err != nil {
		return err
	}

	return nil
}
