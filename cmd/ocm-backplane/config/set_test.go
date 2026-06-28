package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("set command", func() {
	var (
		tempDir    string
		configFile string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "set-test-*")
		Expect(err).To(BeNil())

		configFile = filepath.Join(tempDir, "config.json")
		os.Setenv("BACKPLANE_CONFIG", configFile)

		viper.Reset()
	})

	AfterEach(func() {
		os.Unsetenv("BACKPLANE_CONFIG")
		os.RemoveAll(tempDir)
		viper.Reset()
	})

	writeConfig := func(content string) {
		err := os.WriteFile(configFile, []byte(content), 0600)
		Expect(err).To(BeNil())
	}

	readConfig := func() map[string]interface{} {
		data, err := os.ReadFile(configFile)
		Expect(err).To(BeNil())
		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		Expect(err).To(BeNil())
		return result
	}

	runSet := func(key, value string) error {
		viper.Reset()
		cmd := newSetCmd()
		cmd.SetArgs([]string{key, value})
		return cmd.Execute()
	}

	Context("preserving string proxy-url", func() {
		It("should preserve string proxy-url when setting jira-email", func() {
			writeConfig(`{
				"url": "https://backplane.example.com",
				"proxy-url": "http://proxy.example.com:3128",
				"jira-email": "old@example.com"
			}`)

			err := runSet("jira-email", "new@example.com")
			Expect(err).To(BeNil())

			cfg := readConfig()
			Expect(cfg["jira-email"]).To(Equal("new@example.com"))
			Expect(cfg["proxy-url"]).To(Equal("http://proxy.example.com:3128"))
			Expect(cfg["url"]).To(Equal("https://backplane.example.com"))
		})
	})

	Context("preserving array proxy-url", func() {
		const arrayConfig = `{
			"url": "https://backplane.example.com",
			"proxy-url": ["http://proxy1.example.com:3128", "http://proxy2.example.com:3128"],
			"session-dir": "/tmp/bp-session",
			"pd-key": "existing-pd-key",
			"jira-token": "existing-jira-token",
			"jira-email": "existing@example.com",
			"govcloud": false
		}`

		It("should preserve array proxy-url when setting jira-email", func() {
			writeConfig(arrayConfig)

			err := runSet("jira-email", "new@example.com")
			Expect(err).To(BeNil())

			cfg := readConfig()
			Expect(cfg["jira-email"]).To(Equal("new@example.com"))

			proxyURL, ok := cfg["proxy-url"].([]interface{})
			Expect(ok).To(BeTrue(), "proxy-url should remain an array")
			Expect(proxyURL).To(HaveLen(2))
			Expect(proxyURL[0]).To(Equal("http://proxy1.example.com:3128"))
			Expect(proxyURL[1]).To(Equal("http://proxy2.example.com:3128"))
		})

		It("should preserve array proxy-url when setting jira-token", func() {
			writeConfig(arrayConfig)

			err := runSet("jira-token", "new-token")
			Expect(err).To(BeNil())

			cfg := readConfig()
			Expect(cfg["jira-token"]).To(Equal("new-token"))

			proxyURL, ok := cfg["proxy-url"].([]interface{})
			Expect(ok).To(BeTrue(), "proxy-url should remain an array")
			Expect(proxyURL).To(HaveLen(2))
		})

		It("should preserve array proxy-url when setting pd-key", func() {
			writeConfig(arrayConfig)

			err := runSet("pd-key", "new-pd-key")
			Expect(err).To(BeNil())

			cfg := readConfig()
			Expect(cfg["pd-key"]).To(Equal("new-pd-key"))

			proxyURL, ok := cfg["proxy-url"].([]interface{})
			Expect(ok).To(BeTrue(), "proxy-url should remain an array")
			Expect(proxyURL).To(HaveLen(2))
		})
	})

	Context("preserving server-managed fields", func() {
		It("should preserve server-managed fields when setting a local config key", func() {
			writeConfig(`{
				"url": "https://backplane.example.com",
				"proxy-url": "http://proxy.example.com:3128",
				"pd-key": "old-pd-key",
				"jira-base-url": "https://redhat.atlassian.net",
				"assume-initial-arn": "arn:aws:iam::123456789012:role/Example",
				"prod-env-name": "production",
				"jira-config-for-access-requests": {
					"default-project": "SDAINT",
					"default-issue-type": "Story"
				}
			}`)

			err := runSet("pd-key", "new-pd-key")
			Expect(err).To(BeNil())

			cfg := readConfig()
			Expect(cfg["pd-key"]).To(Equal("new-pd-key"))
			Expect(cfg["jira-base-url"]).To(Equal("https://redhat.atlassian.net"))
			Expect(cfg["assume-initial-arn"]).To(Equal("arn:aws:iam::123456789012:role/Example"))
			Expect(cfg["prod-env-name"]).To(Equal("production"))

			jiraConfig, ok := cfg["jira-config-for-access-requests"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(jiraConfig["default-project"]).To(Equal("SDAINT"))
			Expect(jiraConfig["default-issue-type"]).To(Equal("Story"))
		})
	})

	Context("setting proxy-url explicitly", func() {
		It("should update proxy-url when explicitly set", func() {
			writeConfig(`{
				"url": "https://backplane.example.com",
				"proxy-url": "http://old-proxy.example.com:3128",
				"jira-email": "user@example.com"
			}`)

			err := runSet("proxy-url", "http://new-proxy.example.com:3128")
			Expect(err).To(BeNil())

			cfg := readConfig()
			Expect(cfg["proxy-url"]).To(Equal("http://new-proxy.example.com:3128"))
			Expect(cfg["jira-email"]).To(Equal("user@example.com"))
			Expect(cfg["url"]).To(Equal("https://backplane.example.com"))
		})
	})

	Context("no spurious fields", func() {
		It("should not add empty fields that were not in the original config", func() {
			writeConfig(`{
				"url": "https://backplane.example.com",
				"proxy-url": "http://proxy.example.com:3128"
			}`)

			err := runSet("url", "https://new-backplane.example.com")
			Expect(err).To(BeNil())

			cfg := readConfig()
			Expect(cfg["url"]).To(Equal("https://new-backplane.example.com"))
			Expect(cfg["proxy-url"]).To(Equal("http://proxy.example.com:3128"))

			_, hasJiraToken := cfg["jira-token"]
			Expect(hasJiraToken).To(BeFalse(), "jira-token should not be added when not in original config")

			_, hasSessionDir := cfg["session-dir"]
			Expect(hasSessionDir).To(BeFalse(), "session-dir should not be added when not in original config")

			_, hasJiraEmail := cfg["jira-email"]
			Expect(hasJiraEmail).To(BeFalse(), "jira-email should not be added when not in original config")
		})
	})

	Context("error handling", func() {
		It("should reject unsupported config keys", func() {
			writeConfig(`{"url": "https://backplane.example.com"}`)

			err := runSet("unsupported-key", "value")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("supported config variables"))
		})

		It("should reject invalid govcloud values", func() {
			writeConfig(`{"url": "https://backplane.example.com"}`)

			err := runSet("govcloud", "not-a-bool")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid value for govcloud"))
		})
	})
})
