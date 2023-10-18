package elevate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	logger "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/pkg/utils"
)

var (
	OsRemove              = os.Remove
	ExecCmd               = exec.Command
	ReadKubeConfigRaw     = utils.ReadKubeconfigRaw
	WriteKubeconfigToFile = utils.CreateTempKubeConfig
)

func AddElevationReasonToRawKubeconfig(config api.Config, elevationReason string) error {
	logger.Debugln("Adding reason for backplane-cluster-admin elevation")
	if config.Contexts[config.CurrentContext] == nil {
		return errors.New("no current kubeconfig context")
	}

	currentCtxUsername := config.Contexts[config.CurrentContext].AuthInfo

	if config.AuthInfos[currentCtxUsername] == nil {
		return errors.New("no current user information")
	}

	if config.AuthInfos[currentCtxUsername].ImpersonateUserExtra == nil {
		config.AuthInfos[currentCtxUsername].ImpersonateUserExtra = make(map[string][]string)
	}

	config.AuthInfos[currentCtxUsername].ImpersonateUserExtra["reason"] = []string{elevationReason}
	config.AuthInfos[currentCtxUsername].Impersonate = "backplane-cluster-admin"

	return nil
}

func RunElevate(argv []string) error {
	logger.Debugln("Finding target cluster from kubeconfig")
	config, err := ReadKubeConfigRaw()

	if err != nil {
		return err
	}

	err = AddElevationReasonToRawKubeconfig(config, argv[0])
	if err != nil {
		return err
	}

	err = WriteKubeconfigToFile(&config)
	if err != nil {
		return err
	}

	logger.Debug("Adding impersonation RBAC allow permissions to kubeconfig")

	elevateCmd := argv[1:]

	logger.Debugln("Executing command with temporary kubeconfig as backplane-cluster-admin")
	ocCmd := ExecCmd("oc", elevateCmd...)

	ocCmd.Env = append(ocCmd.Env, os.Environ()...)
	ocCmd.Stderr = os.Stderr
	ocCmd.Stdout = os.Stdout

	if err != nil {
		return err
	}

	err = ocCmd.Run()
	kubeconfigPath, _ := os.LookupEnv("KUBECONFIG")
	defer func() {
		logger.Debugln("Command error; Cleaning up temporary kubeconfig")
		err := OsRemove(kubeconfigPath)
		if err != nil {
			fmt.Println(err)
		}
	}()
	if err != nil {
		return err
	}

	return nil
}
