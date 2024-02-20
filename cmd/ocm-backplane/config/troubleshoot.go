package config

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
)

type troubleshootOptions struct {
}

func newTroubleshootOptions() *troubleshootOptions {
	return &troubleshootOptions{}
}

func newTroubleshootCmd() *cobra.Command {
	ops := newTroubleshootOptions()
	troubleshootCmd := &cobra.Command{
		Use:   "troubleshoot",
		Short: "Show the debug info of backplane",
		Long: `It prints the debug info for troubleshooting backplane issues.
`,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE:         ops.run,
	}
	return troubleshootCmd
}

var (
	// print info when the thing is correct
	printCorrect = func(format string, a ...any) {
		fmt.Printf("[V] "+format, a...)
	}
	// print info when the thing is wrong
	printWrong = func(format string, a ...any) {
		fmt.Printf("[X] "+format, a...)
	}
	// print info when the thing is not wrong but need attention
	printNotice = func(format string, a ...any) {
		fmt.Printf("[-] "+format, a...)
	}
	// normal printf
	printf = func(format string, a ...any) {
		fmt.Printf(format, a...)
	}
	// execute oc get proxy commands in OS
	execOCProxy = func() ([]byte, error) {
		return exec.Command("bash", "-c", "oc config view -o jsonpath='{.clusters[0].cluster.proxy-url}'").Output()
	}
)

var getBackplaneConfiguration = config.GetBackplaneConfiguration

// Print backplane-cli related info
func (o *troubleshootOptions) checkBPCli() error {
	configFilePath, err := config.GetConfigFilePath()
	if err != nil {
		printWrong("Failed to get backplane-cli configuration path: %v\n", err)
	} else {
		printCorrect("backplane-cli configuration path: %s\n", configFilePath)
	}
	currentBPConfig, err := getBackplaneConfiguration()
	if err != nil {
		printWrong("Failed to read backplane-cli config file: %v\n", err)
	} else {
		printCorrect("proxy in backplane-cli config: %s\n", *currentBPConfig.ProxyURL)
	}
	return nil
}

// Print OCM related info
func (o *troubleshootOptions) checkOCM() error {
	ocmEnv, err := ocm.DefaultOCMInterface.GetOCMEnvironment()
	if err != nil {
		printWrong("Failed to get OCM environment: %v\n", err)
	} else {
		printCorrect("backplane url from OCM: %s\n", ocmEnv.BackplaneURL())
	}
	return nil
}

// Print OC related info
func (o *troubleshootOptions) checkOC() error {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultPathOptions().GetDefaultFilename())
	if err != nil {
		printWrong("Failed to get OC configuration: %v\n", err)
	} else {
		printCorrect("oc server url: %s\n", cfg.Host)
	}
	// get the proxy url
	// we might refine it by client-go
	// https://github.com/openshift/oc/blob/master/vendor/k8s.io/kubectl/pkg/cmd/config/view.go
	hasProxy := false
	proxyURL := ""
	getOCProxyOutput, err := execOCProxy()
	if err != nil {
		printWrong("Failed to get proxy in OC configuration: %v\n", err)
	} else {
		proxyURL = strings.TrimSpace(string(getOCProxyOutput))
		if len(proxyURL) > 0 {
			hasProxy = true
			printCorrect("proxy in OC configuration: %s\n", proxyURL)
		} else {
			printNotice("no proxy in OC configuration")
		}
	}
	// Verify network - proxy connectivity
	if hasProxy {
		printf("To verify proxy connectivity, run:\n HTTPS_PROXY=%s curl -Iv https://www.redhat.com \n", proxyURL)
	}
	return nil
}

func (o *troubleshootOptions) run(cmd *cobra.Command, argv []string) error {
	err := o.checkBPCli()
	if err != nil {
		return err
	}
	err = o.checkOCM()
	if err != nil {
		return err
	}
	err = o.checkOC()
	if err != nil {
		return err
	}
	return nil
}
