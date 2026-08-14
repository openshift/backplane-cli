package testjob

import (
	"fmt"
	"os"
	"path"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

const testImage = "quay.io/test/managed-scripts:abc1234"

var _ = Describe("testJob render command", func() {

	var (
		tempDir string
		sut     *cobra.Command
	)

	BeforeEach(func() {
		tempDir, _ = os.MkdirTemp("", "renderJobTest")
		sut = NewTestJobCommand()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Context("render test job YAML", func() {
		It("should render YAML for a simple script with cluster role rules", func() {
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(MetadataYaml), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo hello"), 0600)

			outputFile := path.Join(tempDir, "output.yaml")
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "--output", outputFile, "-i", testImage})
			err := sut.Execute()

			Expect(err).To(BeNil())

			content, err := os.ReadFile(outputFile)
			Expect(err).To(BeNil())

			yamlStr := string(content)
			Expect(yamlStr).To(ContainSubstring("kind: ServiceAccount"))
			Expect(yamlStr).To(ContainSubstring("kind: ClusterRole"))
			Expect(yamlStr).To(ContainSubstring("kind: ClusterRoleBinding"))
			Expect(yamlStr).To(ContainSubstring("kind: Role"))
			Expect(yamlStr).To(ContainSubstring("kind: RoleBinding"))
			Expect(yamlStr).To(ContainSubstring("kind: Pod"))
			Expect(yamlStr).To(ContainSubstring("namespace: openshift-backplane-managed-scripts"))
			Expect(yamlStr).To(ContainSubstring("namespace: kube-system"))
			Expect(yamlStr).To(ContainSubstring("openshift-job-dev-"))
			Expect(yamlStr).To(ContainSubstring("managed.openshift.io/backplane-job-is-test: \"true\""))
			Expect(yamlStr).To(ContainSubstring("image: " + testImage))
		})

		It("should render YAML for a script with only namespaced roles", func() {
			metadata := `
file: script.sh
name: ns-only
description: namespaced only
author: tester
allowedGroups:
  - SREP
rbac:
  roles:
    - namespace: "openshift-monitoring"
      rules:
        - verbs: ["get", "list"]
          apiGroups: [""]
          resources: ["configmaps"]
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo hello"), 0600)

			outputFile := path.Join(tempDir, "output.yaml")
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "--output", outputFile, "-i", testImage})
			err := sut.Execute()

			Expect(err).To(BeNil())

			content, err := os.ReadFile(outputFile)
			Expect(err).To(BeNil())

			yamlStr := string(content)
			Expect(yamlStr).To(ContainSubstring("kind: ServiceAccount"))
			Expect(yamlStr).To(ContainSubstring("kind: Role"))
			Expect(yamlStr).To(ContainSubstring("kind: RoleBinding"))
			Expect(yamlStr).To(ContainSubstring("namespace: openshift-monitoring"))
			Expect(yamlStr).To(ContainSubstring("kind: Pod"))
			Expect(yamlStr).NotTo(ContainSubstring("kind: ClusterRole"))
		})

		It("should include env vars from parameters", func() {
			metadata := `
file: script.sh
name: with-params
description: script with params
author: tester
allowedGroups:
  - SREP
envs:
  - key: MY_VAR
    description: "A param"
    optional: false
rbac:
  roles: []
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo $MY_VAR"), 0600)

			outputFile := path.Join(tempDir, "output.yaml")
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "--output", outputFile, "-p", "MY_VAR=hello", "-i", testImage})
			err := sut.Execute()

			Expect(err).To(BeNil())

			content, err := os.ReadFile(outputFile)
			Expect(err).To(BeNil())

			yamlStr := string(content)
			Expect(yamlStr).To(ContainSubstring("name: MY_VAR"))
			Expect(yamlStr).To(ContainSubstring("value: hello"))
		})

		It("should auto-resolve image from the resolver when --base-image-override is not provided", func() {
			original := resolveBaseImageSHA
			resolveBaseImageSHA = func() (string, error) { return "deadbeef", nil }
			defer func() { resolveBaseImageSHA = original }()

			metadata := `
file: script.sh
name: auto-resolve
description: auto resolve image
author: tester
rbac:
  roles: []
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo test"), 0600)

			outputFile := path.Join(tempDir, "output.yaml")
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "--output", outputFile})
			err := sut.Execute()

			Expect(err).To(BeNil())

			content, err := os.ReadFile(outputFile)
			Expect(err).To(BeNil())

			yamlStr := string(content)
			Expect(yamlStr).To(ContainSubstring("image: " + baseImageRegistry + ":deadbeef"))
		})

		It("should propagate an error when the resolver fails", func() {
			original := resolveBaseImageSHA
			resolveBaseImageSHA = func() (string, error) { return "", fmt.Errorf("boom") }
			defer func() { resolveBaseImageSHA = original }()

			metadata := `
file: script.sh
name: resolve-fail
description: resolver failure
author: tester
rbac:
  roles: []
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo test"), 0600)

			sut.SetArgs([]string{"render", "--source-dir", tempDir})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to resolve managed-scripts image tag"))
		})

		It("should merge rules from multiple rbac.roles entries sharing a namespace", func() {
			metadata := `
file: script.sh
name: shared-ns
description: shared namespace roles
author: tester
allowedGroups:
  - SREP
rbac:
  roles:
    - namespace: "openshift-monitoring"
      rules:
        - verbs: ["get"]
          apiGroups: [""]
          resources: ["configmaps"]
    - namespace: "openshift-monitoring"
      rules:
        - verbs: ["list"]
          apiGroups: [""]
          resources: ["secrets"]
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo hello"), 0600)

			outputFile := path.Join(tempDir, "output.yaml")
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "--output", outputFile, "-i", testImage})
			err := sut.Execute()

			Expect(err).To(BeNil())

			content, err := os.ReadFile(outputFile)
			Expect(err).To(BeNil())

			yamlStr := string(content)
			// Only one top-level Role should be emitted for the shared namespace,
			// containing both rule sets. Count only document-level "kind: Role"
			// lines (no leading indentation) to avoid matching the roleRef inside
			// the RoleBinding.
			topLevelRoles := 0
			for _, line := range strings.Split(yamlStr, "\n") {
				if line == "kind: Role" {
					topLevelRoles++
				}
			}
			Expect(topLevelRoles).To(Equal(1))
			Expect(yamlStr).To(ContainSubstring("configmaps"))
			Expect(yamlStr).To(ContainSubstring("secrets"))
		})

		It("should fail when a required parameter is missing", func() {
			metadata := `
file: script.sh
name: needs-param
description: needs a param
author: tester
envs:
  - key: REQUIRED_VAR
    description: "required"
    optional: false
rbac:
  roles: []
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo test"), 0600)

			sut.SetArgs([]string{"render", "--source-dir", tempDir, "-i", testImage})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("missing required parameter"))
		})

		It("should fail when an invalid parameter is provided", func() {
			metadata := `
file: script.sh
name: valid-only
description: valid only
author: tester
envs:
  - key: VALID_KEY
    description: "valid"
    optional: true
rbac:
  roles: []
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo test"), 0600)

			sut.SetArgs([]string{"render", "--source-dir", tempDir, "-p", "INVALID_KEY=abc", "-i", testImage})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid parameter"))
		})

		It("should fail when metadata.yaml is missing", func() {
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "-i", testImage})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("error reading metadata.yaml"))
		})

		It("should fail when script file is missing", func() {
			metadata := `
file: missing.sh
name: missing
description: missing script
author: tester
rbac:
  roles: []
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)

			sut.SetArgs([]string{"render", "--source-dir", tempDir, "-i", testImage})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("unable to read script file"))
		})

		It("should use the provided base image in the pod spec", func() {
			metadata := `
file: script.sh
name: custom-img
description: custom image
author: tester
rbac:
  roles: []
language: bash
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo test"), 0600)

			outputFile := path.Join(tempDir, "output.yaml")
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "--output", outputFile, "-i", "quay.io/custom/image:v1"})
			err := sut.Execute()

			Expect(err).To(BeNil())

			content, err := os.ReadFile(outputFile)
			Expect(err).To(BeNil())

			yamlStr := string(content)
			Expect(yamlStr).To(ContainSubstring("image: quay.io/custom/image:v1"))
		})

		It("should render python command for python scripts", func() {
			metadata := `
file: script.py
name: python-test
description: python script
author: tester
rbac:
  roles: []
language: python
`
			_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(metadata), 0600)
			_ = os.WriteFile(path.Join(tempDir, "script.py"), []byte("print('hello')"), 0600)

			outputFile := path.Join(tempDir, "output.yaml")
			sut.SetArgs([]string{"render", "--source-dir", tempDir, "--output", outputFile, "-i", testImage})
			err := sut.Execute()

			Expect(err).To(BeNil())

			content, err := os.ReadFile(outputFile)
			Expect(err).To(BeNil())

			yamlStr := string(content)
			Expect(yamlStr).To(ContainSubstring("/bin/python3"))
		})
	})
})
