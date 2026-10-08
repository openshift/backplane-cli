package elevate

import (
	"errors"
	"strings"
	"testing"

	pkgelevate "github.com/openshift/backplane-cli/pkg/elevate"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestRunElevateReason(t *testing.T) {
	readConfig := pkgelevate.ReadKubeConfigRaw
	defer func() { pkgelevate.ReadKubeConfigRaw = readConfig }()
	configError := errors.New("stopped before reading config")
	pkgelevate.ReadKubeConfigRaw = func() (api.Config, error) { return api.Config{}, configError }
	defer func() { noReason = false }()

	for _, tc := range []struct {
		name       string
		args       []string
		wantReason bool
	}{
		{"missing reason with oc", []string{"--", "oc", "get", "pod"}, true},
		{"missing reason without oc", []string{"--", "get", "pod"}, true},
		{"separator alone", []string{"--"}, true},
		{"explicit reason", []string{"OHSS-123", "--", "get", "pod"}, false},
		{"no-reason flag", []string{"-n", "--", "get", "pod"}, false},
		{"no-reason without separator", []string{"-n", "get", "pod"}, false},
		{"empty reason", []string{"", "--", "get", "pod"}, false},
		{"legacy command", []string{"OHSS-123", "get", "--", "pods"}, false},
		{"no arguments", []string{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			noReason = false
			cmd := &cobra.Command{Use: "elevate", RunE: runElevate}
			cmd.Flags().BoolVarP(&noReason, "no-reason", "n", false, "")
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.wantReason {
				if err == nil || !strings.Contains(err.Error(), "elevation reason required") {
					t.Fatalf("expected missing reason error, got %v", err)
				}
			} else if !errors.Is(err, configError) {
				t.Fatalf("expected to reach config loading, got %v", err)
			}
		})
	}
}
