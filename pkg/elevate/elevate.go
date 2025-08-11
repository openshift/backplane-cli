package elevate

import (
	"fmt"
	"os"
	"os/exec"

	logger "github.com/sirupsen/logrus"

	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var (
	OsRemove              = os.Remove
	ExecCmd               = exec.Command
	ReadKubeConfigRaw     = utils.ReadKubeconfigRaw
	WriteKubeconfigToFile = utils.CreateTempKubeConfig
)

// RunElevate executes the elevation process for backplane access.
// It reads the current kubeconfig, adds elevation context with the provided reason,
// and optionally executes a command with elevated permissions.
// The first argument is the elevation reason, remaining arguments are the command to execute.
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
	elevationReasons, err := login.SaveElevateContextReasons(config, elevateReason)
	if err != nil {
		return err
	}

	// If no command are provided, then we just initiate elevate context
	if len(argv) < 2 {
		return nil
	}

	logger.Debug("Adding impersonation RBAC allow permissions to kubeconfig")
	err = login.AddElevationReasonsToRawKubeconfig(config, elevationReasons)
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

	// As WriteKubeconfigToFile is also creating a temporary file referenced by new KUBECONFIG variable setting,
	// we need to take care of it's cleanup
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
