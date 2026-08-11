package testjob

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	backplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	backplaneJobsNamespace          = "openshift-backplane-managed-scripts"
	generateNamePrefixForTestScript = "openshift-job-dev-"
	defaultBaseImage                = "quay.io/redhat-user-workloads/rosa-tenant/managed-scripts:latest"
)

func newRenderTestJobCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render Kubernetes YAML for a test script (client-side, no API call)",
		Long: `
Render the Kubernetes objects needed to run a managed script on a cluster.

This command generates YAML for ServiceAccount, RBAC, and Pod resources
that you can apply directly with 'oc apply -f' on a cluster where you
have cluster-admin access.

No backplane API call is made — everything is generated locally.

Example usage:
  cd scripts/SREP/example
  ocm backplane testjob render -p VAR1=val1 > test-job.yaml
  oc apply -f test-job.yaml

To clean up after testing:
  oc delete -f test-job.yaml
`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runRenderTestJob,
	}

	cmd.Flags().StringArrayP(
		"params",
		"p",
		[]string{},
		"Params to be passed to the script. Example: -p 'VAR1=VAL1' -p VAR2=VAL2",
	)

	cmd.Flags().StringP(
		"source-dir",
		"s",
		"",
		"Optional source dir for the script (defaults to current directory)",
	)

	cmd.Flags().StringP(
		"base-image-override",
		"i",
		"",
		"Custom container image to use instead of the default managed-scripts image",
	)

	cmd.Flags().StringP(
		"output",
		"o",
		"",
		"Write output to file instead of stdout",
	)

	return cmd
}

func runRenderTestJob(cmd *cobra.Command, args []string) error {
	arr, err := cmd.Flags().GetStringArray("params")
	if err != nil {
		return err
	}

	parsedParams, err := utils.ParseParamsFlag(arr)
	if err != nil {
		return err
	}

	sourceDirFlag, err := cmd.Flags().GetString("source-dir")
	if err != nil {
		return err
	}

	baseImageOverride, err := cmd.Flags().GetString("base-image-override")
	if err != nil {
		return err
	}

	outputFile, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	sourceDir := "./"
	if sourceDirFlag != "" {
		sourceDir = sourceDirFlag + "/"
	}

	metadata, scriptBody, err := readScriptFromFiles(sourceDir)
	if err != nil {
		return err
	}

	if err := validateParams(metadata, parsedParams); err != nil {
		return err
	}

	baseImage := defaultBaseImage
	if baseImageOverride != "" {
		baseImage = baseImageOverride
	}

	yamlOutput, err := renderKubeObjects(metadata, scriptBody, parsedParams, baseImage)
	if err != nil {
		return err
	}

	if outputFile != "" {
		return os.WriteFile(outputFile, []byte(yamlOutput), 0600)
	}
	fmt.Print(yamlOutput)
	return nil
}

func readScriptFromFiles(sourceDir string) (backplaneApi.ScriptMetadata, string, error) {
	var metadata backplaneApi.ScriptMetadata

	metaFile := sourceDir + "metadata.yaml"
	yamlFile, err := os.ReadFile(metaFile) //nolint:gosec
	if err != nil {
		return metadata, "", fmt.Errorf("error reading metadata.yaml: %v (ensure you are in a script directory or specify --source-dir)", err)
	}

	if err := yaml.Unmarshal(yamlFile, &metadata); err != nil {
		return metadata, "", fmt.Errorf("error parsing metadata.yaml: %v", err)
	}

	scriptFile := sourceDir + metadata.File
	fileBody, err := os.ReadFile(scriptFile) //nolint:gosec
	if err != nil {
		return metadata, "", fmt.Errorf("unable to read script file %s: %v", scriptFile, err)
	}

	fileBodyStr := string(fileBody)
	fileBodyStr, err = inlineLibrarySourceFiles(fileBodyStr, scriptFile)
	if err != nil {
		return metadata, "", err
	}

	return metadata, fileBodyStr, nil
}

