package troubleshoot

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

func NewTroubleshootCmd() *cobra.Command {
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

func (o *troubleshootOptions) run(cmd *cobra.Command, argv []string) error {
	// Print backplane-cli related info
	configFilePath, err := config.GetConfigFilePath()
	if err != nil {
		fmt.Printf("[X] Failed to get backplane-cli configuration path: %v\n", err)
	} else {
		fmt.Printf("[V] backplane-cli configuration path: %s\n", configFilePath)
	}
	currentBPConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		fmt.Printf("[X] Failed to read backplane-cli config file: %v\n", err)
	} else {
		fmt.Printf("[V] proxy in backplane-cli config: %s\n", *currentBPConfig.ProxyURL)
	}
	// Print OCM related info
	ocmEnv, err := ocm.DefaultOCMInterface.GetOCMEnvironment()
	if err != nil {
		fmt.Printf("[X] Failed to get OCM environment: %v\n", err)
	} else {
		fmt.Printf("[V] backplane url from OCM: %s\n", ocmEnv.BackplaneURL())
	}
	// Print OC related info
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultPathOptions().GetDefaultFilename())
	if err != nil {
		fmt.Printf("[X] Failed to get OC configuration: %v\n", err)
	} else {
		fmt.Printf("[V] oc server url: %s\n", cfg.Host)
	}
	// get the proxy url
	// we might refine it by client-go
	// https://github.com/openshift/oc/blob/master/vendor/k8s.io/kubectl/pkg/cmd/config/view.go
	hasProxy := false
	proxyURL := ""
	getOCProxyCmd := "oc config view -o jsonpath='{.clusters[0].cluster.proxy-url}'"
	getOCProxyOutput, err := exec.Command("bash", "-c", getOCProxyCmd).Output()
	if err != nil {
		fmt.Printf("[X] Failed to get proxy in OC configuration: %v\n", err)
	} else {
		proxyURL = strings.TrimSpace(string(getOCProxyOutput))
		if len(proxyURL) > 0 {
			hasProxy = true
			fmt.Printf("[V] proxy in OC configuration: %s\n", proxyURL)
		} else {
			fmt.Println("[-] no proxy in OC configuration")
		}
	}
	// Verify network - proxy connectivity
	if hasProxy {
		fmt.Printf("To verify proxy connectivity, run:\n HTTPS_PROXY=%s curl -Iv https://www.redhat.com \n", proxyURL)
	}
	// Verify backplane-api connectivity
	if len(ocmEnv.BackplaneURL()) > 0 {
		if hasProxy {
			fmt.Printf("To verify backplane-api connectivity, run:\n HTTPS_PROXY=%s curl -Iv %s \n", proxyURL, ocmEnv.BackplaneURL())
		} else {
			fmt.Printf("To verify backplane-api connectivity, run:\n curl -Iv %s \n", ocmEnv.BackplaneURL())
		}
	}
	return nil
}
