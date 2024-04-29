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
	return AddElevationReasonsToRawKubeconfig(config, []string{elevationReason})
}

func AddElevationReasonsToRawKubeconfig(config api.Config, elevationReasons []string) error {
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

	config.AuthInfos[currentCtxUsername].ImpersonateUserExtra["reason"] = elevationReasons
	config.AuthInfos[currentCtxUsername].Impersonate = "backplane-cluster-admin"

	return nil
}

func RunElevate(argv []string) error {
	logger.Debugln("Finding target cluster from kubeconfig")
	config, err := ReadKubeConfigRaw()
	if err != nil {
		return err
	}

	logger.Debug("Compute and store reason from/to kubeconfig ElevateContext")
	var elevateReason string
	if len(argv) == 0 {
		elevateReason = ""
	} else {
		elevateReason = argv[0]
	}
	elevationReasons, err := ComputeElevateContextAndStoreToKubeConfigFileAndGetReasons(config, elevateReason)
	if err != nil {
		return err
	}

	// If no command are provided, then we just initiate elevate context
	if len(argv) < 2 {
		return nil
	}

	logger.Debug("Adding impersonation RBAC allow permissions to kubeconfig")
	err = AddElevationReasonsToRawKubeconfig(config, elevationReasons)
	if err != nil {
		return err
	}

	// As WriteKubeconfigToFile(utils.CreateTempKubeConfig) is overriding KUBECONFIG,
	// we need to store its definition in order to redefine it to its original (value or unset) when we do not need it anymore
	oldKubeconfigPath, oldKubeconfigDefined := os.LookupEnv("KUBECONFIG")
	defer func() {
		if oldKubeconfigDefined {
			logger.Debugln("Will set KUBECONFIG variable to original", oldKubeconfigPath)
			os.Setenv("KUBECONFIG", oldKubeconfigPath)
		} else {
			logger.Debugln("Will unset KUBECONFIG variable")
			os.Unsetenv("KUBECONFIG")
		}
	}()

	err = WriteKubeconfigToFile(&config)
	if err != nil {
		return err
	}

	// As WriteKubeconfigToFile is also creating a tempory file reference by new KUBECONFIG variable setting,
	// we need to take care of its cleanup
	tempKubeconfigPath, _ := os.LookupEnv("KUBECONFIG")
	defer func() {
		logger.Debugln("Cleaning up temporary kubeconfig", tempKubeconfigPath)
		err := OsRemove(tempKubeconfigPath)
		if err != nil {
			fmt.Println(err)
		}
	}()

	logger.Debugln("Executing command with temporary kubeconfig as backplane-cluster-admin")
	ocCmd := ExecCmd("oc", argv[1:]...)
	ocCmd.Env = append(ocCmd.Env, os.Environ()...)
	ocCmd.Stdin = os.Stdin
	ocCmd.Stderr = os.Stderr
	ocCmd.Stdout = os.Stdout
	err = ocCmd.Run()
	if err != nil {
		return err
	}

	return nil
}