func validateParams(metadata backplaneApi.ScriptMetadata, params map[string]string) error {
	if metadata.Envs == nil {
		if len(params) > 0 {
			return fmt.Errorf("script doesn't accept parameters")
		}
		return nil
	}

	for _, env := range metadata.Envs {
		if env.Key == nil {
			continue
		}
		if env.Optional != nil && !*env.Optional {
			if _, ok := params[*env.Key]; !ok {
				return fmt.Errorf("missing required parameter: %s", *env.Key)
			}
		}
	}

	for key := range params {
		found := false
		for _, env := range metadata.Envs {
			if env.Key != nil && *env.Key == key {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid parameter: %s", key)
		}
	}

	return nil
}

func renderKubeObjects(metadata backplaneApi.ScriptMetadata, scriptBody string, params map[string]string, baseImage string) (string, error) {
	name := fmt.Sprintf("%s%d", generateNamePrefixForTestScript, time.Now().Unix())

	labels := map[string]string{
		"managed.openshift.io/backplane-job-canonical-namespace":   "TEST",
		"managed.openshift.io/backplane-job-canonical-script-name": metadata.Name,
		"managed.openshift.io/backplane-job-is-test":               "true",
		"managed.openshift.io/backplane-job-id":                    name,
	}

	var objects []string

	// ServiceAccount
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: backplaneJobsNamespace,
			Labels:    labels,
		},
		AutomountServiceAccountToken: ptrBool(true),
	}
	saYAML, err := yaml.Marshal(sa)
	if err != nil {
		return "", fmt.Errorf("error marshalling ServiceAccount: %v", err)
	}
	objects = append(objects, string(saYAML))

	// Namespaced Roles and RoleBindings
	if metadata.Rbac.Roles != nil {
		for _, roleDecl := range *metadata.Rbac.Roles {
			if roleDecl.Namespace == nil || *roleDecl.Namespace == "" || roleDecl.Rules == nil || len(*roleDecl.Rules) == 0 {
				continue
			}
			rules := convertPolicyRules(*roleDecl.Rules)
			role := &rbacv1.Role{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "Role",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: *roleDecl.Namespace,
					Labels:    labels,
				},
				Rules: rules,
			}
			roleYAML, err := yaml.Marshal(role)
			if err != nil {
				return "", fmt.Errorf("error marshalling Role: %v", err)
			}
			objects = append(objects, string(roleYAML))

			rb := &rbacv1.RoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "RoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: *roleDecl.Namespace,
					Labels:    labels,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      name,
						Namespace: backplaneJobsNamespace,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "Role",
					Name:     name,
				},
			}
			rbYAML, err := yaml.Marshal(rb)
			if err != nil {
				return "", fmt.Errorf("error marshalling RoleBinding: %v", err)
			}
			objects = append(objects, string(rbYAML))
		}
	}

	// ClusterRole and ClusterRoleBinding
	if metadata.Rbac.ClusterRoleRules != nil && len(*metadata.Rbac.ClusterRoleRules) > 0 {
		rules := convertPolicyRules(*metadata.Rbac.ClusterRoleRules)
		cr := &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Rules: rules,
		}
		crYAML, err := yaml.Marshal(cr)
		if err != nil {
			return "", fmt.Errorf("error marshalling ClusterRole: %v", err)
		}
		objects = append(objects, string(crYAML))

		crb := &rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      name,
					Namespace: backplaneJobsNamespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     name,
			},
		}
		crbYAML, err := yaml.Marshal(crb)
		if err != nil {
			return "", fmt.Errorf("error marshalling ClusterRoleBinding: %v", err)
		}
		objects = append(objects, string(crbYAML))
	}

	// Pod
	envVars := buildEnvVars(metadata, params)
	podCommand := getPodCommand(metadata.Language, scriptBody)
	runAsNonRoot := true

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: backplaneJobsNamespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: name,
			RestartPolicy:      corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   &runAsNonRoot,
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				{
					Name:    "job",
					Image:   baseImage,
					Command: podCommand,
					Env:     envVars,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("2048Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptrBool(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						RunAsNonRoot: &runAsNonRoot,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
						{
							Weight: 100,
							Preference: corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{{
									Key:      "node-role.kubernetes.io/infra",
									Operator: "Exists",
								}},
							},
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{{
				Key:      "node-role.kubernetes.io/infra",
				Operator: "Exists",
				Effect:   "NoSchedule",
			}},
		},
	}
	podYAML, err := yaml.Marshal(pod)
	if err != nil {
		return "", fmt.Errorf("error marshalling Pod: %v", err)
	}
	objects = append(objects, string(podYAML))

	return strings.Join(objects, "---\n"), nil
}

func convertPolicyRules(rules []backplaneApi.PolicyRule) []rbacv1.PolicyRule {
	var k8sRules []rbacv1.PolicyRule
	for _, r := range rules {
		rule := rbacv1.PolicyRule{}
		if r.Verbs != nil {
			rule.Verbs = *r.Verbs
		}
		if r.ApiGroups != nil {
			rule.APIGroups = *r.ApiGroups
		}
		if r.Resources != nil {
			rule.Resources = *r.Resources
		}
		if r.ResourceNames != nil {
			rule.ResourceNames = *r.ResourceNames
		}
		if r.NonResourceURLs != nil {
			rule.NonResourceURLs = *r.NonResourceURLs
		}
		k8sRules = append(k8sRules, rule)
	}
	return k8sRules
}

func buildEnvVars(metadata backplaneApi.ScriptMetadata, params map[string]string) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if metadata.Envs == nil {
		return envVars
	}
	for _, env := range metadata.Envs {
		if env.Key == nil {
			continue
		}
		if val, ok := params[*env.Key]; ok {
			envVars = append(envVars, corev1.EnvVar{
				Name:  *env.Key,
				Value: val,
			})
		}
	}
	return envVars
}

func getPodCommand(language backplaneApi.ScriptMetadataLanguage, scriptBody string) []string {
	encoded := base64.StdEncoding.EncodeToString([]byte(scriptBody))
	switch language {
	case backplaneApi.ScriptMetadataLanguagePython:
		return []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' | base64 -d | /bin/python3", encoded)}
	case backplaneApi.ScriptMetadataLanguageBash:
		return []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' | base64 -d | /bin/bash", encoded)}
	default:
		return []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' | base64 -d | /bin/bash", encoded)}
	}
}

func ptrBool(b bool) *bool {
	return &b
}
